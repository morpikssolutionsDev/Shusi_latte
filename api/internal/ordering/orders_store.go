package ordering

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// validatedItem is a catalog-checked Order Item ready to insert.
type validatedItem struct {
	input         OrderItemInput
	productName   string
	variantName   string
	unitPrice     int64
	prepTime      int
	modifiers     []OrderItemModifier
	warningTagIDs []string
}

func (s *Store) validateItems(customer Customer, inputs []OrderItemInput) ([]validatedItem, error) {
	if len(inputs) == 0 {
		return nil, errors.New("order must contain at least one item")
	}
	restriction := map[string]bool{}
	for _, t := range customer.RestrictionTags {
		restriction[t] = true
	}
	out := []validatedItem{}
	for _, in := range inputs {
		if in.Quantity <= 0 {
			return nil, errors.New("item quantity must be > 0")
		}
		var productNameID, productNameEN, status string
		var prepTime int
		err := s.db.QueryRow(`SELECT name_id, name_en, status, prep_time_seconds FROM product WHERE id = ?`, in.ProductID).Scan(&productNameID, &productNameEN, &status, &prepTime)
		if err != nil {
			return nil, fmt.Errorf("product not found: %s", in.ProductID)
		}
		if status != "active" {
			return nil, fmt.Errorf("product is not active: %s", in.ProductID)
		}
		var variantNameID, variantNameEN string
		var price int64
		var soldOut, archived int
		err = s.db.QueryRow(`SELECT name_id, name_en, price, sold_out, archived_at IS NOT NULL FROM variant WHERE id = ? AND product_id = ?`, in.VariantID, in.ProductID).Scan(&variantNameID, &variantNameEN, &price, &soldOut, &archived)
		if err != nil {
			return nil, fmt.Errorf("variant not found for product: %s", in.VariantID)
		}
		if archived == 1 {
			return nil, fmt.Errorf("variant is archived: %s", in.VariantID)
		}
		if soldOut == 1 {
			return nil, fmt.Errorf("variant is sold out: %s", in.VariantID)
		}
		// Modifiers: must belong to this Product through a group,
		// must not be archived, and group min/max must hold.
		mods := []OrderItemModifier{}
		seenGroups := map[string]int{}
		groupMin := map[string]int{}
		groupMax := map[string]int{}
		for _, modID := range in.ModifierIDs {
			var modNameID, modNameEN string
			var modPrice int64
			var groupID string
			var minSel, maxSel int
			var modArchived int
			err := s.db.QueryRow(
				`SELECT m.name_id, m.name_en, m.price, mg.id, mg.min_select, mg.max_select, m.archived_at IS NOT NULL
				 FROM modifier m JOIN modifier_group mg ON mg.id = m.modifier_group_id
				 WHERE m.id = ? AND mg.product_id = ?`,
				modID, in.ProductID,
			).Scan(&modNameID, &modNameEN, &modPrice, &groupID, &minSel, &maxSel, &modArchived)
			if err != nil {
				return nil, fmt.Errorf("modifier not found for product: %s", modID)
			}
			if modArchived == 1 {
				return nil, fmt.Errorf("modifier is archived: %s", modID)
			}
			mods = append(mods, OrderItemModifier{ModifierID: modID, NameSnapshot: pickName(modNameID, modNameEN), Price: modPrice})
			seenGroups[groupID]++
			groupMin[groupID] = minSel
			groupMax[groupID] = maxSel
		}
		for gid, count := range seenGroups {
			if count > groupMax[gid] {
				return nil, fmt.Errorf("too many modifiers selected in group %s", gid)
			}
		}
		// Min-select applies to every group of the Product, even
		// when the Customer selected nothing from it.
		rows, err := s.db.Query(`SELECT id, min_select FROM modifier_group WHERE product_id = ?`, in.ProductID)
		if err == nil {
			var gid string
			var minSel int
			for rows.Next() {
				if err := rows.Scan(&gid, &minSel); err == nil && minSel > 0 && seenGroups[gid] < minSel {
					rows.Close()
					return nil, fmt.Errorf("modifier group %s requires at least %d selection(s)", gid, minSel)
				}
			}
			rows.Close()
		}
		// Dietary Warnings (TR-DM-05): Product tags union Modifier
		// tags, intersected with the Customer's restriction.
		tagSet := map[string]bool{}
		rows, err = s.db.Query(`SELECT dietary_tag_id FROM product_dietary_tag WHERE product_id = ?`, in.ProductID)
		if err == nil {
			var tid string
			for rows.Next() {
				if err := rows.Scan(&tid); err == nil {
					tagSet[tid] = true
				}
			}
			rows.Close()
		}
		for _, modID := range in.ModifierIDs {
			rows, err = s.db.Query(`SELECT dietary_tag_id FROM modifier_dietary_tag WHERE modifier_id = ?`, modID)
			if err == nil {
				var tid string
				for rows.Next() {
					if err := rows.Scan(&tid); err == nil {
						tagSet[tid] = true
					}
				}
				rows.Close()
			}
		}
		warnings := []string{}
		for tid := range tagSet {
			if restriction[tid] {
				warnings = append(warnings, tid)
			}
		}
		out = append(out, validatedItem{
			input:         in,
			productName:   pickName(productNameID, productNameEN),
			variantName:   pickName(variantNameID, variantNameEN),
			unitPrice:     price,
			prepTime:      prepTime,
			modifiers:     mods,
			warningTagIDs: warnings,
		})
	}
	return out, nil
}

