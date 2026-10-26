package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// MenuTopic carries catalog changes to every open menu (TR-PC-05).
const MenuTopic = "menu"

type Service struct {
	db    *dbx.DB
	clock platform.Clock
}

func NewService(db *dbx.DB, clock platform.Clock) *Service { return &Service{db: db, clock: clock} }

func invalid(detail string) error {
	p := httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_catalog", "Data katalog tidak valid")
	p.Detail = detail
	return p
}

var errNotFound = httpx.NewProblem(http.StatusNotFound, "not_found", "Tidak ditemukan")

func changed(tx *dbx.Tx, kind, id string) {
	tx.Publish(realtime.Event{Topic: MenuTopic, Type: kind, ID: id})
}

// Catalog returns the full catalog for staff (including Archived Products).
func (s *Service) Catalog(ctx context.Context, withArchived bool) (Catalog, error) {
	return Load(ctx, s.db.Read(), withArchived)
}

// ---------------------------------------------------------------- Categories

func (s *Service) SaveCategory(ctx context.Context, actor string, in Category) (Category, error) {
	in.NameID, in.NameEN = strings.TrimSpace(in.NameID), strings.TrimSpace(in.NameEN)
	if in.NameID == "" || in.NameEN == "" {
		return in, invalid("Nama kategori (ID dan EN) wajib diisi")
	}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := prepArea(ctx, tx, in.PrepAreaID); err != nil {
			return err
		}
		now := platform.Millis(s.clock.Now())
		if in.ID == "" {
			in.ID = platform.NewID()
			in.Active = true
			if err := tx.CreateCategory(ctx, store.CreateCategoryParams{ID: in.ID, NameID: in.NameID, NameEn: in.NameEN,
				PrepAreaID: in.PrepAreaID, Sort: in.Sort, Active: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			return s.audit(ctx, tx, actor, "category.created", "category", in.ID, nil, in)
		}
		before, err := tx.GetCategory(ctx, in.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if !in.Active {
			n, err := tx.CountProductsInCategory(ctx, in.ID)
			if err != nil {
				return err
			}
			if n > 0 {
				return invalid("Kategori masih berisi produk; pindahkan atau arsipkan produknya dulu")
			}
		}
		if err := tx.UpdateCategory(ctx, store.UpdateCategoryParams{NameID: in.NameID, NameEn: in.NameEN, PrepAreaID: in.PrepAreaID,
			Sort: in.Sort, Active: boolInt(in.Active), UpdatedAt: now, ID: in.ID}); err != nil {
			return err
		}
		changed(tx, "category.changed", in.ID)
		return s.audit(ctx, tx, actor, "category.updated", "category", in.ID, before, in)
	})
	return in, err
}

func prepArea(ctx context.Context, tx *dbx.Tx, id string) (store.PrepArea, error) {
	areas, err := tx.ListPrepAreas(ctx)
	if err != nil {
		return store.PrepArea{}, err
	}
	for _, a := range areas {
		if a.ID == id {
			return a, nil
		}
	}
	return store.PrepArea{}, invalid("Prep Area tidak dikenal")
}

// SetPrepAreaCapacity sets how many items a Prep Area works on in parallel (TR-KT-02).
func (s *Service) SetPrepAreaCapacity(ctx context.Context, actor, id string, capacity int64) error {
	if capacity < 1 || capacity > 20 {
		return invalid("Kapasitas harus 1–20")
	}
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		before, err := prepArea(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := tx.UpdatePrepAreaCapacity(ctx, store.UpdatePrepAreaCapacityParams{ParallelCapacity: capacity, ID: id}); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "prep_area.capacity_changed", "prep_area", id, before.ParallelCapacity, capacity)
	})
}

// ---------------------------------------------------------------- Dietary Tags

func (s *Service) SaveDietaryTag(ctx context.Context, actor string, in DietaryTag) (DietaryTag, error) {
	in.Code = strings.ToLower(strings.TrimSpace(in.Code))
	if in.NameID == "" || in.NameEN == "" || in.Code == "" {
		return in, invalid("Kode dan nama tag (ID dan EN) wajib diisi")
	}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		if in.ID == "" {
			in.ID, in.Active = platform.NewID(), true
			if err := tx.CreateDietaryTag(ctx, store.CreateDietaryTagParams{ID: in.ID, Code: in.Code, NameID: in.NameID,
				NameEn: in.NameEN, Icon: in.Icon, Sort: in.Sort}); err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					return invalid("Kode tag sudah dipakai")
				}
				return err
			}
		} else if err := tx.UpdateDietaryTag(ctx, store.UpdateDietaryTagParams{NameID: in.NameID, NameEn: in.NameEN, Icon: in.Icon,
			Sort: in.Sort, Active: boolInt(in.Active), ID: in.ID}); err != nil {
			return err
		}
		changed(tx, "dietary_tag.changed", in.ID)
		return s.audit(ctx, tx, actor, "dietary_tag.saved", "dietary_tag", in.ID, nil, in)
	})
	return in, err
}

