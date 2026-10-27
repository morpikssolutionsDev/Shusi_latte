package ordering

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// Bill changes and payment (OP-09, OP-10). Every total here comes from `pricing`; the apps
// only render it (CLAUDE.md).

const (
	PayCash = "cash"
	PayQRIS = "qris"
	PayCard = "card"

	PaySettled      = "settled"
	PayHeldByWaiter = "held_by_waiter"
	PayPending      = "pending_verification"
	PayWrittenOff   = "written_off"
)

type PaymentView struct {
	ID         string `json:"id"`
	SubBillID  string `json:"subBillId,omitempty"`
	Method     string `json:"method"`
	Status     string `json:"status"`
	Amount     int64  `json:"amount"`
	Tendered   *int64 `json:"tendered,omitempty"`
	Change     int64  `json:"change"`
	Rounding   int64  `json:"rounding"`
	TakenBy    string `json:"takenBy"`
	TakenAt    int64  `json:"takenAt"`
	VerifiedAt *int64 `json:"verifiedAt,omitempty"`
}

// SubBillView is one Sub-bill with its own discount share, service, tax and Payments (BR-R-04).
type SubBillView struct {
	ID      string   `json:"id"`
	Index   int64    `json:"index"`
	Count   int64    `json:"count"`
	Method  string   `json:"method"`
	Status  string   `json:"status"`
	Totals  Totals   `json:"totals"`
	Paid    int64    `json:"paid"`
	Balance int64    `json:"balance"`
	CashDue int64    `json:"cashDue"` // balance rounded to Rp 100 for a cash payment (BR-R-18)
	ItemIDs []string `json:"itemIds"`
}

// Billing is what is owed and paid on a Bill right now.
type Billing struct {
	Totals         Totals        `json:"totals"`
	Paid           int64         `json:"paid"`
	Balance        int64         `json:"balance"`
	CashDue        int64         `json:"cashDue"`
	PricedAt       int64         `json:"pricedAt"`
	ManualDiscount string        `json:"manualDiscount,omitempty"`
	SubBills       []SubBillView `json:"subBills"`
	Payments       []PaymentView `json:"payments"`
	Unassigned     []string      `json:"unassigned"` // live items not in any by-item share
}

// credit is what a Payment pays off: cash rounding is not part of the Bill (TR-PR-05).
func credit(p store.Payment) int64 {
	if p.Status == PayWrittenOff {
		return 0
	}
	return p.Amount - p.Rounding
}

