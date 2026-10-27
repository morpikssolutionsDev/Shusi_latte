package ordering

import (
	"context"
	"database/sql"
	"time"

	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// AnonymiseAfter is the 2-day retention of Customer data after a Bill closes (BR-P-03, ADR-0002).
const AnonymiseAfter = 48 * time.Hour

// CloseEmptyBills closes open Bills that never received a live Order within the empty-Bill time,
// freeing their Tables (BR-R-26, G-01). It returns the closed Bill ids.
func (s *Service) CloseEmptyBills(ctx context.Context) ([]string, error) {
	settings, err := s.outlet.Settings(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	cutoff := platform.Millis(now.Add(-time.Duration(settings.Ops.EmptyBillCloseSeconds) * time.Second))
	var closed []string
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		ids, err := tx.ListEmptyBillsBefore(ctx, cutoff)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := closeBill(ctx, tx, id, BillClosed, platform.Millis(now)); err != nil {
				return err
			}
			closed = append(closed, id)
		}
		return nil
	})
	return closed, err
}

// closeBill ends a Bill and frees its Tables; every guest token for it stops working.
func closeBill(ctx context.Context, tx *dbx.Tx, id, status string, now int64) error {
	if err := tx.SetBillStatus(ctx, store.SetBillStatusParams{Status: status, ClosedAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now, ID: id}); err != nil {
		return err
	}
	if err := tx.SetBillTablesOpen(ctx, store.SetBillTablesOpenParams{IsOpen: 0, BillID: id}); err != nil {
		return err
	}
	tx.Publish(
		realtime.Event{Topic: BillTopic(id), Type: "bill.closed", ID: id},
		realtime.Event{Topic: WaitersTopic, Type: "table.changed", ID: id},
	)
	return nil
}

// Escalate re-notifies Waiters about Orders and Waiter calls left unclaimed past the re-notify
// time, alerts the Outlet Admin past the escalation time (BR-WT-10, G-05), and reminds the owning
// Waiter of Ready items not picked up (BR-WT-09, G-09). Each happens once per Order, call or item.
func (s *Service) Escalate(ctx context.Context) error {
	settings, err := s.outlet.Settings(ctx)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	before := func(seconds int64) int64 { return platform.Millis(now.Add(-time.Duration(seconds) * time.Second)) }
	ops := settings.Ops
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		renotify, err := tx.ListOrdersToRenotify(ctx, before(ops.ClaimRenotifySeconds))
		if err != nil {
			return err
		}
		for _, o := range renotify {
			if err := tx.SetOrderRenotified(ctx, store.SetOrderRenotifiedParams{RenotifiedAt: nullMs(nowMs), ID: o.ID}); err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "order.renotify", ID: o.ID})
		}
		escalate, err := tx.ListOrdersToEscalate(ctx, before(ops.ClaimEscalateSeconds))
		if err != nil {
			return err
		}
		for _, o := range escalate {
			if err := tx.SetOrderEscalated(ctx, store.SetOrderEscalatedParams{EscalatedAt: nullMs(nowMs), ID: o.ID}); err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: OutletAdminTopic, Type: "order.escalated", ID: o.ID},
				realtime.Event{Topic: WaitersTopic, Type: "order.escalated", ID: o.ID})
		}
		calls, err := tx.ListCallsToRenotify(ctx, before(ops.ClaimRenotifySeconds))
		if err != nil {
			return err
		}
		for _, c := range calls {
			if err := tx.SetCallRenotified(ctx, store.SetCallRenotifiedParams{RenotifiedAt: nullMs(nowMs), ID: c.ID}); err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "waiter_call.renotify", ID: c.ID})
		}
		calls, err = tx.ListCallsToEscalate(ctx, before(ops.ClaimEscalateSeconds))
		if err != nil {
			return err
		}
		for _, c := range calls {
			if err := tx.SetCallEscalated(ctx, store.SetCallEscalatedParams{EscalatedAt: nullMs(nowMs), ID: c.ID}); err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: OutletAdminTopic, Type: "waiter_call.escalated", ID: c.ID},
				realtime.Event{Topic: WaitersTopic, Type: "waiter_call.escalated", ID: c.ID})
		}
		ready, err := tx.ListItemsToRemind(ctx, nullMs(before(ops.PickupReminderSeconds)))
		if err != nil {
			return err
		}
		for _, it := range ready {
			if err := tx.SetPickupReminded(ctx, store.SetPickupRemindedParams{PickupRemindedAt: nullMs(nowMs), ID: it.ID}); err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "item.pickup_reminder", ID: it.OrderID})
		}
		return nil
	})
}

// Anonymise clears Customer names, Dietary Restrictions and Dietary Notes from Bills closed more
// than 48 hours ago (TR-RET-01). Dietary Warnings on Order Items stay: they hold only the
// Product-side tag.
func (s *Service) Anonymise(ctx context.Context) (int, error) {
	now := s.clock.Now()
	cutoff := sql.NullInt64{Int64: platform.Millis(now.Add(-AnonymiseAfter)), Valid: true}
	n := 0
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		ids, err := tx.ListBillsToAnonymise(ctx, cutoff)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := tx.AnonymiseCustomers(ctx, id); err != nil {
				return err
			}
			if err := tx.AnonymiseOrders(ctx, id); err != nil {
				return err
			}
			if err := tx.MarkBillAnonymised(ctx, store.MarkBillAnonymisedParams{AnonymisedAt: sql.NullInt64{Int64: platform.Millis(now), Valid: true},
				UpdatedAt: platform.Millis(now), ID: id}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}
