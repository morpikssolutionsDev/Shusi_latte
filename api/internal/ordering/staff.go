package ordering

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// Cancel Reasons (BR-R-10).
const (
	ReasonOutOfStock      = "out_of_stock"
	ReasonCustomerRequest = "customer_request"
	ReasonWrongOrder      = "wrong_order"
	ReasonOther           = "other"
)

var cancelReasons = []string{ReasonOutOfStock, ReasonCustomerRequest, ReasonWrongOrder, ReasonOther}

// PrepAreaTopic is the realtime topic of one Prep Area display (TR-RT-02).
func PrepAreaTopic(id string) string { return "prep-area:" + id }

const OutletAdminTopic = "outlet-admin"

func conflict(code, title string) *httpx.Problem {
	return httpx.NewProblem(http.StatusConflict, code, title)
}

func notFound(title string) *httpx.Problem {
	return httpx.NewProblem(http.StatusNotFound, "not_found", title)
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullMs(ms int64) sql.NullInt64 { return sql.NullInt64{Int64: ms, Valid: true} }

func loadOrder(ctx context.Context, tx *dbx.Tx, id string) (store.Order, error) {
	o, err := tx.GetOrder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return o, notFound("Pesanan tidak ditemukan")
	}
	return o, err
}

// orderEvents tells everyone who shows this Order that it changed.
func orderEvents(tx *dbx.Tx, o store.Order, kind string) {
	tx.Publish(
		realtime.Event{Topic: WaitersTopic, Type: kind, ID: o.ID},
		realtime.Event{Topic: BillTopic(o.BillID), Type: kind, ID: o.ID},
	)
}

// ---------------------------------------------------------------- Claim (OP-03)

// Claim takes ownership of a Customer-placed Order. First writer wins; the loser gets 409 and
// every device withdraws the notification (BR-WT-01, TR-SM-03).
func (s *Service) Claim(ctx context.Context, p *auth.Principal, orderID string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		o, err := loadOrder(ctx, tx, orderID)
		if err != nil {
			return err
		}
		n, err := tx.ClaimOrder(ctx, store.ClaimOrderParams{ClaimedBy: nullStr(p.ID), ClaimedAt: nullMs(now), UpdatedAt: now, ID: orderID})
		if err != nil {
			return err
		}
		if n == 0 {
			prob := conflict("already_claimed", "Pesanan sudah diklaim")
			if o.ClaimedBy.Valid {
				if c, err := tx.GetCrew(ctx, o.ClaimedBy.String); err == nil {
					prob.Title = "Diklaim oleh " + strings.Fields(c.FullName)[0]
					prob.With("claimedBy", c.CrewID)
				}
			}
			return prob
		}
		orderEvents(tx, o, "order.claimed")
		return nil
	})
}

