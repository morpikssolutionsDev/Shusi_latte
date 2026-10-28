package seed_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/display"
	"sushilatte/api/internal/hris"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/seed"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
	"sushilatte/api/internal/vault"
)

// DEMO-AND-SEED §1 principle 1: every scenario is live at T0. Builds the real demo database and
// checks the Table states a presenter relies on.
func TestDemoSeedScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the full demo database and its images")
	}
	ctx := context.Background()
	t0 := time.Date(2026, 10, 27, 15, 0, 0, 0, platform.Jakarta)
	v := testutil.NewVault(t)
	opts := buildOptions(t, t0, v, true)
	if err := seed.Build(ctx, opts); err != nil {
		t.Fatal(err)
	}
	db, err := dbx.Open(ctx, opts.DatabaseURL, opts.Schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := ordering.NewService(db, testutil.NewClock(t0), v)
	admin := &auth.Principal{Kind: "crew", Role: auth.RoleOutletAdmin}
	floor, err := svc.Floor(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	card := func(n int64) ordering.TableCard {
		i := slices.IndexFunc(floor, func(c ordering.TableCard) bool { return c.Number == n })
		return floor[i]
	}
	bill := func(n int64) ordering.StaffBill {
		b, err := svc.Bill(ctx, card(n).Bill.ID)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	if card(1).Bill != nil || card(12).Active {
		t.Fatal("Meja 1 must be free and Meja 12 inactive")
	}
	if b := bill(2); len(b.Orders) != 0 {
		t.Fatal("Meja 2 must have an empty Bill")
	}
	// Meja 3: Laras served, Raka pending with an acknowledged Dairy warning.
	b3 := bill(3)
	if b3.Orders[0].Status != "served" || b3.Orders[1].Status != "pending" || len(b3.Orders[1].Items[0].Warnings) != 1 {
		t.Fatalf("Meja 3: %s / %s", b3.Orders[0].Status, b3.Orders[1].Status)
	}
	// Meja 5: Claimed by W001, Iced Hojicha flagged Sold Out, Sakura flagged unavailable.
	o5 := bill(5).Orders[0]
	if o5.Status != "claimed" || o5.ClaimedBy.CrewID != "W001" || !o5.Items[0].SoldOutFlag || !o5.Items[1].UnavailableFlag {
		t.Fatalf("Meja 5: %+v", o5)
	}
	// Meja 6: gyoza late (Confirmed 11 of 10 min), ramen Preparing, Dewi has an Estimated Wait.
	b6 := bill(6)
	if b6.Orders[0].Items[0].Status != "confirmed" || b6.Orders[1].Items[0].Status != "preparing" || b6.Orders[0].EstimatedWait == nil {
		t.Fatalf("Meja 6: %+v", b6.Orders)
	}
	// Meja 7: the Remake is first in the Kitchen queue; Dragon Roll waits for pickup.
	q, err := svc.Queue(ctx, "kitchen")
	if err != nil {
		t.Fatal(err)
	}
	if !q.Tickets[0].Remake || q.Tickets[0].Tables[0] != "Meja 7" || !slices.ContainsFunc(q.Ready, func(k ordering.KitchenTicket) bool { return k.Tables[0] == "Meja 7" }) {
		t.Fatalf("kitchen queue: first=%+v ready=%d", q.Tickets[0], len(q.Ready))
	}
	// Meja 8 + 9: one Settling Bill with cash Held by Waiter and a balance left.
	b8 := bill(8)
	if b8.Status != "settling" || len(b8.Tables) != 2 || b8.Billing.Payments[0].Status != ordering.PayHeldByWaiter || b8.Billing.Balance <= 0 {
		t.Fatalf("Meja 8+9: %s tables=%d %+v", b8.Status, len(b8.Tables), b8.Billing)
	}
	if card(9).Bill == nil || card(9).Bill.ID != b8.ID {
		t.Fatal("Meja 9 must show the merged Bill")
	}
	// Meja 10: Sub-bill 1 closed by QRIS; Sub-bill 2 open with the Voided cheesecake.
	b10 := bill(10).Billing
	if len(b10.SubBills) != 2 || b10.SubBills[0].Status != "closed" || b10.SubBills[1].Status != "open" {
		t.Fatalf("Meja 10: %+v", b10.SubBills)
	}
	// Meja 11: Ayu's own Cancel, Kitchen Cancels, curry Preparing.
	reasons := map[string]string{}
	for _, it := range bill(11).Orders[0].Items {
		reasons[it.ProductName.EN] = it.Status + "/" + it.CancelReason
	}
	if reasons["Truffle Edamame"] != "cancelled/customer_request" || reasons["Hojicha Caramel Latte"] != "cancelled/out_of_stock" ||
		reasons["Matcha Mille-Crêpe"] != "cancelled/wrong_order" || reasons["Chicken Katsu Curry"] != "preparing/" {
		t.Fatalf("Meja 11: %v", reasons)
	}
	// Meja 4: Bima's call and Order wait for a Waiter.
	feed, _ := svc.Feed(ctx, admin)
	if len(feed.Calls) != 1 || feed.Calls[0].Table.Label != "Meja 4" {
		t.Fatalf("calls: %+v", feed.Calls)
	}

	checkCashier(t, ctx, svc, db, t0)
	checkHistory(t, ctx, svc, db, t0)
	checkHRIS(t, ctx, db, t0)
	checkCatalogue(t, ctx, svc, db, t0, card)

	// TR-RET-04: data keys older than three days are destroyed with the seed, as nightly.
	if ids := v.KeyIDs(); len(ids) > 4 {
		t.Fatalf("data keys kept: %v", ids)
	}
	// G-24 in the report Management opens: −47h still named, −49h "Customer".
	named, anonymised := false, false
	for _, d := range []time.Duration{-47 * time.Hour, -49 * time.Hour} {
		rows, err := svc.BillHistory(ctx, ordering.BusinessDate(t0.Add(d)))
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ClosedAt == platform.Millis(t0.Add(d)) {
				named = named || (!r.Anonymised && slices.Contains(r.Customers, "Galih"))
				anonymised = anonymised || (r.Anonymised && slices.Contains(r.Customers, "Customer"))
			}
		}
	}
	if !named || !anonymised {
		t.Fatalf("bill history: named −47h %v, anonymised −49h %v", named, anonymised)
	}
}

// DEMO-AND-SEED §8 HRIS: today's attendance, last night's auto clock-out, clock-out blockers,
// leave and the lockout.
func checkHRIS(t *testing.T, ctx context.Context, db *dbx.DB, t0 time.Time) {
	t.Helper()
	clock := testutil.NewClock(t0)
	hr := hris.NewService(db, clock, auth.NewService(db, clock))
	// Today's shift is the one around T0 (it starts the evening before when T0 is after midnight).
	from := t0.AddDate(0, 0, -2).Format(time.DateOnly)
	rows, err := hr.Attendance(ctx, from, t0.Format(time.DateOnly))
	if err != nil {
		t.Fatal(err)
	}
	now := platform.Millis(t0)
	current := func(code string) hris.AttendanceRow {
		for _, r := range rows {
			if r.Code == code && r.Shift != nil && r.Shift.StartsAt <= now && now < r.Shift.EndsAt {
				return r
			}
		}
		return hris.AttendanceRow{}
	}
	if r := current("W004"); r.Status != "late" {
		t.Fatalf("W004 today: %+v", r)
	}
	if r := current("W003"); r.Status != "working" || r.LateMinutes != 7 {
		t.Fatalf("W003 today: %+v", r)
	}
	if r := current("W001"); r.Status != "working" {
		t.Fatalf("W001 today: %+v", r)
	}
	if !slices.ContainsFunc(rows, func(r hris.AttendanceRow) bool { return r.Code == "K002" && r.Status == "auto" }) {
		t.Fatal("K002 has no auto clock-out last night")
	}
	// W002 holds the Meja 8 cash (G-20); W001 has Meja 5 and Meja 7 Claims (G-06).
	_, err = hr.Clock(ctx, hris.ClockInput{CrewID: "W002", PIN: "333333"})
	var p *httpx.Problem
	if !errors.As(err, &p) || p.Code != "cash_held" || p.Detail != "Setor Rp 200.000 ke kasir dulu." {
		t.Fatalf("W002 clock-out: %v", err)
	}
	_, err = hr.Clock(ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	if !errors.As(err, &p) || p.Code != "open_claims" {
		t.Fatalf("W001 clock-out: %v", err)
	}
	var tables []string
	for _, c := range p.Extra["claims"].([]hris.OpenClaim) {
		tables = append(tables, c.Table)
	}
	if !slices.Contains(tables, "Meja 5") || !slices.Contains(tables, "Meja 7") {
		t.Fatalf("W001 open Claims: %v", tables)
	}
	leave, _ := hr.Leave(ctx, &auth.Principal{Kind: "crew", Role: auth.RoleOutletAdmin})
	byCode := map[string]string{}
	for _, l := range leave {
		byCode[l.Code] = l.Status
	}
	if byCode["W004"] != "pending" || byCode["K001"] != "approved" || byCode["C002"] != "rejected" {
		t.Fatalf("leave: %v", byCode)
	}
	if _, err := hr.Clock(ctx, hris.ClockInput{CrewID: "W005", PIN: "333333"}); !errors.As(err, &p) || p.Code != "account_locked" {
		t.Fatalf("W005 lockout: %v", err)
	}
}

// DEMO-AND-SEED §6.2, §6.3, §8: the Cash Drawer, Takeaways and today's closed Bills at T0.
func checkCashier(t *testing.T, ctx context.Context, svc *ordering.Service, db *dbx.DB, t0 time.Time) {
	t.Helper()
	d, err := svc.Drawer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Session == nil || d.Session.Cashier != "Maya Anggraini" || d.Session.OpeningFloat != 500000 {
		t.Fatalf("drawer session: %+v", d.Session)
	}
	if len(d.Handovers) != 2 || d.Handovers[0].Expected != 312000 || d.Handovers[0].Difference != 0 ||
		d.Handovers[1].Expected != 186000 || d.Handovers[1].Difference != -20000 {
		t.Fatalf("handovers: %+v", d.Handovers)
	}
	if len(d.Held) != 1 || d.Held[0].Code != "W002" || d.Held[0].Amount != 200000 {
		t.Fatalf("held cash: %+v", d.Held)
	}
	if len(d.Pending) != 1 || d.Pending[0].Amount != 94300 {
		t.Fatalf("pending QRIS: %+v", d.Pending)
	}
	e := d.Expected
	if e.Handovers != 478000 || e.Refunds != 612000 || e.Total != e.OpeningFloat+e.Handovers+e.CounterTendered-e.Change-e.Refunds || e.Total <= 0 {
		t.Fatalf("expected cash: %+v", e)
	}

	// TA-1 paid and partly Ready, TA-2 all Ready, TA-3 held unpaid out of the Kitchen.
	bills, err := svc.CashierBills(ctx, &auth.Principal{Kind: "crew", Role: auth.RoleCrew})
	if err != nil {
		t.Fatal(err)
	}
	ta := map[string]string{}
	for _, b := range bills {
		if len(b.Tables) == 1 && strings.HasPrefix(b.Tables[0].Label, "Bawa pulang · ") {
			ta[strings.TrimPrefix(b.Tables[0].Label, "Bawa pulang · ")] = b.Status
		}
	}
	if ta["Sari"] != "closed" || ta["Hana"] != "closed" || ta["Joko"] != "open" {
		t.Fatalf("takeaways: %v", ta)
	}
	bar, _ := svc.Queue(ctx, "bar")
	tickets := map[string]int{}
	for _, k := range append(bar.Tickets, bar.Ready...) {
		tickets[k.Tables[0]]++
	}
	if tickets["Bawa pulang · Sari"] != 2 || tickets["Bawa pulang · Hana"] != 1 || tickets["Bawa pulang · Joko"] != 0 {
		t.Fatalf("bar tickets: %v", tickets)
	}

	today := ordering.BusinessDate(t0)
	sales, err := svc.DailySales(ctx, today)
	if err != nil {
		t.Fatal(err)
	}
	if sales.Bills < 15 || sales.UnpaidBills != 1 {
		t.Fatalf("today's sales: %d bills, %d unpaid", sales.Bills, sales.UnpaidBills)
	}
	ops, err := svc.Operations(ctx, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops.Refunds) != 2 || len(ops.Unpaid) != 1 || ops.Unpaid[0].By != "Rina Kusuma" || len(ops.Remakes) != 1 || len(ops.ShiftClosings) != 1 {
		t.Fatalf("today's operations: %+v", ops)
	}
}

// DEMO-AND-SEED §7: 14 days of history.
func checkHistory(t *testing.T, ctx context.Context, svc *ordering.Service, db *dbx.DB, t0 time.Time) {
	t.Helper()
	unpaid := 0
	for d := 1; d <= 14; d++ {
		date := ordering.BusinessDate(t0.AddDate(0, 0, -d))
		sales, err := svc.DailySales(ctx, date)
		if err != nil {
			t.Fatal(err)
		}
		unpaid += sales.UnpaidBills
		if sales.Bills < 40 || sales.Bills > 75 {
			t.Fatalf("%s: %d Bills", date, sales.Bills)
		}
		prep, _ := svc.PrepTime(ctx, date)
		if prep.Share < 0.82 || prep.Share > 0.97 {
			t.Fatalf("%s: Prep Time on time %.2f", date, prep.Share)
		}
		ops, _ := svc.Operations(ctx, date)
		if len(ops.ShiftClosings) != 1 || ops.ShiftClosings[0].ClosedAt == nil {
			t.Fatalf("%s: shift closings %+v", date, ops.ShiftClosings)
		}
		switch d {
		case 1:
			if *ops.ShiftClosings[0].Difference != 0 || len(ops.PostOutage) != 2 {
				t.Fatalf("yesterday: difference %d, %d post-outage", *ops.ShiftClosings[0].Difference, len(ops.PostOutage))
			}
		case 2:
			if *ops.ShiftClosings[0].Difference != 5000 {
				t.Fatalf("day before: difference %d", *ops.ShiftClosings[0].Difference)
			}
		}
	}
	if unpaid != 1 {
		t.Fatalf("history unpaid Bills: %d", unpaid)
	}

	// Anonymisation: closed at −47h keeps its Customer name, at −49h it is gone (G-24).
	for _, c := range []struct {
		closed time.Duration
		named  bool
	}{{-47 * time.Hour, true}, {-49 * time.Hour, false}} {
		at := platform.Millis(t0.Add(c.closed))
		bills, err := db.Read().ListClosedBillsBetween(ctx, store.ListClosedBillsBetweenParams{ClosedAt: sql.NullInt64{Int64: at, Valid: true},
			ClosedAt_2: sql.NullInt64{Int64: at + 1, Valid: true}})
		if err != nil || len(bills) != 1 {
			t.Fatalf("Bill closed at %s: %v %d", c.closed, err, len(bills))
		}
		b, _ := svc.Bill(ctx, bills[0].ID)
		named := len(b.Customers) == 1 && b.Customers[0].Name == "Galih"
		if named != c.named {
			t.Fatalf("Bill closed at %s: customers %+v", c.closed, b.Customers)
		}
	}
}

// DEMO-AND-SEED §9: the nightly reset seeds at 03.00 WIB, before the 04.00 business-day
// turnover; every story must still fit (today's Bills compressed, shifts around T0).
func TestDemoSeedAtNightlyReset(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the full demo database")
	}
	ctx := context.Background()
	t0 := time.Date(2026, 10, 28, 3, 0, 0, 0, platform.Jakarta)
	v := testutil.NewVault(t)
	opts := buildOptions(t, t0, v, true)
	if err := seed.Build(ctx, opts); err != nil {
		t.Fatal(err)
	}
	db, err := dbx.Open(ctx, opts.DatabaseURL, opts.Schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := ordering.NewService(db, testutil.NewClock(t0), v)
	d, _ := svc.Drawer(ctx)
	if d.Session == nil || len(d.Pending) != 1 || len(d.Handovers) != 2 {
		t.Fatalf("drawer at 03.00: %+v", d)
	}
	checkHRIS(t, ctx, db, t0)
}

// DEMO-AND-SEED §11, Phase 8: the scenario catalogue rows not covered above are visible at T0.
func checkCatalogue(t *testing.T, ctx context.Context, svc *ordering.Service, db *dbx.DB, t0 time.Time, card func(int64) ordering.TableCard) {
	t.Helper()
	q := db.Read()
	audits := func(prefix string) int {
		rows, err := q.SearchAudit(ctx, store.SearchAuditParams{ActionPrefix: sql.NullString{String: prefix, Valid: true}, RowLimit: 50})
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	// G-06 takeover, G-15 reprint, Move audit (§6.3).
	if audits("claim.takeover") < 1 || audits("receipt.reprinted") < 1 || audits("bill.moved") < 1 {
		t.Fatalf("audit: takeover %d, reprint %d, move %d", audits("claim.takeover"), audits("receipt.reprinted"), audits("bill.moved"))
	}
	a001, _ := q.GetCrewByCrewID(ctx, "A001")
	admin := &auth.Principal{ID: a001.ID, CrewID: "A001", Kind: "crew", Role: auth.RoleOutletAdmin}
	// G-11: moving Meja 6 onto occupied Meja 3 is refused; G-12: splitting Meja 8+9 after a Payment too.
	var p *httpx.Problem
	if err := svc.Move(ctx, admin, card(6).Bill.ID, card(6).ID, card(3).ID); !errors.As(err, &p) || p.Code != "table_occupied" {
		t.Fatalf("G-11 move onto Meja 3: %v", err)
	}
	if err := svc.Split(ctx, admin, card(8).Bill.ID, ordering.SplitInput{Method: "equal", Count: 2}); !errors.As(err, &p) || p.Code != "bill_has_payments" {
		t.Fatalf("G-12 split after payment: %v", err)
	}
	// G-13: a Bill closed today as pay now, serve later.
	day := t0.Add(-8 * time.Hour)
	closed, _ := q.ListClosedBillsBetween(ctx, store.ListClosedBillsBetweenParams{ClosedAt: sql.NullInt64{Int64: platform.Millis(day), Valid: true},
		ClosedAt_2: sql.NullInt64{Int64: platform.Millis(t0), Valid: true}})
	if !slices.ContainsFunc(closed, func(b store.Bill) bool { return b.PayNowServeLaterBy.Valid && b.Type == "dine_in" }) {
		t.Fatal("G-13: no Table Bill closed as pay now, serve later")
	}
	// Sold Out cascade: Hojicha Iced from the Kitchen Cancel, Unagi 4 pcs set by hand.
	cat, err := catalog.Load(ctx, q, false)
	if err != nil {
		t.Fatal(err)
	}
	soldOut := map[string]bool{}
	for _, pr := range cat.Products {
		for _, v := range pr.Variants {
			if v.SoldOut {
				soldOut[pr.NameEN+" "+v.NameEN] = true
			}
		}
	}
	if !soldOut["Hojicha Caramel Latte Iced"] || !soldOut["Unagi Kabayaki Nigiri 4 pcs"] {
		t.Fatalf("Sold Out variants: %v", soldOut)
	}
	// Product Display skips the Hidden Sakura Latte's featured slide (playlist slide 5).
	slides, err := q.ListSlides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	playlist, err := display.NewService(db, testutil.NewClock(t0)).Playlist(ctx)
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, sl := range slides {
		if sl.Active == 1 {
			active++
		}
	}
	if len(playlist.Slides) >= active {
		t.Fatalf("playlist shows %d of %d active slides: nothing skipped", len(playlist.Slides), active)
	}
}

// buildOptions seeds into a throwaway schema; processed photos are cached between runs.
func buildOptions(t *testing.T, t0 time.Time, v *vault.Vault, story bool) seed.Options {
	t.Helper()
	url, schema := testutil.NewSchema(t)
	return seed.Options{DatabaseURL: url, Schema: schema, ImagesDir: filepath.Join(os.TempDir(), "sushi-latte-seed-images"),
		AssetsDir: filepath.Join("..", "..", "..", "docs", "images"), T0: t0, Vault: v, Story: story}
}

// DEMO-AND-SEED §9 (decided 2026-10-28): the default seed is the clean start — menu, Crew,
// Tables, discounts, Product Display and settings — with no Bills, cash, HRIS records or lockout.
func TestCleanSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the demo database and its images")
	}
	ctx := context.Background()
	t0 := time.Date(2026, 10, 28, 9, 0, 0, 0, platform.Jakarta)
	opts := buildOptions(t, t0, testutil.NewVault(t), false)
	if err := seed.Build(ctx, opts); err != nil {
		t.Fatal(err)
	}
	db, err := dbx.Open(ctx, opts.DatabaseURL, opts.Schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	count := func(table string) int {
		var n int
		if err := db.Write(ctx, func(tx *dbx.Tx) error { return tx.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n) }); err != nil {
			t.Fatal(err)
		}
		return n
	}
	kept := map[string]int{"crew": 12, "dining_table": 12, "category": 8, "discount": 5, "display_slide": 1}
	for table, min := range kept {
		if n := count(table); n < min {
			t.Errorf("%s: %d rows, want at least %d", table, n, min)
		}
	}
	for _, table := range []string{"bill", "orders", "order_item", "payment", "refund", "drawer_session", "cash_handover", "customer_entry",
		"waiter_call", "shift", "attendance", "leave_request", "employee", "session"} {
		if n := count(table); n != 0 {
			t.Errorf("%s: %d rows in the clean start", table, n)
		}
	}
	q := db.Read()
	if seeded, _ := q.GetDemoSeed(ctx); seeded.Story != 0 {
		t.Error("clean start marked as the story")
	}
	cat, err := catalog.Load(ctx, q, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range cat.Products {
		if p.NameEN == "Ebi Tempura Roll" && p.Status != catalog.StatusArchived {
			t.Error("Ebi Tempura Roll should be retired")
		}
		for _, v := range p.Variants {
			if v.SoldOut {
				t.Errorf("%s %s is Sold Out in the clean start", p.NameEN, v.NameEN)
			}
		}
	}
	if w5, _ := q.GetCrewByCrewID(ctx, "W005"); w5.LockedUntil.Valid {
		t.Error("W005 is locked in the clean start")
	}
}
