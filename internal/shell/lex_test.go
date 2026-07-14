// Lexer tests: quoting, escapes, continuations, expansions, operators.
// The lexer is the layer everything else trusts, so failure modes
// (unterminated quotes, trailing backslashes) get explicit coverage.
package shell

import (
	"errors"
	"testing"
)

func lexWords(t *testing.T, in string) []Word {
	t.Helper()
	res, err := Lex(in)
	if err != nil {
		t.Fatalf("Lex(%q) error: %v", in, err)
	}
	return res.Words
}

func assertValues(t *testing.T, in string, got []Word, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Lex(%q): got %d words, want %d", in, len(got), len(want))
	}
	for i := range want {
		if got[i].Value != want[i] {
			t.Fatalf("Lex(%q): word %d = %q, want %q", in, i, got[i].Value, want[i])
		}
	}
}

func TestLexSplitsSimpleWordsAcrossWhitespaceRuns(t *testing.T) {
	in := "curl   \t -s\t\thttps://example.test"
	assertValues(t, in, lexWords(t, in), "curl", "-s", "https://example.test")
}

func TestLexQuotingForms(t *testing.T) {
	// Every POSIX quoting form the formatter must round-trip: single
	// quotes (fully literal), double quotes (escape processing), lone
	// backslashes, unknown double-quote escapes (kept), and adjacent
	// quoting styles joining into one word.
	cases := []struct {
		in   string
		want []string
	}{
		{`curl 'a b $HOME \n'`, []string{"curl", `a b $HOME \n`}},
		{`curl "say \"hi\" \\ ok"`, []string{"curl", `say "hi" \ ok`}},
		{`curl "a\nb"`, []string{"curl", `a\nb`}},
		{`curl a\ b`, []string{"curl", "a b"}},
		{`curl 'a'"b"c`, []string{"curl", "abc"}},
	}
	for _, c := range cases {
		assertValues(t, c.in, lexWords(t, c.in), c.want...)
	}
}

func TestLexQuotedTextWithoutExpansionIsLiteral(t *testing.T) {
	for _, in := range []string{`curl '$HOME literal'`, `curl "plain text"`, `curl "cost \$5"`} {
		ws := lexWords(t, in)
		if !ws[1].Literal {
			t.Fatalf("Lex(%q): word must be literal", in)
		}
	}
	// The escaped dollar unquotes to a plain dollar sign.
	if ws := lexWords(t, `curl "cost \$5"`); ws[1].Value != "cost $5" {
		t.Fatalf("got %q", ws[1].Value)
	}
}

func TestLexExpansionsAreNonLiteralAndKeepRawSource(t *testing.T) {
	// $VAR, ${...}, $(...), and backticks are live shell syntax: the word
	// must be flagged non-literal and its Raw preserved for verbatim
	// re-emission.
	cases := []struct {
		in, value, raw string
		idx            int
	}{
		{`curl -H "Bearer $TOKEN" u`, "Bearer $TOKEN", `"Bearer $TOKEN"`, 2},
		{`curl ${HOST:-example.test}/api u`, "${HOST:-example.test}/api", "${HOST:-example.test}/api", 1},
		{`curl $(dirname "$0")/x u`, `$(dirname "$0")/x`, `$(dirname "$0")/x`, 1},
		{"curl `date +%s` u", "`date +%s`", "`date +%s`", 1},
	}
	for _, c := range cases {
		ws := lexWords(t, c.in)
		w := ws[c.idx]
		if w.Literal {
			t.Fatalf("Lex(%q): word %q must be non-literal", c.in, w.Value)
		}
		if w.Value != c.value || w.Raw != c.raw {
			t.Fatalf("Lex(%q): got value=%q raw=%q, want %q / %q", c.in, w.Value, w.Raw, c.value, c.raw)
		}
	}
}

func TestLexContinuationsJoinLinesAndWords(t *testing.T) {
	in := "curl -s \\\n  -L url"
	assertValues(t, in, lexWords(t, in), "curl", "-s", "-L", "url")
	// POSIX: backslash-newline vanishes even mid-word.
	in = "curl ab\\\ncd"
	assertValues(t, in, lexWords(t, in), "curl", "abcd")
}

func TestLexNewlineInsideSingleQuotesIsData(t *testing.T) {
	in := "curl '{\n  \"a\": 1\n}' u"
	assertValues(t, in, lexWords(t, in), "curl", "{\n  \"a\": 1\n}", "u")
}

func TestLexComments(t *testing.T) {
	in := "curl -s url # fetch it"
	assertValues(t, in, lexWords(t, in), "curl", "-s", "url")
	// '#' inside a word is data, not a comment.
	in = "curl url#frag"
	assertValues(t, in, lexWords(t, in), "curl", "url#frag")
}

func TestLexOperatorsSplitCommandAndSuffix(t *testing.T) {
	cases := []struct {
		in     string
		words  []string
		suffix string
	}{
		{"curl -s url | jq '.name'", []string{"curl", "-s", "url"}, "| jq '.name'"},
		{"curl http://h/p?a=1&b=2", []string{"curl", "http://h/p?a=1"}, "&b=2"},
		{"curl url > out.json", []string{"curl", "url"}, "> out.json"},
		{"curl url && echo ok", []string{"curl", "url"}, "&& echo ok"},
		{"curl url; echo done", []string{"curl", "url"}, "; echo done"},
	}
	for _, c := range cases {
		res, err := Lex(c.in)
		if err != nil {
			t.Fatalf("Lex(%q): %v", c.in, err)
		}
		assertValues(t, c.in, res.Words, c.words...)
		if res.Suffix != c.suffix {
			t.Fatalf("Lex(%q): suffix = %q, want %q", c.in, res.Suffix, c.suffix)
		}
	}
}

func TestLexUnterminatedInputsError(t *testing.T) {
	// Statement gatherers rely on ErrUnterminated as the "feed me another
	// line" signal, so each open construct must produce it.
	for _, in := range []string{"curl 'oops", `curl "oops`, `curl -s \`, "curl ${X", "curl $(x", "curl `x"} {
		if _, err := Lex(in); !errors.Is(err, ErrUnterminated) {
			t.Fatalf("Lex(%q): err = %v, want ErrUnterminated", in, err)
		}
	}
}

func TestCompleteReflectsLexability(t *testing.T) {
	if Complete("curl 'open") {
		t.Fatal("open quote must be incomplete")
	}
	if !Complete("curl -s url") {
		t.Fatal("plain command must be complete")
	}
	res, err := Lex("   \n  ")
	if err != nil || len(res.Words) != 0 || res.Suffix != "" {
		t.Fatalf("blank input: got %+v, %v", res, err)
	}
}
