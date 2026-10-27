package ordering_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/store"
)

func (f fixture) cashier(t *testing.T) *auth.Principal {
	t.Helper()
	ctx := context.Background()
	var p *auth.Principal
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		id, err := auth.CreateCrew(ctx, tx, t0, auth.NewCrew{CrewID: "C001", FullName: "Maya", Role: auth.RoleCrew, Stations: []string{auth.StationCashier}, PIN: "444444"})
		p = &auth.Principal{ID: id, CrewID: "C001", Name: "Maya", Role: auth.RoleCrew, Stations: []string{auth.StationCashier}, Kind: "crew"}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

// served moves every Confirmed item of a Bill to Served, as the Kitchen and Waiter would.
func (f fixture) served(t *testing.T, billID string, by *auth.Principal) {
	t.Helper()
	ctx := context.Background()
	now := sql.NullInt64{Int64: platform.Millis(f.clock.Now()), Valid: true}
	items, _ := f.db.Read().ListOrderItemsByBill(ctx, billID)
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		for _, it := range items {
			if it.Status != "confirmed" {
				continue
			}
			_, _ = tx.StartItem(ctx, store.StartItemParams{PreparingAt: now, UpdatedAt: now.Int64, ID: it.ID})
			_, _ = tx.ReadyItem(ctx, store.ReadyItemParams{ReadyAt: now, UpdatedAt: now.Int64, ID: it.ID})
			if _, err := tx.ServeItem(ctx, store.ServeItemParams{ServedAt: now, ServedBy: sql.NullString{String: by.ID, Valid: true}, UpdatedAt: now.Int64, ID: it.ID}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// directBill opens Meja n with a direct Order and returns its Bill id.
func (f fixture) directBill(t *testing.T, w *auth.Principal, table int, lines ...ordering.ItemInput) string {
	t.Helper()
	ctx := context.Background()
	tables, _ := f.db.Read().ListTables(ctx)
	id, err := f.svc.DirectOrder(ctx, w, "waiter", ordering.DirectInput{TableID: tables[table].ID, Items: lines})
	if err != nil {
		t.Fatal(err)
	}
	o, _ := f.db.Read().GetOrder(ctx, id)
	return o.BillID
}

// BR-R-18, BR-R-27, BR-WT-14, TR-PR-05: paying Settles the Bill; cash rounds to Rp 100 at the
// final payment with change; a Waiter's cash is Held by Waiter, a Cashier's is settled.
func TestPayCashRoundingAndClose(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	billID := f.directBill(t, s.w1, 0, f.latteLine(0, 0))
	bl, _ := f.svc.Billing(ctx, billID)
	// 38.000, service 5% = 1.900, tax 10% of 39.900 = 3.990 → 43.890.
	if bl.Totals.Total != 43890 {
		t.Fatalf("total = %+v", bl.Totals)
	}
	if err := f.svc.Close(ctx, s.w1, billID, "", false); problemCode(err) != "balance_due" {
		t.Fatalf("close unpaid: %v", err)
	}
	// Partial card payment, then cash for the rest.
	if _, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayCard, Amount: 20000}, ""); err != nil {
		t.Fatal(err)
	}
	b, _ := f.db.Read().GetBill(ctx, billID)
	if b.Status != ordering.BillSettling || !b.PricedAt.Valid {
		t.Fatalf("payment did not Settle: %s", b.Status)
	}
	res, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayCash, Amount: 30000}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Balance 23.890 → rounds to 23.900 (+10), change 6.100.
	pv := res.Payment
	if pv.Amount != 23900 || pv.Rounding != 10 || pv.Change != 6100 || pv.Status != ordering.PayHeldByWaiter || res.Billing.Balance != 0 {
		t.Fatalf("cash payment %+v balance %d", pv, res.Billing.Balance)
	}
	if err := f.svc.Close(ctx, s.w1, billID, "", false); problemCode(err) != "items_unfinished" {
		t.Fatalf("close with unfinished items: %v", err)
	}
	if err := f.svc.Close(ctx, s.w1, billID, "", true); err != nil {
		t.Fatal(err)
	}
	b, _ = f.db.Read().GetBill(ctx, billID)
	if b.Status != "closed" || b.Total.Int64 != 43890 || b.PayNowServeLaterBy.String != s.w1.ID {
		t.Fatalf("closed bill %+v", b)
	}
	floor, _ := f.svc.Floor(ctx, s.w1)
	if floor[0].Bill != nil {
		t.Fatal("Table not freed after close")
	}
	// A Cashier's cash goes straight into the drawer — which must be open (TR-PAY-01).
	c := f.cashier(t)
	billID2 := f.directBill(t, c, 1, f.nigiriLine())
	if _, err := f.svc.Pay(ctx, c, billID2, ordering.PayInput{Method: ordering.PayCash, Amount: 100000}, ""); problemCode(err) != "drawer_closed" {
		t.Fatalf("counter cash with a closed drawer: %v", err)
	}
	if err := f.svc.OpenDrawer(ctx, c, 500000); err != nil {
		t.Fatal(err)
	}
	res, _ = f.svc.Pay(ctx, c, billID2, ordering.PayInput{Method: ordering.PayCash, Amount: 100000}, "")
	if res.Payment.Status != ordering.PaySettled {
		t.Fatalf("cashier cash = %s", res.Payment.Status)
	}
}

// BR-R-30: unverified QRIS needs an Outlet Admin PIN and stays pending verification.
func TestQRISPendingNeedsPIN(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	billID := f.directBill(t, s.w1, 0, f.nigiriLine())
	if _, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayQRIS, Amount: 10000}, ""); problemCode(err) != "override_required" {
		t.Fatalf("unverified QRIS without PIN: %v", err)
	}
	tok, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionQRISUnverified)
	res, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayQRIS, Amount: 10000}, tok)
	if err != nil || res.Payment.Status != ordering.PayPending {
		t.Fatalf("pending QRIS: %+v %v", res.Payment, err)
	}
	if _, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayCard, Amount: 1_000_000}, ""); problemCode(err) != "invalid_input" {
		t.Fatalf("overpaying by card: %v", err)
	}
}