// TransferClaim moves a Claim to another Waiter: a takeover by the new Waiter themselves, or a
// reassignment by the Outlet Admin (BR-WT-11, BR-R-34, TR-SM-05). Both are audit-logged.
func (s *Service) TransferClaim(ctx context.Context, p *auth.Principal, orderID, toCrewID string) error {
	kind := "takeover"
	if toCrewID == "" || toCrewID == p.ID {
		toCrewID = p.ID
	} else {
		if !p.HasRole(auth.RoleOutletAdmin, auth.RoleManagement) {
			return httpx.NewProblem(http.StatusForbidden, "forbidden", "Hanya Outlet Admin yang bisa memindahkan klaim")
		}
		kind = "reassign"
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		o, err := loadOrder(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if !o.ClaimedBy.Valid || o.Status == OrderCancelled {
			return conflict("not_claimed", "Pesanan ini belum diklaim")
		}
		if o.ClaimedBy.String == toCrewID {
			return conflict("same_waiter", "Klaim sudah milik pelayan ini")
		}
		to, err := tx.GetCrew(ctx, toCrewID)
		if errors.Is(err, sql.ErrNoRows) || err == nil && (to.Active == 0 || !hasStation(ctx, tx, toCrewID, auth.StationWaiter, auth.StationCashier)) {
			return httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_waiter", "Pilih pelayan yang aktif")
		}
		if err != nil {
			return err
		}
		if err := tx.SetOrderClaimant(ctx, store.SetOrderClaimantParams{ClaimedBy: nullStr(toCrewID), UpdatedAt: platform.Millis(now), ID: orderID}); err != nil {
			return err
		}
		if err := tx.CreateClaimTransfer(ctx, store.CreateClaimTransferParams{ID: platform.NewID(), OrderID: orderID, FromCrewID: o.ClaimedBy.String,
			ToCrewID: toCrewID, Kind: kind, ByCrewID: p.ID, At: platform.Millis(now)}); err != nil {
			return err
		}
		orderEvents(tx, o, "order.claim_transferred")
		return audit.Write(ctx, tx, now, audit.Entry{Actor: p.ID, Action: "claim." + kind, Entity: "order", EntityID: orderID,
			Before: map[string]string{"claimedBy": o.ClaimedBy.String}, After: map[string]string{"claimedBy": toCrewID}})
	})
}

func hasStation(ctx context.Context, tx *dbx.Tx, crewID string, stations ...string) bool {
	rows, err := tx.ListCrewStations(ctx, crewID)
	if err != nil {
		return false
	}
	for _, r := range rows {
		if slices.Contains(stations, r) {
			return true
		}
	}
	return false
}

// Confirm sends a Claimed Order to the Kitchen: its Pending items become Confirmed and appear
// on their Prep Areas (BR-WT-02, BR-R-06). Flagged items must be Cancelled first (BR-WT-05),
// and every item is re-checked against the live catalog.
func (s *Service) Confirm(ctx context.Context, p *auth.Principal, orderID string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		o, err := loadOrder(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if o.Status != "claimed" {
			return conflict("not_claimed", "Klaim pesanan dulu sebelum konfirmasi")
		}
		if o.ClaimedBy.String != p.ID {
			return conflict("claimed_by_other", "Pesanan ini diklaim pelayan lain")
		}
		bill, err := tx.GetBill(ctx, o.BillID)
		if err != nil {
			return err
		}
		if bill.Status != BillOpen && bill.Status != BillSettling {
			return conflict("bill_closed", "Tagihan sudah ditutup")
		}
		if bill.Type == "takeaway" {
			return conflict("takeaway_unpaid", "Pesanan bawa pulang dikirim ke dapur otomatis setelah lunas")
		}
		items, err := tx.ListOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		cat, err := catalog.Load(ctx, tx.Queries, false)
		if err != nil {
			return err
		}
		areas := map[string]bool{}
		live := 0
		for _, it := range items {
			if it.Status != ItemPending {
				continue
			}
			live++
			if it.SoldOutFlag == 1 || it.UnavailableFlag == 1 || !orderableVariant(cat, it.VariantID) {
				return conflict("flagged_items", "Batalkan item yang habis atau tidak tersedia dulu").With("itemId", it.ID)
			}
			areas[it.PrepAreaID] = true
		}
		if live == 0 {
			return conflict("empty_order", "Tidak ada item untuk dikonfirmasi")
		}
		if n, err := tx.ConfirmOrder(ctx, store.ConfirmOrderParams{ConfirmedBy: nullStr(p.ID), ConfirmedAt: nullMs(now), UpdatedAt: now, ID: orderID}); err != nil {
			return err
		} else if n == 0 {
			return conflict("not_claimed", "Status pesanan berubah, muat ulang")
		}
		if err := tx.ConfirmOrderItems(ctx, store.ConfirmOrderItemsParams{UpdatedAt: now, OrderID: orderID}); err != nil {
			return err
		}
		for a := range areas {
			if err := queueChanged(ctx, tx, a, orderID); err != nil {
				return err
			}
		}
		orderEvents(tx, o, "order.confirmed")
		return nil
	})
}

