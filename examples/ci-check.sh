#!/usr/bin/env bash
# Example CI gate: fail the build when docs contain unformatted or
# lint-dirty curl commands. Usage: bash examples/ci-check.sh [path ...]
# (defaults to the repository's own examples). Requires curlfmt on PATH
# or built at ./curlfmt.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CURLFMT="${CURLFMT:-$ROOT/curlfmt}"
if ! [ -x "$CURLFMT" ]; then
  CURLFMT="$(command -v curlfmt)" || {
    echo "curlfmt not found; run: go build -o curlfmt ./cmd/curlfmt" >&2
    exit 3
  }
fi

TARGETS=("$@")
if [ "${#TARGETS[@]}" -eq 0 ]; then
  TARGETS=("$ROOT/examples/messy-api-doc.md")
fi

status=0

echo "== formatting check =="
"$CURLFMT" --check "${TARGETS[@]}" || status=1

echo "== lint =="
"$CURLFMT" lint "${TARGETS[@]}" || status=1

if [ "$status" -ne 0 ]; then
  echo "docs need attention: run 'curlfmt -w --fix <path>' and review the remaining findings" >&2
fi
exit "$status"
