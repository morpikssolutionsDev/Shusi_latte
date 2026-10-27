-- Cashier: Cash Drawer sessions, Cash Handovers, Refunds (OP-11, OP-13, TRD §4.4, BR-CS-03..07).

-- +goose Up
-- Exactly one drawer in the MVP (BR-CS-04).
CREATE TABLE cash_drawer (
    id    TEXT PRIMARY KEY,
    label TEXT NOT NULL
);
INSERT INTO cash_drawer (id, label) VALUES ('main', 'Laci kasir');

-- One Opening Float → Shift Closing cycle; at most one open at a time.
CREATE TABLE drawer_session (
    id              TEXT PRIMARY KEY,
    cash_drawer_id  TEXT NOT NULL REFERENCES cash_drawer (id),
    cashier_crew_id TEXT NOT NULL REFERENCES crew (id),
    opening_float   INTEGER NOT NULL CHECK (opening_float >= 0),
    opened_at       INTEGER NOT NULL,
    expected_cash   INTEGER,
    counted_cash    INTEGER,
    difference      INTEGER,
    closed_at       INTEGER,
    closed_by       TEXT REFERENCES crew (id)
);
CREATE UNIQUE INDEX drawer_session_one_open ON drawer_session (cash_drawer_id) WHERE closed_at IS NULL;

CREATE TABLE cash_handover (
    id                TEXT PRIMARY KEY,
    drawer_session_id TEXT NOT NULL REFERENCES drawer_session (id),
    waiter_crew_id    TEXT NOT NULL REFERENCES crew (id),
    expected_amount   INTEGER NOT NULL,
    received_amount   INTEGER NOT NULL,
    difference        INTEGER NOT NULL, -- received − expected, recorded against the Waiter
    received_by       TEXT NOT NULL REFERENCES crew (id),
    received_at       INTEGER NOT NULL
);

-- Always paid in cash from the drawer, whatever the original method (BR-R-20, G-17).
CREATE TABLE refund (
    id                TEXT PRIMARY KEY,
    bill_id           TEXT NOT NULL REFERENCES bill (id),
    amount            INTEGER NOT NULL CHECK (amount > 0),
    reason            TEXT NOT NULL,
    refunded_by       TEXT NOT NULL REFERENCES crew (id),
    approved_by       TEXT NOT NULL REFERENCES crew (id),
    drawer_session_id TEXT NOT NULL REFERENCES drawer_session (id),
    created_at        INTEGER NOT NULL
);

-- Counter cash lands in the open drawer session; Waiter cash joins it at the Cash Handover.
ALTER TABLE payment ADD COLUMN drawer_session_id TEXT REFERENCES drawer_session (id);
ALTER TABLE payment ADD COLUMN written_off_reason TEXT;
ALTER TABLE payment ADD COLUMN written_off_by TEXT REFERENCES crew (id);

-- +goose Down
ALTER TABLE payment DROP COLUMN written_off_by;
ALTER TABLE payment DROP COLUMN written_off_reason;
ALTER TABLE payment DROP COLUMN drawer_session_id;
DROP TABLE refund;
DROP TABLE cash_handover;
DROP TABLE drawer_session;
DROP TABLE cash_drawer;
