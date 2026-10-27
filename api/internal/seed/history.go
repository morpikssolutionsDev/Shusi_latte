package seed

import (
	"cmp"
	"errors"
	"math/rand/v2"
	"slices"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
)

// History (DEMO-AND-SEED §7): 14 business days before T0, played through the domain services
// like the live Tables, so reports, Prep Time performance and Shift Closings are real figures.

// namesPool is DEMO-AND-SEED §12.
var namesPool = []string{"Raka", "Laras", "Bima", "Tasya", "Yoga", "Dewi", "Kevin", "Citra", "Gilang", "Nabila", "Reza", "Intan", "Fikri",
	"Ayu", "Sari", "Hana", "Joko", "Wulan", "Bayu", "Melati", "Arif", "Salsa", "Rizky", "Indah", "Galih", "Putu", "Made", "Ketut", "Nanda", "Vina"}

// hourWeights shape a day: peaks 12.00–14.00 and 18.00–20.00.
var hourWeights = map[int]int{10: 3, 11: 5, 12: 10, 13: 10, 14: 5, 15: 4, 16: 4, 17: 5, 18: 10, 19: 10, 20: 6, 21: 3}

// productWeights: Coffee and Nigiri lead the mix.
var productWeights = []struct {
	key    string
	weight int
}{
	{"salmon-aburi", 12}, {"unagi", 7}, {"latte", 12}, {"v60", 6}, {"cold-brew", 7}, {"tonic", 3}, {"matcha", 6}, {"hojicha", 5},
	{"dragon", 5}, {"spicy-salmon", 6}, {"ebi-tempura", 5}, {"sashimi", 4}, {"wagyu", 4}, {"katsu", 5}, {"tonkotsu", 5},
	{"gyoza", 5}, {"edamame", 5}, {"crepe", 4}, {"cheesecake", 4},
}

// businessDayStart is 04:00 WIB of the business day holding t (TRD §4.3).
func businessDayStart(t time.Time) time.Time {
	day, _ := time.ParseInLocation(time.DateOnly, ordering.BusinessDate(t), platform.Jakarta)
	return day.Add(4 * time.Hour)
}

// history seeds the 14 days before today's business day.
func (s *liveSeeder) history() {
	rng := rand.New(rand.NewPCG(Rand, 14))
	today := businessDayStart(s.t0)
	unpaidDay := 9
	for d := 14; d >= 1 && s.err == nil; d-- {
		if d == 7 {
			// Ebi Tempura Roll was retired a week ago: it appears only in older Bills.
			s.setStatus("ebi-tempura", catalog.StatusArchived, today.AddDate(0, 0, -7))
		}
		s.historyDay(today.AddDate(0, 0, -d), d, d == unpaidDay, rng)
	}
	// Anonymisation showcase (TR-RET-01, G-24): closed at T0 −47h keeps its names; −49h does not.
	for _, closed := range []time.Duration{-49 * time.Hour, -47 * time.Hour} {
		at := s.t0.Add(closed)
		b := s.histBill(rng, at.Add(-50*time.Minute), 1, "Galih", s.crew("W001"), false)
		s.histPay(b, at.Add(-time.Minute), ordering.PayQRIS, s.crew("W001"), rng)
		s.closeAt(b.billID, s.crew("W001"), at)
	}
}

type histBill struct {
	billID, orderID string
	served          time.Time
}

