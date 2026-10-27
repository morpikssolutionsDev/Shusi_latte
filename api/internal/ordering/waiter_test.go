package ordering_test

import (
	"context"
	"database/sql"
	"slices"
	"sync"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

type staff struct {
	w1, w2, admin *auth.Principal
	auth          *auth.Service
}

func (f fixture) staff(t *testing.T) staff {
	t.Helper()
	ctx := context.Background()
	mk := func(tx *dbx.Tx, crewID, role string, stations []string, pin string) *auth.Principal {
		id, err := auth.CreateCrew(ctx, tx, t0, auth.NewCrew{CrewID: crewID, FullName: crewID + " Crew", Role: role, Stations: stations, PIN: pin})
		if err != nil {
			t.Fatal(err)
		}
		return &auth.Principal{ID: id, CrewID: crewID, Role: role, Stations: stations, Kind: "crew"}
	}
	var s staff
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		s.w1 = mk(tx, "W001", auth.RoleCrew, []string{auth.StationWaiter}, "333333")
		s.w2 = mk(tx, "W002", auth.RoleCrew, []string{auth.StationWaiter}, "333333")
		s.admin = mk(tx, "A001", auth.RoleOutletAdmin, []string{}, "222222")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.auth = auth.NewService(f.db, f.clock)
	return s
}

func (f fixture) pending(t *testing.T, lines ...ordering.ItemInput) (string, *ordering.Guest) {
	t.Helper()
	_, g := f.join(t, "Tasya")
	id, err := f.svc.PlaceOrder(context.Background(), g, lines)
	if err != nil {
		t.Fatal(err)
	}
	return id, g
}

func (f fixture) nigiriLine() ordering.ItemInput {
	return ordering.ItemInput{VariantID: f.nigiri.Variants[0].ID, Quantity: 1}
}

func (f fixture) items(t *testing.T, orderID string) []store.OrderItem {
	t.Helper()
	items, err := f.db.Read().ListOrderItemsByOrder(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// TR-SM-03, Phase 4 done-when: a Claim race has exactly one winner.
func TestClaimRaceHasOneWinner(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	orderID, _ := f.pending(t, f.nigiriLine())
	waiters := []*auth.Principal{s.w1, s.w2, s.w1, s.w2, s.w1, s.w2}
	var wg sync.WaitGroup
	errs := make([]error, len(waiters))
	for i, w := range waiters {
		wg.Add(1)
		go func(i int, w *auth.Principal) {
			defer wg.Done()
			errs[i] = f.svc.Claim(context.Background(), w, orderID)
		}(i, w)
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case problemCode(err) != "already_claimed":
			t.Fatalf("unexpected error %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d winners, want exactly 1", wins)
	}
}

// BR-WT-02, BR-R-06: only the claiming Waiter confirms; items move to Confirmed.
func TestClaimAndConfirm(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	orderID, g := f.pending(t, f.nigiriLine(), f.latteLine(0, 0))
	if err := f.svc.Confirm(ctx, s.w1, orderID); problemCode(err) != "not_claimed" {
		t.Fatalf("confirm before claim: %v", err)
	}
	if err := f.svc.Claim(ctx, s.w1, orderID); err != nil {
		t.Fatal(err)
	}
	// The Customer can no longer change it (BR-CM-14).
	if err := f.svc.CancelOrder(ctx, g, orderID); problemCode(err) != "order_claimed" {
		t.Fatalf("customer cancel after claim: %v", err)
	}
	if err := f.svc.Confirm(ctx, s.w2, orderID); problemCode(err) != "claimed_by_other" {
		t.Fatalf("other waiter confirm: %v", err)
	}
	// The Waiter adds an item while reviewing, then confirms.
	if err := f.svc.AddItems(ctx, s.w1, orderID, []ordering.ItemInput{f.nigiriLine()}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Confirm(ctx, s.w1, orderID); err != nil {
		t.Fatal(err)
	}
	for _, it := range f.items(t, orderID) {
		if it.Status != "confirmed" {
			t.Fatalf("item %s is %s", it.ProductNameEn, it.Status)
		}
	}
	me, _ := f.svc.Me(ctx, g)
	if me.Orders[0].Status != "confirmed" || me.Orders[0].EstimatedWait == nil {
		t.Fatalf("customer sees %s, wait %v", me.Orders[0].Status, me.Orders[0].EstimatedWait)
	}
}

// BR-R-11..13, TR-SM-04, Phase 4 done-when: an Out of Stock Cancel marks the Variant Sold Out
// and flags its other open items; nothing else is cancelled; the flagged Order can't be
// confirmed until the item is Cancelled.
func TestOutOfStockCascade(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	first, _ := f.pending(t, f.nigiriLine())
	second, _ := f.pending(t, f.nigiriLine(), f.latteLine(0, 0))
	_ = f.svc.Claim(ctx, s.w1, first)
	_ = f.svc.Claim(ctx, s.w2, second)

	firstItem := f.items(t, first)[0]
	if err := f.svc.CancelItem(ctx, s.w1, firstItem.ID, ordering.ReasonOutOfStock, "", []string{"pending", "confirmed"}); err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Load(ctx, f.db.Read(), false)
	for _, p := range cat.Products {
		if p.ID == f.nigiri.ID && (!p.Variants[0].SoldOut || p.Variants[0].SoldOutSource != catalog.SoldOutOutOfStock) {
			t.Fatalf("variant not Sold Out from the Cancel: %+v", p.Variants[0])
		}
	}
	items := f.items(t, second)
	if items[0].SoldOutFlag != 1 || items[0].Status != "pending" || items[1].SoldOutFlag != 0 {
		t.Fatalf("flags: %+v / %+v", items[0], items[1])
	}
	if err := f.svc.Confirm(ctx, s.w2, second); problemCode(err) != "flagged_items" {
		t.Fatalf("confirm with flagged item: %v", err)
	}
	if err := f.svc.CancelItem(ctx, s.w2, items[0].ID, ordering.ReasonCustomerRequest, "", []string{"pending", "confirmed"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Confirm(ctx, s.w2, second); err != nil {
		t.Fatal(err)
	}
	// The first Order had only that item: it is now Cancelled as a whole.
	o, _ := f.db.Read().GetOrder(ctx, first)
	if o.Status != "cancelled" {
		t.Fatalf("emptied order = %s", o.Status)
	}
	// Other reason needs a note.
	if err := f.svc.CancelItem(ctx, s.w2, items[1].ID, ordering.ReasonOther, " ", []string{"pending", "confirmed"}); problemCode(err) != "invalid_input" {
		t.Fatalf("other without note: %v", err)
	}
}

// BR-R-35, TR-SM-06, G-23: hiding a Product flags its open items.
func TestHiddenProductFlagsItems(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID, _ := f.pending(t, f.latteLine(0, 0))
	hidden := f.latte
	hidden.Status = catalog.StatusHidden
	if _, err := f.cat.SaveProduct(ctx, f.actor, hidden); err != nil {
		t.Fatal(err)
	}
	if it := f.items(t, orderID)[0]; it.UnavailableFlag != 1 || it.Status != "pending" {
		t.Fatalf("hidden product item: flag=%d status=%s", it.UnavailableFlag, it.Status)
	}
}

// BR-WT-11, BR-R-34, TR-SM-05: takeover by a Waiter, reassignment by the Outlet Admin, both audited.
func TestTakeoverAndReassign(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	orderID, _ := f.pending(t, f.nigiriLine())
	_ = f.svc.Claim(ctx, s.w1, orderID)
	if err := f.svc.TransferClaim(ctx, s.w2, orderID, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TransferClaim(ctx, s.w2, orderID, s.w1.ID); problemCode(err) != "forbidden" {
		t.Fatalf("waiter reassigning: %v", err)
	}
	if err := f.svc.TransferClaim(ctx, s.admin, orderID, s.w1.ID); err != nil {
		t.Fatal(err)
	}
	o, _ := f.db.Read().GetOrder(ctx, orderID)
	transfers, _ := f.db.Read().ListClaimTransfers(ctx, orderID)
	if o.ClaimedBy.String != s.w1.ID || len(transfers) != 2 || transfers[0].Kind != "takeover" || transfers[1].Kind != "reassign" {
		t.Fatalf("claimant %s, transfers %+v", o.ClaimedBy.String, transfers)
	}
	audits, _ := f.db.Read().SearchAudit(ctx, store.SearchAuditParams{ActionPrefix: "claim.", RowLimit: 10})
	if len(audits) != 2 {
		t.Fatalf("audit entries = %d", len(audits))
	}
}

// BR-WT-03, OP-04: direct entry opens the Bill and creates a Confirmed Order owned by the Waiter;
// a ticked restriction raises warnings.
func TestDirectEntry(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	tables, _ := f.db.Read().ListTables(ctx)
	id, err := f.svc.DirectOrder(ctx, s.w1, "waiter", ordering.DirectInput{TableID: tables[1].ID, CustomerName: "Bima",
		RestrictionTagIDs: []string{f.dairy}, Items: []ordering.ItemInput{f.latteLine(0, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	o, _ := f.db.Read().GetOrder(ctx, id)
	if o.Status != "confirmed" || o.ClaimedBy.String != s.w1.ID || o.Source != "waiter" {
		t.Fatalf("direct order = %+v", o)
	}
	bill, err := f.svc.Bill(ctx, o.BillID)
	if err != nil {
		t.Fatal(err)
	}
	it := bill.Orders[0].Items[0]
	if it.Status != "confirmed" || len(it.Warnings) != 1 || bill.Orders[0].CustomerName != "Bima" {
		t.Fatalf("direct item %+v name %q", it, bill.Orders[0].CustomerName)
	}
	if bill.Totals.Total == 0 || bill.Totals.Tax == 0 || bill.Totals.Service == 0 {
		t.Fatalf("totals %+v", bill.Totals)
	}
}

// BR-WT-09, BR-R-33, BR-R-08: Served needs Ready; Remake and Void need an approver's PIN.
func TestServeRemakeVoid(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	tables, _ := f.db.Read().ListTables(ctx)
	id, _ := f.svc.DirectOrder(ctx, s.w1, "waiter", ordering.DirectInput{TableID: tables[0].ID, Items: []ordering.ItemInput{f.nigiriLine(), f.nigiriLine()}})
	items := f.items(t, id)
	if err := f.svc.Serve(ctx, s.w1, []string{items[0].ID}); problemCode(err) != "invalid_transition" {
		t.Fatalf("serve before ready: %v", err)
	}
	now := platform.Millis(t0)
	if err := f.db.Write(ctx, func(tx *dbx.Tx) error {
		for _, it := range items {
			if _, err := tx.StartItem(ctx, store.StartItemParams{PreparingAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now, ID: it.ID}); err != nil {
				return err
			}
			if _, err := tx.ReadyItem(ctx, store.ReadyItemParams{ReadyAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now, ID: it.ID}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Serve(ctx, s.w1, []string{items[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Remake(ctx, s.w1, items[0].ID, "Kurang pedas", ""); problemCode(err) != "override_required" {
		t.Fatalf("remake without PIN: %v", err)
	}
	tok, _, err := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionRemake)
	if err != nil {
		t.Fatal(err)
	}
	remakeID, err := f.svc.Remake(ctx, s.w1, items[0].ID, "Kurang pedas", tok)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := f.db.Read().GetOrderItem(ctx, remakeID)
	if r.UnitPrice != 0 || r.Status != "confirmed" || r.RemakeOfID.String != items[0].ID || r.RemakeOverrideBy.String != s.admin.ID {
		t.Fatalf("remake = %+v", r)
	}
	// A waiter's own PIN cannot approve a Void.
	if _, _, err := s.auth.IssueOverride(ctx, s.w1, "W002", "333333", auth.ActionVoid); problemCode(err) != "approver_not_allowed" {
		t.Fatalf("waiter approving void: %v", err)
	}
	vt, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionVoid)
	if err := f.svc.Void(ctx, s.w1, items[1].ID, "Jatuh saat disajikan", vt); err != nil {
		t.Fatal(err)
	}
	bill, _ := f.svc.Bill(ctx, f.items(t, id)[0].OrderID)
	_ = bill
	o, _ := f.db.Read().GetOrder(ctx, id)
	b, _ := f.svc.Bill(ctx, o.BillID)
	// Two nigiri at 48.000: one served (charged), one voided (excluded), remake free.
	if b.Totals.Subtotal != 48000 {
		t.Fatalf("subtotal after void and remake = %d", b.Totals.Subtotal)
	}
}

// BR-WT-10, BR-WT-09, G-05, G-09: re-notify at 2 min, escalate at 5 min, pickup reminder at 3 min.
func TestEscalationAndPickupReminder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	orderID, g := f.pending(t, f.nigiriLine())
	if _, err := f.svc.CallWaiter(ctx, g); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(121 * time.Second)
	if err := f.svc.Escalate(ctx); err != nil {
		t.Fatal(err)
	}
	o, _ := f.db.Read().GetOrder(ctx, orderID)
	if !o.RenotifiedAt.Valid || o.EscalatedAt.Valid {
		t.Fatalf("at 2 min: renotified=%v escalated=%v", o.RenotifiedAt.Valid, o.EscalatedAt.Valid)
	}
	f.clock.Advance(180 * time.Second)
	if err := f.svc.Escalate(ctx); err != nil {
		t.Fatal(err)
	}
	o, _ = f.db.Read().GetOrder(ctx, orderID)
	calls, _ := f.db.Read().ListOpenWaiterCalls(ctx)
	if !o.EscalatedAt.Valid || len(calls) != 1 || !calls[0].EscalatedAt.Valid {
		t.Fatalf("at 5 min: order escalated=%v call=%+v", o.EscalatedAt.Valid, calls)
	}
	s := f.staff(t)
	if err := f.svc.AnswerCall(ctx, s.w1, calls[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AnswerCall(ctx, s.w2, calls[0].ID); problemCode(err) != "already_answered" {
		t.Fatalf("second answer: %v", err)
	}

	// Pickup reminder for a Ready item.
	tables, _ := f.db.Read().ListTables(ctx)
	id, _ := f.svc.DirectOrder(ctx, s.w1, "waiter", ordering.DirectInput{TableID: tables[1].ID, Items: []ordering.ItemInput{f.latteLine(0, 0)}})
	it := f.items(t, id)[0]
	now := platform.Millis(f.clock.Now())
	_ = f.db.Write(ctx, func(tx *dbx.Tx) error {
		_, _ = tx.StartItem(ctx, store.StartItemParams{PreparingAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now, ID: it.ID})
		_, err := tx.ReadyItem(ctx, store.ReadyItemParams{ReadyAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now, ID: it.ID})
		return err
	})
	f.clock.Advance(181 * time.Second)
	_ = f.svc.Escalate(ctx)
	if it, _ := f.db.Read().GetOrderItem(ctx, it.ID); !it.PickupRemindedAt.Valid {
		t.Fatal("no pickup reminder after 3 min")
	}
}

// The Waiter feed lists unclaimed Orders for everyone and owned Orders for their owner.
func TestFeedAndFloor(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	orderID, _ := f.pending(t, f.latteLine(0, 0))
	feed, err := f.svc.Feed(ctx, s.w1)
	if err != nil || len(feed.Unclaimed) != 1 || feed.Unclaimed[0].Tables[0].Label != "Meja 3" {
		t.Fatalf("feed %+v %v", feed, err)
	}
	_ = f.svc.Claim(ctx, s.w1, orderID)
	feed, _ = f.svc.Feed(ctx, s.w1)
	other, _ := f.svc.Feed(ctx, s.w2)
	if len(feed.Mine) != 1 || len(feed.Unclaimed) != 0 || len(other.Mine) != 0 || feed.Mine[0].ClaimedBy.CrewID != "W001" {
		t.Fatalf("mine=%d unclaimed=%d other=%d", len(feed.Mine), len(feed.Unclaimed), len(other.Mine))
	}
	floor, _ := f.svc.Floor(ctx, s.w1)
	idx := slices.IndexFunc(floor, func(c ordering.TableCard) bool { return c.Number == 3 })
	if floor[idx].Bill == nil || !floor[idx].Bill.ClaimedByMe || floor[idx].Bill.Customers[0] != "Tasya" {
		t.Fatalf("floor card %+v", floor[idx])
	}
}
