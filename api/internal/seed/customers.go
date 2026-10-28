package seed

import (
	"context"
	"fmt"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/outlet"
)

// TableScenarios labels each Table on the demo launcher (DEMO-AND-SEED §6.1). Keep in sync.
var TableScenarios = map[int64]string{
	1:  "Kosong — scan pertama, nama + pantangan + persetujuan",
	2:  "Tagihan kosong sejak −25 mnt — tutup otomatis",
	3:  "Raka (hindari susu) & Laras — prompt Rejoin, peringatan diet",
	4:  "Bima — pesanan menunggu, panggilan pelayan dieskalasi",
	5:  "Tasya & Yoga — klaim, tanda Habis dan Disembunyikan",
	6:  "Dewi (hindari gluten) & Kevin — timer dapur, perkiraan waktu",
	7:  "Citra — siap diambil, Remake",
	8:  "Gilang, Nabila, Reza — Merge dengan Meja 9, sedang dibayar",
	9:  "Digabung ke Meja 8",
	10: "Intan & Fikri — split per item, Void",
	11: "Ayu — batal sendiri, batal dapur (Habis)",
	12: "Nonaktif — QR-nya meminta tamu memanggil pelayan",
}

const m = time.Minute

// seedCustomers plays the live Tables of DEMO-AND-SEED §6.1 forward to T0, in story order:
// Customers scan and order, Waiters Claim and confirm, the Kitchen works, Bills are merged,
// split and paid. Every step goes through the domain services at its own time offset.
func seedCustomers(ctx context.Context, db *dbx.DB, opts Options, ids catalogIDs, crew map[string]string) error {
	tables, err := outlet.NewService(db, fixedClock{opts.T0}).Tables(ctx)
	if err != nil {
		return err
	}
	qr, tableIDs := map[int64]string{}, map[int64]string{}
	for _, t := range tables {
		qr[t.Number], tableIDs[t.Number] = t.Code, t.ID
	}
	base := ordering.NewService(db, fixedClock{opts.T0}, opts.Vault)
	s := &liveSeeder{ctx: ctx, db: db, t0: opts.T0, base: base, ids: ids, qr: qr, tableIDs: tableIDs, crewIDs: crew}

	// The past first, so every Table is free again when the live story starts (§6.3, §7, §8).
	s.history()
	s.today()
	w1, w2, w3, a1 := s.crew("W001"), s.crew("W002"), s.crew("W003"), s.crew("A001")
	k1, k3 := s.crew("K001"), s.crew("K003")

	// Meja 8 + 9: a long visit, merged, now Settling with cash Held by Waiter W002 (G-04, G-20).
	gilang := s.join(8, -80*m, "Gilang")
	nabila := s.join(8, -79*m, "Nabila")
	reza := s.join(9, -75*m, "Reza")
	o8a := s.order(gilang, -70*m, s.line("sashimi", 1, 1), s.line("salmon-aburi", 1, 1))
	o8b := s.order(nabila, -70*m, s.line("crepe", 0, 1), s.line("hojicha", 0, 1, "Susu segar"))
	o9 := s.order(reza, -69*m, s.line("katsu", 0, 1), s.line("cold-brew", 0, 1))
	for _, o := range []string{o8a, o8b, o9} {
		s.claimConfirm(w2, o, -68*m)
		s.cook(k1, o, -65*m, -50*m)
		s.serve(w2, o, -48*m)
	}
	s.merge(w2, gilang, reza, -50*m)
	s.pay(w2, gilang, -2*m, ordering.PayInput{Method: ordering.PayCash, Amount: 200000}, "")

	// Meja 10: split by item; Intan's Sub-bill paid by QRIS and printed; Fikri's cheesecake
	// Voided on his unpaid Sub-bill (G-10, G-12).
	intan := s.join(10, -60*m, "Intan")
	fikri := s.join(10, -59*m, "Fikri")
	o10a := s.order(intan, -57*m, s.line("matcha", 0, 1, "Susu segar"), s.line("salmon-aburi", 0, 1))
	o10b := s.order(fikri, -57*m, s.line("spicy-salmon", 0, 1), s.line("cheesecake", 0, 1))
	for _, o := range []string{o10a, o10b} {
		s.claimConfirm(w3, o, -55*m)
		s.cook(k1, o, -52*m, -40*m)
		s.serve(w3, o, -35*m)
	}
	s.splitByCustomer(w3, intan, -15*m, map[string]int{o10a: 1, o10b: 2})
	s.payShare(w3, intan, 1, -12*m)
	s.void(w3, a1, o10b, "cheesecake", "Jatuh saat disajikan", -10*m)

	// Meja 3: Laras's Order Served; Raka (avoids Dairy) just ordered, unclaimed (G-02).
	raka := s.join(3, -52*m, "Raka", "dairy")
	laras := s.join(3, -51*m, "Laras")
	oLaras := s.order(laras, -45*m, s.line("edamame", 0, 1))
	s.claimConfirm(w1, oLaras, -44*m)
	s.cook(k1, oLaras, -42*m, -38*m)
	s.serve(w1, oLaras, -36*m)

	// Meja 7: cold brew served; Spicy Salmon served then Remade (A001, "Kurang pedas");
	// Dragon Roll Ready since −4m, waiting pickup (G-08, G-09).
	citra := s.join(7, -40*m, "Citra")
	o7 := s.order(citra, -35*m, s.line("dragon", 0, 1), s.line("cold-brew", 0, 1), s.line("spicy-salmon", 0, 1))
	s.claimConfirm(w1, o7, -33*m)
	s.cookItem(k3, o7, "cold-brew", -32*m, -30*m)
	s.cookItem(k1, o7, "spicy-salmon", -30*m, -20*m)
	s.serveItems(w1, o7, -29*m, "cold-brew")
	s.serveItems(w1, o7, -18*m, "spicy-salmon")
	s.cookItem(k1, o7, "dragon", -20*m, -4*m)
	s.remake(w1, a1, o7, "spicy-salmon", "Kurang pedas", -8*m)

	// Meja 11: Ayu removed an item herself before Claim (G-03); the Bar then Cancels the Iced
	// Hojicha as Out of Stock (Sold Out everywhere, OP-08) and the Kitchen Cancels the crêpe
	// from Confirmed (Wrong Order, G-07); the curry is Preparing.
	ayu := s.join(11, -40*m, "Ayu")
	o11 := s.order(ayu, -35*m, s.line("hojicha", 1, 1, "Susu segar"), s.line("crepe", 0, 1), s.line("katsu", 0, 1), s.line("edamame", 0, 1))
	s.editDropLast(ayu, o11, -33*m)
	s.claimConfirm(w1, o11, -31*m)

	// Meja 5: ordered before the Hojicha ran out and before Sakura went off the menu; Claimed by
	// W001 at −1m, not yet confirmed — both flags show (OP-08, G-23).
	tasya := s.join(5, -30*m, "Tasya")
	s.join(5, -29*m, "Yoga")
	o5 := s.order(tasya, -22*m, s.line("hojicha", 1, 1, "Susu segar"), s.line("sakura", 1, 1, "Susu segar"), s.line("v60", 2, 1))
	s.kitchenCancel(k3, o11, "hojicha", ordering.ReasonOutOfStock, "", -20*m)
	s.kitchenCancel(k1, o11, "crepe", ordering.ReasonWrongOrder, "", -18*m)
	s.cookItem(k1, o11, "katsu", -10*m, 0)
	s.hide("sakura", -15*m)
	s.claim(w1, o5, -1*m)

	// Meja 6: Kitchen timers on time, amber and late; Dewi's cup-fill wait (M9).
	dewi := s.join(6, -20*m, "Dewi", "gluten")
	kevin := s.join(6, -19*m, "Kevin")
	gyoza := s.order(dewi, -13*m, s.line("gyoza", 1, 1).ack("gluten"))
	s.claimConfirm(w3, gyoza, -11*m) // Confirmed 11 of 10 min: late
	ramen := s.order(kevin, -12*m, s.line("tonkotsu", 0, 1))
	s.claimConfirm(w3, ramen, -10*m)
	s.cookItem(k1, ramen, "tonkotsu", -10*m, 0) // Preparing 10 of 12: amber
	wagyu := s.order(kevin, -6*m, s.line("wagyu", 0, 1).ack("gluten"))
	s.claimConfirm(w3, wagyu, -5*m)
	s.cookItem(k1, wagyu, "wagyu", -5*m, 0) // Preparing 5 of 12: on time
	matcha := s.order(dewi, -3*m, s.line("matcha", 1, 1, "Susu segar"))
	s.claimConfirm(w3, matcha, -2*m)
	s.cookItem(k3, matcha, "matcha", -2*m, 0) // Bar: Preparing 2 of 4

	// Meja 2: opened by a scan 25 minutes ago, no Orders — auto-closes at T0+5m (G-01).
	s.join(2, -25*m, "Wulan")

	// Meja 4: Bima's Order waits unclaimed since −3m; his Waiter Call since −6m (G-05).
	bima := s.join(4, -12*m, "Bima")
	s.call(bima, -6*m)
	s.order(bima, -3*m, s.line("katsu", 0, 1, "Sedang"))

	// Meja 3, last: Raka's fresh Order with an acknowledged Dietary Warning.
	s.order(raka, -40*time.Second, s.line("latte", 0, 1, "Susu segar").ack("dairy"), s.line("salmon-aburi", 0, 1))

	// Bills closed more than 48 hours ago lose their Customer names, and data keys older than
	// three days are destroyed, as the nightly jobs do (TR-RET-01, TR-RET-04, G-24).
	if s.err == nil {
		_, err := base.Anonymise(ctx)
		s.fail(err, "anonymise")
		_, err = opts.Vault.DestroyBefore(opts.T0)
		s.fail(err, "destroy old keys")
	}
	return s.err
}

