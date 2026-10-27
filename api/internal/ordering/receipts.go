package ordering

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

// Receipts (TRD §10): the API returns the receipt content; the Staff app lays it out for a
// 58 mm printer (ESC/POS) or the on-screen ReceiptPreview. Data is fetched before printing so
// a print succeeds even if the connection drops afterwards (TR-PRN-04).

const OutletName = "Sushi Latte"

type ReceiptLine struct {
	Quantity  int64    `json:"quantity"`
	Name      string   `json:"name"`
	Variant   string   `json:"variant,omitempty"`
	Modifiers []string `json:"modifiers"`
	Amount    int64    `json:"amount"` // (unit + Modifiers) × quantity
	Remake    bool     `json:"remake"`
}

type Receipt struct {
	Outlet         string        `json:"outlet"`
	Copy           string        `json:"copy"` // customer | internal
	Reprint        bool          `json:"reprint"`
	BillNumber     string        `json:"billNumber"`
	SubBill        string        `json:"subBill,omitempty"` // "1/2" for a Sub-bill
	Tables         []string      `json:"tables"`
	OpenedAt       int64         `json:"openedAt"`
	ClosedAt       *int64        `json:"closedAt,omitempty"`
	PrintedAt      int64         `json:"printedAt"`
	PrintedBy      string        `json:"printedBy"`
	Customers      []string      `json:"customers"` // Customer copy only (TR-PRN-05)
	Lines          []ReceiptLine `json:"lines"`
	Totals         Totals        `json:"totals"`
	ManualDiscount string        `json:"manualDiscount,omitempty"`
	Payments       []PaymentView `json:"payments"`
	Balance        int64         `json:"balance"`
}

// Receipt builds one copy and records the print; any later print of the same copy is a
// REPRINT and is audit-logged (BR-R-37, TR-PRN-08).
func (s *Service) Receipt(ctx context.Context, p *auth.Principal, billID, subBillID, copy string) (Receipt, error) {
	if copy != "customer" && copy != "internal" {
		return Receipt{}, invalid("Pilih salinan struk.")
	}
	now := s.clock.Now()
	var out Receipt
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		bill, err := tx.GetBill(ctx, billID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("Tagihan tidak ditemukan")
		}
		if err != nil {
			return err
		}
		bl, err := s.billing(ctx, tx.Queries, bill)
		if err != nil {
			return err
		}
		out = Receipt{Outlet: OutletName, Copy: copy, BillNumber: BillNumber(bill.Number), OpenedAt: bill.OpenedAt, PrintedAt: platform.Millis(now),
			PrintedBy: p.Name, Customers: []string{}, Lines: []ReceiptLine{}, Totals: bl.Totals, ManualDiscount: bl.ManualDiscount,
			Payments: []PaymentView{}, Balance: bl.Balance}
		if bill.ClosedAt.Valid {
			out.ClosedAt = &bill.ClosedAt.Int64
		}
		tables, err := tx.ListBillTables(ctx, billID)
		if err != nil {
			return err
		}
		out.Tables = []string{}
		for _, t := range tables {
			out.Tables = append(out.Tables, t.Label)
		}
		if bill.Type == "takeaway" {
			out.Tables = append(out.Tables, "Bawa pulang · "+s.takeawayName(bill))
		}
		inShare := func(sub sql.NullString) bool { return subBillID == "" || sub.String == subBillID }
		if subBillID != "" {
			i := -1
			for j, sh := range bl.SubBills {
				if sh.ID == subBillID {
					i = j
				}
			}
			if i < 0 {
				return notFound("Sub-tagihan tidak ditemukan")
			}
			sh := bl.SubBills[i]
			out.SubBill = strconv.FormatInt(sh.Index, 10) + "/" + strconv.FormatInt(sh.Count, 10)
			out.Totals, out.Balance = sh.Totals, sh.Balance
			if sh.Method == "equal" {
				inShare = func(sql.NullString) bool { return true } // every item, a fraction of the total
			}
		}
		for _, pv := range bl.Payments {
			if subBillID == "" || pv.SubBillID == subBillID {
				out.Payments = append(out.Payments, pv)
			}
		}
		items, err := tx.ListOrderItemsByBill(ctx, billID)
		if err != nil {
			return err
		}
		mods, err := tx.ListOrderItemModifiersByBill(ctx, billID)
		if err != nil {
			return err
		}
		modsBy := map[string][]store.OrderItemModifier{}
		for _, m := range mods {
			modsBy[m.OrderItemID] = append(modsBy[m.OrderItemID], m)
		}
		for _, it := range items {
			if it.Status == ItemCancelled || it.Status == "voided" || !inShare(it.SubBillID) {
				continue
			}
			l := ReceiptLine{Quantity: it.Quantity, Name: it.ProductNameID, Modifiers: []string{}, Remake: it.RemakeOfID.Valid}
			if it.VariantNameID != "" {
				l.Variant = it.VariantNameID
			}
			unit := it.UnitPrice
			for _, m := range modsBy[it.ID] {
				l.Modifiers = append(l.Modifiers, m.NameID)
				unit += m.Price
			}
			l.Amount = unit * it.Quantity
			out.Lines = append(out.Lines, l)
		}
		if copy == "customer" {
			entries, err := tx.ListCustomerEntries(ctx, billID)
			if err != nil {
				return err
			}
			for _, e := range entries {
				out.Customers = append(out.Customers, s.name(e))
			}
		}
		prior, err := tx.CountReceiptPrints(ctx, store.CountReceiptPrintsParams{BillID: billID, Copy: copy, SubBillID: nullStr(subBillID), SubBillID_2: nullStr(subBillID)})
		if err != nil {
			return err
		}
		out.Reprint = prior > 0
		if err := tx.CreateReceiptPrint(ctx, store.CreateReceiptPrintParams{ID: platform.NewID(), BillID: billID, SubBillID: nullStr(subBillID),
			Copy: copy, IsReprint: boolInt(out.Reprint), PrintedBy: p.ID, PrintedAt: platform.Millis(now)}); err != nil {
			return err
		}
		if out.Reprint {
			return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "receipt.reprinted", Entity: "bill", EntityID: billID,
				After: map[string]string{"copy": copy, "subBill": subBillID}})
		}
		return nil
	})
	return out, err
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------- daily sales (BR-ADM-11)

