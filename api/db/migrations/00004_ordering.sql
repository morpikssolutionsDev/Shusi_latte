-- Ordering: Bills, Customers, Orders and Order Items (TRD §4.3, BR-CM-*, OP-01, OP-02).
-- Customer names, Dietary Notes and Dietary Restrictions are encrypted with a daily key
-- (key_id) held outside the database (TR-RET-04).

-- +goose Up
CREATE TABLE bill (
    id                     TEXT PRIMARY KEY,
    business_date          TEXT NOT NULL,              -- WIB date the business day started (04:00 boundary)
    number                 BIGINT NOT NULL,           -- daily sequence, shown as B-0427
    type                   TEXT NOT NULL CHECK (type IN ('dine_in', 'takeaway')),
    source                 TEXT NOT NULL DEFAULT 'live' CHECK (source IN ('live', 'post_outage')),
    status                 TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'settling', 'closed', 'unpaid', 'merged')),
    key_id                 TEXT,                       -- data key for takeaway_name_enc
    takeaway_name_enc      BYTEA,
    pay_now_serve_later_by TEXT REFERENCES crew (id),
    unpaid_reason          TEXT,
    unpaid_override_by     TEXT REFERENCES crew (id),
    merged_into_id         TEXT REFERENCES bill (id),
    opened_at              BIGINT NOT NULL,
    closed_at              BIGINT,
    anonymised_at          BIGINT,
    updated_at             BIGINT NOT NULL,
    UNIQUE (business_date, number)
);
CREATE INDEX bill_status ON bill (status);

-- Tables a Bill covers (several after Merge). is_open mirrors bill.status IN ('open','settling')
-- so the partial unique index guarantees one open Bill per Table (TR-DM-01).
CREATE TABLE bill_table (
    bill_id  TEXT NOT NULL REFERENCES bill (id),
    table_id TEXT NOT NULL REFERENCES dining_table (id),
    is_open  BIGINT NOT NULL DEFAULT 1 CHECK (is_open IN (0, 1)),
    PRIMARY KEY (bill_id, table_id)
);
CREATE UNIQUE INDEX bill_table_one_open ON bill_table (table_id) WHERE is_open = 1;

-- One per person at the Table, bound to the Bill rather than a device (BR-CM-13).
CREATE TABLE customer_entry (
    id                   TEXT PRIMARY KEY,
    bill_id              TEXT NOT NULL REFERENCES bill (id),
    key_id               TEXT NOT NULL,
    name_enc             BYTEA,                         -- NULL once Anonymised ("Customer")
    dietary_note_enc     BYTEA,
    restriction_tags_enc BYTEA,                         -- JSON array of dietary_tag ids
    consent_at           BIGINT,                      -- set whenever dietary data is stored
    active_device_id     TEXT,                         -- guest_token.id of the current device
    joined_at            BIGINT NOT NULL,
    rejoined_at          BIGINT
);
CREATE INDEX customer_entry_bill ON customer_entry (bill_id);

-- Guest tokens: one per device a Customer used. Only the active one authenticates; the rest
-- are kept revoked so the old device can be told "You continued on another device".
CREATE TABLE guest_token (
    id                TEXT PRIMARY KEY,
    customer_entry_id TEXT NOT NULL REFERENCES customer_entry (id),
    token_hash        TEXT NOT NULL UNIQUE,
    created_at        BIGINT NOT NULL,
    revoked_at        BIGINT
);

CREATE TABLE orders (
    id                   TEXT PRIMARY KEY,
    bill_id              TEXT NOT NULL REFERENCES bill (id),
    seq                  BIGINT NOT NULL,             -- 1, 2, 3… within the Bill
    customer_entry_id    TEXT REFERENCES customer_entry (id),
    source               TEXT NOT NULL CHECK (source IN ('client_menu', 'waiter', 'cashier')),
    status               TEXT NOT NULL CHECK (status IN ('pending', 'claimed', 'confirmed', 'cancelled')),
    placed_by            TEXT REFERENCES crew (id),    -- staff direct entry
    claimed_by           TEXT REFERENCES crew (id),
    claimed_at           BIGINT,
    confirmed_by         TEXT REFERENCES crew (id),
    confirmed_at         BIGINT,
    key_id               TEXT,
    restriction_tags_enc BYTEA,                         -- Waiter-ticked restriction on direct entry
    renotified_at        BIGINT,
    escalated_at         BIGINT,
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL,
    UNIQUE (bill_id, seq)
);
CREATE INDEX orders_status ON orders (status);