// billing prices a Bill at its Settling moment (or now), with its manual discount, and divides
// it into Sub-bill shares when split (TR-PR-04).
func (s *Service) billing(ctx context.Context, q *store.Queries, bill store.Bill) (Billing, error) {
	at := s.clock.Now()
	if bill.PricedAt.Valid {
		at = platform.FromMillis(bill.PricedAt.Int64)
	}
	if bill.Total.Valid { // closed Bills keep their frozen totals
		at = platform.FromMillis(bill.ClosedAt.Int64)
	}
	lines, err := billLines(ctx, q, bill.ID)
	if err != nil {
		return Billing{}, err
	}
	items, err := q.ListOrderItemsByBill(ctx, bill.ID)
	if err != nil {
		return Billing{}, err
	}
	settings, err := s.outlet.Settings(ctx)
	if err != nil {
		return Billing{}, err
	}
	discounts, err := s.outlet.Discounts(ctx)
	if err != nil {
		return Billing{}, err
	}
	in := pricing.Input{Lines: lines, Auto: outlet.ActiveDiscounts(discounts, at), Settings: settings.Pricing.PricingFor()}
	out := Billing{PricedAt: platform.Millis(at), SubBills: []SubBillView{}, Payments: []PaymentView{}, Unassigned: []string{}}
	if bill.ManualDiscountID.Valid {
		if i := slices.IndexFunc(discounts, func(d outlet.Discount) bool { return d.ID == bill.ManualDiscountID.String }); i >= 0 {
			md := outlet.ToPricing(discounts[i])
			in.ManualBill = &md
			out.ManualDiscount = discounts[i].Name
		}
	}
	b := pricing.Calculate(in)
	out.Totals = Totals{Subtotal: b.Subtotal, ItemDiscount: b.ItemDiscount, BillDiscount: b.BillDiscount, Service: b.Service, Tax: b.Tax, Total: b.Total}

	payments, err := q.ListPaymentsByBill(ctx, bill.ID)
	if err != nil {
		return Billing{}, err
	}
	paidBy := map[string]int64{}
	for _, p := range payments {
		out.Paid += credit(p)
		paidBy[p.SubBillID.String] += credit(p)
		v := PaymentView{ID: p.ID, SubBillID: p.SubBillID.String, Method: p.Method, Status: p.Status, Amount: p.Amount, Change: p.ChangeGiven,
			Rounding: p.Rounding, TakenBy: p.TakenBy, TakenAt: p.TakenAt}
		if p.Tendered.Valid {
			v.Tendered = &p.Tendered.Int64
		}
		if p.VerifiedAt.Valid {
			v.VerifiedAt = &p.VerifiedAt.Int64
		}
		out.Payments = append(out.Payments, v)
	}
	out.Balance = out.Totals.Total - out.Paid
	out.CashDue, _ = pricing.RoundCash(max(out.Balance, 0))

	subs, err := q.ListSubBills(ctx, bill.ID)
	if err != nil {
		return Billing{}, err
	}
	if len(subs) == 0 {
		return out, nil
	}
	shares := make([]SubBillView, len(subs))
	for i, sb := range subs {
		shares[i] = SubBillView{ID: sb.ID, Index: sb.ShareIndex, Count: sb.ShareCount, Method: sb.Method, Status: sb.Status, ItemIDs: []string{}}
	}
	if subs[0].Method == "equal" {
		n := len(subs)
		total := pricing.SplitEqually(b.Total, n)
		sub := pricing.SplitEqually(b.Subtotal, n)
		itd := pricing.SplitEqually(b.ItemDiscount, n)
		bd := pricing.SplitEqually(b.BillDiscount, n)
		svc := pricing.SplitEqually(b.Service, n)
		for i := range shares {
			base := sub[i] - itd[i] - bd[i]
			shares[i].Totals = Totals{Subtotal: sub[i], ItemDiscount: itd[i], BillDiscount: bd[i], Service: svc[i], Tax: total[i] - base - svc[i], Total: total[i]}
		}
	} else {
		// By item: each share priced from its own lines, the Bill discount prorated by subtotal.
		shareOf := map[string]int{}
		for _, it := range items {
			if it.SubBillID.Valid {
				shareOf[it.ID] = slices.IndexFunc(subs, func(sb store.SubBill) bool { return sb.ID == it.SubBillID.String })
				shares[shareOf[it.ID]].ItemIDs = append(shares[shareOf[it.ID]].ItemIDs, it.ID)
			} else if it.Status != ItemCancelled && it.Status != "voided" {
				out.Unassigned = append(out.Unassigned, it.ID)
			}
		}
		after := make([]int64, len(shares))
		for _, lr := range b.Lines {
			i, ok := shareOf[lr.LineID]
			if !ok {
				continue
			}
			shares[i].Totals.Subtotal += lr.Gross
			shares[i].Totals.ItemDiscount += lr.Discount
			after[i] += lr.Net
		}
		bd := pricing.ProrateBillDiscount(b.BillDiscount, after)
		for i := range shares {
			base := after[i] - bd[i]
			shares[i].Totals.BillDiscount = bd[i]
			shares[i].Totals.Service = pricing.ApplyRate(base, in.Settings.ServiceRateBP)
			shares[i].Totals.Tax = pricing.ApplyRate(base+shares[i].Totals.Service, in.Settings.TaxRateBP)
			shares[i].Totals.Total = base + shares[i].Totals.Service + shares[i].Totals.Tax
		}
		// The Bill is the sum of its shares once split by item.
		out.Totals = Totals{}
		for _, sh := range shares {
			out.Totals.Subtotal += sh.Totals.Subtotal
			out.Totals.ItemDiscount += sh.Totals.ItemDiscount
			out.Totals.BillDiscount += sh.Totals.BillDiscount
			out.Totals.Service += sh.Totals.Service
			out.Totals.Tax += sh.Totals.Tax
			out.Totals.Total += sh.Totals.Total
		}
		out.Balance = out.Totals.Total - out.Paid
		out.CashDue, _ = pricing.RoundCash(max(out.Balance, 0))
	}
	for i, sb := range subs {
		if sb.Total.Valid { // closed shares keep their frozen totals
			shares[i].Totals = Totals{Subtotal: sb.Subtotal.Int64, ItemDiscount: sb.ItemDiscount.Int64, BillDiscount: sb.BillDiscount.Int64,
				Service: sb.Service.Int64, Tax: sb.Tax.Int64, Total: sb.Total.Int64}
		}
		shares[i].Paid = paidBy[sb.ID]
		shares[i].Balance = shares[i].Totals.Total - shares[i].Paid
		shares[i].CashDue, _ = pricing.RoundCash(max(shares[i].Balance, 0))
	}
	out.SubBills = shares
	return out, nil
}

