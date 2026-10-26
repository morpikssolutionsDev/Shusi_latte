-- +goose Up
-- Cashier, Phase 6: payments, refunds, drawer and handovers.
-- BR-CS-01 to BR-CS-08, BR-R-05, BR-R-14 to BR-R-18, BR-R-30,
-- BR-R-31, BR-R-32, TR-PR-05, TRD §4.4. QRIS is a static code
-- verified by staff, not a payment gateway (BRD §4.2): a QRIS
-- payment starts pending and must be verified or written off.
-- Cash rounding to nearest Rp 100 applies only to cash
-- (BR-R-18) and is stored on the payment row.

CREATE TABLE IF NOT EXISTS payment (
    id TEXT PRIMARY KEY,
    bill_id TEXT NOT NULL REFERENCES bill(id),
    sub_bill_id TEXT REFERENCES sub_bill(id),
    method TEXT NOT NULL CHECK (method IN ('cash', 'qris', 'card')),
    amount INTEGER NOT NULL CHECK (amount >= 0),
    rounding INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'verified' CHECK (status IN ('pending', 'verified', 'written_off')),
    taken_by TEXT,
    verified_by TEXT,
    verified_at INTEGER,
    write_off_reason TEXT,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_payment_bill ON payment(bill_id);

CREATE TABLE IF NOT EXISTS refund (
    id TEXT PRIMARY KEY,
    bill_id TEXT NOT NULL REFERENCES bill(id),
    payment_id TEXT REFERENCES payment(id),
    amount INTEGER NOT NULL CHECK (amount > 0),
    reason TEXT NOT NULL,
    override_by TEXT,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_refund_bill ON refund(bill_id);

CREATE TABLE IF NOT EXISTS cash_drawer (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS drawer_session (
    id TEXT PRIMARY KEY,
    cash_drawer_id TEXT NOT NULL REFERENCES cash_drawer(id),
    cashier_crew_id TEXT NOT NULL,
    opened_at INTEGER NOT NULL,
    closed_at INTEGER,
    opening_float INTEGER NOT NULL CHECK (opening_float >= 0),
    counted_cash INTEGER,
    expected_cash INTEGER,
    difference INTEGER
);
CREATE INDEX IF NOT EXISTS idx_drawer_open ON drawer_session(closed_at);

CREATE TABLE IF NOT EXISTS cash_handover (
    id TEXT PRIMARY KEY,
    waiter_crew_id TEXT NOT NULL,
    drawer_session_id TEXT REFERENCES drawer_session(id),
    amount INTEGER NOT NULL CHECK (amount >= 0),
    confirmed_amount INTEGER,
    difference INTEGER,
    confirmed_by TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed')),
    created_at INTEGER NOT NULL,
    confirmed_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_handover_waiter ON cash_handover(waiter_crew_id);

CREATE TABLE IF NOT EXISTS applied_discount (
    id TEXT PRIMARY KEY,
    order_item_id TEXT REFERENCES order_item(id),
    bill_id TEXT REFERENCES bill(id),
    sub_bill_id TEXT REFERENCES sub_bill(id),
    discount_id TEXT NOT NULL REFERENCES discount(id),
    amount INTEGER NOT NULL CHECK (amount >= 0),
    override_by TEXT,
    created_at INTEGER NOT NULL
);

INSERT OR IGNORE INTO cash_drawer (id, name) VALUES ('main', 'Main Drawer');

-- +goose Down
DROP TABLE IF EXISTS applied_discount;
DROP TABLE IF EXISTS cash_handover;
DROP TABLE IF EXISTS drawer_session;
DROP TABLE IF EXISTS cash_drawer;
DROP TABLE IF EXISTS refund;
DROP TABLE IF EXISTS payment;
