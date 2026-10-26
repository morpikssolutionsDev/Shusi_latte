-- +goose Up
-- Product Display content: BR-ADM-10, BR-PD-01 to BR-PD-03, TRD §4.5.
-- Management manages banners, featured Products, ordering and
-- schedule in the Admin Panel; the TV slideshow itself is Phase 3.
-- display_slide follows TRD §4.5 key columns, plus an active flag
-- so Management can pause a slide without losing its sort order or
-- schedule. The public feed filters by active and by schedule, and
-- marks Sold Out featured Products (BR-PD-03) at read time rather
-- than storing a copy of Product state, so the catalog stays the
-- single definition of what the Business sells (BRD §6).

CREATE TABLE IF NOT EXISTS display_slide (
    id TEXT PRIMARY KEY,
    template TEXT NOT NULL CHECK (template IN ('hero_promo', 'featured_product', 'happy_hour', 'category_spotlight', 'brand_idle', 'uploaded')),
    image_key TEXT,
    image_focus TEXT,
    product_id TEXT REFERENCES product(id),
    discount_id TEXT REFERENCES discount(id),
    category_id TEXT REFERENCES category(id),
    headline_id TEXT,
    headline_en TEXT,
    sort INTEGER NOT NULL DEFAULT 0 CHECK (sort >= 0),
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    active_from INTEGER,
    active_to INTEGER,
    duration_seconds INTEGER NOT NULL DEFAULT 10 CHECK (duration_seconds > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (active_from IS NULL OR active_to IS NULL OR active_from < active_to)
);
CREATE INDEX IF NOT EXISTS idx_display_slide_sort ON display_slide(sort);
CREATE INDEX IF NOT EXISTS idx_display_slide_active ON display_slide(active);

-- +goose Down
DROP TABLE IF EXISTS display_slide;
