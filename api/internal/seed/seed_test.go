package seed_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/seed"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/testutil"
)

// DEMO-AND-SEED §1 principle 1: every scenario is live at T0. Builds the real demo database and
// checks the Table states a presenter relies on.
func TestDemoSeedScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the full demo database and its images")
	}
	ctx := context.Background()
	dir := t.TempDir()
	cache := filepath.Join(os.TempDir(), "sushi-latte-seed-images") // processed photos are reused between runs
	t0 := time.Date(2026, 10, 27, 15, 0, 0, 0, platform.Jakarta)
	v := testutil.NewVault(t)
	opts := seed.Options{DBPath: filepath.Join(dir, "demo.db"), ImagesDir: cache, AssetsDir: filepath.Join("..", "..", "..", "docs", "images"), T0: t0, Vault: v}
	if err := seed.Build(ctx, opts); err != nil {
		t.Fatal(err)
	}
	db, err := dbx.Open(ctx, opts.DBPath, nil)
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
