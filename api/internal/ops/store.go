package ops

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Store persists the single ops_setting row in SQLite, using
// migration 0006. Ops changes are audit-logged, with actor
// "system" until the audit actor is threaded through from auth.
type Store struct {
	Actor string
	db    *sql.DB
}

func NewStore(db *sql.DB) *Store {
	db.SetMaxOpenConns(1)
	return &Store{db: db}
}

func (s *Store) actor() string {
	if s.Actor != "" {
		return s.Actor
	}
	return "system"
}

func (s *Store) GetSettings() (Settings, error) {
	var st Settings
	err := s.db.QueryRow(
		`SELECT claim_renotify_seconds, claim_escalate_seconds, pickup_reminder_seconds, empty_bill_close_seconds, auto_clock_out_grace_seconds, created_at, updated_at FROM ops_setting WHERE id = 'default'`,
	).Scan(&st.ClaimRenotifySeconds, &st.ClaimEscalateSeconds, &st.PickupReminderSeconds, &st.EmptyBillCloseSeconds, &st.AutoClockOutGraceSeconds, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return Settings{}, errors.New("ops settings not found")
	}
	return st, nil
}

func (s *Store) UpdateSettings(next Settings) (Settings, error) {
	if err := Validate(next); err != nil {
		return Settings{}, err
	}
	before, err := s.GetSettings()
	if err != nil {
		return Settings{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(
		`UPDATE ops_setting SET claim_renotify_seconds = ?, claim_escalate_seconds = ?, pickup_reminder_seconds = ?, empty_bill_close_seconds = ?, auto_clock_out_grace_seconds = ?, updated_at = ? WHERE id = 'default'`,
		next.ClaimRenotifySeconds, next.ClaimEscalateSeconds, next.PickupReminderSeconds, next.EmptyBillCloseSeconds, next.AutoClockOutGraceSeconds, now,
	); err != nil {
		return Settings{}, err
	}
	next.CreatedAt = before.CreatedAt
	next.UpdatedAt = now
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(next)
	beforeStr, afterStr := string(beforeJSON), string(afterJSON)
	if _, err := tx.Exec(
		`INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason)
		 VALUES (?, ?, ?, NULL, 'update', 'ops_setting', 'default', ?, ?, NULL)`,
		NewID(), now, s.actor(), beforeStr, afterStr,
	); err != nil {
		return Settings{}, err
	}
	return next, tx.Commit()
}
