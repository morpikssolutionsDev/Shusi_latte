// Package catalog implements the Product catalog baseline from TRD §4.2
// and BR-PC-01 to BR-PC-07.
//
// Price is INTEGER whole Rupiah and timestamps are INTEGER UTC Unix
// milliseconds, matching TRD §4. This package is stdlib-only: it defines
// the domain types and rules, and a Store interface that a SQLite
// implementation (migration 0001_catalog.sql) will satisfy. The HTTP
// layer in this phase uses the in-memory Service in memory.go, which is
// explicitly temporary until the SQLite store is wired in Phase 1b.
package catalog

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// ProductStatus values from BR-PC-03.
const (
	StatusActive   = "active"
	StatusHidden   = "hidden"
	StatusArchived = "archived"
)

// SoldOutSource values from TRD §4.2.
const (
	SoldOutManual           = "manual"
	SoldOutCancelOutOfStock = "cancel_out_of_stock"
)

type PrepArea struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ParallelCapacity int    `json:"parallel_capacity"`
}

type Category struct {
	ID         string `json:"id"`
	NameID     string `json:"name_id"`
	NameEN     string `json:"name_en"`
	Sort       int    `json:"sort"`
	PrepAreaID string `json:"prep_area_id"`
}

type Product struct {
	ID              string          `json:"id"`
	CategoryID      string          `json:"category_id"`
	NameID          string          `json:"name_id"`
	NameEN          string          `json:"name_en"`
	DescriptionID   string          `json:"description_id"`
	DescriptionEN   string          `json:"description_en"`
	ImageKey        *string         `json:"image_key"`
	ImageFocus      string          `json:"image_focus"`
	PrepTimeSeconds int             `json:"prep_time_seconds"`
	Status          string          `json:"status"`
	Sort            int             `json:"sort"`
	CreatedAt       int64           `json:"created_at"`
	UpdatedAt       int64           `json:"updated_at"`
	ArchivedAt      *int64          `json:"archived_at"`
	Variants        []Variant       `json:"variants,omitempty"`
	ModifierGroups  []ModifierGroup `json:"modifier_groups,omitempty"`
	DietaryTagIDs   []string        `json:"dietary_tag_ids,omitempty"`
}

// Orderable implements TR-PC-02: a Product is orderable when it is active
// and at least one non-archived Variant is not Sold Out. It is derived,
// never stored.
func (p Product) Orderable() bool {
	if p.Status != StatusActive {
		return false
	}
	for _, v := range p.Variants {
		if v.ArchivedAt == nil && !v.SoldOut {
			return true
		}
	}
	return false
}

type Variant struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	NameID        string  `json:"name_id"`
	NameEN        string  `json:"name_en"`
	Price         int64   `json:"price"`
	IsDefault     bool    `json:"is_default"`
	Sort          int     `json:"sort"`
	SoldOut       bool    `json:"sold_out"`
	SoldOutAt     *int64  `json:"sold_out_at"`
	SoldOutBy     *string `json:"sold_out_by"`
	SoldOutSource *string `json:"sold_out_source"`
	ArchivedAt    *int64  `json:"archived_at"`
}

type ModifierGroup struct {
	ID        string     `json:"id"`
	ProductID string     `json:"product_id"`
	NameID    string     `json:"name_id"`
	NameEN    string     `json:"name_en"`
	MinSelect int        `json:"min_select"`
	MaxSelect int        `json:"max_select"`
	Sort      int        `json:"sort"`
	Modifiers []Modifier `json:"modifiers,omitempty"`
}

type Modifier struct {
	ID              string   `json:"id"`
	ModifierGroupID string   `json:"modifier_group_id"`
	NameID          string   `json:"name_id"`
	NameEN          string   `json:"name_en"`
	Price           int64    `json:"price"`
	Sort            int      `json:"sort"`
	ArchivedAt      *int64   `json:"archived_at"`
	DietaryTagIDs   []string `json:"dietary_tag_ids,omitempty"`
}

