package seed

import (
	"math/rand/v2"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/platform"
)

const h = time.Hour

// tm places a nominal offset of today's story (T0 −7h … −30m, DEMO-AND-SEED §6.3) inside the
// current business day: exact when T0 is late enough, compressed after the 04.00 turnover.
func (s *liveSeeder) tm(offset time.Duration) time.Time {
	end := s.t0.Add(-30 * m)
	start := later(s.t0.Add(-7*h), businessDayStart(s.t0).Add(10*m))
	if end.Sub(start) < 70*m {
		start = end.Add(-70 * m)
	}
	f := float64(offset+7*h) / float64(6*h+30*m)
	return start.Add(time.Duration(f * float64(end.Sub(start))))
}

// off is tm as an offset from T0, for the offset-based live helpers.
func (s *liveSeeder) off(offset time.Duration) time.Duration { return s.tm(offset).Sub(s.t0) }

// cooked takes an Order at a Table and plays it through the Kitchen to Served.
func (s *liveSeeder) cooked(rng *rand.Rand, w *auth.Principal, at time.Time, table int64, name string, peak bool, lines ...seededLine) histBill {
	b := s.directAt(w, at, table, name, lines...)
	b.served = s.kitchen(rng, b.orderID, at, peak)
	if s.err == nil {
		s.fail(s.atTime(b.served).Serve(s.ctx, w, s.liveItems(b.orderID, "ready")), "serve")
	}
	return b
}

