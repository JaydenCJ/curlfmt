package shell

import "strings"

// bareSafe are the bytes that never need quoting in any POSIX shell when
// they make up a whole word. Deliberately conservative: glob characters,
// braces, tildes, and '!' are excluded even where most shells would cope.
func bareSafe(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("%+,-./:=@_", c) >= 0
}

// Quote renders a Word back to shell source in canonical form.
//
// Non-literal words (live expansions) are reproduced verbatim so their
// runtime meaning is untouched. Literal words are emitted bare when every
// byte is safe, otherwise single-quoted with embedded single quotes
// escaped via the standard backslash dance.
func Quote(w Word) string {
	if !w.Literal {
		return w.Raw
	}
	return QuoteString(w.Value)
}

// QuoteString canonically quotes a plain string value.
func QuoteString(v string) string {
	if v == "" {
		return "''"
	}
	safe := true
	for i := 0; i < len(v); i++ {
		if !bareSafe(v[i]) {
			safe = false
			break
		}
	}
	if safe {
		return v
	}
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
