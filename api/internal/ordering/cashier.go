package ordering

import (
	"database/sql"
	"errors"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
)

// Cashier operations for Phase 6 (BR-CS-01 to BR-CS-08,
// BR-R-05, BR-R-29, BR-R-30, BR-R-31, BR-R-32, TR-PR-05).
// Takeaway Bills (BR-CS-02) are created via CreateTakeawayBill:
// they are paid first, and Orders on them are created Confirmed
// only after full payment, which the caller checks via
// BillPaymentSummary. Post-outage Bills (BR-CS-08) bypass the
// Kitchen: they are created closed with source post_outage and
// their items marked Served, so they appear in reports as outage
// entries without ever entering a queue.

type Payment struct {
	ID             string  `json:"id"`
	BillID         string  `json:"bill_id"`
	SubBillID      *string `json:"sub_bill_id"`
	Method         string  `json:"method"`
	Amount         int64   `json:"amount"`
	Rounding       int64   `json:"rounding"`
	Status         string  `json:"status"`
	TakenBy        *string `json:"taken_by"`
	VerifiedBy     *string `json:"verified_by"`
	VerifiedAt     *int64  `json:"verified_at"`
	WriteOffReason *string `json:"write_off_reason"`
	CreatedAt      int64   `json:"created_at"`
}

type BillPaymentSummary struct {
	BillSummary
	Payments  []Payment `json:"payments"`
	TotalPaid int64     `json:"total_paid"`
	Balance   int64     `json:"balance"`
	Refunds   int64     `json:"refunds"`
}

// BillPaymentSummary returns totals, payments, refunds and balance.
func (s *Store) BillPaymentSummary(billID string, pricingStore *pricing.Store) (BillPaymentSummary, error) {
	summary, err := s.BillSummary(billID, pricingStore)
	if err != nil {
		return BillPaymentSummary{}, err
	}
	// Applied discounts reduce the total: sum item and bill
	// applied discounts, capped at the total.
	var applied int64
	rows, err := s.db.Query(`SELECT amount FROM applied_discount WHERE bill_id = ? OR order_item_id IN (SELECT id FROM order_item WHERE order_id IN (SELECT id FROM orders WHERE bill_id = ?))`, billID, billID)
	if err == nil {
		for rows.Next() {
			var amt int64
			if err := rows.Scan(&amt); err == nil {
				applied += amt
			}
		}
		rows.Close()
	}
	total := summary.Total - applied
	if total < 0 {
		total = 0
	}
	summary.Total = total
	// Payments: only verified payments count toward paid.
	prows, err := s.db.Query(`SELECT id, bill_id, sub_bill_id, method, amount, rounding, status, taken_by, verified_by, verified_at, write_off_reason, created_at FROM payment WHERE bill_id = ? ORDER BY created_at`, billID)
	payments := []Payment{}
	var paid int64
	if err == nil {
		for prows.Next() {
			var p Payment
			var subBill sqlNullString
			var takenBy, verifiedBy, writeOff sqlNullString
			var verifiedAt sqlNullInt64
			if err := prows.Scan(&p.ID, &p.BillID, &subBill, &p.Method, &p.Amount, &p.Rounding, &p.Status, &takenBy, &verifiedBy, &verifiedAt, &writeOff, &p.CreatedAt); err == nil {
				p.SubBillID = nullStringPtr(subBill)
				p.TakenBy = nullStringPtr(takenBy)
				p.VerifiedBy = nullStringPtr(verifiedBy)
				p.VerifiedAt = nullInt64Ptr(verifiedAt)
				p.WriteOffReason = nullStringPtr(writeOff)
				payments = append(payments, p)
				if p.Status == "verified" {
					paid += p.Amount
				}
			}
		}
		prows.Close()
	}
	var refunds int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM refund WHERE bill_id = ?`, billID).Scan(&refunds)
	balance := total - paid
	if balance < 0 {
		balance = 0
	}
	return BillPaymentSummary{BillSummary: summary, Payments: payments, TotalPaid: paid, Balance: balance, Refunds: refunds}, nil
}

type sqlNullString struct {
	String string
	Valid  bool
}

func (n *sqlNullString) Scan(value any) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	switch v := value.(type) {
	case string:
		n.String, n.Valid = v, true
	case []byte:
		n.String, n.Valid = string(v), true
	}
	return nil
}

type sqlNullInt64 struct {
	Int64 int64
	Valid bool
}

func (n *sqlNullInt64) Scan(value any) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	switch v := value.(type) {
	case int64:
		n.Int64, n.Valid = v, true
	case int:
		n.Int64, n.Valid = int64(v), true
	}
	return nil
}

func nullStringPtr(n sqlNullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func nullInt64(v *int64) sqlNullInt64 {
	if v == nil {
		return sqlNullInt64{}
	}
	return sqlNullInt64{Int64: *v, Valid: true}
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func nullInt64Ptr(n sqlNullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// CreateTakeawayBill opens a Takeaway Bill (BR-CS-02).
func (s *Store) CreateTakeawayBill(crewID, customerName string) (Bill, error) {
	if customerName == "" {
		return Bill{}, errors.New("takeaway customer name is required for pickup call-out")
	}
	now := time.Now()
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	day := now.In(jakarta).Format("20060102")
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM bill WHERE number LIKE ?`, day+"-%").Scan(&count)
	number := billNumber(day, count+1)
	bill := Bill{ID: NewID(), Number: number, Type: "takeaway", Source: "live", Status: "open", OpenedAt: now.UnixMilli()}
	if _, err := s.db.Exec(`INSERT INTO bill (id, number, type, source, status, takeaway_name, opened_at) VALUES (?, ?, 'takeaway', 'live', 'open', ?, ?)`, bill.ID, bill.Number, customerName, bill.OpenedAt); err != nil {
		return Bill{}, err
	}
	// Takeaway names are stored in plain text on the Bill in
	// TRD §4.3 (takeaway_name), unlike customer_entry names
	// which are encrypted; Anonymisation replaces it with
	// "Customer" after 48 hours (TR-RET-01).
	s.audit(crewID, "create", "bill", bill.ID, "takeaway")
	return bill, nil
}

