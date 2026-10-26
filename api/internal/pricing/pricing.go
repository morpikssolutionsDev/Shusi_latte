// Package pricing computes Bill and Sub-bill totals (TRD §9). It is pure: no I/O, no clock,
// integer Rupiah and basis-point arithmetic only, so every rule is table-testable.
//
// Order of operations (BR-R-14):
//  1. subtotal = Σ (variant price + modifier prices) × quantity, excluding Cancelled and Voided
//  2. − item-level discounts (at most one per line, BR-R-16)
//  3. − Bill-level discount (at most one per Bill, BR-R-16)
//  4. + service charge on the result of 3
//  5. + tax on the result of 4
package pricing

import (
	"slices"
	"time"
)

// Kind is how a discount's value is read.
type Kind string

const (
	Percent Kind = "percent" // value in basis points (1500 = 15 %)
	Nominal Kind = "nominal" // value in Rupiah
)

type Scope string

const (
	ItemScope Scope = "item"
	BillScope Scope = "bill"
)

type Trigger string

const (
	Auto   Trigger = "auto"
	Manual Trigger = "manual"
)

// Discount is a discount definition with its eligibility rules.
type Discount struct {
	ID      string
	Name    string
	Kind    Kind
	Value   int64
	Scope   Scope
	Trigger Trigger
	// Targets restrict an item discount; empty means every line.
	Categories, Products, Variants []string
	// MinSubtotal gates a Bill discount on the subtotal after item discounts.
	MinSubtotal int64
	Schedule    Schedule
}

// Schedule limits when an automatic discount applies. Zero values mean "always".
type Schedule struct {
	DaysMask    int        // bit 0 = Monday … bit 6 = Sunday; 0 or 127 = every day
	StartMinute *int       // WIB minutes after midnight
	EndMinute   *int       // exclusive; start > end crosses midnight
	ValidFrom   *time.Time // inclusive
	ValidTo     *time.Time // exclusive
}

// ActiveAt reports whether the schedule allows the discount at t (evaluated in loc).
func (s Schedule) ActiveAt(t time.Time, loc *time.Location) bool {
	if s.ValidFrom != nil && t.Before(*s.ValidFrom) {
		return false
	}
	if s.ValidTo != nil && !t.Before(*s.ValidTo) {
		return false
	}
	local := t.In(loc)
	if s.DaysMask != 0 && s.DaysMask != 127 {
		weekday := (int(local.Weekday()) + 6) % 7 // Monday = 0
		if s.DaysMask&(1<<weekday) == 0 {
			return false
		}
	}
	if s.StartMinute != nil && s.EndMinute != nil {
		m := local.Hour()*60 + local.Minute()
		start, end := *s.StartMinute, *s.EndMinute
		if start <= end {
			return m >= start && m < end
		}
		return m >= start || m < end
	}
	return true
}

// Line is one Order Item as priced.
type Line struct {
	ID         string
	CategoryID string
	ProductID  string
	VariantID  string
	UnitPrice  int64   // Variant price snapshot
	Modifiers  []int64 // Modifier price snapshots
	Quantity   int64
	Excluded   bool // Cancelled or Voided
	// ManualDiscount is a Management-approved manual item discount, if any (BR-R-17).
	ManualDiscount *Discount
}

// Gross is (unit price + modifiers) × quantity.
func (l Line) Gross() int64 {
	unit := l.UnitPrice
	for _, m := range l.Modifiers {
		unit += m
	}
	return unit * l.Quantity
}

// Settings are the outlet's rates (basis points).
type Settings struct {
	ServiceRateBP int64
	TaxRateBP     int64
}

// Input is everything needed to price one Bill or Sub-bill.
type Input struct {
	Lines []Line
	// Auto are automatic discounts already filtered to those active at pricing time.
	Auto []Discount
	// ManualBill is a Management-approved manual Bill discount, if any.
	ManualBill *Discount
	Settings   Settings
}

type LineResult struct {
	LineID     string
	Gross      int64
	Discount   int64
	DiscountID string
	Net        int64
}

type Breakdown struct {
	Lines          []LineResult
	Subtotal       int64 // Σ gross of included lines
	ItemDiscount   int64
	BillDiscount   int64
	BillDiscountID string
	Base           int64 // subtotal − item − bill discounts
	Service        int64
	Tax            int64
	Total          int64
}