type DietaryTag struct {
	ID     string  `json:"id"`
	Code   string  `json:"code"`
	NameID string  `json:"name_id"`
	NameEN string  `json:"name_en"`
	Icon   *string `json:"icon"`
	Sort   int     `json:"sort"`
	Active bool    `json:"active"`
}

// AuditEntry records a catalog change, TR-AUD-01 and TR-AUD-02.
// Before/After are JSON snapshots as strings, matching audit_log.
// ActorName is the full name of the actor crew member, resolved
// from the crew table when the actor is a Crew ID, and null for
// system actions.
type AuditEntry struct {
	ID             string  `json:"id"`
	At             int64   `json:"at"`
	ActorCrewID    *string `json:"actor_crew_id"`
	ActorName      *string `json:"actor_name"`
	OverrideCrewID *string `json:"override_crew_id"`
	Action         string  `json:"action"`
	Entity         string  `json:"entity"`
	EntityID       string  `json:"entity_id"`
	Before         *string `json:"before"`
	After          *string `json:"after"`
	Reason         *string `json:"reason"`
}

// AuditFilter selects audit rows for the audit view (BR-ADM-12).
// FromMS and ToMS are UTC Unix milliseconds, exclusive at ToMS.
// Limit defaults to 100 and is capped at 500; Offset paginates.
type AuditFilter struct {
	Action   string
	Entity   string
	EntityID string
	Actor    string
	FromMS   *int64
	ToMS     *int64
	Limit    int
	Offset   int
}

func (f AuditFilter) effectiveLimit() int {
	if f.Limit <= 0 {
		return 100
	}
	if f.Limit > 500 {
		return 500
	}
	return f.Limit
}

// Catalog is the full active catalog for TR-PC-05: Categories, Products,
// Variants with Sold Out, Modifiers and Dietary Tags in one response.
type Catalog struct {
	PrepAreas   []PrepArea   `json:"prep_areas"`
	Categories  []Category   `json:"categories"`
	Products    []Product    `json:"products"`
	DietaryTags []DietaryTag `json:"dietary_tags"`
	GeneratedAt int64        `json:"generated_at"`
}

func NowMillis() int64 {
	return time.Now().UnixMilli()
}

func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ValidateProduct(p Product) error {
	if p.NameID == "" || p.NameEN == "" {
		return errors.New("product name_id and name_en are required")
	}
	if p.CategoryID == "" {
		return errors.New("product category_id is required")
	}
	if p.PrepTimeSeconds < 0 {
		return errors.New("prep_time_seconds must be >= 0")
	}
	switch p.Status {
	case "", StatusActive, StatusHidden, StatusArchived:
	default:
		return errors.New("product status must be active, hidden or archived")
	}
	return nil
}

func ValidateVariant(v Variant) error {
	if v.NameID == "" || v.NameEN == "" {
		return errors.New("variant name_id and name_en are required")
	}
	if v.Price < 0 {
		return errors.New("variant price must be >= 0")
	}
	return nil
}

// EnsureDefaultVariant implements TR-PC-01: a Product with no options has
// exactly one default Variant, so price and Sold Out always live on a Variant.
func EnsureDefaultVariant(p *Product) {
	if len(p.Variants) > 0 {
		return
	}
	p.Variants = []Variant{{
		ID:        NewID(),
		ProductID: p.ID,
		NameID:    "Reguler",
		NameEN:    "Regular",
		Price:     0,
		IsDefault: true,
	}}
}

// EffectiveDietaryTags implements TR-PC-04 for one Order Item:
// product tags union chosen modifier tags. Tags are not removable by
// Variants or Modifiers in the MVP.
func EffectiveDietaryTags(productTags []string, modifierTags [][]string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, t := range productTags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, tags := range modifierTags {
		for _, t := range tags {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}
