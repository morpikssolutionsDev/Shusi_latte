package catalog

// Store is the catalog persistence interface used by the HTTP Handler.
// Both the temporary in-memory Service and the SQLiteService satisfy it,
// so Phase 1b can swap the implementation without changing handlers.
type Store interface {
	SeedBaseline()
	ListPrepAreas() []PrepArea
	CreatePrepArea(PrepArea) (PrepArea, error)
	ListCategories() []Category
	CreateCategory(Category) (Category, error)
	ListDietaryTags() []DietaryTag
	CreateDietaryTag(DietaryTag) (DietaryTag, error)
	CreateProduct(Product) (Product, error)
	ListProducts(includeArchived bool) []Product
	Catalog() Catalog
	UpdateProductStatus(id, status string, reason *string) (Product, error)
	SetVariantSoldOut(id string, by *string, source string) (Variant, error)
	ClearVariantSoldOut(id string) (Variant, error)
	SetProductDietaryTags(id string, tagIDs []string) (Product, error)
	CreateModifierGroup(ModifierGroup) (ModifierGroup, error)
	CreateModifier(Modifier) (Modifier, error)
	SetModifierDietaryTags(id string, tagIDs []string) (Modifier, error)
	ListAudit() []AuditEntry
	ListAuditFiltered(f AuditFilter) ([]AuditEntry, int)
}
