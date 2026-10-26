# Sushi Latte — Demo Mode and Seed Data

The prototype is a showcase. When it starts, every scenario from [OVERVIEW-AND-FLOWS.md](OVERVIEW-AND-FLOWS.md) should already be **live on screen**, so a presenter can walk through any flow without first creating data. This document defines the seed dataset, the time-relative scenarios, the demo helpers, and how the demo resets.

Related: [BRD §11a](BRD.md#11a-prototype-and-demo) · [TRD](TRD.md) · [ADR-0005 (SQLite)](adr/0005-sqlite-for-demo.md) · [image catalogue](images/README.md)

---

## 1. Principles

1. **Every branch is visible at T0.** T0 is the moment the seed runs, rounded to the minute. Live timestamps are offsets from T0, so timers show green, amber and red, and escalations are about to fire.
2. **Fictional data only.** All names are invented Indonesian names; portraits are mockups reused across Crew ([§4](#4-crew-and-demo-logins)).
3. **Deterministic.** The historical data uses a fixed random seed (`2026`), so the reports look the same after every reset (only shifted to the new T0).
4. **Real rules, no shortcuts.** The seed writes data that the API's own rules would allow (valid state transitions, audit log entries, correct totals). The generator goes through the domain packages and never inserts raw rows that skip a rule.
5. **Demo mode is a switch.** `DEMO_MODE=true` enables the helpers in [§2](#2-demo-mode). With it off, none of the demo endpoints or UI exist.

## 2. Demo mode

| Helper | Where | Behaviour |
|---|---|---|
| **Demo banner** | Every app, top | "Demo — data direset setiap malam · resets nightly" with a link to the launcher |
| **Demo launcher** | `/demo` | Cards for every Table showing its QR code, an "Open as Customer" link and its current scenario label; one-tap logins for each demo account; links to the Kitchen and Bar displays, Product Display and Admin Panel; the guided tours from [§10](#10-guided-demo-tours) |
| **Demo logins** | Staff login screen | A "Demo accounts" panel listing Crew IDs, names, roles and PINs ([§4](#4-crew-and-demo-logins)); tapping one fills the Crew ID |
| **Receipt preview** | Waiter, Cashier | With no Bluetooth printer paired, "Print" opens the on-screen ReceiptPreview (Customer and Internal copies) instead of failing |
| **Reset demo** | Admin Panel → Settings (Management only) | Confirm dialog + Management PIN Override → full reset ([§9](#9-reset)) |
| **Nightly reset** | Scheduler, 03:00 WIB | Same reset, unattended |
| **Multiple phones optional** | Launcher | Each Customer link opens in a new tab with its own guest session, so one laptop can play several Customers at a Table |

## 3. Product catalog

### 3.1 Categories

| Category | Prep Area | Sort |
|---|---|---|
| Nigiri | Kitchen | 1 |
| Rolls | Kitchen | 2 |
| Sashimi | Kitchen | 3 |
| Hot Dishes | Kitchen | 4 |
| Small Plates | Kitchen | 5 |
| Coffee | Bar | 6 |
| Tea | Bar | 7 |
| Dessert | Bar | 8 |

### 3.2 Products

Prices are in Rupiah, excluding tax and service. The Dietary Tags are what the Product contains. Photo paths are relative to `docs/images/`.

| Product | Category | Variants (price) | Prep Time | Dietary Tags | Photo | Status |
|---|---|---|---|---|---|---|
| Salmon Aburi Nigiri | Nigiri | 2 pcs (48.000) · 4 pcs (89.000) | 6 min | Raw Fish, Fish, Egg, Soy | `products/food/salmon-aburi-nigiri.jpg` | Active |
| Unagi Kabayaki Nigiri | Nigiri | 2 pcs (58.000) · **4 pcs (109.000, Sold Out — manual)** | 6 min | Fish, Soy, Gluten | `products/food/unagi-kabayaki-nigiri.jpg` | Active |
| Dragon Roll | Rolls | 8 pcs (88.000) | 10 min | Fish, Shellfish, Soy, Gluten | `products/food/dragon-roll-maki.jpg` | Active |
| Spicy Salmon Roll | Rolls | 8 pcs (68.000) | 9 min | Raw Fish, Fish, Egg, Soy, Sesame, Gluten, Spicy | `products/food/spicy-salmon-roll.jpg` | Active |
| Ebi Tempura Roll | Rolls | 8 pcs (72.000) | 10 min | Shellfish, Gluten, Egg, Soy | — (placeholder) | **Archived** (in old Bills only) |
| Tuna & Salmon Sashimi Platter | Sashimi | Regular, 9 slices (125.000) · Premium, 15 slices (195.000) | 8 min | Raw Fish, Fish | `products/food/tuna-sashimi-platter.jpg` | Active |
| Wagyu Beef Donburi | Hot Dishes | Regular (135.000) · Large (165.000) | 12 min | Egg, Soy, Gluten | `products/food/wagyu-beef-donburi.jpg` | Active |
| Chicken Katsu Curry | Hot Dishes | One size (72.000) | 14 min | Gluten, Egg, Soy | `products/food/chicken-katsu-curry.jpg` | Active |
| Tonkotsu Ramen | Hot Dishes | One size (78.000) | 12 min | Pork, Egg, Gluten, Soy, Sesame | — (placeholder) | Active |
| Crispy Gyoza | Small Plates | 5 pcs (42.000) · 10 pcs (78.000) | 10 min | Pork, Gluten, Soy, Sesame | `products/food/crispy-gyoza-dumplings.jpg` | Active |
| Truffle Edamame | Small Plates | One size (38.000) | 5 min | Soy | `products/food/truffle-edamame.jpg` | Active |
| Signature Caffè Latte | Coffee | Hot (38.000) · Iced (41.000) | 4 min | Dairy | `products/beverages/signature-latte-art.jpg` | Active |
| V60 Pour-Over | Coffee | Gayo Aceh (45.000) · Kintamani Bali (45.000) · Toraja (48.000) | 6 min | — | `products/beverages/v60-pourover-coffee.jpg` | Active |
| Kyoto Cold Brew | Coffee | One size (42.000) | 2 min | — | `products/beverages/japanese-cold-brew.jpg` | Active |
| Iced Espresso Tonic | Coffee | One size (45.000) | 3 min | — | — (placeholder) | Active |
| Sakura Latte | Coffee | Hot (42.000) · Iced (45.000) | 4 min | Dairy | — (placeholder) | **Hidden** (seasonal) |
| Uji Matcha Latte | Tea | Hot (42.000) · Iced (45.000) | 4 min | Dairy | `products/beverages/iced-matcha-latte.jpg` | Active |
| Hojicha Caramel Latte | Tea | Hot (40.000) · **Iced (43.000, Sold Out — via Out of Stock Cancel)** | 4 min | Dairy | `products/beverages/hojicha-caramel-latte.jpg` | Active |
| Matcha Mille-Crêpe | Dessert | One slice (55.000) | 3 min | Dairy, Gluten, Egg | `products/desserts/matcha-mille-crepe.jpg` | Active |
| Yuzu Rare Cheesecake | Dessert | One slice (52.000) | 3 min | Dairy, Gluten, Egg | `products/desserts/yuzu-rare-cheesecake.jpg` | Active |

### 3.3 Modifier groups

Modifiers can add Dietary Tags but never remove them (BR-R-D4). Choosing oat milk therefore still shows the Dairy warning; this over-warning is intentional.

| Group | Applies to | Rule | Options (price · tags added) |
|---|---|---|---|
| Milk | Caffè Latte, Sakura, Matcha, Hojicha | Pick 1 (required) | Fresh milk (0 · Dairy) · Oat milk (+6.000) · Soy milk (+5.000 · Soy) |
| Coffee extras | Caffè Latte, Cold Brew, Espresso Tonic | Up to 3 | Extra shot (+8.000) · Vanilla syrup (+5.000) · Less sugar (0) |
| Sushi extras | Nigiri, Rolls, Sashimi | Up to 3 | No wasabi (0) · Extra gari (+3.000) · Spicy mayo (+5.000 · Egg, Spicy) |
| Ramen extras | Tonkotsu Ramen | Up to 2 | Extra chashu (+18.000 · Pork) · Ajitama egg (+8.000 · Egg) |
| Spice level | Tonkotsu Ramen, Katsu Curry | Pick 1 (optional) | Mild (0) · Medium (0 · Spicy) · Hot (0 · Spicy) |
| Donburi extras | Wagyu Beef Donburi | Up to 2 | Onsen egg (+8.000 · Egg) · Extra rice (+6.000) |
| Gyoza dip | Crispy Gyoza | Pick 1 (optional) | Ponzu (0) · Chilli oil (0 · Spicy, Sesame) |
| Dessert add-on | Mille-Crêpe, Cheesecake | Up to 1 | Matcha ice cream (+15.000 · Dairy) |

## 4. Crew and demo logins

All Crew sign in with Crew ID + PIN. The PINs follow the role so they're easy to demo. Portraits are reused across Crew.

| Crew ID | Name | Role | Stations / screen | PIN | Portrait | Scenario notes |
|---|---|---|---|---|---|---|
| M001 | Hendra Wijaya | Management | — | 111111 | `staff/staff-store-manager.jpg` | Approves manual discounts and Refunds above the cap; Reset demo |
| A001 | Rina Kusuma | Outlet Admin | — | 222222 | `staff/staff-waitress-female.jpg` | Gets escalations; approves Voids, Remakes, Unpaid Bills |
| W001 | Andi Pratama | Crew | Waiter | 333333 | `staff/staff-waiter-male.jpg` | Owns Meja 5 and Meja 7 |
| W002 | Dimas Saputra | Crew | Waiter | 333333 | `staff/staff-waiter-male.jpg` | **Holding Rp 200.000 cash** (clock-out blocked) |
| W003 | Putri Lestari | Crew | Waiter | 333333 | `staff/staff-waitress-female.jpg` | Took over a Claim from W001 earlier today; handover short by Rp 20.000 |
| W004 | Nadia Rahma | Crew | Waiter | 333333 | `staff/staff-waitress-female.jpg` | Leave request pending |
| W005 | Eko Prasetyo | Crew | Waiter | 333333 | `staff/staff-waiter-male.jpg` | **Locked out** (5 failed PINs, unlocks 15 min after T0) |
| C001 | Maya Anggraini | Crew | Cashier | 444444 | `staff/staff-waitress-female.jpg` | Owns today's Cash Drawer session |
| C002 | Fajar Nugroho | Crew | Cashier, Waiter | 444444 | `staff/staff-waiter-male.jpg` | Leave request rejected |
| K001 | Budi Santoso | Crew | Kitchen (Kitchen display) | 555555 | `staff/staff-waiter-male.jpg` | Leave approved next week |
| K002 | Agus Setiawan | Crew | Kitchen (Kitchen display) | 555555 | `staff/staff-waiter-male.jpg` | Auto clocked out yesterday, needs correction |
| K003 | Lia Permata | Crew | Kitchen (Bar display) | 555555 | `staff/staff-waitress-female.jpg` | |

Crew without a suitable portrait can also use the initials avatar ([ICONOGRAPHY §3.1](design/ICONOGRAPHY.md#31-product-and-brand-photography-primary)).

## 5. Outlet settings

| Setting | Value |
|---|---|
| Tables | Meja 1 – Meja 12 (Meja 12 inactive, its QR regenerated) |
| Tax (PB1) | 10 % |
| Service charge | 5 % |
| Price mode | Excluding tax and service |
| Refund cap | Rp 500.000 per Bill |
| Escalation timers | Claim re-notify 2 min · Outlet Admin alert 5 min · pickup reminder 3 min · empty-Bill close 30 min · auto clock-out grace 2 h |
| Prep Area capacity | Kitchen 2 · Bar 1 |
| Shifts | Pagi 08.00–16.00 · Malam 15.00–23.00 |

### Discounts

| Discount | Kind | Scope | Trigger | Rule | Demonstrates |
|---|---|---|---|---|---|
| Happy Hour Kopi | 15 % | Item | Auto | Coffee and Tea, daily 14.00–17.00 | Scheduled promo; Happy Hour countdown slide |
| Weekday Latte | 10 % | Item | Auto | Signature Caffè Latte, Mon–Fri all day | **Best discount wins** (15 % beats 10 % during Happy Hour) |
| Paket Hemat | Rp 25.000 | Bill | Auto | Subtotal ≥ Rp 350.000 | Bill-level discount on top of item discounts |
| Diskon Manual | 10 % | Item or Bill | Manual | Management PIN Override | BR-R-17 |

### Product Display playlist

| # | Template | Content | Notes |
|---|---|---|---|
| 1 | Hero Promo | `marketing/hero-display-banner.jpg`, "Sushi & kopi, satu meja." | |
| 2 | Featured Product | Signature Caffè Latte | Steam overlay |
| 3 | Happy Hour Countdown | Happy Hour Kopi | Shows "starts in …" outside the window |
| 4 | Category Spotlight | Nigiri + Rolls, background `marketing/ingredients-flatlay.jpg` | Unagi 4 pcs shows "Habis" chip |
| 5 | Featured Product | Sakura Latte | **Skipped**: Product is Hidden |
| 6 | Featured Product | Hojicha Caramel Latte | Still shown (Hot available); Iced marked Habis |
| 7 | Brand Idle | Animated logo over `marketing/cafe-interior-ambiance.jpg` | |

## 6. Live scenarios at T0

Times are offsets from T0 (−3m = three minutes before the seed ran).

### 6.1 Tables

| Table | Customers (restriction) | State at T0 | Demonstrates |
|---|---|---|---|
| **Meja 1** | — | Free | First scan, name + restriction + consent, splash animation (OP-01, M1) |
| **Meja 2** | — | Bill opened −25m by a scan, no Orders | Auto-closes at T0+5m (G-01) |
| **Meja 3** | Raka (avoids Dairy), Laras | Laras's Order Served (Truffle Edamame). Raka's Order **Pending, unclaimed, −40s**: Signature Latte Hot (Dietary Warning: Dairy, acknowledged) + Salmon Aburi 2 pcs | Fresh Claim notification; Dietary Warning at Claim; **Rejoin prompt** "occupied by Raka, Laras" (G-02) |
| **Meja 4** | Bima | Order Pending unclaimed since −3m (Chicken Katsu Curry) — already re-notified; Waiter call unanswered since −6m — **escalated to Outlet Admin** | G-05 |
| **Meja 5** | Tasya, Yoga | Order **Claimed by W001 at −1m, not confirmed**: Iced Hojicha (**Sold Out flag**), Sakura Latte Iced (**unavailable flag**, Product Hidden after order), V60 Toraja | Review before confirm, Sold Out / Hidden flags (OP-08, G-23); customer can't edit (Claimed) |
| **Meja 6** | Dewi (avoids Gluten), Kevin | In Kitchen: Wagyu Donburi Preparing 5/12 min (on time); Tonkotsu Ramen Preparing 10/12 (amber); Crispy Gyoza 10 pcs Confirmed 11/10 (**late**, Dietary Warning: Gluten for Dewi). Bar: Uji Matcha Iced Preparing 2/4 | Kitchen timers, lateness, Dietary Warning strip; **cup-fill Estimated Wait** on Dewi's phone (M9) |
| **Meja 7** | Citra | Dragon Roll **Ready since −4m** (waiting pickup, re-notified); Kyoto Cold Brew Served; Spicy Salmon Roll Served then **Remake** (A001, reason "Kurang pedas") — REMAKE ticket at top of the Kitchen queue | G-08, G-09 |
| **Meja 8 + Meja 9** | Gilang, Nabila, Reza | **Merged** Bill (Meja 9 absorbed at −50m), **Settling** since −2m; cash Payment Rp 200.000 **Held by Waiter W002**; balance remaining | Merge, Settling blocks Customer ordering (G-04), Held by Waiter (G-20) |
| **Meja 10** | Intan, Fikri | **Split by item** into 2 Sub-bills. Sub-bill 1 (Intan) paid by QRIS, closed, Receipts printed. Sub-bill 2 (Fikri) unpaid; its Yuzu Cheesecake **Voided** (A001, "Jatuh saat disajikan") | Split, Void only on unpaid Sub-bill (G-10, G-12: Split now blocked) |
| **Meja 11** | Ayu | Item Cancelled by Ayu before Claim (Customer Request); Iced Hojicha **Cancelled by Kitchen, Out of Stock at −20m** (this set it Sold Out); Matcha Mille-Crêpe Cancelled by Kitchen from Confirmed (Wrong Order); Chicken Katsu Curry Preparing | Customer Cancel (G-03), Kitchen Cancel from Confirmed (G-07), Sold Out cascade origin (OP-08) |
| **Meja 12** | — | Inactive, QR regenerated | Old QR in the launcher shows "please ask staff" |

### 6.2 Takeaway

| Bill | Customer | State | Demonstrates |
|---|---|---|---|
| TA-1 | Sari | Paid by card at −8m; Signature Latte Iced Preparing (Bar), Yuzu Cheesecake Ready | Pay-first Takeaway (G-18) |
| TA-2 | Hana | Paid cash at the counter; all items Ready | Name call-out |
| TA-3 | Joko | Draft, unpaid; Order not yet sent to Kitchen | Order is held until paid |

### 6.3 Today's closed Bills (T0 −7h … −30m)

About 18 closed Bills fill today's reports, including:

| Case | Detail | Demonstrates |
|---|---|---|
| Reprinted Receipt | Meja 4 lunch Bill, reprinted by C001 | REPRINT marker, audit (G-15) |
| Unpaid Bill | Meja 9 at −3h, Customer left; closed by A001 "Pelanggan pergi tanpa bayar" | G-14 |
| Refund within the cap | Rp 52.000 on a Meja 6 Bill, approved by A001 | BR-R-20 |
| Refund above the cap | Rp 560.000 on a large group Bill, approved by M001 | Management-only above the cap |
| Pay now, serve later | Meja 2 lunch: paid while dessert was Preparing, confirmed by W003 | G-13 |
| Pending QRIS | Rp 94.300 recorded as "QRIS — pending verification" (A001) | Must be resolved before Shift Closing (G-16) |
| Manual discount | 10 % on a Bill, M001 PIN | BR-R-17 |
| Paket Hemat | Bill subtotal Rp 412.000 → −Rp 25.000 | Bill-level discount |
| Happy Hour | Coffee items at 15 % (seeded inside the last 14.00–17.00 window before T0) | Scheduled promo, best-discount selection |
| Claim takeover | W003 took over W001's Claim at −4h | G-06 audit |
| Move | Bill moved Meja 1 → Meja 3 at −5h | Move audit; Move onto occupied Table is blocked |

## 7. History (14 days before T0)

- **Volume:** 45–70 Bills per day (deterministic), peaks 12.00–14.00 and 18.00–20.00, ~15 % Takeaway.
- **Product mix:** Coffee and Nigiri lead; Ebi Tempura Roll appears only in Bills older than 7 days (now Archived).
- **Prep Time performance:** 88–93 % of Order Items on time per day, slightly worse at peaks.
- **Payment mix:** QRIS 55 %, cash 30 %, card 15 %.
- **Anonymisation showcase:** one Bill closed at **T0 −47h** still shows Customer names; one closed at **T0 −49h** shows "Customer". All older Bills are anonymised.
- **Post-outage Entries:** 2 Bills yesterday 19.10–19.40 entered by C001 after a simulated outage, marked "outage" (G-21).
- **Unpaid Bills:** 1 in the last 14 days besides today's.
- **Shift Closings:** one Drawer session per day; yesterday's difference Rp 0, the day before +Rp 5.000.

## 8. Cash Drawer and HRIS at T0

### Cash Drawer (today)

| Item | Value |
|---|---|
| Drawer session | Opened by C001 at T0 −5h, Opening Float Rp 500.000 |
| Cash Handover W001 | Expected Rp 312.000, received Rp 312.000 |
| Cash Handover W003 | Expected Rp 186.000, received Rp 166.000 → **−Rp 20.000** recorded against W003 |
| Held by Waiter | W002 Rp 200.000 (Meja 8 + 9) |
| Pending QRIS | Rp 94.300 (blocks Shift Closing until resolved) |
| Cash Refund | Rp 52.000 paid from the drawer |

### HRIS

| Item | Detail |
|---|---|
| Shifts | Today and the next 7 days, Pagi and Malam |
| Attendance today | Everyone on the current shift clocked in; W004 not clocked in yet (late) |
| Auto clock-out | K002 yesterday, flagged "auto", waiting for Outlet Admin correction (G-22) |
| Clock-out blockers | W002 holds cash (G-20); W001 has open Claims (Meja 5, Meja 7) to hand over (G-06) |
| Leave | W004 pending (next Monday–Tuesday) · K001 approved (next Thursday) · C002 rejected ("Jadwal penuh") |
| Lockout | W005 locked until T0 +15m |

## 9. Reset

| Trigger | Who | How |
|---|---|---|
| CLI | Developer | `go run ./cmd/seed` (or `make seed`) |
| Admin Panel → Settings → **Reset demo** | Management (PIN Override) | API endpoint, Demo mode only |
| Scheduler | System | 03.00 WIB daily, Demo mode only |

Mechanics:

1. The seed generator builds a **new** database file `data/sushi-latte.next.db` with T0 = now. It goes through the domain packages, so totals, statuses and audit entries are consistent.
2. Image variants (WebP) are generated once and cached in `data/images/`; later resets reuse them.
3. The API enters maintenance for ~1 s: it rejects writes with `503 demo_resetting`, closes its connection pool, atomically renames `.next.db` over `sushi-latte.db`, reopens the pool, regenerates the crypto-shredding key file and clears sessions.
4. It broadcasts `demo.reset` on every channel; all apps reload to their start screen, and Customers return to the scan prompt.
5. Target: under 3 seconds end to end.

## 10. Guided demo tours

Listed on the launcher, each with numbered steps and the screens to open side by side.

| # | Tour | Screens | Flows |
|---|---|---|---|
| 1 | **From scan to table** | Phone (Meja 1) · Waiter · Kitchen | Scan, restriction + warning, order, Claim, confirm, Preparing → Ready, cup-fill, Served |
| 2 | **Running out** | Kitchen · Waiter · Phone | Cancel with Out of Stock → Sold Out everywhere → flags on Meja 5 → re-enable by A001 |
| 3 | **Settling up** | Cashier · Waiter · Phone (Meja 8) | Settling lock, split by item, cash rounding, QRIS, two Receipts (preview), Unpaid Bill |
| 4 | **When things go wrong** | Waiter · Admin | Void with PIN, Remake, Refund within / above cap, audit log |
| 5 | **Closing the day** | Cashier · HRIS tablet | Cash Handover with difference, pending QRIS, Shift Closing, clock-out blockers, auto clock-out correction |
| 6 | **Same table, new phone** | Two phone tabs (Meja 3) | Rejoin as Raka → old tab shows "continued on another device"; join as new Customer |

## 11. Scenario catalogue

Where every flow branch can be seen right after a reset.

| Flow / gap | Where to look |
|---|---|
| OP-01 join, consent | Meja 1 |
| G-01 empty Bill auto-close | Meja 2 |
| G-02 Rejoin | Meja 3 (Tour 6) |
| G-03 Customer edits/cancels before Claim | Meja 3 Raka's Pending Order; Meja 11 history |
| G-04 Settling | Meja 8 + 9 |
| G-05 escalation | Meja 4 |
| G-06 Claim takeover | Today's audit (W003); W001 clock-out list |
| G-07 Kitchen Cancel from Confirmed | Meja 11 |
| G-08 Remake | Meja 7, Kitchen queue |
| G-09 waiting pickup | Meja 7 |
| G-10 Void on unpaid Sub-bill | Meja 10 |
| G-11 Move onto occupied Table blocked | Try moving Meja 6 → Meja 3 |
| G-12 Split after Payment blocked | Try splitting Meja 8 + 9 |
| G-13 pay now, serve later | Today's closed Bills |
| G-14 Unpaid Bill | Today's closed Bills, reports |
| G-15 Reprint | Today's closed Bills |
| G-16 pending QRIS | Cash Drawer, Shift Closing |
| G-17 cash Refund | Cash Drawer |
| G-18 Takeaway pay first | TA-1, TA-3 |
| G-19 Opening Float | Cash Drawer |
| G-20 Held by Waiter, handover, clock-out block | W002, W003 |
| G-21 Post-outage Entry | Yesterday's reports |
| G-22 auto clock-out | K002 |
| G-23 Hidden Product flagged | Meja 5 Sakura Latte |
| G-24 crypto-shredding | Bills at −47h vs −49h; key file rotation on reset |
| Sold Out cascade (OP-08) | Hojicha Iced (Meja 11 origin, Meja 5 flag), Unagi 4 pcs (manual) |
| Dietary Warnings | Meja 3 (Dairy), Meja 6 (Gluten) |
| Kitchen timers on time / amber / late | Meja 6 |
| Lockout | W005 |
| Discounts | Today's closed Bills; Product Display slide 3 |
| Product Display skip Hidden | Playlist slide 5 |

## 12. Customer names pool

Raka, Laras, Bima, Tasya, Yoga, Dewi, Kevin, Citra, Gilang, Nabila, Reza, Intan, Fikri, Ayu, Sari, Hana, Joko, Wulan, Bayu, Melati, Arif, Salsa, Rizky, Indah, Galih, Putu, Made, Ketut, Nanda, Vina. Historical Bills draw from this pool; names on Bills older than 48 hours are anonymised by the seed itself.
