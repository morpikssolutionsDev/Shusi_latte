package ordering

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// Cashier (OP-11..OP-14): the Cash Drawer, Cash Handovers, Refunds, Takeaway and Post-outage
// Entries. CashTopic carries drawer changes to Cashier screens.
const CashTopic = "cash"

func cashEvent(tx *dbx.Tx, kind, id string) {
	tx.Publish(realtime.Event{Topic: CashTopic, Type: kind, ID: id})
}

type HeldCash struct {
	CrewID   string `json:"crewId"` // crew.id
	Code     string `json:"code"`
	Name     string `json:"name"`
	Amount   int64  `json:"amount"`
	Payments int    `json:"payments"`
}

type HandoverView struct {
	ID         string `json:"id"`
	Waiter     string `json:"waiter"`
	Expected   int64  `json:"expected"`
	Received   int64  `json:"received"`
	Difference int64  `json:"difference"`
	ReceivedAt int64  `json:"receivedAt"`
}

type RefundView struct {
	ID         string `json:"id"`
	BillID     string `json:"billId"`
	BillNumber string `json:"billNumber"`
	Amount     int64  `json:"amount"`
	Reason     string `json:"reason"`
	ApprovedBy string `json:"approvedBy"`
	CreatedAt  int64  `json:"createdAt"`
}

type PendingQRIS struct {
	PaymentView
	BillID      string `json:"billId"`
	BillNumber  string `json:"billNumber"`
	TakenByName string `json:"takenByName"`
}

// Expected is TR-PAY-01, itemised for the Shift Closing screen.
type Expected struct {
	OpeningFloat    int64 `json:"openingFloat"`
	Handovers       int64 `json:"handovers"`
	CounterTendered int64 `json:"counterTendered"`
	Change          int64 `json:"change"`
	Refunds         int64 `json:"refunds"`
	Total           int64 `json:"total"`
}

type DrawerView struct {
	ID           string `json:"id"`
	Cashier      string `json:"cashier"`
	OpeningFloat int64  `json:"openingFloat"`
	OpenedAt     int64  `json:"openedAt"`
	ClosedAt     *int64 `json:"closedAt,omitempty"`
	Counted      *int64 `json:"counted,omitempty"`
	Difference   *int64 `json:"difference,omitempty"`
}

// Drawer is the Cashier's Cash Drawer panel.
type Drawer struct {
	Session   *DrawerView    `json:"session,omitempty"`
	Expected  Expected       `json:"expected"`
	Handovers []HandoverView `json:"handovers"`
	Refunds   []RefundView   `json:"refunds"`
	Held      []HeldCash     `json:"held"`    // Waiters still holding cash (BR-WT-14)
	Pending   []PendingQRIS  `json:"pending"` // QRIS to verify or write off (BR-CS-07)
}

func drawerClosed() error {
	return httpx.NewProblem(http.StatusConflict, "drawer_closed", "Buka laci kasir dengan Opening Float dulu")
}