func pickName(nameID, nameEN string) string {
	// Staff modules are in Indonesian (BR-NF-03); snapshots use
	// the Indonesian name when present, else English.
	if nameID != "" {
		return nameID
	}
	return nameEN
}

func (s *Store) buildWarnings(items []validatedItem) map[string][]DietaryWarning {
	warnings := map[string][]DietaryWarning{}
	for idx, vi := range items {
		if len(vi.warningTagIDs) == 0 {
			continue
		}
		ws := []DietaryWarning{}
		for _, tid := range vi.warningTagIDs {
			var nameID, nameEN string
			_ = s.db.QueryRow(`SELECT name_id, name_en FROM dietary_tag WHERE id = ?`, tid).Scan(&nameID, &nameEN)
			ws = append(ws, DietaryWarning{DietaryTagID: tid, TagNameID: nameID, TagNameEN: nameEN, Acknowledged: false})
		}
		warnings[fmt.Sprintf("items[%d]", idx)] = ws
	}
	return warnings
}

func (s *Store) insertItems(tx *sql.Tx, orderID string, items []validatedItem, acknowledge bool, now int64) error {
	for _, vi := range items {
		itemID := NewID()
		if _, err := tx.Exec(
			`INSERT INTO order_item (id, order_id, product_id, variant_id, product_name_snapshot, variant_name_snapshot, quantity, unit_price, note, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
			itemID, orderID, vi.input.ProductID, vi.input.VariantID, vi.productName, vi.variantName, vi.input.Quantity, vi.unitPrice, nullString(vi.input.Note),
		); err != nil {
			return err
		}
		for _, m := range vi.modifiers {
			if _, err := tx.Exec(`INSERT INTO order_item_modifier (order_item_id, modifier_id, name_snapshot, price) VALUES (?, ?, ?, ?)`, itemID, m.ModifierID, m.NameSnapshot, m.Price); err != nil {
				return err
			}
		}
		for _, tid := range vi.warningTagIDs {
			var ack sql.NullInt64
			if acknowledge {
				ack = sql.NullInt64{Int64: now, Valid: true}
			}
			if _, err := tx.Exec(`INSERT INTO order_item_dietary_warning (order_item_id, dietary_tag_id, acknowledged_at) VALUES (?, ?, ?)`, itemID, tid, ack); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateOrder places a Pending Order on the guest's Bill
// (BR-CM-05, BR-CM-06, BR-R-06). The Bill must be open; settling
// blocks with 409 (BR-CM-15, TR-SM-02). If any item carries a
// Dietary Warning the Customer has not acknowledged, no Order is
// created and a WarningError is returned (BR-CM-11).
func (s *Store) CreateOrder(customer Customer, in CreateOrderInput) (Order, error) {
	bill, err := s.loadBill(customer.BillID)
	if err != nil {
		return Order{}, err
	}
	if bill.Status == "settling" {
		return Order{}, ErrBillSettling
	}
	if bill.Status != "open" {
		return Order{}, ErrBillClosed
	}
	items, err := s.validateItems(customer, in.Items)
	if err != nil {
		return Order{}, err
	}
	if !in.AcknowledgeWarnings {
		if warnings := s.buildWarnings(items); len(warnings) > 0 {
			return Order{}, &WarningError{Warnings: warnings}
		}
	}
	now := time.Now().UnixMilli()
	orderID := NewID()
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO orders (id, bill_id, customer_entry_id, source, status, created_at) VALUES (?, ?, ?, 'client_menu', 'pending', ?)`,
		orderID, customer.BillID, customer.ID, now,
	); err != nil {
		return Order{}, err
	}
	if err := s.insertItems(tx, orderID, items, in.AcknowledgeWarnings, now); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.loadOrder(orderID)
}

func (s *Store) loadOrder(orderID string) (Order, error) {
	var o Order
	var claimedBy, createdBy sql.NullString
	var claimedAt, confirmedAt sql.NullInt64
	err := s.db.QueryRow(`SELECT id, bill_id, customer_entry_id, source, status, claimed_by, claimed_at, confirmed_at, created_at FROM orders WHERE id = ?`, orderID).Scan(&o.ID, &o.BillID, &o.CustomerEntryID, &o.Source, &o.Status, &claimedBy, &claimedAt, &confirmedAt, &o.CreatedAt)
	if err != nil {
		return Order{}, ErrOrderNotFound
	}
	// created_by is added by migration 0009; read it separately
	// so older test databases without the column still load.
	_ = s.db.QueryRow(`SELECT created_by FROM orders WHERE id = ?`, orderID).Scan(&createdBy)
	o.ClaimedBy = strPtr(claimedBy)
	o.ClaimedAt = intPtr(claimedAt)
	o.ConfirmedAt = intPtr(confirmedAt)
	if o.CustomerEntryID != nil {
		if c, err := s.loadCustomer(*o.CustomerEntryID); err == nil {
			o.PlacedBy = c.Name
		}
	}
	if o.PlacedBy == "" && createdBy.Valid {
		o.PlacedBy = createdBy.String
	}
	rows, err := s.db.Query(`SELECT id, product_id, variant_id, product_name_snapshot, variant_name_snapshot, quantity, unit_price, note, status FROM order_item WHERE order_id = ? ORDER BY rowid`, orderID)
	if err != nil {
		return Order{}, err
	}
	var itemIDs []string
	var items []OrderItem
	for rows.Next() {
		var it OrderItem
		var note sql.NullString
		if err := rows.Scan(&it.ID, &it.ProductID, &it.VariantID, &it.ProductName, &it.VariantName, &it.Quantity, &it.UnitPrice, &note, &it.Status); err == nil {
			it.OrderID = orderID
			it.Note = strPtr(note)
			itemIDs = append(itemIDs, it.ID)
			items = append(items, it)
		}
	}
	rows.Close()
	for i, it := range items {
		// Modifiers
		mrows, err := s.db.Query(`SELECT modifier_id, name_snapshot, price FROM order_item_modifier WHERE order_item_id = ?`, it.ID)
		if err == nil {
			mods := []OrderItemModifier{}
			for mrows.Next() {
				var m OrderItemModifier
				if err := mrows.Scan(&m.ModifierID, &m.NameSnapshot, &m.Price); err == nil {
					mods = append(mods, m)
				}
			}
			mrows.Close()
			items[i].Modifiers = mods
		}
		// Warnings
		wrows, err := s.db.Query(`SELECT w.dietary_tag_id, t.name_id, t.name_en, w.acknowledged_at FROM order_item_dietary_warning w JOIN dietary_tag t ON t.id = w.dietary_tag_id WHERE w.order_item_id = ?`, it.ID)
		if err == nil {
			ws := []DietaryWarning{}
			for wrows.Next() {
				var w DietaryWarning
				var ack sql.NullInt64
				if err := wrows.Scan(&w.DietaryTagID, &w.TagNameID, &w.TagNameEN, &ack); err == nil {
					w.Acknowledged = ack.Valid
					ws = append(ws, w)
				}
			}
			wrows.Close()
			items[i].DietaryWarnings = ws
		}
		// Prep Time from the current Product row (estimate only).
		_ = s.db.QueryRow(`SELECT prep_time_seconds FROM product WHERE id = ?`, it.ProductID).Scan(&items[i].PrepTimeSeconds)
		items[i].LineTotal = items[i].computeLineTotal()
	}
	if items == nil {
		items = []OrderItem{}
	}
	o.Items = items
	o.EstimatedWaitMinutes = o.estimatedWait()
	return o, nil
}

// LoadOrderForGuest loads an Order only if it belongs to the
// requesting Customer.
func (s *Store) LoadOrderForGuest(customer Customer, orderID string) (Order, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return Order{}, err
	}
	if o.CustomerEntryID == nil || *o.CustomerEntryID != customer.ID {
		return Order{}, ErrNotOrderOwner
	}
	return o, nil
}

