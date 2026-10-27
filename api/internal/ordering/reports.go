package ordering

import (
	"context"
	"time"

	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

// Operations report (BR-ADM-11): what Management reviews after a day — Shift Closings, Unpaid
// Bills, Remakes, Refunds and Post-outage Entries.

type ShiftRow struct {
	DrawerView
	Expected *int64 `json:"expected,omitempty"`
}

type BillRow struct {
	ID       string   `json:"id"`
	Number   string   `json:"number"`
	Tables   []string `json:"tables"`
	Total    int64    `json:"total"`
	ClosedAt int64    `json:"closedAt"`
	Reason   string   `json:"reason,omitempty"`
	By       string   `json:"by,omitempty"`
}

type RemakeRow struct {
	BillNumber string `json:"billNumber"`
	Item       string `json:"item"`
	Reason     string `json:"reason"`
	ApprovedBy string `json:"approvedBy"`
	At         int64  `json:"at"`
}

type Operations struct {
	Date          string       `json:"date"`
	ShiftClosings []ShiftRow   `json:"shiftClosings"`
	Unpaid        []BillRow    `json:"unpaid"`
	Remakes       []RemakeRow  `json:"remakes"`
	Refunds       []RefundView `json:"refunds"`
	PostOutage    []BillRow    `json:"postOutage"`
}

func (s *Service) Operations(ctx context.Context, date string) (Operations, error) {
	day, err := time.ParseInLocation(time.DateOnly, date, platform.Jakarta)
	if err != nil {
		return Operations{}, invalid("Tanggal tidak valid.")
	}
	from, to := platform.Millis(day.Add(4*time.Hour)), platform.Millis(day.Add(28*time.Hour))
	sc, err := s.staffContext(ctx)
	if err != nil {
		return Operations{}, err
	}
	q := sc.q
	out := Operations{Date: date, ShiftClosings: []ShiftRow{}, Unpaid: []BillRow{}, Remakes: []RemakeRow{}, Refunds: []RefundView{}, PostOutage: []BillRow{}}

	sessions, err := q.ListDrawerSessionsBetween(ctx, store.ListDrawerSessionsBetweenParams{OpenedAt: from, OpenedAt_2: to})
	if err != nil {
		return Operations{}, err
	}
	for _, d := range sessions {
		out.ShiftClosings = append(out.ShiftClosings, ShiftRow{DrawerView: DrawerView{ID: d.ID, Cashier: sc.crew[d.CashierCrewID].Name, OpeningFloat: d.OpeningFloat,
			OpenedAt: d.OpenedAt, ClosedAt: ptr(d.ClosedAt), Counted: ptr(d.CountedCash), Difference: ptr(d.Difference)}, Expected: ptr(d.ExpectedCash)})
	}

	bills, err := q.ListClosedBillsBetween(ctx, store.ListClosedBillsBetweenParams{ClosedAt: nullMs(from), ClosedAt_2: nullMs(to)})
	if err != nil {
		return Operations{}, err
	}
	labels := func(id string) []string {
		var out []string
		for _, t := range sc.billTables(ctx, id) {
			out = append(out, t.Label)
		}
		return out
	}
	for _, b := range bills {
		row := BillRow{ID: b.ID, Number: BillNumber(b.Number), Tables: labels(b.ID), Total: b.Total.Int64, ClosedAt: b.ClosedAt.Int64}
		if b.Status == "unpaid" {
			row.Reason, row.By = b.UnpaidReason.String, sc.crew[b.UnpaidOverrideBy.String].Name
			out.Unpaid = append(out.Unpaid, row)
		}
		if b.Source == "post_outage" {
			out.PostOutage = append(out.PostOutage, row)
		}
	}

	remakes, err := q.ListRemakesBetween(ctx, store.ListRemakesBetweenParams{CreatedAt: from, CreatedAt_2: to})
	if err != nil {
		return Operations{}, err
	}
	for _, r := range remakes {
		b, _ := q.GetBill(ctx, r.BillID)
		out.Remakes = append(out.Remakes, RemakeRow{BillNumber: BillNumber(b.Number), Item: r.ProductNameID, Reason: r.RemakeReason.String,
			ApprovedBy: sc.crew[r.RemakeOverrideBy.String].Name, At: r.CreatedAt})
	}

	refunds, err := q.ListRefundsBetween(ctx, store.ListRefundsBetweenParams{CreatedAt: from, CreatedAt_2: to})
	if err != nil {
		return Operations{}, err
	}
	for _, r := range refunds {
		b, _ := q.GetBill(ctx, r.BillID)
		out.Refunds = append(out.Refunds, RefundView{ID: r.ID, BillID: r.BillID, BillNumber: BillNumber(b.Number), Amount: r.Amount, Reason: r.Reason,
			ApprovedBy: sc.crew[r.ApprovedBy].Name, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// HistoryRow is one closed Bill of a business day as Management sees it after the fact: names
// while they are kept, "Customer" once Anonymised (TR-RET-01, G-24).
type HistoryRow struct {
	ID         string   `json:"id"`
	Number     string   `json:"number"`
	Tables     []string `json:"tables"`
	Customers  []string `json:"customers"`
	Total      int64    `json:"total"`
	Status     string   `json:"status"` // closed | unpaid
	Source     string   `json:"source"` // live | post_outage
	ClosedAt   int64    `json:"closedAt"`
	Anonymised bool     `json:"anonymised"`
}

// BillHistory lists the Bills closed in one business day, newest first.
func (s *Service) BillHistory(ctx context.Context, date string) ([]HistoryRow, error) {
	day, err := time.ParseInLocation(time.DateOnly, date, platform.Jakarta)
	if err != nil {
		return nil, invalid("Tanggal tidak valid.")
	}
	sc, err := s.staffContext(ctx)
	if err != nil {
		return nil, err
	}
	from, to := platform.Millis(day.Add(4*time.Hour)), platform.Millis(day.Add(28*time.Hour))
	bills, err := sc.q.ListClosedBillsBetween(ctx, store.ListClosedBillsBetweenParams{ClosedAt: nullMs(from), ClosedAt_2: nullMs(to)})
	if err != nil {
		return nil, err
	}
	out := make([]HistoryRow, 0, len(bills))
	for i := len(bills) - 1; i >= 0; i-- {
		b := bills[i]
		row := HistoryRow{ID: b.ID, Number: BillNumber(b.Number), Tables: []string{}, Customers: []string{}, Total: b.Total.Int64, Status: b.Status,
			Source: b.Source, ClosedAt: b.ClosedAt.Int64, Anonymised: b.AnonymisedAt.Valid}
		for _, t := range sc.billTables(ctx, b.ID) {
			row.Tables = append(row.Tables, t.Label)
		}
		entries, err := sc.q.ListCustomerEntries(ctx, b.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			row.Customers = append(row.Customers, s.name(e))
		}
		out = append(out, row)
	}
	return out, nil
}
