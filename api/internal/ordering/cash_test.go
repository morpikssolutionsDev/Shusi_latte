package ordering_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/hris"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
)

// payAll pays a Bill's balance in full by card and closes it, as staff would after serving.
func (f fixture) payAndClose(t *testing.T, by *auth.Principal, billID string) int64 {
	t.Helper()
	ctx := context.Background()
	f.served(t, billID, by)
	bl, _ := f.svc.Billing(ctx, billID)
	if _, err := f.svc.Pay(ctx, by, billID, ordering.PayInput{Method: ordering.PayCard, Amount: bl.Balance}, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Close(ctx, by, billID, "", false); err != nil {
		t.Fatal(err)
	}
	return bl.Totals.Total
}

// TR-PAY-01, BR-CS-04..06, Phase 6 done-when: expected cash = Opening Float + Cash Handovers +
// counter cash − change − cash Refunds; the Shift Closing records the difference.
func TestExpectedCashAndShiftClosing(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	c := f.cashier(t)
	ctx := context.Background()
	if err := f.svc.OpenDrawer(ctx, c, 500000); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.OpenDrawer(ctx, c, 1); problemCode(err) != "drawer_open" {
		t.Fatalf("second drawer session: %v", err)
	}

	// Counter cash: 48.000 nigiri → total 55.440 → rounds to 55.400; tendered 100.000, change 44.600.
	counter := f.directBill(t, c, 0, f.nigiriLine())
	res, err := f.svc.Pay(ctx, c, counter, ordering.PayInput{Method: ordering.PayCash, Amount: 100000}, "")
	if err != nil || res.Payment.Amount != 55400 || res.Payment.Change != 44600 || res.Payment.Rounding != -40 {
		t.Fatalf("counter cash %+v %v", res.Payment, err)
	}

	// A Waiter holds cash, then hands over Rp 5.000 short.
	held := f.directBill(t, s.w1, 1, f.nigiriLine(), f.nigiriLine())
	res, _ = f.svc.Pay(ctx, s.w1, held, ordering.PayInput{Method: ordering.PayCash, Amount: 120000}, "")
	if res.Payment.Status != ordering.PayHeldByWaiter {
		t.Fatalf("waiter cash = %s", res.Payment.Status)
	}
	d, _ := f.svc.Drawer(ctx)
	if len(d.Held) != 1 || d.Held[0].Amount != res.Payment.Amount {
		t.Fatalf("held cash %+v", d.Held)
	}
	h, err := f.svc.ReceiveHandover(ctx, c, s.w1.ID, res.Payment.Amount-5000)
	if err != nil || h.Difference != -5000 {
		t.Fatalf("handover %+v %v", h, err)
	}

	// A cash Refund of a card-paid Bill comes out of the drawer.
	f.served(t, counter, c)
	if err := f.svc.Close(ctx, c, counter, "", false); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := s.auth.IssueOverride(ctx, c, "A001", "222222", auth.ActionRefundInCap)
	if err := f.svc.Refund(ctx, c, counter, 20000, "Salah hitung", tok); err != nil {
		t.Fatal(err)
	}

	d, _ = f.svc.Drawer(ctx)
	want := int64(500000) + (res.Payment.Amount - 5000) + 100000 - 44600 - 20000
	if d.Expected.Total != want || len(d.Held) != 0 {
		t.Fatalf("expected %d, got %+v", want, d.Expected)
	}

	// Pending QRIS blocks the Shift Closing until written off (BR-CS-07, TR-PAY-03).
	q := f.directBill(t, s.w1, 0, f.nigiriLine())
	qt, _, _ := s.auth.IssueOverride(ctx, s.w1, "A001", "222222", auth.ActionQRISUnverified)
	pq, err := f.svc.Pay(ctx, s.w1, q, ordering.PayInput{Method: ordering.PayQRIS, Amount: 10000}, qt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CloseDrawer(ctx, c, want); problemCode(err) != "pending_qris" {
		t.Fatalf("closing with pending QRIS: %v", err)
	}
	if err := f.svc.WriteOffPayment(ctx, c, pq.Payment.ID, "Tidak masuk ke merchant"); err != nil {
		t.Fatal(err)
	}
	closing, err := f.svc.CloseDrawer(ctx, c, want-1000)
	if err != nil || *closing.Difference != -1000 || closing.Expected.Total != want {
		t.Fatalf("closing %+v %v", closing, err)
	}
}

// BR-R-20, G-17: a Refund above the cap needs Management; Refunds stop at what was paid.
func TestRefundCap(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	c := f.cashier(t)
	ctx := context.Background()
	_ = f.svc.OpenDrawer(ctx, c, 100000)
	billID := f.directBill(t, s.w1, 0, f.nigiriLine(), f.nigiriLine(), f.nigiriLine())
	total := f.payAndClose(t, s.w1, billID)
	// Management sets the cap to Rp 100.000 (BR-ADM-06); 120.000 is above it.
	if _, err := outlet.NewService(f.db, f.clock).UpdatePricing(ctx, f.actor, outlet.PricingSettings{ServiceRateBP: 500, TaxRateBP: 1000,
		PriceMode: "exclusive", RefundCap: 100000}); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := s.auth.IssueOverride(ctx, c, "A001", "222222", auth.ActionRefundInCap)
	if err := f.svc.Refund(ctx, c, billID, 120000, "Pesanan salah", tok); problemCode(err) != "override_invalid" {
		t.Fatalf("above-cap refund with Outlet Admin approval: %v", err)
	}
	mt, _, _ := s.auth.IssueOverride(ctx, c, "M001", "111111", auth.ActionRefundAboveCap)
	if err := f.svc.Refund(ctx, c, billID, 120000, "Pesanan salah", mt); err != nil {
		t.Fatal(err)
	}
	mt2, _, _ := s.auth.IssueOverride(ctx, c, "M001", "111111", auth.ActionRefundAboveCap)
	if err := f.svc.Refund(ctx, c, billID, total, "Lagi", mt2); problemCode(err) != "invalid_input" {
		t.Fatalf("refunding more than paid: %v", err)
	}
}

// BR-CS-02, TR-PAY-04, G-18: a Takeaway reaches the Kitchen only once paid in full.
func TestTakeawayPaidFirst(t *testing.T) {
	f := setup(t)
	c := f.cashier(t)
	ctx := context.Background()
	billID, err := f.svc.CreateTakeaway(ctx, c, ordering.TakeawayInput{CustomerName: "Sari", Items: []ordering.ItemInput{f.nigiriLine()}})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := f.svc.Queue(ctx, "kitchen")
	if len(q.Tickets) != 0 {
		t.Fatal("unpaid Takeaway reached the Kitchen")
	}
	// Neither confirming the held Order nor a direct Order on the Bill bypasses payment.
	bill, _ := f.svc.Bill(ctx, billID)
	if err := f.svc.Confirm(ctx, c, bill.Orders[0].ID); problemCode(err) != "takeaway_unpaid" {
		t.Fatalf("confirming an unpaid Takeaway: %v", err)
	}
	if _, err := f.svc.DirectOrder(ctx, c, "cashier", ordering.DirectInput{BillID: billID, Items: []ordering.ItemInput{f.nigiriLine()}}); problemCode(err) != "takeaway_unpaid" {
		t.Fatalf("direct Order on an unpaid Takeaway: %v", err)
	}
	bl, _ := f.svc.Billing(ctx, billID)
	if _, err := f.svc.Pay(ctx, c, billID, ordering.PayInput{Method: ordering.PayCard, Amount: bl.Balance - 1000}, ""); err != nil {
		t.Fatal(err)
	}
	if q, _ = f.svc.Queue(ctx, "kitchen"); len(q.Tickets) != 0 {
		t.Fatal("partly paid Takeaway reached the Kitchen")
	}
	if _, err := f.svc.Pay(ctx, c, billID, ordering.PayInput{Method: ordering.PayCard, Amount: 1000}, ""); err != nil {
		t.Fatal(err)
	}
	q, _ = f.svc.Queue(ctx, "kitchen")
	b, _ := f.db.Read().GetBill(ctx, billID)
	if len(q.Tickets) != 1 || q.Tickets[0].Tables[0] != "Bawa pulang · Sari" || b.Status != "closed" {
		t.Fatalf("paid Takeaway: tickets=%+v status=%s", q.Tickets, b.Status)
	}
	// Its Receipt names the Takeaway where a Table Bill names its Tables.
	r, err := f.svc.Receipt(ctx, c, billID, "", "customer")
	if err != nil || len(r.Tables) != 1 || r.Tables[0] != "Bawa pulang · Sari" {
		t.Fatalf("takeaway receipt: %v %v", r.Tables, err)
	}
}

// BR-CS-08, G-21: a paper Bill re-entered after an outage keeps its times, skips the Kitchen and
// is marked post_outage; its cash joins the drawer.
func TestPostOutageEntry(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	c := f.cashier(t)
	ctx := context.Background()
	_ = f.svc.OpenDrawer(ctx, c, 0)
	tables, _ := f.db.Read().ListTables(ctx)
	opened, closed := f.clock.Now().Add(-90*time.Minute), f.clock.Now().Add(-60*time.Minute)
	billID, err := f.svc.PostOutage(ctx, c, ordering.OutageInput{TableID: tables[0].ID, OpenedAt: platform.Millis(opened), ClosedAt: platform.Millis(closed),
		ServedBy: s.w1.ID, Items: []ordering.ItemInput{f.nigiriLine()}, Payments: []ordering.OutagePayment{{Method: "cash", Amount: 55400, TakenAt: platform.Millis(closed)}}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := f.db.Read().GetBill(ctx, billID)
	if b.Source != "post_outage" || b.Status != "closed" || b.ClosedAt.Int64 != platform.Millis(closed) || b.Total.Int64 != 55440 {
		t.Fatalf("post-outage bill %+v", b)
	}
	if q, _ := f.svc.Queue(ctx, "kitchen"); len(q.Tickets)+len(q.Ready) != 0 {
		t.Fatal("post-outage items reached the Kitchen")
	}
	floor, _ := f.svc.Floor(ctx, s.w1)
	if floor[0].Bill != nil {
		t.Fatal("post-outage Bill occupies a Table")
	}
	d, _ := f.svc.Drawer(ctx)
	if d.Expected.Total != 55400 {
		t.Fatalf("outage cash not in drawer: %+v", d.Expected)
	}
	if _, err := f.svc.PostOutage(ctx, c, ordering.OutageInput{TableID: tables[0].ID, OpenedAt: platform.Millis(opened), ClosedAt: platform.Millis(closed),
		ServedBy: s.w1.ID, Items: []ordering.ItemInput{f.nigiriLine()}, Payments: []ordering.OutagePayment{{Method: "card", Amount: 1000}}}); problemCode(err) != "invalid_input" {
		t.Fatalf("underpaid outage bill: %v", err)
	}
}

// BR-HR-07, BR-WT-11, TR-HR-03, G-06, G-20, Phase 7 done-when: a Waiter cannot clock out while holding
// cash; once the Cashier receives the handover, open Claims are listed for handover first.
func TestClockOutNeedsHandover(t *testing.T) {
	f := setup(t)
	s := f.staff(t)
	c := f.cashier(t)
	ctx := context.Background()
	hr := hris.NewService(f.db, f.clock, s.auth)
	if _, err := hr.Clock(ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"}); err != nil {
		t.Fatal(err)
	}
	billID := f.directBill(t, s.w1, 0, f.nigiriLine())
	res, err := f.svc.Pay(ctx, s.w1, billID, ordering.PayInput{Method: ordering.PayCash, Amount: 60000}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = hr.Clock(ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	if problemCode(err) != "cash_held" || !strings.Contains(asProblem(err).Detail, "Setor Rp 55.400 ke kasir dulu.") {
		t.Fatalf("clock-out holding cash: %v", err)
	}
	_ = f.svc.OpenDrawer(ctx, c, 0)
	if _, err := f.svc.ReceiveHandover(ctx, c, s.w1.ID, res.Payment.Amount); err != nil {
		t.Fatal(err)
	}
	_, err = hr.Clock(ctx, hris.ClockInput{CrewID: "W001", PIN: "333333"})
	if problemCode(err) != "open_claims" || len(asProblem(err).Extra["claims"].([]hris.OpenClaim)) != 1 {
		t.Fatalf("clock-out with an open Claim: %v", err)
	}
	out, err := hr.Clock(ctx, hris.ClockInput{CrewID: "W001", PIN: "333333", AcknowledgeClaims: true})
	if err != nil || out.Action != "out" {
		t.Fatalf("clock-out after handover: %+v %v", out, err)
	}
}

func asProblem(err error) *httpx.Problem {
	var p *httpx.Problem
	if errors.As(err, &p) {
		return p
	}
	return &httpx.Problem{}
}
