-- +goose Up
-- Ops settings: BR-ADM-13, TRD §4.4 ops_setting and §6.1
-- GET/PUT /settings/ops. Management sets the escalation timers;
-- the scheduler jobs that read them arrive with the Waiter,
-- Kitchen and HRIS phases (TRD §16: claim escalation every 15 s,
-- pickup reminder every 15 s, empty-Bill close every 1 min, auto
-- clock-out every 5 min). This step is the settings themselves,
-- not the scheduler. There is exactly one row, id 'default',
-- seeded with the spec defaults.

CREATE TABLE IF NOT EXISTS ops_setting (
    id TEXT PRIMARY KEY,
    claim_renotify_seconds INTEGER NOT NULL CHECK (claim_renotify_seconds > 0),
    claim_escalate_seconds INTEGER NOT NULL CHECK (claim_escalate_seconds > 0),
    pickup_reminder_seconds INTEGER NOT NULL CHECK (pickup_reminder_seconds > 0),
    empty_bill_close_seconds INTEGER NOT NULL CHECK (empty_bill_close_seconds > 0),
    auto_clock_out_grace_seconds INTEGER NOT NULL CHECK (auto_clock_out_grace_seconds > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (claim_renotify_seconds < claim_escalate_seconds)
);
INSERT OR IGNORE INTO ops_setting (id, claim_renotify_seconds, claim_escalate_seconds, pickup_reminder_seconds, empty_bill_close_seconds, auto_clock_out_grace_seconds, created_at, updated_at)
VALUES ('default', 120, 300, 180, 1800, 7200, CAST(strftime('%s', 'now') AS INTEGER) * 1000, CAST(strftime('%s', 'now') AS INTEGER) * 1000);

-- +goose Down
DROP TABLE IF EXISTS ops_setting;
