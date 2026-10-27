package ordering

import (
	"context"
	"database/sql"
	"errors"

	"sushilatte/api/internal/kitchen"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

type Bilingual struct {
	ID string `json:"id"`
	EN string `json:"en"`
}

type TagRef struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	NameID string `json:"nameId"`
	NameEN string `json:"nameEn"`
}

type ModifierView struct {
	Name  Bilingual `json:"name"`
	Price int64     `json:"price"`
}

type ItemView struct {
	ID              string         `json:"id"`
	ProductID       string         `json:"productId"`
	VariantID       string         `json:"variantId"`
	ProductName     Bilingual      `json:"productName"`
	VariantName     Bilingual      `json:"variantName"`
	Quantity        int64          `json:"quantity"`
	Note            string         `json:"note"`
	Modifiers       []ModifierView `json:"modifiers"`
	UnitPrice       int64          `json:"unitPrice"`
	LineTotal       int64          `json:"lineTotal"` // (unit + Modifiers) × quantity, before discounts
	Status          string         `json:"status"`
	CancelReason    string         `json:"cancelReason,omitempty"`
	Warnings        []TagRef       `json:"warnings"`
	SoldOutFlag     bool           `json:"soldOutFlag"`
	UnavailableFlag bool           `json:"unavailableFlag"`
}

// Wait is the Estimated Wait of a confirmed Order (TR-KT-03, TR-KT-04): Seconds counted from
// AsOf; Minutes is rounded up for display.
type Wait struct {
	Seconds int64 `json:"seconds"`
	Minutes int64 `json:"minutes"`
	AsOf    int64 `json:"asOf"`
}

type OrderView struct {
	ID            string     `json:"id"`
	Seq           int64      `json:"seq"`
	Status        string     `json:"status"` // pending · claimed · confirmed · preparing · ready · served · cancelled
	Editable      bool       `json:"editable"`
	CreatedAt     int64      `json:"createdAt"`
	ConfirmedAt   *int64     `json:"confirmedAt,omitempty"`
	EstimatedWait *Wait      `json:"estimatedWait,omitempty"`
	Subtotal      int64      `json:"subtotal"`
	Items         []ItemView `json:"items"`
}

type BillInfo struct {
	ID       string     `json:"id"`
	Number   string     `json:"number"`
	Status   string     `json:"status"`
	Tables   []TableRef `json:"tables"`
	OpenedAt int64      `json:"openedAt"`
}

type CustomerInfo struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	RestrictionTagIDs []string `json:"restrictionTagIds"`
	DietaryNote       string   `json:"dietaryNote"`
}

// Me is everything the Client Menu shows a Customer about their visit: their own Orders only
// (BR-CM-08), never other people's.
type Me struct {
	Bill       BillInfo     `json:"bill"`
	Customer   CustomerInfo `json:"customer"`
	Orders     []OrderView  `json:"orders"`
	WaiterCall *WaiterCall  `json:"waiterCall,omitempty"`
}

func (s *Service) Me(ctx context.Context, g *Guest) (Me, error) {
	q := s.db.Read()
	bill, err := q.GetBill(ctx, g.BillID)
	if err != nil {
		return Me{}, err
	}
	e, err := q.GetCustomerEntry(ctx, g.CustomerID)
	if err != nil {
		return Me{}, err
	}
	tables, err := q.ListBillTables(ctx, bill.ID)
	if err != nil {
		return Me{}, err
	}
	out := Me{
		Bill:     BillInfo{ID: bill.ID, Number: BillNumber(bill.Number), Status: bill.Status, Tables: []TableRef{}, OpenedAt: bill.OpenedAt},
		Customer: CustomerInfo{ID: e.ID, Name: s.name(e), RestrictionTagIDs: s.restriction(e), DietaryNote: s.note(e)},
		Orders:   []OrderView{},
	}
	for _, t := range tables {
		out.Bill.Tables = append(out.Bill.Tables, TableRef{t.Number, t.Label})
	}
	if call, err := q.GetOpenWaiterCall(ctx, sql.NullString{String: e.ID, Valid: true}); err == nil {
		out.WaiterCall = &WaiterCall{ID: call.ID, CreatedAt: call.CreatedAt}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Me{}, err
	}
	orders, err := s.billOrders(ctx, q, bill.ID)
	if err != nil {
		return Me{}, err
	}
	for _, o := range orders {
		if o.customerID == e.ID {
			out.Orders = append(out.Orders, o.OrderView)
		}
	}
	return out, nil
}

type orderWithOwner struct {
	OrderView
	customerID string
}

