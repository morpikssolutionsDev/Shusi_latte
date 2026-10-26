#!/bin/sh
# Build all three frontends (client-menu, staff, display).
# Usage: sh scripts/build-web.sh   (from the repo root)
set -e
cd "$(dirname "$0")/.."
for app in client-menu staff display; do
  echo "==> web/apps/$app"
  (cd "web/apps/$app" && npm install --no-audit --no-fund && npm run build)
done
