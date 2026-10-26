package catalog

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// SQLiteService is the Phase 1b catalog store. It uses the schema from
// api/db/migrations/0001_catalog.sql, with money as INTEGER whole Rupiah
// and timestamps as INTEGER UTC Unix milliseconds (TRD §4). It opens the
// database with a single connection, matching TR-ARC-02: one process is
// the only writer to the SQLite file.
type SQLiteService struct {
	db *sql.DB
}

func NewSQLiteService(db *sql.DB) *SQLiteService {
	db.SetMaxOpenConns(1)
	return &SQLiteService{db: db}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool { return i != 0 }

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

func (s *SQLiteService) insertAudit(tx *sql.Tx, action, entity, entityID string, before, after any, reason *string) error {
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
		NewID(), NowMillis(), actor, action, entity, entityID,
		nullString(beforeStr), nullString(afterStr), nullString(reason),
	)
	return err
}

func (s *SQLiteService) SeedBaseline() {
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO prep_area (id, name, parallel_capacity) VALUES ('prep-kitchen', 'Kitchen', 1), ('prep-bar', 'Bar', 1)`)
	tags := []struct {
		id, code, nameID, nameEN string
		sort                     int
	}{
		{"tag-dairy", "dairy", "Susu / Laktosa", "Dairy / Lactose", 1},
		{"tag-gluten", "gluten", "Gluten", "Gluten", 2},
		{"tag-egg", "egg", "Telur", "Egg", 3},
		{"tag-fish", "fish", "Ikan", "Fish", 4},
		{"tag-raw-fish", "raw_fish", "Ikan Mentah", "Raw Fish", 5},
		{"tag-shellfish", "shellfish", "Kerang-kerangan", "Shellfish", 6},
		{"tag-nuts", "nuts", "Kacang", "Nuts", 7},
		{"tag-peanut", "peanut", "Kacang Tanah", "Peanut", 8},
		{"tag-soy", "soy", "Kedelai", "Soy", 9},
		{"tag-sesame", "sesame", "Wijen", "Sesame", 10},
		{"tag-pork", "pork", "Babi", "Pork", 11},
		{"tag-alcohol", "alcohol", "Alkohol", "Alcohol", 12},
		{"tag-spicy", "spicy", "Pedas", "Spicy", 13},
	}
	for _, t := range tags {
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO dietary_tag (id, code, name_id, name_en, sort, active) VALUES (?, ?, ?, ?, ?, 1)`,
			t.id, t.code, t.nameID, t.nameEN, t.sort,
		)
	}
}

