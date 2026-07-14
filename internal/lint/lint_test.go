// Linter tests: one focused case per rule (or tight rule family), plus the
// Fix rewrites. Each case is a realistic doc/CI snippet, because these
// rules exist to catch what actually rots in READMEs.
package lint

import (
	"strings"
	"testing"

	"github.com/JaydenCJ/curlfmt/internal/format"
	"github.com/JaydenCJ/curlfmt/internal/parse"
	"github.com/JaydenCJ/curlfmt/internal/shell"
)

func mustCmd(t *testing.T, in string) *parse.Command {
	t.Helper()
	res, err := shell.Lex(in)
	if err != nil {
		t.Fatalf("lex %q: %v", in, err)
	}
	cmd, err := parse.Parse(res.Words, res.Suffix)
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	return cmd
}

func codes(fs []Finding) map[string]Finding {
	m := map[string]Finding{}
	for _, f := range fs {
		m[f.Code] = f
	}
	return m
}

func assertCode(t *testing.T, in, code string, sev Severity) Finding {
	t.Helper()
	fs := Check(mustCmd(t, in))
	f, ok := codes(fs)[code]
	if !ok {
		t.Fatalf("Check(%q): %s missing; got %+v", in, code, fs)
	}
	if f.Severity != sev {
		t.Fatalf("%s severity = %s, want %s", code, f.Severity, sev)
	}
	return f
}

func assertNoCode(t *testing.T, in, code string) {
	t.Helper()
	if f, ok := codes(Check(mustCmd(t, in)))[code]; ok {
		t.Fatalf("Check(%q): unexpected %s (%s)", in, code, f.Message)
	}
}

func TestCF001CF002RedundantExplicitMethod(t *testing.T) {
	f := assertCode(t, "curl -X GET https://example.test", "CF001", Warning)
	if !f.Fixable {
		t.Fatal("CF001 must be fixable")
	}
	assertNoCode(t, "curl -X GET -d q=1 https://example.test", "CF001")

	assertCode(t, "curl -X POST -d a=1 https://example.test", "CF002", Warning)
	// POST without a body is NOT redundant: curl would send GET.
	assertNoCode(t, "curl -X POST https://example.test", "CF002")
	// -G reroutes the data, so the explicit POST matters.
	assertNoCode(t, "curl -G -X POST -d a=1 https://example.test", "CF002")
}

func TestCF003Insecure(t *testing.T) {
	assertCode(t, "curl -k https://example.test", "CF003", Warning)
	assertNoCode(t, "curl https://example.test", "CF003")
}

func TestCF004CF012CredentialLeaks(t *testing.T) {
	// Password in the URL is an error; bare username a warning.
	f := assertCode(t, "curl https://bob:hunter2@example.test/x", "CF004", Error)
	if !strings.Contains(f.Message, "password") {
		t.Fatalf("message = %q", f.Message)
	}
	assertCode(t, "curl https://bob@example.test/x", "CF004", Warning)
	// Inline --user password is advice only (it is at least not in a URL
	// that ends up in logs and browser history).
	assertCode(t, "curl -u bob:hunter2 https://example.test", "CF012", Info)
	assertNoCode(t, "curl -u bob https://example.test", "CF012")
}

func TestCF005PlainHTTPNonLoopback(t *testing.T) {
	assertCode(t, "curl http://api.example.test/v1", "CF005", Warning)
	for _, ok := range []string{
		"curl http://127.0.0.1:8080/health",
		"curl http://localhost:3000/",
		"curl http://api.localhost/x",
		"curl https://api.example.test/v1",
	} {
		assertNoCode(t, ok, "CF005")
	}
}

func TestCF006SilentWithoutShowError(t *testing.T) {
	assertCode(t, "curl -s https://example.test", "CF006", Warning)
	assertNoCode(t, "curl -sS https://example.test", "CF006")
}

func TestCF007MissingFailIsInfoOnly(t *testing.T) {
	assertCode(t, "curl https://example.test", "CF007", Info)
	assertNoCode(t, "curl -f https://example.test", "CF007")
	assertNoCode(t, "curl --fail-with-body https://example.test", "CF007")
}

func TestCF008DuplicateHeader(t *testing.T) {
	in := "curl -H 'Accept: a' -H 'accept: b' https://example.test"
	f := assertCode(t, in, "CF008", Warning)
	if !strings.Contains(f.Message, "Accept") {
		t.Fatalf("message = %q", f.Message)
	}
	assertNoCode(t, "curl -H 'Accept: a' -H 'X-B: b' https://example.test", "CF008")
}