// BR-R-04, TR-PR-04, Phase 4 done-when: split equally and by item; shares match pricing to the Rupiah.
func TestSplitMatchesPricing(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	// A Bill discount so proration is exercised: Rp 25.000 off from Rp 100.000.
	minSub := int64(100000)
	if _, err := outlet.NewService(f.db, f.clock).SaveDiscount(ctx, f.actor, outlet.Discount{Name: "Paket Hemat", Kind: "nominal", Value: 25000,
		Scope: "bill", Trigger: "auto", MinSubtotal: &minSub, Active: true}); err != nil {
		t.Fatal(err)
	}
	billID := f.directBill(t, s.w1, 0, f.latteLine(1, 1), f.nigiriLine(), f.nigiriLine(), ordering.ItemInput{VariantID: f.nigiri.Variants[0].ID, Quantity: 3})
	full, _ := f.svc.Billing(ctx, billID)
	if full.Totals.BillDiscount != 25000 {
		t.Fatalf("bill discount = %d", full.Totals.BillDiscount)
	}

	// Equally in 3: totals are the full total divided, remainder on the last share.
	if err := f.svc.Split(ctx, s.w1, billID, ordering.SplitInput{Method: "equal", Count: 3}); err != nil {
		t.Fatal(err)
	}
	eq, _ := f.svc.Billing(ctx, billID)
	want := pricing.SplitEqually(full.Totals.Total, 3)
	var sum int64
	for i, sh := range eq.SubBills {
		if sh.Totals.Total != want[i] {
			t.Fatalf("equal share %d = %d, want %d", i, sh.Totals.Total, want[i])
		}
		sum += sh.Totals.Total
	}
	if sum != full.Totals.Total {
		t.Fatalf("equal shares sum %d ≠ %d", sum, full.Totals.Total)
	}

	// By item: latte + 1 nigiri | 1 nigiri + 3 nigiri.
	items, _ := f.db.Read().ListOrderItemsByBill(ctx, billID)
	assign := map[string]int{items[0].ID: 1, items[1].ID: 1, items[2].ID: 2, items[3].ID: 2}
	if err := f.svc.Split(ctx, s.w1, billID, ordering.SplitInput{Method: "by_item", Count: 2, Assign: assign}); err != nil {
		t.Fatal(err)
	}
	bi, _ := f.svc.Billing(ctx, billID)
	var bdSum, totalSum int64
	for _, sh := range bi.SubBills {
		base := sh.Totals.Subtotal - sh.Totals.ItemDiscount - sh.Totals.BillDiscount
		svc := pricing.ApplyRate(base, 500)
		tax := pricing.ApplyRate(base+svc, 1000)
		if sh.Totals.Service != svc || sh.Totals.Tax != tax || sh.Totals.Total != base+svc+tax {
			t.Fatalf("share %d not priced by pricing: %+v", sh.Index, sh.Totals)
		}
		bdSum += sh.Totals.BillDiscount
		totalSum += sh.Totals.Total
	}
	// 46.000 + 48.000 = 94.000 vs 4 × 48.000 = 192.000 → discount prorated 7.987 / 16.013 (sums to 25.000).
	if bdSum != 25000 || totalSum != bi.Totals.Total || bi.SubBills[0].Totals.BillDiscount != pricing.ProrateBillDiscount(25000, []int64{94000, 192000})[0] {
		t.Fatalf("by-item shares %+v", bi.SubBills)
	}

	// Pay and close share 1; then Split is refused (G-12) and a Void on share 1 needs a Refund (G-10).
	if _, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayCard, Amount: bi.SubBills[0].Balance}, ""); problemCode(err) != "invalid_input" {
		t.Fatalf("paying a split bill without a share: %v", err)
	}
	if _, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{SubBillID: bi.SubBills[0].ID, Method: ordering.PayCard, Amount: bi.SubBills[0].Balance}, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Split(ctx, s.w1, billID, ordering.SplitInput{Method: "equal", Count: 2}); problemCode(err) != "bill_has_payments" {
		t.Fatalf("split after payment: %v", err)
	}
	f.served(t, billID, s.w1)
	if err := f.svc.Close(ctx, s.w1, billID, bi.SubBills[0].ID, false); err != nil {
		t.Fatal(err)
	}
	vt, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionVoid)
	if err := f.svc.Void(ctx, s.w1, items[0].ID, "Salah", vt); problemCode(err) != "item_paid" {
		t.Fatalf("void on a paid share: %v", err)
	}
	if err := f.svc.Void(ctx, s.w1, items[2].ID, "Jatuh", vt); err != nil {
		t.Fatalf("void on an unpaid share: %v", err)
	}
	after, _ := f.svc.Billing(ctx, billID)
	if after.SubBills[0].Status != "closed" || after.SubBills[0].Totals.Total != bi.SubBills[0].Totals.Total || after.SubBills[1].Totals.Total >= bi.SubBills[1].Totals.Total {
		t.Fatalf("after void: %+v", after.SubBills)
	}
	if b, _ := f.db.Read().GetBill(ctx, billID); b.Status == "closed" {
		t.Fatal("bill closed with an open share")
	}
}

