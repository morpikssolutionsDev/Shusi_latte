// Package reports implements BR-ADM-11 and the report endpoints
// from TRD §6.1: daily sales, sales by payment method, best
// sellers, Prep Time performance, Shift Closing, Unpaid Bills,
// Remakes by reason, and Post-outage Entries, each with CSV export.
//
// Scope note, read this before judging the empty results: the
// sales reports query the bill, payment, order_item and
// drawer_session tables from TRD §4.3 and §4.4. Those tables are
// created by the Waiter, Kitchen and Cashier phases, not by this
// step, so in Phase 1 the endpoints return empty rows. The queries
// are guarded by a table-existence check, so a missing source table
// yields an empty report instead of an error, and the reports will
// populate automatically once those phases create the tables with
// the TRD columns. The query logic itself is tested in
// reports_test.go against a test database that creates those
// tables with the TRD columns and sample rows, so the logic is
// verified before the source data exists in production.
//
// Dates are Asia/Jakarta calendar dates (YYYY-MM-DD). A report
// range is [from 00:00 Jakarta, to+1 day 00:00 Jakarta) in UTC
// milliseconds. Totals come from payments, not from recomputing
// Bill pricing, because a Bill total is not a stored column in
// TRD §4.3; payments are the money actually taken.
package reports

import (
	"database/sql"
	"fmt"
	"time"
)

var jakarta, _ = time.LoadLocation("Asia/Jakarta")

type Range struct {
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
	FromMS   int64  `json:"-"`
	ToMS     int64  `json:"-"`
}

// ParseRange parses from/to YYYY-MM-DD query parameters. When both
// are absent it defaults to the last 7 Jakarta days, inclusive.
func ParseRange(from, to string) (Range, error) {
	now := time.Now().In(jakarta)
	if from == "" && to == "" {
		toDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakarta)
		fromDate := toDate.AddDate(0, 0, -6)
		return Range{
			FromDate: fromDate.Format("2006-01-02"),
			ToDate:   toDate.Format("2006-01-02"),
			FromMS:   fromDate.UnixMilli(),
			ToMS:     toDate.AddDate(0, 0, 1).UnixMilli(),
		}, nil
	}
	if from == "" || to == "" {
		return Range{}, fmt.Errorf("from and to must be given together, as YYYY-MM-DD")
	}
	fromDate, err := time.ParseInLocation("2006-01-02", from, jakarta)
	if err != nil {
		return Range{}, fmt.Errorf("invalid from date: %w", err)
	}
	toDate, err := time.ParseInLocation("2006-01-02", to, jakarta)
	if err != nil {
		return Range{}, fmt.Errorf("invalid to date: %w", err)
	}
	if toDate.Before(fromDate) {
		return Range{}, fmt.Errorf("to must not be before from")
	}
	return Range{
		FromDate: from,
		ToDate:   to,
		FromMS:   fromDate.UnixMilli(),
		ToMS:     toDate.AddDate(0, 0, 1).UnixMilli(),
	}, nil
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	db.SetMaxOpenConns(1)
	return &Store{db: db}
}

func (s *Store) hasTable(name string) bool {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	return err == nil && count > 0
}

func (s *Store) hasAll(names ...string) bool {
	for _, n := range names {
		if !s.hasTable(n) {
			return false
		}
	}
	return true
}

func jakartaDate(ms int64) string {
	return time.UnixMilli(ms).In(jakarta).Format("2006-01-02")
}

// --- Daily sales ---

type DailySalesRow struct {
	Date          string `json:"date"`
	BillsClosed   int    `json:"bills_closed"`
	PaymentsTotal int64  `json:"payments_total"`
	CashTotal     int64  `json:"cash_total"`
	QRISTotal     int64  `json:"qris_total"`
	CardTotal     int64  `json:"card_total"`
}

