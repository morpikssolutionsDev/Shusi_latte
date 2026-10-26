-- +goose Up
-- Tables and QR codes: BR-ADM-08, TRD §4.3 dining_table.
-- qr_token is random and regenerable; regenerating it invalidates
-- printed QR codes for that Table (TR-AUTH-07). A Table has at most
-- one open Bill (BR-R-01); bill occupancy is wired in the ordering
-- phase, so this migration only defines the Table itself.

CREATE TABLE IF NOT EXISTS dining_table (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL UNIQUE,
    qr_token TEXT NOT NULL UNIQUE,
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_dining_table_qr_token ON dining_table(qr_token);
CREATE INDEX IF NOT EXISTS idx_dining_table_active ON dining_table(active);

-- +goose Down
DROP TABLE IF EXISTS dining_table;
