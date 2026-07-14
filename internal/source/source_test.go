// Source-extraction tests: Markdown fences, console prompts, shell
// scripts, heredocs, and the byte-preservation guarantee for everything
// that is not a curl statement.
package source

import (
	"strings"
	"testing"
)

func TestKindForPath(t *testing.T) {
	cases := map[string]Kind{
		"README.md": KindMarkdown, "guide.markdown": KindMarkdown,
		"page.mdx": KindMarkdown, "deploy.sh": KindScript,
		"env.bash": KindScript, "x.zsh": KindScript,
		"snippet.txt": KindCommand, "Makefile": KindCommand,
	}
	for path, want := range cases {
		if got := KindForPath(path); got != want {
			t.Fatalf("KindForPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestMarkdownFindsEveryCurlInShellFences(t *testing.T) {
	md := "intro\n\n```bash\ncurl -s https://a.example.test\ncurl -s https://b.example.test\n```\n"
	doc := New(md, KindMarkdown)
	stmts := doc.Statements()
	if len(stmts) != 2 {
		t.Fatalf("stmts = %+v", stmts)
	}
	if stmts[0].Text != "curl -s https://a.example.test" || stmts[0].Line != 4 {
		t.Fatalf("first = %+v", stmts[0])
	}
	if stmts[1].Line != 5 {
		t.Fatalf("second = %+v", stmts[1])
	}
}

func TestMarkdownIgnoresNonShellContexts(t *testing.T) {
	// Other languages' fences and plain prose must never match.
	md := "Run curl -s https://example.test to fetch.\n\n```python\ncurl = 'not a command'\n```\n"
	doc := New(md, KindMarkdown)
	if len(doc.Statements()) != 0 {
		t.Fatalf("non-shell contexts matched: %+v", doc.Statements())
	}
}

func TestMarkdownConsolePromptStrippedAndRestored(t *testing.T) {
	doc := New("```console\n$ curl -s https://example.test\nok\n```\n", KindMarkdown)
	stmts := doc.Statements()
	if len(stmts) != 1 || stmts[0].Prompt != "$ " || stmts[0].Text != "curl -s https://example.test" {
		t.Fatalf("stmts = %+v", stmts)
	}
	out := doc.Render([]string{"curl --silent https://example.test"})
	if !strings.Contains(out, "$ curl --silent https://example.test\nok\n") {
		t.Fatalf("render:\n%s", out)
	}
}

func TestMarkdownGathersMultiLineStatements(t *testing.T) {
	// Backslash continuations and quoted multi-line bodies both span
	// physical lines; the gatherer must consume exactly the right range.
	md := "```sh\ncurl -s \\\n  -H 'A: b' \\\n  https://example.test\necho done\n```\n"
	doc := New(md, KindMarkdown)
	stmts := doc.Statements()
	if len(stmts) != 1 || !strings.Contains(stmts[0].Text, "-H 'A: b'") {
		t.Fatalf("stmts = %+v", stmts)
	}
	if stmts[0].end-stmts[0].start != 2 {
		t.Fatalf("statement must span 3 lines: %+v", stmts[0])
	}

	md = "```bash\ncurl -d '{\n  \"a\": 1\n}' https://example.test\n```\n"
	stmts = New(md, KindMarkdown).Statements()
	if len(stmts) != 1 || !strings.Contains(stmts[0].Text, "\"a\": 1") {
		t.Fatalf("quoted body: %+v", stmts)
	}
}

func TestMarkdownStatementNeverCrossesClosingFence(t *testing.T) {
	// An unterminated quote inside a block must not swallow the fence and
	// the prose after it.
	md := "```bash\ncurl 'broken\n```\nprose after\n"
	doc := New(md, KindMarkdown)
	if len(doc.Statements()) != 0 {
		t.Fatalf("incomplete statement must be abandoned: %+v", doc.Statements())
	}
}

func TestMarkdownIndentedTildeFenceRoundTrip(t *testing.T) {
	md := "1. step\n\n   ~~~shell\n   curl -s -H 'A: b' https://example.test\n   ~~~\n"
	doc := New(md, KindMarkdown)
	stmts := doc.Statements()
	if len(stmts) != 1 || stmts[0].Indent != "   " {
		t.Fatalf("stmts = %+v", stmts)
	}
	out := doc.Render([]string{"curl --silent \\\n  --header 'A: b' \\\n  https://example.test"})
	want := "1. step\n\n   ~~~shell\n   curl --silent \\\n     --header 'A: b' \\\n     https://example.test\n   ~~~\n"
	if out != want {
		t.Fatalf("render:\n%s\nwant:\n%s", out, want)
	}
}

func TestMarkdownRenderPreservesEveryOtherByte(t *testing.T) {
	md := "# Title\n\ntext `curl inline` text\n\n```bash\ncurl -s https://example.test\n```\n\n```js\nfetch('/x')\n```\n"
	doc := New(md, KindMarkdown)
	out := doc.Render([]string{"curl -s https://example.test"}) // unchanged replacement
	if out != md {
		t.Fatalf("byte drift:\n%q\nvs\n%q", out, md)
	}
}

func TestMarkdownSkipsHeredocBodiesInsideShellFences(t *testing.T) {
	// A script pasted into a Markdown fence carries its heredocs along;
	// their bodies must not be mistaken for curl statements there either.
	md := "```sh\ncat <<'SCRIPT'\ncurl -s https://not-a-command.example.test\nSCRIPT\ncurl -s https://real.example.test\n```\n"
	doc := New(md, KindMarkdown)
	stmts := doc.Statements()
	if len(stmts) != 1 || !strings.Contains(stmts[0].Text, "real.example.test") {
		t.Fatalf("stmts = %+v", stmts)
	}
}

func TestScriptFindsStatementsOutsideComments(t *testing.T) {
	sh := "#!/bin/sh\n# curl -s https://commented.example.test\nset -eu\nif ok; then\n  curl -s https://real.example.test\nfi\n"
	doc := New(sh, KindScript)
	stmts := doc.Statements()
	if len(stmts) != 1 || stmts[0].Indent != "  " || stmts[0].Line != 5 {
		t.Fatalf("stmts = %+v", stmts)
	}
	if !strings.Contains(stmts[0].Text, "real.example.test") {
		t.Fatalf("text = %q", stmts[0].Text)
	}
}

func TestScriptSkipsHeredocBodies(t *testing.T) {
	// Both a foreign heredoc and one attached to the curl statement
	// itself: their bodies must never be scanned for commands.
	sh := "cat <<EOF\ncurl -s https://not-a-command.example.test\nEOF\ncurl -d @- https://real.example.test <<'BODY'\ncurl inside body\nBODY\n"
	doc := New(sh, KindScript)
	stmts := doc.Statements()
	if len(stmts) != 1 || !strings.Contains(stmts[0].Text, "real.example.test") {
		t.Fatalf("stmts = %+v", stmts)
	}
}

func TestScriptContinuationsAndPipelines(t *testing.T) {
	sh := "curl -s \\\n  --retry 3 \\\n  https://example.test | jq '.name'\necho after\n"
	doc := New(sh, KindScript)
	stmts := doc.Statements()
	if len(stmts) != 1 || stmts[0].end != 2 {
		t.Fatalf("stmts = %+v", stmts)
	}
	if !strings.Contains(stmts[0].Text, "| jq '.name'") {
		t.Fatalf("text = %q", stmts[0].Text)
	}
}

func TestCommandKindWholeInputIsOneStatement(t *testing.T) {
	doc := New("curl -s \\\n  https://example.test\n", KindCommand)
	if len(doc.Statements()) != 1 {
		t.Fatalf("stmts = %+v", doc.Statements())
	}
	// A leading prompt and blank lines are tolerated (pasted from docs).
	doc = New("\n$ curl -s https://example.test\n", KindCommand)
	stmts := doc.Statements()
	if len(stmts) != 1 || stmts[0].Prompt != "$ " {
		t.Fatalf("stmts = %+v", stmts)
	}
}

func TestCommandKindRejectsNonCurl(t *testing.T) {
	doc := New("wget https://example.test\n", KindCommand)
	if len(doc.Statements()) != 0 {
		t.Fatal("non-curl input must yield no statements")
	}
}

func TestRenderPreservesMissingTrailingNewline(t *testing.T) {
	doc := New("curl -s https://example.test", KindCommand)
	out := doc.Render([]string{"curl --silent https://example.test"})
	if out != "curl --silent https://example.test" {
		t.Fatalf("out = %q", out)
	}
}
