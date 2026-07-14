#!/bin/sh
# Example deployment script with typical inline curl calls. Run
# `curlfmt -w examples/deploy.sh` (on a copy) to see the statements
# canonicalized while the surrounding script survives byte-for-byte.
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"

# Health gate before deploying — note the expansion: curlfmt keeps
# "$BASE_URL" double-quoted and never judges it statically.
curl -fsS "$BASE_URL/healthz" > /dev/null

# Upload the release manifest (short flags, attached value).
curl -sSX PUT -H 'content-type: application/yaml' -T manifest.yaml "$BASE_URL/v1/releases/current"

# Tell the notifier, piping the response id into the log.
curl -s -d 'event=deployed' -d 'env=staging' "$BASE_URL/v1/notify" | tee -a deploy.log

# Heredoc bodies are recognized and left alone:
cat <<EOF
curl inside this heredoc is documentation, not a command.
EOF
