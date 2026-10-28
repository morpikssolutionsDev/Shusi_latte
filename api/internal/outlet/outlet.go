// Package outlet holds outlet-wide configuration: pricing settings, escalation timers,
// discounts and Tables (BR-ADM-04..06, BR-ADM-08, BR-ADM-13).
package outlet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

type Service struct {
	db    *dbx.DB
	clock platform.Clock
}

func NewService(db *dbx.DB, clock platform.Clock) *Service { return &Service{db: db, clock: clock} }

func invalid(detail string) error {
	p := httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_settings", "Pengaturan tidak valid")
	p.Detail = detail
	return p
}

var errNotFound = httpx.NewProblem(http.StatusNotFound, "not_found", "Tidak ditemukan")

func (s *Service) audit(ctx context.Context, tx *dbx.Tx, actor, action, entity, id string, before, after any) error {
	return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: action, Entity: entity, EntityID: id, Before: before, After: after})
}

// ---------------------------------------------------------------- Settings

type PricingSettings struct {
	ServiceRateBP int64  `json:"serviceRateBp"`
	TaxRateBP     int64  `json:"taxRateBp"`
	PriceMode     string `json:"priceMode"`
	RefundCap     int64  `json:"refundCap"`
}

type OpsSettings struct {
	ClaimRenotifySeconds     int64 `json:"claimRenotifySeconds"`
	ClaimEscalateSeconds     int64 `json:"claimEscalateSeconds"`
	PickupReminderSeconds    int64 `json:"pickupReminderSeconds"`
	EmptyBillCloseSeconds    int64 `json:"emptyBillCloseSeconds"`
	AutoClockOutGraceSeconds int64 `json:"autoClockOutGraceSeconds"`
}

type Settings struct {
	Pricing PricingSettings `json:"pricing"`
	Ops     OpsSettings     `json:"ops"`
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	q := s.db.Read()
	p, err := q.GetPricingSetting(ctx)
	if err != nil {
		return Settings{}, err
	}
	o, err := q.GetOpsSetting(ctx)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Pricing: PricingSettings{ServiceRateBP: p.ServiceRateBp, TaxRateBP: p.TaxRateBp, PriceMode: p.PriceMode, RefundCap: p.RefundCap},
		Ops: OpsSettings{ClaimRenotifySeconds: o.ClaimRenotifySeconds, ClaimEscalateSeconds: o.ClaimEscalateSeconds,
			PickupReminderSeconds: o.PickupReminderSeconds, EmptyBillCloseSeconds: o.EmptyBillCloseSeconds,
			AutoClockOutGraceSeconds: o.AutoClockOutGraceSeconds},
	}, nil
}

// PricingFor returns the pricing package's view of the settings.
func (p PricingSettings) PricingFor() pricing.Settings {
	return pricing.Settings{ServiceRateBP: p.ServiceRateBP, TaxRateBP: p.TaxRateBP}
}

func (s *Service) UpdatePricing(ctx context.Context, actor string, in PricingSettings) (Settings, error) {
	if in.ServiceRateBP < 0 || in.ServiceRateBP > 2500 || in.TaxRateBP < 0 || in.TaxRateBP > 2500 {
		return Settings{}, invalid("Pajak dan service charge harus 0–25%")
	}
	if in.PriceMode != "exclusive" && in.PriceMode != "inclusive" {
		return Settings{}, invalid("Mode harga tidak dikenal")
	}
	if in.RefundCap < 0 {
		return Settings{}, invalid("Batas refund tidak boleh negatif")
	}
	before, err := s.Settings(ctx)
	if err != nil {
		return Settings{}, err
	}
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		if err := tx.UpdatePricingSetting(ctx, store.UpdatePricingSettingParams{ServiceRateBp: in.ServiceRateBP, TaxRateBp: in.TaxRateBP,
			PriceMode: in.PriceMode, RefundCap: in.RefundCap}); err != nil {
			return err
		}
		tx.Publish(realtime.Event{Topic: "menu", Type: "settings.pricing"})
		return s.audit(ctx, tx, actor, "settings.pricing.updated", "settings", "pricing", before.Pricing, in)
	})
	if err != nil {
		return Settings{}, err
	}
	return s.Settings(ctx)
}