// Billing returns a Bill's billing for staff screens.
func (s *Service) Billing(ctx context.Context, billID string) (Billing, error) {
	q := s.db.Read()
	bill, err := q.GetBill(ctx, billID)
	if errors.Is(err, sql.ErrNoRows) {
		return Billing{}, notFound("Tagihan tidak ditemukan")
	}
	if err != nil {
		return Billing{}, err
	}
	return s.billing(ctx, q, bill)
}

func liveBill(ctx context.Context, tx *dbx.Tx, id string) (store.Bill, error) {
	b, err := tx.GetBill(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return b, notFound("Tagihan tidak ditemukan")
	}
	if err != nil {
		return b, err
	}
	if b.Status != BillOpen && b.Status != BillSettling {
		return b, conflict("bill_closed", "Tagihan sudah ditutup")
	}
	return b, nil
}

func billEvents(tx *dbx.Tx, billID, kind string) {
	tx.Publish(realtime.Event{Topic: BillTopic(billID), Type: kind, ID: billID}, realtime.Event{Topic: WaitersTopic, Type: kind, ID: billID})
}

// ---------------------------------------------------------------- Settling (BR-R-27, BR-WT-15)

// Settle locks the Bill for payment: Customer ordering stops and the price is fixed.
func (s *Service) Settle(ctx context.Context, p *auth.Principal, billID string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := liveBill(ctx, tx, billID); err != nil {
			return err
		}
		if n, err := tx.SettleBill(ctx, store.SettleBillParams{PricedAt: nullMs(now), UpdatedAt: now, ID: billID}); err != nil {
			return err
		} else if n == 0 {
			return nil // already Settling
		}
		billEvents(tx, billID, "bill.settling")
		return nil
	})
}

// Unlock reopens a Settling Bill for new Orders (BR-WT-15).
func (s *Service) Unlock(ctx context.Context, p *auth.Principal, billID string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := liveBill(ctx, tx, billID); err != nil {
			return err
		}
		if _, err := tx.UnlockBill(ctx, store.UnlockBillParams{UpdatedAt: now, ID: billID}); err != nil {
			return err
		}
		billEvents(tx, billID, "bill.unlocked")
		return nil
	})
}

// ---------------------------------------------------------------- manual discount (BR-R-17)

