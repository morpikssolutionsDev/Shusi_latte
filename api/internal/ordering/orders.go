package ordering

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

const (
	maxQuantity = 20
	maxItemNote = 140
	maxItems    = 30

	ItemPending   = "pending"
	ItemCancelled = "cancelled"

	OrderPending   = "pending"
	OrderCancelled = "cancelled"
)

// ItemInput is one cart line. ID is set when editing an existing Pending item.
type ItemInput struct {
	ID                 string   `json:"id,omitempty"`
	VariantID          string   `json:"variantId"`
	ModifierIDs        []string `json:"modifierIds"`
	Quantity           int64    `json:"quantity"`
	Note               string   `json:"note"`
	AcknowledgedTagIDs []string `json:"acknowledgedTagIds"`
}

// ItemProblem says why one cart line cannot be ordered, so the Client Menu can mark it (OP-02).
type ItemProblem struct {
	Index   int      `json:"index"`
	Code    string   `json:"code"` // unknown · unavailable · sold_out · bad_quantity · modifier_rule · note_too_long · dietary_ack_required
	TagIDs  []string `json:"tagIds,omitempty"`
	Message string   `json:"message"`
}

// Warnings is BR-R-D2 / TR-DM-05: (Product tags ∪ chosen Modifier tags) ∩ restriction, in
// restriction order, without duplicates.
func Warnings(productTags []string, modifierTags [][]string, restriction []string) []string {
	out := []string{}
	for _, r := range restriction {
		hit := slices.Contains(productTags, r)
		for _, m := range modifierTags {
			hit = hit || slices.Contains(m, r)
		}
		if hit && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

type resolved struct {
	product   catalog.Product
	variant   catalog.Variant
	modifiers []catalog.Modifier
	quantity  int64
	note      string
	warnings  []string
}

type pricedLine struct {
	product   catalog.Product
	variant   catalog.Variant
	modifiers []catalog.Modifier
	quantity  int64
}

// priceLines prices cart lines with the automatic discounts active now (TR-PR-01, TR-PR-03).
func priceLines(lines []pricedLine, auto []pricing.Discount) pricing.Breakdown {
	in := pricing.Input{Auto: auto}
	for i, l := range lines {
		pl := pricing.Line{ID: strconv.Itoa(i), CategoryID: l.product.CategoryID, ProductID: l.product.ID, VariantID: l.variant.ID,
			UnitPrice: l.variant.Price, Quantity: l.quantity}
		for _, m := range l.modifiers {
			pl.Modifiers = append(pl.Modifiers, m.Price)
		}
		in.Lines = append(in.Lines, pl)
	}
	return pricing.Calculate(in)
}

// resolve checks one line against the live catalog (TR-DM-02 guard: Product Active, Variant
// not Sold Out) and computes its Dietary Warnings. requireAck enforces that every warning
// was acknowledged (BR-CM-11); a quote only reports them.
func resolve(cat catalog.Catalog, i int, in ItemInput, restriction []string, requireAck bool) (resolved, *ItemProblem) {
	bad := func(code, msg string) (resolved, *ItemProblem) {
		return resolved{}, &ItemProblem{Index: i, Code: code, Message: msg}
	}
	var p catalog.Product
	var v catalog.Variant
	found := false
	for _, prod := range cat.Products {
		if idx := slices.IndexFunc(prod.Variants, func(x catalog.Variant) bool { return x.ID == in.VariantID }); idx >= 0 {
			p, v, found = prod, prod.Variants[idx], true
			break
		}
	}
	switch {
	case !found:
		return bad("unknown", "Menu ini tidak ditemukan.")
	case p.Status != catalog.StatusActive:
		return bad("unavailable", "Menu ini sedang tidak tersedia. Pilih menu lain, ya.")
	case v.SoldOut:
		return bad("sold_out", "Item ini baru saja habis. Pilih menu lain, ya.")
	case in.Quantity < 1 || in.Quantity > maxQuantity:
		return bad("bad_quantity", "Jumlah 1–20.")
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > maxItemNote {
		return bad("note_too_long", "Catatan maksimal 140 huruf.")
	}
	r := resolved{product: p, variant: v, quantity: in.Quantity, note: note}
	var modTags [][]string
	for _, g := range p.ModifierGroups {
		n := int64(0)
		for _, m := range g.Modifiers {
			if slices.Contains(in.ModifierIDs, m.ID) {
				n++
				r.modifiers = append(r.modifiers, m)
				modTags = append(modTags, m.DietaryTagIDs)
			}
		}
		if n < g.MinSelect || n > g.MaxSelect {
			return bad("modifier_rule", "Pilihan "+g.NameID+" belum sesuai.")
		}
	}
	if len(r.modifiers) != len(uniq(in.ModifierIDs)) {
		return bad("modifier_rule", "Pilihan tidak dikenal.")
	}
	r.warnings = Warnings(p.DietaryTagIDs, modTags, restriction)
	if requireAck {
		var missing []string
		for _, w := range r.warnings {
			if !slices.Contains(in.AcknowledgedTagIDs, w) {
				missing = append(missing, w)
			}
		}
		if len(missing) > 0 {
			return resolved{}, &ItemProblem{Index: i, Code: "dietary_ack_required", TagIDs: missing, Message: "Konfirmasi peringatan diet dulu, ya."}
		}
	}
	return r, nil
}

func uniq(ids []string) []string {
	var out []string
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func rejected(problems []ItemProblem) error {
	p := httpx.NewProblem(http.StatusConflict, "order_rejected", "Beberapa item tidak bisa dipesan")
	return p.With("items", problems)
}

// openBillFor re-reads the Guest's Bill inside the transaction: Settling blocks ordering
// (BR-CM-15) and a closed Bill ends the session.
func openBillFor(ctx context.Context, tx *dbx.Tx, g *Guest) (store.Bill, error) {
	b, err := tx.GetBill(ctx, g.BillID)
	if err != nil {
		return b, err
	}
	switch b.Status {
	case BillOpen:
		return b, nil
	case BillSettling:
		return b, httpx.NewProblem(http.StatusConflict, "bill_settling", "Tagihan sedang dibayar — panggil pelayan untuk pesan lagi.")
	}
	return b, httpx.NewProblem(http.StatusUnauthorized, "bill_closed", "Tagihan ini sudah ditutup. Pindai QR meja untuk pesan lagi.")
}

// ---------------------------------------------------------------- quote

type QuoteLine struct {
	Index         int          `json:"index"`
	Gross         int64        `json:"gross"`
	Discount      int64        `json:"discount"`
	Net           int64        `json:"net"`
	WarningTagIDs []string     `json:"warningTagIds"`
	Problem       *ItemProblem `json:"problem,omitempty"`
}

// Quote prices a cart for the CartSheet without saving it. Tax and service charge are added
// at payment (BR-CM-09), so the quote stops at the item subtotal.
type Quote struct {
	Lines        []QuoteLine `json:"lines"`
	Subtotal     int64       `json:"subtotal"`
	ItemDiscount int64       `json:"itemDiscount"`
	Total        int64       `json:"total"`
}

func (s *Service) Quote(ctx context.Context, g *Guest, items []ItemInput) (Quote, error) {
	if len(items) > maxItems {
		return Quote{}, invalid("Terlalu banyak item dalam satu pesanan.")
	}
	q := s.db.Read()
	cat, err := catalog.Load(ctx, q, false)
	if err != nil {
		return Quote{}, err
	}
	e, err := q.GetCustomerEntry(ctx, g.CustomerID)
	if err != nil {
		return Quote{}, err
	}
	discounts, err := s.outlet.Discounts(ctx)
	if err != nil {
		return Quote{}, err
	}
	restriction := s.restriction(e)
	out := Quote{Lines: []QuoteLine{}}
	var lines []pricedLine
	var at []int
	for i, in := range items {
		r, prob := resolve(cat, i, in, restriction, false)
		if prob != nil {
			out.Lines = append(out.Lines, QuoteLine{Index: i, WarningTagIDs: []string{}, Problem: prob})
			continue
		}
		lines = append(lines, pricedLine{r.product, r.variant, r.modifiers, r.quantity})
		at = append(at, len(out.Lines))
		out.Lines = append(out.Lines, QuoteLine{Index: i, WarningTagIDs: r.warnings})
	}
	if len(lines) > 0 {
		b := priceLines(lines, outlet.ActiveDiscounts(discounts, s.clock.Now()))
		for k, lr := range b.Lines {
			l := &out.Lines[at[k]]
			l.Gross, l.Discount, l.Net = lr.Gross, lr.Discount, lr.Net
		}
		out.Subtotal, out.ItemDiscount, out.Total = b.Subtotal, b.ItemDiscount, b.Subtotal-b.ItemDiscount
	}
	return out, nil
}

// ---------------------------------------------------------------- place, edit, cancel

// PlaceOrder creates a Pending Order from the Customer's cart (BR-CM-05, BE-03). Every line is
// re-checked on the server; if any fails, nothing is saved and each failing line is reported.
func (s *Service) PlaceOrder(ctx context.Context, g *Guest, items []ItemInput) (string, error) {
	if len(items) == 0 {
		return "", invalid("Keranjangmu masih kosong.")
	}
	if len(items) > maxItems {
		return "", invalid("Terlalu banyak item dalam satu pesanan.")
	}
	now := platform.Millis(s.clock.Now())
	orderID := platform.NewID()
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := openBillFor(ctx, tx, g); err != nil {
			return err
		}
		cat, err := catalog.Load(ctx, tx.Queries, false)
		if err != nil {
			return err
		}
		e, err := tx.GetCustomerEntry(ctx, g.CustomerID)
		if err != nil {
			return err
		}
		lines, err := resolveAll(cat, items, s.restriction(e))
		if err != nil {
			return err
		}
		seq, err := tx.NextOrderSeq(ctx, g.BillID)
		if err != nil {
			return err
		}
		if err := tx.CreateOrder(ctx, store.CreateOrderParams{ID: orderID, BillID: g.BillID, Seq: seq,
			CustomerEntryID: sql.NullString{String: g.CustomerID, Valid: true}, Source: "client_menu", Status: OrderPending,
			CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		if err := insertItems(ctx, tx, cat, orderID, 0, lines, now); err != nil {
			return err
		}
		tx.Publish(
			realtime.Event{Topic: WaitersTopic, Type: "order.pending", ID: orderID},
			realtime.Event{Topic: BillTopic(g.BillID), Type: "order.created", ID: orderID},
		)
		return nil
	})
	return orderID, err
}

func resolveAll(cat catalog.Catalog, items []ItemInput, restriction []string) ([]resolved, error) {
	var lines []resolved
	var problems []ItemProblem
	for i, in := range items {
		r, prob := resolve(cat, i, in, restriction, true)
		if prob != nil {
			problems = append(problems, *prob)
			continue
		}
		lines = append(lines, r)
	}
	if len(problems) > 0 {
		return nil, rejected(problems)
	}
	return lines, nil
}

// insertItems writes Order Items with their snapshots (TR-DM-02), Modifiers and acknowledged
// Dietary Warnings (TR-DM-05).
func insertItems(ctx context.Context, tx *dbx.Tx, cat catalog.Catalog, orderID string, sortFrom int, lines []resolved, now int64) error {
	prepArea := map[string]string{}
	for _, c := range cat.Categories {
		prepArea[c.ID] = c.PrepAreaID
	}
	for i, l := range lines {
		id := platform.NewID()
		if err := tx.CreateOrderItem(ctx, store.CreateOrderItemParams{ID: id, OrderID: orderID, Sort: int64(sortFrom + i + 1),
			ProductID: l.product.ID, VariantID: l.variant.ID, CategoryID: l.product.CategoryID, PrepAreaID: prepArea[l.product.CategoryID],
			ProductNameID: l.product.NameID, ProductNameEn: l.product.NameEN, VariantNameID: l.variant.NameID, VariantNameEn: l.variant.NameEN,
			UnitPrice: l.variant.Price, PrepTimeSeconds: l.product.PrepTimeSeconds, Quantity: l.quantity, Note: l.note,
			Status: ItemPending, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		for _, m := range l.modifiers {
			if err := tx.CreateOrderItemModifier(ctx, store.CreateOrderItemModifierParams{OrderItemID: id, ModifierID: m.ID,
				NameID: m.NameID, NameEn: m.NameEN, Price: m.Price}); err != nil {
				return err
			}
		}
		for _, tag := range l.warnings {
			if err := tx.CreateDietaryWarning(ctx, store.CreateDietaryWarningParams{OrderItemID: id, DietaryTagID: tag,
				AcknowledgedAt: sql.NullInt64{Int64: now, Valid: true}}); err != nil {
				return err
			}
		}
	}
	return nil
}

// ownPendingOrder loads an Order the Guest may still change: theirs, Pending and not Claimed
// (BR-CM-14, G-03).
func ownPendingOrder(ctx context.Context, tx *dbx.Tx, g *Guest, orderID string) (store.Order, error) {
	o, err := tx.GetOrder(ctx, orderID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (o.BillID != g.BillID || o.CustomerEntryID.String != g.CustomerID) {
		return o, httpx.NewProblem(http.StatusNotFound, "order_not_found", "Pesanan tidak ditemukan.")
	}
	if err != nil {
		return o, err
	}
	if o.Status != OrderPending || o.ClaimedBy.Valid {
		return o, httpx.NewProblem(http.StatusConflict, "order_claimed", "Pesanan sudah diterima pelayan. Minta pelayan untuk mengubahnya.")
	}
	return o, nil
}

// EditOrder replaces the lines of the Guest's own Pending Order: kept lines may change
// quantity and note, missing lines are Cancelled (Customer Request), new lines are added.
func (s *Service) EditOrder(ctx context.Context, g *Guest, orderID string, items []ItemInput) error {
	if len(items) > maxItems {
		return invalid("Terlalu banyak item dalam satu pesanan.")
	}
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := openBillFor(ctx, tx, g); err != nil {
			return err
		}
		if _, err := ownPendingOrder(ctx, tx, g, orderID); err != nil {
			return err
		}
		existing, err := tx.ListOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		cat, err := catalog.Load(ctx, tx.Queries, false)
		if err != nil {
			return err
		}
		e, err := tx.GetCustomerEntry(ctx, g.CustomerID)
		if err != nil {
			return err
		}
		var fresh []ItemInput
		kept := map[string]bool{}
		var problems []ItemProblem
		for i, in := range items {
			if in.ID == "" {
				fresh = append(fresh, in)
				continue
			}
			idx := slices.IndexFunc(existing, func(x store.OrderItem) bool { return x.ID == in.ID && x.Status == ItemPending })
			note := strings.TrimSpace(in.Note)
			switch {
			case idx < 0:
				problems = append(problems, ItemProblem{Index: i, Code: "unknown", Message: "Item ini sudah tidak bisa diubah."})
			case in.Quantity < 1 || in.Quantity > maxQuantity:
				problems = append(problems, ItemProblem{Index: i, Code: "bad_quantity", Message: "Jumlah 1–20."})
			case utf8.RuneCountInString(note) > maxItemNote:
				problems = append(problems, ItemProblem{Index: i, Code: "note_too_long", Message: "Catatan maksimal 140 huruf."})
			default:
				kept[in.ID] = true
				if _, err := tx.UpdatePendingItem(ctx, store.UpdatePendingItemParams{Quantity: in.Quantity, Note: note, UpdatedAt: now, ID: in.ID}); err != nil {
					return err
				}
			}
		}
		lines, err := resolveAll(cat, fresh, s.restriction(e))
		if err != nil {
			var p *httpx.Problem
			if errors.As(err, &p) {
				// Report new-line problems at their index in the full list.
				for _, fp := range p.Extra["items"].([]ItemProblem) {
					fp.Index = indexOfNth(items, fp.Index)
					problems = append(problems, fp)
				}
			} else {
				return err
			}
		}
		if len(problems) > 0 {
			return rejected(problems)
		}
		live := 0
		for _, it := range existing {
			if it.Status != ItemPending {
				continue
			}
			if !kept[it.ID] {
				if err := cancelPending(ctx, tx, it.ID, now); err != nil {
					return err
				}
				continue
			}
			live++
		}
		if err := insertItems(ctx, tx, cat, orderID, len(existing), lines, now); err != nil {
			return err
		}
		live += len(lines)
		status := OrderPending
		if live == 0 {
			status = OrderCancelled
		}
		if err := tx.SetOrderStatus(ctx, store.SetOrderStatusParams{Status: status, UpdatedAt: now, ID: orderID}); err != nil {
			return err
		}
		tx.Publish(
			realtime.Event{Topic: WaitersTopic, Type: "order.updated", ID: orderID},
			realtime.Event{Topic: BillTopic(g.BillID), Type: "order.updated", ID: orderID},
		)
		return nil
	})
}

// indexOfNth maps the n-th new line (no ID) back to its index in the full item list.
func indexOfNth(items []ItemInput, n int) int {
	for i, in := range items {
		if in.ID == "" {
			if n == 0 {
				return i
			}
			n--
		}
	}
	return n
}

func cancelPending(ctx context.Context, tx *dbx.Tx, itemID string, now int64) error {
	_, err := tx.CancelItem(ctx, store.CancelItemParams{CancelReason: sql.NullString{String: "customer_request", Valid: true},
		CancelledAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now, ID: itemID, Status: ItemPending})
	return err
}

// CancelOrder Cancels every item of the Guest's own Pending Order (Customer Request).
func (s *Service) CancelOrder(ctx context.Context, g *Guest, orderID string) error {
	now := platform.Millis(s.clock.Now())
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		if _, err := ownPendingOrder(ctx, tx, g, orderID); err != nil {
			return err
		}
		items, err := tx.ListOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		for _, it := range items {
			if it.Status == ItemPending {
				if err := cancelPending(ctx, tx, it.ID, now); err != nil {
					return err
				}
			}
		}
		if err := tx.SetOrderStatus(ctx, store.SetOrderStatusParams{Status: OrderCancelled, UpdatedAt: now, ID: orderID}); err != nil {
			return err
		}
		tx.Publish(
			realtime.Event{Topic: WaitersTopic, Type: "order.cancelled", ID: orderID},
			realtime.Event{Topic: BillTopic(g.BillID), Type: "order.cancelled", ID: orderID},
		)
		return nil
	})
}
