package ordering

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// Kitchen and Bar displays (OP-05, BR-KT-01..06, TRD §11).

type KitchenItem struct {
	ID              string   `json:"id"`
	Quantity        int64    `json:"quantity"`
	Name            string   `json:"name"`
	Variant         string   `json:"variant,omitempty"`
	Modifiers       []string `json:"modifiers"`
	Note            string   `json:"note,omitempty"`
	Status          string   `json:"status"`
	PrepTimeSeconds int64    `json:"prepTimeSeconds"`
	PreparingAt     *int64   `json:"preparingAt,omitempty"`
	ReadyAt         *int64   `json:"readyAt,omitempty"`
	Warnings        []TagRef `json:"warnings"`
	RemakeReason    string   `json:"remakeReason,omitempty"`
}

// KitchenTicket is one Order's items for one Prep Area (KitchenTicket, COMPONENTS.md §4). A
// Remake gets its own ticket, sorted first (BR-KT-06, TR-KT-01).
type KitchenTicket struct {
	Key             string        `json:"key"`
	OrderID         string        `json:"orderId"`
	BillID          string        `json:"billId"`
	Seq             int64         `json:"seq"`
	Tables          []string      `json:"tables"`
	Waiter          string        `json:"waiter,omitempty"`
	Customer        string        `json:"customer,omitempty"`
	DietaryNote     string        `json:"dietaryNote,omitempty"`
	ConfirmedAt     int64         `json:"confirmedAt"`
	PrepTimeSeconds int64         `json:"prepTimeSeconds"` // longest item: the ticket's target
	Remake          bool          `json:"remake"`
	Items           []KitchenItem `json:"items"`
}

type KitchenQueue struct {
	PrepArea              PrepAreaRef     `json:"prepArea"`
	Tickets               []KitchenTicket `json:"tickets"` // Confirmed and Preparing
	Ready                 []KitchenTicket `json:"ready"`   // Ready, waiting for pickup
	PickupReminderSeconds int64           `json:"pickupReminderSeconds"`
	Now                   int64           `json:"now"`
}

type PrepAreaRef struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	NameID string `json:"nameId"`
}

func ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

