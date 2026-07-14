package source

import (
	"regexp"
	"strings"
)

// heredocRe spots a heredoc redirection so its body is never mistaken for
// commands. Group 1 captures the (possibly quoted) delimiter.
var heredocRe = regexp.MustCompile("<<-?[ \t]*\\\\?['\"]?([A-Za-z_][A-Za-z0-9_]*)['\"]?")

// scanScript walks a shell script and extracts statements whose first word
// is curl. Comments, heredoc bodies, and everything else are left alone.
func scanScript(lines []string) []Statement {
	var stmts []Statement
	heredoc := "" // pending heredoc delimiter, "" when not inside one

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if heredoc != "" {
			if strings.TrimLeft(line, "\t") == heredoc {
				heredoc = ""
			}
			continue
		}
		indent, stripped := splitIndent(line)
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		if !startsCurl(stripped) {
			if m := heredocRe.FindStringSubmatch(line); m != nil {
				heredoc = m[1]
			}
			continue
		}
		text, last, ok := gather(lines, i, stripped, len(lines))
		if !ok {
			continue
		}
		stmts = append(stmts, Statement{
			Text: text, Line: i + 1, Indent: indent,
			start: i, end: last,
		})
		// A heredoc attached to the curl statement itself starts after it.
		if m := heredocRe.FindStringSubmatch(text); m != nil {
			heredoc = m[1]
		}
		i = last
	}
	return stmts
}