// ApplyManualDiscount sets a manual Bill discount under a Management PIN Override.
func (s *Service) ApplyManualDiscount(ctx context.Context, p *auth.Principal, billID, discountID, token string) error {
	now := s.clock.Now()
	discounts, err := s.outlet.Discounts(ctx)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(discounts, func(d outlet.Discount) bool { return d.ID == discountID })
	if i < 0 || discounts[i].Trigger != "manual" || discounts[i].Scope != "bill" || !discounts[i].Active {
		return invalid("Pilih diskon manual tagihan yang aktif.")
	}
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := liveBill(ctx, tx, billID); err != nil {
			return err
		}
		if n, err := tx.CountPaymentsByBill(ctx, billID); err != nil {
			return err
		} else if n > 0 {
			return conflict("bill_has_payments", "Diskon harus diterapkan sebelum pembayaran")
		}
		approver, err := auth.ConsumeOverride(ctx, tx, now, p, token, auth.ActionManualDiscount, audit.Entry{Action: "bill.manual_discount",
			Entity: "bill", EntityID: billID, After: map[string]string{"discount": discounts[i].Name}})
		if err != nil {
			return err
		}
		if err := tx.SetManualDiscount(ctx, store.SetManualDiscountParams{ManualDiscountID: nullStr(discountID), ManualDiscountBy: nullStr(approver),
			UpdatedAt: platform.Millis(now), ID: billID}); err != nil {
			return err
		}
		billEvents(tx, billID, "bill.updated")
		return nil
	})
}

// ---------------------------------------------------------------- Payment (OP-10)

type PayInput struct {
	SubBillID string `json:"subBillId,omitempty"`
	Method    string `json:"method"`
	// Cash: the amount handed over. QRIS and card: the amount paid.
	Amount int64 `json:"amount"`
	// QRIS: staff saw it on the merchant app. Unverified QRIS needs an Outlet Admin PIN (BR-R-30).
	Verified bool `json:"verified"`
}

type PayResult struct {
	Payment PaymentView `json:"payment"`
	Billing Billing     `json:"billing"`
}

