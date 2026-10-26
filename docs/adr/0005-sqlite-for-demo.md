---
status: accepted
---

# SQLite with an in-process event bus for the prototype and demo

Sushi Latte is built as a prototype and showcase that runs locally on one machine, so the database is a single SQLite file (WAL mode, pure-Go `modernc.org/sqlite` driver) instead of PostgreSQL. The Go API is already one process, so PostgreSQL's cross-process features are not needed: realtime fan-out moves from `LISTEN/NOTIFY` to an in-process event bus that publishes after each commit, and scheduled jobs run on an in-process scheduler without advisory locks. A file database also makes the demo reset trivial and removes Docker from the setup.

## Considered Options

- **PostgreSQL (original TRD)**: rejected for the demo; it adds a server, Docker and testcontainers for capacity the demo never uses.
- **Supabase / Vercel**: rejected; the demo runs locally, and the serverless model breaks the long-lived WebSocket hub.
- **SQLite** — chosen.

## Consequences

- One writer at a time. `BEGIN IMMEDIATE` replaces `SELECT … FOR UPDATE`; the Claim race stays a single conditional `UPDATE`. Capacity (~150 Orders/hour) is far below SQLite's limits.
- Only one API process may own the database file; horizontal scaling is not possible.
- Append-only `audit_log` is enforced by triggers that abort `UPDATE`/`DELETE`, since SQLite has no roles.
- Timestamps are UTC Unix milliseconds, JSON is text with SQLite JSON functions, UUIDv7 is generated in Go.
- sqlc queries and goose migrations are written in SQLite dialect. A move to PostgreSQL for a real deployment means re-targeting migrations and queries — a contained rewrite of the `db/` layer, not of the domain. PostgreSQL remains the documented production path.
- Crypto-shredding (ADR-0002) keeps its data keys in a separate key file outside the database file and its backups.