type liveSeeder struct {
	ctx      context.Context
	db       *dbx.DB
	t0       time.Time
	base     *ordering.Service
	ids      catalogIDs
	qr       map[int64]string
	tableIDs map[int64]string
	crewIDs  map[string]string
	err      error
}

func (s *liveSeeder) at(offset time.Duration) *ordering.Service {
	return s.base.WithClock(fixedClock{s.t0.Add(offset)})
}

func (s *liveSeeder) fail(err error, what string, args ...any) {
	if err != nil && s.err == nil {
		s.err = fmt.Errorf(what+": %w", append(args, err)...)
	}
}

func (s *liveSeeder) crew(crewID string) *auth.Principal {
	for _, a := range DemoCrew {
		if a.CrewID == crewID {
			return &auth.Principal{ID: s.crewIDs[crewID], CrewID: a.CrewID, Name: a.Name, Role: a.Role, Stations: a.Stations, Kind: "crew"}
		}
	}
	return nil
}

type seededLine struct {
	ordering.ItemInput
	s *liveSeeder
}

func (l seededLine) ack(tags ...string) seededLine {
	for _, t := range tags {
		l.AcknowledgedTagIDs = append(l.AcknowledgedTagIDs, l.s.ids.tags[t])
	}
	return l
}

// line picks a Product's Variant by index and Modifiers by Indonesian name.
func (s *liveSeeder) line(product string, variant int, qty int64, modifiers ...string) seededLine {
	p := s.ids.products[product]
	in := ordering.ItemInput{VariantID: p.Variants[variant].ID, Quantity: qty}
	for _, name := range modifiers {
		in.ModifierIDs = append(in.ModifierIDs, modifierID(p, name))
	}
	return seededLine{in, s}
}

