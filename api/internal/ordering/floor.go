package ordering

import (
	"context"
	"database/sql"
	"errors"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/store"
)

// Staff read models for the Waiter and Cashier screens (COMPONENTS.md §3).

type CrewRef struct {
	ID     string `json:"id"`
	CrewID string `json:"crewId"`
	Name   string `json:"name"`
}

// StaffOrder is an Order as staff see it: with its Table, who placed it and who owns it.
type StaffOrder struct {
	OrderView
	BillID       string     `json:"billId"`
	BillNumber   string     `json:"billNumber"`
	Tables       []TableRef `json:"tables"`
	Source       string     `json:"source"`
	CustomerName string     `json:"customerName,omitempty"`
	ClaimedBy    *CrewRef   `json:"claimedBy,omitempty"`
	Flagged      bool       `json:"flagged"`     // an open item is Sold Out or unavailable (BR-WT-05)
	HasWarnings  bool       `json:"hasWarnings"` // Dietary Warnings to highlight at Claim (BR-WT-02)
	RenotifiedAt *int64     `json:"renotifiedAt,omitempty"`
	EscalatedAt  *int64     `json:"escalatedAt,omitempty"`
}

type CallView struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Table        TableRef `json:"table"`
	BillID       string   `json:"billId,omitempty"`
	CustomerName string   `json:"customerName,omitempty"`
	CreatedAt    int64    `json:"createdAt"`
	RenotifiedAt *int64   `json:"renotifiedAt,omitempty"`
	EscalatedAt  *int64   `json:"escalatedAt,omitempty"`
}

// Feed is the Waiter's Notifications tab: unclaimed Orders and calls for everyone, plus the
// Orders this Waiter owns (Ready items to serve, flags to resolve).
type Feed struct {
	Unclaimed []StaffOrder `json:"unclaimed"`
	Calls     []CallView   `json:"calls"`
	Mine      []StaffOrder `json:"mine"`
}

type TableCard struct {
	ID     string     `json:"id"`
	Number int64      `json:"number"`
	Label  string     `json:"label"`
	Active bool       `json:"active"`
	Bill   *BillBrief `json:"bill,omitempty"`
}

type BillBrief struct {
	ID          string     `json:"id"`
	Number      string     `json:"number"`
	Status      string     `json:"status"`
	OpenedAt    int64      `json:"openedAt"`
	Tables      []TableRef `json:"tables"`
	Customers   []string   `json:"customers"`
	LiveOrders  int        `json:"liveOrders"`
	Unclaimed   int        `json:"unclaimed"`
	ReadyItems  int        `json:"readyItems"`
	Flagged     bool       `json:"flagged"`
	ClaimedByMe bool       `json:"claimedByMe"`
	Total       int64      `json:"total"`
}

type Totals struct {
	Subtotal     int64 `json:"subtotal"`
	ItemDiscount int64 `json:"itemDiscount"`
	BillDiscount int64 `json:"billDiscount"`
	Service      int64 `json:"service"`
	Tax          int64 `json:"tax"`
	Total        int64 `json:"total"`
}

type CustomerDetail struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	RestrictionTagIDs []string `json:"restrictionTagIds"`
	DietaryNote       string   `json:"dietaryNote"`
}

// StaffBill is the BillSummary: every Order on the Bill and its totals from `pricing`.
type StaffBill struct {
	BillInfo
	Type      string           `json:"type"`     // dine_in | takeaway
	Source    string           `json:"source"`   // live | post_outage
	Refunded  int64            `json:"refunded"` // cash Refunds paid on this Bill so far
	Customers []CustomerDetail `json:"customers"`
	Orders    []StaffOrder     `json:"orders"`
	Totals    Totals           `json:"totals"`
	Billing   Billing          `json:"billing"`
}

// staffContext caches what every staff view needs once per request.
type staffContext struct {
	svc     *Service
	q       *store.Queries
	crew    map[string]CrewRef
	tables  map[string][]TableRef // by bill
	tableBy map[string]TableRef   // by table id
}