// Calculate prices a Bill (TR-PR-01, TR-PR-02).
func Calculate(in Input) Breakdown {
	var b Breakdown
	var afterItems int64
	for _, l := range in.Lines {
		if l.Excluded {
			continue
		}
		gross := l.Gross()
		r := LineResult{LineID: l.ID, Gross: gross}
		if d := bestItemDiscount(l, in.Auto); d != nil {
			r.Discount = discountAmount(*d, gross, l.Quantity)
			r.DiscountID = d.ID
		}
		r.Net = gross - r.Discount
		b.Lines = append(b.Lines, r)
		b.Subtotal += gross
		b.ItemDiscount += r.Discount
		afterItems += r.Net
	}
	if d := bestBillDiscount(afterItems, in.Auto, in.ManualBill); d != nil {
		b.BillDiscount = discountAmount(*d, afterItems, 1)
		b.BillDiscountID = d.ID
	}
	b.Base = afterItems - b.BillDiscount
	b.Service = ApplyRate(b.Base, in.Settings.ServiceRateBP)
	b.Tax = ApplyRate(b.Base+b.Service, in.Settings.TaxRateBP)
	b.Total = b.Base + b.Service + b.Tax
	return b
}

// bestItemDiscount picks the line's discount: an approved manual discount replaces the
// automatic ones; otherwise the automatic discount worth the most Rupiah wins (TR-PR-03).
func bestItemDiscount(l Line, auto []Discount) *Discount {
	if l.ManualDiscount != nil {
		return l.ManualDiscount
	}
	gross := l.Gross()
	var best *Discount
	var bestAmount int64
	for i := range auto {
		d := &auto[i]
		if d.Scope != ItemScope || !targets(*d, l) {
			continue
		}
		if a := discountAmount(*d, gross, l.Quantity); a > bestAmount {
			best, bestAmount = d, a
		}
	}
	return best
}

func bestBillDiscount(afterItems int64, auto []Discount, manual *Discount) *Discount {
	if manual != nil {
		return manual
	}
	var best *Discount
	var bestAmount int64
	for i := range auto {
		d := &auto[i]
		if d.Scope != BillScope || afterItems < d.MinSubtotal {
			continue
		}
		if a := discountAmount(*d, afterItems, 1); a > bestAmount {
			best, bestAmount = d, a
		}
	}
	return best
}

func targets(d Discount, l Line) bool {
	if len(d.Categories) == 0 && len(d.Products) == 0 && len(d.Variants) == 0 {
		return true
	}
	return slices.Contains(d.Categories, l.CategoryID) || slices.Contains(d.Products, l.ProductID) ||
		slices.Contains(d.Variants, l.VariantID)
}

// discountAmount is the Rupiah value of d on base. A nominal item discount is per unit, so it
// is multiplied by quantity; every discount is capped at the base so no line goes negative.
func discountAmount(d Discount, base, quantity int64) int64 {
	var a int64
	switch d.Kind {
	case Percent:
		a = ApplyRate(base, d.Value)
	case Nominal:
		a = d.Value * quantity
	}
	return min(a, base)
}

// ApplyRate returns amount × rateBP / 10 000, rounded half-up to whole Rupiah (TR-PR-02).
func ApplyRate(amount, rateBP int64) int64 {
	return (amount*rateBP + 5000) / 10000
}

// RoundCash rounds a cash amount to the nearest Rp 100, half-up (BR-R-18, TR-PR-05).
// It returns the rounded amount and the rounding applied (rounded − amount).
func RoundCash(amount int64) (rounded, rounding int64) {
	rounded = (amount + 50) / 100 * 100
	return rounded, rounded - amount
}

// SplitEqually divides total into n parts; the remainder Rupiah go to the last part (TR-PR-04).
func SplitEqually(total int64, n int) []int64 {
	if n <= 0 {
		return nil
	}
	parts := make([]int64, n)
	share := total / int64(n)
	for i := range parts {
		parts[i] = share
	}
	parts[n-1] += total - share*int64(n)
	return parts
}

// ProrateBillDiscount shares a Bill discount across Sub-bills by their subtotals, assigning
// the rounding remainder to the last Sub-bill so the parts always sum to the discount (TR-PR-04).
func ProrateBillDiscount(discount int64, subtotals []int64) []int64 {
	var total int64
	for _, s := range subtotals {
		total += s
	}
	out := make([]int64, len(subtotals))
	if total == 0 || len(subtotals) == 0 {
		return out
	}
	var given int64
	for i, s := range subtotals[:len(subtotals)-1] {
		out[i] = (discount*s + total/2) / total
		given += out[i]
	}
	out[len(out)-1] = discount - given
	return out
}

// NetFromInclusive back-calculates the pre-service, pre-tax price from a tax-inclusive
// display price (TR-PR-06), rounded half-up.
func NetFromInclusive(inclusive int64, s Settings) int64 {
	factor := (10000 + s.ServiceRateBP) * (10000 + s.TaxRateBP) // scaled by 10^8
	return (inclusive*100_000_000 + factor/2) / factor
}
