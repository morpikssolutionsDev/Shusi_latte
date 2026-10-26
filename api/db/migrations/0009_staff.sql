-- +goose Up
-- Staff phases 3-4: screen sessions, sub-bills, receipts.
-- TR-AUTH-08: Kitchen, Product Display and HRIS clock-in screens
-- are signed in once per day by an Outlet Admin into a
-- screen-mode session limited to that screen. Screen sessions
-- expire at the end of the Jakarta day they were created.
-- BR-WT-06: sub_bill for Split by item or equally.
-- TR-PRN-08: receipt_print rows for every print and reprint.
-- orders.created_by records the staff member who entered a
-- direct Order (Phase 4), so PlacedBy can show them.

CREATE TABLE IF NOT EXISTS screen_session (
    token TEXT PRIMARY KEY,
    crew_id TEXT NOT NULL REFERENCES crew(id),
    screen TEXT NOT NULL CHECK (screen IN ('display', 'kitchen', 'hris')),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_screen_session_crew ON screen_session(crew_id);

CREATE TABLE IF NOT EXISTS sub_bill (
    id TEXT PRIMARY KEY,
    bill_id TEXT NOT NULL REFERENCES bill(id),
    method TEXT NOT NULL CHECK (method IN ('by_item', 'equal')),
    share_index INTEGER NOT NULL,
    share_count INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'settling', 'closed', 'unpaid'))
);
CREATE INDEX IF NOT EXISTS idx_sub_bill_bill ON sub_bill(bill_id);

CREATE TABLE IF NOT EXISTS receipt_print (
    id TEXT PRIMARY KEY,
    bill_id TEXT NOT NULL REFERENCES bill(id),
    copy_type TEXT NOT NULL CHECK (copy_type IN ('customer', 'internal')),
    is_reprint INTEGER NOT NULL DEFAULT 0 CHECK (is_reprint IN (0, 1)),
    printed_by TEXT,
    printed_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_receipt_bill ON receipt_print(bill_id);

ALTER TABLE orders ADD COLUMN created_by TEXT;

-- +goose Down
DROP TABLE IF EXISTS receipt_print;
DROP TABLE IF EXISTS sub_bill;
DROP TABLE IF EXISTS screen_session;