CREATE TABLE order_item (
    id                 TEXT PRIMARY KEY,
    order_id           TEXT NOT NULL REFERENCES orders (id),
    sort               BIGINT NOT NULL,
    product_id         TEXT NOT NULL REFERENCES product (id),
    variant_id         TEXT NOT NULL REFERENCES variant (id),
    category_id        TEXT NOT NULL REFERENCES category (id),
    prep_area_id       TEXT NOT NULL REFERENCES prep_area (id),
    -- Snapshots taken at order time (TR-DM-02, BR-PC-08).
    product_name_id    TEXT NOT NULL,
    product_name_en    TEXT NOT NULL,
    variant_name_id    TEXT NOT NULL,
    variant_name_en    TEXT NOT NULL,
    unit_price         BIGINT NOT NULL,
    prep_time_seconds  BIGINT NOT NULL,
    quantity           BIGINT NOT NULL CHECK (quantity BETWEEN 1 AND 99),
    note               TEXT NOT NULL DEFAULT '',
    status             TEXT NOT NULL CHECK (status IN ('pending', 'confirmed', 'preparing', 'ready', 'served', 'cancelled', 'voided')),
    cancel_reason      TEXT CHECK (cancel_reason IN ('out_of_stock', 'customer_request', 'wrong_order', 'other')),
    cancel_note        TEXT,
    cancelled_by       TEXT REFERENCES crew (id),       -- NULL with a reason = the Customer
    cancelled_at       BIGINT,
    void_reason        TEXT,
    voided_by          TEXT REFERENCES crew (id),
    voided_override_by TEXT REFERENCES crew (id),
    voided_at          BIGINT,
    remake_of_id       TEXT REFERENCES order_item (id),
    remake_reason      TEXT,
    remake_override_by TEXT REFERENCES crew (id),
    preparing_at       BIGINT,
    ready_at           BIGINT,
    served_at          BIGINT,
    served_by          TEXT REFERENCES crew (id),
    sold_out_flag      BIGINT NOT NULL DEFAULT 0 CHECK (sold_out_flag IN (0, 1)),
    unavailable_flag   BIGINT NOT NULL DEFAULT 0 CHECK (unavailable_flag IN (0, 1)),
    created_at         BIGINT NOT NULL,
    updated_at         BIGINT NOT NULL
);
CREATE INDEX order_item_order ON order_item (order_id);
CREATE INDEX order_item_status ON order_item (status, prep_area_id);

CREATE TABLE order_item_modifier (
    order_item_id TEXT NOT NULL REFERENCES order_item (id),
    modifier_id   TEXT NOT NULL REFERENCES modifier (id),
    name_id       TEXT NOT NULL,
    name_en       TEXT NOT NULL,
    price         BIGINT NOT NULL,
    PRIMARY KEY (order_item_id, modifier_id)
);

-- Product-side tag only; no link to the person who avoids it (BR-P-03).
CREATE TABLE order_item_dietary_warning (
    order_item_id   TEXT NOT NULL REFERENCES order_item (id),
    dietary_tag_id  TEXT NOT NULL REFERENCES dietary_tag (id),
    acknowledged_at BIGINT,
    PRIMARY KEY (order_item_id, dietary_tag_id)
);

-- Call Waiter (BR-CM-07) and "This isn't my table" alerts (BR-CM-12).
CREATE TABLE waiter_call (
    id                TEXT PRIMARY KEY,
    kind              TEXT NOT NULL CHECK (kind IN ('call', 'not_mine')),
    table_id          TEXT NOT NULL REFERENCES dining_table (id),
    bill_id           TEXT REFERENCES bill (id),
    customer_entry_id TEXT REFERENCES customer_entry (id),
    created_at        BIGINT NOT NULL,
    renotified_at     BIGINT,
    escalated_at      BIGINT,
    answered_by       TEXT REFERENCES crew (id),
    answered_at       BIGINT
);
CREATE INDEX waiter_call_open ON waiter_call (answered_at);

-- +goose Down
DROP TABLE waiter_call;
DROP TABLE order_item_dietary_warning;
DROP TABLE order_item_modifier;
DROP TABLE order_item;
DROP TABLE orders;
DROP TABLE guest_token;
DROP TABLE customer_entry;
DROP TABLE bill_table;
DROP TABLE bill;
