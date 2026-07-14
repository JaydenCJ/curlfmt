// Package cli wires the curlfmt pipeline — extract, parse, lint, fix,
// format, rewrite — behind a small gofmt-shaped command line.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/JaydenCJ/curlfmt/internal/format"
	"github.com/JaydenCJ/curlfmt/internal/lint"
	"github.com/JaydenCJ/curlfmt/internal/parse"
	"github.com/JaydenCJ/curlfmt/internal/shell"
	"github.com/JaydenCJ/curlfmt/internal/source"
	"github.com/JaydenCJ/curlfmt/internal/version"
)

const usage = `curlfmt — format, lint, and canonicalize curl commands

usage:
  curlfmt [flags] [path ...]        format stdin, files, or directories
  curlfmt lint [flags] [path ...]   report findings without rewriting
  curlfmt version                   print the version

format flags:
  -w, --write     rewrite files in place instead of printing
  -l, --list      print only the names of files that would change
      --check     like --list, but exit 1 when anything would change
      --fix       also apply safe lint fixes (CF001 CF002 CF006 CF008 CF018)
      --width N   single-line threshold in columns (default 80)

lint flags:
      --format F  output format: text (default) or json
      --fix       report which findings --fix would resolve

Paths may be Markdown files (.md .markdown .mdx), shell scripts
(.sh .bash .zsh), or directories to walk for both. With no path, stdin is
read as a single curl command.

exit codes: 0 ok · 1 findings / would reformat · 2 usage · 3 I/O error
`

// Run executes curlfmt and returns its exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-V":
			fmt.Fprintf(stdout, "curlfmt %s\n", version.Version)
			return 0
		case "help", "--help", "-h":
			fmt.Fprint(stdout, usage)
			return 0
		case "lint":
			return runLint(args[1:], stdin, stdout, stderr)
		case "fmt":
			args = args[1:]
		}
	}
	return runFmt(args, stdin, stdout, stderr)
}

type fmtOptions struct {
	write, list, check, fix bool
	width                   int
	paths                   []string
}

func parseFmtArgs(args []string, stdout, stderr io.Writer) (fmtOptions, int) {
	o := fmtOptions{width: format.DefaultWidth}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-w", "--write":
			o.write = true
		case "-l", "--list":
			o.list = true
		case "--check":
			o.check = true
		case "--fix":
			o.fix = true
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return o, -1
		case "--width":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "curlfmt: --width needs a value")
				return o, 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintf(stderr, "curlfmt: bad --width %q\n", args[i])
				return o, 2
			}
			o.width = n
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprintf(stderr, "curlfmt: unknown flag %s (see curlfmt --help)\n", a)
				return o, 2
			}
			o.paths = append(o.paths, a)
		}
	}
	return o, 0
}

func runFmt(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, code := parseFmtArgs(args, stdout, stderr)
	if code != 0 {
		if code < 0 {
			return 0
		}
		return code
	}

	if len(o.paths) == 0 {
		return fmtStdin(o, stdin, stdout, stderr)
	}
	files, err := expandPaths(o.paths)
	if err != nil {
		fmt.Fprintf(stderr, "curlfmt: %v\n", err)
		return 3
	}
	changedAny := false
	for _, path := range files {
		changed, code := fmtFile(path, o, stdout, stderr)
		if code != 0 {
			return code
		}
		changedAny = changedAny || changed
	}
	if o.check && changedAny {
		return 1
	}
	return 0
}

func fmtStdin(o fmtOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	if o.write || o.list {
		fmt.Fprintln(stderr, "curlfmt: cannot use -w or -l with standard input; pass a file path")
		return 2
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "curlfmt: reading stdin: %v\n", err)
		return 3
	}
	content := string(data)
	doc := source.New(content, source.KindCommand)
	if len(doc.Statements()) == 0 {
		fmt.Fprintln(stderr, "curlfmt: stdin does not contain a curl command")
		return 2
	}
	out, changed, err := rewrite(doc, o)
	if err != nil {
		fmt.Fprintf(stderr, "curlfmt: %v\n", err)
		return 2
	}
	if o.check {
		if changed {
			fmt.Fprintln(stdout, "<stdin>: needs formatting")
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, out)
	if !strings.HasSuffix(out, "\n") {
		fmt.Fprintln(stdout)
	}
	return 0
}

