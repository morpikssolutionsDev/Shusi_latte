package crew

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Store persists crew records in SQLite, using migration 0004.
// PIN resets and role changes are audit-logged (TR-AUD-01), with
// actor set per-request from the authenticated crew.
type Store struct {
	// Actor is the human-readable Crew ID of the authenticated
	// user for this request, set per-request by handlers.
	// Empty means unauthenticated or system-initiated, and audit
	// rows record \"system\" in that case.
	Actor string
	db    *sql.DB
}

func NewStore(db *sql.DB) *Store {
	db.SetMaxOpenConns(1)
	return &Store{db: db}
}

func (s *Store) insertAudit(tx *sql.Tx, action, entity, entityID string, before, after any, reason *string) error {
	var beforeStr, afterStr *string
	if before != nil {
		b, _ := json.Marshal(before)
		v := string(b)
		beforeStr = &v
	}
	if after != nil {
		b, _ := json.Marshal(after)
		v := string(b)
		afterStr = &v
	}
	actor := s.Actor
	if actor == "" {
		actor = "system"
	}
	_, err := tx.Exec(
		`INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason)
		 VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		NewID(), time.Now().UnixMilli(), actor, action, entity, entityID,
		nullString(beforeStr), nullString(afterStr), nullString(reason),
	)
	return err
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func intPtr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

func (s *Store) loadStations(crewInternalID string) []string {
	rows, err := s.db.Query(`SELECT station FROM crew_station WHERE crew_id = ? ORDER BY station`, crewInternalID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err == nil {
			out = append(out, st)
		}
	}
	return out
}

func (s *Store) loadCrewByID(id string) (Crew, string, error) {
	var c Crew
	var imageKey sql.NullString
	var lockedUntil sql.NullInt64
	var active int
	var pinHash string
	err := s.db.QueryRow(
		`SELECT id, crew_id, full_name, role, image_key, pin_hash, failed_attempts, locked_until, active, created_at, updated_at FROM crew WHERE id = ?`,
		id,
	).Scan(&c.ID, &c.CrewID, &c.FullName, &c.Role, &imageKey, &pinHash, &c.FailedAttempts, &lockedUntil, &active, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return Crew{}, "", errors.New("crew not found")
	}
	c.ImageKey = strPtr(imageKey)
	c.LockedUntil = intPtr(lockedUntil)
	c.Active = active == 1
	c.Stations = s.loadStations(c.ID)
	return c, pinHash, nil
}

func (s *Store) GetCrew(id string) (Crew, error) {
	c, _, err := s.loadCrewByID(id)
	return c, err
}

// GetByCrewID looks a crew record up by its human-readable Crew ID,
// which is what sign-in uses (TR-AUTH-01).
func (s *Store) GetByCrewID(crewID string) (Crew, error) {
	var internalID string
	err := s.db.QueryRow(`SELECT id FROM crew WHERE crew_id = ?`, crewID).Scan(&internalID)
	if err != nil {
		return Crew{}, errors.New("crew not found")
	}
	return s.GetCrew(internalID)
}

// VerifyPINByCrewID verifies a PIN for the human-readable Crew ID,
// applying the same lockout rule as VerifyPIN.
func (s *Store) VerifyPINByCrewID(crewID, pin string) (Crew, bool, error) {
	c, err := s.GetByCrewID(crewID)
	if err != nil {
		return Crew{}, false, err
	}
	ok, err := s.VerifyPIN(c.ID, pin)
	return c, ok, err
}

// CrewCount returns the number of crew records, used for the
// first-user bootstrap rule in the auth middleware.
func (s *Store) CrewCount() int {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM crew`).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (s *Store) ListCrew(includeInactive bool) []Crew {
	query := `SELECT id FROM crew WHERE active = 1 ORDER BY full_name`
	if includeInactive {
		query = `SELECT id FROM crew ORDER BY full_name`
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return []Crew{}
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []Crew{}
	for _, id := range ids {
		if c, err := s.GetCrew(id); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// CreateCrew creates a crew record with a hashed PIN. The PIN is
// supplied once at creation and is never returned.
func (s *Store) CreateCrew(crewID, fullName, role string, stations []string, pin string, imageKey *string) (Crew, error) {
	if strings.TrimSpace(crewID) == "" || strings.TrimSpace(fullName) == "" {
		return Crew{}, errors.New("crew_id and full_name are required")
	}
	if err := ValidateRole(role); err != nil {
		return Crew{}, err
	}
	stations, err := ValidateStations(stations)
	if err != nil {
		return Crew{}, err
	}
	hash, err := HashPIN(pin)
	if err != nil {
		return Crew{}, err
	}
	now := time.Now().UnixMilli()
	c := Crew{ID: NewID(), CrewID: crewID, FullName: fullName, Role: role, ImageKey: imageKey, Stations: stations, Active: true, CreatedAt: now, UpdatedAt: now}
	tx, err := s.db.Begin()
	if err != nil {
		return Crew{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO crew (id, crew_id, full_name, role, image_key, pin_hash, failed_attempts, locked_until, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, NULL, 1, ?, ?)`,
		c.ID, c.CrewID, c.FullName, c.Role, nullString(c.ImageKey), hash, c.CreatedAt, c.UpdatedAt,
	); err != nil {
		return Crew{}, err
	}
	for _, st := range stations {
		if _, err := tx.Exec(`INSERT INTO crew_station (crew_id, station) VALUES (?, ?)`, c.ID, st); err != nil {
			return Crew{}, err
		}
	}
	if err := s.insertAudit(tx, "create", "crew", c.ID, nil, c, nil); err != nil {
		return Crew{}, err
	}
	return c, tx.Commit()
}

func (s *Store) UpdateCrew(id, fullName, role string, stations []string, active bool) (Crew, error) {
	if strings.TrimSpace(fullName) == "" {
		return Crew{}, errors.New("full_name is required")
	}
	if err := ValidateRole(role); err != nil {
		return Crew{}, err
	}
	stations, err := ValidateStations(stations)
	if err != nil {
		return Crew{}, err
	}
	before, _, err := s.loadCrewByID(id)
	if err != nil {
		return Crew{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Crew{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	activeInt := 0
	if active {
		activeInt = 1
	}
	if _, err := tx.Exec(`UPDATE crew SET full_name = ?, role = ?, active = ?, updated_at = ? WHERE id = ?`, fullName, role, activeInt, now, id); err != nil {
		return Crew{}, err
	}
	if _, err := tx.Exec(`DELETE FROM crew_station WHERE crew_id = ?`, id); err != nil {
		return Crew{}, err
	}
	for _, st := range stations {
		if _, err := tx.Exec(`INSERT INTO crew_station (crew_id, station) VALUES (?, ?)`, id, st); err != nil {
			return Crew{}, err
		}
	}
	after := before
	after.FullName = fullName
	after.Role = role
	after.Stations = stations
	after.Active = active
	after.UpdatedAt = now
	action := "update"
	if before.Role != role {
		action = "role_change"
	}
	if err := s.insertAudit(tx, action, "crew", id, before, after, nil); err != nil {
		return Crew{}, err
	}
	if err := tx.Commit(); err != nil {
		return Crew{}, err
	}
	return s.GetCrew(id)
}

// ResetPIN replaces a crew member's PIN hash, clears failed attempts
// and any lockout, and audit-logs the reset (TR-AUD-01). It does not
// return the PIN or the hash.
func (s *Store) ResetPIN(id, newPIN string) (Crew, error) {
	if err := ValidatePIN(newPIN); err != nil {
		return Crew{}, err
	}
	before, _, err := s.loadCrewByID(id)
	if err != nil {
		return Crew{}, err
	}
	hash, err := HashPIN(newPIN)
	if err != nil {
		return Crew{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Crew{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(`UPDATE crew SET pin_hash = ?, failed_attempts = 0, locked_until = NULL, updated_at = ? WHERE id = ?`, hash, now, id); err != nil {
		return Crew{}, err
	}
	after := before
	after.FailedAttempts = 0
	after.LockedUntil = nil
	after.UpdatedAt = now
	if err := s.insertAudit(tx, "pin_reset", "crew", id, before, after, nil); err != nil {
		return Crew{}, err
	}
	if err := tx.Commit(); err != nil {
		return Crew{}, err
	}
	return s.GetCrew(id)
}

// VerifyPIN checks a PIN for the auth phase, applying the lockout
// rule: a locked account always fails, a wrong PIN increments
// failed_attempts and locks for 15 minutes on the 5th consecutive
// failure, and a correct PIN resets the counter. It is not exposed
// as an HTTP endpoint in this step.
func (s *Store) VerifyPIN(id, pin string) (bool, error) {
	c, hash, err := s.loadCrewByID(id)
	if err != nil {
		return false, err
	}
	now := time.Now().UnixMilli()
	if c.Locked(now) {
		return false, nil
	}
	if !c.Active {
		return false, nil
	}
	if VerifyPINHash(pin, hash) {
		_, _ = s.db.Exec(`UPDATE crew SET failed_attempts = 0, locked_until = NULL WHERE id = ?`, id)
		return true, nil
	}
	failed := c.FailedAttempts + 1
	if failed >= maxFailedAttempts {
		lockedUntil := now + lockoutDuration.Milliseconds()
		_, _ = s.db.Exec(`UPDATE crew SET failed_attempts = ?, locked_until = ? WHERE id = ?`, failed, lockedUntil, id)
	} else {
		_, _ = s.db.Exec(`UPDATE crew SET failed_attempts = ? WHERE id = ?`, failed, id)
	}
	return false, nil
}