func (s *Service) UpdateOps(ctx context.Context, actor string, in OpsSettings) (Settings, error) {
	inRange := func(v, lo, hi int64) bool { return v >= lo && v <= hi }
	if !inRange(in.ClaimRenotifySeconds, 30, 3600) || !inRange(in.ClaimEscalateSeconds, 60, 7200) ||
		in.ClaimEscalateSeconds <= in.ClaimRenotifySeconds || !inRange(in.PickupReminderSeconds, 30, 3600) ||
		!inRange(in.EmptyBillCloseSeconds, 300, 4*3600) || !inRange(in.AutoClockOutGraceSeconds, 1800, 12*3600) {
		return Settings{}, invalid("Timer di luar batas, atau eskalasi tidak lebih lama dari notifikasi ulang")
	}
	before, err := s.Settings(ctx)
	if err != nil {
		return Settings{}, err
	}
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		if err := tx.UpdateOpsSetting(ctx, store.UpdateOpsSettingParams{ClaimRenotifySeconds: in.ClaimRenotifySeconds,
			ClaimEscalateSeconds: in.ClaimEscalateSeconds, PickupReminderSeconds: in.PickupReminderSeconds,
			EmptyBillCloseSeconds: in.EmptyBillCloseSeconds, AutoClockOutGraceSeconds: in.AutoClockOutGraceSeconds}); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "settings.ops.updated", "settings", "ops", before.Ops, in)
	})
	if err != nil {
		return Settings{}, err
	}
	return s.Settings(ctx)
}

// ---------------------------------------------------------------- Discounts

type Discount struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Value       int64    `json:"value"`
	Scope       string   `json:"scope"`
	Trigger     string   `json:"trigger"`
	DaysMask    int64    `json:"daysMask"`
	StartMinute *int64   `json:"startMinute,omitempty"`
	EndMinute   *int64   `json:"endMinute,omitempty"`
	ValidFrom   *int64   `json:"validFrom,omitempty"`
	ValidTo     *int64   `json:"validTo,omitempty"`
	MinSubtotal *int64   `json:"minSubtotal,omitempty"`
	Active      bool     `json:"active"`
	Categories  []string `json:"categoryIds"`
	Products    []string `json:"productIds"`
	Variants    []string `json:"variantIds"`
}

