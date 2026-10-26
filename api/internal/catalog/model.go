// Package catalog owns the Product catalog: the single definition of what the Business
// sells, read by every Module (BRD §6.0, TRD §4.2).
package catalog

// API shapes. Names are bilingual: *Id is Indonesian, *En is English.

type PrepArea struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	NameID           string `json:"nameId"`
	NameEN           string `json:"nameEn"`
	ParallelCapacity int64  `json:"parallelCapacity"`
}

type Category struct {
	ID         string `json:"id"`
	NameID     string `json:"nameId"`
	NameEN     string `json:"nameEn"`
	PrepAreaID string `json:"prepAreaId"`
	Sort       int64  `json:"sort"`
	Active     bool   `json:"active"`
}

type DietaryTag struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	NameID string `json:"nameId"`
	NameEN string `json:"nameEn"`
	Icon   string `json:"icon"`
	Sort   int64  `json:"sort"`
	Active bool   `json:"active"`
}

type Focus struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

type Variant struct {
	ID            string `json:"id,omitempty"`
	NameID        string `json:"nameId"`
	NameEN        string `json:"nameEn"`
	Price         int64  `json:"price"`
	IsDefault     bool   `json:"isDefault"`
	Sort          int64  `json:"sort"`
	SoldOut       bool   `json:"soldOut"`
	SoldOutAt     *int64 `json:"soldOutAt,omitempty"`
	SoldOutSource string `json:"soldOutSource,omitempty"`
}

type Modifier struct {
	ID            string   `json:"id,omitempty"`
	NameID        string   `json:"nameId"`
	NameEN        string   `json:"nameEn"`
	Price         int64    `json:"price"`
	Sort          int64    `json:"sort"`
	DietaryTagIDs []string `json:"dietaryTagIds"`
}

type ModifierGroup struct {
	ID        string     `json:"id,omitempty"`
	NameID    string     `json:"nameId"`
	NameEN    string     `json:"nameEn"`
	MinSelect int64      `json:"minSelect"`
	MaxSelect int64      `json:"maxSelect"`
	Sort      int64      `json:"sort"`
	Modifiers []Modifier `json:"modifiers"`
}

type Product struct {
	ID              string          `json:"id"`
	CategoryID      string          `json:"categoryId"`
	NameID          string          `json:"nameId"`
	NameEN          string          `json:"nameEn"`
	DescriptionID   string          `json:"descriptionId"`
	DescriptionEN   string          `json:"descriptionEn"`
	ImageKey        string          `json:"imageKey,omitempty"`
	ImageFocus      Focus           `json:"imageFocus"`
	PrepTimeSeconds int64           `json:"prepTimeSeconds"`
	Status          string          `json:"status"`
	Sort            int64           `json:"sort"`
	DietaryTagIDs   []string        `json:"dietaryTagIds"`
	Variants        []Variant       `json:"variants"`
	ModifierGroups  []ModifierGroup `json:"modifierGroups"`
}

// SoldOut reports whether every Variant is Sold Out, which greys the Product out (BR-PC-04).
func (p Product) SoldOut() bool {
	for _, v := range p.Variants {
		if !v.SoldOut {
			return false
		}
	}
	return len(p.Variants) > 0
}

// Orderable is TR-PC-02: Active and at least one Variant not Sold Out.
func (p Product) Orderable() bool { return p.Status == StatusActive && !p.SoldOut() }

type Catalog struct {
	PrepAreas   []PrepArea   `json:"prepAreas"`
	Categories  []Category   `json:"categories"`
	DietaryTags []DietaryTag `json:"dietaryTags"`
	Products    []Product    `json:"products"`
}

const (
	StatusActive   = "active"
	StatusHidden   = "hidden"
	StatusArchived = "archived"

	SoldOutManual     = "manual"
	SoldOutOutOfStock = "cancel_out_of_stock"
)