func (s *liveSeeder) historyDay(day time.Time, d int, withUnpaid bool, rng *rand.Rand) {
	cashierCode := "C001"
	if d%2 == 1 {
		cashierCode = "C002"
	}
	cashier := s.crew(cashierCode)
	waiters := []*auth.Principal{s.crew("W001"), s.crew("W002"), s.crew("W003"), s.crew("W004")}
	open := day.Add(5*time.Hour + 30*time.Minute) // 09.30 WIB
	if s.err == nil {
		s.fail(s.atTime(open).OpenDrawer(s.ctx, cashier, 500000), "open drawer %s", day.Format(time.DateOnly))
	}

	// Opening times for the day, weighted by hour.
	n := 45 + rng.IntN(26)
	var hours []int
	for h, w := range hourWeights {
		for range w {
			hours = append(hours, h)
		}
	}
	slices.Sort(hours)
	times := make([]time.Time, 0, n)
	for range n {
		h := hours[rng.IntN(len(hours))]
		t := day.Add(time.Duration(h-4)*time.Hour + time.Duration(rng.IntN(60))*time.Minute)
		// Yesterday 19.00–19.45 the system was down: those Bills come in as Post-outage Entries.
		if d == 1 && t.After(day.Add(15*time.Hour)) && t.Before(day.Add(15*time.Hour+45*time.Minute)) {
			continue
		}
		times = append(times, t)
	}
	slices.SortFunc(times, func(a, b time.Time) int { return a.Compare(b) })

	freeAt := map[int64]time.Time{}
	unpaidDone := false
	for i, at := range times {
		if s.err != nil {
			return
		}
		peak := at.Hour() == 12 || at.Hour() == 13 || at.Hour() == 18 || at.Hour() == 19
		stay := time.Duration(35+rng.IntN(40)) * time.Minute
		table := int64(0)
		if rng.IntN(100) >= 15 {
			for t := int64(1); t <= 11; t++ {
				if !freeAt[t].After(at) {
					table = t
					break
				}
			}
		}
		if table == 0 { // ~15 % Takeaway, and every walk-in when the floor is full
			s.histTakeaway(rng, at, cashier, peak)
			continue
		}
		freeAt[table] = at.Add(stay + 10*time.Minute)
		w := waiters[rng.IntN(len(waiters))]
		name := ""
		if rng.IntN(100) < 60 {
			name = namesPool[rng.IntN(len(namesPool))]
		}
		b := s.histBill(rng, at, table, name, w, peak)
		payAt := later(b.served.Add(time.Duration(10+rng.IntN(20))*time.Minute), at.Add(stay-5*time.Minute))
		if withUnpaid && !unpaidDone && i > len(times)/2 {
			unpaidDone = true
			tok := s.overrideAt(w, "A001", "222222", auth.ActionUnpaidBill, payAt)
			if s.err == nil {
				s.fail(s.atTime(payAt).CloseUnpaid(s.ctx, w, b.billID, "Pelanggan pergi tanpa bayar", tok), "unpaid")
			}
			continue
		}
		method := s.pickMethod(rng)
		payer := w
		if method == ordering.PayCash {
			payer = cashier // history cash is paid at the counter
		}
		s.histPay(b, payAt, method, payer, rng)
		s.closeAt(b.billID, payer, payAt.Add(time.Minute))
	}

	if d == 1 {
		s.postOutage(day, cashier)
	}

	// Shift Closing: yesterday exact, the day before +Rp 5.000, earlier small differences.
	diffs := []int64{0, 0, 0, -2000, 1000, 0, -500}
	diff := diffs[rng.IntN(len(diffs))]
	switch d {
	case 1:
		diff = 0
	case 2:
		diff = 5000
	}
	closeAt := day.Add(18*time.Hour + 45*time.Minute) // 22.45 WIB
	if s.err == nil {
		drawer, err := s.atTime(closeAt).Drawer(s.ctx)
		s.fail(err, "drawer")
		if err == nil {
			_, err = s.atTime(closeAt).CloseDrawer(s.ctx, cashier, drawer.Expected.Total+diff)
			s.fail(err, "close drawer %s", day.Format(time.DateOnly))
		}
	}
}

// histLines picks 1–5 lines for a group, Coffee and Nigiri most often.
func (s *liveSeeder) histLines(rng *rand.Rand, at time.Time) []seededLine {
	total := 0
	for _, p := range productWeights {
		total += p.weight
	}
	n := 1 + rng.IntN(5)
	var out []seededLine
	for len(out) < n {
		r := rng.IntN(total)
		for _, p := range productWeights {
			if r -= p.weight; r >= 0 {
				continue
			}
			if p.key == "ebi-tempura" && !at.Before(businessDayStart(s.t0).AddDate(0, 0, -7)) {
				break
			}
			out = append(out, s.autoLine(p.key, rng, int64(1+rng.IntN(2))))
			break
		}
	}
	return out
}

// autoLine is a line with an orderable Variant and the first option of each required group.
func (s *liveSeeder) autoLine(product string, rng *rand.Rand, qty int64) seededLine {
	p := s.ids.products[product]
	variants := []int{}
	for i := range p.Variants {
		if !(product == "unagi" && i == 1) { // Unagi 4 pcs is Sold Out (DEMO-AND-SEED §3.2)
			variants = append(variants, i)
		}
	}
	in := ordering.ItemInput{VariantID: p.Variants[variants[rng.IntN(len(variants))]].ID, Quantity: qty}
	for _, g := range p.ModifierGroups {
		for _, m := range g.Modifiers[:g.MinSelect] {
			in.ModifierIDs = append(in.ModifierIDs, m.ID)
		}
	}
	return seededLine{in, s}
}