func modifierID(p catalog.Product, nameID string) string {
	for _, g := range p.ModifierGroups {
		for _, m := range g.Modifiers {
			if m.NameID == nameID {
				return m.ID
			}
		}
	}
	return "missing:" + nameID
}

func (s *liveSeeder) join(table int64, offset time.Duration, name string, avoids ...string) *ordering.Guest {
	if s.err != nil {
		return nil
	}
	in := ordering.JoinInput{Name: name, Consent: len(avoids) > 0}
	for _, t := range avoids {
		in.RestrictionTagIDs = append(in.RestrictionTagIDs, s.ids.tags[t])
	}
	svc := s.at(offset)
	session, err := svc.Join(s.ctx, s.qr[table], in)
	if err != nil {
		s.fail(err, "join Meja %d as %s", table, name)
		return nil
	}
	g, err := svc.Authenticate(s.ctx, session.Token)
	s.fail(err, "authenticate %s", name)
	return g
}

func (s *liveSeeder) order(g *ordering.Guest, offset time.Duration, lines ...seededLine) string {
	if s.err != nil {
		return ""
	}
	items := make([]ordering.ItemInput, len(lines))
	for i, l := range lines {
		items[i] = l.ItemInput
	}
	id, err := s.at(offset).PlaceOrder(s.ctx, g, items)
	s.fail(err, "order at %s", offset)
	return id
}

