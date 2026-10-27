package ordering_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
)

var t0 = time.Date(2026, 10, 27, 12, 0, 0, 0, platform.Jakarta)

type fixture struct {
	db     *dbx.DB
	clock  *testutil.Clock
	svc    *ordering.Service
	cat    *catalog.Service
	actor  string
	qr     string
	qr2    string
	dairy  string
	latte  catalog.Product // Hot 38.000 / Iced 41.000, contains Dairy, Milk group (pick 1)
	nigiri catalog.Product // one Variant 48.000
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
		for _, id := range []string{"bar", "kitchen"} {
			if err := tx.CreatePrepArea(ctx, store.CreatePrepAreaParams{ID: id, Code: id, NameID: id, NameEn: id, ParallelCapacity: 1}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cat := catalog.NewService(db, clock)
	dairy, err := cat.SaveDietaryTag(ctx, actor, catalog.DietaryTag{Code: "dairy", NameID: "Susu", NameEN: "Dairy", Icon: "milk", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	soy, _ := cat.SaveDietaryTag(ctx, actor, catalog.DietaryTag{Code: "soy", NameID: "Kedelai", NameEN: "Soy", Icon: "bean", Active: true})
	coffee, _ := cat.SaveCategory(ctx, actor, catalog.Category{NameID: "Kopi", NameEN: "Coffee", PrepAreaID: "bar", Active: true})
	sushi, _ := cat.SaveCategory(ctx, actor, catalog.Category{NameID: "Nigiri", NameEN: "Nigiri", PrepAreaID: "kitchen", Active: true})
	latte, err := cat.SaveProduct(ctx, actor, catalog.Product{CategoryID: coffee.ID, NameID: "Latte", NameEN: "Latte", PrepTimeSeconds: 240,
		Status: catalog.StatusActive, DietaryTagIDs: []string{dairy.ID},
		Variants: []catalog.Variant{{NameID: "Panas", NameEN: "Hot", Price: 38000, IsDefault: true}, {NameID: "Dingin", NameEN: "Iced", Price: 41000}},
		ModifierGroups: []catalog.ModifierGroup{{NameID: "Susu", NameEN: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []catalog.Modifier{
			{NameID: "Susu segar", NameEN: "Fresh milk", Price: 0, DietaryTagIDs: []string{dairy.ID}},
			{NameID: "Susu kedelai", NameEN: "Soy milk", Price: 5000, DietaryTagIDs: []string{soy.ID}},
		}}}})
	if err != nil {
		t.Fatal(err)
	}
	nigiri, err := cat.SaveProduct(ctx, actor, catalog.Product{CategoryID: sushi.ID, NameID: "Salmon Aburi", NameEN: "Salmon Aburi", PrepTimeSeconds: 480,
		Status: catalog.StatusActive, Variants: []catalog.Variant{{NameID: "2 pcs", NameEN: "2 pcs", Price: 48000, IsDefault: true}}})
	if err != nil {
		t.Fatal(err)
	}
	tables := outlet.NewService(db, clock)
	t3, err := tables.SaveTable(ctx, actor, outlet.Table{Number: 3, Label: "Meja 3", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	t4, _ := tables.SaveTable(ctx, actor, outlet.Table{Number: 4, Label: "Meja 4", Active: true})
	return fixture{db: db, clock: clock, svc: ordering.NewService(db, clock, testutil.NewVault(t)), cat: cat, actor: actor,
		qr: t3.QRToken, qr2: t4.QRToken, dairy: dairy.ID, latte: latte, nigiri: nigiri}
}

func (f fixture) join(t *testing.T, name string, restriction ...string) (ordering.Session, *ordering.Guest) {
	t.Helper()
	s, err := f.svc.Join(context.Background(), f.qr, ordering.JoinInput{Name: name, RestrictionTagIDs: restriction, Consent: len(restriction) > 0})
	if err != nil {
		t.Fatal(err)
	}
	g, err := f.svc.Authenticate(context.Background(), s.Token)
	if err != nil {
		t.Fatal(err)
	}
	return s, g
}

func (f fixture) latteLine(variant int, modifier int, ack ...string) ordering.ItemInput {
	return ordering.ItemInput{VariantID: f.latte.Variants[variant].ID, Quantity: 1,
		ModifierIDs: []string{f.latte.ModifierGroups[0].Modifiers[modifier].ID}, AcknowledgedTagIDs: ack}
}

func problemCode(err error) string {
	var p *httpx.Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

// TR-DM-01, Phase 3 done-when: two phones scanning the same QR at the same moment produce one Bill.
func TestSimultaneousFirstScansShareOneBill(t *testing.T) {
	f := setup(t)
	var wg sync.WaitGroup
	sessions := make([]ordering.Session, 8)
	errs := make([]error, 8)
	for i := range sessions {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sessions[i], errs[i] = f.svc.Join(context.Background(), f.qr, ordering.JoinInput{Name: "Tamu"})
		}(i)
	}
	wg.Wait()
	first := 0
	for i, s := range sessions {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if s.BillID != sessions[0].BillID {
			t.Fatalf("scan %d opened another Bill", i)
		}
		if s.FirstScan {
			first++
		}
	}
	if first != 1 {
		t.Fatalf("%d scans think they opened the Bill, want 1", first)
	}
	st, _ := f.svc.Lookup(context.Background(), f.qr)
	if st.State != "occupied" || len(st.Customers) != 8 {
		t.Fatalf("lookup = %+v", st)
	}
}

// BR-CM-12, BR-CM-13, TR-AUTH-07: Rejoin moves the Customer to the new device; the old one is
// told it continued elsewhere. Names show on the occupied prompt.
func TestRejoinRevokesPreviousDevice(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	raka, _ := f.join(t, "Raka Aditya", f.dairy)
	f.join(t, "Laras")
	st, _ := f.svc.Lookup(ctx, f.qr)
	names := []string{st.Customers[0].Name, st.Customers[1].Name}
	if !slices.Equal(names, []string{"Raka Aditya", "Laras"}) {
		t.Fatalf("occupied names = %v", names)
	}
	again, err := f.svc.Rejoin(ctx, f.qr, raka.CustomerID)
	if err != nil || again.FirstScan || again.Name != "Raka Aditya" {
		t.Fatalf("rejoin: %+v %v", again, err)
	}
	if _, err := f.svc.Authenticate(ctx, raka.Token); problemCode(err) != "continued_elsewhere" {
		t.Fatalf("old device: %v", err)
	}
	g, err := f.svc.Authenticate(ctx, again.Token)
	if err != nil {
		t.Fatal(err)
	}
	me, _ := f.svc.Me(ctx, g)
	if !slices.Equal(me.Customer.RestrictionTagIDs, []string{f.dairy}) {
		t.Fatalf("restriction not restored: %v", me.Customer.RestrictionTagIDs)
	}
	// A Customer of another Table cannot be taken over from this QR.
	if _, err := f.svc.Rejoin(ctx, f.qr2, raka.CustomerID); err == nil {
		t.Fatal("rejoin through another Table's QR must fail")
	}
}

// BR-CM-02, OP-01, TR-RET-04: without consent the dietary data is dropped; with it, names and
// restrictions are stored encrypted only.
func TestConsentAndEncryption(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, err := f.svc.Join(ctx, f.qr, ordering.JoinInput{Name: "Dewi", RestrictionTagIDs: []string{f.dairy}, DietaryNote: "alergi kacang", Consent: false})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := f.svc.Authenticate(ctx, s.Token)
	me, _ := f.svc.Me(ctx, g)
	if len(me.Customer.RestrictionTagIDs) != 0 || me.Customer.DietaryNote != "" {
		t.Fatalf("dietary data kept without consent: %+v", me.Customer)
	}
	s2, _ := f.svc.Join(ctx, f.qr, ordering.JoinInput{Name: "Kevin", RestrictionTagIDs: []string{f.dairy}, DietaryNote: "alergi kacang", Consent: true})
	row, err := f.db.Read().GetCustomerEntry(ctx, s2.CustomerID)
	if err != nil {
		t.Fatal(err)
	}
	if !row.ConsentAt.Valid || row.RestrictionTagsEnc == nil {
		t.Fatal("consent or restriction missing")
	}
	for _, blob := range [][]byte{row.NameEnc, row.DietaryNoteEnc, row.RestrictionTagsEnc} {
		if bytes.Contains(blob, []byte("Kevin")) || bytes.Contains(blob, []byte("kacang")) || bytes.Contains(blob, []byte(f.dairy)) {
			t.Fatal("plaintext stored")
		}
	}
	if _, err := f.svc.Join(ctx, f.qr, ordering.JoinInput{Name: "   "}); problemCode(err) != "invalid_input" {
		t.Fatalf("empty name: %v", err)
	}
}

// BR-CM-10, BR-CM-11, BR-R-D1..D3, TR-DM-05, Phase 3 done-when: a Dietary Warning Order is
// created with the warning stored on the Order Item; it never blocks, but must be acknowledged.
func TestDietaryWarningStoredOnOrderItem(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, raka := f.join(t, "Raka", f.dairy)
	_, laras := f.join(t, "Laras")

	// Not acknowledged → rejected, naming the tag.
	_, err := f.svc.PlaceOrder(ctx, raka, []ordering.ItemInput{f.latteLine(0, 0)})
	var p *httpx.Problem
	if !errors.As(err, &p) || p.Code != "order_rejected" {
		t.Fatalf("want order_rejected, got %v", err)
	}
	items := p.Extra["items"].([]ordering.ItemProblem)
	if items[0].Code != "dietary_ack_required" || !slices.Equal(items[0].TagIDs, []string{f.dairy}) {
		t.Fatalf("problem = %+v", items[0])
	}

	// Acknowledged → created, warning copied onto the Order Item.
	if _, err := f.svc.PlaceOrder(ctx, raka, []ordering.ItemInput{f.latteLine(0, 0, f.dairy), {VariantID: f.nigiri.Variants[0].ID, Quantity: 2}}); err != nil {
		t.Fatal(err)
	}
	me, _ := f.svc.Me(ctx, raka)
	o := me.Orders[0]
	if o.Status != "pending" || !o.Editable || len(o.Items) != 2 {
		t.Fatalf("order = %+v", o)
	}
	if len(o.Items[0].Warnings) != 1 || o.Items[0].Warnings[0].Code != "dairy" || len(o.Items[1].Warnings) != 0 {
		t.Fatalf("warnings = %+v / %+v", o.Items[0].Warnings, o.Items[1].Warnings)
	}
	if o.Subtotal != 38000+2*48000 {
		t.Fatalf("subtotal = %d", o.Subtotal)
	}

	// Laras avoids nothing: the same latte has no warning (BR-R-D1: warnings follow the orderer).
	if _, err := f.svc.PlaceOrder(ctx, laras, []ordering.ItemInput{f.latteLine(0, 0)}); err != nil {
		t.Fatal(err)
	}
	lme, _ := f.svc.Me(ctx, laras)
	if len(lme.Orders) != 1 || len(lme.Orders[0].Items[0].Warnings) != 0 {
		t.Fatalf("Laras sees %d orders / warnings %v", len(lme.Orders), lme.Orders[0].Items[0].Warnings)
	}
	// Soy milk for Raka still contains Dairy from the Product itself (BR-R-D4: never removes tags).
	q, _ := f.svc.Quote(ctx, raka, []ordering.ItemInput{f.latteLine(1, 1)})
	if !slices.Equal(q.Lines[0].WarningTagIDs, []string{f.dairy}) || q.Total != 41000+5000 {
		t.Fatalf("quote = %+v", q)
	}
}

// BR-CM-04, BR-R-12, OP-02 K1: Sold Out and Hidden lines are rejected by the server re-check.
func TestServerRecheckRejectsUnavailableLines(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, g := f.join(t, "Bima")
	if _, err := f.cat.SetSoldOut(ctx, f.actor, f.nigiri.Variants[0].ID, true); err != nil {
		t.Fatal(err)
	}
	hidden := f.latte
	hidden.Status = catalog.StatusHidden
	if _, err := f.cat.SaveProduct(ctx, f.actor, hidden); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.PlaceOrder(ctx, g, []ordering.ItemInput{f.latteLine(0, 0), {VariantID: f.nigiri.Variants[0].ID, Quantity: 1}})
	var p *httpx.Problem
	if !errors.As(err, &p) {
		t.Fatalf("want problem, got %v", err)
	}
	codes := []string{}
	for _, ip := range p.Extra["items"].([]ordering.ItemProblem) {
		codes = append(codes, ip.Code)
	}
	if !slices.Equal(codes, []string{"unavailable", "sold_out"}) {
		t.Fatalf("codes = %v", codes)
	}
	me, _ := f.svc.Me(ctx, g)
	if len(me.Orders) != 0 {
		t.Fatal("a rejected Order must save nothing")
	}
	// Missing required Modifier (latte is Hidden now, so use a fresh Active copy's rule on the nigiri side).
	hidden.Status = catalog.StatusActive
	if _, err := f.cat.SaveProduct(ctx, f.actor, hidden); err != nil {
		t.Fatal(err)
	}
	q, err := f.svc.Quote(ctx, g, []ordering.ItemInput{{VariantID: f.latte.Variants[0].ID, Quantity: 1}})
	if err != nil || q.Lines[0].Problem == nil || q.Lines[0].Problem.Code != "modifier_rule" {
		t.Fatalf("missing required Modifier: %+v %v", q.Lines, err)
	}
}

// BR-PC-08, TR-DM-02: a price change applies to new Orders only.
func TestPriceSnapshot(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, g := f.join(t, "Citra")
	line := ordering.ItemInput{VariantID: f.nigiri.Variants[0].ID, Quantity: 1}
	if _, err := f.svc.PlaceOrder(ctx, g, []ordering.ItemInput{line}); err != nil {
		t.Fatal(err)
	}
	p := f.nigiri
	p.Variants[0].Price = 52000
	p.NameID = "Salmon Aburi Spesial"
	if _, err := f.cat.SaveProduct(ctx, f.actor, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PlaceOrder(ctx, g, []ordering.ItemInput{line}); err != nil {
		t.Fatal(err)
	}
	me, _ := f.svc.Me(ctx, g)
	first, second := me.Orders[0].Items[0], me.Orders[1].Items[0]
	if first.UnitPrice != 48000 || first.ProductName.ID != "Salmon Aburi" {
		t.Fatalf("existing item changed: %+v", first)
	}
	if second.UnitPrice != 52000 || second.ProductName.ID != "Salmon Aburi Spesial" {
		t.Fatalf("new item ignores the change: %+v", second)
	}
}

// BR-CM-14, G-03: a Customer edits or Cancels their own Order before Claim, never someone
// else's, and never after Claim.
func TestEditAndCancelBeforeClaim(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, ayu := f.join(t, "Ayu")
	_, other := f.join(t, "Reza")
	nigiri := ordering.ItemInput{VariantID: f.nigiri.Variants[0].ID, Quantity: 1}
	id, err := f.svc.PlaceOrder(ctx, ayu, []ordering.ItemInput{f.latteLine(0, 0), nigiri})
	if err != nil {
		t.Fatal(err)
	}
	me, _ := f.svc.Me(ctx, ayu)
	latteID, nigiriID := me.Orders[0].Items[0].ID, me.Orders[0].Items[1].ID

	if err := f.svc.EditOrder(ctx, other, id, nil); problemCode(err) != "order_not_found" {
		t.Fatalf("other customer edit: %v", err)
	}
	// Drop the latte, make it 3 nigiri, add an iced latte.
	if err := f.svc.EditOrder(ctx, ayu, id, []ordering.ItemInput{{ID: nigiriID, Quantity: 3}, f.latteLine(1, 0)}); err != nil {
		t.Fatal(err)
	}
	me, _ = f.svc.Me(ctx, ayu)
	byID := map[string]ordering.ItemView{}
	for _, it := range me.Orders[0].Items {
		byID[it.ID] = it
	}
	if byID[latteID].Status != "cancelled" || byID[latteID].CancelReason != "customer_request" {
		t.Fatalf("dropped latte = %+v", byID[latteID])
	}
	if byID[nigiriID].Quantity != 3 || len(me.Orders[0].Items) != 3 || me.Orders[0].Subtotal != 3*48000+41000 {
		t.Fatalf("after edit: %+v", me.Orders[0])
	}

	// Once Claimed, the Customer must ask the Waiter.
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.Exec(ctx, "UPDATE orders SET claimed_by = ?, claimed_at = ? WHERE id = ?", f.actor, platform.Millis(t0), id)
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CancelOrder(ctx, ayu, id); problemCode(err) != "order_claimed" {
		t.Fatalf("cancel after claim: %v", err)
	}
	me, _ = f.svc.Me(ctx, ayu)
	if me.Orders[0].Status != "claimed" || me.Orders[0].Editable {
		t.Fatalf("claimed order = %s editable=%v", me.Orders[0].Status, me.Orders[0].Editable)
	}

	// A second Order can be fully Cancelled.
	id2, _ := f.svc.PlaceOrder(ctx, ayu, []ordering.ItemInput{nigiri})
	if err := f.svc.CancelOrder(ctx, ayu, id2); err != nil {
		t.Fatal(err)
	}
	me, _ = f.svc.Me(ctx, ayu)
	if me.Orders[1].Status != "cancelled" {
		t.Fatalf("cancelled order = %s", me.Orders[1].Status)
	}
}

// BR-CM-15, G-04: while Settling, new Orders are blocked.
func TestSettlingBlocksOrdering(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, g := f.join(t, "Gilang")
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.SetBillStatus(ctx, store.SetBillStatusParams{Status: ordering.BillSettling, UpdatedAt: platform.Millis(t0), ID: s.BillID})
	}); err != nil {
		t.Fatal(err)
	}
	g, _ = f.svc.Authenticate(ctx, s.Token) // still signed in while Settling
	if _, err := f.svc.PlaceOrder(ctx, g, []ordering.ItemInput{{VariantID: f.nigiri.Variants[0].ID, Quantity: 1}}); problemCode(err) != "bill_settling" {
		t.Fatalf("want bill_settling, got %v", err)
	}
	st, _ := f.svc.Lookup(ctx, f.qr)
	if !st.Settling {
		t.Fatal("lookup should show Settling")
	}
}

// BR-R-26, G-01: an empty Bill closes after the empty-Bill time; one with an Order does not.
// Tokens for the closed Bill stop working and the Table is free again.
func TestEmptyBillAutoClose(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	empty, _ := f.join(t, "Wulan")
	s2, err := f.svc.Join(ctx, f.qr2, ordering.JoinInput{Name: "Bima"})
	if err != nil {
		t.Fatal(err)
	}
	g2, _ := f.svc.Authenticate(ctx, s2.Token)
	if _, err := f.svc.PlaceOrder(ctx, g2, []ordering.ItemInput{{VariantID: f.nigiri.Variants[0].ID, Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(29 * time.Minute)
	if closed, _ := f.svc.CloseEmptyBills(ctx); len(closed) != 0 {
		t.Fatalf("closed too early: %v", closed)
	}
	f.clock.Advance(2 * time.Minute)
	closed, err := f.svc.CloseEmptyBills(ctx)
	if err != nil || !slices.Equal(closed, []string{empty.BillID}) {
		t.Fatalf("closed = %v %v", closed, err)
	}
	if _, err := f.svc.Authenticate(ctx, empty.Token); problemCode(err) != "bill_closed" {
		t.Fatalf("token after close: %v", err)
	}
	if st, _ := f.svc.Lookup(ctx, f.qr); st.State != "free" {
		t.Fatalf("table state = %s", st.State)
	}
	next, _ := f.svc.Join(ctx, f.qr, ordering.JoinInput{Name: "Tamu baru"})
	if next.BillID == empty.BillID || !next.FirstScan {
		t.Fatal("next scan must open a new Bill")
	}
}

// TR-RET-01, BR-P-03: 48 h after closing, names and dietary data are gone; warnings stay.
func TestAnonymisation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, g := f.join(t, "Dewi", f.dairy)
	if _, err := f.svc.PlaceOrder(ctx, g, []ordering.ItemInput{f.latteLine(0, 0, f.dairy)}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.SetBillStatus(ctx, store.SetBillStatusParams{Status: ordering.BillClosed, ClosedAt: nullMs(t0), UpdatedAt: platform.Millis(t0), ID: s.BillID})
	}); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(47 * time.Hour)
	if n, _ := f.svc.Anonymise(ctx); n != 0 {
		t.Fatal("anonymised before 48 h")
	}
	f.clock.Advance(2 * time.Hour)
	if n, err := f.svc.Anonymise(ctx); n != 1 || err != nil {
		t.Fatalf("anonymise = %d %v", n, err)
	}
	row, _ := f.db.Read().GetCustomerEntry(ctx, s.CustomerID)
	if row.NameEnc != nil || row.RestrictionTagsEnc != nil {
		t.Fatal("customer data kept")
	}
	w, _ := f.db.Read().ListDietaryWarningsByBill(ctx, s.BillID)
	if len(w) != 1 {
		t.Fatal("Dietary Warning rows must stay")
	}
}

func TestOrderStatusDerivation(t *testing.T) {
	cases := []struct {
		stored  string
		claimed bool
		items   []string
		want    string
	}{
		{"pending", false, []string{"pending"}, "pending"},
		{"pending", true, []string{"pending"}, "claimed"},
		{"confirmed", true, []string{"confirmed", "preparing"}, "preparing"},
		{"confirmed", true, []string{"ready", "served", "cancelled"}, "ready"},
		{"confirmed", true, []string{"served", "voided"}, "served"},
		{"pending", false, []string{"cancelled"}, "cancelled"},
	}
	for _, c := range cases {
		if got := ordering.OrderStatus(c.stored, c.claimed, c.items); got != c.want {
			t.Errorf("%v → %s, want %s", c, got, c.want)
		}
	}
}

func nullMs(t time.Time) sql.NullInt64 { return sql.NullInt64{Int64: platform.Millis(t), Valid: true} }
