// Package source locates curl statements inside documents — Markdown
// files, shell scripts, or raw command text — and rewrites them in place
// while leaving every other byte untouched.
package source

import (
	"path/filepath"
	"strings"

	"github.com/JaydenCJ/curlfmt/internal/shell"
)

// Statement is one curl invocation found in a document.
type Statement struct {
	// Text is the logical command: prompt stripped from the first line,
	// continuation lines joined verbatim with newlines.
	Text string
	// Line is the 1-based line number of the statement's first line.
	Line int
	// Indent is the leading whitespace of the first line, re-applied to
	// every replacement line.
	Indent string
	// Prompt is a console prompt ("$ ") stripped from the first line and
	// restored on rewrite.
	Prompt string

	start, end int // inclusive line-index range within the document
}

// Document is a parsed file that can locate and replace curl statements.
type Document struct {
	lines      []string
	trailingNL bool
	stmts      []Statement
}

// Kind selects the extraction strategy.
type Kind int

const (
	KindCommand Kind = iota // the whole input is one command
	KindMarkdown
	KindScript
)

// KindForPath picks an extraction strategy from a file extension.
func KindForPath(path string) Kind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdx":
		return KindMarkdown
	case ".sh", ".bash", ".zsh":
		return KindScript
	default:
		return KindCommand
	}
}

// New parses content with the given strategy.
func New(content string, kind Kind) *Document {
	d := &Document{trailingNL: strings.HasSuffix(content, "\n")}
	d.lines = strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	switch kind {
	case KindMarkdown:
		d.stmts = scanMarkdown(d.lines)
	case KindScript:
		d.stmts = scanScript(d.lines)
	default:
		d.stmts = scanCommand(d.lines)
	}
	return d
}

// Statements returns the curl statements found, in document order.
func (d *Document) Statements() []Statement { return d.stmts }

// Render rebuilds the document with statement i replaced by replacements[i]
// (a possibly multi-line formatted command without trailing newline). A nil
// entry keeps the original text.
func (d *Document) Render(replacements []string) string {
	var out []string
	next := 0
	for i := 0; i < len(d.lines); {
		if next < len(d.stmts) && d.stmts[next].start == i {
			s := d.stmts[next]
			out = append(out, s.Replacement(replacements[next])...)
			i = s.end + 1
			next++
			continue
		}
		out = append(out, d.lines[i])
		i++
	}
	res := strings.Join(out, "\n")
	if d.trailingNL {
		res += "\n"
	}
	return res
}

// Replacement expands a formatted command into physical lines with the
// statement's indent and prompt applied.
func (s Statement) Replacement(formatted string) []string {
	lines := strings.Split(formatted, "\n")
	out := make([]string, 0, len(lines))
	for i, ln := range lines {
		if i == 0 {
			out = append(out, s.Indent+s.Prompt+ln)
		} else {
			out = append(out, s.Indent+ln)
		}
	}
	return out
}

// Original returns the statement's physical lines as they appear in the
// document, for change detection.
func (s Statement) Original(d *Document) []string {
	return d.lines[s.start : s.end+1]
}

// startsCurl reports whether stripped begins a curl invocation.
func startsCurl(stripped string) bool {
	if !strings.HasPrefix(stripped, "curl") {
		return false
	}
	rest := stripped[len("curl"):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest == "\\"
}

// splitIndent returns the leading whitespace of line and the remainder.
func splitIndent(line string) (indent, rest string) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i], line[i:]
}

// gather joins lines starting at index i into one lexically complete
// statement. first is the already-stripped first line. It returns the
// statement text and the index of the last line consumed, or ok=false when
// the block ends before the statement completes.
func gather(lines []string, i int, first string, limit int) (text string, last int, ok bool) {
	text = first
	last = i
	for !shell.Complete(text) {
		last++
		if last >= limit {
			return "", 0, false
		}
		text += "\n" + lines[last]
	}
	return text, last, true
}

// scanCommand treats the whole input as one command (stdin mode). A leading
// "$ " prompt is tolerated.
func scanCommand(lines []string) []Statement {
	for i := 0; i < len(lines); i++ {
		indent, rest := splitIndent(lines[i])
		prompt := ""
		if strings.HasPrefix(rest, "$ ") {
			prompt, rest = "$ ", rest[2:]
		}
		if !startsCurl(rest) {
			if strings.TrimSpace(rest) == "" || strings.HasPrefix(rest, "#") {
				continue
			}
			return nil
		}
		text, last, ok := gather(lines, i, rest, len(lines))
		if !ok {
			return nil
		}
		return []Statement{{Text: text, Line: i + 1, Indent: indent, Prompt: prompt, start: i, end: last}}
	}
	return nil
}
