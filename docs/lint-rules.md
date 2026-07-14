# Lint rules

Every rule has a stable code (safe to grep, safe to gate on), a severity,
and — where a rewrite is provably semantics-preserving — an automatic fix
applied by `curlfmt --fix`.

Severities: **error** (the command is broken or leaks secrets),
**warning** (a trap that will bite), **info** (advice; never fails the
exit code). `curlfmt lint` exits 1 when any warning or error is found.

| Code | Severity | Fix | Finding |
|---|---|---|---|
| CF001 | warning | ✅ | `--request GET` without a body — GET is curl's default |
| CF002 | warning | ✅ | `--request POST` with a body — the data option already implies POST |
| CF003 | warning | — | `--insecure` disables TLS certificate verification |
| CF004 | error/warning | — | credentials in the URL userinfo (error with a password, warning with a bare username) |
| CF005 | warning | — | plain-text `http://` to a non-loopback host |
| CF006 | warning | ✅ | `--silent` without `--show-error` hides curl's own error messages |
| CF007 | info | — | no `--fail`/`--fail-with-body`: HTTP 4xx/5xx still exit 0 (dangerous in CI) |
| CF008 | warning | ✅* | the same header field set more than once (*only byte-identical duplicates are removed) |
| CF009 | error | — | unquoted `&` in the query string backgrounds the command and drops the rest of the URL |
| CF010 | warning | — | `--request GET` with a body — use `--get` to move data into the query string |
| CF011 | warning | — | unknown option (passed through unchanged) |
| CF012 | info | — | `--user user:password` puts a secret in argv; prefer `.netrc` or prompting |
| CF013 | warning | — | `--head` combined with a request body |
| CF014 | error | — | no URL found |
| CF015 | warning | — | URL without a scheme; curl's guess differs across versions |
| CF016 | warning | — | `--json` combined with an explicit `Content-Type`/`Accept` header |
| CF017 | warning | — | a last-one-wins option (`--request`, `--user`, `--max-time`, …) given more than once |
| CF018 | warning | ✅ | `--opt=value` spelling, which curl rejects; split into `--opt value` |
| CF019 | error | — | an option is missing its value |

## Design notes

- **Static analysis only.** Words containing `$VAR` or command
  substitutions are skipped by the URL rules (CF004/CF005/CF015) rather
  than guessed at — no false positives on `"$BASE_URL/health"`.
- **Fixes are conservative.** A fix ships only when the rewrite cannot
  change what a working command does: dropping a method curl would pick
  anyway (CF001/CF002), adding `--show-error` to an already-silent
  command (CF006), collapsing byte-identical headers (CF008), and
  splitting an equals form curl would have rejected (CF018). Repeated
  headers with *different* values are reported but never merged — that
  can be intentional.
- **CF009 in practice.** `curl http://h/p?a=1&b=2` unquoted does not send
  `b=2` — the shell backgrounds curl at the `&`. curlfmt detects the
  split (query in the URL, suffix starting with `&`) and the formatter's
  quoting rules prevent reintroducing it.
- **Adding a rule.** One focused test per rule, a stable new code, a row
  here. Codes are never reused or renumbered.