func (s *liveSeeder) pickMethod(rng *rand.Rand) string {
	switch r := rng.IntN(100); {
	case r < 55:
		return ordering.PayQRIS
	case r < 85:
		return ordering.PayCash
	default:
		return ordering.PayCard
	}
}

// histBill takes a Table Order directly and plays it through the Kitchen to Served.
func (s *liveSeeder) histBill(rng *rand.Rand, at time.Time, table int64, name string, w *auth.Principal, peak bool) histBill {
	if s.err != nil {
		return histBill{}
	}
	lines := s.histLines(rng, at)
	b := s.directAt(w, at, table, name, lines...)
	b.served = s.kitchen(rng, b.orderID, at, peak)
	if s.err == nil {
		s.fail(s.atTime(b.served).Serve(s.ctx, w, s.liveItems(b.orderID, "ready")), "serve")
	}
	return b
}

// histTakeaway is paid first at the counter, then cooked.
func (s *liveSeeder) histTakeaway(rng *rand.Rand, at time.Time, cashier *auth.Principal, peak bool) {
	if s.err != nil {
		return
	}
	lines := s.histLines(rng, at)
	items := make([]ordering.ItemInput, len(lines))
	for i, l := range lines {
		items[i] = l.ItemInput
	}
	billID, err := s.atTime(at).CreateTakeaway(s.ctx, cashier, ordering.TakeawayInput{CustomerName: namesPool[rng.IntN(len(namesPool))], Items: items})
	s.fail(err, "takeaway")
	b := histBill{billID: billID}
	paid := at.Add(time.Minute)
	s.histPay(b, paid, s.pickMethod(rng), cashier, rng)
	orders, err := s.db.Read().ListOrdersByBill(s.ctx, billID)
	if err != nil || len(orders) == 0 {
		s.fail(cmp.Or(err, errors.New("no order")), "takeaway order")
		return
	}
	done := s.kitchen(rng, orders[0].ID, paid, peak)
	if s.err == nil {
		s.fail(s.atTime(done).Serve(s.ctx, cashier, s.liveItems(orders[0].ID, "ready")), "hand over takeaway")
	}
}

// kitchen starts every Confirmed item a minute after confirmation and makes each Ready on time
// (about 90 %, a little worse at peaks) or late. Returns when the last item was Ready.
func (s *liveSeeder) kitchen(rng *rand.Rand, orderID string, confirmed time.Time, peak bool) time.Time {
	if s.err != nil {
		return confirmed
	}
	k := s.crew("K001")
	start := confirmed.Add(time.Minute)
	s.fail(s.atTime(start).StartItems(s.ctx, k, s.liveItems(orderID, "confirmed")), "start")
	items, err := s.db.Read().ListOrderItemsByOrder(s.ctx, orderID)
	s.fail(err, "items")
	last := start
	late := 9
	if peak {
		late = 12
	}
	for _, it := range items {
		if it.Status != "preparing" || s.err != nil {
			continue
		}
		f := 0.5 + rng.Float64()*0.45
		if rng.IntN(100) < late {
			f = 1.1 + rng.Float64()*0.5
		}
		ready := confirmed.Add(time.Duration(float64(it.PrepTimeSeconds)*f) * time.Second)
		ready = later(ready, start.Add(30*time.Second))
		s.fail(s.atTime(ready).ReadyItems(s.ctx, k, []string{it.ID}), "ready")
		last = later(last, ready)
	}
	return last.Add(time.Duration(1+rng.IntN(3)) * time.Minute)
}

// histPay pays a Bill's balance in full: QRIS verified, card, or cash tendered as a round note.
func (s *liveSeeder) histPay(b histBill, at time.Time, method string, by *auth.Principal, rng *rand.Rand) {
	if s.err != nil {
		return
	}
	svc := s.atTime(at)
	bl, err := svc.Billing(s.ctx, b.billID)
	if err != nil {
		s.fail(err, "billing")
		return
	}
	in := ordering.PayInput{Method: method, Amount: bl.Balance, Verified: true}
	if method == ordering.PayCash {
		due, _ := pricing.RoundCash(bl.Balance)
		notes := []int64{due, (due/50000 + 1) * 50000, (due/100000 + 1) * 100000}
		in.Amount = notes[rng.IntN(len(notes))]
	}
	_, err = svc.Pay(s.ctx, by, b.billID, in, "")
	s.fail(err, "pay %s", method)
}