// Queue builds a Prep Area's display; prepAreaID may also be its code.
func (s *Service) Queue(ctx context.Context, prepAreaID string) (KitchenQueue, error) {
	sc, err := s.staffContext(ctx)
	if err != nil {
		return KitchenQueue{}, err
	}
	q := sc.q
	area, err := q.GetPrepArea(ctx, prepAreaID)
	if errors.Is(err, sql.ErrNoRows) { // screens address their area by code (kitchen, bar)
		area, err = q.GetPrepAreaByCode(ctx, prepAreaID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return KitchenQueue{}, notFound("Prep Area tidak ditemukan")
	}
	if err != nil {
		return KitchenQueue{}, err
	}
	settings, err := s.outlet.Settings(ctx)
	if err != nil {
		return KitchenQueue{}, err
	}
	rows, err := q.ListKitchenItems(ctx, area.ID)
	if err != nil {
		return KitchenQueue{}, err
	}
	tags, err := q.ListDietaryTags(ctx)
	if err != nil {
		return KitchenQueue{}, err
	}
	tagByID := map[string]TagRef{}
	for _, t := range tags {
		tagByID[t.ID] = TagRef{ID: t.ID, Code: t.Code, NameID: t.NameID, NameEN: t.NameEn}
	}
	out := KitchenQueue{PrepArea: PrepAreaRef{ID: area.ID, Code: area.Code, NameID: area.NameID}, Tickets: []KitchenTicket{}, Ready: []KitchenTicket{},
		PickupReminderSeconds: settings.Ops.PickupReminderSeconds, Now: platform.Millis(s.clock.Now())}

	// Per-Bill details loaded once.
	mods := map[string][]store.OrderItemModifier{}
	warns := map[string][]TagRef{}
	customers := map[string]store.CustomerEntry{}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.BillID] {
			continue
		}
		seen[r.BillID] = true
		ms, err := q.ListOrderItemModifiersByBill(ctx, r.BillID)
		if err != nil {
			return KitchenQueue{}, err
		}
		for _, m := range ms {
			mods[m.OrderItemID] = append(mods[m.OrderItemID], m)
		}
		ws, err := q.ListDietaryWarningsByBill(ctx, r.BillID)
		if err != nil {
			return KitchenQueue{}, err
		}
		for _, w := range ws {
			warns[w.OrderItemID] = append(warns[w.OrderItemID], tagByID[w.DietaryTagID])
		}
		es, err := q.ListCustomerEntries(ctx, r.BillID)
		if err != nil {
			return KitchenQueue{}, err
		}
		for _, e := range es {
			customers[e.ID] = e
		}
	}

	tickets := map[string]*KitchenTicket{}
	var order []string
	for _, r := range rows {
		key := r.OrderID
		if r.RemakeOfID.Valid {
			key += ":remake"
		}
		if r.Status == "ready" {
			key += ":ready"
		}
		t := tickets[key]
		if t == nil {
			t = &KitchenTicket{Key: key, OrderID: r.OrderID, BillID: r.BillID, Seq: r.OrderSeq, ConfirmedAt: r.OrderConfirmedAt.Int64,
				Remake: r.RemakeOfID.Valid, Items: []KitchenItem{}}
			for _, tb := range sc.billTables(ctx, r.BillID) {
				t.Tables = append(t.Tables, tb.Label)
			}
			if r.ClaimedBy.Valid {
				t.Waiter = sc.crew[r.ClaimedBy.String].Name
			}
			if e, ok := customers[r.CustomerEntryID.String]; ok {
				t.Customer, t.DietaryNote = s.name(e), s.note(e)
			}
			tickets[key] = t
			order = append(order, key)
		}
		it := KitchenItem{ID: r.ID, Quantity: r.Quantity, Name: r.ProductNameID, Variant: r.VariantNameID, Modifiers: []string{}, Note: r.Note,
			Status: r.Status, PrepTimeSeconds: r.PrepTimeSeconds, PreparingAt: ptr(r.PreparingAt), ReadyAt: ptr(r.ReadyAt),
			Warnings: orEmpty(warns[r.ID]), RemakeReason: r.RemakeReason.String}
		for _, m := range mods[r.ID] {
			it.Modifiers = append(it.Modifiers, m.NameID)
		}
		t.Items = append(t.Items, it)
		t.PrepTimeSeconds = max(t.PrepTimeSeconds, r.PrepTimeSeconds)
	}
	for _, key := range order {
		t := *tickets[key]
		if t.Items[0].Status == "ready" {
			out.Ready = append(out.Ready, t)
		} else {
			out.Tickets = append(out.Tickets, t)
		}
	}
	// Remakes first, then by confirmation (TR-KT-01); Ready oldest first.
	slices.SortStableFunc(out.Tickets, func(a, b KitchenTicket) int {
		if a.Remake != b.Remake {
			if a.Remake {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.ConfirmedAt, b.ConfirmedAt)
	})
	slices.SortStableFunc(out.Ready, func(a, b KitchenTicket) int { return cmp.Compare(*a.Items[0].ReadyAt, *b.Items[0].ReadyAt) })
	return out, nil
}

// queueChanged tells the Prep Area display and every Customer waiting on it: their Estimated
// Wait moved (TR-KT-04).
func queueChanged(ctx context.Context, tx *dbx.Tx, prepAreaID, orderID string) error {
	tx.Publish(realtime.Event{Topic: PrepAreaTopic(prepAreaID), Type: "queue.changed", ID: orderID})
	bills, err := tx.ListQueueBills(ctx, prepAreaID)
	if err != nil {
		return err
	}
	for _, b := range bills {
		tx.Publish(realtime.Event{Topic: BillTopic(b), Type: "wait.changed", ID: b})
	}
	return nil
}

// StartItems moves Confirmed items to Preparing (BR-KT-03).
func (s *Service) StartItems(ctx context.Context, p *auth.Principal, itemIDs []string) error {
	return s.kitchenStep(ctx, itemIDs, func(tx *dbx.Tx, id string, now int64) (int64, error) {
		return tx.StartItem(ctx, store.StartItemParams{PreparingAt: nullMs(now), UpdatedAt: now, ID: id})
	}, "item.preparing")
}

// ReadyItems moves Preparing items to Ready and signals the owning Waiter (BR-KT-03, BR-WT-09).
func (s *Service) ReadyItems(ctx context.Context, p *auth.Principal, itemIDs []string) error {
	return s.kitchenStep(ctx, itemIDs, func(tx *dbx.Tx, id string, now int64) (int64, error) {
		return tx.ReadyItem(ctx, store.ReadyItemParams{ReadyAt: nullMs(now), UpdatedAt: now, ID: id})
	}, "item.ready")
}

