package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
)

// Store holds sessions in the session table and override tokens
// in memory. It wraps the crew store, which owns PIN verification
// and lockout.
type Store struct {
	db        *sql.DB
	crew      *crew.Store
	mu        sync.Mutex
	overrides map[string]OverrideToken
}

func NewStore(db *sql.DB, crewStore *crew.Store) *Store {
	db.SetMaxOpenConns(1)
	return &Store{db: db, crew: crewStore, overrides: map[string]OverrideToken{}}
}

func (s *Store) CrewStore() *crew.Store { return s.crew }

// Login verifies a Crew ID and PIN, applying the crew lockout, and
// creates a session on success. It returns the session token.
func (s *Store) Login(crewID, pin, deviceLabel string) (Session, error) {
	c, ok, err := s.crew.VerifyPINByCrewID(crewID, pin)
	if err != nil {
		return Session{}, errors.New("invalid crew id or pin")
	}
	if !ok {
		return Session{}, errors.New("invalid crew id or pin")
	}
	if !c.Active {
		return Session{}, errors.New("crew account is inactive")
	}
	now := time.Now()
	token := newToken()
	expires := SessionExpiry(c.Role, now)
	_, err = s.db.Exec(
		`INSERT INTO session (id, crew_id, device_label, created_at, last_seen_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		token, c.ID, deviceLabel, now.UnixMilli(), now.UnixMilli(), expires.UnixMilli(),
	)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, Crew: c, CreatedAt: now.UnixMilli(), LastSeen: now.UnixMilli(), ExpiresAt: expires.UnixMilli()}, nil
}

// ValidateSession loads a session by token, checks expiry, and
// updates last_seen. Management sessions slide their expiry by
// 30 minutes of inactivity; other sessions keep their 12-hour cap.
func (s *Store) ValidateSession(token string) (Session, error) {
	var internalCrewID, deviceLabel string
	var createdAt, lastSeen, expiresAt int64
	err := s.db.QueryRow(
		`SELECT id, crew_id, device_label, created_at, last_seen_at, expires_at FROM session WHERE id = ?`,
		token,
	).Scan(&token, &internalCrewID, &deviceLabel, &createdAt, &lastSeen, &expiresAt)
	if err != nil {
		return Session{}, errors.New("session not found")
	}
	now := time.Now()
	if now.UnixMilli() > expiresAt {
		_, _ = s.db.Exec(`DELETE FROM session WHERE id = ?`, token)
		return Session{}, errors.New("session expired")
	}
	c, err := s.crew.GetCrew(internalCrewID)
	if err != nil || !c.Active {
		return Session{}, errors.New("session crew not found or inactive")
	}
	newExpires := expiresAt
	if c.Role == crew.RoleManagement {
		newExpires = now.Add(managementInactivity).UnixMilli()
	}
	_, _ = s.db.Exec(`UPDATE session SET last_seen_at = ?, expires_at = ? WHERE id = ?`, now.UnixMilli(), newExpires, token)
	return Session{Token: token, Crew: c, CreatedAt: createdAt, LastSeen: now.UnixMilli(), ExpiresAt: newExpires}, nil
}

// Logout deletes a session. Deleting an unknown token is not an error.
func (s *Store) Logout(token string) {
	_, _ = s.db.Exec(`DELETE FROM session WHERE id = ?`, token)
}

// CreateOverride verifies the authoriser's PIN and role for an
// action, audit-logs the override, and returns a single-use token.
func (s *Store) CreateOverride(crewID, pin, action string) (OverrideToken, error) {
	allowed, ok := OverrideRoles(action)
	if !ok {
		return OverrideToken{}, errors.New("unknown override action")
	}
	c, verified, err := s.crew.VerifyPINByCrewID(crewID, pin)
	if err != nil || !verified {
		return OverrideToken{}, errors.New("invalid crew id or pin")
	}
	if !roleAllowed(c.Role, allowed) {
		return OverrideToken{}, errors.New("role does not permit this action")
	}
	token := OverrideToken{Token: newToken(), Action: action, Approver: c, ExpiresAt: time.Now().Add(overrideTTL)}
	s.mu.Lock()
	s.overrides[token.Token] = token
	s.mu.Unlock()
	after, _ := json.Marshal(map[string]string{"action": action, "approver_crew_id": c.CrewID})
	_, _ = s.db.Exec(
		`INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason)
		 VALUES (?, ?, ?, ?, 'pin_override', 'override', ?, NULL, ?, NULL)`,
		newToken(), time.Now().UnixMilli(), c.ID, c.ID, action, string(after),
	)
	return token, nil
}

// ConsumeOverride validates and single-uses an override token for
// an action. It returns the approver on success.
func (s *Store) ConsumeOverride(token, action string) (crew.Crew, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ot, ok := s.overrides[token]
	if !ok || ot.Action != action || time.Now().After(ot.ExpiresAt) {
		return crew.Crew{}, false
	}
	delete(s.overrides, token)
	return ot.Approver, true
}