func (s *Service) Drawer(ctx context.Context) (Drawer, error) {
	sc, err := s.staffContext(ctx)
	if err != nil {
		return Drawer{}, err
	}
	q := sc.q
	out := Drawer{Handovers: []HandoverView{}, Refunds: []RefundView{}, Held: []HeldCash{}, Pending: []PendingQRIS{}}
	held, err := q.ListAllHeldPayments(ctx)
	if err != nil {
		return Drawer{}, err
	}
	for _, p := range held {
		i := slices.IndexFunc(out.Held, func(h HeldCash) bool { return h.CrewID == p.TakenBy })
		if i < 0 {
			c := sc.crew[p.TakenBy]
			out.Held = append(out.Held, HeldCash{CrewID: p.TakenBy, Code: c.CrewID, Name: c.Name})
			i = len(out.Held) - 1
		}
		out.Held[i].Amount += p.Amount
		out.Held[i].Payments++
	}
	pending, err := q.ListPendingPayments(ctx)
	if err != nil {
		return Drawer{}, err
	}
	for _, p := range pending {
		b, err := q.GetBill(ctx, p.BillID)
		if err != nil {
			return Drawer{}, err
		}
		out.Pending = append(out.Pending, PendingQRIS{PaymentView: paymentView(p), BillID: p.BillID, BillNumber: BillNumber(b.Number),
			TakenByName: sc.crew[p.TakenBy].Name})
	}
	sess, err := q.GetOpenDrawerSession(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Drawer{}, err
	}
	out.Session = &DrawerView{ID: sess.ID, Cashier: sc.crew[sess.CashierCrewID].Name, OpeningFloat: sess.OpeningFloat, OpenedAt: sess.OpenedAt}
	if out.Expected, err = expectedCash(ctx, q, sess); err != nil {
		return Drawer{}, err
	}
	hs, err := q.ListSessionHandovers(ctx, sess.ID)
	if err != nil {
		return Drawer{}, err
	}
	for _, h := range hs {
		out.Handovers = append(out.Handovers, HandoverView{ID: h.ID, Waiter: sc.crew[h.WaiterCrewID].Name, Expected: h.ExpectedAmount,
			Received: h.ReceivedAmount, Difference: h.Difference, ReceivedAt: h.ReceivedAt})
	}
	rs, err := q.ListSessionRefunds(ctx, sess.ID)
	if err != nil {
		return Drawer{}, err
	}
	for _, r := range rs {
		b, _ := q.GetBill(ctx, r.BillID)
		out.Refunds = append(out.Refunds, RefundView{ID: r.ID, BillID: r.BillID, BillNumber: BillNumber(b.Number), Amount: r.Amount, Reason: r.Reason,
			ApprovedBy: sc.crew[r.ApprovedBy].Name, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

func paymentView(p store.Payment) PaymentView {
	v := PaymentView{ID: p.ID, SubBillID: p.SubBillID.String, Method: p.Method, Status: p.Status, Amount: p.Amount, Change: p.ChangeGiven,
		Rounding: p.Rounding, TakenBy: p.TakenBy, TakenAt: p.TakenAt}
	if p.Tendered.Valid {
		v.Tendered = &p.Tendered.Int64
	}
	if p.VerifiedAt.Valid {
		v.VerifiedAt = &p.VerifiedAt.Int64
	}
	return v
}

// expectedCash is TR-PAY-01: Opening Float + received Cash Handovers + counter cash tendered −
// change − cash Refunds. Rounding is already inside the Payment amounts.
func expectedCash(ctx context.Context, q *store.Queries, sess store.DrawerSession) (Expected, error) {
	e := Expected{OpeningFloat: sess.OpeningFloat}
	cash, err := q.ListSessionCashPayments(ctx, nullStr(sess.ID))
	if err != nil {
		return e, err
	}
	for _, p := range cash {
		e.CounterTendered += p.Tendered.Int64
		e.Change += p.ChangeGiven
	}
	hs, err := q.ListSessionHandovers(ctx, sess.ID)
	if err != nil {
		return e, err
	}
	for _, h := range hs {
		e.Handovers += h.ReceivedAmount
	}
	rs, err := q.ListSessionRefunds(ctx, sess.ID)
	if err != nil {
		return e, err
	}
	for _, r := range rs {
		e.Refunds += r.Amount
	}
	e.Total = e.OpeningFloat + e.Handovers + e.CounterTendered - e.Change - e.Refunds
	return e, nil
}

// OpenDrawer starts a drawer session with the Cashier's Opening Float (BR-CS-04, G-19).
func (s *Service) OpenDrawer(ctx context.Context, p *auth.Principal, openingFloat int64) error {
	if openingFloat < 0 {
		return invalid("Opening Float tidak boleh negatif.")
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := tx.GetOpenDrawerSession(ctx); err == nil {
			return conflict("drawer_open", "Laci kasir sudah dibuka — tutup shift dulu")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		id := platform.NewID()
		if err := tx.OpenDrawerSession(ctx, store.OpenDrawerSessionParams{ID: id, CashierCrewID: p.ID, OpeningFloat: openingFloat, OpenedAt: platform.Millis(now)}); err != nil {
			return err
		}
		cashEvent(tx, "drawer.opened", id)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "drawer.opened", Entity: "drawer_session", EntityID: id,
			After: map[string]int64{"openingFloat": openingFloat}})
	})
}

// ReceiveHandover settles everything a Waiter holds into the drawer; a difference is recorded
// against that Waiter and audit-logged (BR-CS-05, BR-R-36, TR-PAY-02, G-20).
func (s *Service) ReceiveHandover(ctx context.Context, p *auth.Principal, waiterID string, received int64) (HandoverView, error) {
	if received < 0 {
		return HandoverView{}, invalid("Jumlah diterima tidak valid.")
	}
	now := s.clock.Now()
	var out HandoverView
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		sess, err := tx.GetOpenDrawerSession(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return drawerClosed()
		}
		if err != nil {
			return err
		}
		held, err := tx.ListHeldPayments(ctx, waiterID)
		if err != nil {
			return err
		}
		if len(held) == 0 {
			return conflict("nothing_held", "Pelayan ini tidak memegang uang tunai")
		}
		var expected int64
		for _, h := range held {
			expected += h.Amount
		}
		id := platform.NewID()
		out = HandoverView{ID: id, Expected: expected, Received: received, Difference: received - expected, ReceivedAt: platform.Millis(now)}
		if err := tx.CreateCashHandover(ctx, store.CreateCashHandoverParams{ID: id, DrawerSessionID: sess.ID, WaiterCrewID: waiterID, ExpectedAmount: expected,
			ReceivedAmount: received, Difference: out.Difference, ReceivedBy: p.ID, ReceivedAt: out.ReceivedAt}); err != nil {
			return err
		}
		if err := tx.SettleHeldPayments(ctx, store.SettleHeldPaymentsParams{CashHandoverID: nullStr(id), DrawerSessionID: nullStr(sess.ID), TakenBy: waiterID}); err != nil {
			return err
		}
		if c, err := tx.GetCrew(ctx, waiterID); err == nil {
			out.Waiter = c.FullName
		}
		cashEvent(tx, "handover.received", id)
		if out.Difference != 0 {
			return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "cash_handover.difference", Entity: "cash_handover", EntityID: id,
				After: map[string]any{"waiter": waiterID, "expected": expected, "received": received, "difference": out.Difference}})
		}
		return nil
	})
	return out, err
}