// ListOrdersForCustomer returns a Customer's Orders on their Bill,
// newest first.
func (s *Store) ListOrdersForCustomer(customer Customer) []Order {
	rows, err := s.db.Query(`SELECT id FROM orders WHERE customer_entry_id = ? AND bill_id = ? ORDER BY created_at DESC`, customer.ID, customer.BillID)
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

// PatchOrder replaces the items of a Customer's own Pending,
// unclaimed Order (BR-CM-14, TR-SM-01). Claimed or non-pending
// Orders are rejected, so a Customer must ask the Waiter then.
func (s *Store) PatchOrder(customer Customer, orderID string, in CreateOrderInput) (Order, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return Order{}, err
	}
	if o.CustomerEntryID == nil || *o.CustomerEntryID != customer.ID {
		return Order{}, ErrNotOrderOwner
	}
	if o.Status != "pending" {
		return Order{}, ErrOrderNotPending
	}
	if o.ClaimedBy != nil {
		return Order{}, ErrOrderClaimed
	}
	items, err := s.validateItems(customer, in.Items)
	if err != nil {
		return Order{}, err
	}
	if !in.AcknowledgeWarnings {
		if warnings := s.buildWarnings(items); len(warnings) > 0 {
			return Order{}, &WarningError{Warnings: warnings}
		}
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	// Delete old items and their modifiers/warnings first.
	var oldIDs []string
	rows, err := tx.Query(`SELECT id FROM order_item WHERE order_id = ?`, orderID)
	if err != nil {
		return Order{}, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			oldIDs = append(oldIDs, id)
		}
	}
	rows.Close()
	for _, id := range oldIDs {
		if _, err := tx.Exec(`DELETE FROM order_item_dietary_warning WHERE order_item_id = ?`, id); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(`DELETE FROM order_item_modifier WHERE order_item_id = ?`, id); err != nil {
			return Order{}, err
		}
	}
	if _, err := tx.Exec(`DELETE FROM order_item WHERE order_id = ?`, orderID); err != nil {
		return Order{}, err
	}
	if err := s.insertItems(tx, orderID, items, in.AcknowledgeWarnings, now); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.loadOrder(orderID)
}

