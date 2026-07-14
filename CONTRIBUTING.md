# Contributing to curlfmt

Issues, discussions and pull requests are all welcome.

## Getting started

You need Go ≥1.22; nothing else — no runtime dependencies, no services.

```bash
git clone https://github.com/JaydenCJ/curlfmt && cd curlfmt
go build ./...
go test ./...
bash scripts/smoke.sh
```

`scripts/smoke.sh` builds the binary and drives the real CLI end to end —
stdin formatting, Markdown and shell-script rewriting, lint text/JSON
output, `--fix`, and every exit code; it must finish by printing `SMOKE OK`.

## Before you open a pull request

1. `gofmt -l .` reports nothing (formatting is enforced).
2. `go vet ./...` passes with no findings.
3. `go test ./...` passes (91 deterministic tests, no network).
4. `bash scripts/smoke.sh` prints `SMOKE OK`.
5. Add tests for behavior changes; keep logic in pure, unit-testable
   modules (`shell`, `parse`, `format`, `lint`, `source` never touch the
   filesystem — only `cli` does).

## Ground rules

- Keep dependencies at zero — curlfmt is standard library only, and that
  is a feature users rely on. Adding one needs strong justification.
- No network calls, ever. curlfmt formats curl commands; it never runs
  them. No telemetry.
- Formatting must stay semantics-preserving and idempotent: for any input,
  `format(format(x)) == format(x)`, and the emitted command must execute
  identically to the original. Every formatter change needs an idempotence
  case.
- New lint rules are data plus one function: a stable `CF0xx` code, a
  severity, a focused test, and a row in `docs/lint-rules.md`. Rules that
  rewrite must prove the rewrite safe before earning a `--fix`.
- New curl options go into the table in `internal/spec/spec.go` with the
  correct group and arity, plus a lookup test.
- Code comments and doc comments are written in English.

## Reporting bugs

Include the output of `curlfmt version`, the exact input command or
document (redact secrets — that is what CF004/CF012 are for), the output
you got, and the output you expected. For rewrite bugs, `curlfmt --check`
plus the original file in a gist is the fastest repro.

## Security

Please do not open public issues for security problems; use GitHub's
private vulnerability reporting on this repository instead.
