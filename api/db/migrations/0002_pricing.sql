-- +goose Up
-- Pricing settings and discounts: BR-ADM-04, BR-ADM-05, BR-ADM-06,
-- BR-R-14 to BR-R-18, TRD §4.4.
-- Rates are basis points (1000 bp = 10%). Discount value is percent
-- points when kind = 'percent' (10 = 10%) and whole Rupiah when
-- kind = 'nominal'. Price mode is 'exclusive' by default (BR-ADM-04,
-- BR-R-15); 'inclusive' is available later and is stored, not yet
-- used to recompute menu prices.

CREATE TABLE IF NOT EXISTS tax_setting (
    id TEXT PRIMARY KEY,
    service_rate_bp INTEGER NOT NULL DEFAULT 0 CHECK (service_rate_bp >= 0),
    tax_rate_bp INTEGER NOT NULL DEFAULT 1000 CHECK (tax_rate_bp >= 0),
    price_mode TEXT NOT NULL DEFAULT 'exclusive' CHECK (price_mode IN ('exclusive', 'inclusive'))
);

CREATE TABLE IF NOT EXISTS refund_setting (
    id TEXT PRIMARY KEY,
    cap_amount INTEGER NOT NULL DEFAULT 500000 CHECK (cap_amount >= 0)
);

CREATE TABLE IF NOT EXISTS discount (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('percent', 'nominal')),
    value INTEGER NOT NULL CHECK (value >= 0),
    scope TEXT NOT NULL CHECK (scope IN ('item', 'bill')),
    trigger TEXT NOT NULL CHECK (trigger IN ('auto', 'manual')),
    days TEXT,
    start_time TEXT,
    end_time TEXT,
    valid_from INTEGER,
    valid_to INTEGER,
    target_kind TEXT,
    target_id TEXT,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_discount_scope_trigger ON discount(scope, trigger, active);

INSERT OR IGNORE INTO tax_setting (id, service_rate_bp, tax_rate_bp, price_mode)
VALUES ('default', 0, 1000, 'exclusive');

INSERT OR IGNORE INTO refund_setting (id, cap_amount)
VALUES ('default', 500000);

-- +goose Down
DROP TABLE IF EXISTS discount;
DROP TABLE IF EXISTS refund_setting;
DROP TABLE IF EXISTS tax_setting;
