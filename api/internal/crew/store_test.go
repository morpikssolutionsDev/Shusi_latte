package crew

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	schema := `
CREATE TABLE crew (id TEXT PRIMARY KEY, crew_id TEXT NOT NULL UNIQUE, full_name TEXT NOT NULL, role TEXT NOT NULL, image_key TEXT, pin_hash TEXT NOT NULL, failed_attempts INTEGER NOT NULL DEFAULT 0, locked_until INTEGER, active INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE crew_station (crew_id TEXT NOT NULL REFERENCES crew(id), station TEXT NOT NULL, PRIMARY KEY (crew_id, station));
CREATE TABLE audit_log (id TEXT PRIMARY KEY, at INTEGER NOT NULL, actor_crew_id TEXT, override_crew_id TEXT, action TEXT NOT NULL, entity TEXT NOT NULL, entity_id TEXT NOT NULL, before_json TEXT, after_json TEXT, reason TEXT);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestVerifyPINLockout(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	s := NewStore(db)
	c, err := s.CreateCrew("W01", "Andi", RoleCrew, []string{StationWaiter}, "123456", nil)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := s.VerifyPIN(c.ID, "123456")
	if err != nil || !ok {
		t.Fatalf("correct PIN should verify, ok=%v err=%v", ok, err)
	}
	for i := 0; i < 5; i++ {
		ok, err := s.VerifyPIN(c.ID, "000000")
		if err != nil || ok {
			t.Fatalf("wrong PIN attempt %d should fail", i+1)
		}
	}
	after, err := s.GetCrew(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.FailedAttempts != 5 || after.LockedUntil == nil {
		t.Fatalf("after 5 failures should lock, got %+v", after)
	}
	ok, _ = s.VerifyPIN(c.ID, "123456")
	if ok {
		t.Fatal("locked account should not verify even with correct PIN")
	}
	if _, err := s.ResetPIN(c.ID, "654321"); err != nil {
		t.Fatal(err)
	}
	ok, err = s.VerifyPIN(c.ID, "654321")
	if err != nil || !ok {
		t.Fatalf("PIN after reset should verify, ok=%v err=%v", ok, err)
	}
}
