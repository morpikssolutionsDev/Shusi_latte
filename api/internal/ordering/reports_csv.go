package ordering

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
)

// CSV exports of the reports (TRD §6.1 Reports): the same figures as the JSON, one section per
// table, Rupiah as plain integers and times in WIB, for a spreadsheet.

type section struct {
	title  string
	header []string
	rows   [][]string
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func clockWIB(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).In(platform.Jakarta).Format("2006-01-02 15:04")
}

// respond writes v as JSON, or its sections as CSV when ?format=csv.
func respond(w http.ResponseWriter, r *http.Request, name, date string, v any, sections func() []section) error {
	if r.URL.Query().Get("format") != "csv" {
		httpx.WriteJSON(w, http.StatusOK, v)
		return nil
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.csv"`, name, date))
	cw := csv.NewWriter(w)
	for i, s := range sections() {
		if i > 0 {
			_ = cw.Write(nil)
		}
		_ = cw.Write([]string{s.title})
		_ = cw.Write(s.header)
		_ = cw.WriteAll(s.rows)
	}
	cw.Flush()
	return cw.Error()
}

func (d DailySales) sections() []section {
	t := d.Totals
	out := []section{{"Ringkasan", []string{"date", "bills", "subtotal", "item_discount", "bill_discount", "service", "tax", "total", "rounding", "unpaid_bills", "unpaid_total"},
		[][]string{{d.Date, strconv.Itoa(d.Bills), itoa(t.Subtotal), itoa(t.ItemDiscount), itoa(t.BillDiscount), itoa(t.Service), itoa(t.Tax), itoa(t.Total),
			itoa(d.Rounding), strconv.Itoa(d.UnpaidBills), itoa(d.UnpaidTotal)}}}}
	methods := section{title: "Metode pembayaran", header: []string{"method", "count", "amount", "pending"}}
	for _, m := range d.Methods {
		methods.rows = append(methods.rows, []string{m.Method, strconv.Itoa(m.Count), itoa(m.Amount), itoa(m.Pending)})
	}
	products := section{title: "Terlaris", header: []string{"product", "quantity", "gross"}}
	for _, p := range d.Products {
		products.rows = append(products.rows, []string{p.Name, itoa(p.Quantity), itoa(p.Gross)})
	}
	return append(out, methods, products)
}

func (p PrepReport) sections() []section {
	row := func(r PrepRow) []string {
		return []string{r.Name, strconv.Itoa(r.Items), strconv.Itoa(r.OnTime), strconv.FormatFloat(r.Share, 'f', 3, 64)}
	}
	header := []string{"name", "items", "on_time", "share"}
	areas := section{title: "Prep Area", header: header, rows: [][]string{{"Semua", strconv.Itoa(p.Items), strconv.Itoa(p.OnTime), strconv.FormatFloat(p.Share, 'f', 3, 64)}}}
	for _, r := range p.PrepAreas {
		areas.rows = append(areas.rows, row(r))
	}
	products := section{title: "Produk", header: header}
	for _, r := range p.Products {
		products.rows = append(products.rows, row(r))
	}
	return []section{areas, products}
}

func (o Operations) sections() []section {
	shifts := section{title: "Tutup shift", header: []string{"cashier", "opened_at", "closed_at", "opening_float", "expected", "counted", "difference"}}
	for _, s := range o.ShiftClosings {
		v := func(p *int64) string {
			if p == nil {
				return ""
			}
			return itoa(*p)
		}
		closed := ""
		if s.ClosedAt != nil {
			closed = clockWIB(*s.ClosedAt)
		}
		shifts.rows = append(shifts.rows, []string{s.Cashier, clockWIB(s.OpenedAt), closed, itoa(s.OpeningFloat), v(s.Expected), v(s.Counted), v(s.Difference)})
	}
	bills := func(title string, rows []BillRow) section {
		sec := section{title: title, header: []string{"bill", "tables", "total", "closed_at", "reason", "by"}}
		for _, b := range rows {
			sec.rows = append(sec.rows, []string{b.Number, strings.Join(b.Tables, " + "), itoa(b.Total), clockWIB(b.ClosedAt), b.Reason, b.By})
		}
		return sec
	}
	remakes := section{title: "Remake", header: []string{"bill", "item", "reason", "approved_by", "at"}}
	for _, r := range o.Remakes {
		remakes.rows = append(remakes.rows, []string{r.BillNumber, r.Item, r.Reason, r.ApprovedBy, clockWIB(r.At)})
	}
	refunds := section{title: "Refund", header: []string{"bill", "amount", "reason", "approved_by", "at"}}
	for _, r := range o.Refunds {
		refunds.rows = append(refunds.rows, []string{r.BillNumber, itoa(r.Amount), r.Reason, r.ApprovedBy, clockWIB(r.CreatedAt)})
	}
	return []section{shifts, bills("Tagihan tidak dibayar", o.Unpaid), remakes, refunds, bills("Input pasca-gangguan", o.PostOutage)}
}

func historySections(rows []HistoryRow) []section {
	sec := section{title: "Tagihan ditutup", header: []string{"bill", "tables", "customers", "status", "source", "total", "closed_at", "anonymised"}}
	for _, b := range rows {
		sec.rows = append(sec.rows, []string{b.Number, strings.Join(b.Tables, " + "), strings.Join(b.Customers, ", "), b.Status, b.Source, itoa(b.Total),
			clockWIB(b.ClosedAt), strconv.FormatBool(b.Anonymised)})
	}
	return []section{sec}
}
