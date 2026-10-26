// Package pricing implements the Bill calculation from BR-R-14 to
// BR-R-18 and the pricing settings from BR-ADM-04, BR-ADM-05 and
// BR-ADM-06.
//
// Calculation order (BR-R-14):
//  1. Subtotal = sum (Variant price + Modifier prices) x quantity,
//     excluding Cancelled and Voided items (callers pass only the
//     items that count).
//  2. Minus item-level discounts.
//  3. Minus Bill-level discount.
//  4. Plus service charge on the result of steps 2-3.
//  5. Plus PB1 tax on the result of step 4.
//
// Money is INTEGER whole Rupiah. Rates are basis points
// (1000 bp = 10%). Discount value is percent points when Kind is
// "percent" and whole Rupiah when Kind is "nominal".
// BR-R-16: at most one item-level discount per Order Item, and the
// best one for the Customer applies automatically; at most one
// Bill-level discount per Bill. BR-R-17: manual discounts require a
// Management PIN Override, which is enforced by the auth layer, not
// by these pure functions. BR-R-18: only cash Payments are rounded,
// to the nearest Rp 100, and the rounding difference is recorded.
package pricing

import (
	"errors"
	"strings"
	"time"
)

const (
	KindPercent   = "percent"
	KindNominal   = "nominal"
	ScopeItem     = "item"
	ScopeBill     = "bill"
	TriggerAuto   = "auto"
	TriggerManual = "manual"
	ModeExclusive = "exclusive"
	ModeInclusive = "inclusive"
)

type Settings struct {
	ServiceRateBP int    `json:"service_rate_bp"`
	TaxRateBP     int    `json:"tax_rate_bp"`
	PriceMode     string `json:"price_mode"`
	RefundCap     int64  `json:"refund_cap"`
}

func DefaultSettings() Settings {
	return Settings{ServiceRateBP: 0, TaxRateBP: 1000, PriceMode: ModeExclusive, RefundCap: 500000}
}

type Discount struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Value      int64   `json:"value"`
	Scope      string  `json:"scope"`
	Trigger    string  `json:"trigger"`
	Days       *string `json:"days"`
	StartTime  *string `json:"start_time"`
	EndTime    *string `json:"end_time"`
	ValidFrom  *int64  `json:"valid_from"`
	ValidTo    *int64  `json:"valid_to"`
	TargetKind *string `json:"target_kind"`
	TargetID   *string `json:"target_id"`
	Active     bool    `json:"active"`
	CreatedAt  int64   `json:"created_at"`
	UpdatedAt  int64   `json:"updated_at"`
}

func ValidateDiscount(d Discount) error {
	if d.Name == "" {
		return errors.New("discount name is required")
	}
	switch d.Kind {
	case KindPercent:
		if d.Value < 0 || d.Value > 100 {
			return errors.New("percent discount value must be 0 to 100")
		}
	case KindNominal:
		if d.Value < 0 {
			return errors.New("nominal discount value must be >= 0")
		}
	default:
		return errors.New("discount kind must be percent or nominal")
	}
	switch d.Scope {
	case ScopeItem, ScopeBill:
	default:
		return errors.New("discount scope must be item or bill")
	}
	switch d.Trigger {
	case TriggerAuto, TriggerManual:
	default:
		return errors.New("discount trigger must be auto or manual")
	}
	return nil
}

