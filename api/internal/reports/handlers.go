package reports

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Handler wires the eight report endpoints from TRD §6.1.
// Every endpoint accepts from/to as YYYY-MM-DD (Asia/Jakarta) and
// format=csv for CSV export; otherwise it returns JSON.
type Handler struct {
	Store *Store
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, code, title string) {
	writeJSON(w, status, map[string]any{
		"type": "about:blank", "title": title, "status": status, "code": code,
	})
}

func writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	cw := csv.NewWriter(w)
	_ = cw.Write(header)
	_ = cw.WriteAll(rows)
	cw.Flush()
}

type reportFunc struct {
	name    string
	headers []string
	run     func(r Range) (any, [][]string)
}

func (h *Handler) Register(mux *http.ServeMux) {
	defs := []reportFunc{
		{"daily-sales", []string{"date", "bills_closed", "payments_total", "cash_total", "qris_total", "card_total"}, func(r Range) (any, [][]string) {
			rows := h.Store.DailySales(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.Date, strconv.Itoa(row.BillsClosed), strconv.FormatInt(row.PaymentsTotal, 10), strconv.FormatInt(row.CashTotal, 10), strconv.FormatInt(row.QRISTotal, 10), strconv.FormatInt(row.CardTotal, 10)})
			}
			return rows, out
		}},
		{"payment-methods", []string{"method", "payments", "total"}, func(r Range) (any, [][]string) {
			rows := h.Store.PaymentMethods(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.Method, strconv.Itoa(row.Payments), strconv.FormatInt(row.Total, 10)})
			}
			return rows, out
		}},
		{"best-sellers", []string{"product_id", "product_name", "category_id", "quantity", "amount"}, func(r Range) (any, [][]string) {
			rows := h.Store.BestSellers(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.ProductID, row.ProductName, row.CategoryID, strconv.Itoa(row.Quantity), strconv.FormatInt(row.Amount, 10)})
			}
			return rows, out
		}},
		{"prep-time", []string{"product_id", "product_name", "category_id", "prep_area_id", "items_ready", "items_on_time", "on_time_pct", "avg_prep_seconds"}, func(r Range) (any, [][]string) {
			rows := h.Store.PrepTime(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.ProductID, row.ProductName, row.CategoryID, row.PrepAreaID, strconv.Itoa(row.ItemsReady), strconv.Itoa(row.ItemsOnTime), strconv.FormatFloat(row.OnTimePct, 'f', 2, 64), strconv.FormatFloat(row.AvgPrepSec, 'f', 2, 64)})
			}
			return rows, out
		}},
		{"shift-closings", []string{"id", "cashier_crew_id", "opening_float", "expected_cash", "counted_cash", "difference", "opened_at", "closed_at"}, func(r Range) (any, [][]string) {
			rows := h.Store.ShiftClosings(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.ID, row.CashierCrew, strconv.FormatInt(row.OpeningFloat, 10), strconv.FormatInt(row.ExpectedCash, 10), strconv.FormatInt(row.CountedCash, 10), strconv.FormatInt(row.Difference, 10), strconv.FormatInt(row.OpenedAt, 10), strconv.FormatInt(row.ClosedAt, 10)})
			}
			return rows, out
		}},
		{"unpaid-bills", []string{"id", "number", "type", "unpaid_reason", "opened_at", "closed_at", "items", "amount"}, func(r Range) (any, [][]string) {
			rows := h.Store.UnpaidBills(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.ID, row.Number, row.Type, row.UnpaidReason, strconv.FormatInt(row.OpenedAt, 10), strconv.FormatInt(row.ClosedAt, 10), strconv.Itoa(row.Items), strconv.FormatInt(row.Amount, 10)})
			}
			return rows, out
		}},
		{"remakes", []string{"reason", "count"}, func(r Range) (any, [][]string) {
			rows := h.Store.Remakes(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.Reason, strconv.Itoa(row.Count)})
			}
			return rows, out
		}},
		{"post-outage", []string{"id", "number", "type", "opened_at", "closed_at", "items", "payments_total"}, func(r Range) (any, [][]string) {
			rows := h.Store.PostOutage(r)
			out := [][]string{}
			for _, row := range rows {
				out = append(out, []string{row.ID, row.Number, row.Type, strconv.FormatInt(row.OpenedAt, 10), strconv.FormatInt(row.ClosedAt, 10), strconv.Itoa(row.Items), strconv.FormatInt(row.PaymentsTotal, 10)})
			}
			return rows, out
		}},
	}
	for _, def := range defs {
		def := def
		mux.HandleFunc("GET /api/v1/reports/"+def.name, func(w http.ResponseWriter, r *http.Request) {
			rng, err := ParseRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
			if err != nil {
				writeProblem(w, http.StatusBadRequest, "invalid_date_range", err.Error())
				return
			}
			data, csvRows := def.run(rng)
			if r.URL.Query().Get("format") == "csv" {
				writeCSV(w, def.name+".csv", def.headers, csvRows)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"from_date": rng.FromDate, "to_date": rng.ToDate, "data": data,
			})
		})
	}
}
