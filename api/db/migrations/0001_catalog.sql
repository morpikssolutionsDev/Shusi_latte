-- +goose Up
-- Catalog baseline: TRD §4.2, BR-PC-01 to BR-PC-07, BR-ADM-01, BR-ADM-02.
-- Conventions from TRD §4: money is INTEGER whole Rupiah, timestamps are
-- INTEGER UTC Unix milliseconds, primary keys are TEXT UUIDv7, foreign
-- keys are enforced by the application (PRAGMA foreign_keys = ON).

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS prep_area (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    parallel_capacity INTEGER NOT NULL DEFAULT 1 CHECK (parallel_capacity >= 1)
);

CREATE TABLE IF NOT EXISTS category (
    id TEXT PRIMARY KEY,
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    sort INTEGER NOT NULL DEFAULT 0,
    prep_area_id TEXT NOT NULL REFERENCES prep_area(id)
);
CREATE INDEX IF NOT EXISTS idx_category_prep_area ON category(prep_area_id);

CREATE TABLE IF NOT EXISTS product (
    id TEXT PRIMARY KEY,
    category_id TEXT NOT NULL REFERENCES category(id),
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    description_id TEXT NOT NULL DEFAULT '',
    description_en TEXT NOT NULL DEFAULT '',
    image_key TEXT,
    image_focus TEXT NOT NULL DEFAULT '50,50',
    prep_time_seconds INTEGER NOT NULL DEFAULT 0 CHECK (prep_time_seconds >= 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'hidden', 'archived')),
    sort INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    archived_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_product_category ON product(category_id);
CREATE INDEX IF NOT EXISTS idx_product_status ON product(status);

CREATE TABLE IF NOT EXISTS variant (
    id TEXT PRIMARY KEY,
    product_id TEXT NOT NULL REFERENCES product(id),
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    price INTEGER NOT NULL CHECK (price >= 0),
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    sort INTEGER NOT NULL DEFAULT 0,
    sold_out INTEGER NOT NULL DEFAULT 0 CHECK (sold_out IN (0, 1)),
    sold_out_at INTEGER,
    sold_out_by TEXT,
    sold_out_source TEXT CHECK (sold_out_source IS NULL OR sold_out_source IN ('manual', 'cancel_out_of_stock')),
    archived_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_variant_product ON variant(product_id);

CREATE TABLE IF NOT EXISTS modifier_group (
    id TEXT PRIMARY KEY,
    product_id TEXT NOT NULL REFERENCES product(id),
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    min_select INTEGER NOT NULL DEFAULT 0 CHECK (min_select >= 0),
    max_select INTEGER NOT NULL DEFAULT 1 CHECK (max_select >= min_select),
    sort INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_modifier_group_product ON modifier_group(product_id);

CREATE TABLE IF NOT EXISTS modifier (
    id TEXT PRIMARY KEY,
    modifier_group_id TEXT NOT NULL REFERENCES modifier_group(id),
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    price INTEGER NOT NULL DEFAULT 0 CHECK (price >= 0),
    sort INTEGER NOT NULL DEFAULT 0,
    archived_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_modifier_group ON modifier(modifier_group_id);

CREATE TABLE IF NOT EXISTS dietary_tag (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name_id TEXT NOT NULL,
    name_en TEXT NOT NULL,
    icon TEXT,
    sort INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);

CREATE TABLE IF NOT EXISTS product_dietary_tag (
    product_id TEXT NOT NULL REFERENCES product(id),
    dietary_tag_id TEXT NOT NULL REFERENCES dietary_tag(id),
    PRIMARY KEY (product_id, dietary_tag_id)
);

CREATE TABLE IF NOT EXISTS modifier_dietary_tag (
    modifier_id TEXT NOT NULL REFERENCES modifier(id),
    dietary_tag_id TEXT NOT NULL REFERENCES dietary_tag(id),
    PRIMARY KEY (modifier_id, dietary_tag_id)
);

-- Audit log: TR-AUD-01, TR-AUD-02, TR-DM-04 (append-only).
CREATE TABLE IF NOT EXISTS audit_log (
    id TEXT PRIMARY KEY,
    at INTEGER NOT NULL,
    actor_crew_id TEXT,
    override_crew_id TEXT,
    action TEXT NOT NULL,
    entity TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    before_json TEXT,
    after_json TEXT,
    reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log(entity, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_log(at);

CREATE TRIGGER IF NOT EXISTS audit_log_no_update
BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;

CREATE TRIGGER IF NOT EXISTS audit_log_no_delete
BEFORE DELETE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'audit_log is append-only');
END;

-- Baseline seed: Prep Areas from BR-PC-07 and Dietary Tags from BR-PC-06.
INSERT OR IGNORE INTO prep_area (id, name, parallel_capacity) VALUES
    ('prep-kitchen', 'Kitchen', 1),
    ('prep-bar', 'Bar', 1);

INSERT OR IGNORE INTO dietary_tag (id, code, name_id, name_en, sort, active) VALUES
    ('tag-dairy', 'dairy', 'Susu / Laktosa', 'Dairy / Lactose', 1, 1),
    ('tag-gluten', 'gluten', 'Gluten', 'Gluten', 2, 1),
    ('tag-egg', 'egg', 'Telur', 'Egg', 3, 1),
    ('tag-fish', 'fish', 'Ikan', 'Fish', 4, 1),
    ('tag-raw-fish', 'raw_fish', 'Ikan Mentah', 'Raw Fish', 5, 1),
    ('tag-shellfish', 'shellfish', 'Kerang-kerangan', 'Shellfish', 6, 1),
    ('tag-nuts', 'nuts', 'Kacang', 'Nuts', 7, 1),
    ('tag-peanut', 'peanut', 'Kacang Tanah', 'Peanut', 8, 1),
    ('tag-soy', 'soy', 'Kedelai', 'Soy', 9, 1),
    ('tag-sesame', 'sesame', 'Wijen', 'Sesame', 10, 1),
    ('tag-pork', 'pork', 'Babi', 'Pork', 11, 1),
    ('tag-alcohol', 'alcohol', 'Alkohol', 'Alcohol', 12, 1),
    ('tag-spicy', 'spicy', 'Pedas', 'Spicy', 13, 1);

-- +goose Down
DROP TRIGGER IF EXISTS audit_log_no_delete;
DROP TRIGGER IF EXISTS audit_log_no_update;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS modifier_dietary_tag;
DROP TABLE IF EXISTS product_dietary_tag;
DROP TABLE IF EXISTS dietary_tag;
DROP TABLE IF EXISTS modifier;
DROP TABLE IF EXISTS modifier_group;
DROP TABLE IF EXISTS variant;
DROP TABLE IF EXISTS product;
DROP TABLE IF EXISTS category;
DROP TABLE IF EXISTS prep_area;