// VerifyPayment confirms a pending QRIS once it shows on the merchant app (BR-R-30).
func (s *Service) VerifyPayment(ctx context.Context, p *auth.Principal, paymentID string) error {
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		n, err := tx.VerifyPayment(ctx, store.VerifyPaymentParams{VerifiedBy: nullStr(p.ID), VerifiedAt: nullMs(platform.Millis(now)), ID: paymentID})
		if err != nil {
			return err
		}
		if n == 0 {
			return conflict("not_pending", "Pembayaran ini tidak menunggu verifikasi")
		}
		cashEvent(tx, "qris.verified", paymentID)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "payment.qris_verified", Entity: "payment", EntityID: paymentID})
	})
}

// WriteOffPayment resolves a QRIS that never arrived, with a reason, so the shift can close
// (BR-CS-07, TR-PAY-03).
func (s *Service) WriteOffPayment(ctx context.Context, p *auth.Principal, paymentID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return invalid("Tulis alasan penghapusan.")
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		pay, err := tx.GetPayment(ctx, paymentID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("Pembayaran tidak ditemukan")
		}
		if err != nil {
			return err
		}
		n, err := tx.WriteOffPayment(ctx, store.WriteOffPaymentParams{WrittenOffReason: nullStr(reason), WrittenOffBy: nullStr(p.ID), ID: paymentID})
		if err != nil {
			return err
		}
		if n == 0 {
			return conflict("not_pending", "Pembayaran ini tidak menunggu verifikasi")
		}
		cashEvent(tx, "qris.written_off", paymentID)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "payment.qris_written_off", Entity: "payment", EntityID: paymentID,
			Before: map[string]int64{"amount": pay.Amount}, Reason: reason})
	})
}

