// CLI integration tests: the whole pipeline driven in-process through
// Run, against real files in temp dirs — stdin, write-back, check mode,
// directory walking, lint text/JSON, and every exit code.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JaydenCJ/curlfmt/internal/version"
)

func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errBuf strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVersionAndHelp(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-V"} {
		code, out, _ := run(t, "", arg)
		if code != 0 || out != "curlfmt "+version.Version+"\n" {
			t.Fatalf("%s: code=%d out=%q", arg, code, out)
		}
	}
	code, out, _ := run(t, "", "--help")
	if code != 0 || !strings.Contains(out, "usage:") {
		t.Fatalf("help: code=%d out=%q", code, out)
	}
}

func TestStdinFormatsCommand(t *testing.T) {
	code, out, _ := run(t, "curl -s https://example.test\n")
	if code != 0 || out != "curl --silent https://example.test\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
	// The explicit fmt subcommand is an alias for the default.
	code, out, _ = run(t, "curl -s https://example.test\n", "fmt")
	if code != 0 || out != "curl --silent https://example.test\n" {
		t.Fatalf("fmt alias: code=%d out=%q", code, out)
	}
}

func TestStdinCheckExitCodes(t *testing.T) {
	code, out, _ := run(t, "curl -s https://example.test\n", "--check")
	if code != 1 || !strings.Contains(out, "<stdin>: needs formatting") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	code, _, _ = run(t, "curl --silent https://example.test\n", "--check")
	if code != 0 {
		t.Fatalf("canonical input: code=%d", code)
	}
}

func TestStdinRejectsWriteAndList(t *testing.T) {
	// gofmt parity: -w / -l make no sense without a file, and silently
	// ignoring them would hide a broken CI invocation.
	for _, flag := range []string{"-w", "--write", "-l", "--list"} {
		code, _, errOut := run(t, "curl -s https://example.test\n", flag)
		if code != 2 || !strings.Contains(errOut, "standard input") {
			t.Fatalf("%s: code=%d err=%q", flag, code, errOut)
		}
	}
}

func TestStdinFixAppliesSafeRewrites(t *testing.T) {
	code, out, _ := run(t, "curl -s -X GET https://example.test\n", "--fix")
	if code != 0 || out != "curl --show-error --silent https://example.test\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestWriteRewritesMarkdownInPlaceIdempotently(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "guide.md",
		"# API\n\n```bash\ncurl -s -X GET https://api.example.test/v1/users\n```\n")
	code, _, errOut := run(t, "", "-w", path)
	if code != 0 {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	got, _ := os.ReadFile(path)
	want := "# API\n\n```bash\ncurl --silent --request GET https://api.example.test/v1/users\n```\n"
	if string(got) != want {
		t.Fatalf("file:\n%s", got)
	}
	run(t, "", "-w", path)
	second, _ := os.ReadFile(path)
	if string(second) != want {
		t.Fatal("second write changed the file")
	}
}

func TestListAndCheckReportOnlyChangedFiles(t *testing.T) {
	dir := t.TempDir()
	messy := writeFile(t, dir, "messy.md", "```bash\ncurl -s https://example.test\n```\n")
	clean := writeFile(t, dir, "clean.md", "```bash\ncurl --silent https://example.test\n```\n")
	code, out, _ := run(t, "", "-l", messy, clean)
	if code != 0 || !strings.Contains(out, "messy.md") || strings.Contains(out, "clean.md") {
		t.Fatalf("list: code=%d out=%q", code, out)
	}
	code, out, _ = run(t, "", "--check", messy, clean)
	if code != 1 || !strings.Contains(out, "messy.md: needs formatting") {
		t.Fatalf("check: code=%d out=%q", code, out)
	}
	// Neither mode touches the files.
	got, _ := os.ReadFile(messy)
	if !strings.Contains(string(got), "curl -s ") {
		t.Fatal("--check/-l must not rewrite")
	}
}

func TestDirectoryWalkPicksUpMarkdownAndScripts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", "```bash\ncurl -s https://example.test\n```\n")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, sub, "b.sh", "curl -s https://example.test\n")
	writeFile(t, dir, "c.txt", "curl -s https://example.test\n") // not walked
	code, out, _ := run(t, "", "-l", dir)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "b.sh") || strings.Contains(out, "c.txt") {
		t.Fatalf("out=%q", out)
	}
}

