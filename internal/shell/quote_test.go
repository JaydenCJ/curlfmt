// Quoter tests: the formatter's output must round-trip through a real
// shell unchanged, so canonical quoting is deliberately conservative.
package shell

import "testing"

func TestQuoteStringCanonicalForms(t *testing.T) {
	cases := []struct{ in, want string }{
		// Bare when every byte is safe.
		{"https://example.test/a-b_c.d", "https://example.test/a-b_c.d"},
		{"POST", "POST"},
		{"a=b", "a=b"},
		{"user@host:path", "user@host:path"},
		// Single-quoted when anything could be shell syntax.
		{"a b", "'a b'"},
		{"a&b", "'a&b'"},
		{"a|b", "'a|b'"},
		{"a;b", "'a;b'"},
		{"a>b", "'a>b'"},
		{"a*b", "'a*b'"},
		{"a?b", "'a?b'"},
		{"a{b}", "'a{b}'"},
		{"~x", "'~x'"},
		{"!x", "'!x'"},
		{"a#b", "'a#b'"},
		{"a$b", "'a$b'"},
		{"a`b", "'a`b'"},
		// Edge shapes.
		{"", "''"},
		{"it's", `'it'\''s'`},
		{"{\n}", "'{\n}'"},
	}
	for _, c := range cases {
		if got := QuoteString(c.in); got != c.want {
			t.Fatalf("QuoteString(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestQuoteWordRespectsLiteralness(t *testing.T) {
	// Live expansions are emitted verbatim from their raw source; literal
	// words get canonical quoting even if the source over-quoted them.
	w := Word{Value: "Bearer $TOKEN", Raw: `"Bearer $TOKEN"`, Literal: false}
	if got := Quote(w); got != `"Bearer $TOKEN"` {
		t.Fatalf("got %q, want the raw source preserved", got)
	}
	w = Word{Value: "GET", Raw: `"GET"`, Literal: true}
	if got := Quote(w); got != "GET" {
		t.Fatalf("got %q, want bare GET", got)
	}
}
