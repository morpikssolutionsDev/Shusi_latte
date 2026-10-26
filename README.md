# Sushi Latte

Single-outlet HORECA platform: QR Client Menu, Waiter, Kitchen,
Cashier, Product Display and HRIS. API in Go + SQLite (`api/`),
frontends in React + Vite + TypeScript (`web/apps/`).

## Run the whole repo

From the repo root, on branch `work/sushi-latte`:

```sh
# Dev mode: API on :8080 + Vite dev servers for all three apps.
npm run dev
# Production-style: build the frontends, then serve everything
# from the API server on :8080.
npm run build
go run .
```

Or, without npm:

```sh
sh scripts/dev.sh    # dev mode
sh scripts/run.sh    # build + serve from the API server
```

Requires Go 1.26+ and Node 20+ with npm.

## Open it

Dev mode (note the path — opening just the port gives a 404,
because each app lives under its own base path):

- Client Menu: http://localhost:5173/client-menu/
- Staff: http://localhost:5174/staff/
- Display: http://localhost:5175/display/
- API health: http://localhost:8080/api/v1/health

Built mode (after `npm run build` or `sh scripts/run.sh`):

- Client Menu: http://localhost:8080/client-menu/
- Staff: http://localhost:8080/staff/
- Display: http://localhost:8080/display/

## First user

There is no seeded login. The first crew record is created
without a login, and must be Management:

```sh
curl -X POST http://localhost:8080/api/v1/crew \
  -H 'Content-Type: application/json' \
  -d '{"crew_id":"MG01","full_name":"Manager","role":"management","stations":[],"pin":"123456"}'
```

Then sign in at `/staff/` with `MG01` / `123456`.

## Tests

```sh
npm test
# or: cd api && go test ./...
```

## Why "Request failed (404)" happens

Running a frontend dev server alone, without the API server,
makes every API call fail — the Vite dev servers only proxy
`/api`, `/public` and `/auth` to `http://localhost:8080`, they
do not serve the API themselves. `npm run dev` starts both,
so that failure goes away. Opening just `http://localhost:5173/`
also 404s: open the full path, for example
`http://localhost:5173/client-menu/`.