// AddItems adds lines to a Claimed Order while the Waiter reviews it with the Customer (OP-03 H3).
// To change a quantity, the Waiter Cancels the line and adds it again.
func (s *Service) AddItems(ctx context.Context, p *auth.Principal, orderID string, items []ItemInput) error {
	if len(items) == 0 {
		return invalid("Pilih minimal satu item.")
	}
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		o, err := loadOrder(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if o.Status != "claimed" || o.ClaimedBy.String != p.ID {
			return conflict("not_claimed", "Hanya pelayan yang mengklaim bisa mengubah pesanan sebelum konfirmasi")
		}
		cat, err := catalog.Load(ctx, tx.Queries, false)
		if err != nil {
			return err
		}
		var restriction []string
		if o.CustomerEntryID.Valid {
			if e, err := tx.GetCustomerEntry(ctx, o.CustomerEntryID.String); err == nil {
				restriction = s.restriction(e)
			}
		}
		var lines []resolved
		var problems []ItemProblem
		for i, in := range items {
			r, prob := resolve(cat, i, in, restriction, false)
			if prob != nil {
				problems = append(problems, *prob)
				continue
			}
			lines = append(lines, r)
		}
		if len(problems) > 0 {
			return rejected(problems)
		}
		existing, err := tx.ListOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if err := insertItems(ctx, tx, cat, orderID, len(existing), lines, now); err != nil {
			return err
		}
		orderEvents(tx, o, "order.updated")
		return nil
	})
}

func orderableVariant(cat catalog.Catalog, variantID string) bool {
	for _, p := range cat.Products {
		for _, v := range p.Variants {
			if v.ID == variantID {
				return p.Status == catalog.StatusActive && !v.SoldOut
			}
		}
	}
	return false
}

// ---------------------------------------------------------------- direct entry (OP-04)

type DirectInput struct {
	TableID           string      `json:"tableId"`
	BillID            string      `json:"billId,omitempty"`
	CustomerName      string      `json:"customerName,omitempty"`
	RestrictionTagIDs []string    `json:"restrictionTagIds"`
	Items             []ItemInput `json:"items"`
}