func billNumber(day string, n int) string {
	// Daily sequence YYYYMMDD-NNN, matching OpenOrJoinBill.
	return day + "-" + pad3(n)
}

func pad3(n int) string {
	if n < 10 {
		return "00" + itoa(n)
	}
	if n < 100 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// RecordPayment records a Payment on a Bill (BR-WT-07,
// BR-CS-01, BR-R-05). Cash is rounded to nearest Rp 100
// (BR-R-18); QRIS starts pending verification (BR-R-30);
// card and cash are verified immediately. A Bill with items
// still Pending/Confirmed/Preparing can be paid only with
// payNowServeLater set (BR-R-31), which the caller passes.
func (s *Store) RecordPayment(crewID, billID string, method string, amount int64, subBillID *string, payNowServeLater bool) (Payment, error) {
	if method != "cash" && method != "qris" && method != "card" {
		return Payment{}, errors.New("payment method must be cash, qris or card")
	}
	if amount <= 0 {
		return Payment{}, errors.New("payment amount must be > 0")
	}
	bill, err := s.loadBill(billID)
	if err != nil {
		return Payment{}, err
	}
	if bill.Status == "closed" || bill.Status == "merged" {
		return Payment{}, errors.New("bill is already closed")
	}
	// BR-R-29: a Bill cannot be split once any Payment exists;
	// that is enforced in SplitBill by checking payments, not here.
	// BR-R-31: unfinished items require pay-now-serve-later.
	if !payNowServeLater {
		var unfinished int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM order_item oi JOIN orders o ON o.id = oi.order_id WHERE o.bill_id = ? AND oi.status IN ('pending','confirmed','preparing')`, billID).Scan(&unfinished)
		if unfinished > 0 {
			return Payment{}, errors.New("bill has unfinished items; confirm pay now, serve later first")
		}
	}
	now := time.Now().UnixMilli()
	status := "verified"
	var verifiedAt *int64
	var verifiedBy *string
	if method == "qris" {
		status = "pending"
	} else {
		verifiedAt = &now
		verifiedBy = &crewID
	}
	var rounding int64
	if method == "cash" {
		rounded := ((amount + 50) / 100) * 100
		rounding = rounded - amount
	}
	p := Payment{ID: NewID(), BillID: billID, SubBillID: subBillID, Method: method, Amount: amount, Rounding: rounding, Status: status, TakenBy: &crewID, VerifiedBy: verifiedBy, VerifiedAt: verifiedAt, CreatedAt: now}
	if _, err := s.db.Exec(
		`INSERT INTO payment (id, bill_id, sub_bill_id, method, amount, rounding, status, taken_by, verified_by, verified_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, billID, nullString(subBillID), method, amount, rounding, status, crewID, nullString(verifiedBy), sql.NullInt64{Int64: derefInt64(verifiedAt), Valid: verifiedAt != nil}, now,
	); err != nil {
		return Payment{}, err
	}
	// Starting payment puts the Bill into Settling (BR-R-27).
	if bill.Status == "open" {
		_, _ = s.db.Exec(`UPDATE bill SET status = 'settling' WHERE id = ?`, billID)
	}
	if payNowServeLater {
		_, _ = s.db.Exec(`UPDATE bill SET pay_now_serve_later_by = ? WHERE id = ?`, crewID, billID)
	}
	return p, nil
}

// VerifyPayment verifies a pending QRIS payment (BR-R-30).
func (s *Store) VerifyPayment(crewID, paymentID string) (Payment, error) {
	res, err := s.db.Exec(`UPDATE payment SET status = 'verified', verified_by = ?, verified_at = ? WHERE id = ? AND status = 'pending'`, crewID, time.Now().UnixMilli(), paymentID)
	if err != nil {
		return Payment{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Payment{}, errors.New("payment is not pending verification")
	}
	return s.loadPayment(paymentID)
}

// WriteOffPayment writes off a pending QRIS payment (BR-CS-07).
func (s *Store) WriteOffPayment(crewID, paymentID, reason string) (Payment, error) {
	if reason == "" {
		return Payment{}, errors.New("write-off reason is required")
	}
	res, err := s.db.Exec(`UPDATE payment SET status = 'written_off', write_off_reason = ?, verified_by = ?, verified_at = ? WHERE id = ? AND status = 'pending'`, reason, crewID, time.Now().UnixMilli(), paymentID)
	if err != nil {
		return Payment{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Payment{}, errors.New("payment is not pending verification")
	}
	return s.loadPayment(paymentID)
}

func (s *Store) loadPayment(id string) (Payment, error) {
	var p Payment
	var subBill sqlNullString
	var takenBy, verifiedBy, writeOff sqlNullString
	var verifiedAt sqlNullInt64
	err := s.db.QueryRow(`SELECT id, bill_id, sub_bill_id, method, amount, rounding, status, taken_by, verified_by, verified_at, write_off_reason, created_at FROM payment WHERE id = ?`, id).Scan(&p.ID, &p.BillID, &subBill, &p.Method, &p.Amount, &p.Rounding, &p.Status, &takenBy, &verifiedBy, &verifiedAt, &writeOff, &p.CreatedAt)
	if err != nil {
		return Payment{}, errors.New("payment not found")
	}
	p.SubBillID = nullStringPtr(subBill)
	p.TakenBy = nullStringPtr(takenBy)
	p.VerifiedBy = nullStringPtr(verifiedBy)
	p.VerifiedAt = nullInt64Ptr(verifiedAt)
	p.WriteOffReason = nullStringPtr(writeOff)
	return p, nil
}

// CloseBill closes a fully paid Bill (BR-R-05). Balance must be
// zero, and no QRIS payment may still be pending.
func (s *Store) CloseBill(crewID, billID string, pricingStore *pricing.Store) (Bill, error) {
	summary, err := s.BillPaymentSummary(billID, pricingStore)
	if err != nil {
		return Bill{}, err
	}
	if summary.Balance != 0 {
		return Bill{}, errors.New("bill is not fully paid")
	}
	var pending int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM payment WHERE bill_id = ? AND status = 'pending'`, billID).Scan(&pending)
	if pending > 0 {
		return Bill{}, errors.New("bill has pending QRIS payments to verify or write off")
	}
	res, err := s.db.Exec(`UPDATE bill SET status = 'closed', closed_at = ? WHERE id = ? AND status IN ('open','settling')`, time.Now().UnixMilli(), billID)
	if err != nil {
		return Bill{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Bill{}, errors.New("bill cannot be closed")
	}
	// Free the Table(s).
	_, _ = s.db.Exec(`UPDATE bill_table SET is_open = 0 WHERE bill_id = ?`, billID)
	// Receipt: customer + internal copies are printed by the
	// Staff app from the summary; record both prints here.
	now := time.Now().UnixMilli()
	_, _ = s.db.Exec(`INSERT INTO receipt_print (id, bill_id, copy_type, is_reprint, printed_by, printed_at) VALUES (?, ?, 'customer', 0, ?, ?)`, NewID(), billID, crewID, now)
	_, _ = s.db.Exec(`INSERT INTO receipt_print (id, bill_id, copy_type, is_reprint, printed_by, printed_at) VALUES (?, ?, 'internal', 0, ?, ?)`, NewID(), billID, crewID, now)
	s.audit(crewID, "close", "bill", billID, "")
	return s.loadBill(billID)
}

// CloseUnpaidBill closes a Bill as Unpaid with an Override
// (BR-R-32). The caller must have consumed an override token.
func (s *Store) CloseUnpaidBill(crewID, approverCrewID, billID, reason string) (Bill, error) {
	if reason == "" {
		return Bill{}, errors.New("unpaid reason is required")
	}
	res, err := s.db.Exec(`UPDATE bill SET status = 'unpaid', unpaid_reason = ?, unpaid_override_by = ?, closed_at = ? WHERE id = ? AND status IN ('open','settling')`, reason, approverCrewID, time.Now().UnixMilli(), billID)
	if err != nil {
		return Bill{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Bill{}, errors.New("bill cannot be closed as unpaid")
	}
	_, _ = s.db.Exec(`UPDATE bill_table SET is_open = 0 WHERE bill_id = ?`, billID)
	s.audit(crewID, "close_unpaid", "bill", billID, reason)
	return s.loadBill(billID)
}

// RefundBill records a cash Refund from the Drawer (BR-CS-03).
// The amount must not exceed the refund cap for the Bill, or
// the caller must have a Management override for over-cap.
func (s *Store) RefundBill(crewID, approverCrewID string, billID string, amount int64, reason string, refundCap int64, overCap bool) error {
	if amount <= 0 || reason == "" {
		return errors.New("refund amount and reason are required")
	}
	if amount > refundCap && !overCap {
		return errors.New("refund exceeds the cap; a management override is required")
	}
	_, err := s.db.Exec(`INSERT INTO refund (id, bill_id, amount, reason, override_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`, NewID(), billID, amount, reason, approverCrewID, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	s.audit(crewID, "refund", "bill", billID, reason)
	return nil
}

// DrawerSession types for BR-CS-04 to BR-CS-06.
type DrawerSession struct {
	ID            string `json:"id"`
	CashDrawerID  string `json:"cash_drawer_id"`
	CashierCrewID string `json:"cashier_crew_id"`
	OpenedAt      int64  `json:"opened_at"`
	ClosedAt      *int64 `json:"closed_at"`
	OpeningFloat  int64  `json:"opening_float"`
	CountedCash   *int64 `json:"counted_cash"`
	ExpectedCash  *int64 `json:"expected_cash"`
	Difference    *int64 `json:"difference"`
}

func (s *Store) OpenDrawerSession(crewID string, openingFloat int64) (DrawerSession, error) {
	if openingFloat < 0 {
		return DrawerSession{}, errors.New("opening float must be >= 0")
	}
	// A Cashier change mid-day triggers a Shift Closing first:
	// only one open drawer session at a time.
	var openCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM drawer_session WHERE closed_at IS NULL`).Scan(&openCount)
	if openCount > 0 {
		return DrawerSession{}, errors.New("a drawer session is already open; close it first")
	}
	ds := DrawerSession{ID: NewID(), CashDrawerID: "main", CashierCrewID: crewID, OpenedAt: time.Now().UnixMilli(), OpeningFloat: openingFloat}
	if _, err := s.db.Exec(`INSERT INTO drawer_session (id, cash_drawer_id, cashier_crew_id, opened_at, opening_float) VALUES (?, 'main', ?, ?, ?)`, ds.ID, crewID, ds.OpenedAt, openingFloat); err != nil {
		return DrawerSession{}, err
	}
	return ds, nil
}

// CloseDrawerSession computes expected cash (BR-CS-06):
// opening float + confirmed handovers + cash taken at counter
// - change given - cash refunds. Change given is approximated
// by cash payment rounding differences? No: change is not
// stored separately in this phase, so expected cash uses
// cash payments as taken, minus refunds, plus handovers.
// Counted cash and difference are recorded.
func (s *Store) CloseDrawerSession(crewID, sessionID string, countedCash int64) (DrawerSession, error) {
	var openingFloat int64
	var openedAt int64
	err := s.db.QueryRow(`SELECT opening_float, opened_at FROM drawer_session WHERE id = ? AND closed_at IS NULL`, sessionID).Scan(&openingFloat, &openedAt)
	if err != nil {
		return DrawerSession{}, errors.New("drawer session is not open")
	}
	var handovers int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(confirmed_amount),0) FROM cash_handover WHERE drawer_session_id = ? AND status = 'confirmed'`, sessionID).Scan(&handovers)
	var cashTaken int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM payment WHERE method = 'cash' AND status = 'verified' AND created_at >= ?`, openedAt).Scan(&cashTaken)
	var refunds int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM refund WHERE created_at >= ?`, openedAt).Scan(&refunds)
	expected := openingFloat + handovers + cashTaken - refunds
	difference := countedCash - expected
	now := time.Now().UnixMilli()
	if _, err := s.db.Exec(`UPDATE drawer_session SET closed_at = ?, counted_cash = ?, expected_cash = ?, difference = ? WHERE id = ?`, now, countedCash, expected, difference, sessionID); err != nil {
		return DrawerSession{}, err
	}
	s.audit(crewID, "shift_close", "drawer_session", sessionID, "")
	return DrawerSession{ID: sessionID, CashDrawerID: "main", CashierCrewID: crewID, OpenedAt: openedAt, ClosedAt: &now, OpeningFloat: openingFloat, CountedCash: &countedCash, ExpectedCash: &expected, Difference: &difference}, nil
}

// RecordHandover records cash from a Waiter as Held (BR-WT-14).
func (s *Store) RecordHandover(waiterCrewID string, amount int64, drawerSessionID *string) error {
	if amount < 0 {
		return errors.New("handover amount must be >= 0")
	}
	_, err := s.db.Exec(`INSERT INTO cash_handover (id, waiter_crew_id, drawer_session_id, amount, status, created_at) VALUES (?, ?, ?, ?, 'pending', ?)`, NewID(), waiterCrewID, nullString(drawerSessionID), amount, time.Now().UnixMilli())
	return err
}

// ConfirmHandover confirms a handover with the amount received
// (BR-CS-05); any difference is recorded against the Waiter.
func (s *Store) ConfirmHandover(crewID, handoverID string, confirmedAmount int64) error {
	var amount int64
	err := s.db.QueryRow(`SELECT amount FROM cash_handover WHERE id = ? AND status = 'pending'`, handoverID).Scan(&amount)
	if err != nil {
		return errors.New("handover is not pending")
	}
	difference := confirmedAmount - amount
	_, err = s.db.Exec(`UPDATE cash_handover SET confirmed_amount = ?, difference = ?, confirmed_by = ?, status = 'confirmed', confirmed_at = ? WHERE id = ?`, confirmedAmount, difference, crewID, time.Now().UnixMilli(), handoverID)
	return err
}

// HeldCash returns cash held by a Waiter (BR-HR-07 helper).
func (s *Store) HeldCash(waiterCrewID string) int64 {
	var total int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM payment WHERE taken_by = ? AND method = 'cash' AND status = 'verified'`, waiterCrewID).Scan(&total)
	var handed int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(confirmed_amount),0) FROM cash_handover WHERE waiter_crew_id = ? AND status = 'confirmed'`, waiterCrewID).Scan(&handed)
	if total < handed {
		return 0
	}
	return total - handed
}

// CreatePostOutageBill enters a paper Bill after an outage
// (BR-CS-08). It is created closed with source post_outage,
// bypassing the Kitchen, and marked outage in reports.
type PostOutagePayment struct {
	Method string `json:"method"`
	Amount int64  `json:"amount"`
}

func (s *Store) CreatePostOutageBill(crewID string, billNumber string, items []OrderItemInput, payments []PostOutagePayment) (Bill, error) {
	// Simplified post-outage entry: create a closed Bill with
	// the given items as Served order items, and payments as
	// verified. Original times are taken as now in this phase;
	// the Staff app supplies original times in a later
	// refinement if the owner asks for them.
	now := time.Now().UnixMilli()
	bill := Bill{ID: NewID(), Number: billNumber, Type: "dine_in", Source: "post_outage", Status: "closed", OpenedAt: now, ClosedAt: &now}
	if _, err := s.db.Exec(`INSERT INTO bill (id, number, type, source, status, opened_at, closed_at) VALUES (?, ?, 'dine_in', 'post_outage', 'closed', ?, ?)`, bill.ID, bill.Number, now, now); err != nil {
		return Bill{}, err
	}
	orderID := NewID()
	if _, err := s.db.Exec(`INSERT INTO orders (id, bill_id, source, status, confirmed_at, created_at) VALUES (?, ?, 'cashier', 'served', ?, ?)`, orderID, bill.ID, now, now); err != nil {
		return Bill{}, err
	}
	for _, in := range items {
		// Price from catalog at entry time.
		var price int64
		var prodNameID, prodNameEN, varNameID, varNameEN string
		if err := s.db.QueryRow(`SELECT v.price, p.name_id, p.name_en, v.name_id, v.name_en FROM variant v JOIN product p ON p.id = v.product_id WHERE v.id = ?`, in.VariantID).Scan(&price, &prodNameID, &prodNameEN, &varNameID, &varNameEN); err != nil {
			return Bill{}, err
		}
		if _, err := s.db.Exec(`INSERT INTO order_item (id, order_id, product_id, variant_id, product_name_snapshot, variant_name_snapshot, quantity, unit_price, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'served')`, NewID(), orderID, in.ProductID, in.VariantID, pickName(prodNameID, prodNameEN), pickName(varNameID, varNameEN), in.Quantity, price); err != nil {
			return Bill{}, err
		}
	}
	for _, pay := range payments {
		if _, err := s.db.Exec(`INSERT INTO payment (id, bill_id, method, amount, status, taken_by, verified_by, verified_at, created_at) VALUES (?, ?, ?, ?, 'verified', ?, ?, ?, ?)`, NewID(), bill.ID, pay.Method, pay.Amount, crewID, crewID, now, now); err != nil {
			return Bill{}, err
		}
	}
	s.audit(crewID, "create", "bill", bill.ID, "post_outage")
	return bill, nil
}
