package seed

import (
	"context"
	"fmt"
	"path/filepath"

	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/display"
	"sushilatte/api/internal/media"
	"sushilatte/api/internal/outlet"
)

// seedDisplay creates the Product Display playlist (DEMO-AND-SEED §5).
func seedDisplay(ctx context.Context, db *dbx.DB, opts Options, managementID string, ids catalogIDs) error {
	images := media.NewStore(opts.ImagesDir)
	marketing := map[string]string{}
	for _, name := range []string{"hero-display-banner", "ingredients-flatlay", "cafe-interior-ambiance"} {
		key, err := saveImage(images, filepath.Join(opts.AssetsDir, "marketing", name+".jpg"))
		if err != nil {
			return fmt.Errorf("marketing %s: %w", name, err)
		}
		if err := images.EnsureWide(key, 50, 50); err != nil {
			return err
		}
		marketing[name] = key
	}
	discounts, err := outlet.NewService(db, fixedClock{opts.T0}).Discounts(ctx)
	if err != nil {
		return err
	}
	var happyHour string
	for _, d := range discounts {
		if d.Name == "Happy Hour Kopi" {
			happyHour = d.ID
		}
	}
	focus := catalog.Focus{X: 50, Y: 50}
	slides := []display.Slide{
		{Template: display.HeroPromo, ImageKey: marketing["hero-display-banner"], ImageFocus: focus,
			Headline: display.Bilingual{ID: "Sushi & kopi, satu meja.", EN: "Sushi & coffee, one table."},
			Subline:  display.Bilingual{ID: "Pindai QR di mejamu untuk memesan", EN: "Scan the QR on your table to order"}},
		{Template: display.FeaturedProduct, ProductID: ids.products["latte"].ID, Steam: true,
			Headline: display.Bilingual{ID: "Andalan barista", EN: "Barista's signature"}},
		{Template: display.HappyHour, DiscountID: happyHour, ProductID: ids.products["matcha"].ID,
			Headline: display.Bilingual{ID: "Happy Hour Kopi & Teh", EN: "Coffee & Tea Happy Hour"},
			Subline:  display.Bilingual{ID: "Setiap hari 14.00–17.00", EN: "Every day 2–5 pm"}},
		{Template: display.CategorySpotlight, ImageKey: marketing["ingredients-flatlay"], ImageFocus: focus,
			CategoryIDs: []string{ids.categories["nigiri"], ids.categories["rolls"]},
			Headline:    display.Bilingual{ID: "Nigiri & roll segar", EN: "Fresh nigiri & rolls"}},
		// Skipped while Sakura Latte is Hidden (BR-PD-03).
		{Template: display.FeaturedProduct, ProductID: ids.products["sakura"].ID, Steam: true,
			Headline: display.Bilingual{ID: "Edisi musiman", EN: "Seasonal special"}},
		{Template: display.FeaturedProduct, ProductID: ids.products["hojicha"].ID, Steam: true,
			Headline: display.Bilingual{ID: "Hangat & manis", EN: "Warm & toasty"}},
		{Template: display.BrandIdle, ImageKey: marketing["cafe-interior-ambiance"], ImageFocus: focus},
	}
	svc := display.NewService(db, fixedClock{opts.T0})
	for i, s := range slides {
		s.Active = true
		s.Sort = int64(i+1) * 10
		if _, err := svc.Save(ctx, managementID, s); err != nil {
			return fmt.Errorf("slide %d (%s): %w", i+1, s.Template, err)
		}
	}
	return nil
}