// DirectOrder is a Waiter or Cashier taking an Order verbally: it opens the Table's Bill if
// needed and is created already Confirmed, owned by the person entering it (BR-WT-03). A ticked
// Dietary Restriction raises Dietary Warnings exactly like a Customer's (BR-R-D1).
func (s *Service) DirectOrder(ctx context.Context, p *auth.Principal, source string, in DirectInput) (string, error) {
	if len(in.Items) == 0 {
		return "", invalid("Pilih minimal satu item.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	orderID := platform.NewID()
	in.CustomerName = strings.Join(strings.Fields(in.CustomerName), " ")
	var keyID string
	var nameEnc, tagsEnc []byte
	var err error
	if in.CustomerName != "" {
		if keyID, nameEnc, err = s.vault.Seal(now, []byte(in.CustomerName)); err != nil {
			return "", err
		}
	}
	if len(in.RestrictionTagIDs) > 0 {
		raw, _ := json.Marshal(in.RestrictionTagIDs)
		if keyID == "" {
			keyID, tagsEnc, err = s.vault.Seal(now, raw)
		} else {
			tagsEnc, err = s.vault.SealWith(keyID, raw)
		}
		if err != nil {
			return "", err
		}
	}
	err = s.db.Write(ctx, func(tx *dbx.Tx) error {
		var bill store.Bill
		if in.BillID != "" {
			b, err := tx.GetBill(ctx, in.BillID)
			if errors.Is(err, sql.ErrNoRows) {
				return notFound("Tagihan tidak ditemukan")
			}
			if err != nil {
				return err
			}
			bill = b
		} else {
			b, err := tx.GetOpenBillForTable(ctx, in.TableID)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				if b, err = s.openBill(ctx, tx, in.TableID, now); err != nil {
					return err
				}
				tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "table.changed", ID: in.TableID})
			case err != nil:
				return err
			}
			bill = b
		}
		if bill.Status != BillOpen && bill.Status != BillSettling {
			return conflict("bill_closed", "Tagihan sudah ditutup")
		}
		// A Takeaway keeps one held Order until paid (TR-PAY-04): lines are added to it instead.
		if bill.Type == "takeaway" {
			return conflict("takeaway_unpaid", "Tambahkan item ke pesanan bawa pulang yang sudah ada")
		}
		cat, err := catalog.Load(ctx, tx.Queries, false)
		if err != nil {
			return err
		}
		// Staff see the warnings on screen while choosing, so acknowledgement is implied.
		var lines []resolved
		var problems []ItemProblem
		for i, item := range in.Items {
			r, prob := resolve(cat, i, item, in.RestrictionTagIDs, false)
			if prob != nil {
				problems = append(problems, *prob)
				continue
			}
			lines = append(lines, r)
		}
		if len(problems) > 0 {
			return rejected(problems)
		}
		var customer sql.NullString
		if nameEnc != nil {
			customer = nullStr(platform.NewID())
			if err := tx.CreateCustomerEntry(ctx, store.CreateCustomerEntryParams{ID: customer.String, BillID: bill.ID, KeyID: keyID,
				NameEnc: nameEnc, JoinedAt: nowMs}); err != nil {
				return err
			}
		}
		seq, err := tx.NextOrderSeq(ctx, bill.ID)
		if err != nil {
			return err
		}
		if err := tx.CreateConfirmedOrder(ctx, store.CreateConfirmedOrderParams{ID: orderID, BillID: bill.ID, Seq: seq, CustomerEntryID: customer,
			Source: source, PlacedBy: nullStr(p.ID), ClaimedBy: nullStr(p.ID), ClaimedAt: nullMs(nowMs), ConfirmedBy: nullStr(p.ID),
			ConfirmedAt: nullMs(nowMs), KeyID: nullStr(keyID), RestrictionTagsEnc: tagsEnc, CreatedAt: nowMs, UpdatedAt: nowMs}); err != nil {
			return err
		}
		if err := insertItems(ctx, tx, cat, orderID, 0, lines, nowMs); err != nil {
			return err
		}
		if err := tx.ConfirmOrderItems(ctx, store.ConfirmOrderItemsParams{UpdatedAt: nowMs, OrderID: orderID}); err != nil {
			return err
		}
		areas := map[string]bool{}
		for _, l := range lines {
			for _, c := range cat.Categories {
				if c.ID == l.product.CategoryID {
					areas[c.PrepAreaID] = true
				}
			}
		}
		for a := range areas {
			if err := queueChanged(ctx, tx, a, orderID); err != nil {
				return err
			}
		}
		tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "order.confirmed", ID: orderID},
			realtime.Event{Topic: BillTopic(bill.ID), Type: "order.created", ID: orderID})
		return nil
	})
	return orderID, err
}

// ---------------------------------------------------------------- Cancel, Serve, Remake, Void