// today seeds today's closed Bills, the Cash Drawer (§8) and the Takeaways (§6.2).
func (s *liveSeeder) today() {
	rng := rand.New(rand.NewPCG(Rand, 0))
	w1, w2, w3 := s.crew("W001"), s.crew("W002"), s.crew("W003") // W004 is late today: not clocked in (§8)
	c1 := s.crew("C001")
	pay := func(b histBill, at time.Duration, method string, by *auth.Principal) {
		s.histPay(b, s.tm(at), method, by, rng)
		s.closeAt(b.billID, by, s.tm(at).Add(time.Minute))
	}

	// Morning, before the drawer opens: QRIS and card only.
	a := s.cooked(rng, w2, s.tm(-7*h), 1, "Bayu", false, s.line("latte", 0, 1, "Susu segar"), s.line("salmon-aburi", 0, 1))
	pay(a, -6*h-20*m, ordering.PayQRIS, w2)
	b := s.cooked(rng, w2, s.tm(-6*h-40*m), 2, "", false, s.line("tonkotsu", 0, 1, "Sedang"), s.line("cold-brew", 0, 1))
	pay(b, -6*h, ordering.PayCard, w2)

	// The Cash Drawer opens with Rp 500.000 (G-19).
	if s.err == nil {
		s.fail(s.atTime(s.tm(-5*h-5*m)).OpenDrawer(s.ctx, c1, 500000), "open drawer")
	}

	// Move: Meja 1 → Meja 3 (BR-WT-06), paid cash at the counter.
	mv := s.cooked(rng, w1, s.tm(-5*h-20*m), 1, "Arif", false, s.line("katsu", 0, 1, "Sedang"), s.line("edamame", 0, 1), s.line("tonic", 0, 1))
	if s.err == nil {
		s.fail(s.atTime(s.tm(-5*h)).Move(s.ctx, w1, mv.billID, s.tableIDs[1], s.tableIDs[3]), "move")
	}
	pay(mv, -4*h-20*m, ordering.PayCash, c1)

	filler := s.cooked(rng, w2, s.tm(-4*h-40*m), 1, "", false, s.histLines(rng, s.t0)...)
	pay(filler, -4*h, ordering.PayCash, c1)

	// Claim takeover: W003 takes over W001's Claim (G-06).
	melati := s.join(5, s.off(-4*h-20*m), "Melati")
	o := s.order(melati, s.off(-4*h-15*m), s.line("dragon", 0, 1), s.line("hojicha", 0, 1, "Susu segar"))
	s.claim(w1, o, s.off(-4*h-10*m))
	if s.err == nil {
		s.fail(s.at(s.off(-4*h)).TransferClaim(s.ctx, w3, o, ""), "takeover")
		s.fail(s.at(s.off(-4*h+2*m)).Confirm(s.ctx, w3, o), "confirm takeover")
	}
	if s.err == nil {
		tk := histBill{billID: melati.BillID, orderID: o}
		tk.served = s.kitchen(rng, o, s.tm(-4*h+2*m), false)
		s.fail(s.atTime(tk.served).Serve(s.ctx, w3, s.liveItems(o, "ready")), "serve takeover")
		pay(tk, -3*h-15*m, ordering.PayQRIS, w3)
	}

	// Meja 4 lunch: paid at the counter, both Receipts printed, the Customer copy reprinted (G-15).
	lunch := s.cooked(rng, w2, s.tm(-4*h), 4, "Kevin", true, s.line("wagyu", 0, 1), s.line("matcha", 0, 1, "Susu segar"), s.line("gyoza", 0, 1))
	pay(lunch, -3*h-20*m, ordering.PayQRIS, c1)
	for _, p := range []struct {
		at   time.Duration
		copy string
	}{{-3*h - 19*m, "customer"}, {-3*h - 19*m, "internal"}, {-3*h - 10*m, "customer"}} {
		if s.err == nil {
			_, err := s.atTime(s.tm(p.at)).Receipt(s.ctx, c1, lunch.billID, "", p.copy)
			s.fail(err, "receipt")
		}
	}

	// Unpaid Bill: the Customer left; closed with A001's PIN (G-14).
	gone := s.cooked(rng, w2, s.tm(-3*h-50*m), 9, "Rizky", true, s.line("spicy-salmon", 0, 1), s.line("cold-brew", 0, 1))
	tok := s.overrideAt(w2, "A001", "222222", auth.ActionUnpaidBill, s.tm(-3*h))
	if s.err == nil {
		s.fail(s.atTime(s.tm(-3*h)).CloseUnpaid(s.ctx, w2, gone.billID, "Pelanggan pergi tanpa bayar", tok), "unpaid")
	}

	// Large group: W001 takes Rp 312.000 in cash (handed over exactly later), the rest by card;
	// Refund Rp 560.000 above the cap with Management's PIN (BR-R-20).
	group := s.cooked(rng, w1, s.tm(-3*h-40*m), 10, "Putu", true,
		s.line("sashimi", 1, 2), s.line("wagyu", 1, 2), s.line("dragon", 0, 2), s.line("matcha", 1, 4, "Susu segar"))
	s.partPay(group, s.tm(-2*h-55*m), w1, ordering.PayInput{Method: ordering.PayCash, Amount: 312000})
	pay(group, -2*h-54*m, ordering.PayCard, w1)
	s.refund(c1, group.billID, 560000, "Hidangan rombongan salah, tamu menolak", "M001", "111111", auth.ActionRefundAboveCap, s.tm(-2*h-30*m))

	// Pay now, serve later: the dessert is still Preparing when W003 closes Meja 2 (G-13).
	// W003 takes Rp 186.000 in cash and hands over Rp 166.000 later (G-20).
	dessert := s.directAt(w3, s.tm(-3*h-30*m), 2, "Salsa", s.line("katsu", 0, 1, "Sedang"), s.line("sashimi", 0, 1), s.line("crepe", 0, 1))
	k1, k3 := s.crew("K001"), s.crew("K003")
	s.cookItem(k1, dessert.orderID, "katsu", s.off(-3*h-28*m), s.off(-3*h-15*m))
	s.cookItem(k1, dessert.orderID, "sashimi", s.off(-3*h-28*m), s.off(-3*h-20*m))
	s.cookItem(k3, dessert.orderID, "crepe", s.off(-2*h-50*m), 0)
	if s.err == nil {
		s.fail(s.atTime(s.tm(-3*h-12*m)).Serve(s.ctx, w3, s.liveItems(dessert.orderID, "ready")), "serve mains")
	}
	s.partPay(dessert, s.tm(-2*h-46*m), w3, ordering.PayInput{Method: ordering.PayCash, Amount: 186000})
	s.histPay(dessert, s.tm(-2*h-45*m), ordering.PayQRIS, w3, rng)
	if s.err == nil {
		s.fail(s.atTime(s.tm(-2*h-44*m)).Close(s.ctx, w3, dessert.billID, "", true), "pay now, serve later")
	}
	if s.err == nil {
		s.fail(s.atTime(s.tm(-2*h-40*m)).ReadyItems(s.ctx, k3, s.liveItems(dessert.orderID, "preparing")), "dessert ready")
		s.fail(s.atTime(s.tm(-2*h-38*m)).Serve(s.ctx, w3, s.liveItems(dessert.orderID, "ready")), "dessert served")
	}

	// Manual 10 % Bill discount with Management's PIN (BR-R-17).
	disc := s.cooked(rng, w2, s.tm(-2*h-50*m), 7, "Indah", false, s.line("dragon", 0, 1), s.line("spicy-salmon", 0, 1), s.line("tonic", 0, 1))
	if s.err == nil {
		tok := s.overrideAt(w2, "M001", "111111", auth.ActionManualDiscount, s.tm(-2*h-10*m))
		s.fail(s.atTime(s.tm(-2*h-10*m)).ApplyManualDiscount(s.ctx, w2, disc.billID, s.discountID("Diskon Manual 10%"), tok), "manual discount")
	}
	pay(disc, -2*h-5*m, ordering.PayCard, w2)

	// Paket Hemat: subtotal Rp 412.000 → −Rp 25.000 (Bill-level discount).
	hemat := s.cooked(rng, w2, s.tm(-2*h-40*m), 11, "Nanda", false, s.line("sashimi", 1, 1), s.line("wagyu", 1, 1), s.line("cheesecake", 0, 1))
	pay(hemat, -2*h, ordering.PayQRIS, w2)

	// Pending QRIS Rp 94.300 approved by A001; it blocks the Shift Closing (G-16).
	pend := s.cooked(rng, w2, s.tm(-2*h-40*m), 8, "Made", false, s.line("unagi", 0, 1), s.line("tonkotsu", 0, 1, "Sedang"), s.line("hojicha", 0, 1, "Susu segar"))
	qt := s.overrideAt(w2, "A001", "222222", auth.ActionQRISUnverified, s.tm(-2*h))
	s.partPay(pend, s.tm(-2*h), w2, ordering.PayInput{Method: ordering.PayQRIS, Amount: 94300}, qt)
	pay(pend, -2*h+m, ordering.PayCard, w2)

	// Meja 6: paid cash at the counter; Rp 52.000 refunded within the cap, A001 approving.
	six := s.cooked(rng, w3, s.tm(-3*h), 6, "Ketut", false, s.line("salmon-aburi", 1, 1), s.line("latte", 1, 1, "Susu segar"), s.line("cheesecake", 0, 1))
	pay(six, -2*h-15*m, ordering.PayCash, c1)
	s.refund(c1, six.billID, 52000, "Cheesecake tidak sesuai pesanan", "A001", "222222", auth.ActionRefundInCap, s.tm(-h))

	more := s.cooked(rng, w1, s.tm(-3*h-30*m), 3, "", true, s.histLines(rng, s.t0)...)
	pay(more, -2*h-50*m, ordering.PayCash, c1)
	more = s.cooked(rng, w1, s.tm(-2*h), 1, "Vina", false, s.histLines(rng, s.t0)...)
	pay(more, -h-20*m, ordering.PayQRIS, w1)
	more = s.cooked(rng, w2, s.tm(-h-15*m), 1, "", false, s.histLines(rng, s.t0)...)
	pay(more, -35*m, ordering.PayCard, w2)

	// Cash Handovers: W001 exact, W003 Rp 20.000 short (BR-CS-05, BR-R-36).
	for _, ho := range []struct {
		w        *auth.Principal
		received int64
		at       time.Duration
	}{{w1, 312000, -h - 10*m}, {w3, 166000, -50 * m}} {
		if s.err == nil {
			_, err := s.atTime(s.tm(ho.at)).ReceiveHandover(s.ctx, c1, ho.w.ID, ho.received)
			s.fail(err, "handover %s", ho.w.CrewID)
		}
	}

	s.happyHour(rng, w1)
	s.takeaways(rng, c1)
}

