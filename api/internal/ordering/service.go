// Package ordering owns Tables in use, Bills, Customers, Orders and Order Items (TRD §4.3).
//
// Phase 3 covers the Customer side: scanning a Table QR opens or joins its single open Bill
// (OP-01, BE-02), Customers are bound to the Bill through guest tokens (BR-CM-13), and they
// place, edit and cancel Pending Orders (OP-02, BE-03). Claim and confirmation arrive with
// the Waiter module.
package ordering

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/vault"
)

// Realtime topics (TR-RT-02).
const WaitersTopic = "waiters"

func BillTopic(id string) string     { return "bill:" + id }
func CustomerTopic(id string) string { return "customer:" + id }

const (
	BillOpen     = "open"
	BillSettling = "settling"
	BillClosed   = "closed"

	maxName = 40
	maxNote = 300
)

type Service struct {
	db     *dbx.DB
	clock  platform.Clock
	vault  *vault.Vault
	outlet *outlet.Service
}

func NewService(db *dbx.DB, clock platform.Clock, v *vault.Vault) *Service {
	return &Service{db: db, clock: clock, vault: v, outlet: outlet.NewService(db, clock)}
}

// WithClock returns a copy acting at another time (the seed places Orders at offsets from T0).
func (s *Service) WithClock(clock platform.Clock) *Service {
	c := *s
	c.clock = clock
	c.outlet = outlet.NewService(s.db, clock)
	return &c
}

func problem(status int, code, title string) *httpx.Problem {
	return httpx.NewProblem(status, code, title)
}

func invalid(detail string) error {
	p := problem(http.StatusUnprocessableEntity, "invalid_input", "Data belum lengkap")
	p.Detail = detail
	return p
}

// BusinessDate is the WIB date a business day belongs to; the day turns over at 04:00 so a
// late-night Bill keeps its evening's numbering.
func BusinessDate(t time.Time) string {
	return t.In(platform.Jakarta).Add(-4 * time.Hour).Format(time.DateOnly)
}

// BillNumber formats the daily sequence for people: B-0427.
func BillNumber(n int64) string { return fmt.Sprintf("B-%04d", n) }

// ---------------------------------------------------------------- guests

// Guest is the Customer behind a guest token.
type Guest struct {
	CustomerID string
	BillID     string
	BillStatus string
}

// Authenticate resolves a guest token. A revoked token means the Customer Rejoined on another
// device (BR-CM-13); a closed Bill ends every token for it (TR-AUTH-07).
func (s *Service) Authenticate(ctx context.Context, token string) (*Guest, error) {
	row, err := s.db.Read().GetGuestByTokenHash(ctx, platform.Digest(token))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, problem(http.StatusUnauthorized, "guest_invalid", "Pindai QR meja untuk mulai memesan")
	}
	if err != nil {
		return nil, err
	}
	if row.RevokedAt.Valid {
		return nil, problem(http.StatusUnauthorized, "continued_elsewhere", "Kamu melanjutkan di perangkat lain.")
	}
	if row.BillStatus != BillOpen && row.BillStatus != BillSettling {
		return nil, problem(http.StatusUnauthorized, "bill_closed", "Tagihan ini sudah ditutup. Pindai QR meja untuk pesan lagi.")
	}
	return &Guest{CustomerID: row.CustomerEntryID, BillID: row.BillID, BillStatus: row.BillStatus}, nil
}

// ---------------------------------------------------------------- tables

type TableRef struct {
	Number int64  `json:"number"`
	Label  string `json:"label"`
}

type CustomerRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TableState is what a scan shows (OP-01): free, occupied (with the full names, BR-CM-12,
// BR-P-07) or unavailable (QR regenerated or Table inactive).
type TableState struct {
	State     string        `json:"state"`
	Table     *TableRef     `json:"table,omitempty"`
	Settling  bool          `json:"settling"`
	Customers []CustomerRef `json:"customers"`
}

func unavailable() error {
	return problem(http.StatusNotFound, "table_unavailable", "QR ini tidak aktif. Minta bantuan pelayan, ya.")
}

func (s *Service) Lookup(ctx context.Context, qr string) (TableState, error) {
	q := s.db.Read()
	t, err := q.GetTableByQR(ctx, qr)
	if errors.Is(err, sql.ErrNoRows) || err == nil && t.Active == 0 {
		return TableState{State: "unavailable", Customers: []CustomerRef{}}, nil
	}
	if err != nil {
		return TableState{}, err
	}
	out := TableState{State: "free", Table: &TableRef{t.Number, t.Label}, Customers: []CustomerRef{}}
	bill, err := q.GetOpenBillForTable(ctx, t.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return TableState{}, err
	}
	out.State = "occupied"
	out.Settling = bill.Status == BillSettling
	entries, err := q.ListCustomerEntries(ctx, bill.ID)
	if err != nil {
		return TableState{}, err
	}
	for _, e := range entries {
		out.Customers = append(out.Customers, CustomerRef{ID: e.ID, Name: s.name(e)})
	}
	return out, nil
}

// name decrypts a Customer's name; Anonymised or shredded entries read "Customer" (TR-RET-01).
func (s *Service) name(e store.CustomerEntry) string {
	if e.NameEnc == nil {
		return "Customer"
	}
	b, err := s.vault.Open(e.KeyID, e.NameEnc)
	if err != nil {
		return "Customer"
	}
	return string(b)
}

// restriction decrypts a Customer's Dietary Restriction tag ids.
func (s *Service) restriction(e store.CustomerEntry) []string {
	if e.RestrictionTagsEnc == nil {
		return []string{}
	}
	b, err := s.vault.Open(e.KeyID, e.RestrictionTagsEnc)
	if err != nil {
		return []string{}
	}
	var ids []string
	if json.Unmarshal(b, &ids) != nil {
		return []string{}
	}
	return ids
}

func (s *Service) note(e store.CustomerEntry) string {
	if e.DietaryNoteEnc == nil {
		return ""
	}
	b, err := s.vault.Open(e.KeyID, e.DietaryNoteEnc)
	if err != nil {
		return ""
	}
	return string(b)
}

// JoinInput is the name prompt (BR-CM-02). Restriction and note are kept only with consent.
type JoinInput struct {
	Name              string   `json:"name"`
	RestrictionTagIDs []string `json:"restrictionTagIds"`
	DietaryNote       string   `json:"dietaryNote"`
	Consent           bool     `json:"consent"`
}

// Session is returned on join and Rejoin. FirstScan is true for the scan that opened the Bill,
// which plays the splash (MOTION M1).
type Session struct {
	Token      string `json:"token"`
	CustomerID string `json:"customerId"`
	BillID     string `json:"billId"`
	Name       string `json:"name"`
	FirstScan  bool   `json:"firstScan"`
}

func (s *Service) validateJoin(ctx context.Context, in *JoinInput) error {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	if in.Name == "" {
		return invalid("Tulis namamu dulu, ya.")
	}
	if utf8.RuneCountInString(in.Name) > maxName {
		return invalid("Nama maksimal 40 huruf.")
	}
	in.DietaryNote = strings.TrimSpace(in.DietaryNote)
	if !in.Consent {
		// OP-01: without consent, dietary data is discarded and the Customer continues.
		in.RestrictionTagIDs, in.DietaryNote = nil, ""
		return nil
	}
	if utf8.RuneCountInString(in.DietaryNote) > maxNote {
		return invalid("Catatan diet maksimal 300 huruf.")
	}
	tags, err := s.db.Read().ListDietaryTags(ctx)
	if err != nil {
		return err
	}
	var clean []string
	for _, id := range in.RestrictionTagIDs {
		ok := slices.ContainsFunc(tags, func(t store.DietaryTag) bool { return t.ID == id && t.Active == 1 })
		if !ok {
			return invalid("Pantangan makanan tidak dikenal.")
		}
		if !slices.Contains(clean, id) {
			clean = append(clean, id)
		}
	}
	in.RestrictionTagIDs = clean
	return nil
}

// Join opens the Table's Bill if it has none, or joins the open one, and adds a new Customer
// (BR-CM-01, BR-CM-06). BEGIN IMMEDIATE plus the one-open-Bill index make two simultaneous
// first scans produce one Bill (TR-DM-01).
func (s *Service) Join(ctx context.Context, qr string, in JoinInput) (Session, error) {
	if err := s.validateJoin(ctx, &in); err != nil {
		return Session{}, err
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	keyID, nameEnc, err := s.vault.Seal(now, []byte(in.Name))
	if err != nil {
		return Session{}, err
	}
	var noteEnc, tagsEnc []byte
	var consentAt sql.NullInt64
	if in.DietaryNote != "" || len(in.RestrictionTagIDs) > 0 {
		consentAt = sql.NullInt64{Int64: nowMs, Valid: true} // TR-RET-03
		if in.DietaryNote != "" {
			if noteEnc, err = s.vault.SealWith(keyID, []byte(in.DietaryNote)); err != nil {
				return Session{}, err
			}
		}
		if len(in.RestrictionTagIDs) > 0 {
			raw, _ := json.Marshal(in.RestrictionTagIDs)
			if tagsEnc, err = s.vault.SealWith(keyID, raw); err != nil {
				return Session{}, err
			}
		}
	}
	token, digest := platform.NewToken()
	out := Session{Token: token, CustomerID: platform.NewID(), Name: in.Name}
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		t, err := tx.GetTableByQR(ctx, qr)
		if errors.Is(err, sql.ErrNoRows) || err == nil && t.Active == 0 {
			return unavailable()
		}
		if err != nil {
			return err
		}
		bill, err := tx.GetOpenBillForTable(ctx, t.ID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if bill, err = s.openBill(ctx, tx, t.ID, now); err != nil {
				return err
			}
			out.FirstScan = true
		case err != nil:
			return err
		}
		out.BillID = bill.ID
		tokenID := platform.NewID()
		if err := tx.CreateCustomerEntry(ctx, store.CreateCustomerEntryParams{ID: out.CustomerID, BillID: bill.ID, KeyID: keyID,
			NameEnc: nameEnc, DietaryNoteEnc: noteEnc, RestrictionTagsEnc: tagsEnc, ConsentAt: consentAt,
			ActiveDeviceID: sql.NullString{String: tokenID, Valid: true}, JoinedAt: nowMs}); err != nil {
			return err
		}
		if err := tx.CreateGuestToken(ctx, store.CreateGuestTokenParams{ID: tokenID, CustomerEntryID: out.CustomerID, TokenHash: digest, CreatedAt: nowMs}); err != nil {
			return err
		}
		tx.Publish(
			realtime.Event{Topic: BillTopic(bill.ID), Type: "customer.joined", ID: out.CustomerID},
			realtime.Event{Topic: WaitersTopic, Type: "table.changed", ID: t.ID},
		)
		return nil
	})
	return out, err
}

