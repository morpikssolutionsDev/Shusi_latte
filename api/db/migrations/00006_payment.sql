-- Bills and payment (OP-09, OP-10, TRD §4.4, BR-R-14..32).

-- +goose Up
-- Settling stamps the pricing moment so a promo ending mid-payment cannot move the total.
ALTER TABLE bill ADD COLUMN priced_at INTEGER;
-- A Management-approved manual Bill discount (BR-R-17).
ALTER TABLE bill ADD COLUMN manual_discount_id TEXT REFERENCES discount (id);
ALTER TABLE bill ADD COLUMN manual_discount_by TEXT REFERENCES crew (id);
-- Totals frozen when the Bill closes (reports read these, TR-PR-01).
ALTER TABLE bill ADD COLUMN subtotal INTEGER;
ALTER TABLE bill ADD COLUMN item_discount INTEGER;
ALTER TABLE bill ADD COLUMN bill_discount INTEGER;
ALTER TABLE bill ADD COLUMN service INTEGER;
ALTER TABLE bill ADD COLUMN tax INTEGER;
ALTER TABLE bill ADD COLUMN total INTEGER;

CREATE TABLE sub_bill (
    id            TEXT PRIMARY KEY,
    bill_id       TEXT NOT NULL REFERENCES bill (id),
    method        TEXT NOT NULL CHECK (method IN ('by_item', 'equal')),
    share_index   INTEGER NOT NULL,
    share_count   INTEGER NOT NULL,
    status        TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    subtotal      INTEGER,
    item_discount INTEGER,
    bill_discount INTEGER,
    service       INTEGER,
    tax           INTEGER,
    total         INTEGER,
    created_at    INTEGER NOT NULL,
    closed_at     INTEGER,
    UNIQUE (bill_id, share_index)
);

ALTER TABLE order_item ADD COLUMN sub_bill_id TEXT REFERENCES sub_bill (id);

CREATE TABLE payment (
    id               TEXT PRIMARY KEY,
    bill_id          TEXT NOT NULL REFERENCES bill (id),
    sub_bill_id      TEXT REFERENCES sub_bill (id),
    method           TEXT NOT NULL CHECK (method IN ('cash', 'qris', 'card')),
    status           TEXT NOT NULL CHECK (status IN ('settled', 'held_by_waiter', 'pending_verification', 'written_off')),
    amount           INTEGER NOT NULL CHECK (amount > 0), -- credited to the Bill (cash: after rounding)
    tendered         INTEGER,                             -- cash handed over
    change_given     INTEGER NOT NULL DEFAULT 0,
    rounding         INTEGER NOT NULL DEFAULT 0,          -- cash only: rounded − exact (BR-R-18)
    taken_by         TEXT NOT NULL REFERENCES crew (id),
    taken_at         INTEGER NOT NULL,
    cash_handover_id TEXT,                                -- Cashier phase
    override_by      TEXT REFERENCES crew (id),           -- pending QRIS approver (BR-R-30)
    verified_by      TEXT REFERENCES crew (id),
    verified_at      INTEGER
);
CREATE INDEX payment_bill ON payment (bill_id);
CREATE INDEX payment_status ON payment (status);

CREATE TABLE receipt_print (
    id          TEXT PRIMARY KEY,
    bill_id     TEXT NOT NULL REFERENCES bill (id),
    sub_bill_id TEXT REFERENCES sub_bill (id),
    copy        TEXT NOT NULL CHECK (copy IN ('customer', 'internal')),
    is_reprint  INTEGER NOT NULL DEFAULT 0 CHECK (is_reprint IN (0, 1)),
    printed_by  TEXT NOT NULL REFERENCES crew (id),
    printed_at  INTEGER NOT NULL
);

-- +goose Down
DROP TABLE receipt_print;
DROP TABLE payment;
ALTER TABLE order_item DROP COLUMN sub_bill_id;
DROP TABLE sub_bill;
ALTER TABLE bill DROP COLUMN total;
ALTER TABLE bill DROP COLUMN tax;
ALTER TABLE bill DROP COLUMN service;
ALTER TABLE bill DROP COLUMN bill_discount;
ALTER TABLE bill DROP COLUMN item_discount;
ALTER TABLE bill DROP COLUMN subtotal;
ALTER TABLE bill DROP COLUMN manual_discount_by;
ALTER TABLE bill DROP COLUMN manual_discount_id;
ALTER TABLE bill DROP COLUMN priced_at;
