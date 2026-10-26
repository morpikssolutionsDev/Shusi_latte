-- Core platform tables: Crew and sign-in, PIN Overrides, audit log, ops settings, idempotency.
-- Conventions (TRD §4): ids are UUIDv7 TEXT, timestamps are UTC Unix milliseconds, money is INTEGER Rupiah.

-- +goose Up
CREATE TABLE crew (
    id              TEXT PRIMARY KEY,
    crew_id         TEXT NOT NULL UNIQUE,
    full_name       TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('management', 'outlet_admin', 'crew')),
    image_key       TEXT,
    pin_hash        TEXT NOT NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until    INTEGER,
    active          INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE crew_station (
    crew_id TEXT NOT NULL REFERENCES crew (id) ON DELETE CASCADE,
    station TEXT NOT NULL CHECK (station IN ('waiter', 'cashier', 'kitchen')),
    PRIMARY KEY (crew_id, station)
);

-- id is the SHA-256 of the session token; the token itself is only ever in the cookie.
CREATE TABLE session (
    id              TEXT PRIMARY KEY,
    crew_id         TEXT NOT NULL REFERENCES crew (id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('crew', 'screen')),
    screen          TEXT CHECK (screen IN ('kitchen', 'bar', 'display', 'hris')),
    device_label    TEXT,
    created_at      INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    expires_at      INTEGER NOT NULL,
    idle_timeout_ms INTEGER
);
CREATE INDEX session_expires_at ON session (expires_at);

-- One-time PIN Override tokens (TR-AUTH-06, TR-API-03). id is the SHA-256 of the token.
CREATE TABLE override_token (
    id              TEXT PRIMARY KEY,
    approver_id     TEXT NOT NULL REFERENCES crew (id),
    requester_id    TEXT NOT NULL REFERENCES crew (id),
    action          TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    expires_at      INTEGER NOT NULL,
    used_at         INTEGER
);

CREATE TABLE audit_log (
    id               TEXT PRIMARY KEY,
    at               INTEGER NOT NULL,
    actor_crew_id    TEXT REFERENCES crew (id),
    override_crew_id TEXT REFERENCES crew (id),
    action           TEXT NOT NULL,
    entity           TEXT NOT NULL,
    entity_id        TEXT,
    before_json      TEXT,
    after_json       TEXT,
    reason           TEXT
);
CREATE INDEX audit_log_at ON audit_log (at);
CREATE INDEX audit_log_entity ON audit_log (entity, entity_id);

-- +goose StatementBegin
CREATE TRIGGER audit_log_no_update BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER audit_log_no_delete BEFORE DELETE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;
-- +goose StatementEnd

CREATE TABLE ops_setting (
    id                           INTEGER PRIMARY KEY CHECK (id = 1),
    claim_renotify_seconds       INTEGER NOT NULL DEFAULT 120,
    claim_escalate_seconds       INTEGER NOT NULL DEFAULT 300,
    pickup_reminder_seconds      INTEGER NOT NULL DEFAULT 180,
    empty_bill_close_seconds     INTEGER NOT NULL DEFAULT 1800,
    auto_clock_out_grace_seconds INTEGER NOT NULL DEFAULT 7200
);
INSERT INTO ops_setting (id) VALUES (1);

-- Replays for mutating requests carrying an Idempotency-Key (TR-API-01).
CREATE TABLE idempotency_key (
    scope        TEXT NOT NULL,
    key          TEXT NOT NULL,
    method       TEXT NOT NULL,
    path         TEXT NOT NULL,
    status       INTEGER NOT NULL,
    content_type TEXT,
    body         BLOB,
    created_at   INTEGER NOT NULL,
    PRIMARY KEY (scope, key)
);

-- +goose Down
DROP TABLE idempotency_key;
DROP TABLE ops_setting;
DROP TRIGGER audit_log_no_delete;
DROP TRIGGER audit_log_no_update;
DROP TABLE audit_log;
DROP TABLE override_token;
DROP TABLE session;
DROP TABLE crew_station;
DROP TABLE crew;