func (s *Service) openBill(ctx context.Context, tx *dbx.Tx, tableID string, now time.Time) (store.Bill, error) {
	date := BusinessDate(now)
	n, err := tx.NextBillNumber(ctx, date)
	if err != nil {
		return store.Bill{}, err
	}
	nowMs := platform.Millis(now)
	b := store.CreateBillParams{ID: platform.NewID(), BusinessDate: date, Number: n, Type: "dine_in", Source: "live", OpenedAt: nowMs, UpdatedAt: nowMs}
	if err := tx.CreateBill(ctx, b); err != nil {
		return store.Bill{}, err
	}
	if err := tx.AddBillTable(ctx, store.AddBillTableParams{BillID: b.ID, TableID: tableID}); err != nil {
		return store.Bill{}, err
	}
	return tx.GetBill(ctx, b.ID)
}

// Rejoin continues as an existing Customer on a new device: the new token replaces the old
// one, which is told "You continued on another device" (BR-CM-12, BR-CM-13, TR-AUTH-07).
func (s *Service) Rejoin(ctx context.Context, qr, customerID string) (Session, error) {
	now := platform.Millis(s.clock.Now())
	token, digest := platform.NewToken()
	out := Session{Token: token, CustomerID: customerID}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		t, err := tx.GetTableByQR(ctx, qr)
		if errors.Is(err, sql.ErrNoRows) || err == nil && t.Active == 0 {
			return unavailable()
		}
		if err != nil {
			return err
		}
		bill, err := tx.GetOpenBillForTable(ctx, t.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return problem(http.StatusConflict, "table_free", "Meja ini sudah kosong. Mulai pesanan baru, ya.")
		}
		if err != nil {
			return err
		}
		e, err := tx.GetCustomerEntry(ctx, customerID)
		if errors.Is(err, sql.ErrNoRows) || err == nil && e.BillID != bill.ID {
			return problem(http.StatusNotFound, "customer_not_found", "Nama ini tidak ada di meja ini.")
		}
		if err != nil {
			return err
		}
		out.BillID, out.Name = bill.ID, s.name(e)
		if err := tx.RevokeGuestTokens(ctx, store.RevokeGuestTokensParams{RevokedAt: sql.NullInt64{Int64: now, Valid: true}, CustomerEntryID: customerID}); err != nil {
			return err
		}
		tokenID := platform.NewID()
		if err := tx.CreateGuestToken(ctx, store.CreateGuestTokenParams{ID: tokenID, CustomerEntryID: customerID, TokenHash: digest, CreatedAt: now}); err != nil {
			return err
		}
		if err := tx.SetCustomerDevice(ctx, store.SetCustomerDeviceParams{ActiveDeviceID: sql.NullString{String: tokenID, Valid: true},
			RejoinedAt: sql.NullInt64{Int64: now, Valid: true}, ID: customerID}); err != nil {
			return err
		}
		tx.Publish(realtime.Event{Topic: CustomerTopic(customerID), Type: "customer.revoked", ID: customerID})
		return nil
	})
	return out, err
}

