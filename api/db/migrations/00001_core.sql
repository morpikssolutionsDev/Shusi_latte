-- Core platform tables: Crew and sign-in, PIN Overrides, audit log, ops settings, idempotency.
-- Conventions (TRD §4): ids are UUIDv7 TEXT, timestamps are UTC Unix milliseconds, money is BIGINT Rupiah,
-- flags are BIGINT 0/1 (PostgreSQL, ADR-0007).

-- +goose Up
CREATE TABLE crew (
    id              TEXT PRIMARY KEY,
    crew_id         TEXT NOT NULL UNIQUE,
    full_name       TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('management', 'outlet_admin', 'crew')),
    image_key       TEXT,
    pin_hash        TEXT NOT NULL,
    failed_attempts BIGINT NOT NULL DEFAULT 0,
    locked_until    BIGINT,
    active          BIGINT NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at      BIGINT NOT NULL,
    updated_at      BIGINT NOT NULL
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
    created_at      BIGINT NOT NULL,
    last_seen_at    BIGINT NOT NULL,
    expires_at      BIGINT NOT NULL,
    idle_timeout_ms BIGINT
);
CREATE INDEX session_expires_at ON session (expires_at);

-- One-time PIN Override tokens (TR-AUTH-06, TR-API-03). id is the SHA-256 of the token.
CREATE TABLE override_token (
    id              TEXT PRIMARY KEY,
    approver_id     TEXT NOT NULL REFERENCES crew (id),
    requester_id    TEXT NOT NULL REFERENCES crew (id),
    action          TEXT NOT NULL,
    created_at      BIGINT NOT NULL,
    expires_at      BIGINT NOT NULL,
    used_at         BIGINT
);

CREATE TABLE audit_log (
    id               TEXT PRIMARY KEY,
    at               BIGINT NOT NULL,
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
CREATE FUNCTION audit_log_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_log_no_update BEFORE UPDATE ON audit_log FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();
CREATE TRIGGER audit_log_no_delete BEFORE DELETE ON audit_log FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

CREATE TABLE ops_setting (
    id                           BIGINT PRIMARY KEY CHECK (id = 1),
    claim_renotify_seconds       BIGINT NOT NULL DEFAULT 120,
    claim_escalate_seconds       BIGINT NOT NULL DEFAULT 300,
    pickup_reminder_seconds      BIGINT NOT NULL DEFAULT 180,
    empty_bill_close_seconds     BIGINT NOT NULL DEFAULT 1800,
    auto_clock_out_grace_seconds BIGINT NOT NULL DEFAULT 7200
);
INSERT INTO ops_setting (id) VALUES (1);

-- Replays for mutating requests carrying an Idempotency-Key (TR-API-01).
CREATE TABLE idempotency_key (
    scope        TEXT NOT NULL,
    key          TEXT NOT NULL,
    method       TEXT NOT NULL,
    path         TEXT NOT NULL,
    status       BIGINT NOT NULL,
    content_type TEXT,
    body         BYTEA,
    created_at   BIGINT NOT NULL,
    PRIMARY KEY (scope, key)
);

-- +goose Down
DROP TABLE idempotency_key;
DROP TABLE ops_setting;
DROP TABLE audit_log;
DROP FUNCTION audit_log_append_only();
DROP TABLE override_token;
DROP TABLE session;
DROP TABLE crew_station;
DROP TABLE crew;
