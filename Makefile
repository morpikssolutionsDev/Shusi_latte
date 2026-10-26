# Sushi Latte — one-command local setup.
#
#   make run    Build the three frontends, then start the API server
#               on http://localhost:8080 (serves /client-menu/,
#               /staff/ and /display/). Note: go.mod lives in api/,
#               so the server is run from api/ (the server finds
#               web/apps/*/dist via ../web).
#   make test   Run all Go tests.
#   make build-web   Build the frontends only.
#   make build  Build the API binary to ./bin/sushi-latte.
#
# Requires Go 1.26+ and Node 20+ with npm. On Windows, run
# these commands from Git Bash, or use scripts/run.sh there.

.PHONY: run test build-web build

run: build-web
	cd api && go run ./cmd/server

test:
	cd api && go test ./...

build-web:
	@for app in client-menu staff display; do \
		echo "==> web/apps/$$app"; \
		(cd web/apps/$$app && npm install --no-audit --no-fund && npm run build); \
	done

build: build-web
	cd api && go build -o ../bin/sushi-latte ./cmd/server