// IsActiveAt checks the schedule window for an automatic promo.
// valid_from/valid_to are UTC Unix milliseconds. Days is a comma list
// like "Mon,Tue", and start/end times are HH:MM in Asia/Jakarta.
// Empty schedule fields mean no restriction on that dimension.
func (d Discount) IsActiveAt(t time.Time) bool {
	if !d.Active {
		return false
	}
	ms := t.UnixMilli()
	if d.ValidFrom != nil && ms < *d.ValidFrom {
		return false
	}
	if d.ValidTo != nil && ms > *d.ValidTo {
		return false
	}
	if d.Days != nil && strings.TrimSpace(*d.Days) != "" {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			loc = time.UTC
		}
		wd := t.In(loc).Format("Mon")
		found := false
		for _, day := range strings.Split(*d.Days, ",") {
			if strings.TrimSpace(day) == wd {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if d.StartTime != nil && d.EndTime != nil && *d.StartTime != "" && *d.EndTime != "" {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			loc = time.UTC
		}
		now := t.In(loc).Format("15:04")
		if now < *d.StartTime || now > *d.EndTime {
			return false
		}
	}
	return true
}

// DiscountAmount computes one discount on a base amount, capped at the base.
func DiscountAmount(d Discount, base int64) int64 {
	if base <= 0 {
		return 0
	}
	var amount int64
	if d.Kind == KindPercent {
		amount = (base*d.Value + 50) / 100
	} else {
		amount = d.Value
	}
	if amount > base {
		amount = base
	}
	return amount
}

// BestItemDiscount returns the largest discount amount for one item,
// implementing the "best one for the Customer" rule in BR-R-16.
func BestItemDiscount(candidates []Discount, base int64) int64 {
	var best int64
	for _, d := range candidates {
		if d.Scope != ScopeItem {
			continue
		}
		if amt := DiscountAmount(d, base); amt > best {
			best = amt
		}
	}
	return best
}

func rateAmount(base int64, rateBP int) int64 {
	if base <= 0 || rateBP <= 0 {
		return 0
	}
	return (base*int64(rateBP) + 5000) / 10000
}

type ItemInput struct {
	VariantPrice   int64      `json:"variant_price"`
	ModifierPrices []int64    `json:"modifier_prices"`
	Quantity       int        `json:"quantity"`
	ItemDiscounts  []Discount `json:"item_discounts"`
}

type Breakdown struct {
	Subtotal          int64 `json:"subtotal"`
	ItemDiscountTotal int64 `json:"item_discount_total"`
	BillDiscount      int64 `json:"bill_discount"`
	ServiceCharge     int64 `json:"service_charge"`
	Tax               int64 `json:"tax"`
	Total             int64 `json:"total"`
	CashRounding      int64 `json:"cash_rounding"`
	TotalForCash      int64 `json:"total_for_cash"`
}

// Calculate applies BR-R-14 in order. BillDiscount, if present, must
// already be the single Bill-level discount for the Bill (BR-R-16).
// PaymentMethod "cash" triggers BR-R-18 rounding to the nearest Rp 100;
// any other method leaves TotalForCash equal to Total.
func Calculate(items []ItemInput, billDiscount *Discount, settings Settings, paymentMethod string) (Breakdown, error) {
	if settings.TaxRateBP < 0 || settings.ServiceRateBP < 0 {
		return Breakdown{}, errors.New("rates must be >= 0")
	}
	var subtotal, itemDiscountTotal int64
	for _, it := range items {
		if it.Quantity <= 0 {
			return Breakdown{}, errors.New("item quantity must be > 0")
		}
		if it.VariantPrice < 0 {
			return Breakdown{}, errors.New("variant price must be >= 0")
		}
		unit := it.VariantPrice
		for _, mp := range it.ModifierPrices {
			if mp < 0 {
				return Breakdown{}, errors.New("modifier price must be >= 0")
			}
			unit += mp
		}
		amount := unit * int64(it.Quantity)
		subtotal += amount
		itemDiscountTotal += BestItemDiscount(it.ItemDiscounts, amount)
	}
	afterItems := subtotal - itemDiscountTotal
	if afterItems < 0 {
		afterItems = 0
	}
	var billDiscountAmount int64
	if billDiscount != nil {
		if billDiscount.Scope != ScopeBill {
			return Breakdown{}, errors.New("bill discount must have scope bill")
		}
		billDiscountAmount = DiscountAmount(*billDiscount, afterItems)
	}
	afterBill := afterItems - billDiscountAmount
	if afterBill < 0 {
		afterBill = 0
	}
	service := rateAmount(afterBill, settings.ServiceRateBP)
	tax := rateAmount(afterBill+service, settings.TaxRateBP)
	total := afterBill + service + tax
	out := Breakdown{
		Subtotal: subtotal, ItemDiscountTotal: itemDiscountTotal,
		BillDiscount: billDiscountAmount, ServiceCharge: service,
		Tax: tax, Total: total, TotalForCash: total,
	}
	if paymentMethod == "cash" {
		rounded := ((total + 50) / 100) * 100
		out.CashRounding = rounded - total
		out.TotalForCash = rounded
	}
	return out, nil
}
