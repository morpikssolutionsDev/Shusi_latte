package display_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/display"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
)

// Monday 2026-10-26 15:00 WIB: inside a 14:00–17:00 Happy Hour.
var t0 = time.Date(2026, 10, 26, 15, 0, 0, 0, platform.Jakarta)

type fixture struct {
	clock  *testutil.Clock
	cat    *catalog.Service
	svc    *display.Service
	actor  string
	latte  catalog.Product
	coffee string
	hh     outlet.Discount
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, _ := testutil.NewDB(t)
	clock := testutil.NewClock(t0)
	ctx := context.Background()
	var actor string
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		var err error
		actor, err = auth.CreateCrew(ctx, tx, t0, auth.NewCrew{CrewID: "M001", FullName: "Hendra", Role: auth.RoleManagement, PIN: "111111"})
		if err != nil {
			return err
		}
		return tx.CreatePrepArea(ctx, store.CreatePrepAreaParams{ID: "bar", Code: "bar", NameID: "Bar", NameEn: "Bar", ParallelCapacity: 1})
	}); err != nil {
		t.Fatal(err)
	}
	cat := catalog.NewService(db, clock)
	coffee, _ := cat.SaveCategory(ctx, actor, catalog.Category{NameID: "Kopi", NameEN: "Coffee", PrepAreaID: "bar"})
	latte, err := cat.SaveProduct(ctx, actor, catalog.Product{CategoryID: coffee.ID, NameID: "Latte", NameEN: "Latte", PrepTimeSeconds: 240, ImageKey: "img/x",
		Variants: []catalog.Variant{{NameID: "Panas", NameEN: "Hot", Price: 38000}, {NameID: "Dingin", NameEN: "Iced", Price: 41000}}})
	if err != nil {
		t.Fatal(err)
	}
	start, end := int64(14*60), int64(17*60)
	hh, err := outlet.NewService(db, clock).SaveDiscount(ctx, actor, outlet.Discount{Name: "Happy Hour Kopi", Kind: "percent", Value: 1500,
		Scope: "item", Trigger: "auto", StartMinute: &start, EndMinute: &end, Active: true, Categories: []string{coffee.ID}})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{clock: clock, cat: cat, svc: display.NewService(db, clock), actor: actor, latte: latte, coffee: coffee.ID, hh: hh}
}

