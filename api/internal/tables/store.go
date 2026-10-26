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

func (s *Store) slugTaken(slug, excludeID string) bool {
	var id string
	err := s.db.QueryRow(`SELECT id FROM dining_table WHERE slug = ?`, slug).Scan(&id)
	return err == nil && id != excludeID
}

func (s *Store) uniqueSlug(base, excludeID string) string {
	slug := base
	for i := 2; s.slugTaken(slug, excludeID); i++ {
		slug = base + "-" + itoa(i)
	}
	return slug
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + byte(n%10))}, digits...)
		n /= 10
	}
	return string(digits)
}

// EnsureSlug backfills a slug for a Table created before slugs
// existed (slug NULL/empty), from its label, and persists it.
func (s *Store) EnsureSlug(t Table) Table {
	if t.Slug != "" {
		return t
	}
	slug := s.uniqueSlug(Slugify(t.Label), t.ID)
	_, _ = s.db.Exec(`UPDATE dining_table SET slug = ?, updated_at = ? WHERE id = ?`, slug, t.UpdatedAt, t.ID)
	t.Slug = slug
	return t
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func (s *Store) ListTables(includeInactive bool) []Table {
	query := `SELECT id, label, slug, qr_token, active, created_at, updated_at FROM dining_table WHERE active = 1 ORDER BY label`
	if includeInactive {
		query = `SELECT id, label, slug, qr_token, active, created_at, updated_at FROM dining_table ORDER BY label`
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return []Table{}
	}
	out := []Table{}
	for rows.Next() {
		var t Table
		var active int
		var slug sql.NullString
		if err := rows.Scan(&t.ID, &t.Label, &slug, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt); err == nil {
			t.Active = active == 1
			t.Slug = slug.String
			out = append(out, t)
		}
	}
	rows.Close()
	for i := range out {
		out[i] = s.EnsureSlug(out[i])
	}
	return out
}

func (s *Store) GetTable(id string) (Table, error) {
	var t Table
	var active int
	var slug sql.NullString
	err := s.db.QueryRow(`SELECT id, label, slug, qr_token, active, created_at, updated_at FROM dining_table WHERE id = ?`, id).Scan(&t.ID, &t.Label, &slug, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Table{}, errors.New("table not found")
	}
	t.Active = active == 1
	t.Slug = slug.String
	return s.EnsureSlug(t), nil
}

func (s *Store) GetByToken(token string) (Table, error) {
	var t Table
	var active int
	var slug sql.NullString
	err := s.db.QueryRow(`SELECT id, label, slug, qr_token, active, created_at, updated_at FROM dining_table WHERE qr_token = ? AND active = 1`, token).Scan(&t.ID, &t.Label, &slug, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Table{}, errors.New("table not found for qr token")
	}
	t.Active = active == 1
	t.Slug = slug.String
	return s.EnsureSlug(t), nil
}

func (s *Store) GetBySlug(slug string) (Table, error) {
	var t Table
	var active int
	var slugCol sql.NullString
	err := s.db.QueryRow(`SELECT id, label, slug, qr_token, active, created_at, updated_at FROM dining_table WHERE slug = ? AND active = 1`, slug).Scan(&t.ID, &t.Label, &slugCol, &t.QRToken, &active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Table{}, errors.New("table not found for slug")
	}
	t.Active = active == 1
	t.Slug = slugCol.String
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
	t := Table{ID: NewID(), Label: label, Slug: s.uniqueSlug(Slugify(label), ""), QRToken: token, Active: true, CreatedAt: now, UpdatedAt: now}
	tx, err := s.db.Begin()
	if err != nil {
		return Table{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO dining_table (id, label, slug, qr_token, active, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`, t.ID, t.Label, t.Slug, t.QRToken, t.CreatedAt, t.UpdatedAt); err != nil {
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

// RegenerateSlug replaces a Table's slug with a fresh one derived
// from its label plus a random suffix. The old slug immediately
// stops resolving via GetBySlug, which is what invalidates a
// printed static QR code. Token regeneration alone no longer
// invalidates a printed QR, because the printed URL uses the
// slug, not the token.
func (s *Store) RegenerateSlug(id string) (Table, error) {
	before, err := s.GetTable(id)
	if err != nil {
		return Table{}, err
	}
	token, err := NewToken()
	if err != nil {
		return Table{}, err
	}
	// Static slug + random suffix, so the new URL is still
	// readable but the old printed QR stops resolving.
	slug := s.uniqueSlug(Slugify(before.Label)+token[:6], id)
	tx, err := s.db.Begin()
	if err != nil {
		return Table{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(`UPDATE dining_table SET slug = ?, updated_at = ? WHERE id = ?`, slug, now, id); err != nil {
		return Table{}, err
	}
	after := before
	after.Slug = slug
	after.UpdatedAt = now
	if err := s.insertAudit(tx, "table_slug_regenerated", "dining_table", id, before, after, nil); err != nil {
		return Table{}, err
	}
	if err := tx.Commit(); err != nil {
		return Table{}, err
	}
	return s.GetTable(id)
}

// RegenerateQRToken replaces a Table's qr_token with a fresh random
// token. The old token immediately stops resolving via GetByToken.
// Note: printed QR codes now encode the static slug URL, not the
// token, so token regeneration alone does not invalidate a printed
// QR — use RegenerateSlug for that.
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
