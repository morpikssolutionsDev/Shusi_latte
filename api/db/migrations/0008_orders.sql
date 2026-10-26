-- +goose Up
-- Orders, Phase 2b: customer Orders on an open Bill.
-- BR-CM-05, BR-CM-06, BR-CM-08, BR-CM-09, BR-CM-10, BR-CM-11,
-- BR-CM-14, BR-CM-15, BR-R-02, BR-R-06, BR-R-07, TR-DM-02,
-- TR-DM-05, TR-SM-01, TR-SM-02. Tables follow TRD §4.3 exactly.
-- Order Item names and prices are snapshots at order time
-- (TR-DM-02): later catalog changes never alter existing Orders.
-- Dietary Warnings hold only the Product-side tag (TR-RET-01):
-- order_item_dietary_warning has no link to the person who
-- avoided it, so Anonymisation can keep the warning while
-- deleting who avoided it. Order status in Phase 2 is pending
-- until a Waiter Claims and confirms it in Phase 4; the status
-- column already allows the later lifecycle values.

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    bill_id TEXT NOT NULL REFERENCES bill(id),
    customer_entry_id TEXT REFERENCES customer_entry(id),
    source TEXT NOT NULL DEFAULT 'client_menu' CHECK (source IN ('client_menu', 'waiter', 'cashier')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'preparing', 'ready', 'served', 'cancelled')),
    claimed_by TEXT,
    claimed_at INTEGER,
    confirmed_at INTEGER,
    key_id TEXT,
    restriction_tags_enc TEXT,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_orders_bill ON orders(bill_id);
CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(customer_entry_id);

CREATE TABLE IF NOT EXISTS order_item (
    id TEXT PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id),
    product_id TEXT NOT NULL REFERENCES product(id),
    variant_id TEXT NOT NULL REFERENCES variant(id),
    product_name_snapshot TEXT NOT NULL,
    variant_name_snapshot TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price INTEGER NOT NULL CHECK (unit_price >= 0),
    note TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'preparing', 'ready', 'served', 'cancelled', 'voided')),
    cancel_reason TEXT,
    cancel_note TEXT,
    cancelled_by TEXT,
    void_reason TEXT,
    voided_by TEXT,
    voided_override_by TEXT,
    remake_of_id TEXT REFERENCES order_item(id),
    remake_reason TEXT,
    remake_override_by TEXT,
    ready_at INTEGER,
    sub_bill_id TEXT,
    sold_out_flag INTEGER NOT NULL DEFAULT 0 CHECK (sold_out_flag IN (0, 1)),
    unavailable_flag INTEGER NOT NULL DEFAULT 0 CHECK (unavailable_flag IN (0, 1))
);
CREATE INDEX IF NOT EXISTS idx_order_item_order ON order_item(order_id);

CREATE TABLE IF NOT EXISTS order_item_modifier (
    order_item_id TEXT NOT NULL REFERENCES order_item(id),
    modifier_id TEXT NOT NULL REFERENCES modifier(id),
    name_snapshot TEXT NOT NULL,
    price INTEGER NOT NULL CHECK (price >= 0),
    PRIMARY KEY (order_item_id, modifier_id)
);

CREATE TABLE IF NOT EXISTS order_item_dietary_warning (
    order_item_id TEXT NOT NULL REFERENCES order_item(id),
    dietary_tag_id TEXT NOT NULL REFERENCES dietary_tag(id),
    acknowledged_at INTEGER,
    PRIMARY KEY (order_item_id, dietary_tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS order_item_dietary_warning;
DROP TABLE IF EXISTS order_item_modifier;
DROP TABLE IF EXISTS order_item;
DROP TABLE IF EXISTS orders;