// CancelOrder cancels a Customer's own Pending, unclaimed Order
// (BR-CM-14, BR-R-07). Items are marked cancelled with the reason;
// a Cancel with reason out_of_stock is a Waiter/Kitchen action in
// Phase 4, not a Customer action, so it does not set Sold Out here.
func (s *Store) CancelOrder(customer Customer, orderID, reason string) (Order, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return Order{}, err
	}
	if o.CustomerEntryID == nil || *o.CustomerEntryID != customer.ID {
		return Order{}, ErrNotOrderOwner
	}
	if o.Status != "pending" {
		return Order{}, ErrOrderNotPending
	}
	if o.ClaimedBy != nil {
		return Order{}, ErrOrderClaimed
	}
	if reason == "" {
		reason = "customer_request"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	// Optimistic concurrency (TR-SM-02): only cancel if still pending.
	res, err := tx.Exec(`UPDATE orders SET status = 'cancelled' WHERE id = ? AND status = 'pending'`, orderID)
	if err != nil {
		return Order{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Order{}, ErrOrderNotPending
	}
	if _, err := tx.Exec(`UPDATE order_item SET status = 'cancelled', cancel_reason = ?, cancelled_by = ? WHERE order_id = ? AND status = 'pending'`, reason, customer.ID, orderID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.loadOrder(orderID)
}
