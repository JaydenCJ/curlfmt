// Package shell provides a small POSIX-flavored lexer and quoter for the
// single-command subset curlfmt cares about: one simple command, possibly
// spread over continuation lines, possibly followed by a pipe or another
// operator whose text must be preserved verbatim.
package shell

import (
	"errors"
	"strings"
)

// ErrUnterminated is returned when the input ends inside a quoted string or
// on a line-continuation backslash. Callers gathering a statement line by
// line use it as the "feed me another line" signal.
var ErrUnterminated = errors.New("shell: unterminated quote or trailing continuation")

// Word is one shell word after quote processing.
//
// Literal words carry their fully-unescaped Value and may be re-quoted
// canonically. Non-literal words contain live shell syntax — parameter
// expansions, command substitutions, arithmetic — whose meaning would change
// if re-quoted, so the formatter must emit Raw verbatim.
type Word struct {
	Value   string // unescaped text (best effort for non-literal words)
	Raw     string // the exact source text of the word
	Literal bool   // true when Value fully captures the word's meaning
}

// Result is the outcome of lexing one logical command line.
type Result struct {
	Words []Word
	// Suffix is the raw remainder starting at the first unquoted control
	// operator (|, ;, &, &&, ||, redirections, parentheses), with leading
	// whitespace trimmed. Empty when the whole input was one command.
	Suffix string
}

// operator characters that terminate a simple command.
const operatorChars = "|&;<>()"

// Lex splits input into the words of the first simple command plus a raw
// suffix. Line continuations (backslash-newline) are removed outside single
// quotes; newlines inside quotes are preserved as data. A '#' at the start
// of a word begins a comment that runs to end of line.
func Lex(input string) (Result, error) {
	l := &lexer{src: input}
	return l.run()
}

// Complete reports whether input forms a lexically complete command: no
// unterminated quote and no trailing continuation backslash. Statement
// gatherers use it to decide whether to consume another source line.
func Complete(input string) bool {
	_, err := Lex(input)
	return err == nil
}

type lexer struct {
	src string
	pos int
}

func (l *lexer) run() (Result, error) {
	var res Result
	for {
		l.skipBlank()
		if l.pos >= len(l.src) {
			return res, nil
		}
		c := l.src[l.pos]
		if c == '#' {
			l.skipComment()
			continue
		}
		if strings.IndexByte(operatorChars, c) >= 0 {
			res.Suffix = strings.TrimSpace(l.src[l.pos:])
			return res, nil
		}
		w, err := l.word()
		if err != nil {
			return res, err
		}
		res.Words = append(res.Words, w)
	}
}

// skipBlank consumes spaces, tabs, newlines, and backslash-newline
// continuations between words.
func (l *lexer) skipBlank() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			l.pos++
		case c == '\\' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n':
			l.pos += 2
		default:
			return
		}
	}
}

func (l *lexer) skipComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.pos++
	}
}

// word consumes one shell word starting at l.pos.
func (l *lexer) word() (Word, error) {
	start := l.pos
	var val strings.Builder
	literal := true
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			return l.finish(start, val.String(), literal), nil
		case strings.IndexByte(operatorChars, c) >= 0:
			return l.finish(start, val.String(), literal), nil
		case c == '\'':
			s, err := l.singleQuoted()
			if err != nil {
				return Word{}, err
			}
			val.WriteString(s)
		case c == '"':
			s, lit, err := l.doubleQuoted()
			if err != nil {
				return Word{}, err
			}
			literal = literal && lit
			val.WriteString(s)
		case c == '\\':
			if l.pos+1 >= len(l.src) {
				return Word{}, ErrUnterminated
			}
			next := l.src[l.pos+1]
			if next == '\n' {
				// Continuation: the word continues on the next line.
				l.pos += 2
				continue
			}
			val.WriteByte(next)
			l.pos += 2
		case c == '$' || c == '`':
			// Live shell syntax outside quotes: expansion or substitution.
			literal = false
			if err := l.consumeExpansion(&val); err != nil {
				return Word{}, err
			}
		default:
			val.WriteByte(c)
			l.pos++
		}
	}
	return l.finish(start, val.String(), literal), nil
}

func (l *lexer) finish(start int, value string, literal bool) Word {
	return Word{Value: value, Raw: l.src[start:l.pos], Literal: literal}
}

// singleQuoted consumes '...' starting at the opening quote and returns the
// literal contents. Everything, including newlines, is data.
func (l *lexer) singleQuoted() (string, error) {
	l.pos++ // opening quote
	start := l.pos
	for l.pos < len(l.src) {
		if l.src[l.pos] == '\'' {
			s := l.src[start:l.pos]
			l.pos++
			return s, nil
		}
		l.pos++
	}
	return "", ErrUnterminated
}

// doubleQuoted consumes "..." and returns the contents with backslash
// escapes for \" \\ \` \$ processed. It reports literal=false when the
// contents include an unescaped $ or ` (live expansion).
func (l *lexer) doubleQuoted() (string, bool, error) {
	l.pos++ // opening quote
	var val strings.Builder
	literal := true
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch c {
		case '"':
			l.pos++
			return val.String(), literal, nil
		case '\\':
			if l.pos+1 >= len(l.src) {
				return "", false, ErrUnterminated
			}
			next := l.src[l.pos+1]
			switch next {
			case '"', '\\', '$', '`':
				val.WriteByte(next)
				l.pos += 2
			case '\n':
				l.pos += 2 // continuation inside double quotes
			default:
				val.WriteByte('\\')
				val.WriteByte(next)
				l.pos += 2
			}
		case '$', '`':
			literal = false
			val.WriteByte(c)
			l.pos++
		default:
			val.WriteByte(c)
			l.pos++
		}
	}
	return "", false, ErrUnterminated
}

// consumeExpansion copies $VAR, ${...}, $(...), or `...` verbatim into val,
// tracking nesting for ${} and $() so words like "$(dirname "$0")" survive.
func (l *lexer) consumeExpansion(val *strings.Builder) error {
	c := l.src[l.pos]
	if c == '`' {
		val.WriteByte(c)
		l.pos++
		for l.pos < len(l.src) {
			ch := l.src[l.pos]
			val.WriteByte(ch)
			l.pos++
			if ch == '`' {
				return nil
			}
		}
		return ErrUnterminated
	}
	// c == '$'
	val.WriteByte(c)
	l.pos++
	if l.pos >= len(l.src) {
		return nil
	}
	switch l.src[l.pos] {
	case '{':
		return l.consumeBracketed(val, '{', '}')
	case '(':
		return l.consumeBracketed(val, '(', ')')
	default:
		for l.pos < len(l.src) && isNameByte(l.src[l.pos]) {
			val.WriteByte(l.src[l.pos])
			l.pos++
		}
		return nil
	}
}

func (l *lexer) consumeBracketed(val *strings.Builder, open, close byte) error {
	depth := 0
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		val.WriteByte(ch)
		l.pos++
		switch ch {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return nil
			}
		}
	}
	return ErrUnterminated
}

func isNameByte(c byte) bool {
	return c == '_' || c == '?' || c == '#' || c == '@' || c == '*' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