func (s *liveSeeder) closeAt(billID string, by *auth.Principal, at time.Time) {
	if s.err == nil {
		s.fail(s.atTime(at).Close(s.ctx, by, billID, "", false), "close")
	}
}

// directAt takes an Order at a Table directly, opening the Bill (OP-04).
func (s *liveSeeder) directAt(w *auth.Principal, at time.Time, table int64, name string, lines ...seededLine) histBill {
	if s.err != nil {
		return histBill{}
	}
	items := make([]ordering.ItemInput, len(lines))
	for i, l := range lines {
		items[i] = l.ItemInput
	}
	orderID, err := s.atTime(at).DirectOrder(s.ctx, w, "waiter", ordering.DirectInput{TableID: s.tableIDs[table], CustomerName: name, Items: items})
	if err != nil {
		s.fail(err, "direct order Meja %d", table)
		return histBill{}
	}
	o, err := s.db.Read().GetOrder(s.ctx, orderID)
	s.fail(err, "order")
	return histBill{billID: o.BillID, orderID: orderID, served: at}
}

// postOutage re-enters yesterday's two paper Bills from 19.10–19.40 (G-21).
func (s *liveSeeder) postOutage(day time.Time, cashier *auth.Principal) {
	entries := []struct {
		table          int64
		opened, closed time.Duration
		lines          []seededLine
		method         string
	}{
		{4, 15*time.Hour + 10*time.Minute, 15*time.Hour + 35*time.Minute,
			[]seededLine{s.line("salmon-aburi", 0, 2), s.line("latte", 0, 2, "Susu segar")}, ordering.PayCash},
		{7, 15*time.Hour + 12*time.Minute, 15*time.Hour + 40*time.Minute,
			[]seededLine{s.line("katsu", 0, 1, "Sedang"), s.line("cold-brew", 0, 1)}, ordering.PayQRIS},
	}
	enteredAt := day.Add(16*time.Hour + 30*time.Minute) // 20.30 WIB, after power came back
	for _, e := range entries {
		if s.err != nil {
			return
		}
		items := make([]ordering.ItemInput, len(e.lines))
		for i, l := range e.lines {
			items[i] = l.ItemInput
		}
		in := ordering.OutageInput{TableID: s.tableIDs[e.table], OpenedAt: platform.Millis(day.Add(e.opened)), ClosedAt: platform.Millis(day.Add(e.closed)),
			ServedBy: "W002", Items: items}
		svc := s.atTime(enteredAt)
		// The paper Bill carries the total the API computes; ask for it with a token payment first.
		in.Payments = []ordering.OutagePayment{{Method: e.method, Amount: 1, TakenAt: in.ClosedAt}}
		_, err := svc.PostOutage(s.ctx, cashier, in)
		var p *httpx.Problem
		total, ok := int64(0), errors.As(err, &p)
		if ok {
			if v, isNum := p.Extra["total"].(int64); isNum {
				total = v
			}
		}
		if total == 0 {
			s.fail(cmp.Or(err, errors.New("no total")), "outage total")
			return
		}
		if e.method == ordering.PayCash {
			total, _ = pricing.RoundCash(total)
		}
		in.Payments[0].Amount = total
		_, err = svc.PostOutage(s.ctx, cashier, in)
		s.fail(err, "post-outage entry")
	}
}

func (s *liveSeeder) setStatus(product, status string, at time.Time) {
	if s.err != nil {
		return
	}
	p := s.ids.products[product]
	p.Status = status
	saved, err := catalog.NewService(s.db, fixedClock{at}).SaveProduct(s.ctx, s.crewIDs["M001"], p)
	s.fail(err, "%s %s", status, product)
	if err == nil {
		s.ids.products[product] = saved
	}
}

func (s *liveSeeder) atTime(t time.Time) *ordering.Service {
	return s.base.WithClock(fixedClock{t})
}

func (s *liveSeeder) overrideAt(requester *auth.Principal, approver, pin, action string, at time.Time) string {
	if s.err != nil {
		return ""
	}
	tok, _, err := auth.NewService(s.db, fixedClock{at}).IssueOverride(s.ctx, requester, approver, pin, action)
	s.fail(err, "override %s by %s", action, approver)
	return tok
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
