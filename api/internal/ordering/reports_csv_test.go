package ordering

import (
	"encoding/csv"
	"net/http/httptest"
	"strings"
	"testing"
)

// TRD §6.1 Reports: ?format=csv returns the same figures as sections a spreadsheet can read.
func TestReportCSV(t *testing.T) {
	d := DailySales{Date: "2026-10-27", Bills: 2, Totals: Totals{Subtotal: 100000, Total: 115500}, Methods: []MethodTotal{{Method: "cash", Count: 2, Amount: 115500}},
		Products: []ProductSales{{Name: "Kyoto Cold Brew", Quantity: 3, Gross: 126000}}}
	w := httptest.NewRecorder()
	if err := respond(w, httptest.NewRequest("GET", "/reports/daily-sales?date=2026-10-27&format=csv", nil), "penjualan", d.Date, d, d.sections); err != nil {
		t.Fatal(err)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") || !strings.Contains(w.Header().Get("Content-Disposition"), "penjualan-2026-10-27.csv") {
		t.Fatalf("headers: %v", w.Header())
	}
	r := csv.NewReader(strings.NewReader(w.Body.String()))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[2][7] != "115500" || rows[5][0] != "cash" || rows[8][0] != "Kyoto Cold Brew" { // blank lines between sections are skipped
		t.Fatalf("csv: %q", rows)
	}
	w = httptest.NewRecorder()
	_ = respond(w, httptest.NewRequest("GET", "/reports/daily-sales?date=2026-10-27", nil), "penjualan", d.Date, d, d.sections)
	if !strings.HasPrefix(w.Body.String(), "{") {
		t.Fatal("without format the report stays JSON")
	}
}
