#!/bin/sh
# Sushi Latte — one-command local run.
# Builds the three frontends, then starts the API server on
# http://localhost:8080 (serves /client-menu/, /staff/ and
# /display/). Requires Go 1.26+ and Node 20+ with npm.
# Usage: sh scripts/run.sh   (from the repo root)
set -e
cd "$(dirname "$0")/.."
# Go is often installed outside PATH on fresh machines; check the
# usual locations before giving up with a clear message.
if ! command -v go >/dev/null 2>&1; then
  for d in /usr/local/go/bin "$HOME/go/bin" "$HOME/sdk/go/bin"; do
    if [ -x "$d/go" ]; then PATH="$d:$PATH"; break; fi
  done
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.26+ not found. Install it from https://go.dev/dl/ and make sure 'go' is on your PATH, then run this script again." >&2
  exit 1
fi
sh scripts/build-web.sh
# go.mod lives in api/, so run the server from there.
# The server finds web/apps/*/dist via ../web.
cd api
exec go run ./cmd/server
