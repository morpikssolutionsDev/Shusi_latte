package catalog

import (
	"context"

	"sushilatte/api/internal/store"
)

// Load assembles the full catalog. Archived Products are included only when withArchived
// is set (staff screens); Archived Variants, Modifier groups and Modifiers never are.
func Load(ctx context.Context, q *store.Queries, withArchived bool) (Catalog, error) {
	var c Catalog
	areas, err := q.ListPrepAreas(ctx)
	if err != nil {
		return c, err
	}
	for _, a := range areas {
		c.PrepAreas = append(c.PrepAreas, PrepArea{ID: a.ID, Code: a.Code, NameID: a.NameID, NameEN: a.NameEn, ParallelCapacity: a.ParallelCapacity})
	}
	cats, err := q.ListCategories(ctx)
	if err != nil {
		return c, err
	}
	for _, x := range cats {
		c.Categories = append(c.Categories, Category{ID: x.ID, NameID: x.NameID, NameEN: x.NameEn, PrepAreaID: x.PrepAreaID, Sort: x.Sort, Active: x.Active == 1})
	}
	tags, err := q.ListDietaryTags(ctx)
	if err != nil {
		return c, err
	}
	for _, t := range tags {
		c.DietaryTags = append(c.DietaryTags, DietaryTag{ID: t.ID, Code: t.Code, NameID: t.NameID, NameEN: t.NameEn, Icon: t.Icon, Sort: t.Sort, Active: t.Active == 1})
	}

	variants, err := q.ListVariants(ctx)
	if err != nil {
		return c, err
	}
	groups, err := q.ListModifierGroups(ctx)
	if err != nil {
		return c, err
	}
	mods, err := q.ListModifiers(ctx)
	if err != nil {
		return c, err
	}
	ptags, err := q.ListProductDietaryTags(ctx)
	if err != nil {
		return c, err
	}
	mtags, err := q.ListModifierDietaryTags(ctx)
	if err != nil {
		return c, err
	}

	productTags := map[string][]string{}
	for _, t := range ptags {
		productTags[t.ProductID] = append(productTags[t.ProductID], t.DietaryTagID)
	}
	modifierTags := map[string][]string{}
	for _, t := range mtags {
		modifierTags[t.ModifierID] = append(modifierTags[t.ModifierID], t.DietaryTagID)
	}
	modsByGroup := map[string][]Modifier{}
	for _, m := range mods {
		modsByGroup[m.ModifierGroupID] = append(modsByGroup[m.ModifierGroupID], Modifier{
			ID: m.ID, NameID: m.NameID, NameEN: m.NameEn, Price: m.Price, Sort: m.Sort, DietaryTagIDs: orEmpty(modifierTags[m.ID]),
		})
	}
	groupsByProduct := map[string][]ModifierGroup{}
	for _, g := range groups {
		groupsByProduct[g.ProductID] = append(groupsByProduct[g.ProductID], ModifierGroup{
			ID: g.ID, NameID: g.NameID, NameEN: g.NameEn, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect, Sort: g.Sort,
			Modifiers: orEmpty(modsByGroup[g.ID]),
		})
	}
	variantsByProduct := map[string][]Variant{}
	for _, v := range variants {
		vv := Variant{ID: v.ID, NameID: v.NameID, NameEN: v.NameEn, Price: v.Price, IsDefault: v.IsDefault == 1, Sort: v.Sort,
			SoldOut: v.SoldOut == 1, SoldOutSource: v.SoldOutSource.String}
		if v.SoldOutAt.Valid {
			at := v.SoldOutAt.Int64
			vv.SoldOutAt = &at
		}
		variantsByProduct[v.ProductID] = append(variantsByProduct[v.ProductID], vv)
	}

	products, err := q.ListProducts(ctx)
	if err != nil {
		return c, err
	}
	for _, p := range products {
		if p.Status == StatusArchived && !withArchived {
			continue
		}
		c.Products = append(c.Products, Product{
			ID: p.ID, CategoryID: p.CategoryID, NameID: p.NameID, NameEN: p.NameEn,
			DescriptionID: p.DescriptionID, DescriptionEN: p.DescriptionEn, ImageKey: p.ImageKey.String,
			ImageFocus: Focus{X: p.ImageFocusX, Y: p.ImageFocusY}, PrepTimeSeconds: p.PrepTimeSeconds,
			Status: p.Status, Sort: p.Sort, DietaryTagIDs: orEmpty(productTags[p.ID]),
			Variants: orEmpty(variantsByProduct[p.ID]), ModifierGroups: orEmpty(groupsByProduct[p.ID]),
		})
	}
	c.PrepAreas, c.Categories, c.DietaryTags, c.Products = orEmpty(c.PrepAreas), orEmpty(c.Categories), orEmpty(c.DietaryTags), orEmpty(c.Products)
	return c, nil
}

// LoadProduct returns one Product from a freshly loaded catalog.
func LoadProduct(ctx context.Context, q *store.Queries, id string) (Product, bool, error) {
	c, err := Load(ctx, q, true)
	if err != nil {
		return Product{}, false, err
	}
	for _, p := range c.Products {
		if p.ID == id {
			return p, true, nil
		}
	}
	return Product{}, false, nil
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
