# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-07-13

### Added

- POSIX-flavored shell lexer for curl statements: single/double quoting,
  escapes, backslash-newline continuations, comments, `$VAR` / `${…}` /
  `$(…)` / backtick expansions preserved verbatim, and command-terminating
  operators (`|`, `;`, `&`, redirections) carried through as a raw suffix.
- Curated spec table of 100+ curl options (short/long spellings,
  value arity, `--no-<flag>` negation, last-one-wins annotations); unknown
  options pass through untouched.
- Canonical formatter: long option names, sorted deduplicated boolean
  flags (negation-aware: of `--silent --no-silent` only the last
  spelling survives), one option per continuation line, deterministic grouping
  (method → auth → headers → body → other → output → URLs), canonical
  quoting, header-name casing, HTTP-method uppercasing, `--width`
  single-line threshold — idempotent by construction.
- Lint engine with 19 stable CF-coded rules covering redundant methods,
  `--insecure`, credentials in URLs and argv, plain-text `http://`,
  `--silent` without `--show-error`, missing `--fail`, duplicate headers,
  unquoted `&` splitting query strings, `--json` header conflicts,
  repeated last-one-wins options, and parser-level mistakes.
- Safe auto-fixes via `--fix` (CF001 CF002 CF006 CF008 CF018), applied
  only where the rewrite is provably semantics-preserving.
- Source rewriting for Markdown fenced code blocks (bash/sh/shell/console,
  ``` and ~~~ fences, indented blocks, `$ ` prompts) and shell scripts
  (comments and heredoc bodies skipped, pipelines preserved) — every byte
  outside a curl statement is left untouched.
- gofmt-shaped CLI: stdin filter, `-w` in-place rewrite, `-l` list,
  `--check` for CI, `lint` with text and JSON output, directory walking,
  exit codes 0/1/2/3.
- Runnable examples (`examples/messy-api-doc.md`, `examples/deploy.sh`,
  `examples/ci-check.sh`) and reference docs (`docs/canonical-form.md`,
  `docs/lint-rules.md`).
- 91 deterministic offline tests (unit + in-process CLI integration) and
  `scripts/smoke.sh`.

[0.1.0]: https://github.com/JaydenCJ/curlfmt/releases/tag/v0.1.0
