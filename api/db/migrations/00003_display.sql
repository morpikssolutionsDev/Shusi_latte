-- Product Display playlist (BR-ADM-10, BR-PD-01..03, MOTION.md §6).

-- +goose Up
CREATE TABLE display_slide (
    id               TEXT PRIMARY KEY,
    template         TEXT NOT NULL CHECK (template IN ('hero_promo', 'featured_product', 'happy_hour', 'category_spotlight', 'brand_idle', 'uploaded')),
    sort             BIGINT NOT NULL DEFAULT 0,
    active           BIGINT NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    headline_id      TEXT NOT NULL DEFAULT '',
    headline_en      TEXT NOT NULL DEFAULT '',
    subline_id       TEXT NOT NULL DEFAULT '',
    subline_en       TEXT NOT NULL DEFAULT '',
    image_key        TEXT,
    video_key        TEXT, -- uploaded template: looping MP4/WebM, image_key is its poster
    image_focus_x    BIGINT NOT NULL DEFAULT 50 CHECK (image_focus_x BETWEEN 0 AND 100),
    image_focus_y    BIGINT NOT NULL DEFAULT 50 CHECK (image_focus_y BETWEEN 0 AND 100),
    product_id       TEXT REFERENCES product (id),
    discount_id      TEXT REFERENCES discount (id),
    category_ids     TEXT NOT NULL DEFAULT '[]', -- JSON array, Category Spotlight
    steam            BIGINT NOT NULL DEFAULT 0 CHECK (steam IN (0, 1)), -- steam overlay on hot drinks
    duration_seconds BIGINT NOT NULL DEFAULT 8 CHECK (duration_seconds BETWEEN 4 AND 60),
    active_from      BIGINT,
    active_to        BIGINT,
    created_at       BIGINT NOT NULL,
    updated_at       BIGINT NOT NULL
);
CREATE INDEX display_slide_sort ON display_slide (sort);

-- +goose Down
DROP TABLE display_slide;