// ---------------------------------------------------------------- Products

// SaveProduct creates or replaces a Product with its tags, Variants and Modifier groups in
// one transaction. Variants, groups and Modifiers missing from the input are archived, never
// deleted, because past Orders refer to them (TR-PC-03).
func (s *Service) SaveProduct(ctx context.Context, actor string, in Product) (Product, error) {
	if err := validateProduct(&in); err != nil {
		return in, err
	}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		cat, err := tx.GetCategory(ctx, in.CategoryID)
		if errors.Is(err, sql.ErrNoRows) {
			return invalid("Kategori tidak dikenal")
		}
		if err != nil {
			return err
		}
		if cat.Active == 0 {
			return invalid("Kategori tidak aktif")
		}
		now := s.clock.Now()
		ms := platform.Millis(now)
		var before *Product
		params := store.UpdateProductParams{CategoryID: in.CategoryID, NameID: in.NameID, NameEn: in.NameEN,
			DescriptionID: in.DescriptionID, DescriptionEn: in.DescriptionEN, ImageKey: nullString(in.ImageKey),
			ImageFocusX: in.ImageFocus.X, ImageFocusY: in.ImageFocus.Y, PrepTimeSeconds: in.PrepTimeSeconds,
			Status: in.Status, Sort: in.Sort, UpdatedAt: ms, ID: in.ID}
		if in.ID == "" {
			in.ID = platform.NewID()
			if in.Sort == 0 {
				maxSort, err := tx.MaxProductSort(ctx, in.CategoryID)
				if err != nil {
					return err
				}
				in.Sort = maxSort + 10
			}
			if err := tx.CreateProduct(ctx, store.CreateProductParams{ID: in.ID, CategoryID: in.CategoryID, NameID: in.NameID,
				NameEn: in.NameEN, DescriptionID: in.DescriptionID, DescriptionEn: in.DescriptionEN, ImageKey: nullString(in.ImageKey),
				ImageFocusX: in.ImageFocus.X, ImageFocusY: in.ImageFocus.Y, PrepTimeSeconds: in.PrepTimeSeconds,
				Status: in.Status, Sort: in.Sort, CreatedAt: ms, UpdatedAt: ms}); err != nil {
				return err
			}
		} else {
			old, found, err := LoadProduct(ctx, tx.Queries, in.ID)
			if err != nil {
				return err
			}
			if !found {
				return errNotFound
			}
			before = &old
			if err := tx.UpdateProduct(ctx, params); err != nil {
				return err
			}
		}
		if err := s.saveTags(ctx, tx, in.ID, in.DietaryTagIDs); err != nil {
			return err
		}
		if err := s.saveVariants(ctx, tx, actor, now, in.ID, before, in.Variants); err != nil {
			return err
		}
		if err := s.saveModifierGroups(ctx, tx, now, in.ID, before, in.ModifierGroups); err != nil {
			return err
		}
		saved, _, err := LoadProduct(ctx, tx.Queries, in.ID)
		if err != nil {
			return err
		}
		in = saved
		action := "product.created"
		if before != nil {
			action = "product.updated"
			if before.Status != in.Status {
				action = "product.status." + in.Status
			}
		}
		changed(tx, "product.changed", in.ID)
		var beforeAny any
		if before != nil {
			beforeAny = before
		}
		return s.audit(ctx, tx, actor, action, "product", in.ID, beforeAny, in)
	})
	return in, err
}