func fmtFile(path string, o fmtOptions, stdout, stderr io.Writer) (changed bool, code int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "curlfmt: %v\n", err)
		return false, 3
	}
	doc := source.New(string(data), source.KindForPath(path))
	out, changed, err := rewrite(doc, o)
	if err != nil {
		fmt.Fprintf(stderr, "curlfmt: %s: %v\n", path, err)
		return false, 2
	}
	switch {
	case o.check:
		if changed {
			fmt.Fprintf(stdout, "%s: needs formatting\n", path)
		}
	case o.list:
		if changed {
			fmt.Fprintln(stdout, path)
		}
	case o.write:
		if changed {
			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(stderr, "curlfmt: %v\n", err)
				return false, 3
			}
			if err := os.WriteFile(path, []byte(out), info.Mode().Perm()); err != nil {
				fmt.Fprintf(stderr, "curlfmt: %v\n", err)
				return false, 3
			}
		}
	default:
		fmt.Fprint(stdout, out)
	}
	return changed, 0
}

// rewrite formats every statement in doc and reports whether anything
// changed.
func rewrite(doc *source.Document, o fmtOptions) (string, bool, error) {
	stmts := doc.Statements()
	replacements := make([]string, len(stmts))
	changed := false
	for i, s := range stmts {
		cmd, err := parseStatement(s.Text)
		if err != nil {
			return "", false, err
		}
		if o.fix {
			lint.Fix(cmd)
		}
		replacements[i] = format.Format(cmd, format.Options{Width: o.width})
		if !equalLines(s.Replacement(replacements[i]), s.Original(doc)) {
			changed = true
		}
	}
	return doc.Render(replacements), changed, nil
}

func parseStatement(text string) (*parse.Command, error) {
	res, err := shell.Lex(text)
	if err != nil {
		return nil, err
	}
	return parse.Parse(res.Words, res.Suffix)
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// jsonFinding is the stable machine-readable lint record.
type jsonFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fixable  bool   `json:"fixable"`
}

func runLint(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	outFormat := "text"
	showFix := false
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "curlfmt: --format needs a value")
				return 2
			}
			i++
			outFormat = args[i]
			if outFormat != "text" && outFormat != "json" {
				fmt.Fprintf(stderr, "curlfmt: bad --format %q (want text or json)\n", outFormat)
				return 2
			}
		case "--fix":
			showFix = true
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprintf(stderr, "curlfmt: unknown flag %s (see curlfmt --help)\n", a)
				return 2
			}
			paths = append(paths, a)
		}
	}

	var records []jsonFinding
	collect := func(name string, doc *source.Document) error {
		for _, s := range doc.Statements() {
			cmd, err := parseStatement(s.Text)
			if err != nil {
				return err
			}
			for _, f := range lint.Check(cmd) {
				records = append(records, jsonFinding{
					File: name, Line: s.Line, Code: f.Code,
					Severity: f.Severity.String(), Message: f.Message,
					Fixable: f.Fixable,
				})
			}
		}
		return nil
	}

	if len(paths) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "curlfmt: reading stdin: %v\n", err)
			return 3
		}
		doc := source.New(string(data), source.KindCommand)
		if len(doc.Statements()) == 0 {
			fmt.Fprintln(stderr, "curlfmt: stdin does not contain a curl command")
			return 2
		}
		if err := collect("<stdin>", doc); err != nil {
			fmt.Fprintf(stderr, "curlfmt: %v\n", err)
			return 2
		}
	} else {
		files, err := expandPaths(paths)
		if err != nil {
			fmt.Fprintf(stderr, "curlfmt: %v\n", err)
			return 3
		}
		for _, path := range files {
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(stderr, "curlfmt: %v\n", err)
				return 3
			}
			doc := source.New(string(data), source.KindForPath(path))
			if err := collect(path, doc); err != nil {
				fmt.Fprintf(stderr, "curlfmt: %s: %v\n", path, err)
				return 2
			}
		}
	}

	worst := lint.Info
	for _, r := range records {
		if r.Severity == "error" {
			worst = lint.Error
		} else if r.Severity == "warning" && worst != lint.Error {
			worst = lint.Warning
		}
	}

	if outFormat == "json" {
		env := struct {
			Tool     string        `json:"tool"`
			Version  string        `json:"version"`
			Findings []jsonFinding `json:"findings"`
		}{"curlfmt", version.Version, records}
		if env.Findings == nil {
			env.Findings = []jsonFinding{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(env)
	} else {
		for _, r := range records {
			suffix := ""
			if showFix && r.Fixable {
				suffix = "  [--fix]"
			}
			fmt.Fprintf(stdout, "%s:%d: %s %s: %s%s\n",
				r.File, r.Line, r.Code, r.Severity, r.Message, suffix)
		}
		if len(records) == 0 {
			fmt.Fprintln(stdout, "no findings")
		}
	}

	if worst >= lint.Warning {
		return 1
	}
	return 0
}

// expandPaths resolves files and directories into a sorted, deduplicated
// file list. Directories are walked for Markdown and shell files.
func expandPaths(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	addFile := func(p string) {
		if !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			addFile(p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if source.KindForPath(path) != source.KindCommand {
				addFile(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}
