package ordering_test

import (
	"context"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/ordering"
)

// BR-KT-01, BR-KT-06, TR-KT-01: a Prep Area queue shows its own items per Order ticket,
// Remakes first; Ready items move to the pickup strip.
func TestKitchenQueue(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	billA := f.directBill(t, s.w1, 0, f.nigiriLine(), f.latteLine(0, 0)) // kitchen + bar
	f.clock.Advance(time.Minute)
	f.directBill(t, s.w1, 1, f.nigiriLine())
	q, err := f.svc.Queue(ctx, "kitchen")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Tickets) != 2 || q.Tickets[0].Tables[0] != "Meja 3" || len(q.Tickets[0].Items) != 1 || q.Tickets[0].Items[0].Name != "Salmon Aburi" {
		t.Fatalf("kitchen tickets %+v", q.Tickets)
	}
	bar, _ := f.svc.Queue(ctx, "bar")
	if len(bar.Tickets) != 1 || bar.Tickets[0].Items[0].Modifiers[0] != "Susu segar" {
		t.Fatalf("bar tickets %+v", bar.Tickets)
	}

	// Start → Ready → Served, then a Remake jumps to the top.
	item := q.Tickets[0].Items[0].ID
	if err := f.svc.ReadyItems(ctx, s.w1, []string{item}); problemCode(err) != "invalid_transition" {
		t.Fatalf("ready before start: %v", err)
	}
	if err := f.svc.StartItems(ctx, s.w1, []string{item}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReadyItems(ctx, s.w1, []string{item}); err != nil {
		t.Fatal(err)
	}
	q, _ = f.svc.Queue(ctx, "kitchen")
	if len(q.Tickets) != 1 || len(q.Ready) != 1 || q.Ready[0].Items[0].ReadyAt == nil {
		t.Fatalf("after ready: tickets=%d ready=%d", len(q.Tickets), len(q.Ready))
	}
	if err := f.svc.Serve(ctx, s.w1, []string{item}); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionRemake)
	if _, err := f.svc.Remake(ctx, s.w1, item, "Kurang matang", tok); err != nil {
		t.Fatal(err)
	}
	q, _ = f.svc.Queue(ctx, "kitchen")
	if len(q.Tickets) != 2 || !q.Tickets[0].Remake || q.Tickets[0].BillID != billA || q.Tickets[0].Items[0].RemakeReason != "Kurang matang" {
		t.Fatalf("remake not first: %+v", q.Tickets)
	}
}

// BR-KT-04, G-07: the Kitchen Cancels from Preparing; Out of Stock still cascades.
func TestKitchenCancel(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	f.directBill(t, s.w1, 0, f.nigiriLine())
	q, _ := f.svc.Queue(ctx, "kitchen")
	item := q.Tickets[0].Items[0].ID
	_ = f.svc.StartItems(ctx, s.w1, []string{item})
	kitchenStatuses := []string{"confirmed", "preparing"}
	if err := f.svc.CancelItem(ctx, s.w1, item, ordering.ReasonOutOfStock, "", kitchenStatuses); err != nil {
		t.Fatal(err)
	}
	v, _ := f.db.Read().GetVariant(ctx, f.nigiri.Variants[0].ID)
	if v.SoldOut != 1 {
		t.Fatal("Kitchen Out of Stock did not mark the Variant Sold Out")
	}
	// Waiters cannot Cancel a Preparing item.
	f.directBill(t, s.w1, 1, f.latteLine(0, 0))
	bar, _ := f.svc.Queue(ctx, "bar")
	latte := bar.Tickets[0].Items[0].ID
	_ = f.svc.StartItems(ctx, s.w1, []string{latte})
	if err := f.svc.CancelItem(ctx, s.w1, latte, ordering.ReasonCustomerRequest, "", []string{"pending", "confirmed"}); problemCode(err) != "invalid_transition" {
		t.Fatalf("waiter cancelling a Preparing item: %v", err)
	}
}

// TR-KT-03, TR-KT-04, Phase 5 done-when: the Customer's Estimated Wait moves as the Kitchen
// works, and ends when everything is Ready.
func TestEstimatedWaitMovesWithKitchen(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	// Someone ahead in the Kitchen queue.
	f.directBill(t, s.w1, 1, f.nigiriLine())
	f.clock.Advance(time.Minute)
	orderID, g := f.pending(t, f.nigiriLine())
	_ = f.svc.Claim(ctx, s.w2, orderID)
	if err := f.svc.Confirm(ctx, s.w2, orderID); err != nil {
		t.Fatal(err)
	}
	wait := func() *ordering.Wait {
		me, _ := f.svc.Me(ctx, g)
		return me.Orders[0].EstimatedWait
	}
	// Ahead: 8 min ÷ capacity 1 + own 8 min = 16 min.
	if w := wait(); w == nil || w.Minutes != 16 {
		t.Fatalf("initial wait %+v", w)
	}
	q, _ := f.svc.Queue(ctx, "kitchen")
	ahead, mine := q.Tickets[0].Items[0].ID, q.Tickets[1].Items[0].ID
	_ = f.svc.StartItems(ctx, s.w1, []string{ahead})
	f.clock.Advance(4 * time.Minute)
	if w := wait(); w.Minutes != 12 {
		t.Fatalf("after 4 min of the item ahead: %d min", w.Minutes)
	}
	_ = f.svc.StartItems(ctx, s.w1, []string{ahead}) // no-op guard: already Preparing
	_ = f.svc.ReadyItems(ctx, s.w1, []string{ahead})
	if w := wait(); w.Minutes != 8 {
		t.Fatalf("after the item ahead is Ready: %d min", w.Minutes)
	}
	_ = f.svc.StartItems(ctx, s.w1, []string{mine})
	_ = f.svc.ReadyItems(ctx, s.w1, []string{mine})
	me, _ := f.svc.Me(ctx, g)
	if me.Orders[0].EstimatedWait != nil || me.Orders[0].Status != "ready" {
		t.Fatalf("after Ready: %+v %s", me.Orders[0].EstimatedWait, me.Orders[0].Status)
	}
}

// TR-KT-05: share of items Ready within their Prep Time.
func TestPrepTimeReport(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	ctx := context.Background()
	f.directBill(t, s.w1, 0, f.nigiriLine(), f.latteLine(0, 0))
	k, _ := f.svc.Queue(ctx, "kitchen")
	b, _ := f.svc.Queue(ctx, "bar")
	nigiri, latte := k.Tickets[0].Items[0].ID, b.Tickets[0].Items[0].ID
	_ = f.svc.StartItems(ctx, s.w1, []string{nigiri, latte})
	f.clock.Advance(5 * time.Minute)
	_ = f.svc.ReadyItems(ctx, s.w1, []string{nigiri, latte}) // nigiri 8 min target: on time; latte 4 min: late
	rep, err := f.svc.PrepTime(ctx, ordering.BusinessDate(f.clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items != 2 || rep.OnTime != 1 || rep.Products[0].Name != "Latte" || rep.Products[0].Share != 0 {
		t.Fatalf("prep report %+v", rep)
	}
}