// billOrders builds the views of every Order on a Bill, with derived status and Estimated Wait.
func (s *Service) billOrders(ctx context.Context, q *store.Queries, billID string) ([]orderWithOwner, error) {
	orders, err := q.ListOrdersByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	items, err := q.ListOrderItemsByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	mods, err := q.ListOrderItemModifiersByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	warnings, err := q.ListDietaryWarningsByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	tags, err := q.ListDietaryTags(ctx)
	if err != nil {
		return nil, err
	}
	tagByID := map[string]TagRef{}
	for _, t := range tags {
		tagByID[t.ID] = TagRef{ID: t.ID, Code: t.Code, NameID: t.NameID, NameEN: t.NameEn}
	}
	modsBy := map[string][]ModifierView{}
	modSum := map[string]int64{}
	for _, m := range mods {
		modsBy[m.OrderItemID] = append(modsBy[m.OrderItemID], ModifierView{Name: Bilingual{m.NameID, m.NameEn}, Price: m.Price})
		modSum[m.OrderItemID] += m.Price
	}
	warnBy := map[string][]TagRef{}
	for _, w := range warnings {
		warnBy[w.OrderItemID] = append(warnBy[w.OrderItemID], tagByID[w.DietaryTagID])
	}
	itemsBy := map[string][]store.OrderItem{}
	for _, it := range items {
		itemsBy[it.OrderID] = append(itemsBy[it.OrderID], it)
	}

	wait, err := s.waits(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]orderWithOwner, 0, len(orders))
	for _, o := range orders {
		v := OrderView{ID: o.ID, Seq: o.Seq, CreatedAt: o.CreatedAt, Items: []ItemView{},
			Editable: o.Status == OrderPending && !o.ClaimedBy.Valid}
		if o.ConfirmedAt.Valid {
			at := o.ConfirmedAt.Int64
			v.ConfirmedAt = &at
		}
		var statuses []string
		for _, it := range itemsBy[o.ID] {
			iv := ItemView{ID: it.ID, ProductID: it.ProductID, VariantID: it.VariantID,
				ProductName: Bilingual{it.ProductNameID, it.ProductNameEn}, VariantName: Bilingual{it.VariantNameID, it.VariantNameEn},
				Quantity: it.Quantity, Note: it.Note, Modifiers: orEmpty(modsBy[it.ID]), UnitPrice: it.UnitPrice,
				LineTotal: (it.UnitPrice + modSum[it.ID]) * it.Quantity, Status: it.Status, CancelReason: it.CancelReason.String,
				Warnings: orEmpty(warnBy[it.ID]), SoldOutFlag: it.SoldOutFlag == 1, UnavailableFlag: it.UnavailableFlag == 1}
			if it.Status != ItemCancelled && it.Status != "voided" {
				v.Subtotal += iv.LineTotal
			}
			statuses = append(statuses, it.Status)
			v.Items = append(v.Items, iv)
		}
		v.Status = OrderStatus(o.Status, o.ClaimedBy.Valid, statuses)
		if w, ok := wait(o.ID); ok {
			v.EstimatedWait = &w
		}
		out = append(out, orderWithOwner{OrderView: v, customerID: o.CustomerEntryID.String})
	}
	return out, nil
}

// OrderStatus derives what a Customer sees from the Order and its items
// (OVERVIEW-AND-FLOWS §4, "Order (derived from its items)").
func OrderStatus(stored string, claimed bool, items []string) string {
	live := 0
	count := map[string]int{}
	for _, st := range items {
		if st != ItemCancelled && st != "voided" {
			live++
			count[st]++
		}
	}
	switch {
	case stored == OrderCancelled || live == 0:
		return "cancelled"
	case stored == OrderPending && claimed, stored == "claimed":
		return "claimed"
	case stored == OrderPending:
		return "pending"
	case count["served"] == live:
		return "served"
	case count["ready"]+count["served"] == live:
		return "ready"
	case count["preparing"]+count["ready"]+count["served"] > 0:
		return "preparing"
	}
	return "confirmed"
}

// waits snapshots the Prep Area queues once and returns an Estimated Wait lookup by Order.
func (s *Service) waits(ctx context.Context, q *store.Queries) (func(orderID string) (Wait, bool), error) {
	rows, err := q.ListQueueItems(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return func(string) (Wait, bool) { return Wait{}, false }, nil
	}
	areas, err := q.ListPrepAreas(ctx)
	if err != nil {
		return nil, err
	}
	capacity := map[string]int64{}
	for _, a := range areas {
		capacity[a.ID] = a.ParallelCapacity
	}
	queue := make([]kitchen.QueueItem, 0, len(rows))
	for _, r := range rows {
		queue = append(queue, kitchen.QueueItem{ID: r.ID, OrderID: r.OrderID, PrepAreaID: r.PrepAreaID, PrepTimeSeconds: r.PrepTimeSeconds,
			Preparing: r.Status == "preparing", PreparingAt: r.PreparingAt.Int64, ConfirmedAt: r.ConfirmedAt.Int64, Remake: r.RemakeOfID.Valid})
	}
	now := platform.Millis(s.clock.Now())
	return func(orderID string) (Wait, bool) {
		sec, ok := kitchen.EstimatedWait(queue, orderID, capacity, now)
		return Wait{Seconds: sec, Minutes: (sec + 59) / 60, AsOf: now}, ok
	}, nil
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