func (s *SQLiteService) ListPrepAreas() []PrepArea {
	rows, err := s.db.Query(`SELECT id, name, parallel_capacity FROM prep_area ORDER BY name`)
	if err != nil {
		return []PrepArea{}
	}
	defer rows.Close()
	out := []PrepArea{}
	for rows.Next() {
		var p PrepArea
		if err := rows.Scan(&p.ID, &p.Name, &p.ParallelCapacity); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *SQLiteService) CreatePrepArea(p PrepArea) (PrepArea, error) {
	if p.Name == "" {
		return PrepArea{}, errors.New("prep area name is required")
	}
	if p.ID == "" {
		p.ID = NewID()
	}
	if p.ParallelCapacity < 1 {
		p.ParallelCapacity = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PrepArea{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO prep_area (id, name, parallel_capacity) VALUES (?, ?, ?)`, p.ID, p.Name, p.ParallelCapacity); err != nil {
		return PrepArea{}, err
	}
	if err := s.insertAudit(tx, "create", "prep_area", p.ID, nil, p, nil); err != nil {
		return PrepArea{}, err
	}
	return p, tx.Commit()
}

func (s *SQLiteService) ListCategories() []Category {
	rows, err := s.db.Query(`SELECT id, name_id, name_en, sort, prep_area_id FROM category ORDER BY sort`)
	if err != nil {
		return []Category{}
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.NameID, &c.NameEN, &c.Sort, &c.PrepAreaID); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func (s *SQLiteService) CreateCategory(c Category) (Category, error) {
	if c.NameID == "" || c.NameEN == "" {
		return Category{}, errors.New("category name_id and name_en are required")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM prep_area WHERE id = ?`, c.PrepAreaID).Scan(&exists); err != nil || exists == 0 {
		return Category{}, errors.New("prep_area_id not found")
	}
	if c.ID == "" {
		c.ID = NewID()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Category{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO category (id, name_id, name_en, sort, prep_area_id) VALUES (?, ?, ?, ?, ?)`, c.ID, c.NameID, c.NameEN, c.Sort, c.PrepAreaID); err != nil {
		return Category{}, err
	}
	if err := s.insertAudit(tx, "create", "category", c.ID, nil, c, nil); err != nil {
		return Category{}, err
	}
	return c, tx.Commit()
}

func (s *SQLiteService) ListDietaryTags() []DietaryTag {
	rows, err := s.db.Query(`SELECT id, code, name_id, name_en, icon, sort, active FROM dietary_tag ORDER BY sort`)
	if err != nil {
		return []DietaryTag{}
	}
	defer rows.Close()
	out := []DietaryTag{}
	for rows.Next() {
		var t DietaryTag
		var icon sql.NullString
		var active int
		if err := rows.Scan(&t.ID, &t.Code, &t.NameID, &t.NameEN, &icon, &t.Sort, &active); err == nil {
			t.Icon = strPtr(icon)
			t.Active = intToBool(active)
			out = append(out, t)
		}
	}
	return out
}

func (s *SQLiteService) CreateDietaryTag(t DietaryTag) (DietaryTag, error) {
	if t.Code == "" || t.NameID == "" || t.NameEN == "" {
		return DietaryTag{}, errors.New("dietary tag code, name_id and name_en are required")
	}
	if t.ID == "" {
		t.ID = NewID()
	}
	t.Active = true
	tx, err := s.db.Begin()
	if err != nil {
		return DietaryTag{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO dietary_tag (id, code, name_id, name_en, icon, sort, active) VALUES (?, ?, ?, ?, ?, ?, 1)`, t.ID, t.Code, t.NameID, t.NameEN, nullString(t.Icon), t.Sort); err != nil {
		return DietaryTag{}, err
	}
	if err := s.insertAudit(tx, "create", "dietary_tag", t.ID, nil, t, nil); err != nil {
		return DietaryTag{}, err
	}
	return t, tx.Commit()
}

func (s *SQLiteService) loadProduct(id string) (Product, error) {
	var p Product
	var imageKey sql.NullString
	var archivedAt sql.NullInt64
	err := s.db.QueryRow(
		`SELECT id, category_id, name_id, name_en, description_id, description_en, image_key, image_focus, prep_time_seconds, status, sort, created_at, updated_at, archived_at FROM product WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.CategoryID, &p.NameID, &p.NameEN, &p.DescriptionID, &p.DescriptionEN, &imageKey, &p.ImageFocus, &p.PrepTimeSeconds, &p.Status, &p.Sort, &p.CreatedAt, &p.UpdatedAt, &archivedAt)
	if err != nil {
		return Product{}, errors.New("product not found")
	}
	p.ImageKey = strPtr(imageKey)
	p.ArchivedAt = intPtr(archivedAt)

	vrows, err := s.db.Query(`SELECT id, product_id, name_id, name_en, price, is_default, sort, sold_out, sold_out_at, sold_out_by, sold_out_source, archived_at FROM variant WHERE product_id = ? ORDER BY sort`, id)
	if err == nil {
		for vrows.Next() {
			var v Variant
			var soldOutAt, vArchived sql.NullInt64
			var soldOutBy, soldOutSource sql.NullString
			var isDefault, soldOut int
			if err := vrows.Scan(&v.ID, &v.ProductID, &v.NameID, &v.NameEN, &v.Price, &isDefault, &v.Sort, &soldOut, &soldOutAt, &soldOutBy, &soldOutSource, &vArchived); err == nil {
				v.IsDefault = intToBool(isDefault)
				v.SoldOut = intToBool(soldOut)
				v.SoldOutAt = intPtr(soldOutAt)
				v.SoldOutBy = strPtr(soldOutBy)
				v.SoldOutSource = strPtr(soldOutSource)
				v.ArchivedAt = intPtr(vArchived)
				p.Variants = append(p.Variants, v)
			}
		}
		vrows.Close()
	}
	grows, err := s.db.Query(`SELECT id, product_id, name_id, name_en, min_select, max_select, sort FROM modifier_group WHERE product_id = ? ORDER BY sort`, id)
	if err == nil {
		var groups []ModifierGroup
		for grows.Next() {
			var g ModifierGroup
			if err := grows.Scan(&g.ID, &g.ProductID, &g.NameID, &g.NameEN, &g.MinSelect, &g.MaxSelect, &g.Sort); err == nil {
				groups = append(groups, g)
			}
		}
		grows.Close()
		for _, g := range groups {
			mrows, err := s.db.Query(`SELECT id, modifier_group_id, name_id, name_en, price, sort, archived_at FROM modifier WHERE modifier_group_id = ? ORDER BY sort`, g.ID)
			if err != nil {
				p.ModifierGroups = append(p.ModifierGroups, g)
				continue
			}
			var mods []Modifier
			for mrows.Next() {
				var m Modifier
				var mArchived sql.NullInt64
				if err := mrows.Scan(&m.ID, &m.ModifierGroupID, &m.NameID, &m.NameEN, &m.Price, &m.Sort, &mArchived); err == nil {
					m.ArchivedAt = intPtr(mArchived)
					mods = append(mods, m)
				}
			}
			mrows.Close()
			for i := range mods {
				mods[i].DietaryTagIDs = s.loadModifierTags(mods[i].ID)
			}
			g.Modifiers = mods
			p.ModifierGroups = append(p.ModifierGroups, g)
		}
	}

	trows, err := s.db.Query(`SELECT dietary_tag_id FROM product_dietary_tag WHERE product_id = ?`, id)
	if err == nil {
		defer trows.Close()
		for trows.Next() {
			var tid string
			if err := trows.Scan(&tid); err == nil {
				p.DietaryTagIDs = append(p.DietaryTagIDs, tid)
			}
		}
	}
	return p, nil
}

func (s *SQLiteService) loadModifierTags(modifierID string) []string {
	rows, err := s.db.Query(`SELECT dietary_tag_id FROM modifier_dietary_tag WHERE modifier_id = ?`, modifierID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err == nil {
			out = append(out, tid)
		}
	}
	return out
}

func (s *SQLiteService) CreateProduct(p Product) (Product, error) {
	if err := ValidateProduct(p); err != nil {
		return Product{}, err
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM category WHERE id = ?`, p.CategoryID).Scan(&exists); err != nil || exists == 0 {
		return Product{}, errors.New("category_id not found")
	}
	if p.ID == "" {
		p.ID = NewID()
	}
	if p.Status == "" {
		p.Status = StatusActive
	}
	if p.ImageFocus == "" {
		p.ImageFocus = "50,50"
	}
	now := NowMillis()
	p.CreatedAt = now
	p.UpdatedAt = now
	for i := range p.Variants {
		if err := ValidateVariant(p.Variants[i]); err != nil {
			return Product{}, err
		}
		p.Variants[i].ProductID = p.ID
		if p.Variants[i].ID == "" {
			p.Variants[i].ID = NewID()
		}
	}
	EnsureDefaultVariant(&p)

	tx, err := s.db.Begin()
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO product (id, category_id, name_id, name_en, description_id, description_en, image_key, image_focus, prep_time_seconds, status, sort, created_at, updated_at, archived_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		p.ID, p.CategoryID, p.NameID, p.NameEN, p.DescriptionID, p.DescriptionEN, nullString(p.ImageKey), p.ImageFocus, p.PrepTimeSeconds, p.Status, p.Sort, p.CreatedAt, p.UpdatedAt,
	); err != nil {
		return Product{}, err
	}
	for _, v := range p.Variants {
		if _, err := tx.Exec(
			`INSERT INTO variant (id, product_id, name_id, name_en, price, is_default, sort, sold_out, sold_out_at, sold_out_by, sold_out_source, archived_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			v.ID, v.ProductID, v.NameID, v.NameEN, v.Price, boolToInt(v.IsDefault), v.Sort, boolToInt(v.SoldOut), nullInt64(v.SoldOutAt), nullString(v.SoldOutBy), nullString(v.SoldOutSource),
		); err != nil {
			return Product{}, err
		}
	}
	for _, tid := range p.DietaryTagIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO product_dietary_tag (product_id, dietary_tag_id) VALUES (?, ?)`, p.ID, tid); err != nil {
			return Product{}, err
		}
	}
	if err := s.insertAudit(tx, "create", "product", p.ID, nil, p, nil); err != nil {
		return Product{}, err
	}
	if err := tx.Commit(); err != nil {
		return Product{}, err
	}
	return s.loadProduct(p.ID)
}

func (s *SQLiteService) ListProducts(includeArchived bool) []Product {
	query := `SELECT id FROM product WHERE status != 'archived' ORDER BY sort`
	if includeArchived {
		query = `SELECT id FROM product ORDER BY sort`
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return []Product{}
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []Product{}
	for _, id := range ids {
		if p, err := s.loadProduct(id); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *SQLiteService) Catalog() Catalog {
	return Catalog{
		PrepAreas:   s.ListPrepAreas(),
		Categories:  s.ListCategories(),
		Products:    s.ListProducts(false),
		DietaryTags: s.ListDietaryTags(),
		GeneratedAt: NowMillis(),
	}
}

func (s *SQLiteService) UpdateProductStatus(id, status string, reason *string) (Product, error) {
	switch status {
	case StatusActive, StatusHidden, StatusArchived:
	default:
		return Product{}, errors.New("product status must be active, hidden or archived")
	}
	before, err := s.loadProduct(id)
	if err != nil {
		return Product{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback()
	now := NowMillis()
	if status == StatusArchived {
		if _, err := tx.Exec(`UPDATE product SET status = ?, updated_at = ?, archived_at = ? WHERE id = ?`, status, now, now, id); err != nil {
			return Product{}, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE product SET status = ?, updated_at = ?, archived_at = NULL WHERE id = ?`, status, now, id); err != nil {
			return Product{}, err
		}
	}
	after, err := s.loadProduct(id)
	if err != nil {
		// The row was updated in the transaction but is not visible to a
		// separate connection until commit; load after commit instead.
		after = before
		after.Status = status
		after.UpdatedAt = now
	}
	if err := s.insertAudit(tx, "product_status_change", "product", id, before, after, reason); err != nil {
		return Product{}, err
	}
	if err := tx.Commit(); err != nil {
		return Product{}, err
	}
	return s.loadProduct(id)
}

func (s *SQLiteService) findVariant(id string) (Variant, string, error) {
	var v Variant
	var productID string
	var soldOutAt, vArchived sql.NullInt64
	var soldOutBy, soldOutSource sql.NullString
	var isDefault, soldOut int
	err := s.db.QueryRow(
		`SELECT id, product_id, name_id, name_en, price, is_default, sort, sold_out, sold_out_at, sold_out_by, sold_out_source, archived_at FROM variant WHERE id = ?`,
		id,
	).Scan(&v.ID, &productID, &v.NameID, &v.NameEN, &v.Price, &isDefault, &v.Sort, &soldOut, &soldOutAt, &soldOutBy, &soldOutSource, &vArchived)
	if err != nil {
		return Variant{}, "", errors.New("variant not found")
	}
	v.ProductID = productID
	v.IsDefault = intToBool(isDefault)
	v.SoldOut = intToBool(soldOut)
	v.SoldOutAt = intPtr(soldOutAt)
	v.SoldOutBy = strPtr(soldOutBy)
	v.SoldOutSource = strPtr(soldOutSource)
	v.ArchivedAt = intPtr(vArchived)
	return v, productID, nil
}

func (s *SQLiteService) SetVariantSoldOut(id string, by *string, source string) (Variant, error) {
	if source == "" {
		source = SoldOutManual
	}
	before, _, err := s.findVariant(id)
	if err != nil {
		return Variant{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Variant{}, err
	}
	defer tx.Rollback()
	now := NowMillis()
	if _, err := tx.Exec(`UPDATE variant SET sold_out = 1, sold_out_at = ?, sold_out_by = ?, sold_out_source = ? WHERE id = ?`, now, nullString(by), source, id); err != nil {
		return Variant{}, err
	}
	after := before
	after.SoldOut = true
	after.SoldOutAt = &now
	after.SoldOutBy = by
	src := source
	after.SoldOutSource = &src
	if err := s.insertAudit(tx, "sold_out_set", "variant", id, before, after, nil); err != nil {
		return Variant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Variant{}, err
	}
	v, _, err := s.findVariant(id)
	return v, err
}

func (s *SQLiteService) ClearVariantSoldOut(id string) (Variant, error) {
	before, _, err := s.findVariant(id)
	if err != nil {
		return Variant{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Variant{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE variant SET sold_out = 0, sold_out_at = NULL, sold_out_by = NULL, sold_out_source = NULL WHERE id = ?`, id); err != nil {
		return Variant{}, err
	}
	after := before
	after.SoldOut = false
	after.SoldOutAt = nil
	after.SoldOutBy = nil
	after.SoldOutSource = nil
	// BR-ADM-07: every Sold Out re-enable is audit-logged.
	if err := s.insertAudit(tx, "sold_out_re_enable", "variant", id, before, after, nil); err != nil {
		return Variant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Variant{}, err
	}
	v, _, err := s.findVariant(id)
	return v, err
}

func (s *SQLiteService) SetProductDietaryTags(id string, tagIDs []string) (Product, error) {
	before, err := s.loadProduct(id)
	if err != nil {
		return Product{}, err
	}
	for _, tid := range tagIDs {
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM dietary_tag WHERE id = ?`, tid).Scan(&exists); err != nil || exists == 0 {
			return Product{}, errors.New("dietary_tag_id not found: " + tid)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM product_dietary_tag WHERE product_id = ?`, id); err != nil {
		return Product{}, err
	}
	for _, tid := range tagIDs {
		if _, err := tx.Exec(`INSERT INTO product_dietary_tag (product_id, dietary_tag_id) VALUES (?, ?)`, id, tid); err != nil {
			return Product{}, err
		}
	}
	after := before
	after.DietaryTagIDs = append([]string(nil), tagIDs...)
	after.UpdatedAt = NowMillis()
	if _, err := tx.Exec(`UPDATE product SET updated_at = ? WHERE id = ?`, after.UpdatedAt, id); err != nil {
		return Product{}, err
	}
	if err := s.insertAudit(tx, "product_dietary_tag_change", "product", id, before, after, nil); err != nil {
		return Product{}, err
	}
	if err := tx.Commit(); err != nil {
		return Product{}, err
	}
	return s.loadProduct(id)
}

func (s *SQLiteService) CreateModifierGroup(g ModifierGroup) (ModifierGroup, error) {
	if g.NameID == "" || g.NameEN == "" {
		return ModifierGroup{}, errors.New("modifier group name_id and name_en are required")
	}
	if g.MaxSelect < g.MinSelect {
		return ModifierGroup{}, errors.New("max_select must be >= min_select")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM product WHERE id = ?`, g.ProductID).Scan(&exists); err != nil || exists == 0 {
		return ModifierGroup{}, errors.New("product_id not found")
	}
	if g.ID == "" {
		g.ID = NewID()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return ModifierGroup{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO modifier_group (id, product_id, name_id, name_en, min_select, max_select, sort) VALUES (?, ?, ?, ?, ?, ?, ?)`, g.ID, g.ProductID, g.NameID, g.NameEN, g.MinSelect, g.MaxSelect, g.Sort); err != nil {
		return ModifierGroup{}, err
	}
	if err := s.insertAudit(tx, "create", "modifier_group", g.ID, nil, g, nil); err != nil {
		return ModifierGroup{}, err
	}
	return g, tx.Commit()
}

func (s *SQLiteService) CreateModifier(m Modifier) (Modifier, error) {
	if m.NameID == "" || m.NameEN == "" {
		return Modifier{}, errors.New("modifier name_id and name_en are required")
	}
	if m.Price < 0 {
		return Modifier{}, errors.New("modifier price must be >= 0")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM modifier_group WHERE id = ?`, m.ModifierGroupID).Scan(&exists); err != nil || exists == 0 {
		return Modifier{}, errors.New("modifier_group_id not found")
	}
	if m.ID == "" {
		m.ID = NewID()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Modifier{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO modifier (id, modifier_group_id, name_id, name_en, price, sort, archived_at) VALUES (?, ?, ?, ?, ?, ?, NULL)`, m.ID, m.ModifierGroupID, m.NameID, m.NameEN, m.Price, m.Sort); err != nil {
		return Modifier{}, err
	}
	if err := s.insertAudit(tx, "create", "modifier", m.ID, nil, m, nil); err != nil {
		return Modifier{}, err
	}
	return m, tx.Commit()
}

func (s *SQLiteService) SetModifierDietaryTags(id string, tagIDs []string) (Modifier, error) {
	var m Modifier
	var mArchived sql.NullInt64
	err := s.db.QueryRow(`SELECT id, modifier_group_id, name_id, name_en, price, sort, archived_at FROM modifier WHERE id = ?`, id).Scan(&m.ID, &m.ModifierGroupID, &m.NameID, &m.NameEN, &m.Price, &m.Sort, &mArchived)
	if err != nil {
		return Modifier{}, errors.New("modifier not found")
	}
	m.ArchivedAt = intPtr(mArchived)
	m.DietaryTagIDs = s.loadModifierTags(id)
	for _, tid := range tagIDs {
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM dietary_tag WHERE id = ?`, tid).Scan(&exists); err != nil || exists == 0 {
			return Modifier{}, errors.New("dietary_tag_id not found: " + tid)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Modifier{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM modifier_dietary_tag WHERE modifier_id = ?`, id); err != nil {
		return Modifier{}, err
	}
	for _, tid := range tagIDs {
		if _, err := tx.Exec(`INSERT INTO modifier_dietary_tag (modifier_id, dietary_tag_id) VALUES (?, ?)`, id, tid); err != nil {
			return Modifier{}, err
		}
	}
	after := m
	after.DietaryTagIDs = append([]string(nil), tagIDs...)
	if err := s.insertAudit(tx, "modifier_dietary_tag_change", "modifier", id, m, after, nil); err != nil {
		return Modifier{}, err
	}
	if err := tx.Commit(); err != nil {
		return Modifier{}, err
	}
	after.DietaryTagIDs = s.loadModifierTags(id)
	return after, nil
}

func (s *SQLiteService) ListAudit() []AuditEntry {
	rows, err := s.db.Query(`SELECT id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason FROM audit_log ORDER BY at`)
	if err != nil {
		return []AuditEntry{}
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var a AuditEntry
		var actor, override, before, after, reason sql.NullString
		if err := rows.Scan(&a.ID, &a.At, &actor, &override, &a.Action, &a.Entity, &a.EntityID, &before, &after, &reason); err == nil {
			a.ActorCrewID = strPtr(actor)
			a.OverrideCrewID = strPtr(override)
			a.Before = strPtr(before)
			a.After = strPtr(after)
			a.Reason = strPtr(reason)
			out = append(out, a)
		}
	}
	return out
}