// itemID finds an Order's live item for a seeded Product key.
func (s *liveSeeder) itemID(orderID, product string) string {
	items, err := s.db.Read().ListOrderItemsByOrder(s.ctx, orderID)
	if err != nil {
		s.fail(err, "items of %s", orderID)
		return ""
	}
	want := s.ids.products[product].ID
	for i := len(items) - 1; i >= 0; i-- { // the newest copy (a Remake) wins
		if items[i].ProductID == want && items[i].Status != "cancelled" {
			return items[i].ID
		}
	}
	s.fail(fmt.Errorf("no %s", product), "item of %s", orderID)
	return ""
}

func (s *liveSeeder) liveItems(orderID, status string) []string {
	items, err := s.db.Read().ListOrderItemsByOrder(s.ctx, orderID)
	s.fail(err, "items of %s", orderID)
	var out []string
	for _, it := range items {
		if it.Status == status {
			out = append(out, it.ID)
		}
	}
	return out
}

func (s *liveSeeder) claim(w *auth.Principal, orderID string, offset time.Duration) {
	if s.err == nil {
		s.fail(s.at(offset).Claim(s.ctx, w, orderID), "claim %s", orderID)
	}
}

func (s *liveSeeder) claimConfirm(w *auth.Principal, orderID string, offset time.Duration) {
	s.claim(w, orderID, offset-30*time.Second)
	if s.err == nil {
		s.fail(s.at(offset).Confirm(s.ctx, w, orderID), "confirm %s", orderID)
	}
}

// cook starts every Confirmed item of an Order at start and makes them Ready at ready.
func (s *liveSeeder) cook(k *auth.Principal, orderID string, start, ready time.Duration) {
	if s.err != nil {
		return
	}
	s.fail(s.at(start).StartItems(s.ctx, k, s.liveItems(orderID, "confirmed")), "start %s", orderID)
	if s.err == nil {
		s.fail(s.at(ready).ReadyItems(s.ctx, k, s.liveItems(orderID, "preparing")), "ready %s", orderID)
	}
}

// cookItem starts one item; a zero ready offset leaves it Preparing.
func (s *liveSeeder) cookItem(k *auth.Principal, orderID, product string, start, ready time.Duration) {
	if s.err != nil {
		return
	}
	id := s.itemID(orderID, product)
	s.fail(s.at(start).StartItems(s.ctx, k, []string{id}), "start %s", product)
	if ready != 0 && s.err == nil {
		s.fail(s.at(ready).ReadyItems(s.ctx, k, []string{id}), "ready %s", product)
	}
}

func (s *liveSeeder) serve(w *auth.Principal, orderID string, offset time.Duration) {
	if s.err == nil {
		s.fail(s.at(offset).Serve(s.ctx, w, s.liveItems(orderID, "ready")), "serve %s", orderID)
	}
}

func (s *liveSeeder) serveItems(w *auth.Principal, orderID string, offset time.Duration, products ...string) {
	if s.err != nil {
		return
	}
	var ids []string
	for _, p := range products {
		ids = append(ids, s.itemID(orderID, p))
	}
	s.fail(s.at(offset).Serve(s.ctx, w, ids), "serve %v", products)
}

func (s *liveSeeder) override(requester *auth.Principal, action string, offset time.Duration) string {
	if s.err != nil {
		return ""
	}
	tok, _, err := auth.NewService(s.db, fixedClock{s.t0.Add(offset)}).IssueOverride(s.ctx, requester, "A001", "222222", action)
	s.fail(err, "override %s", action)
	return tok
}

func (s *liveSeeder) remake(w, _ *auth.Principal, orderID, product, reason string, offset time.Duration) {
	tok := s.override(w, auth.ActionRemake, offset)
	if s.err == nil {
		_, err := s.at(offset).Remake(s.ctx, w, s.itemID(orderID, product), reason, tok)
		s.fail(err, "remake %s", product)
	}
}

