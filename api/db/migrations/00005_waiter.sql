-- Waiter module: Claim transfers and pickup reminders (OP-03, OP-06, BR-WT-09..11, TR-SM-05).

-- +goose Up
CREATE TABLE claim_transfer (
    id           TEXT PRIMARY KEY,
    order_id     TEXT NOT NULL REFERENCES orders (id),
    from_crew_id TEXT NOT NULL REFERENCES crew (id),
    to_crew_id   TEXT NOT NULL REFERENCES crew (id),
    kind         TEXT NOT NULL CHECK (kind IN ('takeover', 'reassign')),
    by_crew_id   TEXT NOT NULL REFERENCES crew (id),
    at           INTEGER NOT NULL
);
CREATE INDEX claim_transfer_order ON claim_transfer (order_id);

ALTER TABLE order_item ADD COLUMN pickup_reminded_at INTEGER;

-- +goose Down
ALTER TABLE order_item DROP COLUMN pickup_reminded_at;
DROP TABLE claim_transfer;
