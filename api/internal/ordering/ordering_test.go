package ordering

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crypto"
	_ "modernc.org/sqlite"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	schema := `
CREATE TABLE dining_table (id TEXT PRIMARY KEY, label TEXT NOT NULL UNIQUE, qr_token TEXT NOT NULL UNIQUE, active INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE bill (id TEXT PRIMARY KEY, number TEXT NOT NULL UNIQUE, type TEXT NOT NULL, source TEXT NOT NULL, status TEXT NOT NULL, takeaway_name TEXT, pay_now_serve_later_by TEXT, unpaid_reason TEXT, unpaid_override_by TEXT, opened_at INTEGER NOT NULL, closed_at INTEGER, anonymised_at INTEGER);
CREATE TABLE bill_table (bill_id TEXT NOT NULL, table_id TEXT NOT NULL, is_open INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (bill_id, table_id));
CREATE UNIQUE INDEX idx_bill_table_one_open ON bill_table(table_id) WHERE is_open = 1;
CREATE TABLE customer_entry (id TEXT PRIMARY KEY, bill_id TEXT NOT NULL, key_id TEXT NOT NULL, name_enc TEXT NOT NULL, dietary_note_enc TEXT, restriction_tags_enc TEXT, consent_at INTEGER NOT NULL, active_device_id TEXT, rejoined_at INTEGER, created_at INTEGER NOT NULL);
CREATE TABLE guest_token (token TEXT PRIMARY KEY, customer_entry_id TEXT NOT NULL, device_id TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER, revoked_at INTEGER);
CREATE TABLE waiter_call (id TEXT PRIMARY KEY, table_id TEXT NOT NULL, bill_id TEXT, customer_entry_id TEXT, kind TEXT NOT NULL, created_at INTEGER NOT NULL, acknowledged_at INTEGER);
CREATE TABLE dietary_tag (id TEXT PRIMARY KEY, code TEXT, name_id TEXT, name_en TEXT, icon TEXT, sort INTEGER, active INTEGER);
INSERT INTO dining_table VALUES ('t1','T1','qr-abc',1,0,0);
INSERT INTO dietary_tag VALUES ('tag-fish','fish','Ikan','Fish',NULL,1,1);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	km, err := crypto.NewKeyManager(filepath.Join(t.TempDir(), "keys.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(db, km)
}

func TestJoinScanRejoin(t *testing.T) {
	s := testStore(t)
	scan, err := s.ScanTable("qr-abc")
	if err != nil || scan.Status != "free" {
		t.Fatalf("scan free = %+v, err = %v", scan, err)
	}
	if _, err := s.Join("t1", "", nil, nil, true); err == nil {
		t.Fatal("join without name should fail")
	}
	if _, err := s.Join("t1", "Andi", nil, nil, false); err == nil {
		t.Fatal("join without consent should fail")
	}
	note := "No spicy"
	joined, err := s.Join("t1", "Andi", []string{"tag-fish"}, &note, true)
	if err != nil {
		t.Fatal(err)
	}
	if joined.Customer.Name != "Andi" || joined.GuestToken == "" {
		t.Fatalf("joined = %+v", joined)
	}
	// Second join on same Table joins the same Bill.
	joined2, err := s.Join("t1", "Budi", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if joined2.Bill.ID != joined.Bill.ID {
		t.Fatalf("second customer should join same bill, got %s vs %s", joined2.Bill.ID, joined.Bill.ID)
	}
	scan, err = s.ScanTable("qr-abc")
	if err != nil || scan.Status != "occupied" || len(scan.Customers) != 2 {
		t.Fatalf("scan occupied = %+v, err = %v", scan, err)
	}
	// Rejoin revokes the old token.
	rejoined, err := s.Rejoin(joined.Customer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateGuestToken(joined.GuestToken); err == nil {
		t.Fatal("old guest token should be revoked after rejoin")
	}
	if _, err := s.ValidateGuestToken(rejoined.GuestToken); err != nil {
		t.Fatalf("new guest token should validate, err = %v", err)
	}
	// Plaintext must not be in the database.
	var nameEnc string
	if err := s.db.QueryRow(`SELECT name_enc FROM customer_entry WHERE id = ?`, joined.Customer.ID).Scan(&nameEnc); err != nil {
		t.Fatal(err)
	}
	if nameEnc == "Andi" {
		t.Fatal("name_enc should be ciphertext, not plaintext")
	}
}