type ShiftClosing struct {
	DrawerView
	Expected Expected `json:"expected"`
}

// CloseDrawer is the Shift Closing: refused while a QRIS is pending; records counted cash and
// the difference against expected (BR-CS-06, BR-CS-07, TR-PAY-01, TR-PAY-03).
func (s *Service) CloseDrawer(ctx context.Context, p *auth.Principal, counted int64) (ShiftClosing, error) {
	if counted < 0 {
		return ShiftClosing{}, invalid("Jumlah uang tidak valid.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	var out ShiftClosing
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		sess, err := tx.GetOpenDrawerSession(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return drawerClosed()
		}
		if err != nil {
			return err
		}
		if pending, err := tx.ListPendingPayments(ctx); err != nil {
			return err
		} else if len(pending) > 0 {
			return conflict("pending_qris", "Selesaikan QRIS yang menunggu verifikasi dulu").With("count", len(pending))
		}
		e, err := expectedCash(ctx, tx.Queries, sess)
		if err != nil {
			return err
		}
		diff := counted - e.Total
		if err := tx.CloseDrawerSession(ctx, store.CloseDrawerSessionParams{ExpectedCash: nullMs(e.Total), CountedCash: nullMs(counted),
			Difference: sql.NullInt64{Int64: diff, Valid: true}, ClosedAt: nullMs(nowMs), ClosedBy: nullStr(p.ID), ID: sess.ID}); err != nil {
			return err
		}
		out = ShiftClosing{DrawerView: DrawerView{ID: sess.ID, OpeningFloat: sess.OpeningFloat, OpenedAt: sess.OpenedAt, ClosedAt: &nowMs, Counted: &counted, Difference: &diff},
			Expected: e}
		cashEvent(tx, "drawer.closed", sess.ID)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "shift_closing", Entity: "drawer_session", EntityID: sess.ID,
			After: map[string]int64{"expected": e.Total, "counted": counted, "difference": diff}})
	})
	return out, err
}

// Refund pays money back on a closed Bill, always in cash from the drawer. Up to the cap an
// Outlet Admin or Management approves; above it only Management (BR-R-20, BR-CS-03, G-17).
func (s *Service) Refund(ctx context.Context, p *auth.Principal, billID string, amount int64, reason, token string) error {
	reason = strings.TrimSpace(reason)
	if amount <= 0 || reason == "" {
		return invalid("Isi jumlah dan alasan refund.")
	}
	settings, err := s.outlet.Settings(ctx)
	if err != nil {
		return err
	}
	action := auth.ActionRefundInCap
	if amount > settings.Pricing.RefundCap {
		action = auth.ActionRefundAboveCap
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		bill, err := tx.GetBill(ctx, billID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("Tagihan tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if bill.Status != BillClosed {
			return conflict("bill_not_closed", "Refund hanya untuk tagihan yang sudah lunas — gunakan Void untuk tagihan terbuka")
		}
		bl, err := s.billing(ctx, tx.Queries, bill)
		if err != nil {
			return err
		}
		prior, err := tx.ListRefundsByBill(ctx, billID)
		if err != nil {
			return err
		}
		refundable := bl.Paid
		for _, r := range prior {
			refundable -= r.Amount
		}
		if amount > refundable {
			return invalid("Refund melebihi yang sudah dibayar.")
		}
		sess, err := tx.GetOpenDrawerSession(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return drawerClosed()
		}
		if err != nil {
			return err
		}
		id := platform.NewID()
		approver, err := auth.ConsumeOverride(ctx, tx, now, p, token, action, audit.Entry{Action: "refund.paid", Entity: "bill", EntityID: billID,
			After: map[string]any{"refund": id, "amount": amount}, Reason: reason})
		if err != nil {
			return err
		}
		if err := tx.CreateRefund(ctx, store.CreateRefundParams{ID: id, BillID: billID, Amount: amount, Reason: reason, RefundedBy: p.ID,
			ApprovedBy: approver, DrawerSessionID: sess.ID, CreatedAt: platform.Millis(now)}); err != nil {
			return err
		}
		cashEvent(tx, "refund.paid", id)
		billEvents(tx, billID, "bill.refunded")
		return nil
	})
}