func (s *Store) DailySales(r Range) []DailySalesRow {
	if !s.hasAll("bill", "payment") {
		return []DailySalesRow{}
	}
	rows, err := s.db.Query(
		`SELECT b.closed_at, p.method, p.amount FROM bill b
		 JOIN payment p ON p.bill_id = b.id
		 WHERE b.status = 'closed' AND b.closed_at >= ? AND b.closed_at < ?
		   AND p.status IN ('verified', 'settled', 'held_by_waiter')`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []DailySalesRow{}
	}
	defer rows.Close()
	byDate := map[string]*DailySalesRow{}
	for rows.Next() {
		var closedAt int64
		var method string
		var amount int64
		if err := rows.Scan(&closedAt, &method, &amount); err != nil {
			continue
		}
		date := jakartaDate(closedAt)
		row, ok := byDate[date]
		if !ok {
			row = &DailySalesRow{Date: date}
			byDate[date] = row
		}
		row.PaymentsTotal += amount
		switch method {
		case "cash":
			row.CashTotal += amount
		case "qris":
			row.QRISTotal += amount
		case "card":
			row.CardTotal += amount
		}
	}
	// Bill counts need bill ids, so count them separately.
	countRows, err := s.db.Query(
		`SELECT closed_at, COUNT(*) FROM bill WHERE status = 'closed' AND closed_at >= ? AND closed_at < ? GROUP BY closed_at`,
		r.FromMS, r.ToMS,
	)
	if err == nil {
		defer countRows.Close()
		for countRows.Next() {
			var closedAt int64
			var count int
			if err := countRows.Scan(&closedAt, &count); err == nil {
				date := jakartaDate(closedAt)
				if row, ok := byDate[date]; ok {
					row.BillsClosed += count
				} else {
					byDate[date] = &DailySalesRow{Date: date, BillsClosed: count}
				}
			}
		}
	}
	out := []DailySalesRow{}
	for date := r.FromDate; ; {
		if row, ok := byDate[date]; ok {
			out = append(out, *row)
		} else {
			out = append(out, DailySalesRow{Date: date})
		}
		if date == r.ToDate {
			break
		}
		next, _ := time.ParseInLocation("2006-01-02", date, jakarta)
		date = next.AddDate(0, 0, 1).Format("2006-01-02")
	}
	return out
}

// --- Payment methods ---

type PaymentMethodRow struct {
	Method   string `json:"method"`
	Payments int    `json:"payments"`
	Total    int64  `json:"total"`
}

func (s *Store) PaymentMethods(r Range) []PaymentMethodRow {
	if !s.hasTable("payment") {
		return []PaymentMethodRow{}
	}
	rows, err := s.db.Query(
		`SELECT method, COUNT(*), COALESCE(SUM(amount), 0) FROM payment
		 WHERE created_at >= ? AND created_at < ? AND status IN ('verified', 'settled', 'held_by_waiter')
		 GROUP BY method ORDER BY method`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []PaymentMethodRow{}
	}
	defer rows.Close()
	out := []PaymentMethodRow{}
	for rows.Next() {
		var row PaymentMethodRow
		if err := rows.Scan(&row.Method, &row.Payments, &row.Total); err == nil {
			out = append(out, row)
		}
	}
	return out
}

// --- Best sellers ---

type BestSellerRow struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	CategoryID  string `json:"category_id"`
	Quantity    int    `json:"quantity"`
	Amount      int64  `json:"amount"`
}

func (s *Store) BestSellers(r Range) []BestSellerRow {
	if !s.hasAll("order_item", "orders", "bill", "product") {
		return []BestSellerRow{}
	}
	rows, err := s.db.Query(
		`SELECT oi.product_id, COALESCE(oi.product_name_snapshot, p.name_id, oi.product_id),
		        p.category_id, SUM(oi.quantity), SUM(oi.quantity * oi.unit_price)
		 FROM order_item oi
		 JOIN orders o ON o.id = oi.order_id
		 JOIN bill b ON b.id = o.bill_id
		 LEFT JOIN product p ON p.id = oi.product_id
		 WHERE b.status = 'closed' AND b.closed_at >= ? AND b.closed_at < ?
		   AND oi.status NOT IN ('cancelled', 'voided')
		 GROUP BY oi.product_id ORDER BY SUM(oi.quantity) DESC, SUM(oi.quantity * oi.unit_price) DESC`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []BestSellerRow{}
	}
	defer rows.Close()
	out := []BestSellerRow{}
	for rows.Next() {
		var row BestSellerRow
		var category sql.NullString
		if err := rows.Scan(&row.ProductID, &row.ProductName, &category, &row.Quantity, &row.Amount); err == nil {
			row.CategoryID = category.String
			out = append(out, row)
		}
	}
	return out
}

