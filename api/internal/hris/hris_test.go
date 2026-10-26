package hris

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/ops"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE crew (id TEXT PRIMARY KEY, crew_id TEXT UNIQUE, full_name TEXT, role TEXT, image_key TEXT, pin_hash TEXT, failed_attempts INTEGER, locked_until INTEGER, active INTEGER, created_at INTEGER, updated_at INTEGER)`,
		`CREATE TABLE crew_station (crew_id TEXT, station TEXT, PRIMARY KEY (crew_id, station))`,
		`CREATE TABLE ops_setting (id TEXT PRIMARY KEY, claim_renotify_seconds INTEGER, claim_escalate_seconds INTEGER, pickup_reminder_seconds INTEGER, empty_bill_close_seconds INTEGER, auto_clock_out_grace_seconds INTEGER, created_at INTEGER, updated_at INTEGER)`,
		`INSERT INTO ops_setting VALUES ('default', 120, 300, 180, 1800, 7200, 0, 0)`,
		`CREATE TABLE audit_log (id TEXT PRIMARY KEY, at INTEGER, actor_crew_id TEXT, override_crew_id TEXT, action TEXT, entity TEXT, entity_id TEXT, before_json TEXT, after_json TEXT, reason TEXT)`,
		`CREATE TABLE employee (id TEXT PRIMARY KEY, crew_id TEXT UNIQUE, hire_date TEXT, phone TEXT, emergency_contact TEXT, employment_type TEXT, notes TEXT, created_at INTEGER, updated_at INTEGER)`,
		`CREATE TABLE shift (id TEXT PRIMARY KEY, crew_id TEXT, starts_at INTEGER, ends_at INTEGER)`,
		`CREATE TABLE attendance (id TEXT PRIMARY KEY, crew_id TEXT, clock_in_at INTEGER, clock_out_at INTEGER, auto_clocked_out INTEGER, corrected_by TEXT, correction_reason TEXT)`,
		`CREATE TABLE leave_request (id TEXT PRIMARY KEY, crew_id TEXT, from_date TEXT, to_date TEXT, type TEXT, reason TEXT, status TEXT, decided_by TEXT, decided_at INTEGER, created_at INTEGER)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	crewStore := crew.NewStore(db)
	// Create a crew record directly, bypassing PIN hashing test helpers.
	now := time.Now().UnixMilli()
	if _, err := db.Exec(`INSERT INTO crew (id, crew_id, full_name, role, pin_hash, failed_attempts, active, created_at, updated_at) VALUES ('c1', 'W01', 'Waiter One', 'crew', '', 0, 1, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO crew (id, crew_id, full_name, role, pin_hash, failed_attempts, active, created_at, updated_at) VALUES ('c2', 'MG01', 'Manager', 'management', '', 0, 1, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db, crewStore, ops.NewStore(db))
	store.HeldCash = func(string) int64 { return 0 }
	return store
}

func TestShiftOverlapAndLeaveAndAutoClockOut(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()
	start := now - 10*3600*1000
	end := now - 3*3600*1000
	if _, err := s.CreateShift(ShiftInput{CrewID: "W01", StartsAt: start, EndsAt: end}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateShift(ShiftInput{CrewID: "W01", StartsAt: start + 3600000, EndsAt: end + 3600000}); err == nil {
		t.Fatal("overlapping shift should fail")
	}
	// Clock in, then auto clock-out: shift ended 3h ago, grace 2h.
	att, err := s.ClockIn("W01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClockIn("W01"); err == nil {
		t.Fatal("second clock-in should fail")
	}
	auto := s.AutoClockOut()
	if len(auto) != 1 || !auto[0].AutoClockedOut {
		t.Fatalf("auto clock-out = %+v", auto)
	}
	if auto[0].ClockOutAt == nil || *auto[0].ClockOutAt != end+7200000 {
		t.Fatalf("auto clock-out time should be shift end + grace, got %+v", auto[0])
	}
	// Leave: create, approve, then clock-in is blocked on that date.
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(jakarta).Format("2006-01-02")
	lr, err := s.CreateLeave(LeaveInput{CrewID: "W01", FromDate: today, ToDate: today, Type: "annual"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideLeave("MG01", lr.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClockIn("W01"); err == nil {
		t.Fatal("clock-in on approved leave should fail")
	}
	_ = att
}