func (s *liveSeeder) void(w, _ *auth.Principal, orderID, product, reason string, offset time.Duration) {
	tok := s.override(w, auth.ActionVoid, offset)
	if s.err == nil {
		s.fail(s.at(offset).Void(s.ctx, w, s.itemID(orderID, product), reason, tok), "void %s", product)
	}
}

func (s *liveSeeder) kitchenCancel(k *auth.Principal, orderID, product, reason, note string, offset time.Duration) {
	if s.err == nil {
		s.fail(s.at(offset).CancelItem(s.ctx, k, s.itemID(orderID, product), reason, note, []string{"confirmed", "preparing"}), "cancel %s", product)
	}
}

func (s *liveSeeder) merge(w *auth.Principal, into, from *ordering.Guest, offset time.Duration) {
	if s.err == nil {
		s.fail(s.at(offset).Merge(s.ctx, w, into.BillID, []string{from.BillID}), "merge")
	}
}

func (s *liveSeeder) pay(w *auth.Principal, g *ordering.Guest, offset time.Duration, in ordering.PayInput, token string) {
	if s.err == nil {
		_, err := s.at(offset).Pay(s.ctx, w, g.BillID, in, token)
		s.fail(err, "pay")
	}
}

// splitByCustomer splits a Bill by item, each Order's items to the given Sub-bill.
func (s *liveSeeder) splitByCustomer(w *auth.Principal, g *ordering.Guest, offset time.Duration, orders map[string]int) {
	if s.err != nil {
		return
	}
	assign := map[string]int{}
	for orderID, share := range orders {
		items, err := s.db.Read().ListOrderItemsByOrder(s.ctx, orderID)
		s.fail(err, "items")
		for _, it := range items {
			assign[it.ID] = share
		}
	}
	s.fail(s.at(offset).Split(s.ctx, w, g.BillID, ordering.SplitInput{Method: "by_item", Count: len(orders), Assign: assign}), "split")
}

// payShare pays a Sub-bill by verified QRIS, closes it and prints both Receipts.
func (s *liveSeeder) payShare(w *auth.Principal, g *ordering.Guest, index int64, offset time.Duration) {
	if s.err != nil {
		return
	}
	svc := s.at(offset)
	bl, err := svc.Billing(s.ctx, g.BillID)
	s.fail(err, "billing")
	for _, sb := range bl.SubBills {
		if sb.Index != index || s.err != nil {
			continue
		}
		_, err := svc.Pay(s.ctx, w, g.BillID, ordering.PayInput{SubBillID: sb.ID, Method: ordering.PayQRIS, Amount: sb.Balance, Verified: true}, "")
		s.fail(err, "pay share")
		s.fail(svc.Close(s.ctx, w, g.BillID, sb.ID, false), "close share")
		for _, copy := range []string{"customer", "internal"} {
			_, err := svc.Receipt(s.ctx, w, g.BillID, sb.ID, copy)
			s.fail(err, "receipt")
		}
	}
}

func (s *liveSeeder) hide(product string, offset time.Duration) {
	if s.err != nil {
		return
	}
	p := s.ids.products[product]
	p.Status = catalog.StatusHidden
	_, err := catalog.NewService(s.db, fixedClock{s.t0.Add(offset)}).SaveProduct(s.ctx, s.crewIDs["M001"], p)
	s.fail(err, "hide %s", product)
}

// editDropLast removes the Order's last line, as the Customer does in the cart before Claim.
func (s *liveSeeder) editDropLast(g *ordering.Guest, orderID string, offset time.Duration) {
	if s.err != nil {
		return
	}
	svc := s.at(offset)
	me, err := svc.Me(s.ctx, g)
	if err != nil {
		s.fail(err, "me")
		return
	}
	for _, o := range me.Orders {
		if o.ID != orderID {
			continue
		}
		var keep []ordering.ItemInput
		for _, it := range o.Items[:len(o.Items)-1] {
			keep = append(keep, ordering.ItemInput{ID: it.ID, VariantID: it.VariantID, Quantity: it.Quantity, Note: it.Note})
		}
		s.fail(svc.EditOrder(s.ctx, g, orderID, keep), "edit")
	}
}

func (s *liveSeeder) call(g *ordering.Guest, offset time.Duration) {
	if s.err == nil {
		_, err := s.at(offset).CallWaiter(s.ctx, g)
		s.fail(err, "call")
	}
}