// partPay records one payment without closing the Bill.
func (s *liveSeeder) partPay(b histBill, at time.Time, by *auth.Principal, in ordering.PayInput, token ...string) {
	if s.err != nil {
		return
	}
	tok := ""
	if len(token) > 0 {
		tok = token[0]
	}
	_, err := s.atTime(at).Pay(s.ctx, by, b.billID, in, tok)
	s.fail(err, "pay %s %d", in.Method, in.Amount)
}

func (s *liveSeeder) refund(c *auth.Principal, billID string, amount int64, reason, approver, pin, action string, at time.Time) {
	tok := s.overrideAt(c, approver, pin, action, at)
	if s.err == nil {
		s.fail(s.atTime(at).Refund(s.ctx, c, billID, amount, reason, tok), "refund %d", amount)
	}
}

func (s *liveSeeder) discountID(name string) string {
	ds, err := s.db.Read().ListDiscounts(s.ctx)
	s.fail(err, "discounts")
	for _, d := range ds {
		if d.Name == name {
			return d.ID
		}
	}
	return "missing:" + name
}

// happyHour seeds coffee at 15 % inside the last 14.00–17.00 window before T0 (DEMO-AND-SEED §6.3).
func (s *liveSeeder) happyHour(rng *rand.Rand, w *auth.Principal) {
	local := s.t0.In(platform.Jakarta)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	minutes := local.Sub(midnight)
	var at time.Time
	switch {
	case minutes >= 17*h+30*m:
		at = midnight.Add(15 * h)
	case minutes >= 14*h+50*m:
		at = midnight.Add(14*h + 5*m)
	default:
		at = midnight.Add(-24*h + 15*h)
	}
	b := s.cooked(rng, w, at, 1, "Wulan", false, s.line("latte", 0, 2, "Susu segar"), s.line("cold-brew", 0, 1), s.line("v60", 0, 1), s.line("salmon-aburi", 0, 1))
	s.histPay(b, later(b.served, at.Add(25*m)), ordering.PayQRIS, w, rng)
	s.closeAt(b.billID, w, later(b.served, at.Add(25*m)).Add(m))
}