func validateProduct(p *Product) error {
	p.NameID, p.NameEN = strings.TrimSpace(p.NameID), strings.TrimSpace(p.NameEN)
	if p.NameID == "" || p.NameEN == "" {
		return invalid("Nama produk (ID dan EN) wajib diisi")
	}
	if p.Status == "" {
		p.Status = StatusActive
	}
	switch p.Status {
	case StatusActive, StatusHidden, StatusArchived:
	default:
		return invalid("Status produk tidak dikenal")
	}
	if p.PrepTimeSeconds < 60 || p.PrepTimeSeconds > 2*60*60 {
		return invalid("Prep Time harus 1–120 menit")
	}
	if p.ImageFocus == (Focus{}) {
		p.ImageFocus = Focus{X: 50, Y: 50}
	}
	if p.ImageFocus.X < 0 || p.ImageFocus.X > 100 || p.ImageFocus.Y < 0 || p.ImageFocus.Y > 100 {
		return invalid("Titik fokus gambar harus 0–100")
	}
	if len(p.Variants) == 0 {
		return invalid("Produk wajib punya minimal satu varian")
	}
	defaults := 0
	for i := range p.Variants {
		v := &p.Variants[i]
		v.NameID, v.NameEN = strings.TrimSpace(v.NameID), strings.TrimSpace(v.NameEN)
		if v.NameID == "" || v.NameEN == "" {
			return invalid("Nama varian (ID dan EN) wajib diisi")
		}
		if v.Price < 0 {
			return invalid("Harga tidak boleh negatif")
		}
		if v.IsDefault {
			defaults++
		}
		v.Sort = int64(i)
	}
	if defaults != 1 {
		for i := range p.Variants {
			p.Variants[i].IsDefault = i == 0
		}
	}
	for i := range p.ModifierGroups {
		g := &p.ModifierGroups[i]
		g.Sort = int64(i)
		if g.NameID == "" || g.NameEN == "" {
			return invalid("Nama grup modifier (ID dan EN) wajib diisi")
		}
		if g.MaxSelect < 1 || g.MinSelect < 0 || g.MinSelect > g.MaxSelect {
			return invalid(fmt.Sprintf("Aturan pilihan grup %q tidak valid", g.NameID))
		}
		if len(g.Modifiers) == 0 {
			return invalid(fmt.Sprintf("Grup %q belum punya pilihan", g.NameID))
		}
		if g.MinSelect > int64(len(g.Modifiers)) {
			return invalid(fmt.Sprintf("Grup %q mewajibkan lebih banyak pilihan daripada yang ada", g.NameID))
		}
		for j := range g.Modifiers {
			m := &g.Modifiers[j]
			m.Sort = int64(j)
			if m.NameID == "" || m.NameEN == "" {
				return invalid("Nama modifier (ID dan EN) wajib diisi")
			}
			if m.Price < 0 {
				return invalid("Harga modifier tidak boleh negatif")
			}
		}
	}
	return nil
}

func (s *Service) saveTags(ctx context.Context, tx *dbx.Tx, productID string, tagIDs []string) error {
	if err := tx.ClearProductDietaryTags(ctx, productID); err != nil {
		return err
	}
	for _, t := range dedupe(tagIDs) {
		if err := tx.AddProductDietaryTag(ctx, store.AddProductDietaryTagParams{ProductID: productID, DietaryTagID: t}); err != nil {
			return invalid("Dietary Tag tidak dikenal")
		}
	}
	return nil
}

