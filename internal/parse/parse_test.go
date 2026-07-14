// Parser tests: option recognition, short clusters, attached values,
// passthrough of the unknown, and the notes the linter consumes.
package parse

import (
	"testing"

	"github.com/JaydenCJ/curlfmt/internal/shell"
)

func mustParse(t *testing.T, in string) *Command {
	t.Helper()
	res, err := shell.Lex(in)
	if err != nil {
		t.Fatalf("lex %q: %v", in, err)
	}
	cmd, err := Parse(res.Words, res.Suffix)
	if err != nil {
		t.Fatalf("parse %q: %v", in, err)
	}
	return cmd
}

func TestParseRejectsNonCurl(t *testing.T) {
	res, _ := shell.Lex("wget https://example.test")
	if _, err := Parse(res.Words, ""); err == nil {
		t.Fatal("want error for non-curl command")
	}
}

func TestParsePositionalBecomesURL(t *testing.T) {
	c := mustParse(t, "curl https://example.test/a")
	if len(c.URLs) != 1 || c.URLs[0].Value != "https://example.test/a" {
		t.Fatalf("URLs = %+v", c.URLs)
	}
}

func TestParseShortClustersAndAttachedValues(t *testing.T) {
	// The four shapes curl accepts for short options: pure boolean
	// clusters, a trailing valued option taking the next argument, an
	// attached value, and long options with a separate value.
	c := mustParse(t, "curl -sSL https://example.test")
	for _, long := range []string{"silent", "show-error", "location"} {
		if !c.Has(long) {
			t.Fatalf("-sSL: missing %s", long)
		}
	}

	c = mustParse(t, "curl -sX POST https://example.test")
	if it, ok := c.First("request"); !ok || it.Value.Value != "POST" || !c.Has("silent") {
		t.Fatalf("-sX POST: %+v", c.Items)
	}

	c = mustParse(t, "curl -XPOST -d@body.json https://example.test")
	if it, _ := c.First("request"); it.Value.Value != "POST" {
		t.Fatalf("-XPOST value = %q", it.Value.Value)
	}
	if it, _ := c.First("data"); it.Value.Value != "@body.json" {
		t.Fatalf("-d@ value = %q", it.Value.Value)
	}

	c = mustParse(t, "curl --max-time 5 https://example.test")
	if it, ok := c.First("max-time"); !ok || it.Value.Value != "5" {
		t.Fatalf("--max-time: %+v, %v", it, ok)
	}

	// A lone dash is a value (stdin/stdout), never an option.
	c = mustParse(t, "curl -T - https://example.test")
	if it, ok := c.First("upload-file"); !ok || it.Value.Value != "-" {
		t.Fatalf("upload-file = %+v, %v", it, ok)
	}
}

func TestParseNotesForLinter(t *testing.T) {
	// Every parser observation the linter turns into a finding: equals
	// form, unknown long, unknown short, missing value.
	c := mustParse(t, "curl --max-time=5 https://example.test")
	if it, ok := c.First("max-time"); !ok || it.Value.Value != "5" || !it.EqualsForm {
		t.Fatalf("equals form: %+v, %v", it, ok)
	}
	if len(c.Notes) != 1 || c.Notes[0].Kind != NoteEqualsForm {
		t.Fatalf("notes = %+v", c.Notes)
	}

	c = mustParse(t, "curl --frobnicate https://example.test")
	if len(c.Items) != 1 || c.Items[0].Known || c.Items[0].Raw != "--frobnicate" {
		t.Fatalf("unknown long: %+v", c.Items)
	}
	if len(c.Notes) != 1 || c.Notes[0].Kind != NoteUnknownLong {
		t.Fatalf("notes = %+v", c.Notes)
	}

	c = mustParse(t, "curl https://example.test --header")
	if len(c.Notes) != 1 || c.Notes[0].Kind != NoteMissingValue || c.Notes[0].Text != "--header" {
		t.Fatalf("missing value: %+v", c.Notes)
	}
}

func TestParseUnknownShortPreservesWholeCluster(t *testing.T) {
	// -sQ: Q is unknown, so the entire original token survives verbatim —
	// splitting it could change what a curl fork would do.
	c := mustParse(t, "curl -sQ https://example.test")
	found := false
	for _, it := range c.Items {
		if !it.Known && it.Raw == "-sQ" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cluster not preserved: %+v", c.Items)
	}
	if len(c.Notes) != 1 || c.Notes[0].Kind != NoteUnknownShort {
		t.Fatalf("notes = %+v", c.Notes)
	}
}

func TestParseURLOptionJoinsPositionals(t *testing.T) {
	c := mustParse(t, "curl --url https://a.example.test https://b.example.test")
	if len(c.URLs) != 2 {
		t.Fatalf("URLs = %+v", c.URLs)
	}
	if len(c.Items) != 0 {
		t.Fatalf("--url must not remain an item: %+v", c.Items)
	}
}

func TestParseKeepsMultipleDataInOrder(t *testing.T) {
	c := mustParse(t, "curl -d a=1 -d b=2 https://example.test")
	var vals []string
	for _, it := range c.Items {
		if it.Opt.Long == "data" {
			vals = append(vals, it.Value.Value)
		}
	}
	if len(vals) != 2 || vals[0] != "a=1" || vals[1] != "b=2" {
		t.Fatalf("data order = %v", vals)
	}
}

func TestParseExpansionFlagWordPassesThrough(t *testing.T) {
	// A word like -H"$AUTH" mixes an option with an expansion; it must
	// survive verbatim so runtime behavior is untouched.
	c := mustParse(t, `curl -H"$AUTH" https://example.test`)
	if len(c.Items) != 1 || c.Items[0].Known || c.Items[0].Raw != `-H"$AUTH"` {
		t.Fatalf("items = %+v", c.Items)
	}
}

func TestParseNegatedBooleanKeepsSpelling(t *testing.T) {
	c := mustParse(t, "curl --no-progress-meter https://example.test")
	if !c.Has("no-progress-meter") {
		t.Fatalf("items = %+v", c.Items)
	}
}

func TestParseSuffixCarriedThrough(t *testing.T) {
	c := mustParse(t, "curl -s https://example.test | jq .")
	if c.Suffix != "| jq ." {
		t.Fatalf("suffix = %q", c.Suffix)
	}
}

func TestHasDataSeesEveryBodyOption(t *testing.T) {
	for _, in := range []string{
		"curl -d x u", "curl --data-raw x u", "curl --data-binary @f u",
		"curl --data-urlencode a=b u", "curl -F a=b u", "curl --json '{}' u",
	} {
		if !mustParse(t, in).HasData() {
			t.Fatalf("HasData(%q) = false", in)
		}
	}
	if mustParse(t, "curl -s u").HasData() {
		t.Fatal("HasData must be false without body options")
	}
}
