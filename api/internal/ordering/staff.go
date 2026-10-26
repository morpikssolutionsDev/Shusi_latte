package ordering

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crypto"
)

// Staff operations for Phase 4 Waiter (BR-WT-01 to BR-WT-06,
// BR-WT-09, BR-WT-11, BR-WT-12, BR-WT-13, BR-WT-15, BR-R-34,
// TR-SM-01 to TR-SM-05). Payment recording and Bill closing are
// Phase 6, shared by Waiter and Cashier (BR-CS-01); this package
// provides summary, settle/unlock, move, merge and split here.

func (s *Store) audit(actor, action, entity, entityID, reason string) {
	_, _ = s.db.Exec(
		`INSERT INTO audit_log (id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before_json, after_json, reason)
		 VALUES (?, ?, ?, NULL, ?, ?, ?, NULL, NULL, ?)`,
		NewID(), time.Now().UnixMilli(), actor, action, entity, entityID, nullString(&reason),
	)
}

func (s *Store) recomputeOrderStatus(tx *sql.Tx, orderID string) {
	rows, err := tx.Query(`SELECT status FROM order_item WHERE order_id = ?`, orderID)
	if err != nil {
		return
	}
	var statuses []string
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err == nil {
			statuses = append(statuses, st)
		}
	}
	rows.Close()
	nonCancelled := []string{}
	for _, st := range statuses {
		if st != "cancelled" && st != "voided" {
			nonCancelled = append(nonCancelled, st)
		}
	}
	var orderStatus string
	switch {
	case len(nonCancelled) == 0:
		orderStatus = "cancelled"
	case allEqual(nonCancelled, "served"):
		orderStatus = "served"
	case allIn(nonCancelled, []string{"ready", "served"}):
		orderStatus = "ready"
	case allIn(nonCancelled, []string{"confirmed", "preparing", "ready", "served"}):
		orderStatus = "confirmed"
	default:
		return // pending stays pending
	}
	_, _ = tx.Exec(`UPDATE orders SET status = ? WHERE id = ?`, orderStatus, orderID)
}

func allEqual(list []string, v string) bool {
	for _, x := range list {
		if x != v {
			return false
		}
	}
	return len(list) > 0
}