// Pay records a Payment. Starting payment puts the Bill into Settling (BR-R-27). Cash is rounded
// to Rp 100 when it settles the balance (BR-R-18); a Waiter's cash is Held by Waiter until the
// Cash Handover (BR-WT-14, TR-PAY-02).
func (s *Service) Pay(ctx context.Context, p *auth.Principal, billID string, in PayInput, token string) (PayResult, error) {
	if in.Amount <= 0 {
		return PayResult{}, invalid("Masukkan jumlah pembayaran.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	var out PayResult
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		bill, err := liveBill(ctx, tx, billID)
		if err != nil {
			return err
		}
		if bill.Status == BillOpen {
			if _, err := tx.SettleBill(ctx, store.SettleBillParams{PricedAt: nullMs(nowMs), UpdatedAt: nowMs, ID: billID}); err != nil {
				return err
			}
			if bill, err = tx.GetBill(ctx, billID); err != nil {
				return err
			}
		}
		bl, err := s.billing(ctx, tx.Queries, bill)
		if err != nil {
			return err
		}
		balance := bl.Balance
		if len(bl.SubBills) > 0 {
			i := slices.IndexFunc(bl.SubBills, func(sh SubBillView) bool { return sh.ID == in.SubBillID })
			if i < 0 {
				return invalid("Pilih sub-tagihan yang dibayar.")
			}
			if bl.SubBills[i].Status == "closed" {
				return conflict("sub_bill_closed", "Sub-tagihan ini sudah lunas")
			}
			if len(bl.Unassigned) > 0 {
				return conflict("split_outdated", "Ada item baru yang belum masuk sub-tagihan — atur ulang split")
			}
			balance = bl.SubBills[i].Balance
		} else if in.SubBillID != "" {
			return invalid("Tagihan ini tidak di-split.")
		}
		if balance <= 0 {
			return conflict("already_paid", "Sudah lunas")
		}
		pay := store.CreatePaymentParams{ID: platform.NewID(), BillID: billID, SubBillID: nullStr(in.SubBillID), Method: in.Method,
			Status: PaySettled, Amount: in.Amount, TakenBy: p.ID, TakenAt: nowMs}
		switch in.Method {
		case PayCash:
			pay.Tendered = nullMs(in.Amount)
			rounded, rounding := pricing.RoundCash(balance)
			if in.Amount >= rounded {
				pay.Amount, pay.Rounding, pay.ChangeGiven = rounded, rounding, in.Amount-rounded
			}
			if p.HasStation(auth.StationWaiter) && !p.HasStation(auth.StationCashier) {
				pay.Status = PayHeldByWaiter
			}
		case PayCard, PayQRIS:
			if in.Amount > balance {
				return invalid("Jumlah melebihi sisa tagihan.")
			}
			if in.Method == PayQRIS && !in.Verified {
				approver, err := auth.ConsumeOverride(ctx, tx, now, p, token, auth.ActionQRISUnverified, audit.Entry{Action: "payment.qris_pending",
					Entity: "bill", EntityID: billID, After: map[string]int64{"amount": in.Amount}})
				if err != nil {
					return err
				}
				pay.Status, pay.OverrideBy = PayPending, nullStr(approver)
			} else if in.Method == PayQRIS {
				pay.VerifiedBy, pay.VerifiedAt = nullStr(p.ID), nullMs(nowMs)
			}
		default:
			return invalid("Metode pembayaran tidak dikenal.")
		}
		if err := tx.CreatePayment(ctx, pay); err != nil {
			return err
		}
		// Counter cash goes straight into the open drawer session (TR-PAY-01).
		if pay.Method == PayCash && pay.Status == PaySettled {
			sess, err := tx.GetOpenDrawerSession(ctx)
			if errors.Is(err, sql.ErrNoRows) {
				return drawerClosed()
			}
			if err != nil {
				return err
			}
			if err := tx.SetPaymentSession(ctx, store.SetPaymentSessionParams{DrawerSessionID: nullStr(sess.ID), ID: pay.ID}); err != nil {
				return err
			}
		}
		if bill, err = tx.GetBill(ctx, billID); err != nil {
			return err
		}
		if out.Billing, err = s.billing(ctx, tx.Queries, bill); err != nil {
			return err
		}
		// A Takeaway is paid first; full payment sends it to the Kitchen (TR-PAY-04).
		if bill.Type == "takeaway" && out.Billing.Balance <= 0 {
			if err := s.releaseTakeaway(ctx, tx, p, bill, out.Billing, nowMs); err != nil {
				return err
			}
		}
		out.Payment = PaymentView{ID: pay.ID, SubBillID: in.SubBillID, Method: pay.Method, Status: pay.Status, Amount: pay.Amount,
			Change: pay.ChangeGiven, Rounding: pay.Rounding, TakenBy: p.ID, TakenAt: nowMs}
		if pay.Tendered.Valid {
			out.Payment.Tendered = &pay.Tendered.Int64
		}
		billEvents(tx, billID, "bill.paid")
		if pay.Status == PayHeldByWaiter || pay.Method == PayCash {
			tx.Publish(realtime.Event{Topic: "cash", Type: "payment.cash", ID: pay.ID})
		}
		return nil
	})
	return out, err
}

// ---------------------------------------------------------------- Close (OP-10)

