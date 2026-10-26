package auth

import (
	"database/sql"
	"testing"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
	_ "modernc.org/sqlite"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	schema := `
CREATE TABLE crew (id TEXT PRIMARY KEY, crew_id TEXT NOT NULL UNIQUE, full_name TEXT NOT NULL, role TEXT NOT NULL, image_key TEXT, pin_hash TEXT NOT NULL, failed_attempts INTEGER NOT NULL DEFAULT 0, locked_until INTEGER, active INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE crew_station (crew_id TEXT NOT NULL REFERENCES crew(id), station TEXT NOT NULL, PRIMARY KEY (crew_id, station));
CREATE TABLE session (id TEXT PRIMARY KEY, crew_id TEXT NOT NULL REFERENCES crew(id), device_label TEXT, created_at INTEGER NOT NULL, last_seen_at INTEGER NOT NULL, expires_at INTEGER NOT NULL);
CREATE TABLE audit_log (id TEXT PRIMARY KEY, at INTEGER NOT NULL, actor_crew_id TEXT, override_crew_id TEXT, action TEXT NOT NULL, entity TEXT NOT NULL, entity_id TEXT NOT NULL, before_json TEXT, after_json TEXT, reason TEXT);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	cs := crew.NewStore(db)
	if _, err := cs.CreateCrew("MG01", "Manager", crew.RoleManagement, nil, "123456", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CreateCrew("W01", "Waiter", crew.RoleCrew, []string{crew.StationWaiter}, "654321", nil); err != nil {
		t.Fatal(err)
	}
	return NewStore(db, cs)
}

func TestLoginValidateLogout(t *testing.T) {
	s := testStore(t)
	sess, err := s.Login("MG01", "123456", "tablet")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ValidateSession(sess.Token)
	if err != nil || got.Crew.CrewID != "MG01" {
		t.Fatalf("session should validate, got %+v err %v", got, err)
	}
	s.Logout(sess.Token)
	if _, err := s.ValidateSession(sess.Token); err == nil {
		t.Fatal("logged out session should not validate")
	}
	if _, err := s.Login("MG01", "000000", ""); err == nil {
		t.Fatal("wrong PIN should not log in")
	}
}

func TestOverrideRolesAndSingleUse(t *testing.T) {
	s := testStore(t)
	// Waiter cannot approve a manual discount.
	if _, err := s.CreateOverride("W01", "654321", "manual_discount"); err == nil {
		t.Fatal("waiter should not approve manual discount")
	}
	// Manager can, and the token is single-use.
	token, err := s.CreateOverride("MG01", "123456", "manual_discount")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.ConsumeOverride(token.Token, "manual_discount"); !ok {
		t.Fatal("override token should consume once")
	}
	if _, ok := s.ConsumeOverride(token.Token, "manual_discount"); ok {
		t.Fatal("override token should not consume twice")
	}
	if _, err := s.CreateOverride("MG01", "123456", "unknown_action"); err == nil {
		t.Fatal("unknown override action should fail")
	}
}

func TestSessionExpiryByRole(t *testing.T) {
	now := time.Now()
	if got := SessionExpiry(crew.RoleManagement, now); got.Sub(now) != managementInactivity {
		t.Fatalf("management expiry should be 30m, got %v", got.Sub(now))
	}
	if got := SessionExpiry(crew.RoleCrew, now); got.Sub(now) != otherSessionTTL {
		t.Fatalf("crew expiry should be 12h, got %v", got.Sub(now))
	}
}

func TestIPLimiter(t *testing.T) {
	l := NewIPLimiter()
	now := time.Now()
	for i := 0; i < ipMaxFailures; i++ {
		l.RecordFailure("1.2.3.4", now)
	}
	if !l.Blocked("1.2.3.4", now) {
		t.Fatal("IP should be blocked after limit")
	}
	if l.Blocked("5.6.7.8", now) {
		t.Fatal("other IP should not be blocked")
	}
	l.Clear("1.2.3.4")
	if l.Blocked("1.2.3.4", now) {
		t.Fatal("cleared IP should not be blocked")
	}
}