func allIn(list, allowed []string) bool {
	if len(list) == 0 {
		return false
	}
	for _, x := range list {
		ok := false
		for _, a := range allowed {
			if x == a {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// ClaimOrder is first-writer-wins (TR-SM-03): only a Pending,
// unclaimed Order can be claimed, and the update is bound to
// claimed_by IS NULL so a losing Waiter gets 409.
func (s *Store) ClaimOrder(crewID, orderID string) (Order, error) {
	res, err := s.db.Exec(`UPDATE orders SET claimed_by = ?, claimed_at = ? WHERE id = ? AND status = 'pending' AND claimed_by IS NULL`, crewID, time.Now().UnixMilli(), orderID)
	if err != nil {
		return Order{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Order{}, errors.New("order cannot be claimed: already claimed or not pending")
	}
	s.audit(crewID, "claim", "order", orderID, "")
	return s.loadOrder(orderID)
}

// TakeoverOrder lets another Waiter take a Claim (BR-WT-11).
func (s *Store) TakeoverOrder(crewID, orderID string) (Order, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return Order{}, err
	}
	if o.Status != "pending" || o.ClaimedBy == nil {
		return Order{}, errors.New("order has no claim to take over")
	}
	if *o.ClaimedBy == crewID {
		return o, nil
	}
	if _, err := s.db.Exec(`UPDATE orders SET claimed_by = ?, claimed_at = ? WHERE id = ?`, crewID, time.Now().UnixMilli(), orderID); err != nil {
		return Order{}, err
	}
	s.audit(crewID, "claim_transfer", "order", orderID, "takeover")
	return s.loadOrder(orderID)
}

// ReassignOrder is the Outlet Admin version of takeover.
func (s *Store) ReassignOrder(adminCrewID, orderID, toCrewID string) (Order, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return Order{}, err
	}
	if o.Status != "pending" {
		return Order{}, errors.New("only a pending order can be reassigned")
	}
	if _, err := s.db.Exec(`UPDATE orders SET claimed_by = ?, claimed_at = ? WHERE id = ?`, toCrewID, time.Now().UnixMilli(), orderID); err != nil {
		return Order{}, err
	}
	s.audit(adminCrewID, "claim_transfer", "order", orderID, "reassign to "+toCrewID)
	return s.loadOrder(orderID)
}

// ConfirmOrder confirms a Claimed Order and sends it to the
// Kitchen (BR-WT-02, BR-R-06). If items are supplied, they
// replace the Order's items first (Waiter adjustment), with
// the same validation as a Customer Order, against the Order's
// applicable restriction.
func (s *Store) ConfirmOrder(crewID, orderID string, items *[]OrderItemInput) (Order, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return Order{}, err
	}
	if o.Status != "pending" {
		return Order{}, errors.New("only a pending order can be confirmed")
	}
	if o.ClaimedBy == nil || *o.ClaimedBy != crewID {
		return Order{}, errors.New("only the claiming waiter can confirm this order")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if items != nil {
		customer := Customer{BillID: o.BillID}
		if o.CustomerEntryID != nil {
			if c, err := s.loadCustomer(*o.CustomerEntryID); err == nil {
				customer = c
			}
		}
		validated, err := s.validateItems(customer, *items)
		if err != nil {
			return Order{}, err
		}
		// Delete old items.
		var oldIDs []string
		rows, _ := tx.Query(`SELECT id FROM order_item WHERE order_id = ?`, orderID)
		if rows != nil {
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					oldIDs = append(oldIDs, id)
				}
			}
			rows.Close()
		}
		for _, id := range oldIDs {
			_, _ = tx.Exec(`DELETE FROM order_item_dietary_warning WHERE order_item_id = ?`, id)
			_, _ = tx.Exec(`DELETE FROM order_item_modifier WHERE order_item_id = ?`, id)
		}
		_, _ = tx.Exec(`DELETE FROM order_item WHERE order_id = ?`, orderID)
		now := time.Now().UnixMilli()
		if err := s.insertItems(tx, orderID, validated, true, now); err != nil {
			return Order{}, err
		}
	}
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(`UPDATE orders SET status = 'confirmed', confirmed_at = ? WHERE id = ? AND status = 'pending'`, now, orderID); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(`UPDATE order_item SET status = 'confirmed' WHERE order_id = ? AND status = 'pending'`, orderID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	s.audit(crewID, "confirm", "order", orderID, "")
	return s.loadOrder(orderID)
}

// StaffCreateOrder enters an Order directly for a Table
// (BR-WT-03). Customer name is optional: when given, a
// customer_entry is created for the Bill so the name shows on
// the Bill, with consent recorded as staff-entered; otherwise
// the Order carries only the Waiter-ticked restriction, stored
// encrypted on the Order (TR-DM-05, BR-R-D1). Staff Orders are
// created Confirmed, so they go straight to the Kitchen, and
// warnings are recorded but do not block.
func (s *Store) StaffCreateOrder(crewID, tableID string, customerName *string, restrictionTags []string, inputs []OrderItemInput) (Order, error) {
	bill, err := s.OpenOrJoinBill(tableID)
	if err != nil {
		return Order{}, err
	}
	if bill.Status != "open" {
		return Order{}, ErrBillClosed
	}
	customer := Customer{BillID: bill.ID, RestrictionTags: restrictionTags}
	var customerEntryID *string
	if customerName != nil && *customerName != "" {
		// A name-only entry, so the name shows on the Bill.
		// The Waiter-ticked restriction belongs to the Order,
		// not to this entry, so it is stored on the Order below
		// and the entry's restriction list stays empty.
		now := time.Now()
		keyID := crypto.TodayKeyID(now)
		nameEnc, err := s.km.Encrypt(keyID, *customerName)
		if err != nil {
			return Order{}, err
		}
		tagsEnc, err := s.km.Encrypt(keyID, "[]")
		if err != nil {
			return Order{}, err
		}
		id := NewID()
		if _, err := s.db.Exec(
			`INSERT INTO customer_entry (id, bill_id, key_id, name_enc, dietary_note_enc, restriction_tags_enc, consent_at, created_at) VALUES (?, ?, ?, ?, NULL, ?, ?, ?)`,
			id, bill.ID, keyID, nameEnc, tagsEnc, now.UnixMilli(), now.UnixMilli(),
		); err != nil {
			return Order{}, err
		}
		customerEntryID = &id
		customer.ID = id
		customer.Name = *customerName
	}
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
	var keyID *string
	var tagsEnc *string
	if len(restrictionTags) > 0 {
		kid := crypto.TodayKeyID(time.Now())
		enc, err := s.km.Encrypt(kid, marshalStrings(restrictionTags))
		if err != nil {
			return Order{}, err
		}
		keyID = &kid
		tagsEnc = &enc
	}
	if _, err := tx.Exec(
		`INSERT INTO orders (id, bill_id, customer_entry_id, source, status, claimed_by, claimed_at, confirmed_at, key_id, restriction_tags_enc, created_by, created_at) VALUES (?, ?, ?, 'waiter', 'confirmed', ?, ?, ?, ?, ?, ?, ?)`,
		orderID, bill.ID, nullString(customerEntryID), crewID, now, now, nullString(keyID), nullString(tagsEnc), crewID, now,
	); err != nil {
		return Order{}, err
	}
	if err := s.insertItems(tx, orderID, validated, true, now); err != nil {
		return Order{}, err
	}
	// Staff Orders are Confirmed immediately.
	if _, err := tx.Exec(`UPDATE order_item SET status = 'confirmed' WHERE order_id = ?`, orderID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	s.audit(crewID, "create", "order", orderID, "waiter direct entry")
	return s.loadOrder(orderID)
}

func marshalStrings(list []string) string {
	out := "["
	for i, v := range list {
		if i > 0 {
			out += ","
		}
		out += `"` + v + `"`
	}
	return out + "]"
}

// CancelItem cancels a Pending or Confirmed item (BR-WT-04,
// BR-R-07). Out-of-stock cancels set the Variant Sold Out and
// flag other open items with that Variant (TR-SM-04).
func (s *Store) CancelItem(crewID, itemID, reason string) (Order, error) {
	if reason == "" {
		return Order{}, errors.New("cancel reason is required")
	}
	var orderID, variantID, status string
	err := s.db.QueryRow(`SELECT order_id, variant_id, status FROM order_item WHERE id = ?`, itemID).Scan(&orderID, &variantID, &status)
	if err != nil {
		return Order{}, errors.New("order item not found")
	}
	if status != "pending" && status != "confirmed" {
		return Order{}, errors.New("only pending or confirmed items can be cancelled")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE order_item SET status = 'cancelled', cancel_reason = ?, cancelled_by = ? WHERE id = ? AND status IN ('pending','confirmed')`, reason, crewID, itemID); err != nil {
		return Order{}, err
	}
	if reason == "out_of_stock" {
		_, _ = tx.Exec(`UPDATE variant SET sold_out = 1, sold_out_at = ?, sold_out_by = ?, sold_out_source = 'cancel_out_of_stock' WHERE id = ?`, time.Now().UnixMilli(), crewID, variantID)
		_, _ = tx.Exec(`UPDATE order_item SET sold_out_flag = 1 WHERE variant_id = ? AND status IN ('pending','confirmed') AND id != ?`, variantID, itemID)
	}
	s.recomputeOrderStatus(tx, orderID)
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	s.audit(crewID, "cancel", "order_item", itemID, reason)
	return s.loadOrder(orderID)
}

// ServeItem marks a Ready item Served (BR-WT-09).
func (s *Store) ServeItem(crewID, itemID string) (Order, error) {
	var orderID, status string
	err := s.db.QueryRow(`SELECT order_id, status FROM order_item WHERE id = ?`, itemID).Scan(&orderID, &status)
	if err != nil {
		return Order{}, errors.New("order item not found")
	}
	if status != "ready" {
		return Order{}, errors.New("only ready items can be served")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE order_item SET status = 'served' WHERE id = ? AND status = 'ready'`, itemID); err != nil {
		return Order{}, err
	}
	s.recomputeOrderStatus(tx, orderID)
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.loadOrder(orderID)
}

// VoidItem voids a Ready or Served item on an unpaid Bill
// (BR-R-08). The caller must have consumed an override token;
// this function records the approver.
func (s *Store) VoidItem(crewID, approverCrewID, itemID, reason string) (Order, error) {
	if reason == "" {
		return Order{}, errors.New("void reason is required")
	}
	var orderID, billID, status string
	err := s.db.QueryRow(`SELECT oi.order_id, o.bill_id, oi.status FROM order_item oi JOIN orders o ON o.id = oi.order_id WHERE oi.id = ?`, itemID).Scan(&orderID, &billID, &status)
	if err != nil {
		return Order{}, errors.New("order item not found")
	}
	if status != "ready" && status != "served" {
		return Order{}, errors.New("only ready or served items can be voided")
	}
	bill, err := s.loadBill(billID)
	if err != nil || (bill.Status != "open" && bill.Status != "settling") {
		return Order{}, errors.New("void requires an unpaid bill")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE order_item SET status = 'voided', void_reason = ?, voided_by = ?, voided_override_by = ? WHERE id = ?`, reason, crewID, approverCrewID, itemID); err != nil {
		return Order{}, err
	}
	s.recomputeOrderStatus(tx, orderID)
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	s.audit(crewID, "void", "order_item", itemID, reason)
	return s.loadOrder(orderID)
}

// RemakeItem creates a no-charge Confirmed copy of a Served
// item, prioritised in the Kitchen queue (BR-WT-12, BR-R-33,
// TR-DM-07).
func (s *Store) RemakeItem(crewID, approverCrewID, itemID, reason string) (Order, error) {
	if reason == "" {
		return Order{}, errors.New("remake reason is required")
	}
	var orderID, productID, variantID, productName, variantName, status string
	var qty int
	var unitPrice int64
	var note sql.NullString
	err := s.db.QueryRow(`SELECT order_id, product_id, variant_id, product_name_snapshot, variant_name_snapshot, quantity, unit_price, note, status FROM order_item WHERE id = ?`, itemID).Scan(&orderID, &productID, &variantID, &productName, &variantName, &qty, &unitPrice, &note, &status)
	if err != nil {
		return Order{}, errors.New("order item not found")
	}
	if status != "served" {
		return Order{}, errors.New("only served items can be remade")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	newID := NewID()
	if _, err := tx.Exec(
		`INSERT INTO order_item (id, order_id, product_id, variant_id, product_name_snapshot, variant_name_snapshot, quantity, unit_price, note, status, remake_of_id, remake_reason, remake_override_by) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, 'confirmed', ?, ?, ?)`,
		newID, orderID, productID, variantID, productName, variantName, qty, nullString(strPtr(note)), itemID, reason, approverCrewID,
	); err != nil {
		return Order{}, err
	}
	// Copy modifiers at zero price.
	rows, _ := tx.Query(`SELECT modifier_id, name_snapshot FROM order_item_modifier WHERE order_item_id = ?`, itemID)
	if rows != nil {
		for rows.Next() {
			var modID, modName string
			if err := rows.Scan(&modID, &modName); err == nil {
				_, _ = tx.Exec(`INSERT INTO order_item_modifier (order_item_id, modifier_id, name_snapshot, price) VALUES (?, ?, ?, 0)`, newID, modID, modName)
			}
		}
		rows.Close()
	}
	// Copy warnings.
	rows, _ = tx.Query(`SELECT dietary_tag_id FROM order_item_dietary_warning WHERE order_item_id = ?`, itemID)
	if rows != nil {
		for rows.Next() {
			var tid string
			if err := rows.Scan(&tid); err == nil {
				_, _ = tx.Exec(`INSERT INTO order_item_dietary_warning (order_item_id, dietary_tag_id, acknowledged_at) VALUES (?, ?, ?)`, newID, tid, time.Now().UnixMilli())
			}
		}
		rows.Close()
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	s.audit(crewID, "remake", "order_item", newID, reason)
	return s.loadOrder(orderID)
}

// ListOpenClaims returns a Waiter's open Claims (BR-WT-11).
func (s *Store) ListOpenClaims(crewID string) []Order {
	rows, err := s.db.Query(`SELECT id FROM orders WHERE claimed_by = ? AND status IN ('pending','confirmed') ORDER BY created_at`, crewID)
	if err != nil {
		return []Order{}
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []Order{}
	for _, id := range ids {
		if o, err := s.loadOrder(id); err == nil {
			out = append(out, o)
		}
	}
	return out
}

var _ = fmt.Sprintf