// Close ends a fully paid Bill or Sub-bill. Items still Pending, Confirmed or Preparing need the
// staff member's "pay now, serve later" confirmation (BR-R-31, TR-DM-08). Totals are frozen.
func (s *Service) Close(ctx context.Context, p *auth.Principal, billID, subBillID string, payNowServeLater bool) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		bill, err := liveBill(ctx, tx, billID)
		if err != nil {
			return err
		}
		bl, err := s.billing(ctx, tx.Queries, bill)
		if err != nil {
			return err
		}
		if subBillID != "" {
			i := slices.IndexFunc(bl.SubBills, func(sh SubBillView) bool { return sh.ID == subBillID })
			if i < 0 {
				return notFound("Sub-tagihan tidak ditemukan")
			}
			sh := bl.SubBills[i]
			if sh.Balance > 0 {
				return conflict("balance_due", "Sub-tagihan belum lunas").With("balance", sh.Balance)
			}
			if err := tx.CloseSubBill(ctx, store.CloseSubBillParams{Subtotal: nullMs(sh.Totals.Subtotal), ItemDiscount: nullMs(sh.Totals.ItemDiscount),
				BillDiscount: nullMs(sh.Totals.BillDiscount), Service: nullMs(sh.Totals.Service), Tax: nullMs(sh.Totals.Tax), Total: nullMs(sh.Totals.Total),
				ClosedAt: nullMs(now), ID: subBillID}); err != nil {
				return err
			}
			for j, other := range bl.SubBills {
				if j != i && other.Status != "closed" {
					billEvents(tx, billID, "bill.paid")
					return nil // the Bill stays open until every share is closed
				}
			}
		} else if len(bl.SubBills) > 0 {
			return invalid("Tutup tiap sub-tagihan.")
		} else if bl.Balance > 0 {
			return conflict("balance_due", "Tagihan belum lunas").With("balance", bl.Balance)
		}
		items, err := tx.ListOrderItemsByBill(ctx, billID)
		if err != nil {
			return err
		}
		unfinished := slices.ContainsFunc(items, func(it store.OrderItem) bool {
			return it.Status == ItemPending || it.Status == "confirmed" || it.Status == "preparing"
		})
		if unfinished {
			if !payNowServeLater {
				return conflict("items_unfinished", "Masih ada item yang belum siap — konfirmasi bayar sekarang, sajikan nanti")
			}
			if err := tx.SetPayNowServeLater(ctx, store.SetPayNowServeLaterParams{PayNowServeLaterBy: nullStr(p.ID), ID: billID}); err != nil {
				return err
			}
		}
		t := bl.Totals
		if err := tx.FreezeBillTotals(ctx, store.FreezeBillTotalsParams{Subtotal: nullMs(t.Subtotal), ItemDiscount: nullMs(t.ItemDiscount),
			BillDiscount: nullMs(t.BillDiscount), Service: nullMs(t.Service), Tax: nullMs(t.Tax), Total: nullMs(t.Total), UpdatedAt: now, ID: billID}); err != nil {
			return err
		}
		return closeBill(ctx, tx, billID, BillClosed, now)
	})
}

// CloseUnpaid closes a Bill whose Customer left without paying, under an Outlet Admin PIN
// Override with a reason; it is reported as a loss (BR-R-32, G-14).
func (s *Service) CloseUnpaid(ctx context.Context, p *auth.Principal, billID, reason, token string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return invalid("Tulis alasan tagihan tak terbayar.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		bill, err := liveBill(ctx, tx, billID)
		if err != nil {
			return err
		}
		bl, err := s.billing(ctx, tx.Queries, bill)
		if err != nil {
			return err
		}
		approver, err := auth.ConsumeOverride(ctx, tx, now, p, token, auth.ActionUnpaidBill, audit.Entry{Action: "bill.unpaid", Entity: "bill",
			EntityID: billID, After: map[string]int64{"total": bl.Totals.Total, "paid": bl.Paid}, Reason: reason})
		if err != nil {
			return err
		}
		t := bl.Totals
		if err := tx.FreezeBillTotals(ctx, store.FreezeBillTotalsParams{Subtotal: nullMs(t.Subtotal), ItemDiscount: nullMs(t.ItemDiscount),
			BillDiscount: nullMs(t.BillDiscount), Service: nullMs(t.Service), Tax: nullMs(t.Tax), Total: nullMs(t.Total), UpdatedAt: nowMs, ID: billID}); err != nil {
			return err
		}
		if err := tx.CloseBillUnpaid(ctx, store.CloseBillUnpaidParams{UnpaidReason: nullStr(reason), UnpaidOverrideBy: nullStr(approver),
			ClosedAt: nullMs(nowMs), UpdatedAt: nowMs, ID: billID}); err != nil {
			return err
		}
		if err := tx.SetBillTablesOpen(ctx, store.SetBillTablesOpenParams{IsOpen: 0, BillID: billID}); err != nil {
			return err
		}
		tx.Publish(realtime.Event{Topic: BillTopic(billID), Type: "bill.closed", ID: billID}, realtime.Event{Topic: WaitersTopic, Type: "table.changed", ID: billID})
		return nil
	})
}