// --- Prep Time performance (TR-KT-05) ---

type PrepTimeRow struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	CategoryID  string  `json:"category_id"`
	PrepAreaID  string  `json:"prep_area_id"`
	ItemsReady  int     `json:"items_ready"`
	ItemsOnTime int     `json:"items_on_time"`
	OnTimePct   float64 `json:"on_time_pct"`
	AvgPrepSec  float64 `json:"avg_prep_seconds"`
}

func (s *Store) PrepTime(r Range) []PrepTimeRow {
	if !s.hasAll("order_item", "orders", "product", "category") {
		return []PrepTimeRow{}
	}
	rows, err := s.db.Query(
		`SELECT oi.product_id, COALESCE(oi.product_name_snapshot, p.name_id, oi.product_id),
		        p.category_id, c.prep_area_id, oi.ready_at, o.confirmed_at, p.prep_time_seconds
		 FROM order_item oi
		 JOIN orders o ON o.id = oi.order_id
		 LEFT JOIN product p ON p.id = oi.product_id
		 LEFT JOIN category c ON c.id = p.category_id
		 WHERE oi.ready_at IS NOT NULL AND o.confirmed_at >= ? AND o.confirmed_at < ?`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []PrepTimeRow{}
	}
	defer rows.Close()
	byProduct := map[string]*PrepTimeRow{}
	sumSec := map[string]float64{}
	for rows.Next() {
		var productID, productName string
		var categoryID, prepAreaID sql.NullString
		var readyAt, confirmedAt int64
		var prepTimeSec sql.NullInt64
		if err := rows.Scan(&productID, &productName, &categoryID, &prepAreaID, &readyAt, &confirmedAt, &prepTimeSec); err != nil {
			continue
		}
		row, ok := byProduct[productID]
		if !ok {
			row = &PrepTimeRow{ProductID: productID, ProductName: productName, CategoryID: categoryID.String, PrepAreaID: prepAreaID.String}
			byProduct[productID] = row
		}
		elapsedSec := float64(readyAt-confirmedAt) / 1000.0
		row.ItemsReady++
		sumSec[productID] += elapsedSec
		if prepTimeSec.Valid && elapsedSec <= float64(prepTimeSec.Int64) {
			row.ItemsOnTime++
		}
	}
	out := []PrepTimeRow{}
	for _, row := range byProduct {
		if row.ItemsReady > 0 {
			row.OnTimePct = float64(row.ItemsOnTime) / float64(row.ItemsReady) * 100.0
			row.AvgPrepSec = sumSec[row.ProductID] / float64(row.ItemsReady)
		}
		out = append(out, *row)
	}
	return out
}

// --- Shift closings ---

type ShiftClosingRow struct {
	ID           string `json:"id"`
	CashierCrew  string `json:"cashier_crew_id"`
	OpeningFloat int64  `json:"opening_float"`
	ExpectedCash int64  `json:"expected_cash"`
	CountedCash  int64  `json:"counted_cash"`
	Difference   int64  `json:"difference"`
	OpenedAt     int64  `json:"opened_at"`
	ClosedAt     int64  `json:"closed_at"`
}

func (s *Store) ShiftClosings(r Range) []ShiftClosingRow {
	if !s.hasTable("drawer_session") {
		return []ShiftClosingRow{}
	}
	rows, err := s.db.Query(
		`SELECT id, cashier_crew_id, opening_float, expected_cash, counted_cash, difference, opened_at, closed_at
		 FROM drawer_session WHERE closed_at IS NOT NULL AND closed_at >= ? AND closed_at < ? ORDER BY closed_at`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []ShiftClosingRow{}
	}
	defer rows.Close()
	out := []ShiftClosingRow{}
	for rows.Next() {
		var row ShiftClosingRow
		var expected, counted, diff sql.NullInt64
		if err := rows.Scan(&row.ID, &row.CashierCrew, &row.OpeningFloat, &expected, &counted, &diff, &row.OpenedAt, &row.ClosedAt); err == nil {
			row.ExpectedCash = expected.Int64
			row.CountedCash = counted.Int64
			row.Difference = diff.Int64
			out = append(out, row)
		}
	}
	return out
}

// --- Unpaid bills (BR-R-32: shown as a loss) ---

