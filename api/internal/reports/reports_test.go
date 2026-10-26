package reports

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	schema := `
CREATE TABLE bill (id TEXT PRIMARY KEY, number TEXT, type TEXT, source TEXT, status TEXT, unpaid_reason TEXT, opened_at INTEGER, closed_at INTEGER);
CREATE TABLE payment (id TEXT PRIMARY KEY, bill_id TEXT, method TEXT, status TEXT, amount INTEGER, taken_at INTEGER);
CREATE TABLE orders (id TEXT PRIMARY KEY, bill_id TEXT, confirmed_at INTEGER);
CREATE TABLE order_item (id TEXT PRIMARY KEY, order_id TEXT, product_id TEXT, product_name_snapshot TEXT, quantity INTEGER, unit_price INTEGER, status TEXT, remake_of_id TEXT, remake_reason TEXT, ready_at INTEGER);
CREATE TABLE product (id TEXT PRIMARY KEY, category_id TEXT, name_id TEXT, prep_time_seconds INTEGER);
CREATE TABLE category (id TEXT PRIMARY KEY, prep_area_id TEXT);
CREATE TABLE drawer_session (id TEXT PRIMARY KEY, cashier_crew_id TEXT, opening_float INTEGER, expected_cash INTEGER, counted_cash INTEGER, difference INTEGER, opened_at INTEGER, closed_at INTEGER);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func ms(date string) int64 {
	t, _ := time.ParseInLocation("2006-01-02 15:04", date, jakarta)
	return t.UnixMilli()
}

func TestReportsWithSampleData(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	// One Exec per statement: the SQLite driver does not distribute
	// arguments across multiple statements in a single Exec call.
	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO bill VALUES ('b1','1','dine_in','live','closed',NULL,?,?)`, []any{ms("2026-10-26 10:00"), ms("2026-10-26 12:00")}},
		{`INSERT INTO payment VALUES ('p1','b1','cash','settled',100000,?)`, []any{ms("2026-10-26 12:00")}},
		{`INSERT INTO payment VALUES ('p2','b1','qris','settled',50000,?)`, []any{ms("2026-10-26 12:00")}},
		{`INSERT INTO orders VALUES ('o1','b1',?)`, []any{ms("2026-10-26 11:00")}},
		{`INSERT INTO order_item VALUES ('i1','o1','prod1','Salmon Nigiri',2,45000,'served',NULL,NULL,?)`, []any{ms("2026-10-26 11:05")}},
		{`INSERT INTO order_item VALUES ('i2','o1','prod1','Salmon Nigiri',1,0,'served','i1','wrong order',?)`, []any{ms("2026-10-26 11:10")}},
		{`INSERT INTO product VALUES ('prod1','cat1','Salmon Nigiri',300)`, nil},
		{`INSERT INTO category VALUES ('cat1','prep-kitchen')`, nil},
		{`INSERT INTO bill VALUES ('b2','2','dine_in','live','unpaid','customer left',?,?)`, []any{ms("2026-10-26 13:00"), ms("2026-10-26 13:30")}},
		{`INSERT INTO drawer_session VALUES ('d1','C01',200000,350000,350000,0,?,?)`, []any{ms("2026-10-26 08:00"), ms("2026-10-26 20:00")}},
	}
	for _, st := range stmts {
		if _, err := db.Exec(st.sql, st.args...); err != nil {
			t.Fatal(err)
		}
	}
	s := NewStore(db)
	rng, err := ParseRange("2026-10-26", "2026-10-26")
	if err != nil {
		t.Fatal(err)
	}
	daily := s.DailySales(rng)
	if len(daily) != 1 || daily[0].PaymentsTotal != 150000 || daily[0].BillsClosed != 1 {
		t.Fatalf("daily sales = %+v", daily)
	}
	methods := s.PaymentMethods(rng)
	if len(methods) != 2 {
		t.Fatalf("payment methods = %+v", methods)
	}
	best := s.BestSellers(rng)
	if len(best) != 1 || best[0].Quantity != 3 || best[0].Amount != 90000 {
		t.Fatalf("best sellers = %+v", best)
	}
	prep := s.PrepTime(rng)
	// i1 took 300s (on time for a 300s Prep Time), i2 took 600s (late).
	if len(prep) != 1 || prep[0].ItemsReady != 2 || prep[0].ItemsOnTime != 1 || prep[0].OnTimePct != 50 {
		t.Fatalf("prep time = %+v", prep)
	}
	unpaid := s.UnpaidBills(rng)
	if len(unpaid) != 1 || unpaid[0].UnpaidReason != "customer left" {
		t.Fatalf("unpaid bills = %+v", unpaid)
	}
	remakes := s.Remakes(rng)
	if len(remakes) != 1 || remakes[0].Reason != "wrong order" || remakes[0].Count != 1 {
		t.Fatalf("remakes = %+v", remakes)
	}
	closings := s.ShiftClosings(rng)
	if len(closings) != 1 || closings[0].Difference != 0 {
		t.Fatalf("shift closings = %+v", closings)
	}
	if len(s.PostOutage(rng)) != 0 {
		t.Fatal("post-outage should be empty for live bills")
	}
}

func TestReportsWithoutSourceTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	s := NewStore(db)
	rng, _ := ParseRange("2026-10-26", "2026-10-26")
	if len(s.DailySales(rng)) != 0 || len(s.BestSellers(rng)) != 0 || len(s.UnpaidBills(rng)) != 0 {
		t.Fatal("reports without source tables should return empty rows, not errors")
	}
}

func TestParseRange(t *testing.T) {
	rng, err := ParseRange("2026-10-26", "2026-10-27")
	if err != nil || rng.FromMS >= rng.ToMS {
		t.Fatalf("range = %+v, err = %v", rng, err)
	}
	if _, err := ParseRange("2026-10-27", "2026-10-26"); err == nil {
		t.Fatal("reversed range should fail")
	}
}