func (s *Service) saveVariants(ctx context.Context, tx *dbx.Tx, actor string, now time.Time, productID string, before *Product, in []Variant) error {
	existing := map[string]Variant{}
	if before != nil {
		for _, v := range before.Variants {
			existing[v.ID] = v
		}
	}
	kept := map[string]bool{}
	for _, v := range in {
		if v.ID == "" {
			if err := tx.CreateVariant(ctx, store.CreateVariantParams{ID: platform.NewID(), ProductID: productID, NameID: v.NameID,
				NameEn: v.NameEN, Price: v.Price, IsDefault: boolInt(v.IsDefault), Sort: v.Sort}); err != nil {
				return err
			}
			continue
		}
		old, ok := existing[v.ID]
		if !ok {
			return invalid("Varian bukan milik produk ini")
		}
		kept[v.ID] = true
		if err := tx.UpdateVariant(ctx, store.UpdateVariantParams{NameID: v.NameID, NameEn: v.NameEN, Price: v.Price,
			IsDefault: boolInt(v.IsDefault), Sort: v.Sort, ID: v.ID}); err != nil {
			return err
		}
		if old.Price != v.Price {
			if err := s.audit(ctx, tx, actor, "variant.price_changed", "variant", v.ID, old.Price, v.Price); err != nil {
				return err
			}
		}
	}
	for id := range existing {
		if !kept[id] {
			if err := tx.ArchiveVariant(ctx, store.ArchiveVariantParams{ArchivedAt: nullInt(platform.Millis(now)), ID: id}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) saveModifierGroups(ctx context.Context, tx *dbx.Tx, now time.Time, productID string, before *Product, in []ModifierGroup) error {
	existingGroups := map[string]ModifierGroup{}
	if before != nil {
		for _, g := range before.ModifierGroups {
			existingGroups[g.ID] = g
		}
	}
	archivedAt := nullInt(platform.Millis(now))
	keptGroups := map[string]bool{}
	for _, g := range in {
		groupID := g.ID
		var oldMods map[string]bool
		if groupID == "" {
			groupID = platform.NewID()
			if err := tx.CreateModifierGroup(ctx, store.CreateModifierGroupParams{ID: groupID, ProductID: productID, NameID: g.NameID,
				NameEn: g.NameEN, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Sort: g.Sort}); err != nil {
				return err
			}
		} else {
			old, ok := existingGroups[groupID]
			if !ok {
				return invalid("Grup modifier bukan milik produk ini")
			}
			keptGroups[groupID] = true
			oldMods = map[string]bool{}
			for _, m := range old.Modifiers {
				oldMods[m.ID] = true
			}
			if err := tx.UpdateModifierGroup(ctx, store.UpdateModifierGroupParams{NameID: g.NameID, NameEn: g.NameEN,
				MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Sort: g.Sort, ID: groupID}); err != nil {
				return err
			}
		}
		keptMods := map[string]bool{}
		for _, m := range g.Modifiers {
			modID := m.ID
			if modID == "" {
				modID = platform.NewID()
				if err := tx.CreateModifier(ctx, store.CreateModifierParams{ID: modID, ModifierGroupID: groupID, NameID: m.NameID,
					NameEn: m.NameEN, Price: m.Price, Sort: m.Sort}); err != nil {
					return err
				}
			} else {
				if !oldMods[modID] {
					return invalid("Modifier bukan milik grup ini")
				}
				keptMods[modID] = true
				if err := tx.UpdateModifier(ctx, store.UpdateModifierParams{NameID: m.NameID, NameEn: m.NameEN, Price: m.Price,
					Sort: m.Sort, ID: modID}); err != nil {
					return err
				}
			}
			if err := tx.ClearModifierDietaryTags(ctx, modID); err != nil {
				return err
			}
			for _, t := range dedupe(m.DietaryTagIDs) {
				if err := tx.AddModifierDietaryTag(ctx, store.AddModifierDietaryTagParams{ModifierID: modID, DietaryTagID: t}); err != nil {
					return invalid("Dietary Tag tidak dikenal")
				}
			}
		}
		for id := range oldMods {
			if !keptMods[id] {
				if err := tx.ArchiveModifier(ctx, store.ArchiveModifierParams{ArchivedAt: archivedAt, ID: id}); err != nil {
					return err
				}
			}
		}
	}
	for id := range existingGroups {
		if !keptGroups[id] {
			if err := tx.ArchiveModifierGroup(ctx, store.ArchiveModifierGroupParams{ArchivedAt: archivedAt, ID: id}); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------- Sold Out

// SetSoldOut marks a Variant Sold Out (manually) or re-enables it (BR-ADM-07, BR-R-12).
// Every change is audit-logged and pushed to all menus.
func (s *Service) SetSoldOut(ctx context.Context, actor, variantID string, soldOut bool) (Variant, error) {
	var out Variant
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		v, err := tx.GetVariant(ctx, variantID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && v.ArchivedAt.Valid) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		now := platform.Millis(s.clock.Now())
		if soldOut && v.SoldOut == 0 {
			if err := tx.SetVariantSoldOut(ctx, store.SetVariantSoldOutParams{SoldOutAt: nullInt(now), SoldOutBy: nullString(actor),
				SoldOutSource: nullString(SoldOutManual), ID: variantID}); err != nil {
				return err
			}
			if err := s.audit(ctx, tx, actor, "variant.sold_out.set", "variant", variantID, nil, map[string]string{"source": SoldOutManual}); err != nil {
				return err
			}
		}
		if !soldOut && v.SoldOut == 1 {
			if err := tx.ClearVariantSoldOut(ctx, variantID); err != nil {
				return err
			}
			if err := s.audit(ctx, tx, actor, "variant.sold_out.reenabled", "variant", variantID,
				map[string]any{"source": v.SoldOutSource.String, "since": v.SoldOutAt.Int64}, nil); err != nil {
				return err
			}
		}
		changed(tx, "variant.sold_out", variantID)
		nv, err := tx.GetVariant(ctx, variantID)
		if err != nil {
			return err
		}
		out = Variant{ID: nv.ID, NameID: nv.NameID, NameEN: nv.NameEn, Price: nv.Price, IsDefault: nv.IsDefault == 1, Sort: nv.Sort,
			SoldOut: nv.SoldOut == 1, SoldOutSource: nv.SoldOutSource.String}
		return nil
	})
	return out, err
}

// ---------------------------------------------------------------- helpers

func (s *Service) audit(ctx context.Context, tx *dbx.Tx, actor, action, entity, id string, before, after any) error {
	return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: action, Entity: entity, EntityID: id, Before: before, After: after})
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func nullInt(v int64) sql.NullInt64      { return sql.NullInt64{Int64: v, Valid: true} }

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
