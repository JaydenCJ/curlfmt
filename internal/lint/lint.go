// Package lint implements curlfmt's rule set: nineteen checks that catch
// redundant, dangerous, or subtly broken curl usage in docs and CI. Every
// rule has a stable CF-prefixed code, a severity, and — where a rewrite is
// provably semantics-preserving — a fix applied by `curlfmt --fix`.
package lint

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/JaydenCJ/curlfmt/internal/format"
	"github.com/JaydenCJ/curlfmt/internal/parse"
	"github.com/JaydenCJ/curlfmt/internal/spec"
)

// Severity orders findings. Errors describe commands that are broken or
// leak secrets; warnings describe traps; infos are advice.
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "info"
	}
}

// Finding is one lint result.
type Finding struct {
	Code     string
	Severity Severity
	Message  string
	Fixable  bool
}

// Check runs every rule against cmd and returns findings in rule order.
func Check(cmd *parse.Command) []Finding {
	var fs []Finding
	add := func(code string, sev Severity, fixable bool, msg string, args ...any) {
		fs = append(fs, Finding{Code: code, Severity: sev, Fixable: fixable,
			Message: fmt.Sprintf(msg, args...)})
	}

	method := requestMethod(cmd)
	hasData := cmd.HasData()

	// CF001 / CF002: redundant explicit method.
	if method == "GET" && !hasData {
		add("CF001", Warning, true, "--request GET is curl's default; drop it")
	}
	if method == "POST" && hasData && !cmd.Has("get") {
		add("CF002", Warning, true, "--request POST is implied by the data option; drop it")
	}

	// CF003: TLS verification disabled.
	if cmd.Has("insecure") {
		add("CF003", Warning, false, "--insecure disables TLS certificate verification")
	}

	// CF004 / CF005 / CF015: URL problems.
	for _, u := range cmd.URLs {
		if !u.Literal {
			continue
		}
		v := u.Value
		if hasScheme(v) {
			parsed, err := url.Parse(v)
			if err == nil && parsed.User != nil {
				if _, hasPw := parsed.User.Password(); hasPw {
					add("CF004", Error, false, "URL embeds a password in its userinfo; use --user or a .netrc file")
				} else {
					add("CF004", Warning, false, "URL embeds a username in its userinfo; prefer --user")
				}
			}
			if strings.HasPrefix(v, "http://") && !loopbackHost(parsed) {
				add("CF005", Warning, false, "plain-text http:// to a non-loopback host; use https://")
			}
		} else if !strings.HasPrefix(v, "-") {
			add("CF015", Warning, false, "URL %q has no scheme; curl will guess, which differs across versions", v)
		}
	}

	// CF006: silent mode eats error messages.
	if cmd.Has("silent") && !cmd.Has("show-error") {
		add("CF006", Warning, true, "--silent without --show-error hides curl's own error messages; add --show-error")
	}

	// CF007: HTTP errors exit zero.
	if !cmd.Has("fail") && !cmd.Has("fail-with-body") {
		add("CF007", Info, false, "without --fail or --fail-with-body, HTTP 4xx/5xx still exit 0 (dangerous in CI)")
	}

	// CF008: duplicate header fields.
	for _, name := range duplicateHeaders(cmd) {
		add("CF008", Warning, true, "header %q is set more than once", name)
	}

	// CF009: an unquoted & split the command at the query string.
	if strings.HasPrefix(cmd.Suffix, "&") && !strings.HasPrefix(cmd.Suffix, "&&") && lastURLHasQuery(cmd) {
		add("CF009", Error, false, "unquoted '&' in the query string backgrounds the command and drops the rest of the URL; quote the URL")
	}

	// CF010: body with an explicit GET.
	if method == "GET" && hasData && !cmd.Has("get") {
		add("CF010", Warning, false, "--request GET with a body sends a GET with a payload; use --get to move data into the query string")
	}

	// CF011 / CF018 / CF019: parser observations.
	for _, n := range cmd.Notes {
		switch n.Kind {
		case parse.NoteUnknownLong, parse.NoteUnknownShort:
			add("CF011", Warning, false, "unknown option %s (passed through unchanged)", n.Text)
		case parse.NoteEqualsForm:
			add("CF018", Warning, true, "curl does not accept %s=value; separate the value with a space", n.Text)
		case parse.NoteMissingValue:
			add("CF019", Error, false, "option %s is missing its value", n.Text)
		}
	}

	// CF012: password on the command line.
	if it, ok := cmd.First("user"); ok && it.Value.Literal && strings.Contains(it.Value.Value, ":") {
		add("CF012", Info, false, "--user carries an inline password; prefer a .netrc file or omit the password to be prompted")
	}

	// CF013: HEAD requests cannot carry a body.
	if cmd.Has("head") && hasData {
		add("CF013", Warning, false, "--head with a request body is contradictory; the body will confuse servers")
	}

	// CF014: nothing to fetch.
	if len(cmd.URLs) == 0 {
		add("CF014", Error, false, "no URL found")
	}

	// CF016: --json already sets the JSON headers.
	if cmd.Has("json") {
		for _, name := range headerNames(cmd) {
			if name == "Content-Type" || name == "Accept" {
				add("CF016", Warning, false, "--json already sets %s; the explicit header conflicts", name)
			}
		}
	}

	// CF017: repeating a last-one-wins option.
	for _, long := range repeatedSingleUse(cmd) {
		add("CF017", Warning, false, "--%s is given more than once; curl keeps only the last value", long)
	}

	return fs
}