// ---------------------------------------------------------------- Takeaway (BR-CS-02, TR-PAY-04, G-18)

type TakeawayInput struct {
	CustomerName      string      `json:"customerName"`
	RestrictionTagIDs []string    `json:"restrictionTagIds"`
	Items             []ItemInput `json:"items"`
}

// CreateTakeaway opens a Takeaway Bill. Its Order is held, Claimed by the Cashier, and goes to
// the Kitchen only once the Bill is paid in full.
func (s *Service) CreateTakeaway(ctx context.Context, p *auth.Principal, in TakeawayInput) (string, error) {
	in.CustomerName = strings.Join(strings.Fields(in.CustomerName), " ")
	if in.CustomerName == "" {
		return "", invalid("Tulis nama tamu untuk dipanggil.")
	}
	if len(in.Items) == 0 {
		return "", invalid("Pilih minimal satu item.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	keyID, nameEnc, err := s.vault.Seal(now, []byte(in.CustomerName))
	if err != nil {
		return "", err
	}
	var tagsEnc []byte
	if len(in.RestrictionTagIDs) > 0 {
		raw, _ := json.Marshal(in.RestrictionTagIDs)
		if tagsEnc, err = s.vault.SealWith(keyID, raw); err != nil {
			return "", err
		}
	}
	billID := platform.NewID()
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		cat, err := catalog.Load(ctx, tx.Queries, false)
		if err != nil {
			return err
		}
		lines, problems := []resolved{}, []ItemProblem{}
		for i, item := range in.Items {
			r, prob := resolve(cat, i, item, in.RestrictionTagIDs, false)
			if prob != nil {
				problems = append(problems, *prob)
				continue
			}
			lines = append(lines, r)
		}
		if len(problems) > 0 {
			return rejected(problems)
		}
		date := BusinessDate(now)
		n, err := tx.NextBillNumber(ctx, date)
		if err != nil {
			return err
		}
		if err := tx.CreateBill(ctx, store.CreateBillParams{ID: billID, BusinessDate: date, Number: n, Type: "takeaway", Source: "live",
			KeyID: nullStr(keyID), TakeawayNameEnc: nameEnc, OpenedAt: nowMs, UpdatedAt: nowMs}); err != nil {
			return err
		}
		orderID := platform.NewID()
		if err := tx.CreateOrder(ctx, store.CreateOrderParams{ID: orderID, BillID: billID, Seq: 1, Source: "cashier", Status: OrderPending,
			PlacedBy: nullStr(p.ID), KeyID: nullStr(keyID), RestrictionTagsEnc: tagsEnc, CreatedAt: nowMs, UpdatedAt: nowMs}); err != nil {
			return err
		}
		if _, err := tx.ClaimOrder(ctx, store.ClaimOrderParams{ClaimedBy: nullStr(p.ID), ClaimedAt: nullMs(nowMs), UpdatedAt: nowMs, ID: orderID}); err != nil {
			return err
		}
		if err := insertItems(ctx, tx, cat, orderID, 0, lines, nowMs); err != nil {
			return err
		}
		cashEvent(tx, "takeaway.opened", billID)
		return nil
	})
	return billID, err
}

