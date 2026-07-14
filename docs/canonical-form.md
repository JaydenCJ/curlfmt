# The canonical form

curlfmt is opinionated the way gofmt is: there is exactly one canonical
rendering for any given curl command, so diffs stay about behavior, never
about style. This page specifies that rendering.

## Layout

A command whose canonical single-line form fits in `--width` columns
(default 80) stays on one line. Otherwise:

- Line 1 is `curl` followed by every **boolean flag**, expanded to its
  long name, sorted alphabetically, duplicates removed (curl's booleans
  are idempotent, so this is safe). A flag and its `--no-` negation are
  one flag: curl applies them left to right, so only the last spelling
  survives (`--silent --no-silent` → `--no-silent`), never both sorted
  side by side.
- Every **valued option** gets its own continuation line, indented two
  spaces, ending in ` \`.
- **URLs** come last, one per line, in original order. `--url VALUE` is
  rewritten to a positional URL.
- A trailing shell suffix (`| jq .`, `> out.json`, `&& …`) is appended
  after the final line, verbatim.

## Option order

Valued options are grouped; original order is preserved within a group:

| Order | Group | Options |
|---|---|---|
| 1 | method | `--request` |
| 2 | auth | `--user`, `--oauth2-bearer` |
| 3 | headers | `--header` |
| 4 | body | `--data*`, `--form*`, `--json` |
| 5 | everything else | timeouts, retries, proxy, TLS, cookies, … (and unknown options, verbatim) |
| 6 | output | `--output`, `--write-out`, `--dump-header`, `--trace*`, `--stderr` |

## Spelling and quoting

- Short options are expanded (`-sSL` → `--location --show-error
  --silent`; `-XPOST` → `--request POST`; `-d@f` → `--data @f`).
- `--opt=value` is split into `--opt value` — curl itself rejects the
  equals form, so this can only repair a command, never change a working
  one (lint CF018 reports it).
- Values are bare when every byte is shell-safe, otherwise single-quoted
  with the standard `'\''` escape. Glob characters, `&`, `?`, `~`, `{}`
  and friends always force quotes — this is how curlfmt un-breaks the
  classic unquoted-query-string bug.
- Known HTTP methods are uppercased (`-X delete` → `--request DELETE`);
  custom methods pass through untouched.
- Literal `Name: value` headers get canonical field casing and one space
  after the colon (`content-type:x` → `Content-Type: x`). The special
  forms `Name;` (send empty) and `Name:` (unset) are never rewritten.

## What is never changed

- **Live shell syntax.** Any word containing `$VAR`, `${…}`, `$(…)`, or
  backticks is re-emitted from its original source text, quoting
  included. curlfmt formats around your variables, not through them.
- **Unknown options.** Anything not in the spec table passes through
  byte-for-byte (and lint CF011 tells you about it).
- **Everything outside the command.** In Markdown and scripts, only the
  curl statements are replaced; every other byte is preserved.

## Idempotence

For every input, `format(format(x)) == format(x)`. The test suite pins
this property on gnarly inputs (multi-line bodies, expansions, suffixes,
unknown options), and CONTRIBUTING.md requires a case for every
formatter change.
