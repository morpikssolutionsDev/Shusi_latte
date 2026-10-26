package pricing

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// Store persists pricing settings and discounts in SQLite, using the
// tables from migration 0002_pricing.sql. Price, tax and discount
// changes are audit-logged (TR-AUD-01), with actor "system" until the
// auth phase supplies a Crew ID.
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

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

func (s *Store) GetSettings() (Settings, error) {
	var st Settings
	var serviceBP, taxBP int
	var mode string
	var cap int64
	err := s.db.QueryRow(`SELECT service_rate_bp, tax_rate_bp, price_mode FROM tax_setting WHERE id = 'default'`).Scan(&serviceBP, &taxBP, &mode)
	if err != nil {
		return Settings{}, err
	}
	if err := s.db.QueryRow(`SELECT cap_amount FROM refund_setting WHERE id = 'default'`).Scan(&cap); err != nil {
		return Settings{}, err
	}
	st.ServiceRateBP = serviceBP
	st.TaxRateBP = taxBP
	st.PriceMode = mode
	st.RefundCap = cap
	return st, nil
}

func (s *Store) UpdateSettings(st Settings) (Settings, error) {
	if st.ServiceRateBP < 0 || st.TaxRateBP < 0 || st.RefundCap < 0 {
		return Settings{}, errors.New("rates and refund cap must be >= 0")
	}
	if st.PriceMode != ModeExclusive && st.PriceMode != ModeInclusive {
		return Settings{}, errors.New("price_mode must be exclusive or inclusive")
	}
	before, _ := s.GetSettings()
	tx, err := s.db.Begin()
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE tax_setting SET service_rate_bp = ?, tax_rate_bp = ?, price_mode = ? WHERE id = 'default'`, st.ServiceRateBP, st.TaxRateBP, st.PriceMode); err != nil {
		return Settings{}, err
	}
	if _, err := tx.Exec(`UPDATE refund_setting SET cap_amount = ? WHERE id = 'default'`, st.RefundCap); err != nil {
		return Settings{}, err
	}
	if err := s.insertAudit(tx, "pricing_settings_change", "tax_setting", "default", before, st, nil); err != nil {
		return Settings{}, err
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, err
	}
	return s.GetSettings()
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

func nullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
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

func (s *Store) ListDiscounts() []Discount {
	rows, err := s.db.Query(`SELECT id, name, kind, value, scope, trigger, days, start_time, end_time, valid_from, valid_to, target_kind, target_id, active, created_at, updated_at FROM discount ORDER BY name`)
	if err != nil {
		return []Discount{}
	}
	defer rows.Close()
	out := []Discount{}
	for rows.Next() {
		var d Discount
		var days, start, end, targetKind, targetID sql.NullString
		var validFrom, validTo sql.NullInt64
		var active int
		if err := rows.Scan(&d.ID, &d.Name, &d.Kind, &d.Value, &d.Scope, &d.Trigger, &days, &start, &end, &validFrom, &validTo, &targetKind, &targetID, &active, &d.CreatedAt, &d.UpdatedAt); err == nil {
			d.Days = strPtr(days)
			d.StartTime = strPtr(start)
			d.EndTime = strPtr(end)
			d.ValidFrom = intPtr(validFrom)
			d.ValidTo = intPtr(validTo)
			d.TargetKind = strPtr(targetKind)
			d.TargetID = strPtr(targetID)
			d.Active = active == 1
			out = append(out, d)
		}
	}
	return out
}

func (s *Store) CreateDiscount(d Discount) (Discount, error) {
	if err := ValidateDiscount(d); err != nil {
		return Discount{}, err
	}
	if d.ID == "" {
		d.ID = NewID()
	}
	d.Active = true
	now := time.Now().UnixMilli()
	d.CreatedAt = now
	d.UpdatedAt = now
	tx, err := s.db.Begin()
	if err != nil {
		return Discount{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO discount (id, name, kind, value, scope, trigger, days, start_time, end_time, valid_from, valid_to, target_kind, target_id, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		d.ID, d.Name, d.Kind, d.Value, d.Scope, d.Trigger,
		nullString(d.Days), nullString(d.StartTime), nullString(d.EndTime),
		nullInt64(d.ValidFrom), nullInt64(d.ValidTo),
		nullString(d.TargetKind), nullString(d.TargetID), d.CreatedAt, d.UpdatedAt,
	); err != nil {
		return Discount{}, err
	}
	if err := s.insertAudit(tx, "discount_change", "discount", d.ID, nil, d, nil); err != nil {
		return Discount{}, err
	}
	return d, tx.Commit()
}

func (s *Store) GetDiscount(id string) (Discount, error) {
	var d Discount
	var days, start, end, targetKind, targetID sql.NullString
	var validFrom, validTo sql.NullInt64
	var active int
	err := s.db.QueryRow(
		`SELECT id, name, kind, value, scope, trigger, days, start_time, end_time, valid_from, valid_to, target_kind, target_id, active, created_at, updated_at FROM discount WHERE id = ?`,
		id,
	).Scan(&d.ID, &d.Name, &d.Kind, &d.Value, &d.Scope, &d.Trigger, &days, &start, &end, &validFrom, &validTo, &targetKind, &targetID, &active, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Discount{}, errors.New("discount not found")
	}
	d.Days = strPtr(days)
	d.StartTime = strPtr(start)
	d.EndTime = strPtr(end)
	d.ValidFrom = intPtr(validFrom)
	d.ValidTo = intPtr(validTo)
	d.TargetKind = strPtr(targetKind)
	d.TargetID = strPtr(targetID)
	d.Active = active == 1
	return d, nil
}