// releaseTakeaway sends a fully paid Takeaway's held Orders to the Kitchen and closes the Bill
// as pay now, serve later (TR-PAY-04, BR-R-31).
func (s *Service) releaseTakeaway(ctx context.Context, tx *dbx.Tx, p *auth.Principal, bill store.Bill, bl Billing, now int64) error {
	orders, err := tx.ListOrdersByBill(ctx, bill.ID)
	if err != nil {
		return err
	}
	areas := map[string]string{}
	for _, o := range orders {
		if o.Status != "claimed" {
			continue
		}
		if _, err := tx.ConfirmOrder(ctx, store.ConfirmOrderParams{ConfirmedBy: nullStr(p.ID), ConfirmedAt: nullMs(now), UpdatedAt: now, ID: o.ID}); err != nil {
			return err
		}
		if err := tx.ConfirmOrderItems(ctx, store.ConfirmOrderItemsParams{UpdatedAt: now, OrderID: o.ID}); err != nil {
			return err
		}
		items, err := tx.ListOrderItemsByOrder(ctx, o.ID)
		if err != nil {
			return err
		}
		for _, it := range items {
			areas[it.PrepAreaID] = o.ID
		}
	}
	for area, orderID := range areas {
		if err := queueChanged(ctx, tx, area, orderID); err != nil {
			return err
		}
	}
	if err := tx.SetPayNowServeLater(ctx, store.SetPayNowServeLaterParams{PayNowServeLaterBy: nullStr(p.ID), ID: bill.ID}); err != nil {
		return err
	}
	t := bl.Totals
	if err := tx.FreezeBillTotals(ctx, store.FreezeBillTotalsParams{Subtotal: nullMs(t.Subtotal), ItemDiscount: nullMs(t.ItemDiscount),
		BillDiscount: nullMs(t.BillDiscount), Service: nullMs(t.Service), Tax: nullMs(t.Tax), Total: nullMs(t.Total), UpdatedAt: now, ID: bill.ID}); err != nil {
		return err
	}
	return closeBill(ctx, tx, bill.ID, BillClosed, now)
}

// takeawayName decrypts a Takeaway Bill's Customer name ("Customer" once Anonymised).
func (s *Service) takeawayName(b store.Bill) string {
	if b.TakeawayNameEnc == nil {
		return "Customer"
	}
	name, err := s.vault.Open(b.KeyID.String, b.TakeawayNameEnc)
	if err != nil {
		return "Customer"
	}
	return string(name)
}

// ---------------------------------------------------------------- Post-outage Entry (BR-CS-08, G-21)

type OutagePayment struct {
	Method  string `json:"method"`
	Amount  int64  `json:"amount"`
	TakenAt int64  `json:"takenAt"`
}

type OutageInput struct {
	TableID      string          `json:"tableId,omitempty"`
	TakeawayName string          `json:"takeawayName,omitempty"`
	OpenedAt     int64           `json:"openedAt"`
	ClosedAt     int64           `json:"closedAt"`
	ServedBy     string          `json:"servedBy"` // crew.id or the Crew ID (W001) from the paper record
	Items        []ItemInput     `json:"items"`
	Payments     []OutagePayment `json:"payments"`
}

