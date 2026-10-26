-- +goose Up
-- Crew records: BR-ADM-09, TRD §4.1, BR-R-24, TR-AUTH-01, TR-AUTH-02.
-- Crew ID is the human-readable sign-in ID (unique). The internal id
-- is an opaque TEXT key. PINs are never stored in plain text; the
-- pin_hash column holds an Argon2id hash (see internal/crew/pin.go).
-- failed_attempts and locked_until implement the 15-minute lockout
-- after 5 consecutive failed attempts. Session rows are defined here
-- for the auth phase; login and session handling arrive then, not
-- in this step.

CREATE TABLE IF NOT EXISTS crew (
    id TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('management', 'outlet_admin', 'crew')),
    image_key TEXT,
    pin_hash TEXT NOT NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
    locked_until INTEGER,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_crew_role ON crew(role);
CREATE INDEX IF NOT EXISTS idx_crew_active ON crew(active);

CREATE TABLE IF NOT EXISTS crew_station (
    crew_id TEXT NOT NULL REFERENCES crew(id),
    station TEXT NOT NULL CHECK (station IN ('waiter', 'cashier', 'kitchen')),
    PRIMARY KEY (crew_id, station)
);

CREATE TABLE IF NOT EXISTS session (
    id TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL REFERENCES crew(id),
    device_label TEXT,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_crew ON session(crew_id);
CREATE INDEX IF NOT EXISTS idx_session_expires ON session(expires_at);

-- +goose Down
DROP TABLE IF EXISTS session;
DROP TABLE IF EXISTS crew_station;
DROP TABLE IF EXISTS crew;
