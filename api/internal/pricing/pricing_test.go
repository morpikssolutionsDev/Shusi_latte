package pricing

import "testing"

func TestCalculateOrder(t *testing.T) {
	// BR-R-14 order: subtotal 100000, item discount 10000, bill discount
	// 5000, service 0, tax 10% on 85000 = 8500, total 93500.
	items := []ItemInput{
		{VariantPrice: 45000, ModifierPrices: []int64{5000}, Quantity: 2, ItemDiscounts: []Discount{
			{Kind: KindPercent, Value: 10, Scope: ScopeItem},
			{Kind: KindNominal, Value: 5000, Scope: ScopeItem},
		}},
	}
	// Item amount = (45000+5000)*2 = 100000. Best item discount is
	// 10% = 10000, beating nominal 5000 (BR-R-16).
	bill := Discount{Kind: KindNominal, Value: 5000, Scope: ScopeBill}
	out, err := Calculate(items, &bill, Settings{TaxRateBP: 1000}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Subtotal != 100000 || out.ItemDiscountTotal != 10000 || out.BillDiscount != 5000 || out.Tax != 8500 || out.Total != 93500 {
		t.Fatalf("unexpected breakdown: %+v", out)
	}
}

func TestCalculateCashRounding(t *testing.T) {
	// BR-R-18: only cash is rounded to the nearest Rp 100.
	items := []ItemInput{{VariantPrice: 45123, Quantity: 1}}
	out, err := Calculate(items, nil, Settings{TaxRateBP: 0}, "cash")
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 45123 || out.TotalForCash != 45100 || out.CashRounding != -23 {
		t.Fatalf("unexpected cash rounding: %+v", out)
	}
	out, err = Calculate(items, nil, Settings{TaxRateBP: 0}, "card")
	if err != nil {
		t.Fatal(err)
	}
	if out.TotalForCash != 45123 || out.CashRounding != 0 {
		t.Fatalf("non-cash should not round: %+v", out)
	}
}

func TestDiscountAmountCapped(t *testing.T) {
	if amt := DiscountAmount(Discount{Kind: KindNominal, Value: 999999, Scope: ScopeBill}, 1000); amt != 1000 {
		t.Fatalf("discount should cap at base, got %d", amt)
	}
}