func (s *Service) Discounts(ctx context.Context) ([]Discount, error) {
	q := s.db.Read()
	rows, err := q.ListDiscounts(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := q.ListDiscountTargets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Discount, 0, len(rows))
	for _, r := range rows {
		d := Discount{ID: r.ID, Name: r.Name, Kind: r.Kind, Value: r.Value, Scope: r.Scope, Trigger: r.Trigger, DaysMask: r.DaysMask,
			StartMinute: ptrInt(r.StartMinute), EndMinute: ptrInt(r.EndMinute), ValidFrom: ptrInt(r.ValidFrom), ValidTo: ptrInt(r.ValidTo),
			MinSubtotal: ptrInt(r.MinSubtotal), Active: r.Active == 1, Categories: []string{}, Products: []string{}, Variants: []string{}}
		for _, t := range targets {
			if t.DiscountID != r.ID {
				continue
			}
			switch t.TargetType {
			case "category":
				d.Categories = append(d.Categories, t.TargetID)
			case "product":
				d.Products = append(d.Products, t.TargetID)
			case "variant":
				d.Variants = append(d.Variants, t.TargetID)
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func validateDiscount(d *Discount) error {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return invalid("Nama diskon wajib diisi")
	}
	switch d.Kind {
	case "percent":
		if d.Value < 1 || d.Value > 10000 {
			return invalid("Persentase harus 0,01–100%")
		}
	case "nominal":
		if d.Value < 1 {
			return invalid("Nominal diskon harus lebih dari 0")
		}
	default:
		return invalid("Jenis diskon tidak dikenal")
	}
	if d.Scope != "item" && d.Scope != "bill" {
		return invalid("Cakupan diskon tidak dikenal")
	}
	if d.Trigger != "auto" && d.Trigger != "manual" {
		return invalid("Pemicu diskon tidak dikenal")
	}
	if d.Scope == "bill" && len(d.Categories)+len(d.Products)+len(d.Variants) > 0 {
		return invalid("Diskon tagihan tidak punya target produk")
	}
	if d.Scope == "item" && d.MinSubtotal != nil {
		return invalid("Minimum subtotal hanya untuk diskon tagihan")
	}
	if d.DaysMask == 0 {
		d.DaysMask = 127
	}
	if d.DaysMask < 1 || d.DaysMask > 127 {
		return invalid("Hari berlaku tidak valid")
	}
	if (d.StartMinute == nil) != (d.EndMinute == nil) {
		return invalid("Jam mulai dan selesai harus diisi berdua")
	}
	if d.StartMinute != nil && (*d.StartMinute < 0 || *d.StartMinute > 1439 || *d.EndMinute < 0 || *d.EndMinute > 1440 || *d.StartMinute == *d.EndMinute) {
		return invalid("Jam berlaku tidak valid")
	}
	if d.ValidFrom != nil && d.ValidTo != nil && *d.ValidTo <= *d.ValidFrom {
		return invalid("Tanggal selesai harus setelah tanggal mulai")
	}
	return nil
}

func (s *Service) SaveDiscount(ctx context.Context, actor string, in Discount) (Discount, error) {
	if err := validateDiscount(&in); err != nil {
		return in, err
	}
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		now := platform.Millis(s.clock.Now())
		action := "discount.updated"
		var before any
		if in.ID == "" {
			in.ID = platform.NewID()
			action = "discount.created"
			if err := tx.CreateDiscount(ctx, store.CreateDiscountParams{ID: in.ID, Name: in.Name, Kind: in.Kind, Value: in.Value,
				Scope: in.Scope, Trigger: in.Trigger, DaysMask: in.DaysMask, StartMinute: nullPtr(in.StartMinute), EndMinute: nullPtr(in.EndMinute),
				ValidFrom: nullPtr(in.ValidFrom), ValidTo: nullPtr(in.ValidTo), MinSubtotal: nullPtr(in.MinSubtotal),
				Active: boolInt(in.Active), CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
		} else {
			old, err := tx.GetDiscount(ctx, in.ID)
			if errors.Is(err, sql.ErrNoRows) {
				return errNotFound
			}
			if err != nil {
				return err
			}
			before = old
			if err := tx.UpdateDiscount(ctx, store.UpdateDiscountParams{Name: in.Name, Kind: in.Kind, Value: in.Value, Scope: in.Scope,
				Trigger: in.Trigger, DaysMask: in.DaysMask, StartMinute: nullPtr(in.StartMinute), EndMinute: nullPtr(in.EndMinute),
				ValidFrom: nullPtr(in.ValidFrom), ValidTo: nullPtr(in.ValidTo), MinSubtotal: nullPtr(in.MinSubtotal),
				Active: boolInt(in.Active), UpdatedAt: now, ID: in.ID}); err != nil {
				return err
			}
		}
		if err := tx.ClearDiscountTargets(ctx, in.ID); err != nil {
			return err
		}
		for typ, ids := range map[string][]string{"category": in.Categories, "product": in.Products, "variant": in.Variants} {
			for _, id := range ids {
				if err := tx.AddDiscountTarget(ctx, store.AddDiscountTargetParams{DiscountID: in.ID, TargetType: typ, TargetID: id}); err != nil {
					return err
				}
			}
		}
		tx.Publish(realtime.Event{Topic: "menu", Type: "discount.changed", ID: in.ID})
		return s.audit(ctx, tx, actor, action, "discount", in.ID, before, in)
	})
	return in, err
}

// ActiveDiscounts converts the automatic discounts active at t into pricing definitions.
func ActiveDiscounts(all []Discount, t time.Time) []pricing.Discount {
	var out []pricing.Discount
	for _, d := range all {
		if !d.Active || d.Trigger != "auto" {
			continue
		}
		pd := ToPricing(d)
		if pd.Schedule.ActiveAt(t, platform.Jakarta) {
			out = append(out, pd)
		}
	}
	return out
}

func ToPricing(d Discount) pricing.Discount {
	toInt := func(p *int64) *int {
		if p == nil {
			return nil
		}
		v := int(*p)
		return &v
	}
	toTime := func(p *int64) *time.Time {
		if p == nil {
			return nil
		}
		t := platform.FromMillis(*p)
		return &t
	}
	pd := pricing.Discount{ID: d.ID, Name: d.Name, Kind: pricing.Kind(d.Kind), Value: d.Value, Scope: pricing.Scope(d.Scope),
		Trigger: pricing.Trigger(d.Trigger), Categories: d.Categories, Products: d.Products, Variants: d.Variants,
		Schedule: pricing.Schedule{DaysMask: int(d.DaysMask), StartMinute: toInt(d.StartMinute), EndMinute: toInt(d.EndMinute),
			ValidFrom: toTime(d.ValidFrom), ValidTo: toTime(d.ValidTo)}}
	if d.MinSubtotal != nil {
		pd.MinSubtotal = *d.MinSubtotal
	}
	return pd
}

// ---------------------------------------------------------------- Tables

type Table struct {
	ID     string `json:"id"`
	Number int64  `json:"number"`
	Label  string `json:"label"`
	Code   string `json:"code"` // table-01: the path its printed QR opens (TableCode)
	Active bool   `json:"active"`
}

func toTable(t store.DiningTable) Table {
	return Table{ID: t.ID, Number: t.Number, Label: t.Label, Code: t.Code, Active: t.Active == 1}
}

// TableCode is a Table's fixed public code, table-01 … table-12: its QR opens /table-01 and never
// changes, since it follows the number (BR-ADM-08, decided 2026-10-28).
func TableCode(number int64) string { return fmt.Sprintf("table-%02d", number) }

// NormaliseTableCode accepts table-1 or TABLE-01 as table-01; anything else is returned as is
// (and finds no Table).
func NormaliseTableCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if rest, ok := strings.CutPrefix(s, "table-"); ok {
		if n, err := strconv.ParseInt(rest, 10, 64); err == nil && n > 0 {
			return TableCode(n)
		}
	}
	return s
}

func (s *Service) Tables(ctx context.Context) ([]Table, error) {
	rows, err := s.db.Read().ListTables(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Table, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTable(r))
	}
	return out, nil
}

func (s *Service) SaveTable(ctx context.Context, actor string, in Table) (Table, error) {
	if in.Number < 1 || in.Number > 999 {
		return in, invalid("Nomor meja harus 1–999")
	}
	in.Label = strings.TrimSpace(in.Label)
	if in.Label == "" {
		in.Label = fmt.Sprintf("Meja %d", in.Number)
	}
	var out Table
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		now := platform.Millis(s.clock.Now())
		if in.ID == "" {
			in.ID = platform.NewID()
			if err := tx.CreateTable(ctx, store.CreateTableParams{ID: in.ID, Number: in.Number, Label: in.Label, Code: TableCode(in.Number),
				Active: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
				return uniqueNumber(err)
			}
		} else {
			if _, err := tx.GetTable(ctx, in.ID); errors.Is(err, sql.ErrNoRows) {
				return errNotFound
			} else if err != nil {
				return err
			}
			if err := tx.UpdateTable(ctx, store.UpdateTableParams{Number: in.Number, Code: TableCode(in.Number), Label: in.Label, Active: boolInt(in.Active),
				UpdatedAt: now, ID: in.ID}); err != nil {
				return uniqueNumber(err)
			}
		}
		row, err := tx.GetTable(ctx, in.ID)
		if err != nil {
			return err
		}
		out = toTable(row)
		return s.audit(ctx, tx, actor, "table.saved", "table", in.ID, nil, out)
	})
	return out, err
}

func uniqueNumber(err error) error {
	if dbx.IsUniqueViolation(err) {
		return invalid("Nomor meja sudah dipakai")
	}
	return err
}

// ---------------------------------------------------------------- helpers

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func ptrInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func nullPtr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}