func TestScriptRewritePreservesSurroundingCode(t *testing.T) {
	dir := t.TempDir()
	sh := "#!/bin/sh\nset -eu\ncurl -s https://example.test | jq .id\necho done\n"
	path := writeFile(t, dir, "deploy.sh", sh)
	code, _, _ := run(t, "", "-w", path)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got, _ := os.ReadFile(path)
	want := "#!/bin/sh\nset -eu\ncurl --silent https://example.test | jq .id\necho done\n"
	if string(got) != want {
		t.Fatalf("file:\n%s", got)
	}
}

func TestLintTextOutputExitCodeAndFixMarkers(t *testing.T) {
	code, out, _ := run(t, "curl -k -s https://example.test\n", "lint", "--fix")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "CF003 warning") || !strings.Contains(out, "CF006 warning") {
		t.Fatalf("out=%q", out)
	}
	if !strings.Contains(out, "<stdin>:1:") {
		t.Fatalf("out=%q", out)
	}
	// CF006 is fixable and must carry the [--fix] marker; CF003 is not.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "CF006") && !strings.Contains(line, "[--fix]") {
			t.Fatalf("CF006 missing marker: %q", line)
		}
		if strings.Contains(line, "CF003") && strings.Contains(line, "[--fix]") {
			t.Fatalf("CF003 must not be fixable: %q", line)
		}
	}
}

func TestLintSeverityGating(t *testing.T) {
	// CF007 (missing --fail) is advice, not a gate.
	code, out, _ := run(t, "curl -sS https://example.test\n", "lint")
	if code != 0 || !strings.Contains(out, "CF007 info") {
		t.Fatalf("info-only: code=%d out=%q", code, out)
	}
	code, out, _ = run(t, "curl --fail --show-error --silent https://example.test\n", "lint")
	if code != 0 || !strings.Contains(out, "no findings") {
		t.Fatalf("clean: code=%d out=%q", code, out)
	}
}

func TestLintJSONIsWellFormedAndStable(t *testing.T) {
	code, out, _ := run(t, "curl -k https://example.test\n", "lint", "--format", "json")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	var env struct {
		Tool     string `json:"tool"`
		Version  string `json:"version"`
		Findings []struct {
			File, Code, Severity, Message string
			Line                          int
			Fixable                       bool
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if env.Tool != "curlfmt" || env.Version != version.Version {
		t.Fatalf("envelope = %+v", env)
	}
	found := false
	for _, f := range env.Findings {
		if f.Code == "CF003" && f.Severity == "warning" && f.Line == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("CF003 missing: %+v", env.Findings)
	}
}

func TestLintFileReportsRealLineNumbers(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "api.md",
		"# API\n\nfetch:\n\n```bash\ncurl -k https://api.example.test\n```\n")
	code, out, _ := run(t, "", "lint", path)
	if code != 1 || !strings.Contains(out, path+":6: CF003") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestErrorExitCodes(t *testing.T) {
	// Usage errors exit 2 with a pointer to --help.
	code, _, errOut := run(t, "", "--bogus")
	if code != 2 || !strings.Contains(errOut, "--bogus") {
		t.Fatalf("unknown flag: code=%d err=%q", code, errOut)
	}
	if code, _, _ := run(t, "", "lint", "--bogus"); code != 2 {
		t.Fatalf("lint unknown flag: code=%d", code)
	}
	if code, _, _ := run(t, "", "--width", "zero"); code != 2 {
		t.Fatalf("bad width: code=%d", code)
	}
	if code, _, _ := run(t, "", "--width"); code != 2 {
		t.Fatalf("missing width: code=%d", code)
	}
	code, _, errOut = run(t, "wget https://example.test\n")
	if code != 2 || !strings.Contains(errOut, "curl command") {
		t.Fatalf("non-curl stdin: code=%d err=%q", code, errOut)
	}
	// I/O problems exit 3.
	code, _, errOut = run(t, "", filepath.Join(t.TempDir(), "absent.md"))
	if code != 3 || errOut == "" {
		t.Fatalf("missing file: code=%d err=%q", code, errOut)
	}
}

func TestWidthFlagChangesLayout(t *testing.T) {
	in := "curl -s -H 'A: b' https://example.test\n"
	_, wide, _ := run(t, in, "--width", "200")
	if strings.Contains(wide, "\\") {
		t.Fatalf("wide output should be single line: %q", wide)
	}
	_, narrow, _ := run(t, in, "--width", "20")
	if !strings.Contains(narrow, " \\\n") {
		t.Fatalf("narrow output should wrap: %q", narrow)
	}
}

func TestDefaultPrintRewritesWholeDocumentToStdout(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "g.md", "before\n\n```bash\ncurl -s https://example.test\n```\n")
	code, out, _ := run(t, "", path)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "before\n") || !strings.Contains(out, "curl --silent https://example.test") {
		t.Fatalf("out=%q", out)
	}
}