func (s *Service) kitchenStep(ctx context.Context, itemIDs []string, step func(*dbx.Tx, string, int64) (int64, error), kind string) error {
	if len(itemIDs) == 0 {
		return invalid("Pilih item.")
	}
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		areas := map[string]string{}
		for _, id := range itemIDs {
			it, err := tx.GetOrderItem(ctx, id)
			if errors.Is(err, sql.ErrNoRows) {
				return notFound("Item tidak ditemukan")
			}
			if err != nil {
				return err
			}
			n, err := step(tx, id, now)
			if err != nil {
				return err
			}
			if n == 0 {
				return conflict("invalid_transition", "Status item sudah berubah, muat ulang").With("itemId", id)
			}
			o, err := loadOrder(ctx, tx, it.OrderID)
			if err != nil {
				return err
			}
			orderEvents(tx, o, kind)
			areas[it.PrepAreaID] = it.OrderID
		}
		for area, orderID := range areas {
			if err := queueChanged(ctx, tx, area, orderID); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------- Prep Time report (TR-KT-05)

type PrepRow struct {
	Key    string  `json:"key"` // product, category or Prep Area id
	Name   string  `json:"name"`
	Items  int     `json:"items"`
	OnTime int     `json:"onTime"`
	Share  float64 `json:"share"` // 0..1
}

type PrepReport struct {
	Date      string    `json:"date"`
	Items     int       `json:"items"`
	OnTime    int       `json:"onTime"`
	Share     float64   `json:"share"`
	PrepAreas []PrepRow `json:"prepAreas"`
	Products  []PrepRow `json:"products"`
}

// PrepTime is the share of items Ready within their Prep Time for one business day.
func (s *Service) PrepTime(ctx context.Context, date string) (PrepReport, error) {
	day, err := time.ParseInLocation(time.DateOnly, date, platform.Jakarta)
	if err != nil {
		return PrepReport{}, invalid("Tanggal tidak valid.")
	}
	q := s.db.Read()
	rows, err := q.ListReadyItemsBetween(ctx, store.ListReadyItemsBetweenParams{ReadyAt: nullMs(platform.Millis(day.Add(4 * time.Hour))),
		ReadyAt_2: nullMs(platform.Millis(day.Add(28 * time.Hour)))})
	if err != nil {
		return PrepReport{}, err
	}
	areas, err := q.ListPrepAreas(ctx)
	if err != nil {
		return PrepReport{}, err
	}
	areaName := map[string]string{}
	for _, a := range areas {
		areaName[a.ID] = a.NameID
	}
	out := PrepReport{Date: date, PrepAreas: []PrepRow{}, Products: []PrepRow{}}
	byArea, byProduct := map[string]*PrepRow{}, map[string]*PrepRow{}
	add := func(m map[string]*PrepRow, key, name string, onTime bool) {
		r := m[key]
		if r == nil {
			r = &PrepRow{Key: key, Name: name}
			m[key] = r
		}
		r.Items++
		if onTime {
			r.OnTime++
		}
	}
	for _, r := range rows {
		onTime := r.ReadyAt.Int64-r.ConfirmedAt.Int64 <= r.PrepTimeSeconds*1000
		out.Items++
		if onTime {
			out.OnTime++
		}
		add(byArea, r.PrepAreaID, areaName[r.PrepAreaID], onTime)
		add(byProduct, r.ProductID, r.ProductNameID, onTime)
	}
	share := func(r *PrepRow) PrepRow {
		if r.Items > 0 {
			r.Share = float64(r.OnTime) / float64(r.Items)
		}
		return *r
	}
	if out.Items > 0 {
		out.Share = float64(out.OnTime) / float64(out.Items)
	}
	for _, r := range byArea {
		out.PrepAreas = append(out.PrepAreas, share(r))
	}
	for _, r := range byProduct {
		out.Products = append(out.Products, share(r))
	}
	slices.SortFunc(out.PrepAreas, func(a, b PrepRow) int { return cmp.Compare(a.Name, b.Name) })
	slices.SortFunc(out.Products, func(a, b PrepRow) int { return cmp.Compare(a.Share, b.Share) }) // worst first
	return out, nil
}