func (s *Service) staffContext(ctx context.Context) (*staffContext, error) {
	q := s.db.Read()
	sc := &staffContext{svc: s, q: q, crew: map[string]CrewRef{}, tables: map[string][]TableRef{}, tableBy: map[string]TableRef{}}
	crew, err := q.ListAllCrew(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range crew {
		sc.crew[c.ID] = CrewRef{ID: c.ID, CrewID: c.CrewID, Name: c.FullName}
	}
	open, err := q.ListOpenBillTables(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range open {
		sc.tables[t.BillID] = append(sc.tables[t.BillID], TableRef{t.Number, t.Label})
	}
	all, err := q.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		sc.tableBy[t.ID] = TableRef{t.Number, t.Label}
	}
	return sc, nil
}

// billTables labels a Bill by its Tables; a Takeaway Bill by its Customer's name.
func (sc *staffContext) billTables(ctx context.Context, billID string) []TableRef {
	if t, ok := sc.tables[billID]; ok {
		return t
	}
	rows, _ := sc.q.ListBillTables(ctx, billID)
	out := []TableRef{}
	for _, r := range rows {
		out = append(out, TableRef{r.Number, r.Label})
	}
	if len(out) == 0 {
		if b, err := sc.q.GetBill(ctx, billID); err == nil && b.Type == "takeaway" {
			out = append(out, TableRef{Label: "Bawa pulang · " + sc.svc.takeawayName(b)})
		}
	}
	return out
}

// staffOrders builds every Order of a Bill as staff see it.
func (s *Service) staffOrders(ctx context.Context, sc *staffContext, bill store.Bill) ([]StaffOrder, map[string]string, error) {
	views, err := s.billOrders(ctx, sc.q, bill.ID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := sc.q.ListOrdersByBill(ctx, bill.ID)
	if err != nil {
		return nil, nil, err
	}
	entries, err := sc.q.ListCustomerEntries(ctx, bill.ID)
	if err != nil {
		return nil, nil, err
	}
	names := map[string]string{}
	for _, e := range entries {
		names[e.ID] = s.name(e)
	}
	tables := sc.billTables(ctx, bill.ID)
	out := make([]StaffOrder, 0, len(views))
	for i, v := range views {
		o := rows[i]
		so := StaffOrder{OrderView: v.OrderView, BillID: bill.ID, BillNumber: BillNumber(bill.Number), Tables: tables, Source: o.Source,
			CustomerName: names[o.CustomerEntryID.String]}
		if o.ClaimedBy.Valid {
			c := sc.crew[o.ClaimedBy.String]
			so.ClaimedBy = &c
		}
		if o.RenotifiedAt.Valid {
			so.RenotifiedAt = &o.RenotifiedAt.Int64
		}
		if o.EscalatedAt.Valid {
			so.EscalatedAt = &o.EscalatedAt.Int64
		}
		for _, it := range v.Items {
			live := it.Status == ItemPending || it.Status == "confirmed"
			so.Flagged = so.Flagged || live && (it.SoldOutFlag || it.UnavailableFlag)
			so.HasWarnings = so.HasWarnings || len(it.Warnings) > 0 && it.Status != ItemCancelled
		}
		out = append(out, so)
	}
	return out, names, nil
}

func (s *Service) Feed(ctx context.Context, p *auth.Principal) (Feed, error) {
	sc, err := s.staffContext(ctx)
	if err != nil {
		return Feed{}, err
	}
	out := Feed{Unclaimed: []StaffOrder{}, Calls: []CallView{}, Mine: []StaffOrder{}}
	bills, err := sc.q.ListOpenBills(ctx)
	if err != nil {
		return Feed{}, err
	}
	customerNames := map[string]string{}
	for _, b := range bills {
		orders, names, err := s.staffOrders(ctx, sc, b)
		if err != nil {
			return Feed{}, err
		}
		for k, v := range names {
			customerNames[k] = v
		}
		for _, o := range orders {
			switch {
			case o.Status == "pending":
				out.Unclaimed = append(out.Unclaimed, o)
			case o.ClaimedBy != nil && o.ClaimedBy.ID == p.ID && o.Status != "cancelled" && o.Status != "served":
				out.Mine = append(out.Mine, o)
			}
		}
	}
	calls, err := sc.q.ListOpenWaiterCalls(ctx)
	if err != nil {
		return Feed{}, err
	}
	for _, c := range calls {
		v := CallView{ID: c.ID, Kind: c.Kind, Table: sc.tableBy[c.TableID], BillID: c.BillID.String, CustomerName: customerNames[c.CustomerEntryID.String],
			CreatedAt: c.CreatedAt}
		if c.RenotifiedAt.Valid {
			v.RenotifiedAt = &c.RenotifiedAt.Int64
		}
		if c.EscalatedAt.Valid {
			v.EscalatedAt = &c.EscalatedAt.Int64
		}
		out.Calls = append(out.Calls, v)
	}
	return out, nil
}

// Floor is the TableMap: every Table with its open Bill, if any.
func (s *Service) Floor(ctx context.Context, p *auth.Principal) ([]TableCard, error) {
	sc, err := s.staffContext(ctx)
	if err != nil {
		return nil, err
	}
	tables, err := sc.q.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	open, err := sc.q.ListOpenBillTables(ctx)
	if err != nil {
		return nil, err
	}
	billOf := map[string]string{}
	for _, t := range open {
		billOf[t.TableID] = t.BillID
	}
	briefs := map[string]*BillBrief{}
	out := make([]TableCard, 0, len(tables))
	for _, t := range tables {
		card := TableCard{ID: t.ID, Number: t.Number, Label: t.Label, Active: t.Active == 1}
		if id, ok := billOf[t.ID]; ok {
			if briefs[id] == nil {
				b, err := s.brief(ctx, sc, p, id)
				if err != nil {
					return nil, err
				}
				briefs[id] = b
			}
			card.Bill = briefs[id]
		}
		out = append(out, card)
	}
	return out, nil
}

func (s *Service) brief(ctx context.Context, sc *staffContext, p *auth.Principal, billID string) (*BillBrief, error) {
	bill, err := sc.q.GetBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	orders, names, err := s.staffOrders(ctx, sc, bill)
	if err != nil {
		return nil, err
	}
	bl, err := s.billing(ctx, sc.q, bill)
	if err != nil {
		return nil, err
	}
	b := &BillBrief{ID: bill.ID, Number: BillNumber(bill.Number), Status: bill.Status, OpenedAt: bill.OpenedAt, Tables: sc.billTables(ctx, billID),
		Customers: []string{}, Total: bl.Totals.Total}
	entries, _ := sc.q.ListCustomerEntries(ctx, billID)
	for _, e := range entries {
		b.Customers = append(b.Customers, names[e.ID])
	}
	for _, o := range orders {
		if o.Status == "cancelled" {
			continue
		}
		b.LiveOrders++
		if o.Status == "pending" {
			b.Unclaimed++
		}
		b.Flagged = b.Flagged || o.Flagged
		b.ClaimedByMe = b.ClaimedByMe || o.ClaimedBy != nil && o.ClaimedBy.ID == p.ID
		for _, it := range o.Items {
			if it.Status == "ready" {
				b.ReadyItems++
			}
		}
	}
	return b, nil
}

// Bill is the staff BillSummary.
func (s *Service) Bill(ctx context.Context, billID string) (StaffBill, error) {
	sc, err := s.staffContext(ctx)
	if err != nil {
		return StaffBill{}, err
	}
	bill, err := sc.q.GetBill(ctx, billID)
	if errors.Is(err, sql.ErrNoRows) {
		return StaffBill{}, notFound("Tagihan tidak ditemukan")
	}
	if err != nil {
		return StaffBill{}, err
	}
	orders, _, err := s.staffOrders(ctx, sc, bill)
	if err != nil {
		return StaffBill{}, err
	}
	bl, err := s.billing(ctx, sc.q, bill)
	if err != nil {
		return StaffBill{}, err
	}
	out := StaffBill{BillInfo: BillInfo{ID: bill.ID, Number: BillNumber(bill.Number), Status: bill.Status, Tables: sc.billTables(ctx, billID), OpenedAt: bill.OpenedAt},
		Type: bill.Type, Source: bill.Source, Customers: []CustomerDetail{}, Orders: orders, Totals: bl.Totals, Billing: bl}
	refunds, err := sc.q.ListRefundsByBill(ctx, billID)
	if err != nil {
		return StaffBill{}, err
	}
	for _, r := range refunds {
		out.Refunded += r.Amount
	}
	entries, err := sc.q.ListCustomerEntries(ctx, billID)
	if err != nil {
		return StaffBill{}, err
	}
	for _, e := range entries {
		out.Customers = append(out.Customers, CustomerDetail{ID: e.ID, Name: s.name(e), RestrictionTagIDs: s.restriction(e), DietaryNote: s.note(e)})
	}
	return out, nil
}

func billLines(ctx context.Context, q *store.Queries, billID string) ([]pricing.Line, error) {
	items, err := q.ListOrderItemsByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	mods, err := q.ListOrderItemModifiersByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	modPrices := map[string][]int64{}
	for _, m := range mods {
		modPrices[m.OrderItemID] = append(modPrices[m.OrderItemID], m.Price)
	}
	lines := make([]pricing.Line, 0, len(items))
	for _, it := range items {
		lines = append(lines, pricing.Line{ID: it.ID, CategoryID: it.CategoryID, ProductID: it.ProductID, VariantID: it.VariantID,
			UnitPrice: it.UnitPrice, Modifiers: modPrices[it.ID], Quantity: it.Quantity,
			Excluded: it.Status == ItemCancelled || it.Status == "voided"})
	}
	return lines, nil
}
