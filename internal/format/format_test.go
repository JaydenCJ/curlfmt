// Formatter tests: canonical layout, grouping, quoting, normalization,
// and — the property that makes curlfmt safe to run in CI — idempotence.
package format

import (
	"strings"
	"testing"

	"github.com/JaydenCJ/curlfmt/internal/parse"
	"github.com/JaydenCJ/curlfmt/internal/shell"
)

func fmtCmd(t *testing.T, in string, width int) string {
	t.Helper()
	res, err := shell.Lex(in)
	if err != nil {
		t.Fatalf("lex %q: %v", in, err)
	}
	cmd, err := parse.Parse(res.Words, res.Suffix)
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	return Format(cmd, Options{Width: width})
}

func TestFormatSingleLineCanonicalization(t *testing.T) {
	// Short commands stay on one line: long names, sorted deduplicated
	// boolean flags.
	cases := []struct{ in, want string }{
		{"curl -s https://example.test", "curl --silent https://example.test"},
		{"curl -fsSL https://example.test", "curl --fail --location --show-error --silent https://example.test"},
		{"curl -v -s --verbose https://example.test", "curl --silent --verbose https://example.test"},
	}
	for _, c := range cases {
		if got := fmtCmd(t, c.in, 80); got != c.want {
			t.Fatalf("fmt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatNegatedFlagKeepsLastSpelling(t *testing.T) {
	// curl applies boolean flags left to right, so --silent --no-silent
	// means "not silent". Sorting both spellings side by side would flip
	// the winner; only the last spelling of a flag family may survive.
	cases := []struct{ in, want string }{
		{"curl --silent --no-silent https://example.test", "curl --no-silent https://example.test"},
		{"curl --no-silent -s https://example.test", "curl --silent https://example.test"},
		// --no-buffer is a real long name (short -N), not a negation of a
		// known boolean: it is its own family and never collapses.
		{"curl -N -s https://example.test", "curl --no-buffer --silent https://example.test"},
	}
	for _, c := range cases {
		if got := fmtCmd(t, c.in, 100); got != c.want {
			t.Fatalf("fmt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatWidthControlsLineBreaking(t *testing.T) {
	in := "curl -s -H 'A: b' https://example.test"
	if got := fmtCmd(t, in, 200); strings.Contains(got, "\n") {
		t.Fatalf("wide: got multi-line %q", got)
	}
	if got := fmtCmd(t, in, 20); !strings.Contains(got, " \\\n") {
		t.Fatalf("narrow: got single line %q", got)
	}
}

func TestFormatMultiLineLayoutOneOptionPerLine(t *testing.T) {
	got := fmtCmd(t, `curl -X PUT -H 'Accept: application/json' -d '{"a":1}' https://api.example.test/items/1`, 40)
	want := strings.Join([]string{
		"curl \\",
		"  --request PUT \\",
		"  --header 'Accept: application/json' \\",
		`  --data '{"a":1}' \`,
		"  https://api.example.test/items/1",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatGroupOrderMethodAuthHeaderDataOtherOutput(t *testing.T) {
	got := fmtCmd(t, "curl -o out.json --retry 3 --frobnicate -d x=1 -H 'A: b' -u bob:pw -X PATCH https://example.test", 1)
	lines := strings.Split(got, "\n")
	wantOrder := []string{"--request", "--user", "--header", "--data", "--retry", "--frobnicate", "--output"}
	if len(lines) != len(wantOrder)+2 {
		t.Fatalf("got %d lines:\n%s", len(lines), got)
	}
	for i, opt := range wantOrder {
		if !strings.HasPrefix(strings.TrimSpace(lines[i+1]), opt) {
			t.Fatalf("line %d = %q, want %s first\nfull:\n%s", i+1, lines[i+1], opt, got)
		}
	}
}

func TestFormatURLsAlwaysLastInOrder(t *testing.T) {
	got := fmtCmd(t, "curl https://a.example.test -H 'A: b' https://b.example.test", 1)
	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[len(lines)-2], "a.example.test") ||
		strings.TrimSpace(strings.TrimSuffix(lines[len(lines)-1], " \\")) != "https://b.example.test" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestFormatMethodCasing(t *testing.T) {
	// Known HTTP methods are uppercased; custom methods pass through.
	if got := fmtCmd(t, "curl -X delete https://example.test", 80); !strings.Contains(got, "--request DELETE") {
		t.Fatalf("got %q", got)
	}
	if got := fmtCmd(t, "curl -X Purge-All https://example.test", 80); !strings.Contains(got, "--request Purge-All") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHeaderNormalization(t *testing.T) {
	// Field names get canonical casing and one space after the colon.
	got := fmtCmd(t, `curl -H "content-type:application/json" https://example.test`, 80)
	if !strings.Contains(got, "--header 'Content-Type: application/json'") {
		t.Fatalf("got %q", got)
	}
	// "Name;" sends an empty header, "Name:" unsets one — both survive
	// untouched because they do not match the Name: value shape.
	got = fmtCmd(t, `curl -H 'X-Empty;' -H 'Accept:' https://example.test`, 200)
	if !strings.Contains(got, "'X-Empty;'") || !strings.Contains(got, "--header Accept:") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatPreservesExpansionsVerbatim(t *testing.T) {
	got := fmtCmd(t, `curl -H "Authorization: Bearer $TOKEN" https://example.test`, 200)
	if !strings.Contains(got, `--header "Authorization: Bearer $TOKEN"`) {
		t.Fatalf("got %q", got)
	}
}

func TestFormatPreservesMultiLineQuotedBody(t *testing.T) {
	in := "curl -d '{\n  \"a\": 1\n}' https://example.test"
	got := fmtCmd(t, in, 20)
	if !strings.Contains(got, "--data '{\n  \"a\": 1\n}'") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestFormatAppendsSuffixAfterLastLine(t *testing.T) {
	got := fmtCmd(t, "curl -s -H 'A: b' https://example.test | jq '.name'", 1)
	lines := strings.Split(got, "\n")
	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, "https://example.test | jq '.name'") {
		t.Fatalf("last line = %q", last)
	}
}

func TestFormatSplitsEqualsFormOptions(t *testing.T) {
	// curl rejects --opt=value, so canonical output always separates it.
	got := fmtCmd(t, "curl --connect-timeout=3 https://example.test", 80)
	if !strings.Contains(got, "--connect-timeout 3") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatQuotesQueryStringURLs(t *testing.T) {
	got := fmtCmd(t, `curl 'https://example.test/s?q=a&b=1'`, 80)
	if !strings.Contains(got, `'https://example.test/s?q=a&b=1'`) {
		t.Fatalf("got %q", got)
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	inputs := []string{
		"curl -sSLX POST -H 'a: b' -H 'c: d' -d x=1 -d y=2 'https://example.test/p?q=1' -o out",
		`curl -H "Bearer $T" --retry 2 https://example.test`,
		"curl -d '{\n \"k\": \"v\"\n}' -u bob https://example.test",
		"curl -s https://example.test | jq .",
		"curl --frobnicate -s https://example.test",
	}
	for _, in := range inputs {
		once := fmtCmd(t, in, 60)
		twice := fmtCmd(t, once, 60)
		if once != twice {
			t.Fatalf("not idempotent for %q:\nonce:\n%s\ntwice:\n%s", in, once, twice)
		}
	}
}

func TestHeaderNameExtraction(t *testing.T) {
	w := shell.Word{Value: "x-request-id: 7", Literal: true}
	if got := HeaderName(w); got != "X-Request-Id" {
		t.Fatalf("got %q", got)
	}
	if got := HeaderName(shell.Word{Value: "not a header", Literal: true}); got != "" {
		t.Fatalf("got %q for non-header", got)
	}
}
