package catalog_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
)

type fixture struct {
	db       *dbx.DB
	bus      *realtime.Bus
	svc      *catalog.Service
	actor    string
	category string
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, bus := testutil.NewDB(t)
	clock := testutil.NewClock(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	ctx := context.Background()
	var actor string
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		var err error
		actor, err = auth.CreateCrew(ctx, tx, clock.Now(), auth.NewCrew{CrewID: "M001", FullName: "Hendra", Role: auth.RoleManagement, PIN: "111111"})
		if err != nil {
			return err
		}
		return tx.CreatePrepArea(ctx, store.CreatePrepAreaParams{ID: "pa-kitchen", Code: "kitchen", NameID: "Dapur", NameEn: "Kitchen", ParallelCapacity: 2})
	}); err != nil {
		t.Fatal(err)
	}
	svc := catalog.NewService(db, clock)
	cat, err := svc.SaveCategory(ctx, actor, catalog.Category{NameID: "Nigiri", NameEN: "Nigiri", PrepAreaID: "pa-kitchen"})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{db: db, bus: bus, svc: svc, actor: actor, category: cat.ID}
}

func (f fixture) product(t *testing.T) catalog.Product {
	t.Helper()
	p, err := f.svc.SaveProduct(context.Background(), f.actor, catalog.Product{
		CategoryID: f.category, NameID: "Salmon Nigiri", NameEN: "Salmon Nigiri", PrepTimeSeconds: 360,
		Variants: []catalog.Variant{{NameID: "2 pcs", NameEN: "2 pcs", Price: 48000}, {NameID: "4 pcs", NameEN: "4 pcs", Price: 89000}},
		ModifierGroups: []catalog.ModifierGroup{{NameID: "Tambahan", NameEN: "Extras", MaxSelect: 2,
			Modifiers: []catalog.Modifier{{NameID: "Tanpa wasabi", NameEN: "No wasabi"}, {NameID: "Gari", NameEN: "Gari", Price: 3000}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func code(err error) string {
	var p *httpx.Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

// TR-PC-01, BR-PC-02: a Product needs at least one Variant; exactly one becomes default.
func TestProductValidation(t *testing.T) {
	f := setup(t)
	_, err := f.svc.SaveProduct(context.Background(), f.actor, catalog.Product{CategoryID: f.category, NameID: "X", NameEN: "X", PrepTimeSeconds: 300})
	if code(err) != "invalid_catalog" {
		t.Fatalf("want invalid_catalog without variants, got %v", err)
	}
	p := f.product(t)
	if !p.Variants[0].IsDefault || p.Variants[1].IsDefault {
		t.Fatalf("first variant should be the only default: %+v", p.Variants)
	}
	if p.Status != catalog.StatusActive || p.ImageFocus != (catalog.Focus{X: 50, Y: 50}) {
		t.Fatalf("defaults not applied: %+v", p)
	}
}

// TR-PC-03: removing a Variant or Modifier archives it rather than deleting it; price changes are audited.
func TestSaveProductArchivesRemovedChildrenAndAuditsPrice(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.product(t)
	removed := p.Variants[1].ID
	p.Variants = p.Variants[:1]
	p.Variants[0].Price = 50000
	p.ModifierGroups[0].Modifiers = p.ModifierGroups[0].Modifiers[:1]
	saved, err := f.svc.SaveProduct(ctx, f.actor, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Variants) != 1 || len(saved.ModifierGroups[0].Modifiers) != 1 {
		t.Fatalf("removed children still listed: %+v", saved)
	}
	v, err := f.db.Read().GetVariant(ctx, removed)
	if err != nil || !v.ArchivedAt.Valid {
		t.Fatalf("variant should be archived, not deleted: %+v %v", v, err)
	}
	rows, _ := f.db.Read().SearchAudit(ctx, store.SearchAuditParams{ActionPrefix: sql.NullString{String: "variant.price_changed", Valid: true}, RowLimit: 10})
	if len(rows) != 1 || rows[0].BeforeJson.String != "48000" || rows[0].AfterJson.String != "50000" {
		t.Fatalf("price change not audited: %+v", rows)
	}
}

// BR-ADM-07, BR-R-12: Sold Out greys the Variant; re-enabling is audit-logged; menus are notified.
func TestSoldOutAndReenable(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.product(t)
	var events []realtime.Event
	f.bus.Subscribe(func(e realtime.Event) { events = append(events, e) })

	for _, v := range p.Variants {
		if _, err := f.svc.SetSoldOut(ctx, f.actor, v.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	got, _, _ := catalog.LoadProduct(ctx, f.db.Read(), p.ID)
	if !got.SoldOut() || got.Orderable() {
		t.Fatalf("all variants sold out should grey the product: %+v", got)
	}
	if _, err := f.svc.SetSoldOut(ctx, f.actor, p.Variants[0].ID, false); err != nil {
		t.Fatal(err)
	}
	got, _, _ = catalog.LoadProduct(ctx, f.db.Read(), p.ID)
	if got.SoldOut() || !got.Orderable() {
		t.Fatalf("re-enabled product should be orderable: %+v", got)
	}
	rows, _ := f.db.Read().SearchAudit(ctx, store.SearchAuditParams{ActionPrefix: sql.NullString{String: "variant.sold_out.reenabled", Valid: true}, RowLimit: 10})
	if len(rows) != 1 {
		t.Fatalf("re-enable not audited: %d rows", len(rows))
	}
	if len(events) != 3 || events[0].Topic != catalog.MenuTopic {
		t.Fatalf("want 3 menu events, got %+v", events)
	}
}

// BR-PC-03: Hidden and Archived Products are not orderable; Archived ones drop out of public loads.
func TestProductStatusVisibility(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.product(t)
	p.Status = catalog.StatusArchived
	if _, err := f.svc.SaveProduct(ctx, f.actor, p); err != nil {
		t.Fatal(err)
	}
	public, _ := catalog.Load(ctx, f.db.Read(), false)
	staff, _ := catalog.Load(ctx, f.db.Read(), true)
	if len(public.Products) != 0 || len(staff.Products) != 1 {
		t.Fatalf("archived visibility wrong: public %d staff %d", len(public.Products), len(staff.Products))
	}
}

func TestCategoryWithProductsCannotBeDeactivated(t *testing.T) {
	f := setup(t)
	f.product(t)
	_, err := f.svc.SaveCategory(context.Background(), f.actor, catalog.Category{ID: f.category, NameID: "Nigiri", NameEN: "Nigiri", PrepAreaID: "pa-kitchen", Active: false})
	if code(err) != "invalid_catalog" {
		t.Fatalf("want invalid_catalog, got %v", err)
	}
}
