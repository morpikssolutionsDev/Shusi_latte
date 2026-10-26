package pricing

import (
	"reflect"
	"testing"
	"time"
)

var wib = time.FixedZone("WIB", 7*3600)

func ptr[T any](v T) *T { return &v }

var settings = Settings{ServiceRateBP: 500, TaxRateBP: 1000} // 5 % service, 10 % PB1 (DEMO-AND-SEED §5)

var (
	happyHour = Discount{ID: "hh", Kind: Percent, Value: 1500, Scope: ItemScope, Trigger: Auto, Categories: []string{"coffee"}}
	weekday   = Discount{ID: "wd", Kind: Percent, Value: 1000, Scope: ItemScope, Trigger: Auto, Products: []string{"latte"}}
	hemat     = Discount{ID: "hemat", Kind: Nominal, Value: 25_000, Scope: BillScope, Trigger: Auto, MinSubtotal: 350_000}
	manual10  = Discount{ID: "manual", Kind: Percent, Value: 1000, Scope: BillScope, Trigger: Manual}
)

func latte(qty int64) Line {
	return Line{ID: "l-latte", CategoryID: "coffee", ProductID: "latte", VariantID: "latte-hot", UnitPrice: 38_000, Quantity: qty}
}

func TestCalculate(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want Breakdown
	}{
		{
			// BR-R-14: service on subtotal, tax on subtotal + service.
			name: "no discounts",
			in: Input{Settings: settings, Lines: []Line{
				{ID: "a", UnitPrice: 48_000, Quantity: 1},
				{ID: "b", UnitPrice: 38_000, Modifiers: []int64{6_000, 8_000}, Quantity: 2}, // (38+6+8)×2 = 104 000
			}},
			want: Breakdown{Subtotal: 152_000, Base: 152_000, Service: 7_600, Tax: 15_960, Total: 175_560},
		},
		{
			// Cancelled and Voided items are excluded from every total.
			name: "excluded lines",
			in: Input{Settings: settings, Lines: []Line{
				{ID: "a", UnitPrice: 48_000, Quantity: 1},
				{ID: "x", UnitPrice: 99_000, Quantity: 3, Excluded: true},
			}},
			want: Breakdown{Subtotal: 48_000, Base: 48_000, Service: 2_400, Tax: 5_040, Total: 55_440},
		},
		{
			// BR-R-16, TR-PR-03: both auto discounts target the latte; the larger (15 %) wins.
			name: "best item discount wins",
			in:   Input{Settings: settings, Auto: []Discount{weekday, happyHour}, Lines: []Line{latte(2)}},
			want: Breakdown{Subtotal: 76_000, ItemDiscount: 11_400, Base: 64_600, Service: 3_230, Tax: 6_783, Total: 74_613},
		},
		{
			// Bill discount applies after item discounts, gated on min subtotal.
			name: "bill discount on top of item discount",
			in: Input{Settings: settings, Auto: []Discount{happyHour, hemat}, Lines: []Line{
				latte(2), // 76 000 − 11 400
				{ID: "s", CategoryID: "sashimi", UnitPrice: 195_000, Quantity: 2}, // 390 000
			}},
			want: Breakdown{Subtotal: 466_000, ItemDiscount: 11_400, BillDiscount: 25_000, BillDiscountID: "hemat",
				Base: 429_600, Service: 21_480, Tax: 45_108, Total: 496_188},
		},
		{
			name: "bill discount below minimum does not apply",
			in:   Input{Settings: settings, Auto: []Discount{hemat}, Lines: []Line{latte(1)}},
			want: Breakdown{Subtotal: 38_000, Base: 38_000, Service: 1_900, Tax: 3_990, Total: 43_890},
		},
		{
			// BR-R-17: an approved manual Bill discount replaces the automatic one.
			name: "manual bill discount replaces auto",
			in: Input{Settings: settings, Auto: []Discount{hemat}, ManualBill: &manual10, Lines: []Line{
				{ID: "s", UnitPrice: 400_000, Quantity: 1},
			}},
			want: Breakdown{Subtotal: 400_000, BillDiscount: 40_000, BillDiscountID: "manual", Base: 360_000,
				Service: 18_000, Tax: 37_800, Total: 415_800},
		},
		{
			// A nominal item discount is per unit and never exceeds the line.
			name: "nominal item discount capped at line",
			in: Input{Settings: Settings{}, Auto: []Discount{{ID: "n", Kind: Nominal, Value: 50_000, Scope: ItemScope}},
				Lines: []Line{{ID: "e", UnitPrice: 38_000, Quantity: 2}}},
			want: Breakdown{Subtotal: 76_000, ItemDiscount: 76_000, Base: 0, Total: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate(tt.in)
			got.Lines = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("\n got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestManualItemDiscountReplacesAuto(t *testing.T) {
	l := latte(1)
	l.ManualDiscount = &Discount{ID: "m", Kind: Nominal, Value: 5_000, Scope: ItemScope, Trigger: Manual}
	got := Calculate(Input{Settings: settings, Auto: []Discount{happyHour}, Lines: []Line{l}})
	if got.Lines[0].DiscountID != "m" || got.Lines[0].Discount != 5_000 {
		t.Fatalf("manual discount not applied: %+v", got.Lines[0])
	}
}

// BR-R-18: cash only, nearest Rp 100, half-up.
func TestRoundCash(t *testing.T) {
	for _, tc := range []struct{ in, want, rounding int64 }{
		{186_450, 186_500, 50}, {186_449, 186_400, -49}, {186_400, 186_400, 0}, {74_613, 74_600, -13}, {50, 100, 50},
	} {
		got, r := RoundCash(tc.in)
		if got != tc.want || r != tc.rounding {
			t.Errorf("RoundCash(%d) = %d, %d; want %d, %d", tc.in, got, r, tc.want, tc.rounding)
		}
	}
}

// TR-PR-04: equal split puts the remainder on the last part.
func TestSplitEqually(t *testing.T) {
	got := SplitEqually(100_001, 3)
	want := []int64{33_333, 33_333, 33_335}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestProrateBillDiscountSumsExactly(t *testing.T) {
	parts := ProrateBillDiscount(25_000, []int64{100_000, 133_333, 66_667})
	var sum int64
	for _, p := range parts {
		sum += p
	}
	if sum != 25_000 {
		t.Fatalf("parts %v sum %d", parts, sum)
	}
}

func TestNetFromInclusive(t *testing.T) {
	// 43 890 inclusive of 5 % service and 10 % tax is 38 000 net (inverse of the 38 000 case above).
	if got := NetFromInclusive(43_890, settings); got != 38_000 {
		t.Fatalf("got %d", got)
	}
}

func TestScheduleActiveAt(t *testing.T) {
	hh := Schedule{StartMinute: ptr(14 * 60), EndMinute: ptr(17 * 60)}
	weekdays := Schedule{DaysMask: 0b0011111}
	overnight := Schedule{StartMinute: ptr(22 * 60), EndMinute: ptr(2 * 60)}
	// 2026-10-26 is a Monday.
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, wib) }
	cases := []struct {
		name string
		s    Schedule
		t    time.Time
		want bool
	}{
		{"happy hour start inclusive", hh, at(26, 14, 0), true},
		{"happy hour end exclusive", hh, at(26, 17, 0), false},
		{"before happy hour", hh, at(26, 13, 59), false},
		{"weekday on Monday", weekdays, at(26, 9, 0), true},
		{"weekday on Saturday", weekdays, at(31, 9, 0), false},
		{"overnight late", overnight, at(26, 23, 0), true},
		{"overnight early", overnight, at(27, 1, 30), true},
		{"overnight midday", overnight, at(27, 12, 0), false},
		{"valid to exclusive", Schedule{ValidTo: ptr(at(27, 0, 0))}, at(27, 0, 0), false},
	}
	for _, c := range cases {
		if got := c.s.ActiveAt(c.t, wib); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