// CancelItem stops an Order Item before Ready with a Cancel Reason, no PIN (BR-R-07, BR-WT-04).
// Waiters Cancel Pending or Confirmed items; the Kitchen Confirmed or Preparing ones. Out of
// Stock also marks the Variant Sold Out everywhere and flags its other open items (TR-SM-04).
func (s *Service) CancelItem(ctx context.Context, p *auth.Principal, itemID, reason, note string, allowed []string) error {
	if !slices.Contains(cancelReasons, reason) {
		return invalid("Pilih alasan pembatalan.")
	}
	note = strings.TrimSpace(note)
	if reason == ReasonOther && note == "" {
		return invalid("Tulis alasan lainnya.")
	}
	now := s.clock.Now()
	nowMs := platform.Millis(now)
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		it, err := tx.GetOrderItem(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("Item tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if !slices.Contains(allowed, it.Status) {
			return conflict("invalid_transition", "Item ini tidak bisa dibatalkan pada status ini").With("status", it.Status)
		}
		o, err := loadOrder(ctx, tx, it.OrderID)
		if err != nil {
			return err
		}
		if n, err := tx.CancelItem(ctx, store.CancelItemParams{CancelReason: nullStr(reason), CancelNote: nullStr(note), CancelledBy: nullStr(p.ID),
			CancelledAt: nullMs(nowMs), UpdatedAt: nowMs, ID: itemID, Status: it.Status}); err != nil {
			return err
		} else if n == 0 {
			return conflict("invalid_transition", "Status item berubah, muat ulang")
		}
		if reason == ReasonOutOfStock {
			v, err := tx.GetVariant(ctx, it.VariantID)
			if err != nil {
				return err
			}
			if v.SoldOut == 0 {
				if err := catalog.MarkSoldOut(ctx, tx, now, p.ID, it.VariantID, catalog.SoldOutOutOfStock); err != nil {
					return err
				}
			}
		}
		if err := s.closeIfEmpty(ctx, tx, o, nowMs); err != nil {
			return err
		}
		if it.Status == "confirmed" || it.Status == "preparing" {
			if err := queueChanged(ctx, tx, it.PrepAreaID, it.OrderID); err != nil {
				return err
			}
		}
		orderEvents(tx, o, "item.cancelled")
		return nil
	})
}

// closeIfEmpty marks an Order Cancelled once every item is Cancelled.
func (s *Service) closeIfEmpty(ctx context.Context, tx *dbx.Tx, o store.Order, now int64) error {
	items, err := tx.ListOrderItemsByOrder(ctx, o.ID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Status != ItemCancelled {
			return nil
		}
	}
	return tx.SetOrderStatus(ctx, store.SetOrderStatusParams{Status: OrderCancelled, UpdatedAt: now, ID: o.ID})
}

// Serve marks Ready items Served (BR-WT-09).
func (s *Service) Serve(ctx context.Context, p *auth.Principal, itemIDs []string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		for _, id := range itemIDs {
			it, err := tx.GetOrderItem(ctx, id)
			if errors.Is(err, sql.ErrNoRows) {
				return notFound("Item tidak ditemukan")
			}
			if err != nil {
				return err
			}
			n, err := tx.ServeItem(ctx, store.ServeItemParams{ServedAt: nullMs(now), ServedBy: nullStr(p.ID), UpdatedAt: now, ID: id})
			if err != nil {
				return err
			}
			if n == 0 {
				return conflict("invalid_transition", "Item belum siap").With("itemId", id)
			}
			o, err := loadOrder(ctx, tx, it.OrderID)
			if err != nil {
				return err
			}
			tx.Publish(realtime.Event{Topic: PrepAreaTopic(it.PrepAreaID), Type: "queue.changed", ID: it.OrderID})
			orderEvents(tx, o, "item.served")
		}
		return nil
	})
}