// PostOutage re-enters a paper Bill after an outage: original times, items, Payments and
// serving Crew; the items never reach the Kitchen; the Bill is closed and marked `post_outage`;
// cash joins the open drawer session. Audit-logged.
func (s *Service) PostOutage(ctx context.Context, p *auth.Principal, in OutageInput) (string, error) {
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	switch {
	case in.OpenedAt <= 0 || in.ClosedAt < in.OpenedAt || in.ClosedAt > nowMs:
		return "", invalid("Isi waktu buka dan tutup yang sudah lewat.")
	case in.TableID == "" && strings.TrimSpace(in.TakeawayName) == "":
		return "", invalid("Pilih meja atau tulis nama bawa pulang.")
	case len(in.Items) == 0 || len(in.Payments) == 0:
		return "", invalid("Isi item dan pembayaran dari catatan kertas.")
	}
	billID := platform.NewID()
	opened := platform.FromMillis(in.OpenedAt)
	var keyID string
	var nameEnc []byte
	if in.TableID == "" {
		var err error
		if keyID, nameEnc, err = s.vault.Seal(opened, []byte(strings.TrimSpace(in.TakeawayName))); err != nil {
			return "", err
		}
	}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		served, err := tx.GetCrew(ctx, in.ServedBy)
		if err != nil {
			// The paper record carries the Crew ID (W001), so that is accepted as well as crew.id.
			if served, err = tx.GetCrewByCrewID(ctx, strings.ToUpper(strings.TrimSpace(in.ServedBy))); err != nil {
				return invalid("Crew ID yang melayani tidak dikenal.")
			}
		}
		in.ServedBy = served.ID
		cat, err := catalog.Load(ctx, tx.Queries, true) // paper Bills may hold since-retired Products
		if err != nil {
			return err
		}
		var lines []resolved
		for i, item := range in.Items {
			r, prob := resolve(cat, i, item, nil, false)
			if prob != nil && prob.Code != "sold_out" && prob.Code != "unavailable" { // it was sold, whatever its state now
				return rejected([]ItemProblem{*prob})
			}
			if prob != nil {
				r = forceResolve(cat, item)
			}
			lines = append(lines, r)
		}
		date := BusinessDate(opened)
		n, err := tx.NextBillNumber(ctx, date)
		if err != nil {
			return err
		}
		kind := "dine_in"
		if in.TableID == "" {
			kind = "takeaway"
		}
		if err := tx.CreateBill(ctx, store.CreateBillParams{ID: billID, BusinessDate: date, Number: n, Type: kind, Source: "post_outage",
			KeyID: nullStr(keyID), TakeawayNameEnc: nameEnc, OpenedAt: in.OpenedAt, UpdatedAt: nowMs}); err != nil {
			return err
		}
		if in.TableID != "" {
			if err := tx.Exec(ctx, "INSERT INTO bill_table (bill_id, table_id, is_open) VALUES (?, ?, 0)", billID, in.TableID); err != nil {
				return err
			}
		}
		orderID := platform.NewID()
		if err := tx.CreateConfirmedOrder(ctx, store.CreateConfirmedOrderParams{ID: orderID, BillID: billID, Seq: 1, Source: "cashier", PlacedBy: nullStr(p.ID),
			ClaimedBy: nullStr(in.ServedBy), ClaimedAt: nullMs(in.OpenedAt), ConfirmedBy: nullStr(in.ServedBy), ConfirmedAt: nullMs(in.OpenedAt),
			CreatedAt: in.OpenedAt, UpdatedAt: nowMs}); err != nil {
			return err
		}
		if err := insertItems(ctx, tx, cat, orderID, 0, lines, in.OpenedAt); err != nil {
			return err
		}
		items, err := tx.ListOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		for _, it := range items {
			if err := tx.ServeItemAt(ctx, store.ServeItemAtParams{PreparingAt: nullMs(in.OpenedAt), ReadyAt: nullMs(in.ClosedAt), ServedAt: nullMs(in.ClosedAt),
				ServedBy: nullStr(in.ServedBy), UpdatedAt: nowMs, ID: it.ID}); err != nil {
				return err
			}
		}
		if _, err := tx.SettleBill(ctx, store.SettleBillParams{PricedAt: nullMs(in.ClosedAt), UpdatedAt: nowMs, ID: billID}); err != nil {
			return err
		}
		bill, err := tx.GetBill(ctx, billID)
		if err != nil {
			return err
		}
		bl, err := s.billing(ctx, tx.Queries, bill)
		if err != nil {
			return err
		}
		sess, sessErr := tx.GetOpenDrawerSession(ctx)
		var paid int64
		for _, pay := range in.Payments {
			if pay.Amount <= 0 || !slices.Contains([]string{PayCash, PayQRIS, PayCard}, pay.Method) {
				return invalid("Pembayaran tidak valid.")
			}
			id := platform.NewID()
			if err := tx.CreatePayment(ctx, store.CreatePaymentParams{ID: id, BillID: billID, Method: pay.Method, Status: PaySettled, Amount: pay.Amount,
				Tendered: sql.NullInt64{Int64: pay.Amount, Valid: pay.Method == PayCash}, TakenBy: in.ServedBy, TakenAt: max(pay.TakenAt, in.OpenedAt)}); err != nil {
				return err
			}
			if pay.Method == PayCash && sessErr == nil {
				if err := tx.SetPaymentSession(ctx, store.SetPaymentSessionParams{DrawerSessionID: nullStr(sess.ID), ID: id}); err != nil {
					return err
				}
			}
			paid += pay.Amount
		}
		if paid < bl.Totals.Total-100 { // cash may have been rounded on paper
			return invalid("Pembayaran kurang dari total tagihan.").(*httpx.Problem).With("total", bl.Totals.Total)
		}
		t := bl.Totals
		if err := tx.FreezeBillTotals(ctx, store.FreezeBillTotalsParams{Subtotal: nullMs(t.Subtotal), ItemDiscount: nullMs(t.ItemDiscount),
			BillDiscount: nullMs(t.BillDiscount), Service: nullMs(t.Service), Tax: nullMs(t.Tax), Total: nullMs(t.Total), UpdatedAt: nowMs, ID: billID}); err != nil {
			return err
		}
		if err := tx.SetBillStatus(ctx, store.SetBillStatusParams{Status: BillClosed, ClosedAt: nullMs(in.ClosedAt), UpdatedAt: nowMs, ID: billID}); err != nil {
			return err
		}
		cashEvent(tx, "post_outage.entered", billID)
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "post_outage.entered", Entity: "bill", EntityID: billID,
			After: map[string]any{"openedAt": in.OpenedAt, "closedAt": in.ClosedAt, "total": t.Total}})
	})
	return billID, err
}

