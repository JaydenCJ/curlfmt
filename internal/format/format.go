// Package format renders a parsed curl command in curlfmt's canonical
// style: long option names, one option per continuation line, boolean flags
// sorted on the curl line, options grouped (method, auth, headers, body,
// everything else, output), URLs last, values quoted canonically, header
// names normalized. Formatting is semantics-preserving and idempotent.
package format

import (
	"net/textproto"
	"regexp"
	"sort"
	"strings"

	"github.com/JaydenCJ/curlfmt/internal/parse"
	"github.com/JaydenCJ/curlfmt/internal/shell"
	"github.com/JaydenCJ/curlfmt/internal/spec"
)

// Options controls rendering.
type Options struct {
	// Width is the longest a command may be and still stay on one line.
	Width int
	// Indent prefixes continuation lines (the caller's base indentation is
	// applied separately by the source rewriters).
	Indent string
}

// DefaultWidth is the single-line threshold used when Options.Width is 0.
const DefaultWidth = 80

// Format renders cmd canonically. The result never ends with a newline.
func Format(cmd *parse.Command, opts Options) string {
	if opts.Width == 0 {
		opts.Width = DefaultWidth
	}
	if opts.Indent == "" {
		opts.Indent = "  "
	}

	flags, valued := split(cmd)
	head := "curl"
	if len(flags) > 0 {
		head += " " + strings.Join(flags, " ")
	}

	var tail []string
	tail = append(tail, valued...)
	for _, u := range cmd.URLs {
		tail = append(tail, shell.Quote(u))
	}

	oneLine := head
	if len(tail) > 0 {
		oneLine += " " + strings.Join(tail, " ")
	}
	if cmd.Suffix != "" {
		oneLine += " " + cmd.Suffix
	}
	if len(oneLine) <= opts.Width && !strings.Contains(oneLine, "\n") {
		return oneLine
	}
	if len(tail) == 0 {
		return oneLine // nothing to break across lines
	}

	var b strings.Builder
	b.WriteString(head)
	for i, part := range tail {
		b.WriteString(" \\\n")
		b.WriteString(opts.Indent)
		b.WriteString(part)
		if i == len(tail)-1 && cmd.Suffix != "" {
			b.WriteString(" ")
			b.WriteString(cmd.Suffix)
		}
	}
	return b.String()
}

// groupRank is the canonical print order of the valued-option groups.
var groupRank = map[spec.Group]int{
	spec.GroupMethod: 0,
	spec.GroupAuth:   1,
	spec.GroupHeader: 2,
	spec.GroupData:   3,
	spec.GroupOther:  4,
	spec.GroupOutput: 5,
}

// split separates sorted boolean flags from the ordered valued/passthrough
// parts.
func split(cmd *parse.Command) (flags []string, valued []string) {
	type ranked struct {
		rank int
		text string
	}
	var parts []ranked
	var bools []string
	for _, it := range cmd.Items {
		switch {
		case !it.Known:
			parts = append(parts, ranked{groupRank[spec.GroupOther], it.Raw})
		case !it.Opt.TakesValue:
			bools = append(bools, it.Opt.Long)
		default:
			text := "--" + it.Opt.Long + " " + shell.Quote(canonValue(it))
			parts = append(parts, ranked{groupRank[it.Opt.Group], text})
		}
	}
	flags = canonFlags(bools)
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].rank < parts[j].rank })
	for _, p := range parts {
		valued = append(valued, p.text)
	}
	return flags, valued
}

// canonFlags reduces the boolean flags (in source order) to a sorted set.
// A flag and its --no- negation belong to one family, and curl applies
// them left to right, so only the last spelling of each family survives:
// emitting both --silent and --no-silent in sorted order would silently
// flip which one wins.
func canonFlags(bools []string) []string {
	lastByFamily := map[string]string{}
	for _, long := range bools {
		lastByFamily[flagFamily(long)] = long
	}
	flags := make([]string, 0, len(lastByFamily))
	for _, long := range lastByFamily {
		flags = append(flags, "--"+long)
	}
	sort.Strings(flags)
	return flags
}

// flagFamily maps a --no-<flag> negation to its base flag's name. Long
// names that merely start with "no-" (--no-buffer, --noproxy) are their
// own family because the bare name is not a boolean in the spec table.
func flagFamily(long string) string {
	if rest, ok := strings.CutPrefix(long, "no-"); ok {
		if o, exists := spec.ByLong(rest); exists && !o.TakesValue {
			return rest
		}
	}
	return long
}

// knownMethods gates method uppercasing: only spellings of real HTTP
// methods are normalized, custom methods pass through untouched.
var knownMethods = map[string]string{
	"GET": "GET", "HEAD": "HEAD", "POST": "POST", "PUT": "PUT",
	"PATCH": "PATCH", "DELETE": "DELETE", "OPTIONS": "OPTIONS",
	"TRACE": "TRACE", "CONNECT": "CONNECT",
}

// headerRe matches "Field-Name: value" shapes eligible for normalization.
var headerRe = regexp.MustCompile(`^([!#$%&'*+.^_` + "`" + `|~0-9A-Za-z-]+):[ \t]*(\S.*)$`)

// canonValue applies per-option value normalization to literal values.
func canonValue(it parse.Item) shell.Word {
	w := it.Value
	if !w.Literal {
		return w
	}
	switch it.Opt.Long {
	case "request":
		if m, ok := knownMethods[strings.ToUpper(w.Value)]; ok {
			return shell.Word{Value: m, Literal: true}
		}
	case "header":
		if m := headerRe.FindStringSubmatch(w.Value); m != nil {
			name := textproto.CanonicalMIMEHeaderKey(m[1])
			return shell.Word{Value: name + ": " + m[2], Literal: true}
		}
	}
	return w
}

// HeaderName extracts the canonical field name from a literal --header
// value, or "" when the value is not a plain "Name: value" header.
func HeaderName(w shell.Word) string {
	if !w.Literal {
		return ""
	}
	if m := headerRe.FindStringSubmatch(w.Value); m != nil {
		return textproto.CanonicalMIMEHeaderKey(m[1])
	}
	return ""
}