// takeaways seeds TA-1..TA-3 (DEMO-AND-SEED §6.2, G-18).
func (s *liveSeeder) takeaways(rng *rand.Rand, c *auth.Principal) {
	k3 := s.crew("K003")
	create := func(name string, offset time.Duration, lines ...seededLine) (string, string) {
		if s.err != nil {
			return "", ""
		}
		items := make([]ordering.ItemInput, len(lines))
		for i, l := range lines {
			items[i] = l.ItemInput
		}
		billID, err := s.at(offset).CreateTakeaway(s.ctx, c, ordering.TakeawayInput{CustomerName: name, Items: items})
		s.fail(err, "takeaway %s", name)
		orders, err := s.db.Read().ListOrdersByBill(s.ctx, billID)
		if err != nil || len(orders) == 0 {
			s.fail(err, "takeaway order")
			return billID, ""
		}
		return billID, orders[0].ID
	}

	// TA-2 Hana: paid cash at the counter; everything Ready for the name call-out.
	hana, ho := create("Hana", -16*m, s.line("cold-brew", 0, 1), s.line("crepe", 0, 1))
	s.histPay(histBill{billID: hana}, s.t0.Add(-15*m), ordering.PayCash, c, rng)
	s.cookItem(k3, ho, "cold-brew", -14*m, -13*m)
	s.cookItem(k3, ho, "crepe", -14*m, -9*m)

	// TA-1 Sari: paid by card; the iced latte Preparing at the Bar, the cheesecake Ready.
	sari, so := create("Sari", -10*m, s.line("latte", 1, 1, "Susu segar"), s.line("cheesecake", 0, 1))
	s.histPay(histBill{billID: sari}, s.t0.Add(-8*m), ordering.PayCard, c, rng)
	s.cookItem(k3, so, "cheesecake", -7*m, -3*m)
	s.cookItem(k3, so, "latte", -6*m, 0)

	// TA-3 Joko: drafted, not paid — the Kitchen has not seen it.
	create("Joko", -2*m, s.line("tonkotsu", 0, 1, "Sedang"), s.line("latte", 0, 1, "Susu segar"))
}
