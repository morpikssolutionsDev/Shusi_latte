package catalog

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
)

// Service is a temporary in-memory catalog store for Phase 1a.
// It implements the same domain rules as migration 0001_catalog.sql and
// will be replaced by a SQLite store in Phase 1b. It is not a second
// source of truth for production data: the migration is the schema, and
// this service exists so the Phase 1 endpoints can be exercised before
// the SQLite wiring lands. Auth is not yet implemented, so audit entries
// record actor "system" until Crew sessions arrive.
type Service struct {
	Actor          string
	mu             sync.RWMutex
	prepAreas      map[string]PrepArea
	categories     map[string]Category
	products       map[string]*Product
	modifierGroups map[string]*ModifierGroup
	modifiers      map[string]*Modifier
	dietaryTags    map[string]DietaryTag
	audit          []AuditEntry
}

func NewService() *Service {
	return &Service{
		prepAreas:      make(map[string]PrepArea),
		categories:     make(map[string]Category),
		products:       make(map[string]*Product),
		modifierGroups: make(map[string]*ModifierGroup),
		modifiers:      make(map[string]*Modifier),
		dietaryTags:    make(map[string]DietaryTag),
	}
}

// SeedBaseline mirrors the migration seed: Kitchen and Bar, and the
// BR-PC-06 Dietary Tag list. It is idempotent.
func (s *Service) SeedBaseline() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.prepAreas["prep-kitchen"]; !ok {
		s.prepAreas["prep-kitchen"] = PrepArea{ID: "prep-kitchen", Name: "Kitchen", ParallelCapacity: 1}
	}
	if _, ok := s.prepAreas["prep-bar"]; !ok {
		s.prepAreas["prep-bar"] = PrepArea{ID: "prep-bar", Name: "Bar", ParallelCapacity: 1}
	}
	tags := []DietaryTag{
		{ID: "tag-dairy", Code: "dairy", NameID: "Susu / Laktosa", NameEN: "Dairy / Lactose", Sort: 1, Active: true},
		{ID: "tag-gluten", Code: "gluten", NameID: "Gluten", NameEN: "Gluten", Sort: 2, Active: true},
		{ID: "tag-egg", Code: "egg", NameID: "Telur", NameEN: "Egg", Sort: 3, Active: true},
		{ID: "tag-fish", Code: "fish", NameID: "Ikan", NameEN: "Fish", Sort: 4, Active: true},
		{ID: "tag-raw-fish", Code: "raw_fish", NameID: "Ikan Mentah", NameEN: "Raw Fish", Sort: 5, Active: true},
		{ID: "tag-shellfish", Code: "shellfish", NameID: "Kerang-kerangan", NameEN: "Shellfish", Sort: 6, Active: true},
		{ID: "tag-nuts", Code: "nuts", NameID: "Kacang", NameEN: "Nuts", Sort: 7, Active: true},
		{ID: "tag-peanut", Code: "peanut", NameID: "Kacang Tanah", NameEN: "Peanut", Sort: 8, Active: true},
		{ID: "tag-soy", Code: "soy", NameID: "Kedelai", NameEN: "Soy", Sort: 9, Active: true},
		{ID: "tag-sesame", Code: "sesame", NameID: "Wijen", NameEN: "Sesame", Sort: 10, Active: true},
		{ID: "tag-pork", Code: "pork", NameID: "Babi", NameEN: "Pork", Sort: 11, Active: true},
		{ID: "tag-alcohol", Code: "alcohol", NameID: "Alkohol", NameEN: "Alcohol", Sort: 12, Active: true},
		{ID: "tag-spicy", Code: "spicy", NameID: "Pedas", NameEN: "Spicy", Sort: 13, Active: true},
	}
	for _, t := range tags {
		if _, ok := s.dietaryTags[t.ID]; !ok {
			s.dietaryTags[t.ID] = t
		}
	}
}

func (s *Service) auditLocked(action, entity, entityID string, before, after any, reason *string) {
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
	s.audit = append(s.audit, AuditEntry{
		ID: NewID(), At: NowMillis(), ActorCrewID: &actor,
		Action: action, Entity: entity, EntityID: entityID,
		Before: beforeStr, After: afterStr, Reason: reason,
	})
}

func (s *Service) ListPrepAreas() []PrepArea {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PrepArea, 0, len(s.prepAreas))
	for _, v := range s.prepAreas {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Service) CreatePrepArea(p PrepArea) (PrepArea, error) {
	if p.Name == "" {
		return PrepArea{}, errors.New("prep area name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = NewID()
	}
	if p.ParallelCapacity < 1 {
		p.ParallelCapacity = 1
	}
	s.prepAreas[p.ID] = p
	s.auditLocked("create", "prep_area", p.ID, nil, p, nil)
	return p, nil
}

func (s *Service) ListCategories() []Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Category, 0, len(s.categories))
	for _, v := range s.categories {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sort < out[j].Sort })
	return out
}

