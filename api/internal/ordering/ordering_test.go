package ordering

import (
	"database/sql"
	"errors"
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
CREATE TABLE product (id TEXT PRIMARY KEY, category_id TEXT, name_id TEXT, name_en TEXT, description_id TEXT, description_en TEXT, image_key TEXT, image_focus TEXT, prep_time_seconds INTEGER, status TEXT, sort INTEGER, created_at INTEGER, updated_at INTEGER, archived_at INTEGER);
CREATE TABLE variant (id TEXT PRIMARY KEY, product_id TEXT, name_id TEXT, name_en TEXT, price INTEGER, is_default INTEGER, sort INTEGER, sold_out INTEGER, sold_out_at INTEGER, sold_out_by TEXT, sold_out_source TEXT, archived_at INTEGER);
CREATE TABLE modifier_group (id TEXT PRIMARY KEY, product_id TEXT, name_id TEXT, name_en TEXT, min_select INTEGER, max_select INTEGER, sort INTEGER);
CREATE TABLE modifier (id TEXT PRIMARY KEY, modifier_group_id TEXT, name_id TEXT, name_en TEXT, price INTEGER, sort INTEGER, archived_at INTEGER);
CREATE TABLE product_dietary_tag (product_id TEXT, dietary_tag_id TEXT, PRIMARY KEY (product_id, dietary_tag_id));
CREATE TABLE modifier_dietary_tag (modifier_id TEXT, dietary_tag_id TEXT, PRIMARY KEY (modifier_id, dietary_tag_id));
CREATE TABLE orders (id TEXT PRIMARY KEY, bill_id TEXT NOT NULL, customer_entry_id TEXT, source TEXT NOT NULL, status TEXT NOT NULL, claimed_by TEXT, claimed_at INTEGER, confirmed_at INTEGER, key_id TEXT, restriction_tags_enc TEXT, created_at INTEGER NOT NULL);
CREATE TABLE order_item (id TEXT PRIMARY KEY, order_id TEXT NOT NULL, product_id TEXT NOT NULL, variant_id TEXT NOT NULL, product_name_snapshot TEXT NOT NULL, variant_name_snapshot TEXT NOT NULL, quantity INTEGER NOT NULL, unit_price INTEGER NOT NULL, note TEXT, status TEXT NOT NULL, cancel_reason TEXT, cancel_note TEXT, cancelled_by TEXT, void_reason TEXT, voided_by TEXT, voided_override_by TEXT, remake_of_id TEXT, remake_reason TEXT, remake_override_by TEXT, ready_at INTEGER, sub_bill_id TEXT, sold_out_flag INTEGER NOT NULL DEFAULT 0, unavailable_flag INTEGER NOT NULL DEFAULT 0);
CREATE TABLE order_item_modifier (order_item_id TEXT NOT NULL, modifier_id TEXT NOT NULL, name_snapshot TEXT NOT NULL, price INTEGER NOT NULL, PRIMARY KEY (order_item_id, modifier_id));
CREATE TABLE order_item_dietary_warning (order_item_id TEXT NOT NULL, dietary_tag_id TEXT NOT NULL, acknowledged_at INTEGER, PRIMARY KEY (order_item_id, dietary_tag_id));
INSERT INTO dining_table VALUES ('t1','T1','qr-abc',1,0,0);
INSERT INTO dietary_tag VALUES ('tag-fish','fish','Ikan','Fish',NULL,1,1);
INSERT INTO product VALUES ('p1','c1','Salmon Nigiri','Salmon Nigiri','','',NULL,'50,50',300,'active',1,0,0,NULL);
INSERT INTO variant VALUES ('v1','p1','Reguler','Regular',45000,1,1,0,NULL,NULL,NULL,NULL);
INSERT INTO variant VALUES ('v2','p1','Besar','Large',60000,0,2,1,NULL,NULL,NULL,NULL);
INSERT INTO modifier_group VALUES ('g1','p1','Saus','Sauce',0,2,1);
INSERT INTO modifier VALUES ('m1','g1','Saus Pedas','Spicy Sauce',5000,1,NULL);
INSERT INTO product_dietary_tag VALUES ('p1','tag-fish');
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

func TestCreateOrderWarningsEditCancel(t *testing.T) {
	s := testStore(t)
	joined, err := s.Join("t1", "Andi", []string{"tag-fish"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	customer, err := s.ValidateGuestToken(joined.GuestToken)
	if err != nil {
		t.Fatal(err)
	}
	// Sold out variant cannot be ordered.
	if _, err := s.CreateOrder(customer, CreateOrderInput{Items: []OrderItemInput{{ProductID: "p1", VariantID: "v2", Quantity: 1}}, AcknowledgeWarnings: true}); err == nil {
		t.Fatal("sold out variant should be rejected")
	}
	// Warning without acknowledgement returns WarningError, no order.
	if _, err := s.CreateOrder(customer, CreateOrderInput{Items: []OrderItemInput{{ProductID: "p1", VariantID: "v1", Quantity: 2, ModifierIDs: []string{"m1"}}}}); err == nil {
		t.Fatal("unacknowledged warning should fail")
	} else {
		var we *WarningError
		if !errors.As(err, &we) {
			t.Fatalf("expected WarningError, got %v", err)
		}
	}
	// With acknowledgement, order is pending, prices snapshotted, wait = 5 min.
	order, err := s.CreateOrder(customer, CreateOrderInput{Items: []OrderItemInput{{ProductID: "p1", VariantID: "v1", Quantity: 2, ModifierIDs: []string{"m1"}}}, AcknowledgeWarnings: true})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "pending" || order.PlacedBy != "Andi" || len(order.Items) != 1 {
		t.Fatalf("order = %+v", order)
	}
	it := order.Items[0]
	if it.UnitPrice != 45000 || it.LineTotal != 100000 || len(it.DietaryWarnings) != 1 || !it.DietaryWarnings[0].Acknowledged {
		t.Fatalf("item = %+v", it)
	}
	if order.EstimatedWaitMinutes != 5 {
		t.Fatalf("estimated wait = %d, want 5", order.EstimatedWaitMinutes)
	}
	// Edit before claim replaces items.
	edited, err := s.PatchOrder(customer, order.ID, CreateOrderInput{Items: []OrderItemInput{{ProductID: "p1", VariantID: "v1", Quantity: 1}}, AcknowledgeWarnings: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(edited.Items) != 1 || edited.Items[0].Quantity != 1 {
		t.Fatalf("edited = %+v", edited)
	}
	// Another customer cannot edit or cancel it.
	joined2, _ := s.Join("t1", "Budi", nil, nil, true)
	customer2, _ := s.ValidateGuestToken(joined2.GuestToken)
	if _, err := s.PatchOrder(customer2, order.ID, CreateOrderInput{Items: []OrderItemInput{{ProductID: "p1", VariantID: "v1", Quantity: 1}}, AcknowledgeWarnings: true}); err == nil {
		t.Fatal("non-owner edit should fail")
	}
	// Owner cancels.
	cancelled, err := s.CancelOrder(customer, order.ID, "customer_request")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" || cancelled.Items[0].Status != "cancelled" {
		t.Fatalf("cancelled = %+v", cancelled)
	}
}