func (f fixture) save(t *testing.T, s display.Slide) display.Slide {
	t.Helper()
	s.Active = true
	out, err := f.svc.Save(context.Background(), f.actor, s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func templatesOf(p display.Playlist) []string {
	var out []string
	for _, s := range p.Slides {
		out = append(out, s.Template)
	}
	return out
}

// BR-PD-03: a Hidden or fully Sold Out featured Product drops out of the playlist; partial
// Sold Out keeps it, with the lowest available price.
func TestPlaylistSkipsUnavailableProducts(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.save(t, display.Slide{Template: display.BrandIdle})
	f.save(t, display.Slide{Template: display.FeaturedProduct, ProductID: f.latte.ID})

	p, _ := f.svc.Playlist(ctx)
	if got := templatesOf(p); len(got) != 2 || p.Slides[1].Product.Price != 38000 || !p.Slides[1].Product.PriceFrom {
		t.Fatalf("want idle + featured from 38 000, got %v %+v", got, p.Slides[1].Product)
	}

	// Hot sold out: still shown, now from 41 000 with the Sold Out Variant listed.
	if _, err := f.cat.SetSoldOut(ctx, f.actor, f.latte.Variants[0].ID, true); err != nil {
		t.Fatal(err)
	}
	p, _ = f.svc.Playlist(ctx)
	if pv := p.Slides[1].Product; pv.Price != 41000 || pv.PriceFrom || len(pv.SoldOutSome) != 1 {
		t.Fatalf("partial sold out view wrong: %+v", pv)
	}

	// Every Variant sold out: skipped.
	if _, err := f.cat.SetSoldOut(ctx, f.actor, f.latte.Variants[1].ID, true); err != nil {
		t.Fatal(err)
	}
	p, _ = f.svc.Playlist(ctx)
	if got := templatesOf(p); len(got) != 1 {
		t.Fatalf("fully sold out featured product should be skipped, got %v", got)
	}

	// Hidden: skipped even when available again.
	for _, v := range f.latte.Variants {
		_, _ = f.cat.SetSoldOut(ctx, f.actor, v.ID, false)
	}
	hidden := f.latte
	hidden.Status = catalog.StatusHidden
	if _, err := f.cat.SaveProduct(ctx, f.actor, hidden); err != nil {
		t.Fatal(err)
	}
	p, _ = f.svc.Playlist(ctx)
	if got := templatesOf(p); len(got) != 1 {
		t.Fatalf("hidden product should be skipped, got %v", got)
	}
}

// MOTION.md §6 Happy Hour: active window, discounted showcase price, and the next window outside it.
func TestHappyHourSlide(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.save(t, display.Slide{Template: display.HappyHour, DiscountID: f.hh.ID, ProductID: f.latte.ID})

	p, _ := f.svc.Playlist(ctx)
	d := p.Slides[0].Discount
	wantEnd := time.Date(2026, 10, 26, 17, 0, 0, 0, platform.Jakarta)
	if !d.ActiveNow || d.EndsAt != platform.Millis(wantEnd) || d.DiscountedPrice != 32300 {
		t.Fatalf("active happy hour wrong: %+v", d)
	}

	f.clock.Advance(3 * time.Hour) // 18:00, after the window
	p, _ = f.svc.Playlist(ctx)
	d = p.Slides[0].Discount
	wantStart := time.Date(2026, 10, 27, 14, 0, 0, 0, platform.Jakarta)
	if d.ActiveNow || d.StartsAt != platform.Millis(wantStart) {
		t.Fatalf("next happy hour wrong: %+v", d)
	}
}

func TestNextWindow(t *testing.T) {
	m := func(v int) *int { return &v }
	at := func(day, h int) time.Time { return time.Date(2026, 10, day, h, 0, 0, 0, platform.Jakarta) }
	// Overnight 22:00–02:00, weekdays only (2026-10-31 is Saturday).
	s := pricing.Schedule{DaysMask: 0b0011111, StartMinute: m(22 * 60), EndMinute: m(2 * 60)}
	start, end, active, ok := display.NextWindow(s, at(27, 1), platform.Jakarta)
	if !ok || !active || !start.Equal(at(26, 22)) || !end.Equal(at(27, 2)) {
		t.Fatalf("inside overnight window: %v %v %v %v", start, end, active, ok)
	}
	start, _, active, _ = display.NextWindow(s, at(30, 23), platform.Jakarta) // Friday night window
	if !active || !start.Equal(at(30, 22)) {
		t.Fatalf("friday window: %v %v", start, active)
	}
	start, _, active, _ = display.NextWindow(s, at(31, 12), platform.Jakarta) // Saturday noon → Monday 22:00
	if active || !start.Equal(time.Date(2026, 11, 2, 22, 0, 0, 0, platform.Jakarta)) {
		t.Fatalf("weekend skip: %v %v", start, active)
	}
}

func TestSlideValidation(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Save(context.Background(), f.actor, display.Slide{Template: display.HeroPromo}); err == nil {
		t.Fatal("hero promo without headline and image must fail")
	}
	if _, err := f.svc.Save(context.Background(), f.actor, display.Slide{Template: "marquee"}); err == nil {
		t.Fatal("unknown template must fail")
	}
	if _, err := f.svc.Save(context.Background(), f.actor, display.Slide{Template: display.Uploaded}); err == nil {
		t.Fatal("uploaded slide without image or video must fail")
	}
	// A video-only uploaded slide is valid and reaches the playlist with its key.
	s, err := f.svc.Save(context.Background(), f.actor, display.Slide{Template: display.Uploaded, Active: true, VideoKey: "vid/abc.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.Playlist(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	idx := slices.IndexFunc(p.Slides, func(x display.PlaylistSlide) bool { return x.ID == s.ID })
	if idx < 0 || p.Slides[idx].VideoKey != "vid/abc.mp4" {
		t.Fatalf("video slide missing from playlist: %+v", p.Slides)
	}
	// Video is dropped from other templates.
	b, err := f.svc.Save(context.Background(), f.actor, display.Slide{Template: display.BrandIdle, VideoKey: "vid/abc.mp4"})
	if err != nil || b.VideoKey != "" {
		t.Fatalf("brand idle kept video: %v %q", err, b.VideoKey)
	}
}
