-- Product catalog (BRD §6.0, TRD §4.2), pricing settings and discounts (TRD §4.4), Tables (TRD §4.3).

-- +goose Up
CREATE TABLE prep_area (
    id                TEXT PRIMARY KEY,
    code              TEXT NOT NULL UNIQUE,
    name_id           TEXT NOT NULL,
    name_en           TEXT NOT NULL,
    parallel_capacity INTEGER NOT NULL DEFAULT 1 CHECK (parallel_capacity >= 1),
    sort              INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE category (
    id           TEXT PRIMARY KEY,
    name_id      TEXT NOT NULL,
    name_en      TEXT NOT NULL,
    prep_area_id TEXT NOT NULL REFERENCES prep_area (id),
    sort         INTEGER NOT NULL DEFAULT 0,
    active       INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE product (
    id                TEXT PRIMARY KEY,
    category_id       TEXT NOT NULL REFERENCES category (id),
    name_id           TEXT NOT NULL,
    name_en           TEXT NOT NULL,
    description_id    TEXT NOT NULL DEFAULT '',
    description_en    TEXT NOT NULL DEFAULT '',
    image_key         TEXT,
    image_focus_x     INTEGER NOT NULL DEFAULT 50 CHECK (image_focus_x BETWEEN 0 AND 100),
    image_focus_y     INTEGER NOT NULL DEFAULT 50 CHECK (image_focus_y BETWEEN 0 AND 100),
    prep_time_seconds INTEGER NOT NULL CHECK (prep_time_seconds > 0),
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'hidden', 'archived')),
    sort              INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);
CREATE INDEX product_category ON product (category_id, sort);

-- Price and Sold Out live on the Variant; a Product with no options has one default Variant (TR-PC-01).
CREATE TABLE variant (
    id              TEXT PRIMARY KEY,
    product_id      TEXT NOT NULL REFERENCES product (id),
    name_id         TEXT NOT NULL,
    name_en         TEXT NOT NULL,
    price           INTEGER NOT NULL CHECK (price >= 0),
    is_default      INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    sort            INTEGER NOT NULL DEFAULT 0,
    sold_out        INTEGER NOT NULL DEFAULT 0 CHECK (sold_out IN (0, 1)),
    sold_out_at     INTEGER,
    sold_out_by     TEXT REFERENCES crew (id),
    sold_out_source TEXT CHECK (sold_out_source IN ('manual', 'cancel_out_of_stock')),
    archived_at     INTEGER
);
CREATE INDEX variant_product ON variant (product_id, sort);

CREATE TABLE modifier_group (
    id          TEXT PRIMARY KEY,
    product_id  TEXT NOT NULL REFERENCES product (id),
    name_id     TEXT NOT NULL,
    name_en     TEXT NOT NULL,
    min_select  INTEGER NOT NULL DEFAULT 0 CHECK (min_select >= 0),
    max_select  INTEGER NOT NULL DEFAULT 1 CHECK (max_select >= 1),
    sort        INTEGER NOT NULL DEFAULT 0,
    archived_at INTEGER,
    CHECK (min_select <= max_select)
);
CREATE INDEX modifier_group_product ON modifier_group (product_id, sort);

CREATE TABLE modifier (
    id                TEXT PRIMARY KEY,
    modifier_group_id TEXT NOT NULL REFERENCES modifier_group (id),
    name_id           TEXT NOT NULL,
    name_en           TEXT NOT NULL,
    price             INTEGER NOT NULL DEFAULT 0 CHECK (price >= 0),
    sort              INTEGER NOT NULL DEFAULT 0,
    archived_at       INTEGER
);
CREATE INDEX modifier_group_sort ON modifier (modifier_group_id, sort);

CREATE TABLE dietary_tag (
    id      TEXT PRIMARY KEY,
    code    TEXT NOT NULL UNIQUE,
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    icon    TEXT NOT NULL DEFAULT '',
    sort    INTEGER NOT NULL DEFAULT 0,
    active  INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);

CREATE TABLE product_dietary_tag (
    product_id     TEXT NOT NULL REFERENCES product (id) ON DELETE CASCADE,
    dietary_tag_id TEXT NOT NULL REFERENCES dietary_tag (id),
    PRIMARY KEY (product_id, dietary_tag_id)
);

CREATE TABLE modifier_dietary_tag (
    modifier_id    TEXT NOT NULL REFERENCES modifier (id) ON DELETE CASCADE,
    dietary_tag_id TEXT NOT NULL REFERENCES dietary_tag (id),
    PRIMARY KEY (modifier_id, dietary_tag_id)
);

-- Tax, service charge, price mode and Refund cap (BR-ADM-04, BR-ADM-06). Rates are basis points.
CREATE TABLE pricing_setting (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    service_rate_bp INTEGER NOT NULL DEFAULT 500 CHECK (service_rate_bp BETWEEN 0 AND 10000),
    tax_rate_bp     INTEGER NOT NULL DEFAULT 1000 CHECK (tax_rate_bp BETWEEN 0 AND 10000),
    price_mode      TEXT NOT NULL DEFAULT 'exclusive' CHECK (price_mode IN ('exclusive', 'inclusive')),
    refund_cap      INTEGER NOT NULL DEFAULT 500000 CHECK (refund_cap >= 0)
);
INSERT INTO pricing_setting (id) VALUES (1);

-- Discounts (BR-ADM-05). value: basis points for percent, Rupiah for nominal.
-- days_mask: bit 0 = Monday … bit 6 = Sunday. Minutes are WIB minutes after midnight;
-- start > end means the window crosses midnight.
CREATE TABLE discount (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('percent', 'nominal')),
    value        INTEGER NOT NULL CHECK (value > 0),
    scope        TEXT NOT NULL CHECK (scope IN ('item', 'bill')),
    trigger      TEXT NOT NULL CHECK (trigger IN ('auto', 'manual')),
    days_mask    INTEGER NOT NULL DEFAULT 127 CHECK (days_mask BETWEEN 1 AND 127),
    start_minute INTEGER CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute   INTEGER CHECK (end_minute BETWEEN 0 AND 1440),
    valid_from   INTEGER,
    valid_to     INTEGER,
    min_subtotal INTEGER CHECK (min_subtotal >= 0),
    active       INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    CHECK (kind = 'nominal' OR value <= 10000)
);

-- No rows for an item discount means it applies to every Product.
CREATE TABLE discount_target (
    discount_id TEXT NOT NULL REFERENCES discount (id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('category', 'product', 'variant')),
    target_id   TEXT NOT NULL,
    PRIMARY KEY (discount_id, target_type, target_id)
);

CREATE TABLE dining_table (
    id                TEXT PRIMARY KEY,
    number            INTEGER NOT NULL UNIQUE CHECK (number > 0),
    label             TEXT NOT NULL,
    qr_token          TEXT NOT NULL UNIQUE,
    active            INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    qr_regenerated_at INTEGER,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

-- +goose Down
DROP TABLE dining_table;
DROP TABLE discount_target;
DROP TABLE discount;
DROP TABLE pricing_setting;
DROP TABLE modifier_dietary_tag;
DROP TABLE product_dietary_tag;
DROP TABLE dietary_tag;
DROP TABLE modifier;
DROP TABLE modifier_group;
DROP TABLE variant;
DROP TABLE product;
DROP TABLE category;
DROP TABLE prep_area;
