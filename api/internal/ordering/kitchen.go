package ordering

import (
	"database/sql"
	"errors"
	"math"
	"time"
)

// Kitchen queue for Phase 5 (BR-KT-01 to BR-KT-06, TR-KT-01 to
// TR-KT-05). Each Prep Area has its own queue of Confirmed and
// Preparing items, Remakes first, then by confirmation time.
// Elapsed time is shown against Prep Time, amber at 80% and red
// at 100%. Ready items not picked up within the pickup reminder
// time are marked waiting pickup.

type QueueItem struct {
	OrderItem
	OrderID        string   `json:"order_id"`
	BillID         string   `json:"bill_id"`
	TableLabels    []string `json:"table_labels"`
	PlacedBy       string   `json:"placed_by"`
	PrepAreaID     string   `json:"prep_area_id"`
	ConfirmedAt    *int64   `json:"confirmed_at"`
	ElapsedSeconds int64    `json:"elapsed_seconds"`
	LateStatus     string   `json:"late_status"` // "" | "amber" | "red"
	WaitingPickup  bool     `json:"waiting_pickup"`
	IsRemake       bool     `json:"is_remake"`
	ReadyAt        *int64   `json:"ready_at"`
}

type Queue struct {
	PrepAreaID         string      `json:"prep_area_id"`
	Items              []QueueItem `json:"items"`
	ReadyWaitingPickup []QueueItem `json:"ready_waiting_pickup"`
}

// KitchenQueue returns the queue for one Prep Area. Prep Area
// is taken from the Product's Category (BR-PC-07).
func (s *Store) KitchenQueue(prepAreaID string, pickupReminderSeconds int) (Queue, error) {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM prep_area WHERE id = ?`, prepAreaID).Scan(&exists); err != nil || exists == 0 {
		return Queue{}, errors.New("prep area not found")
	}
	rows, err := s.db.Query(
		`SELECT oi.id, oi.order_id, o.bill_id, o.confirmed_at, oi.remake_of_id IS NOT NULL, oi.ready_at
		 FROM order_item oi
		 JOIN orders o ON o.id = oi.order_id
		 JOIN product p ON p.id = oi.product_id
		 JOIN category c ON c.id = p.category_id
		 WHERE c.prep_area_id = ? AND oi.status IN ('confirmed','preparing','ready')
		 ORDER BY CASE WHEN oi.remake_of_id IS NOT NULL THEN 0 ELSE 1 END, o.confirmed_at ASC, oi.rowid ASC`,
		prepAreaID,
	)
	if err != nil {
		return Queue{}, err
	}
	type meta struct {
		itemID, orderID, billID string
		confirmedAt             *int64
		isRemake                bool
		readyAt                 *int64
	}
	var metas []meta
	for rows.Next() {
		var m meta
		var confirmedAt, readyAt sql.NullInt64
		var isRemake int
		if err := rows.Scan(&m.itemID, &m.orderID, &m.billID, &confirmedAt, &isRemake, &readyAt); err == nil {
			m.confirmedAt = intPtr(confirmedAt)
			m.readyAt = intPtr(readyAt)
			m.isRemake = isRemake == 1
			metas = append(metas, m)
		}
	}
	rows.Close()
	now := time.Now().UnixMilli()
	q := Queue{PrepAreaID: prepAreaID, Items: []QueueItem{}, ReadyWaitingPickup: []QueueItem{}}
	for _, m := range metas {
		o, err := s.loadOrder(m.orderID)
		if err != nil {
			continue
		}
		var it *OrderItem
		for j := range o.Items {
			if o.Items[j].ID == m.itemID {
				it = &o.Items[j]
				break
			}
		}
		if it == nil {
			continue
		}
		qi := QueueItem{OrderItem: *it, OrderID: o.ID, BillID: o.BillID, PlacedBy: o.PlacedBy, PrepAreaID: prepAreaID, ConfirmedAt: m.confirmedAt, IsRemake: m.isRemake, ReadyAt: m.readyAt}
		trows, err := s.db.Query(`SELECT dt.label FROM bill_table bt JOIN dining_table dt ON dt.id = bt.table_id WHERE bt.bill_id = ?`, o.BillID)
		if err == nil {
			for trows.Next() {
				var label string
				if err := trows.Scan(&label); err == nil {
					qi.TableLabels = append(qi.TableLabels, label)
				}
			}
			trows.Close()
		}
		if m.confirmedAt != nil {
			qi.ElapsedSeconds = (now - *m.confirmedAt) / 1000
			if it.PrepTimeSeconds > 0 {
				ratio := float64(qi.ElapsedSeconds) / float64(it.PrepTimeSeconds)
				if ratio >= 1.0 {
					qi.LateStatus = "red"
				} else if ratio >= 0.8 {
					qi.LateStatus = "amber"
				}
			}
		}
		if m.readyAt != nil && it.Status == "ready" && pickupReminderSeconds > 0 && now-*m.readyAt >= int64(pickupReminderSeconds)*1000 {
			qi.WaitingPickup = true
		}
		if it.Status == "ready" {
			q.ReadyWaitingPickup = append(q.ReadyWaitingPickup, qi)
		} else {
			q.Items = append(q.Items, qi)
		}
	}
	return q, nil
}

// TransitionItem moves a Kitchen item Confirmed -> Preparing ->
// Ready (BR-KT-03, TR-SM-01).
func (s *Store) TransitionItem(crewID, itemID, toStatus string) (Order, error) {
	if toStatus != "preparing" && toStatus != "ready" {
		return Order{}, errors.New("kitchen transition must be preparing or ready")
	}
	var orderID, status string
	err := s.db.QueryRow(`SELECT order_id, status FROM order_item WHERE id = ?`, itemID).Scan(&orderID, &status)
	if err != nil {
		return Order{}, errors.New("order item not found")
	}
	expected := "confirmed"
	if toStatus == "ready" {
		expected = "preparing"
	}
	if status != expected {
		return Order{}, errors.New("item must be " + expected + " to become " + toStatus)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if toStatus == "ready" {
		if _, err := tx.Exec(`UPDATE order_item SET status = 'ready', ready_at = ? WHERE id = ? AND status = 'preparing'`, time.Now().UnixMilli(), itemID); err != nil {
			return Order{}, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE order_item SET status = 'preparing' WHERE id = ? AND status = 'confirmed'`, itemID); err != nil {
			return Order{}, err
		}
	}
	s.recomputeOrderStatus(tx, orderID)
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.loadOrder(orderID)
}