// BR-R-28, BR-R-03, TR-DM-03, TR-DM-06: Move refuses an occupied Table; Merge keeps every Order
// and Customer on the surviving Bill and the absorbed Customer keeps ordering.
func TestMoveAndMerge(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	tables, _ := f.db.Read().ListTables(ctx)
	_, g3 := f.join(t, "Gilang") // Meja 3
	if _, err := f.svc.PlaceOrder(ctx, g3, []ordering.ItemInput{f.nigiriLine()}); err != nil {
		t.Fatal(err)
	}
	s4, err := f.svc.Join(ctx, f.qr2, ordering.JoinInput{Name: "Reza"}) // Meja 4
	if err != nil {
		t.Fatal(err)
	}
	g4, _ := f.svc.Authenticate(ctx, s4.Token)
	if _, err := f.svc.PlaceOrder(ctx, g4, []ordering.ItemInput{f.nigiriLine()}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Move(ctx, s.w1, g3.BillID, "", tables[1].ID); problemCode(err) != "table_occupied" {
		t.Fatalf("move onto Meja 4: %v", err)
	}
	if err := f.svc.Merge(ctx, s.w1, g3.BillID, []string{g4.BillID}); err != nil {
		t.Fatal(err)
	}
	absorbed, _ := f.db.Read().GetBill(ctx, g4.BillID)
	if absorbed.Status != "merged" || absorbed.MergedIntoID.String != g3.BillID {
		t.Fatalf("absorbed bill %+v", absorbed)
	}
	g4, err = f.svc.Authenticate(ctx, s4.Token)
	if err != nil || g4.BillID != g3.BillID {
		t.Fatalf("Reza after merge: %+v %v", g4, err)
	}
	me, _ := f.svc.Me(ctx, g4)
	if len(me.Bill.Tables) != 2 || len(me.Orders) != 1 || me.Orders[0].Seq != 2 {
		t.Fatalf("Reza sees %+v", me)
	}
	b, _ := f.svc.Bill(ctx, g3.BillID)
	if len(b.Orders) != 2 || len(b.Customers) != 2 {
		t.Fatalf("merged bill orders=%d customers=%d", len(b.Orders), len(b.Customers))
	}
	// Move the merged Bill's Meja 4 to a free Meja 5.
	free, err := outlet.NewService(f.db, f.clock).SaveTable(ctx, f.actor, outlet.Table{Number: 5, Label: "Meja 5", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Move(ctx, s.w1, g3.BillID, tables[1].ID, free.ID); err != nil {
		t.Fatal(err)
	}
	me, _ = f.svc.Me(ctx, g4)
	if me.Bill.Tables[0].Label != "Meja 3" || me.Bill.Tables[1].Label != "Meja 5" {
		t.Fatalf("tables after move: %+v", me.Bill.Tables)
	}
}

// BR-R-32, G-14: an Unpaid Bill needs an Outlet Admin PIN and a reason, and frees the Table.
func TestUnpaidBill(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	billID := f.directBill(t, s.w1, 0, f.nigiriLine())
	if err := f.svc.CloseUnpaid(ctx, s.w1, billID, "Pelanggan pergi", ""); problemCode(err) != "override_required" {
		t.Fatalf("unpaid without PIN: %v", err)
	}
	tok, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionUnpaidBill)
	if err := f.svc.CloseUnpaid(ctx, s.w1, billID, "Pelanggan pergi", tok); err != nil {
		t.Fatal(err)
	}
	b, _ := f.db.Read().GetBill(ctx, billID)
	if b.Status != "unpaid" || b.UnpaidOverrideBy.String != s.admin.ID || b.Total.Int64 == 0 {
		t.Fatalf("unpaid bill %+v", b)
	}
}

// BR-R-17: a manual Bill discount needs a Management PIN.
func TestManualDiscount(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	d, err := outlet.NewService(f.db, f.clock).SaveDiscount(ctx, f.actor, outlet.Discount{Name: "Manual 10%", Kind: "percent", Value: 1000,
		Scope: "bill", Trigger: "manual", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	billID := f.directBill(t, s.w1, 0, f.nigiriLine())
	tok, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionManualDiscount)
	if tok != "" {
		t.Fatal("Outlet Admin approved a manual discount")
	}
	tok, _, err = s.auth.IssueOverride(ctx, s.w1, "M001", "111111", auth.ActionManualDiscount)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ApplyManualDiscount(ctx, s.w1, billID, d.ID, tok); err != nil {
		t.Fatal(err)
	}
	bl, _ := f.svc.Billing(ctx, billID)
	if bl.Totals.BillDiscount != 4800 || bl.ManualDiscount != "Manual 10%" {
		t.Fatalf("manual discount: %+v", bl)
	}
}

// TR-PRN-05, TR-PRN-08, BR-R-37: Customer copy carries names, Internal does not; a second print
// is a REPRINT and is audited.
func TestReceiptsAndReprint(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	_, g := f.join(t, "Intan")
	if _, err := f.svc.PlaceOrder(ctx, g, []ordering.ItemInput{f.latteLine(0, 0)}); err != nil {
		t.Fatal(err)
	}
	cust, err := f.svc.Receipt(ctx, s.w1, g.BillID, "", "customer")
	if err != nil {
		t.Fatal(err)
	}
	internal, _ := f.svc.Receipt(ctx, s.w1, g.BillID, "", "internal")
	if len(cust.Customers) != 1 || cust.Customers[0] != "Intan" || len(internal.Customers) != 0 || cust.Reprint || internal.Reprint {
		t.Fatalf("receipts %+v / %+v", cust, internal)
	}
	if len(cust.Lines) != 1 || cust.Lines[0].Modifiers[0] != "Susu segar" || cust.Totals.Total == 0 {
		t.Fatalf("receipt lines %+v", cust.Lines)
	}
	again, _ := f.svc.Receipt(ctx, s.w1, g.BillID, "", "customer")
	audits, _ := f.db.Read().SearchAudit(ctx, store.SearchAuditParams{ActionPrefix: "receipt.reprinted", RowLimit: 5})
	if !again.Reprint || len(audits) != 1 {
		t.Fatalf("reprint=%v audits=%d", again.Reprint, len(audits))
	}
}

// BR-ADM-11: daily sales from frozen totals; Unpaid reported apart.
func TestDailySales(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	billID := f.directBill(t, s.w1, 0, f.nigiriLine())
	f.served(t, billID, s.w1)
	bl, _ := f.svc.Billing(ctx, billID)
	if _, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayCard, Amount: bl.Balance}, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Close(ctx, s.w1, billID, "", false); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	rep, err := f.svc.DailySales(ctx, ordering.BusinessDate(f.clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Bills != 1 || rep.Totals.Total != bl.Totals.Total || rep.Methods[2].Amount != bl.Totals.Total || rep.Products[0].Quantity != 1 {
		t.Fatalf("daily sales %+v", rep)
	}
}