// forceResolve builds a line for an item that is Sold Out or Hidden now but was sold on paper.
func forceResolve(cat catalog.Catalog, in ItemInput) resolved {
	for _, p := range cat.Products {
		for _, v := range p.Variants {
			if v.ID != in.VariantID {
				continue
			}
			r := resolved{product: p, variant: v, quantity: max(in.Quantity, 1), note: strings.TrimSpace(in.Note)}
			for _, g := range p.ModifierGroups {
				for _, m := range g.Modifiers {
					if slices.Contains(in.ModifierIDs, m.ID) {
						r.modifiers = append(r.modifiers, m)
					}
				}
			}
			return r
		}
	}
	return resolved{}
}

// CashierBills lists the Bills the Cashier works with: every open Bill (Tables and Takeaway)
// and those closed this business day, newest first.
func (s *Service) CashierBills(ctx context.Context, p *auth.Principal) ([]BillBrief, error) {
	sc, err := s.staffContext(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	day, _ := time.ParseInLocation(time.DateOnly, BusinessDate(now), platform.Jakarta)
	start := platform.Millis(day.Add(4 * time.Hour)) // the business day starts at 04:00 WIB
	bills, err := sc.q.ListTodayBills(ctx, store.ListTodayBillsParams{OpenedAt: start, ClosedAt: nullMs(start)})
	if err != nil {
		return nil, err
	}
	out := make([]BillBrief, 0, len(bills))
	for _, b := range bills {
		if b.Status == "merged" {
			continue
		}
		brief, err := s.brief(ctx, sc, p, b.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, *brief)
	}
	return out, nil
}

// HeldBy is the cash one Waiter still holds (BR-WT-14); clock-out is refused while it is
// non-zero (BR-HR-07).
func (s *Service) HeldBy(ctx context.Context, crewID string) (HeldCash, error) {
	held, err := s.db.Read().ListHeldPayments(ctx, crewID)
	if err != nil {
		return HeldCash{}, err
	}
	out := HeldCash{CrewID: crewID}
	for _, p := range held {
		out.Amount += p.Amount
		out.Payments++
	}
	return out, nil
}