type UnpaidBillRow struct {
	ID           string `json:"id"`
	Number       string `json:"number"`
	Type         string `json:"type"`
	UnpaidReason string `json:"unpaid_reason"`
	OpenedAt     int64  `json:"opened_at"`
	ClosedAt     int64  `json:"closed_at"`
	Items        int    `json:"items"`
	Amount       int64  `json:"amount"`
}

func (s *Store) UnpaidBills(r Range) []UnpaidBillRow {
	if !s.hasAll("bill", "orders", "order_item") {
		return []UnpaidBillRow{}
	}
	rows, err := s.db.Query(
		`SELECT b.id, b.number, b.type, b.unpaid_reason, b.opened_at, b.closed_at,
		        COUNT(oi.id), COALESCE(SUM(oi.quantity * oi.unit_price), 0)
		 FROM bill b
		 LEFT JOIN orders o ON o.bill_id = b.id
		 LEFT JOIN order_item oi ON oi.order_id = o.id AND oi.status NOT IN ('cancelled', 'voided')
		 WHERE b.status = 'unpaid' AND b.closed_at >= ? AND b.closed_at < ?
		 GROUP BY b.id ORDER BY b.closed_at`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []UnpaidBillRow{}
	}
	defer rows.Close()
	out := []UnpaidBillRow{}
	for rows.Next() {
		var row UnpaidBillRow
		var reason sql.NullString
		var closedAt sql.NullInt64
		if err := rows.Scan(&row.ID, &row.Number, &row.Type, &reason, &row.OpenedAt, &closedAt, &row.Items, &row.Amount); err == nil {
			row.UnpaidReason = reason.String
			row.ClosedAt = closedAt.Int64
			out = append(out, row)
		}
	}
	return out
}

// --- Remakes by reason (BR-R-33) ---

type RemakeRow struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

func (s *Store) Remakes(r Range) []RemakeRow {
	if !s.hasAll("order_item", "orders") {
		return []RemakeRow{}
	}
	rows, err := s.db.Query(
		`SELECT COALESCE(oi.remake_reason, ''), COUNT(*)
		 FROM order_item oi JOIN orders o ON o.id = oi.order_id
		 WHERE oi.remake_of_id IS NOT NULL AND o.confirmed_at >= ? AND o.confirmed_at < ?
		 GROUP BY oi.remake_reason ORDER BY COUNT(*) DESC`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []RemakeRow{}
	}
	defer rows.Close()
	out := []RemakeRow{}
	for rows.Next() {
		var row RemakeRow
		if err := rows.Scan(&row.Reason, &row.Count); err == nil {
			out = append(out, row)
		}
	}
	return out
}

// --- Post-outage entries (BR-CS-08) ---

type PostOutageRow struct {
	ID            string `json:"id"`
	Number        string `json:"number"`
	Type          string `json:"type"`
	OpenedAt      int64  `json:"opened_at"`
	ClosedAt      int64  `json:"closed_at"`
	Items         int    `json:"items"`
	PaymentsTotal int64  `json:"payments_total"`
}

func (s *Store) PostOutage(r Range) []PostOutageRow {
	if !s.hasAll("bill", "orders", "order_item", "payment") {
		return []PostOutageRow{}
	}
	rows, err := s.db.Query(
		`SELECT b.id, b.number, b.type, b.opened_at, b.closed_at,
		        (SELECT COUNT(*) FROM order_item oi JOIN orders o ON o.id = oi.order_id WHERE o.bill_id = b.id),
		        COALESCE((SELECT SUM(amount) FROM payment p WHERE p.bill_id = b.id), 0)
		 FROM bill b
		 WHERE b.source = 'post_outage' AND b.opened_at >= ? AND b.opened_at < ?
		 ORDER BY b.opened_at`,
		r.FromMS, r.ToMS,
	)
	if err != nil {
		return []PostOutageRow{}
	}
	defer rows.Close()
	out := []PostOutageRow{}
	for rows.Next() {
		var row PostOutageRow
		var closedAt sql.NullInt64
		if err := rows.Scan(&row.ID, &row.Number, &row.Type, &row.OpenedAt, &closedAt, &row.Items, &row.PaymentsTotal); err == nil {
			row.ClosedAt = closedAt.Int64
			out = append(out, row)
		}
	}
	return out
}