type MethodTotal struct {
	Method  string `json:"method"`
	Count   int    `json:"count"`
	Amount  int64  `json:"amount"`
	Pending int64  `json:"pending"` // QRIS pending verification
}

type ProductSales struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Quantity  int64  `json:"quantity"`
	Gross     int64  `json:"gross"`
}

type DailySales struct {
	Date        string         `json:"date"`
	Bills       int            `json:"bills"`
	Totals      Totals         `json:"totals"`
	Rounding    int64          `json:"rounding"`
	UnpaidBills int            `json:"unpaidBills"`
	UnpaidTotal int64          `json:"unpaidTotal"`
	Methods     []MethodTotal  `json:"methods"`
	Products    []ProductSales `json:"products"`
}

// DailySales sums the Bills closed in one business day (04:00 to 04:00 WIB) from their frozen
// totals; Unpaid Bills are reported separately as a loss (BR-R-32).
func (s *Service) DailySales(ctx context.Context, date string) (DailySales, error) {
	day, err := time.ParseInLocation(time.DateOnly, date, platform.Jakarta)
	if err != nil {
		return DailySales{}, invalid("Tanggal tidak valid.")
	}
	from, to := day.Add(4*time.Hour), day.Add(28*time.Hour)
	q := s.db.Read()
	window := store.ListClosedBillsBetweenParams{ClosedAt: nullMs(platform.Millis(from)), ClosedAt_2: nullMs(platform.Millis(to))}
	bills, err := q.ListClosedBillsBetween(ctx, window)
	if err != nil {
		return DailySales{}, err
	}
	out := DailySales{Date: date, Methods: []MethodTotal{}, Products: []ProductSales{}}
	for _, b := range bills {
		if b.Status == "unpaid" {
			out.UnpaidBills++
			out.UnpaidTotal += b.Total.Int64
			continue
		}
		out.Bills++
		out.Totals.Subtotal += b.Subtotal.Int64
		out.Totals.ItemDiscount += b.ItemDiscount.Int64
		out.Totals.BillDiscount += b.BillDiscount.Int64
		out.Totals.Service += b.Service.Int64
		out.Totals.Tax += b.Tax.Int64
		out.Totals.Total += b.Total.Int64
	}
	payments, err := q.ListPaymentsBetween(ctx, store.ListPaymentsBetweenParams{ClosedAt: window.ClosedAt, ClosedAt_2: window.ClosedAt_2})
	if err != nil {
		return DailySales{}, err
	}
	by := map[string]*MethodTotal{}
	for _, m := range []string{PayCash, PayQRIS, PayCard} {
		by[m] = &MethodTotal{Method: m}
	}
	for _, p := range payments {
		m := by[p.Method]
		switch p.Status {
		case PayWrittenOff:
			continue
		case PayPending:
			m.Pending += p.Amount
		default:
			m.Amount += p.Amount
		}
		m.Count++
		out.Rounding += p.Rounding
	}
	for _, m := range []string{PayCash, PayQRIS, PayCard} {
		out.Methods = append(out.Methods, *by[m])
	}
	sold, err := q.ListSoldItemsBetween(ctx, store.ListSoldItemsBetweenParams{ClosedAt: window.ClosedAt, ClosedAt_2: window.ClosedAt_2})
	if err != nil {
		return DailySales{}, err
	}
	for i, r := range sold {
		if i == 10 {
			break
		}
		out.Products = append(out.Products, ProductSales{ProductID: r.ProductID, Name: r.ProductNameID, Quantity: int64(r.Quantity.Float64), Gross: int64(r.Gross.Float64)})
	}
	return out, nil
}
