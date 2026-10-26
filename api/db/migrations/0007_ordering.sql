-- +goose Up
-- Ordering, Phase 2a: table scan, join and the single open Bill.
-- BR-CM-01, BR-CM-02, BR-CM-12, BR-CM-13, BR-R-01, TR-DM-01,
-- TR-AUTH-07. A Table has at most one open Bill; bill_table
-- carries a denormalised is_open flag with a partial unique index
-- so two scans cannot both open a Bill for the same Table.
-- Customer personal data (name, dietary note, restriction tags)
-- is stored encrypted (TR-RET-04): name_enc, dietary_note_enc
-- and restriction_tags_enc hold AES-256-GCM ciphertext, with
-- key_id identifying the per-day data key. Plaintext is never
-- stored in these columns. Guest tokens implement TR-AUTH-07:
-- a Customer is bound to the Bill, not the device; rejoin issues
-- a new token and revokes previous ones.

CREATE TABLE IF NOT EXISTS bill (
    id TEXT PRIMARY KEY,
    number TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL DEFAULT 'dine_in' CHECK (type IN ('dine_in', 'takeaway')),
    source TEXT NOT NULL DEFAULT 'live' CHECK (source IN ('live', 'post_outage')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'settling', 'closed', 'unpaid', 'merged')),
    takeaway_name TEXT,
    pay_now_serve_later_by TEXT,
    unpaid_reason TEXT,
    unpaid_override_by TEXT,
    opened_at INTEGER NOT NULL,
    closed_at INTEGER,
    anonymised_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_bill_status ON bill(status);
CREATE INDEX IF NOT EXISTS idx_bill_closed ON bill(closed_at);

CREATE TABLE IF NOT EXISTS bill_table (
    bill_id TEXT NOT NULL REFERENCES bill(id),
    table_id TEXT NOT NULL REFERENCES dining_table(id),
    is_open INTEGER NOT NULL DEFAULT 1 CHECK (is_open IN (0, 1)),
    PRIMARY KEY (bill_id, table_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bill_table_one_open ON bill_table(table_id) WHERE is_open = 1;

CREATE TABLE IF NOT EXISTS customer_entry (
    id TEXT PRIMARY KEY,
    bill_id TEXT NOT NULL REFERENCES bill(id),
    key_id TEXT NOT NULL,
    name_enc TEXT NOT NULL,
    dietary_note_enc TEXT,
    restriction_tags_enc TEXT,
    consent_at INTEGER NOT NULL,
    active_device_id TEXT,
    rejoined_at INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_customer_entry_bill ON customer_entry(bill_id);

CREATE TABLE IF NOT EXISTS guest_token (
    token TEXT PRIMARY KEY,
    customer_entry_id TEXT NOT NULL REFERENCES customer_entry(id),
    device_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER,
    revoked_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_guest_token_entry ON guest_token(customer_entry_id);

CREATE TABLE IF NOT EXISTS waiter_call (
    id TEXT PRIMARY KEY,
    table_id TEXT NOT NULL REFERENCES dining_table(id),
    bill_id TEXT REFERENCES bill(id),
    customer_entry_id TEXT REFERENCES customer_entry(id),
    kind TEXT NOT NULL CHECK (kind IN ('waiter_call', 'not_mine')),
    created_at INTEGER NOT NULL,
    acknowledged_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_waiter_call_table ON waiter_call(table_id);

-- +goose Down
DROP TABLE IF EXISTS waiter_call;
DROP TABLE IF EXISTS guest_token;
DROP TABLE IF EXISTS customer_entry;
DROP TABLE IF EXISTS bill_table;
DROP TABLE IF EXISTS bill;