// Remake replaces a Served item that was wrong or bad with a no-charge copy at the top of the
// Kitchen queue, under an Outlet Admin or Management PIN Override (BR-R-33, TR-DM-07).
func (s *Service) Remake(ctx context.Context, p *auth.Principal, itemID, reason, overrideToken string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", invalid("Tulis alasan Remake.")
	}
	now := s.clock.Now()
	newID := platform.NewID()
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		it, err := tx.GetOrderItem(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("Item tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if it.Status != "served" {
			return conflict("invalid_transition", "Remake hanya untuk item yang sudah disajikan")
		}
		approver, err := auth.ConsumeOverride(ctx, tx, now, p, overrideToken, auth.ActionRemake, audit.Entry{Action: "order_item.remade",
			Entity: "order_item", EntityID: itemID, After: map[string]string{"remakeId": newID}, Reason: reason})
		if err != nil {
			return err
		}
		if err := tx.CreateRemakeItem(ctx, store.CreateRemakeItemParams{NewID: newID, Reason: nullStr(reason), OverrideBy: nullStr(approver),
			Now: platform.Millis(now), OriginalID: itemID}); err != nil {
			return err
		}
		if err := tx.CopyRemakeModifiers(ctx, store.CopyRemakeModifiersParams{NewID: newID, OriginalID: itemID}); err != nil {
			return err
		}
		if err := tx.CopyRemakeWarnings(ctx, store.CopyRemakeWarningsParams{NewID: newID, OriginalID: itemID}); err != nil {
			return err
		}
		o, err := loadOrder(ctx, tx, it.OrderID)
		if err != nil {
			return err
		}
		if err := queueChanged(ctx, tx, it.PrepAreaID, it.OrderID); err != nil {
			return err
		}
		orderEvents(tx, o, "item.remade")
		return nil
	})
	return newID, err
}

// Void removes a Ready or Served item from an unpaid Bill under a PIN Override (BR-R-08).
// Items on a paid Bill go through Refund instead.
func (s *Service) Void(ctx context.Context, p *auth.Principal, itemID, reason, overrideToken string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return invalid("Tulis alasan Void.")
	}
	now := s.clock.Now()
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		it, err := tx.GetOrderItem(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("Item tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if it.Status != "ready" && it.Status != "served" {
			return conflict("invalid_transition", "Void hanya untuk item siap atau disajikan; batalkan item lainnya")
		}
		o, err := loadOrder(ctx, tx, it.OrderID)
		if err != nil {
			return err
		}
		if paid, err := s.itemPaid(ctx, tx, o.BillID, it); err != nil {
			return err
		} else if paid {
			return conflict("item_paid", "Item ini sudah dibayar — gunakan Refund")
		}
		approver, err := auth.ConsumeOverride(ctx, tx, now, p, overrideToken, auth.ActionVoid, audit.Entry{Action: "order_item.voided",
			Entity: "order_item", EntityID: itemID, Before: map[string]string{"status": it.Status}, Reason: reason})
		if err != nil {
			return err
		}
		if _, err := tx.VoidItem(ctx, store.VoidItemParams{VoidReason: nullStr(reason), VoidedBy: nullStr(p.ID), VoidedOverrideBy: nullStr(approver),
			VoidedAt: nullMs(platform.Millis(now)), UpdatedAt: platform.Millis(now), ID: itemID}); err != nil {
			return err
		}
		orderEvents(tx, o, "item.voided")
		return nil
	})
}

// itemPaid reports whether the item is already paid: its Sub-bill is closed. An unsplit Bill is
// unpaid until it closes, and a closed Bill no longer takes Voids (BR-R-08, G-10).
func (s *Service) itemPaid(ctx context.Context, tx *dbx.Tx, _ string, it store.OrderItem) (bool, error) {
	if !it.SubBillID.Valid {
		return false, nil
	}
	sb, err := tx.GetSubBill(ctx, it.SubBillID.String)
	if err != nil {
		return false, err
	}
	return sb.Status == "closed", nil
}

// AnswerCall marks a Waiter call handled.
func (s *Service) AnswerCall(ctx context.Context, p *auth.Principal, callID string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		n, err := tx.AnswerWaiterCall(ctx, store.AnswerWaiterCallParams{AnsweredBy: nullStr(p.ID), AnsweredAt: nullMs(now), ID: callID})
		if err != nil {
			return err
		}
		if n == 0 {
			return conflict("already_answered", "Panggilan sudah ditangani")
		}
		tx.Publish(realtime.Event{Topic: WaitersTopic, Type: "waiter_call.answered", ID: callID})
		return nil
	})
}
