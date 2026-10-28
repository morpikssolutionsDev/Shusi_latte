---
status: accepted
supersedes: 0005
---

# PostgreSQL replaces SQLite; the demo starts clean

Decided 2026-10-28 by the owner. The API, the seed and the tests use a local PostgreSQL database named by the `POSTGRE_URL` environment variable (`postgres://user:pass@localhost:5432/sushi_latte?sslmode=disable`). SQLite is removed. ADR-0005 noted that PostgreSQL was the production path and that moving meant re-targeting `db/` only; that is what was done. The domain packages are unchanged.

## Considered Options

- **Keep SQLite for tests and the offline demo, add PostgreSQL as an option**: rejected; two sqlc engines and two migration sets to keep in step.
- **Replace SQLite** — chosen.

## Consequences

- **Schemas, not files.** Each database lives in one schema of the `POSTGRE_URL` database: the running demo in `app`, a reset's build in `app_next`, each test in its own `test_…` schema dropped afterwards. A demo reset builds `app_next` and swaps it in with `ALTER SCHEMA … RENAME` inside one transaction (DEMO-AND-SEED §9).
- **Writes stay serial.** The write pool has one connection, so units of work run one at a time as the business rules were written for (first-writer-wins Claims, gapless Bill numbers, one open Bill per Table). This replaces SQLite's `BEGIN IMMEDIATE`. Reads use a separate pool with `default_transaction_read_only`.
- **Live pools prepare nothing server-side** (pgx exec mode), so table names resolve on every statement and a schema swap is picked up at once. Build pools (the seed) cache statements and turn `synchronous_commit` off, since a failed build is simply rebuilt.
- **Types**: money, timestamps (UTC Unix milliseconds) and 0/1 flags are `BIGINT`, so Go keeps `int64`; encrypted fields are `BYTEA`; sums are cast to `bigint`. Queries use `$1` placeholders. The append-only `audit_log` is enforced by a plpgsql trigger function. Unique violations are recognised by SQLSTATE `23505`, not by message text.
- **Setup**: a PostgreSQL server is now a prerequisite for `pnpm dev`, `pnpm seed`, `pnpm demo` and the Go tests; `POSTGRE_URL` must be set in the environment. Images and the crypto-shredding key file stay in `data/` (ADR-0002, TR-RET-04).
- **The demo starts clean.** `pnpm seed`, Admin → Pengaturan → Reset demo and the 03.00 nightly reset build the menu, Crew, Tables, discounts, Product Display and settings only. The scripted story (busy Tables, Takeaways, Cash Drawer, HRIS records, 14 days of history, W005 lockout) is loaded on purpose with `pnpm seed --story` or the "Muat cerita demo" option of Reset demo; the guided tours need it.