// ---------------------------------------------------------------- Move, Merge, Split (OP-09)

// Move re-points a Bill from one Table to a free one; an occupied target is refused — Merge
// instead (BR-R-28, TR-DM-06, G-11).
func (s *Service) Move(ctx context.Context, p *auth.Principal, billID, fromTableID, toTableID string) error {
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := liveBill(ctx, tx, billID); err != nil {
			return err
		}
		tables, err := tx.ListBillTables(ctx, billID)
		if err != nil {
			return err
		}
		if fromTableID == "" && len(tables) == 1 {
			fromTableID = tables[0].ID
		}
		if !slices.ContainsFunc(tables, func(t store.ListBillTablesRow) bool { return t.ID == fromTableID }) {
			return invalid("Pilih meja asal.")
		}
		target, err := tx.GetTable(ctx, toTableID)
		if errors.Is(err, sql.ErrNoRows) || err == nil && target.Active == 0 {
			return invalid("Pilih meja tujuan yang aktif.")
		}
		if err != nil {
			return err
		}
		if _, err := tx.GetOpenBillForTable(ctx, toTableID); err == nil {
			return conflict("table_occupied", "Meja tujuan sudah terisi — gunakan Gabung")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := tx.MoveBillTable(ctx, store.MoveBillTableParams{ToTable: toTableID, BillID: billID, FromTable: fromTableID}); err != nil {
			return err
		}
		billEvents(tx, billID, "bill.moved")
		tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "table.changed", ID: toTableID})
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "bill.moved", Entity: "bill", EntityID: billID,
			Before: map[string]string{"table": fromTableID}, After: map[string]string{"table": toTableID}})
	})
}

// Merge combines the Bills of several Tables into one; the absorbed Bills close as `merged`.
// It cannot be undone (BR-R-03, TR-DM-03).
func (s *Service) Merge(ctx context.Context, p *auth.Principal, intoID string, fromIDs []string) error {
	if len(fromIDs) == 0 {
		return invalid("Pilih tagihan yang digabung.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		check := func(id string) error {
			if _, err := liveBill(ctx, tx, id); err != nil {
				return err
			}
			if n, err := tx.CountPaymentsByBill(ctx, id); err != nil {
				return err
			} else if n > 0 {
				return conflict("bill_has_payments", "Tagihan yang sudah dibayar sebagian tidak bisa digabung")
			}
			if subs, err := tx.ListSubBills(ctx, id); err != nil {
				return err
			} else if len(subs) > 0 {
				return conflict("bill_split", "Batalkan split sebelum menggabung")
			}
			return nil
		}
		if err := check(intoID); err != nil {
			return err
		}
		for _, id := range fromIDs {
			if id == intoID {
				return invalid("Tagihan tujuan tidak bisa digabung ke dirinya sendiri.")
			}
			if err := check(id); err != nil {
				return err
			}
			// Keep Order numbers unique on the surviving Bill.
			max, err := tx.MaxOrderSeq(ctx, intoID)
			if err != nil {
				return err
			}
			if err := tx.RenumberOrders(ctx, store.RenumberOrdersParams{Seq: max, BillID: id}); err != nil {
				return err
			}
			for _, repoint := range []func(context.Context, string, string) error{
				func(c context.Context, into, from string) error {
					return tx.RepointOrders(c, store.RepointOrdersParams{IntoBill: into, FromBill: from})
				},
				func(c context.Context, into, from string) error {
					return tx.RepointCustomers(c, store.RepointCustomersParams{IntoBill: into, FromBill: from})
				},
				func(c context.Context, into, from string) error {
					return tx.RepointBillTables(c, store.RepointBillTablesParams{IntoBill: into, FromBill: from})
				},
				func(c context.Context, into, from string) error {
					return tx.RepointWaiterCalls(c, store.RepointWaiterCallsParams{IntoBill: nullStr(into), FromBill: nullStr(from)})
				},
			} {
				if err := repoint(ctx, intoID, id); err != nil {
					return err
				}
			}
			if err := tx.MarkBillMerged(ctx, store.MarkBillMergedParams{MergedIntoID: nullStr(intoID), ClosedAt: nullMs(nowMs), UpdatedAt: nowMs, ID: id}); err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: BillTopic(id), Type: "bill.merged", ID: intoID})
		}
		billEvents(tx, intoID, "bill.merged")
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "bill.merged", Entity: "bill", EntityID: intoID,
			Before: map[string][]string{"bills": fromIDs}})
	})
}