// WorstSeverity returns the highest severity present, and ok=false when
// there are no findings.
func WorstSeverity(fs []Finding) (Severity, bool) {
	if len(fs) == 0 {
		return Info, false
	}
	worst := Info
	for _, f := range fs {
		if f.Severity > worst {
			worst = f.Severity
		}
	}
	return worst, true
}

// Fix applies every safe, semantics-preserving fix to cmd in place and
// returns the codes it applied (deduplicated, in rule order). Callers run
// it before formatting; the rewrites mirror exactly the Fixable findings.
func Fix(cmd *parse.Command) []string {
	var applied []string
	mark := func(code string) {
		for _, c := range applied {
			if c == code {
				return
			}
		}
		applied = append(applied, code)
	}

	method := requestMethod(cmd)
	hasData := cmd.HasData()

	// CF001 / CF002: drop the redundant --request.
	if (method == "GET" && !hasData) || (method == "POST" && hasData && !cmd.Has("get")) {
		code := "CF001"
		if method == "POST" {
			code = "CF002"
		}
		cmd.Items = removeOption(cmd.Items, "request")
		mark(code)
	}

	// CF006: pair --silent with --show-error.
	if cmd.Has("silent") && !cmd.Has("show-error") {
		opt, _ := spec.ByLong("show-error")
		cmd.Items = append(cmd.Items, parse.Item{Opt: opt, Known: true})
		mark("CF006")
	}

	// CF008: drop byte-identical duplicate headers (differing values are
	// kept — repeating a header with different values can be intentional).
	seen := map[string]bool{}
	var kept []parse.Item
	for _, it := range cmd.Items {
		if it.Known && it.Opt.Long == "header" && it.Value.Literal {
			key := format.HeaderName(it.Value)
			if key != "" {
				id := key + "\x00" + headerValue(it.Value.Value)
				if seen[id] {
					mark("CF008")
					continue
				}
				seen[id] = true
			}
		}
		kept = append(kept, it)
	}
	cmd.Items = kept

	// CF018: equals-form options are re-emitted split by the formatter;
	// clearing the flag records the fix.
	for i := range cmd.Items {
		if cmd.Items[i].EqualsForm {
			cmd.Items[i].EqualsForm = false
			mark("CF018")
		}
	}
	var notes []parse.Note
	for _, n := range cmd.Notes {
		if n.Kind == parse.NoteEqualsForm {
			continue
		}
		notes = append(notes, n)
	}
	cmd.Notes = notes

	return applied
}

func requestMethod(cmd *parse.Command) string {
	it, ok := cmd.First("request")
	if !ok || !it.Value.Literal {
		return ""
	}
	return strings.ToUpper(it.Value.Value)
}

var schemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

func hasScheme(v string) bool { return schemeRe.MatchString(v) }

func loopbackHost(u *url.URL) bool {
	h := u.Hostname()
	return h == "localhost" || h == "::1" || h == "0.0.0.0" ||
		strings.HasPrefix(h, "127.") || strings.HasSuffix(h, ".localhost")
}

func headerNames(cmd *parse.Command) []string {
	var names []string
	for _, it := range cmd.Items {
		if it.Known && it.Opt.Long == "header" {
			if n := format.HeaderName(it.Value); n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

func duplicateHeaders(cmd *parse.Command) []string {
	counts := map[string]int{}
	var order []string
	for _, n := range headerNames(cmd) {
		if counts[n] == 0 {
			order = append(order, n)
		}
		counts[n]++
	}
	var dups []string
	for _, n := range order {
		if counts[n] > 1 {
			dups = append(dups, n)
		}
	}
	return dups
}

func repeatedSingleUse(cmd *parse.Command) []string {
	counts := map[string]int{}
	var order []string
	for _, it := range cmd.Items {
		if it.Known && it.Opt.SingleUse {
			if counts[it.Opt.Long] == 0 {
				order = append(order, it.Opt.Long)
			}
			counts[it.Opt.Long]++
		}
	}
	var out []string
	for _, long := range order {
		if counts[long] > 1 {
			out = append(out, long)
		}
	}
	return out
}

func lastURLHasQuery(cmd *parse.Command) bool {
	if len(cmd.URLs) == 0 {
		return false
	}
	return strings.Contains(cmd.URLs[len(cmd.URLs)-1].Value, "?")
}

func removeOption(items []parse.Item, long string) []parse.Item {
	var out []parse.Item
	for _, it := range items {
		if it.Known && it.Opt.Long == long {
			continue
		}
		out = append(out, it)
	}
	return out
}

// headerValue returns the part after the colon, trimmed, for duplicate
// detection.
func headerValue(v string) string {
	if i := strings.IndexByte(v, ':'); i >= 0 {
		return strings.TrimSpace(v[i+1:])
	}
	return v
}