func (s *Service) CreateCategory(c Category) (Category, error) {
	if c.NameID == "" || c.NameEN == "" {
		return Category{}, errors.New("category name_id and name_en are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.prepAreas[c.PrepAreaID]; !ok {
		return Category{}, errors.New("prep_area_id not found")
	}
	if c.ID == "" {
		c.ID = NewID()
	}
	s.categories[c.ID] = c
	s.auditLocked("create", "category", c.ID, nil, c, nil)
	return c, nil
}

func (s *Service) ListDietaryTags() []DietaryTag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DietaryTag, 0, len(s.dietaryTags))
	for _, v := range s.dietaryTags {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sort < out[j].Sort })
	return out
}

func (s *Service) CreateDietaryTag(t DietaryTag) (DietaryTag, error) {
	if t.Code == "" || t.NameID == "" || t.NameEN == "" {
		return DietaryTag{}, errors.New("dietary tag code, name_id and name_en are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.dietaryTags {
		if existing.Code == t.Code {
			return DietaryTag{}, errors.New("dietary tag code already exists")
		}
	}
	if t.ID == "" {
		t.ID = NewID()
	}
	t.Active = true
	s.dietaryTags[t.ID] = t
	s.auditLocked("create", "dietary_tag", t.ID, nil, t, nil)
	return t, nil
}

func copyProduct(p *Product) Product {
	if p == nil {
		return Product{}
	}
	cp := *p
	cp.Variants = append([]Variant(nil), p.Variants...)
	cp.ModifierGroups = append([]ModifierGroup(nil), p.ModifierGroups...)
	cp.DietaryTagIDs = append([]string(nil), p.DietaryTagIDs...)
	for i := range cp.ModifierGroups {
		cp.ModifierGroups[i].Modifiers = append([]Modifier(nil), p.ModifierGroups[i].Modifiers...)
	}
	return cp
}

func (s *Service) CreateProduct(p Product) (Product, error) {
	if err := ValidateProduct(p); err != nil {
		return Product{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.categories[p.CategoryID]; !ok {
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
	s.products[p.ID] = &p
	s.auditLocked("create", "product", p.ID, nil, p, nil)
	return copyProduct(&p), nil
}

func (s *Service) ListProducts(includeArchived bool) []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Product, 0, len(s.products))
	for _, p := range s.products {
		if !includeArchived && p.Status == StatusArchived {
			continue
		}
		out = append(out, copyProduct(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sort < out[j].Sort })
	return out
}

// Catalog returns the full active catalog for TR-PC-05. Staff callers
// include Hidden products; Archived products are excluded here.
func (s *Service) Catalog() Catalog {
	products := s.ListProducts(false)
	return Catalog{
		PrepAreas:   s.ListPrepAreas(),
		Categories:  s.ListCategories(),
		Products:    products,
		DietaryTags: s.ListDietaryTags(),
		GeneratedAt: NowMillis(),
	}
}

func (s *Service) UpdateProductStatus(id, status string, reason *string) (Product, error) {
	switch status {
	case StatusActive, StatusHidden, StatusArchived:
	default:
		return Product{}, errors.New("product status must be active, hidden or archived")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.products[id]
	if !ok {
		return Product{}, errors.New("product not found")
	}
	// TR-PC-03: a Product that has ever been ordered cannot be deleted,
	// only Archived. Deletion is not offered at all, so archiving is the
	// retirement path and is audit-logged.
	before := copyProduct(p)
	p.Status = status
	p.UpdatedAt = NowMillis()
	if status == StatusArchived {
		now := NowMillis()
		p.ArchivedAt = &now
	} else {
		p.ArchivedAt = nil
	}
	s.auditLocked("product_status_change", "product", id, before, copyProduct(p), reason)
	return copyProduct(p), nil
}

func (s *Service) SetVariantSoldOut(id string, by *string, source string) (Variant, error) {
	if source == "" {
		source = SoldOutManual
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.products {
		for i := range p.Variants {
			if p.Variants[i].ID == id {
				before := p.Variants[i]
				now := NowMillis()
				src := source
				p.Variants[i].SoldOut = true
				p.Variants[i].SoldOutAt = &now
				p.Variants[i].SoldOutBy = by
				p.Variants[i].SoldOutSource = &src
				p.UpdatedAt = now
				s.auditLocked("sold_out_set", "variant", id, before, p.Variants[i], nil)
				return p.Variants[i], nil
			}
		}
	}
	return Variant{}, errors.New("variant not found")
}

func (s *Service) ClearVariantSoldOut(id string) (Variant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.products {
		for i := range p.Variants {
			if p.Variants[i].ID == id {
				before := p.Variants[i]
				p.Variants[i].SoldOut = false
				p.Variants[i].SoldOutAt = nil
				p.Variants[i].SoldOutBy = nil
				p.Variants[i].SoldOutSource = nil
				p.UpdatedAt = NowMillis()
				// BR-ADM-07: every Sold Out re-enable is audit-logged.
				s.auditLocked("sold_out_re_enable", "variant", id, before, p.Variants[i], nil)
				return p.Variants[i], nil
			}
		}
	}
	return Variant{}, errors.New("variant not found")
}

func (s *Service) SetProductDietaryTags(id string, tagIDs []string) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.products[id]
	if !ok {
		return Product{}, errors.New("product not found")
	}
	for _, tid := range tagIDs {
		if _, ok := s.dietaryTags[tid]; !ok {
			return Product{}, errors.New("dietary_tag_id not found: " + tid)
		}
	}
	before := copyProduct(p)
	p.DietaryTagIDs = append([]string(nil), tagIDs...)
	p.UpdatedAt = NowMillis()
	s.auditLocked("product_dietary_tag_change", "product", id, before, copyProduct(p), nil)
	return copyProduct(p), nil
}

func (s *Service) CreateModifierGroup(g ModifierGroup) (ModifierGroup, error) {
	if g.NameID == "" || g.NameEN == "" {
		return ModifierGroup{}, errors.New("modifier group name_id and name_en are required")
	}
	if g.MaxSelect < g.MinSelect {
		return ModifierGroup{}, errors.New("max_select must be >= min_select")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.products[g.ProductID]
	if !ok {
		return ModifierGroup{}, errors.New("product_id not found")
	}
	if g.ID == "" {
		g.ID = NewID()
	}
	s.modifierGroups[g.ID] = &g
	p.ModifierGroups = append(p.ModifierGroups, g)
	s.auditLocked("create", "modifier_group", g.ID, nil, g, nil)
	return g, nil
}

func (s *Service) CreateModifier(m Modifier) (Modifier, error) {
	if m.NameID == "" || m.NameEN == "" {
		return Modifier{}, errors.New("modifier name_id and name_en are required")
	}
	if m.Price < 0 {
		return Modifier{}, errors.New("modifier price must be >= 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.modifierGroups[m.ModifierGroupID]
	if !ok {
		return Modifier{}, errors.New("modifier_group_id not found")
	}
	if m.ID == "" {
		m.ID = NewID()
	}
	s.modifiers[m.ID] = &m
	g.Modifiers = append(g.Modifiers, m)
	// Keep the product copy in sync for Catalog responses.
	for _, p := range s.products {
		for i := range p.ModifierGroups {
			if p.ModifierGroups[i].ID == g.ID {
				p.ModifierGroups[i] = *g
			}
		}
	}
	s.auditLocked("create", "modifier", m.ID, nil, m, nil)
	return m, nil
}

func (s *Service) SetModifierDietaryTags(id string, tagIDs []string) (Modifier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.modifiers[id]
	if !ok {
		return Modifier{}, errors.New("modifier not found")
	}
	for _, tid := range tagIDs {
		if _, ok := s.dietaryTags[tid]; !ok {
			return Modifier{}, errors.New("dietary_tag_id not found: " + tid)
		}
	}
	before := *m
	m.DietaryTagIDs = append([]string(nil), tagIDs...)
	s.auditLocked("modifier_dietary_tag_change", "modifier", id, before, *m, nil)
	return *m, nil
}

func (s *Service) ListAudit() []AuditEntry {
	entries, _ := s.ListAuditFiltered(AuditFilter{})
	return entries
}

func (s *Service) ListAuditFiltered(f AuditFilter) ([]AuditEntry, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filtered := []AuditEntry{}
	for _, a := range s.audit {
		if f.Action != "" && a.Action != f.Action {
			continue
		}
		if f.Entity != "" && a.Entity != f.Entity {
			continue
		}
		if f.EntityID != "" && a.EntityID != f.EntityID {
			continue
		}
		if f.Actor != "" && (a.ActorCrewID == nil || *a.ActorCrewID != f.Actor) {
			continue
		}
		if f.FromMS != nil && a.At < *f.FromMS {
			continue
		}
		if f.ToMS != nil && a.At >= *f.ToMS {
			continue
		}
		filtered = append(filtered, a)
	}
	// Newest first.
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	total := len(filtered)
	limit := f.effectiveLimit()
	if f.Offset >= total {
		return []AuditEntry{}, total
	}
	end := f.Offset + limit
	if end > total {
		end = total
	}
	return filtered[f.Offset:end], total
}