// NotMine handles "This isn't my table": Waiters are notified (BR-CM-12).
func (s *Service) NotMine(ctx context.Context, qr string) error {
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		t, err := tx.GetTableByQR(ctx, qr)
		if errors.Is(err, sql.ErrNoRows) || err == nil && t.Active == 0 {
			return unavailable()
		}
		if err != nil {
			return err
		}
		call := store.CreateWaiterCallParams{ID: platform.NewID(), Kind: "not_mine", TableID: t.ID, CreatedAt: platform.Millis(s.clock.Now())}
		if bill, err := tx.GetOpenBillForTable(ctx, t.ID); err == nil {
			call.BillID = sql.NullString{String: bill.ID, Valid: true}
		}
		if err := tx.CreateWaiterCall(ctx, call); err != nil {
			return err
		}
		tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "waiter_call.created", ID: call.ID})
		return nil
	})
}

// CallWaiter notifies on-duty Waiters (BR-CM-07). A Customer with an unanswered call gets the
// same call back instead of a second one.
func (s *Service) CallWaiter(ctx context.Context, g *Guest) (WaiterCall, error) {
	var out WaiterCall
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		cid := sql.NullString{String: g.CustomerID, Valid: true}
		if open, err := tx.GetOpenWaiterCall(ctx, cid); err == nil {
			out = WaiterCall{ID: open.ID, CreatedAt: open.CreatedAt}
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		tables, err := tx.ListBillTables(ctx, g.BillID)
		if err != nil || len(tables) == 0 {
			return errors.Join(err, errors.New("bill has no table"))
		}
		out = WaiterCall{ID: platform.NewID(), CreatedAt: platform.Millis(s.clock.Now())}
		if err := tx.CreateWaiterCall(ctx, store.CreateWaiterCallParams{ID: out.ID, Kind: "call", TableID: tables[0].ID,
			BillID: sql.NullString{String: g.BillID, Valid: true}, CustomerEntryID: cid, CreatedAt: out.CreatedAt}); err != nil {
			return err
		}
		tx.Publish(
			realtime.Event{Topic: WaitersTopic, Type: "waiter_call.created", ID: out.ID},
			realtime.Event{Topic: BillTopic(g.BillID), Type: "waiter_call.created", ID: out.ID},
		)
		return nil
	})
	return out, err
}

type WaiterCall struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
}

// ---------------------------------------------------------------- menu

// Promo is the best automatic item discount on a Variant right now, priced by `pricing`.
type Promo struct {
	DiscountID string `json:"discountId"`
	Name       string `json:"name"`
	Price      int64  `json:"price"`
}

// Menu is the Client Menu catalog: Active Products only (BR-CM-03), Sold Out Variants kept so
// they can be greyed out (BR-CM-04), with the price display settings (BR-CM-09).
type Menu struct {
	Categories  []catalog.Category   `json:"categories"`
	DietaryTags []catalog.DietaryTag `json:"dietaryTags"`
	Products    []catalog.Product    `json:"products"`
	Promos      map[string]Promo     `json:"promos"` // by Variant id
	PriceMode   string               `json:"priceMode"`
	TaxRateBP   int64                `json:"taxRateBp"`
	ServiceBP   int64                `json:"serviceRateBp"`
}

func (s *Service) Menu(ctx context.Context) (Menu, error) {
	cat, err := catalog.Load(ctx, s.db.Read(), false)
	if err != nil {
		return Menu{}, err
	}
	settings, err := s.outlet.Settings(ctx)
	if err != nil {
		return Menu{}, err
	}
	discounts, err := s.outlet.Discounts(ctx)
	if err != nil {
		return Menu{}, err
	}
	auto := outlet.ActiveDiscounts(discounts, s.clock.Now())
	m := Menu{Categories: []catalog.Category{}, DietaryTags: []catalog.DietaryTag{}, Products: []catalog.Product{}, Promos: map[string]Promo{},
		PriceMode: settings.Pricing.PriceMode, TaxRateBP: settings.Pricing.TaxRateBP, ServiceBP: settings.Pricing.ServiceRateBP}
	for _, c := range cat.Categories {
		if c.Active {
			m.Categories = append(m.Categories, c)
		}
	}
	for _, t := range cat.DietaryTags {
		if t.Active {
			m.DietaryTags = append(m.DietaryTags, t)
		}
	}
	names := map[string]string{}
	for _, d := range discounts {
		names[d.ID] = d.Name
	}
	for _, p := range cat.Products {
		if p.Status != catalog.StatusActive {
			continue
		}
		m.Products = append(m.Products, p)
		for _, v := range p.Variants {
			if r := priceLines([]pricedLine{{p, v, nil, 1}}, auto); r.Lines[0].Discount > 0 {
				m.Promos[v.ID] = Promo{DiscountID: r.Lines[0].DiscountID, Name: names[r.Lines[0].DiscountID], Price: r.Lines[0].Net}
			}
		}
	}
	return m, nil
}
