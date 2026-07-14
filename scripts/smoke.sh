#!/usr/bin/env bash
# End-to-end smoke test for curlfmt: builds the binary, then drives the
# real CLI over stdin, a Markdown document, and a shell script, asserting
# on actual output and exit codes. No network, idempotent, finishes in
# seconds.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

fail() {
  echo "SMOKE FAIL: $*" >&2
  exit 1
}

BIN="$WORKDIR/curlfmt"

echo "1. build"
(cd "$ROOT" && go build -o "$BIN" ./cmd/curlfmt) || fail "go build failed"

echo "2. version matches manifest"
"$BIN" --version | grep -qx "curlfmt 0.1.0" || fail "--version mismatch"

echo "3. stdin formatting canonicalizes a gnarly one-liner"
OUT="$(printf 'curl -sSLX POST http://127.0.0.1:8080/v1/items -H "content-type: application/json" -d @body.json\n' | "$BIN")"
echo "$OUT" | grep -q -- "--location --show-error --silent" || fail "boolean flags not sorted/expanded"
echo "$OUT" | grep -q -- "--request POST" || fail "-X not expanded"
echo "$OUT" | grep -q -- "--header 'Content-Type: application/json'" || fail "header not canonicalized"
echo "$OUT" | grep -q "http://127.0.0.1:8080/v1/items" || fail "URL missing"

echo "4. formatting is idempotent"
TWICE="$(echo "$OUT" | "$BIN")"
[ "$OUT" = "$TWICE" ] || fail "second format pass changed the output"

echo "5. Markdown rewrite touches only the curl block"
cat > "$WORKDIR/guide.md" <<'EOF'
# API guide

Fetch the users:

```bash
curl -s -X GET https://api.example.test/v1/users
```

```python
curl = "not a shell command"
```
EOF
"$BIN" --check "$WORKDIR/guide.md" >/dev/null && fail "--check should exit 1 before rewrite"
"$BIN" -w "$WORKDIR/guide.md" || fail "write failed"
grep -q -- "curl --silent --request GET https://api.example.test/v1/users" "$WORKDIR/guide.md" \
  || fail "markdown block not rewritten"
grep -q 'curl = "not a shell command"' "$WORKDIR/guide.md" || fail "python block was touched"
"$BIN" --check "$WORKDIR/guide.md" || fail "--check should pass after rewrite"

echo "6. shell script rewrite preserves the pipeline"
printf '#!/bin/sh\nset -eu\ncurl -s https://api.example.test/id | jq .id\n' > "$WORKDIR/get.sh"
"$BIN" -w "$WORKDIR/get.sh" || fail "script write failed"
grep -q -- "curl --silent https://api.example.test/id | jq .id" "$WORKDIR/get.sh" \
  || fail "pipeline suffix lost"

echo "7. lint reports findings with codes and exits 1"
set +e
LINT="$(printf 'curl -k -s http://api.example.test/v1\n' | "$BIN" lint)"
CODE=$?
set -e
[ "$CODE" -eq 1 ] || fail "lint should exit 1 on warnings (got $CODE)"
echo "$LINT" | grep -q "CF003 warning" || fail "CF003 (--insecure) missing"
echo "$LINT" | grep -q "CF005 warning" || fail "CF005 (plain http) missing"
echo "$LINT" | grep -q "CF006 warning" || fail "CF006 (silent w/o show-error) missing"

echo "8. lint --format json is machine-readable"
set +e
JSON="$(printf 'curl -k https://api.example.test\n' | "$BIN" lint --format json)"
set -e
echo "$JSON" | grep -q '"tool": "curlfmt"' || fail "json envelope missing"
echo "$JSON" | grep -q '"code": "CF003"' || fail "json finding missing"

echo "9. --fix applies safe rewrites"
FIXED="$(printf 'curl -s -X GET https://api.example.test\n' | "$BIN" --fix)"
[ "$FIXED" = "curl --show-error --silent https://api.example.test" ] \
  || fail "--fix output unexpected: $FIXED"

echo "10. usage errors exit 2"
set +e
"$BIN" --bogus >/dev/null 2>&1
[ $? -eq 2 ] || fail "unknown flag should exit 2"
printf 'wget https://example.test\n' | "$BIN" >/dev/null 2>&1
[ $? -eq 2 ] || fail "non-curl stdin should exit 2"
set -e

echo "SMOKE OK"
