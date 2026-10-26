package ordering

import (
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
)

// BillView is the Customer's view of their Bill (BR-CM-08,
// BR-CM-09, TR-KT-04). Prices are shown excluding tax and
// service, with a note that both are added; totals are computed
// with the pricing engine (BR-R-14) from current settings, with
// no discounts applied yet — discounts are recorded as
// applied_discount rows at payment/confirmation in later phases.
// Orders listed are the requesting Customer's own Orders; the
// totals cover the whole shared Bill, since the Bill is shared.
type BillView struct {
	Bill                 Bill     `json:"bill"`
	Customer             Customer `json:"customer"`
	Orders               []Order  `json:"orders"`
	Subtotal             int64    `json:"subtotal"`
	ServiceCharge        int64    `json:"service_charge"`
	Tax                  int64    `json:"tax"`
	Total                int64    `json:"total"`
	EstimatedWaitMinutes int      `json:"estimated_wait_minutes"`
	PriceNoteID          string   `json:"price_note_id"`
	PriceNoteEN          string   `json:"price_note_en"`
}

func (s *Store) BillView(customer Customer, pricingStore *pricing.Store) (BillView, error) {
	bill, err := s.loadBill(customer.BillID)
	if err != nil {
		return BillView{}, err
	}
	orders := s.ListOrdersForCustomer(customer)
	// Totals over all non-cancelled, non-voided items on the Bill,
	// rebuilt from loaded items so Modifier prices are included
	// (unit_price is Variant-only).
	var inputs []pricing.ItemInput
	allRows, err := s.db.Query(`SELECT id FROM orders WHERE bill_id = ?`, customer.BillID)
	if err == nil {
		var orderIDs []string
		for allRows.Next() {
			var id string
			if err := allRows.Scan(&id); err == nil {
				orderIDs = append(orderIDs, id)
			}
		}
		allRows.Close()
		for _, oid := range orderIDs {
			if o, err := s.loadOrder(oid); err == nil {
				for _, it := range o.Items {
					if it.Status == "cancelled" || it.Status == "voided" {
						continue
					}
					modPrices := []int64{}
					for _, m := range it.Modifiers {
						modPrices = append(modPrices, m.Price)
					}
					inputs = append(inputs, pricing.ItemInput{VariantPrice: it.UnitPrice, ModifierPrices: modPrices, Quantity: it.Quantity})
				}
			}
		}
	}
	settings, err := pricingStore.GetSettings()
	if err != nil {
		settings = pricing.DefaultSettings()
	}
	breakdown, err := pricing.Calculate(inputs, nil, settings, "")
	if err != nil {
		return BillView{}, err
	}
	maxWait := 0
	for _, o := range orders {
		if o.EstimatedWaitMinutes > maxWait {
			maxWait = o.EstimatedWaitMinutes
		}
	}
	return BillView{
		Bill: bill, Customer: customer, Orders: orders,
		Subtotal: breakdown.Subtotal, ServiceCharge: breakdown.ServiceCharge,
		Tax: breakdown.Tax, Total: breakdown.Total,
		EstimatedWaitMinutes: maxWait,
		PriceNoteID:          "Harga belum termasuk pajak dan biaya layanan, yang ditambahkan saat pembayaran.",
		PriceNoteEN:          "Prices exclude tax and service, which are added at payment.",
	}, nil
}
