package ops

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestValidate(t *testing.T) {
	if err := Validate(Defaults()); err != nil {
		t.Fatal(err)
	}
	bad := Defaults()
	bad.ClaimRenotifySeconds = 0
	if err := Validate(bad); err == nil {
		t.Fatal("zero timer should fail")
	}
	bad = Defaults()
	bad.ClaimRenotifySeconds = 300
	bad.ClaimEscalateSeconds = 300
	if err := Validate(bad); err == nil {
		t.Fatal("renotify equal to escalate should fail")
	}
}

func TestStoreDefaultsAndUpdate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := `
CREATE TABLE ops_setting (id TEXT PRIMARY KEY, claim_renotify_seconds INTEGER NOT NULL, claim_escalate_seconds INTEGER NOT NULL, pickup_reminder_seconds INTEGER NOT NULL, empty_bill_close_seconds INTEGER NOT NULL, auto_clock_out_grace_seconds INTEGER NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE audit_log (id TEXT PRIMARY KEY, at INTEGER NOT NULL, actor_crew_id TEXT, override_crew_id TEXT, action TEXT NOT NULL, entity TEXT NOT NULL, entity_id TEXT NOT NULL, before_json TEXT, after_json TEXT, reason TEXT);
INSERT INTO ops_setting VALUES ('default', 120, 300, 180, 1800, 7200, 0, 0);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	got, err := s.GetSettings()
	if err != nil || got.ClaimRenotifySeconds != 120 || got.AutoClockOutGraceSeconds != 7200 {
		t.Fatalf("defaults = %+v, err = %v", got, err)
	}
	next := Defaults()
	next.PickupReminderSeconds = 240
	updated, err := s.UpdateSettings(next)
	if err != nil || updated.PickupReminderSeconds != 240 {
		t.Fatalf("update = %+v, err = %v", updated, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE entity = 'ops_setting'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ops update should be audit-logged, count = %d, err = %v", count, err)
	}
}
