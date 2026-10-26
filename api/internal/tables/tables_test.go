package tables

import "testing"

func TestQRURL(t *testing.T) {
	tbl := Table{Slug: "table3", QRToken: "abc123"}
	if got := QRURL("", tbl); got != "/order/table3" {
		t.Fatalf("relative QR URL = %s", got)
	}
	if got := QRURL("https://sushi.latte/", tbl); got != "https://sushi.latte/order/table3" {
		t.Fatalf("absolute QR URL = %s", got)
	}
	// No slug yet (pre-slug Table): fall back to the token path.
	if got := QRURL("", Table{QRToken: "abc123"}); got != "/public/tables/abc123" {
		t.Fatalf("fallback QR URL = %s", got)
	}
}

func TestSlugify(t *testing.T) {
	if got := Slugify("Table 3"); got != "table3" {
		t.Fatalf("Table 3 -> %s", got)
	}
	if got := Slugify("T1"); got != "t1" {
		t.Fatalf("T1 -> %s", got)
	}
}

func TestNewTokenUnique(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) != 64 {
		t.Fatalf("tokens should be unique 64-char hex, got %s and %s", a, b)
	}
}

func TestValidateLabel(t *testing.T) {
	if err := ValidateLabel(""); err == nil {
		t.Fatal("empty label should fail")
	}
	if err := ValidateLabel("T1"); err != nil {
		t.Fatal(err)
	}
}
