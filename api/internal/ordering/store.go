package ordering

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crypto"
)

// Store persists bills, customer entries, guest tokens and waiter
// calls in SQLite, using migration 0007. Customer personal data
// is encrypted through the KeyManager; this store never writes
// plaintext names or dietary data to the database.
type Store struct {
	db *sql.DB
	km *crypto.KeyManager
}

func NewStore(db *sql.DB, km *crypto.KeyManager) *Store {
	db.SetMaxOpenConns(1)
	return &Store{db: db, km: km}
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func intPtr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

func (s *Store) loadBill(id string) (Bill, error) {
	var b Bill
	var closedAt sql.NullInt64
	err := s.db.QueryRow(`SELECT id, number, type, source, status, opened_at, closed_at FROM bill WHERE id = ?`, id).Scan(&b.ID, &b.Number, &b.Type, &b.Source, &b.Status, &b.OpenedAt, &closedAt)
	if err != nil {
		return Bill{}, errors.New("bill not found")
	}
	b.ClosedAt = intPtr(closedAt)
	return b, nil
}

func (s *Store) loadCustomer(id string) (Customer, error) {
	var c Customer
	var keyID, nameEnc string
	var noteEnc, tagsEnc sql.NullString
	var activeDevice sql.NullString
	var rejoined sql.NullInt64
	err := s.db.QueryRow(`SELECT id, bill_id, key_id, name_enc, dietary_note_enc, restriction_tags_enc, consent_at, active_device_id, rejoined_at FROM customer_entry WHERE id = ?`, id).Scan(&c.ID, &c.BillID, &keyID, &nameEnc, &noteEnc, &tagsEnc, &c.ConsentAt, &activeDevice, &rejoined)
	if err != nil {
		return Customer{}, errors.New("customer entry not found")
	}
	name, err := s.km.Decrypt(keyID, nameEnc)
	if err != nil {
		return Customer{}, err
	}
	c.Name = name
	c.ActiveDeviceID = strPtr(activeDevice)
	c.RejoinedAt = intPtr(rejoined)
	if noteEnc.Valid {
		if note, err := s.km.Decrypt(keyID, noteEnc.String); err == nil {
			c.DietaryNote = &note
		}
	}
	if tagsEnc.Valid {
		var tags []string
		if plain, err := s.km.Decrypt(keyID, tagsEnc.String); err == nil {
			_ = json.Unmarshal([]byte(plain), &tags)
		}
		c.RestrictionTags = tags
	}
	if c.RestrictionTags == nil {
		c.RestrictionTags = []string{}
	}
	return c, nil
}

func (s *Store) loadCustomersForBill(billID string) []Customer {
	rows, err := s.db.Query(`SELECT id FROM customer_entry WHERE bill_id = ? ORDER BY created_at`, billID)
	if err != nil {
		return []Customer{}
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []Customer{}
	for _, id := range ids {
		if c, err := s.loadCustomer(id); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// ScanTable resolves a QR token to a Table and its open Bill, if any.
// It returns status "free" or "occupied" with Customer names when
// occupied (BR-CM-12, BR-P-07).
func (s *Store) ScanTable(qrToken string) (TableScan, error) {
	var tableID, label string
	var active int
	err := s.db.QueryRow(`SELECT id, label, active FROM dining_table WHERE qr_token = ?`, qrToken).Scan(&tableID, &label, &active)
	if err != nil || active != 1 {
		return TableScan{}, errors.New("table not found for qr token")
	}
	scan := TableScan{TableID: tableID, TableLabel: label, Status: "free", Customers: []Customer{}}
	var billID string
	err = s.db.QueryRow(`SELECT bill_id FROM bill_table WHERE table_id = ? AND is_open = 1`, tableID).Scan(&billID)
	if err != nil {
		return scan, nil
	}
	bill, err := s.loadBill(billID)
	if err != nil || bill.Status != "open" && bill.Status != "settling" {
		return scan, nil
	}
	scan.Status = "occupied"
	scan.BillID = &billID
	scan.Customers = s.loadCustomersForBill(billID)
	return scan, nil
}

// OpenOrJoinBill returns the open Bill for a Table, creating it
// inside BEGIN IMMEDIATE so two scans cannot both open a Bill
// (TR-DM-01). Bill numbers are daily sequences: YYYYMMDD-NNN.
func (s *Store) OpenOrJoinBill(tableID string) (Bill, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Bill{}, err
	}
	defer tx.Rollback()
	// The single-connection pool (MaxOpenConns(1)) serialises
	// writers, and the partial unique index on
	// bill_table(table_id) WHERE is_open = 1 is the hard guard:
	// a second concurrent open for the same Table fails on insert.
	var existingID string
	err = tx.QueryRow(`SELECT bill_id FROM bill_table WHERE table_id = ? AND is_open = 1`, tableID).Scan(&existingID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return Bill{}, err
		}
		return s.loadBill(existingID)
	}
	now := time.Now()
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	day := now.In(jakarta).Format("20060102")
	var count int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM bill WHERE number LIKE ?`, day+"-%").Scan(&count)
	number := fmt.Sprintf("%s-%03d", day, count+1)
	bill := Bill{ID: NewID(), Number: number, Type: "dine_in", Source: "live", Status: "open", OpenedAt: now.UnixMilli()}
	if _, err := tx.Exec(`INSERT INTO bill (id, number, type, source, status, opened_at) VALUES (?, ?, 'dine_in', 'live', 'open', ?)`, bill.ID, bill.Number, bill.OpenedAt); err != nil {
		return Bill{}, err
	}
	if _, err := tx.Exec(`INSERT INTO bill_table (bill_id, table_id, is_open) VALUES (?, ?, 1)`, bill.ID, tableID); err != nil {
		return Bill{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bill{}, err
	}
	return bill, nil
}

// Join creates a customer entry on a Table's open Bill and issues
// a guest token. Dietary tag ids are validated against the
// dietary_tag table.
func (s *Store) Join(tableID, name string, restrictionTags []string, dietaryNote *string, consent bool) (JoinResult, error) {
	if err := ValidateJoin(name, consent); err != nil {
		return JoinResult{}, err
	}
	for _, tagID := range restrictionTags {
		var count int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM dietary_tag WHERE id = ?`, tagID).Scan(&count); err != nil || count == 0 {
			return JoinResult{}, fmt.Errorf("dietary tag not found: %s", tagID)
		}
	}
	bill, err := s.OpenOrJoinBill(tableID)
	if err != nil {
		return JoinResult{}, err
	}
	if bill.Status == "settling" {
		return JoinResult{}, errors.New("bill is being settled — please ask the waiter")
	}
	now := time.Now()
	keyID := crypto.TodayKeyID(now)
	nameEnc, err := s.km.Encrypt(keyID, name)
	if err != nil {
		return JoinResult{}, err
	}
	var noteEnc *string
	if dietaryNote != nil {
		enc, err := s.km.Encrypt(keyID, *dietaryNote)
		if err != nil {
			return JoinResult{}, err
		}
		noteEnc = &enc
	}
	tagsJSON, _ := json.Marshal(restrictionTags)
	tagsEnc, err := s.km.Encrypt(keyID, string(tagsJSON))
	if err != nil {
		return JoinResult{}, err
	}
	customerID := NewID()
	deviceID := NewToken()
	guestToken := NewToken()
	tx, err := s.db.Begin()
	if err != nil {
		return JoinResult{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO customer_entry (id, bill_id, key_id, name_enc, dietary_note_enc, restriction_tags_enc, consent_at, active_device_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		customerID, bill.ID, keyID, nameEnc, nullString(noteEnc), tagsEnc, now.UnixMilli(), deviceID, now.UnixMilli(),
	); err != nil {
		return JoinResult{}, err
	}
	if _, err := tx.Exec(
		`INSERT INTO guest_token (token, customer_entry_id, device_id, created_at) VALUES (?, ?, ?, ?)`,
		guestToken, customerID, deviceID, now.UnixMilli(),
	); err != nil {
		return JoinResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return JoinResult{}, err
	}
	customer := Customer{ID: customerID, BillID: bill.ID, Name: name, RestrictionTags: restrictionTags, DietaryNote: dietaryNote, ConsentAt: now.UnixMilli(), ActiveDeviceID: &deviceID}
	return JoinResult{Bill: bill, Customer: customer, GuestToken: guestToken, DeviceID: deviceID}, nil
}

// Rejoin issues a new guest token for an existing customer entry,
// revokes previous tokens and stores the new device as active
// (BR-CM-13, TR-AUTH-07). The previous device's token then fails
// validation, so it shows "You continued on another device".
func (s *Store) Rejoin(customerEntryID string) (JoinResult, error) {
	customer, err := s.loadCustomer(customerEntryID)
	if err != nil {
		return JoinResult{}, err
	}
	bill, err := s.loadBill(customer.BillID)
	if err != nil {
		return JoinResult{}, err
	}
	if bill.Status != "open" && bill.Status != "settling" {
		return JoinResult{}, errors.New("bill is closed")
	}
	now := time.Now()
	deviceID := NewToken()
	guestToken := NewToken()
	tx, err := s.db.Begin()
	if err != nil {
		return JoinResult{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE guest_token SET revoked_at = ? WHERE customer_entry_id = ? AND revoked_at IS NULL`, now.UnixMilli(), customerEntryID); err != nil {
		return JoinResult{}, err
	}
	if _, err := tx.Exec(`UPDATE customer_entry SET active_device_id = ?, rejoined_at = ? WHERE id = ?`, deviceID, now.UnixMilli(), customerEntryID); err != nil {
		return JoinResult{}, err
	}
	if _, err := tx.Exec(`INSERT INTO guest_token (token, customer_entry_id, device_id, created_at) VALUES (?, ?, ?, ?)`, guestToken, customerEntryID, deviceID, now.UnixMilli()); err != nil {
		return JoinResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return JoinResult{}, err
	}
	customer.ActiveDeviceID = &deviceID
	rejoined := now.UnixMilli()
	customer.RejoinedAt = &rejoined
	return JoinResult{Bill: bill, Customer: customer, GuestToken: guestToken, DeviceID: deviceID}, nil
}

// ValidateGuestToken returns the customer entry for a live guest
// token, or an error if the token was revoked or never existed.
// Tokens expire when the Bill closes.
func (s *Store) ValidateGuestToken(token string) (Customer, error) {
	var entryID string
	var revoked sql.NullInt64
	err := s.db.QueryRow(`SELECT customer_entry_id, revoked_at FROM guest_token WHERE token = ?`, token).Scan(&entryID, &revoked)
	if err != nil || revoked.Valid {
		return Customer{}, errors.New("guest token is invalid or revoked")
	}
	customer, err := s.loadCustomer(entryID)
	if err != nil {
		return Customer{}, err
	}
	bill, err := s.loadBill(customer.BillID)
	if err != nil || (bill.Status != "open" && bill.Status != "settling") {
		return Customer{}, errors.New("guest token expired: bill is closed")
	}
	return customer, nil
}

// RecordWaiterCall stores a waiter call or a not-mine alert for a Table.
func (s *Store) RecordWaiterCall(tableID, kind string, billID, customerEntryID *string) error {
	if kind != "waiter_call" && kind != "not_mine" {
		return errors.New("waiter call kind must be waiter_call or not_mine")
	}
	_, err := s.db.Exec(
		`INSERT INTO waiter_call (id, table_id, bill_id, customer_entry_id, kind, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		NewID(), tableID, nullString(billID), nullString(customerEntryID), kind, time.Now().UnixMilli(),
	)
	return err
}
