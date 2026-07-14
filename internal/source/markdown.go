package source

import "strings"

// shellLangs are the fenced-code info strings curlfmt treats as shell.
var shellLangs = map[string]bool{
	"":              true,
	"bash":          true,
	"sh":            true,
	"shell":         true,
	"zsh":           true,
	"console":       true,
	"shell-session": true,
	"shellsession":  true,
}

// scanMarkdown walks a Markdown document and extracts curl statements from
// fenced code blocks whose language is shell-like. Everything outside those
// blocks — prose, other languages, inline code — is never touched, and
// heredoc bodies inside a shell fence are skipped just like in scripts.
func scanMarkdown(lines []string) []Statement {
	var stmts []Statement
	inBlock := false
	blockShell := false
	var fenceChar byte
	fenceLen := 0
	blockEnd := 0 // exclusive index of the current block's closing fence
	heredoc := "" // pending heredoc delimiter within the current shell block

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		_, rest := splitIndent(line)

		if !inBlock {
			if ch, n, info, ok := openFence(rest); ok {
				inBlock, fenceChar, fenceLen = true, ch, n
				blockShell = shellLangs[strings.ToLower(info)]
				blockEnd = findClose(lines, i+1, fenceChar, fenceLen)
				heredoc = ""
			}
			continue
		}
		if closesFence(rest, fenceChar, fenceLen) {
			inBlock = false
			heredoc = ""
			continue
		}
		if !blockShell {
			continue
		}
		if heredoc != "" {
			if strings.TrimLeft(line, "\t") == heredoc {
				heredoc = ""
			}
			continue
		}
		indent, stripped := splitIndent(line)
		prompt := ""
		if strings.HasPrefix(stripped, "$ ") {
			prompt, stripped = "$ ", stripped[2:]
		}
		if !startsCurl(stripped) {
			if m := heredocRe.FindStringSubmatch(line); m != nil {
				heredoc = m[1]
			}
			continue
		}
		text, last, ok := gather(lines, i, stripped, blockEnd)
		if !ok {
			continue
		}
		stmts = append(stmts, Statement{
			Text: text, Line: i + 1, Indent: indent, Prompt: prompt,
			start: i, end: last,
		})
		if m := heredocRe.FindStringSubmatch(text); m != nil {
			heredoc = m[1]
		}
		i = last
	}
	return stmts
}

// openFence recognizes a fence opener (``` or ~~~, three or more) and
// returns the fence character, its length, and the info-string language.
func openFence(rest string) (ch byte, n int, info string, ok bool) {
	if len(rest) < 3 {
		return 0, 0, "", false
	}
	ch = rest[0]
	if ch != '`' && ch != '~' {
		return 0, 0, "", false
	}
	n = 0
	for n < len(rest) && rest[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	info = strings.TrimSpace(rest[n:])
	if f := strings.Fields(info); len(f) > 0 {
		info = f[0]
	}
	// An info string containing the fence character is not a fence opener.
	if strings.ContainsRune(info, rune(ch)) {
		return 0, 0, "", false
	}
	return ch, n, info, true
}

func closesFence(rest string, ch byte, minLen int) bool {
	n := 0
	for n < len(rest) && rest[n] == ch {
		n++
	}
	return n >= minLen && strings.TrimSpace(rest[n:]) == ""
}

// findClose locates the closing fence, returning its index (or the document
// end), so statement gathering never runs past the block.
func findClose(lines []string, from int, ch byte, minLen int) int {
	for i := from; i < len(lines); i++ {
		_, rest := splitIndent(lines[i])
		if closesFence(rest, ch, minLen) {
			return i
		}
	}
	return len(lines)
}