type SplitInput struct {
	Method string `json:"method"` // by_item | equal
	Count  int    `json:"count"`  // number of Sub-bills
	// By item: Order Item id → Sub-bill number (1..Count). Every live item must be assigned.
	Assign map[string]int `json:"assign,omitempty"`
}

// Split divides a Bill into Sub-bills, by item or equally, before any Payment (BR-R-04,
// BR-R-29, G-12). Splitting again replaces the previous split.
func (s *Service) Split(ctx context.Context, p *auth.Principal, billID string, in SplitInput) error {
	if in.Count < 2 || in.Count > 20 {
		return invalid("Jumlah sub-tagihan 2–20.")
	}
	if in.Method != "by_item" && in.Method != "equal" {
		return invalid("Pilih cara split.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := liveBill(ctx, tx, billID); err != nil {
			return err
		}
		if n, err := tx.CountPaymentsByBill(ctx, billID); err != nil {
			return err
		} else if n > 0 {
			return conflict("bill_has_payments", "Split harus diputuskan sebelum pembayaran")
		}
		if err := tx.ClearItemSubBills(ctx, billID); err != nil {
			return err
		}
		if err := tx.DeleteSubBills(ctx, billID); err != nil {
			return err
		}
		ids := make([]string, in.Count)
		for i := range ids {
			ids[i] = platform.NewID()
			if err := tx.CreateSubBill(ctx, store.CreateSubBillParams{ID: ids[i], BillID: billID, Method: in.Method, ShareIndex: int64(i + 1),
				ShareCount: int64(in.Count), CreatedAt: nowMs}); err != nil {
				return err
			}
		}
		if in.Method == "by_item" {
			items, err := tx.ListOrderItemsByBill(ctx, billID)
			if err != nil {
				return err
			}
			used := make([]bool, in.Count)
			for _, it := range items {
				if it.Status == ItemCancelled || it.Status == "voided" {
					continue
				}
				n, ok := in.Assign[it.ID]
				if !ok || n < 1 || n > in.Count {
					return invalid("Setiap item harus masuk satu sub-tagihan.")
				}
				used[n-1] = true
				if err := tx.AssignItemSubBill(ctx, store.AssignItemSubBillParams{SubBillID: nullStr(ids[n-1]), ID: it.ID}); err != nil {
					return err
				}
			}
			if slices.Contains(used, false) {
				return invalid("Setiap sub-tagihan butuh minimal satu item.")
			}
		}
		billEvents(tx, billID, "bill.split")
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "bill.split", Entity: "bill", EntityID: billID,
			After: map[string]any{"method": in.Method, "shares": in.Count}})
	})
}
