package tables

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Store persists dining_table rows in SQLite, using migration 0003.
// Table changes are audit-logged (TR-AUD-01), with actor set per-request from the authenticated crew.
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

func (s *Store) ListTables(includeInactive bool) []Table {
	query := `SELECT id, label, qr_token, active, created_at, updated_at FROM dining_table WHERE active = 1 ORDER BY label`
	if includeInactive {
		query = `SELECT id, label, qr_token, active, created_at, updated_at FROM dining_table ORDER BY label`
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return []Table{}
	}
	defer rows.Close()
	out := []Table{}
	for rows.Next() {
		var t Table
		var active int
		if err := rows.Scan(&t.ID, &t.Label, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt); err == nil {
			t.Active = active == 1
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) GetTable(id string) (Table, error) {
	var t Table
	var active int
	err := s.db.QueryRow(`SELECT id, label, qr_token, active, created_at, updated_at FROM dining_table WHERE id = ?`, id).Scan(&t.ID, &t.Label, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Table{}, errors.New("table not found")
	}
	t.Active = active == 1
	return t, nil
}

func (s *Store) GetByToken(token string) (Table, error) {
	var t Table
	var active int
	err := s.db.QueryRow(`SELECT id, label, qr_token, active, created_at, updated_at FROM dining_table WHERE qr_token = ? AND active = 1`, token).Scan(&t.ID, &t.Label, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Table{}, errors.New("table not found for qr token")
	}
	t.Active = active == 1
	return t, nil
}

func (s *Store) CreateTable(label string) (Table, error) {
	if err := ValidateLabel(label); err != nil {
		return Table{}, err
	}
	token, err := NewToken()
	if err != nil {
		return Table{}, err
	}
	now := time.Now().UnixMilli()
	t := Table{ID: NewID(), Label: label, QRToken: token, Active: true, CreatedAt: now, UpdatedAt: now}
	tx, err := s.db.Begin()
	if err != nil {
		return Table{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO dining_table (id, label, qr_token, active, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`, t.ID, t.Label, t.QRToken, t.CreatedAt, t.UpdatedAt); err != nil {
		return Table{}, err
	}
	if err := s.insertAudit(tx, "create", "dining_table", t.ID, nil, t, nil); err != nil {
		return Table{}, err
	}
	return t, tx.Commit()
}

func (s *Store) UpdateTable(id, label string, active bool) (Table, error) {
	if err := ValidateLabel(label); err != nil {
		return Table{}, err
	}
	before, err := s.GetTable(id)
	if err != nil {
		return Table{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Table{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	activeInt := 0
	if active {
		activeInt = 1
	}
	if _, err := tx.Exec(`UPDATE dining_table SET label = ?, active = ?, updated_at = ? WHERE id = ?`, label, activeInt, now, id); err != nil {
		return Table{}, err
	}
	after := before
	after.Label = label
	after.Active = active
	after.UpdatedAt = now
	if err := s.insertAudit(tx, "update", "dining_table", id, before, after, nil); err != nil {
		return Table{}, err
	}
	if err := tx.Commit(); err != nil {
		return Table{}, err
	}
	return s.GetTable(id)
}

// RegenerateQRToken replaces a Table's qr_token with a fresh random
// token. The old token immediately stops resolving via GetByToken,
// which is what invalidates printed QR codes (TR-AUTH-07).
func (s *Store) RegenerateQRToken(id string) (Table, error) {
	before, err := s.GetTable(id)
	if err != nil {
		return Table{}, err
	}
	token, err := NewToken()
	if err != nil {
		return Table{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Table{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(`UPDATE dining_table SET qr_token = ?, updated_at = ? WHERE id = ?`, token, now, id); err != nil {
		return Table{}, err
	}
	after := before
	after.QRToken = token
	after.UpdatedAt = now
	if err := s.insertAudit(tx, "table_qr_regenerated", "dining_table", id, before, after, nil); err != nil {
		return Table{}, err
	}
	if err := tx.Commit(); err != nil {
		return Table{}, err
	}
	return s.GetTable(id)
}
