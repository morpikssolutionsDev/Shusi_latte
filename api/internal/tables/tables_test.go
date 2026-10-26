package tables

import "testing"

func TestQRURL(t *testing.T) {
	tbl := Table{QRToken: "abc123"}
	if got := QRURL("", tbl); got != "/public/tables/abc123" {
		t.Fatalf("relative QR URL = %s", got)
	}
	if got := QRURL("https://demo.example/", tbl); got != "https://demo.example/public/tables/abc123" {
		t.Fatalf("absolute QR URL = %s", got)
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