// KitchenCancelItem cancels a Confirmed or Preparing item
// (BR-KT-04), reusing the Waiter cancel logic which handles
// out-of-stock Sold Out flagging.
func (s *Store) KitchenCancelItem(crewID, itemID, reason string) (Order, error) {
	var status string
	err := s.db.QueryRow(`SELECT status FROM order_item WHERE id = ?`, itemID).Scan(&status)
	if err != nil {
		return Order{}, errors.New("order item not found")
	}
	if status != "confirmed" && status != "preparing" {
		return Order{}, errors.New("only confirmed or preparing items can be cancelled by kitchen")
	}
	return s.CancelItem(crewID, itemID, reason)
}

// EstimatedWaitForOrder implements TR-KT-03: for each Prep Area,
// (sum of remaining prep time of items ahead in that area /
// parallel capacity) + longest Prep Time among this Order's
// items in that area, maximised over areas, rounded up to minutes.
func (s *Store) EstimatedWaitForOrder(orderID string) (int, error) {
	o, err := s.loadOrder(orderID)
	if err != nil {
		return 0, err
	}
	// Map item -> prep area and confirmed_at.
	type itemMeta struct {
		prepArea    string
		prepTime    int
		confirmedAt int64
		status      string
	}
	// Load all confirmed/preparing items grouped by prep area.
	rows, err := s.db.Query(
		`SELECT oi.id, c.prep_area_id, p.prep_time_seconds, o.confirmed_at, oi.status
		 FROM order_item oi JOIN orders o ON o.id = oi.order_id
		 JOIN product p ON p.id = oi.product_id
		 JOIN category c ON c.id = p.category_id
		 WHERE oi.status IN ('confirmed','preparing')`,
	)
	if err != nil {
		return 0, err
	}
	type queueItem struct {
		id          string
		prepTime    int
		confirmedAt int64
	}
	byArea := map[string][]queueItem{}
	capacities := map[string]int{}
	for rows.Next() {
		var id, area string
		var prepTime int
		var confirmedAt sql.NullInt64
		var status string
		if err := rows.Scan(&id, &area, &prepTime, &confirmedAt, &status); err == nil {
			byArea[area] = append(byArea[area], queueItem{id, prepTime, confirmedAt.Int64})
		}
	}
	rows.Close()
	// Capacities
	crows, err := s.db.Query(`SELECT id, parallel_capacity FROM prep_area`)
	if err == nil {
		for crows.Next() {
			var id string
			var cap int
			if err := crows.Scan(&id, &cap); err == nil {
				capacities[id] = cap
			}
		}
		crows.Close()
	}
	// This order's items by area, only those still in the
	// Kitchen queue (confirmed/preparing). A Ready/Served order
	// has no wait left.
	ownByArea := map[string][]OrderItem{}
	for _, it := range o.Items {
		if it.Status != "confirmed" && it.Status != "preparing" {
			continue
		}
		var area string
		_ = s.db.QueryRow(`SELECT c.prep_area_id FROM product p JOIN category c ON c.id = p.category_id WHERE p.id = ?`, it.ProductID).Scan(&area)
		ownByArea[area] = append(ownByArea[area], it)
	}
	now := time.Now().UnixMilli()
	maxWaitSeconds := 0
	for area, ownItems := range ownByArea {
		cap := capacities[area]
		if cap <= 0 {
			cap = 1
		}
		// Items ahead: confirmed before this order's confirmed_at,
		// excluding this order's own items.
		aheadSeconds := 0
		ownIDs := map[string]bool{}
		for _, it := range ownItems {
			ownIDs[it.ID] = true
		}
		var ownConfirmedAt int64
		if o.ConfirmedAt != nil {
			ownConfirmedAt = *o.ConfirmedAt
		}
		for _, qi := range byArea[area] {
			if ownIDs[qi.id] {
				continue
			}
			if qi.confirmedAt < ownConfirmedAt {
				// Remaining time: full prep time for confirmed,
				// prep minus elapsed for preparing (approx: use
				// full prep time minus elapsed since confirmation
				// as a conservative estimate).
				elapsed := (now - qi.confirmedAt) / 1000
				remaining := qi.prepTime - int(elapsed)
				if remaining < 0 {
					remaining = 0
				}
				aheadSeconds += remaining
			}
		}
		longestOwn := 0
		for _, it := range ownItems {
			if it.PrepTimeSeconds > longestOwn {
				longestOwn = it.PrepTimeSeconds
			}
		}
		wait := aheadSeconds/cap + longestOwn
		if wait > maxWaitSeconds {
			maxWaitSeconds = wait
		}
	}
	if maxWaitSeconds == 0 {
		return 0, nil
	}
	return int(math.Ceil(float64(maxWaitSeconds) / 60.0)), nil
}