func TestCF009UnquotedAmpersandSplitsQuery(t *testing.T) {
	assertCode(t, "curl https://example.test/s?a=1&b=2", "CF009", Error)
	// Quoted URL: the & is data, no finding.
	assertNoCode(t, "curl 'https://example.test/s?a=1&b=2'", "CF009")
	// && is a control operator, not a lost query parameter.
	assertNoCode(t, "curl https://example.test/s?a=1 && echo ok", "CF009")
}

func TestCF010CF013BodyMethodConflicts(t *testing.T) {
	assertCode(t, "curl -X GET -d q=1 https://example.test", "CF010", Warning)
	assertNoCode(t, "curl -G -d q=1 https://example.test", "CF010")

	assertCode(t, "curl -I -d x=1 https://example.test", "CF013", Warning)
	assertNoCode(t, "curl -I https://example.test", "CF013")
}

func TestCF011CF018CF019ParserNoteRules(t *testing.T) {
	f := assertCode(t, "curl --frobnicate https://example.test", "CF011", Warning)
	if !strings.Contains(f.Message, "--frobnicate") {
		t.Fatalf("message = %q", f.Message)
	}
	assertCode(t, "curl --max-time=5 https://example.test", "CF018", Warning)
	assertCode(t, "curl https://example.test -H", "CF019", Error)
}

func TestCF014CF015URLPresenceAndScheme(t *testing.T) {
	assertCode(t, "curl -s", "CF014", Error)
	assertCode(t, "curl example.test/api", "CF015", Warning)
	assertNoCode(t, "curl https://example.test/api", "CF015")
}

func TestCF016JSONFlagVersusExplicitHeaders(t *testing.T) {
	assertCode(t, `curl --json '{}' -H 'Content-Type: text/plain' https://example.test`, "CF016", Warning)
	assertNoCode(t, `curl --json '{}' -H 'X-Trace: 1' https://example.test`, "CF016")
}

func TestCF017RepeatedLastOneWinsOption(t *testing.T) {
	f := assertCode(t, "curl -X PUT -X PATCH https://example.test", "CF017", Warning)
	if !strings.Contains(f.Message, "--request") {
		t.Fatalf("message = %q", f.Message)
	}
	// Repeating --header is normal; no finding.
	assertNoCode(t, "curl -H 'A: 1' -H 'B: 2' https://example.test", "CF017")
}

func TestExpansionURLsAreSkippedByURLRules(t *testing.T) {
	// $BASE_URL cannot be judged statically; no CF005/CF015/CF004 noise.
	for _, code := range []string{"CF005", "CF015", "CF004"} {
		assertNoCode(t, `curl "$BASE_URL/health"`, code)
	}
}

func TestWorstSeverity(t *testing.T) {
	fs := Check(mustCmd(t, "curl https://bob:pw@example.test"))
	worst, ok := WorstSeverity(fs)
	if !ok || worst != Error {
		t.Fatalf("worst = %v, %v", worst, ok)
	}
	if _, ok := WorstSeverity(nil); ok {
		t.Fatal("no findings must report ok=false")
	}
}

func TestFixAppliesSafeRewrites(t *testing.T) {
	// One messy command exercising every fix: redundant -X GET dropped,
	// --show-error paired with --silent, identical duplicate header
	// collapsed, differing duplicate kept.
	cmd := mustCmd(t, "curl -s -X GET -H 'Accept: a' -H 'accept: a' -H 'X-K: 1' -H 'X-K: 2' https://example.test")
	applied := Fix(cmd)
	if cmd.Has("request") {
		t.Fatal("--request GET should be removed")
	}
	if !cmd.Has("show-error") {
		t.Fatal("--show-error should be added")
	}
	var accepts, xks int
	for _, it := range cmd.Items {
		if it.Known && it.Opt.Long == "header" {
			switch format.HeaderName(it.Value) {
			case "Accept":
				accepts++
			case "X-K":
				xks++
			}
		}
	}
	if accepts != 1 {
		t.Fatalf("identical Accept headers not collapsed: %d", accepts)
	}
	if xks != 2 {
		t.Fatalf("differing X-K headers must both survive: %d", xks)
	}
	want := map[string]bool{"CF001": true, "CF006": true, "CF008": true}
	for _, c := range applied {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Fatalf("applied = %v, missing %v", applied, want)
	}
}

func TestFixIsIdempotentAndConservative(t *testing.T) {
	cmd := mustCmd(t, "curl -s -X GET -H 'A: 1' -H 'A: 1' https://example.test")
	Fix(cmd)
	if again := Fix(cmd); len(again) != 0 {
		t.Fatalf("second Fix applied %v", again)
	}
	// A command with nothing redundant is left completely alone.
	cmd = mustCmd(t, "curl -sS -X DELETE https://example.test")
	if applied := Fix(cmd); len(applied) != 0 {
		t.Fatalf("applied = %v", applied)
	}
	if !cmd.Has("request") {
		t.Fatal("--request DELETE must survive")
	}
}
