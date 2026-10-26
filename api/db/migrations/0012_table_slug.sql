-- +goose Up
-- Static, printable order URLs for Tables (customer order change):
-- a Table gets a human slug (for example "table3" for "Table 3"),
-- and its QR code prints {PUBLIC_URL}/order/{slug} instead of the
-- old random-token API path /public/tables/{qr_token}. The old
-- token path still resolves, for QR codes already printed.
ALTER TABLE dining_table ADD COLUMN slug TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_dining_table_slug ON dining_table(slug) WHERE slug IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_dining_table_slug;
