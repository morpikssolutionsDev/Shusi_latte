package ordering

import (
	"errors"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
)

// Takeaway Orders are created Pending on the Takeaway Bill
// (BR-CS-02) and only Confirmed to the Kitchen after full
// payment, via ConfirmTakeawayOrders.
func (s *Store) CreateTakeawayOrder(crewID, billID string, restrictionTags []string, inputs []OrderItemInput) (Order, error) {
	bill, err := s.loadBill(billID)
	if err != nil {
		return Order{}, err
	}
	if bill.Type != "takeaway" {
		return Order{}, errors.New("only takeaway bills use this endpoint")
	}
	if bill.Status != "open" && bill.Status != "settling" {
		return Order{}, ErrBillClosed
	}
	customer := Customer{BillID: billID, RestrictionTags: restrictionTags}
	validated, err := s.validateItems(customer, inputs)
	if err != nil {
		return Order{}, err
	}
	now := time.Now().UnixMilli()
	orderID := NewID()
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO orders (id, bill_id, source, status, created_by, created_at) VALUES (?, ?, 'cashier', 'pending', ?, ?)`,
		orderID, billID, crewID, now,
	); err != nil {
		return Order{}, err
	}
	if err := s.insertItems(tx, orderID, validated, true, now); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.loadOrder(orderID)
}

// ConfirmTakeawayOrders confirms all Pending Orders on a
// Takeaway Bill after it is fully paid.
func (s *Store) ConfirmTakeawayOrders(crewID, billID string, pricingStore *pricing.Store) ([]Order, error) {
	summary, err := s.BillPaymentSummary(billID, pricingStore)
	if err != nil {
		return nil, err
	}
	if summary.Balance != 0 {
		return nil, errors.New("takeaway bill is not fully paid")
	}
	rows, err := s.db.Query(`SELECT id FROM orders WHERE bill_id = ? AND status = 'pending'`, billID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	now := time.Now().UnixMilli()
	for _, id := range ids {
		if _, err := s.db.Exec(`UPDATE orders SET status = 'confirmed', confirmed_at = ?, claimed_by = ? WHERE id = ?`, now, crewID, id); err != nil {
			return nil, err
		}
		if _, err := s.db.Exec(`UPDATE order_item SET status = 'confirmed' WHERE order_id = ? AND status = 'pending'`, id); err != nil {
			return nil, err
		}
	}
	out := []Order{}
	for _, id := range ids {
		if o, err := s.loadOrder(id); err == nil {
			out = append(out, o)
		}
	}
	return out, nil
}
