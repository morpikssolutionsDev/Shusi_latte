#!/bin/sh
# Sushi Latte — whole repo in dev mode, from the repo root.
#
#   npm run dev   (or: sh scripts/dev.sh)
#
# Starts the API server on :8080 plus a Vite dev server for each
# frontend, all at once. Ctrl+C stops everything.
#
# Why this exists: running a frontend dev server alone gives
# "Request failed (404)" for API calls, because there is no API
# server behind it. The Vite dev servers proxy /api, /public
# and /auth to http://localhost:8080, so the API must be running
# too — this script starts it first.
#
# Open these (note the path — opening just the port gives a 404,
# because each app is served under its own base path):
#   API:          http://localhost:8080/api/v1/health
#   Client Menu:  http://localhost:5173/client-menu/
#   Staff:        http://localhost:5174/staff/
#   Display:      http://localhost:5175/display/
# The built versions are also served by the API itself at
# http://localhost:8080/client-menu/ etc. once you run
# `npm run build` (sh scripts/build-web.sh).
set -e
cd "$(dirname "$0")/.."
if ! command -v go >/dev/null 2>&1; then
  for d in /usr/local/go/bin "$HOME/go/bin"; do
    if [ -x "$d/go" ]; then PATH="$d:$PATH"; break; fi
  done
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.26+ not found. Install it from https://go.dev/dl/ and make sure 'go' is on your PATH." >&2
  exit 1
fi
cleanup() {
  echo "Stopping..."
  kill $API_PID $MENU_PID $STAFF_PID $DISPLAY_PID 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup INT TERM
echo "==> API server on :8080"
(cd api && go run ./cmd/server) &
API_PID=$!
echo "==> Client Menu dev on :5173"
(cd web/apps/client-menu && npm run dev -- --host) &
MENU_PID=$!
echo "==> Staff dev on :5174"
(cd web/apps/staff && npm run dev -- --host) &
STAFF_PID=$!
echo "==> Display dev on :5175"
(cd web/apps/display && npm run dev -- --host) &
DISPLAY_PID=$!
echo ""
echo "Open:"
echo "  Client Menu: http://localhost:5173/client-menu/"
echo "  Staff:       http://localhost:5174/staff/"
echo "  Display:     http://localhost:5175/display/"
echo "Press Ctrl+C to stop everything."
wait
