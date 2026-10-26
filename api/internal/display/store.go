package display

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Store persists display_slide rows in SQLite, using migration 0005.
// Slide changes are audit-logged (TR-AUD-01), with actor "system"
// until the auth phase supplies a Crew ID.
type Store struct {
	db *sql.DB
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
	actor := "system"
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

func scanSlide(row interface {
	Scan(...any) error
}) (Slide, error) {
	var sl Slide
	var imageKey, imageFocus, productID, discountID, categoryID, headlineID, headlineEN sql.NullString
	var activeFrom, activeTo sql.NullInt64
	var active int
	err := row.Scan(&sl.ID, &sl.Template, &imageKey, &imageFocus, &productID, &discountID, &categoryID, &headlineID, &headlineEN, &sl.Sort, &active, &activeFrom, &activeTo, &sl.DurationSeconds, &sl.CreatedAt, &sl.UpdatedAt)
	if err != nil {
		return Slide{}, err
	}
	sl.ImageKey = strPtr(imageKey)
	sl.ImageFocus = strPtr(imageFocus)
	sl.ProductID = strPtr(productID)
	sl.DiscountID = strPtr(discountID)
	sl.CategoryID = strPtr(categoryID)
	sl.HeadlineID = strPtr(headlineID)
	sl.HeadlineEN = strPtr(headlineEN)
	sl.Active = active == 1
	sl.ActiveFrom = intPtr(activeFrom)
	sl.ActiveTo = intPtr(activeTo)
	return sl, nil
}

const slideColumns = `id, template, image_key, image_focus, product_id, discount_id, category_id, headline_id, headline_en, sort, active, active_from, active_to, duration_seconds, created_at, updated_at`

// productOrderable mirrors the catalog rule: a Product is orderable
// when it is active and has at least one non-sold-out, non-archived
// Variant. A featured slide for a non-orderable Product is marked
// Sold Out (BR-PD-03).
func (s *Store) productOrderable(productID string) bool {
	var status string
	err := s.db.QueryRow(`SELECT status FROM product WHERE id = ?`, productID).Scan(&status)
	if err != nil || status != "active" {
		return false
	}
	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM variant WHERE product_id = ? AND sold_out = 0 AND archived_at IS NULL`, productID).Scan(&count)
	return err == nil && count > 0
}

func (s *Store) markSoldOut(slides []Slide) []Slide {
	for i := range slides {
		if slides[i].Template == TemplateFeaturedProduct && slides[i].ProductID != nil {
			slides[i].SoldOut = !s.productOrderable(*slides[i].ProductID)
		}
	}
	return slides
}

func (s *Store) ListSlides() []Slide {
	rows, err := s.db.Query(`SELECT ` + slideColumns + ` FROM display_slide ORDER BY sort ASC, created_at ASC`)
	if err != nil {
		return []Slide{}
	}
	var slides []Slide
	for rows.Next() {
		if sl, err := scanSlide(rows); err == nil {
			slides = append(slides, sl)
		}
	}
	rows.Close()
	return s.markSoldOut(slides)
}

// PublicSlides returns the live feed for the TV: active slides
// inside their schedule, in sort order, with Sold Out marking.
func (s *Store) PublicSlides(nowMillis int64) []Slide {
	rows, err := s.db.Query(
		`SELECT `+slideColumns+` FROM display_slide
		 WHERE active = 1
		   AND (active_from IS NULL OR active_from <= ?)
		   AND (active_to IS NULL OR active_to > ?)
		 ORDER BY sort ASC, created_at ASC`,
		nowMillis, nowMillis,
	)
	if err != nil {
		return []Slide{}
	}
	var slides []Slide
	for rows.Next() {
		if sl, err := scanSlide(rows); err == nil {
			slides = append(slides, sl)
		}
	}
	rows.Close()
	return s.markSoldOut(slides)
}

func (s *Store) GetSlide(id string) (Slide, error) {
	sl, err := scanSlide(s.db.QueryRow(`SELECT `+slideColumns+` FROM display_slide WHERE id = ?`, id))
	if err != nil {
		return Slide{}, errors.New("display slide not found")
	}
	sl.SoldOut = sl.Template == TemplateFeaturedProduct && sl.ProductID != nil && !s.productOrderable(*sl.ProductID)
	return sl, nil
}

func (s *Store) CreateSlide(sl Slide) (Slide, error) {
	if sl.DurationSeconds == 0 {
		sl.DurationSeconds = DefaultDurationSeconds
	}
	if err := Validate(sl); err != nil {
		return Slide{}, err
	}
	now := time.Now().UnixMilli()
	sl.ID = NewID()
	sl.CreatedAt = now
	sl.UpdatedAt = now
	tx, err := s.db.Begin()
	if err != nil {
		return Slide{}, err
	}
	defer tx.Rollback()
	activeInt := 0
	if sl.Active {
		activeInt = 1
	}
	if _, err := tx.Exec(
		`INSERT INTO display_slide (id, template, image_key, image_focus, product_id, discount_id, category_id, headline_id, headline_en, sort, active, active_from, active_to, duration_seconds, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sl.ID, sl.Template, nullString(sl.ImageKey), nullString(sl.ImageFocus), nullString(sl.ProductID), nullString(sl.DiscountID), nullString(sl.CategoryID), nullString(sl.HeadlineID), nullString(sl.HeadlineEN), sl.Sort, activeInt, nullInt(sl.ActiveFrom), nullInt(sl.ActiveTo), sl.DurationSeconds, sl.CreatedAt, sl.UpdatedAt,
	); err != nil {
		return Slide{}, err
	}
	if err := s.insertAudit(tx, "create", "display_slide", sl.ID, nil, sl, nil); err != nil {
		return Slide{}, err
	}
	return sl, tx.Commit()
}

func nullInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func (s *Store) UpdateSlide(id string, sl Slide) (Slide, error) {
	if sl.DurationSeconds == 0 {
		sl.DurationSeconds = DefaultDurationSeconds
	}
	if err := Validate(sl); err != nil {
		return Slide{}, err
	}
	before, err := s.GetSlide(id)
	if err != nil {
		return Slide{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Slide{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	activeInt := 0
	if sl.Active {
		activeInt = 1
	}
	if _, err := tx.Exec(
		`UPDATE display_slide SET template = ?, image_key = ?, image_focus = ?, product_id = ?, discount_id = ?, category_id = ?, headline_id = ?, headline_en = ?, sort = ?, active = ?, active_from = ?, active_to = ?, duration_seconds = ?, updated_at = ? WHERE id = ?`,
		sl.Template, nullString(sl.ImageKey), nullString(sl.ImageFocus), nullString(sl.ProductID), nullString(sl.DiscountID), nullString(sl.CategoryID), nullString(sl.HeadlineID), nullString(sl.HeadlineEN), sl.Sort, activeInt, nullInt(sl.ActiveFrom), nullInt(sl.ActiveTo), sl.DurationSeconds, now, id,
	); err != nil {
		return Slide{}, err
	}
	sl.ID = id
	sl.CreatedAt = before.CreatedAt
	sl.UpdatedAt = now
	if err := s.insertAudit(tx, "update", "display_slide", id, before, sl, nil); err != nil {
		return Slide{}, err
	}
	if err := tx.Commit(); err != nil {
		return Slide{}, err
	}
	return s.GetSlide(id)
}

func (s *Store) DeleteSlide(id string) error {
	before, err := s.GetSlide(id)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM display_slide WHERE id = ?`, id); err != nil {
		return err
	}
	if err := s.insertAudit(tx, "delete", "display_slide", id, before, nil, nil); err != nil {
		return err
	}
	return tx.Commit()
}
