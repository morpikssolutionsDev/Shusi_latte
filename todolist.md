# Sushi Latte — Development Todo List

Phase-by-phase tracking for the prototype. Each task cites the spec ID it implements (`BR-*` in [BRD](docs/BRD.md), `TR-*` in [TRD](docs/TRD.md), `G-*` gap decisions in [OVERVIEW-AND-FLOWS §6](docs/OVERVIEW-AND-FLOWS.md#6-gap-resolution-log)). A phase is done when every box is ticked **and** its "Done when" checks pass.

**Legend:** `[ ]` todo · `[~]` in progress · `[x]` done · `[-]` dropped (say why)

| Phase | Module | Status |
|---|---|---|
| 0 | Foundation | Done |
| 1 | Admin Panel | Done (2 checks deferred) |
| 2 | Product Display Showcase | Done |
| 3 | Client Menu | Done (TalkBack pass needs a device) |
| 4 | Waiter / Server | Done (no printer hardware) |
| 5 | Kitchen | Done (3 m test needs the venue) |
| 6 | Cashier | Not started |
| 7 | HRIS | Not started |
| 8 | Demo polish | Not started |

### Cross-phase dependencies

- **Reports** (BR-ADM-11): Phase 1 builds the report screen and daily sales; each later phase adds the report for the data it creates.
- **Client Menu before Waiter**: in Phase 3, customer Orders stay **Pending** until Phase 4 adds Claim. Phase 3 is complete when Orders are created correctly and visible to the API and tests.
- **Waiter cash before Cashier**: Phase 4 records waiter cash as **Held by Waiter**; the Cash Handover arrives in Phase 6.
- **Crew accounts before HRIS**: login and crew records come in Phases 0–1; HRIS (Phase 7) adds shifts, attendance and leave on top.
- **Seed data grows with each phase**: every phase adds its scenarios from [DEMO-AND-SEED.md](docs/DEMO-AND-SEED.md) to the seed generator.

---

## Phase 0 — Foundation

Shared base that every module needs. **Done 2026-10-26.**

**Repository and tooling**
- [x] Monorepo layout per TR §3: `api/` (Go module), `web/` (pnpm workspaces + Turborepo), `deploy/`, `tools/`, `scripts/`
- [x] Go: chi, sqlc (SQLite engine), goose, `modernc.org/sqlite`, `coder/websocket`
- [x] Web: React + Vite + TypeScript apps `client-menu`, `staff`, `display`; packages `ui`, `api-client`
- [-] Package `printing` — moved to Phase 4, where receipts are built
- [x] Format, vet, typecheck and test commands; GitHub Actions CI runs them on every push (TR-DEP-03). Web lint is typecheck only for now
- [x] `deploy/Caddyfile` with `tls internal` + `pnpm demo` start script (TR-ARC-05, TR-DEP-01)
- [x] Pinned dev tools (sqlc 1.31.1, Caddy 2.11.7) fetched by `tools/fetch-tools.ps1` into `.tools/`

**Database and core**
- [x] SQLite connection: WAL, `busy_timeout`, `foreign_keys = ON`, single-writer pool with `BEGIN IMMEDIATE`, separate read-only pool (ADR-0005)
- [x] Migration 00001: `crew`, `crew_station`, `session`, `override_token`, `audit_log` (+ append-only triggers, TR-DM-04), `ops_setting`, `idempotency_key`
- [x] UUIDv7 and time helpers (money helpers land with `pricing` in Phase 1)
- [x] In-process event bus that publishes only after `COMMIT` (TR-ARC-03)
- [x] WebSocket hub: cookie auth, per-principal topic rules, subscribe/ack, heartbeat; client reconnect with backoff ≤ 10 s and refetch on reconnect (TR-RT-01..04)
- [x] Idempotency-Key middleware (TR-API-01); RFC 9457 errors (TR-API-02)
- [x] OpenAPI 3 spec (`api/openapi.yaml`) + generated TypeScript client (`web/packages/api-client`)
- [x] Scheduler for in-process jobs; session, override-token and idempotency cleanup jobs (TRD §19)
- [x] Audit log writer used inside the acting transaction (TR-AUD-01..03)
- [x] Media route `/media/images/*` serving `data/images`

**Auth**
- [x] Crew ID + 6-digit PIN login, Argon2id hashes (TR-AUTH-01, BR-R-24)
- [x] Lockout: 5 failures → 15 min; per-IP rate limit (TR-AUTH-02)
- [x] Server-side sessions; Management 30 min idle (TR-AUTH-04); Crew 12 h (TR-AUTH-09 — the "shift end" bound arrives with shifts in Phase 7)
- [x] Role and Station checks per BRD §5.1 (TR-AUTH-05)
- [x] PIN Override: `/auth/override` → one-time token valid 60 s, consumed inside the action's transaction with an audit entry (TR-AUTH-06, TR-API-03)
- [x] Screen-mode sessions for Kitchen, Bar, Display and HRIS, until 04:00 WIB (TR-AUTH-08)

**Design system in code**
- [x] Style Dictionary build: `docs/design/tokens/*.json` → CSS variables (light, dark, 5 surfaces, components), Tailwind theme, shadcn variable names, TS motion constants (TOKENS.md §5)
- [x] Fonts self-hosted via Fontsource: Poppins, Plus Jakarta Sans, JetBrains Mono (TYPOGRAPHY.md §1)
- [x] First themed components: Button, Input, Label, Card, PinPad, Logo; `[data-surface]` and `[data-theme]` switching
- [x] Lucide domain icon map (ICONOGRAPHY.md §1); custom Dietary Tag / Prep Area icons arrive with the catalog in Phase 1
- [x] i18next in the staff app (ID); Client Menu ID/EN is set up in Phase 3
- [x] Brand assets wired from `docs/brand`: animated logo, favicon, PWA manifest with 192/512 PNG icons

**Demo foundation**
- [x] `SL_DEMO_MODE` flag; demo banner in the staff app (DEMO-AND-SEED §2)
- [x] Seed generator `api/cmd/seed` (`pnpm seed`): T0 anchoring, builds `.next.db` then swaps it in (API must be stopped; the in-app reset comes in Phase 8)
- [x] Seed: 12 demo Crew with PINs and portraits, W005 locked out (DEMO-AND-SEED §4)
- [x] Staff login screen with "Demo accounts" panel

**Done when**
- [x] `pnpm demo` brings up API + Caddy; the LAN address serves HTTPS that verifies against the downloaded demo CA (verified with OpenSSL and curl)
- [x] Every demo Crew can log in; W005 is locked out (`423 account_locked`)
- [x] A PIN Override round trip writes an audit entry with actor and approver (`auth` tests)
- [x] An event published after commit reaches a subscribed WebSocket client; a rolled-back transaction publishes nothing (`dbx` and `server` tests)

---

## Phase 1 — Admin Panel

**Done 2026-10-26**, except the two checks that need Orders (noted below).

**Product catalog** (BRD §6.0, TRD §4.2)
- [x] Migration 00002: `prep_area`, `category`, `product`, `variant`, `modifier_group`, `modifier`, `dietary_tag`, `product_dietary_tag`, `modifier_dietary_tag`
- [x] Catalog API: `GET /catalog` with ETag; whole-Product save in one transaction; `menu` topic after commit (TR-PC-05)
- [x] Product Status Active / Hidden / Archived; Variants, groups and Modifiers are archived, never deleted (BR-PC-03, TR-PC-03)
- [x] Every Product has at least one Variant; exactly one default; price and Sold Out on the Variant (BR-PC-02, TR-PC-01)
- [x] Dietary Tag list management; Modifiers can add tags (BR-PC-06, BR-R-D4)
- [x] Image upload: content-addressed `img/<hash>`, WebP sq320/640/1024 + 16:9 crop at the focal point, immutable caching (TR-DM-09)
- [x] Admin screens: Products (list, search, category filter), product editor (details, Variants, Modifier groups, tags, photo + focal point, Prep Time, status), Categories, Prep Area capacity, Dietary Tags

**Sold Out** (BR-ADM-07, BR-R-12)
- [x] Toggle Sold Out per Variant (Outlet Admin, Management); set and re-enable audit-logged; menus notified

**Settings and pricing** (BR-ADM-04..06, BR-ADM-13)
- [x] Tax, service charge, price mode (`pricing_setting`)
- [x] Discounts: percent/nominal, item/Bill, auto/manual, day mask, hour window (incl. overnight), validity, min subtotal, targets
- [x] `internal/pricing` pure package with table-driven tests covering BR-R-14..18 and TR-PR-01..06
- [x] Refund cap; escalation timers (`ops_setting`)

**Tables** (BR-ADM-08)
- [x] Tables CRUD, active flag, QR codes (printable tent cards), QR regeneration invalidates the old token

**Crew** (BR-ADM-09)
- [x] Crew records: role, Stations, Crew ID, activation, PIN reset (clears lockout, ends sessions), initials avatar; last active Management protected
- [ ] Portrait upload in the Crew dialog (API accepts `imageKey`; the dialog only keeps the existing portrait) — small follow-up

**Reports and audit** (BR-ADM-11, BR-ADM-12)
- [x] Reports page shell (Management)
- [-] Daily sales report — moved to Phase 4: it needs closed Bills and Payments
- [x] Audit log viewer: newest first, action-prefix filter, paging (Outlet Admin, Management)

**Seed**
- [x] Full Product catalog (20 Products, 16 photos), Modifiers, 13 Dietary Tags, processed images cached between seeds (DEMO-AND-SEED §3)
- [x] Settings, 5 discounts, 12 Tables (Meja 12 inactive with regenerated QR) (DEMO-AND-SEED §5)
- [x] Unagi 4 pcs manually Sold Out by A001

**Done when**
- [x] Management edits a price; the change applies to new Orders only (BR-PC-08) — **verified in Phase 3** (`TestPriceSnapshot`), where Orders snapshot prices (TR-DM-02). Phase 1 verifies the price change and its audit entry
- [x] Outlet Admin can toggle Sold Out but cannot edit prices (`server` permission test + browser check)
- [x] Pricing tests pass, including "best discount wins" and cash rounding
- [ ] Admin Panel passes the accessibility checklist — partially: labelled fields, focus rings, keyboard-reachable controls, colour + label on every status. The automated axe run and the keyboard-only pass happen with the Playwright suite in Phase 8

---

## Phase 2 — Product Display Showcase

**Done 2026-10-27.**

- [x] Migration 00003: `display_slide` (template, headlines/sublines ID+EN, image + focus, video, Product / discount / category refs, steam, duration, schedule)
- [x] Admin: playlist editor with the 5 templates + uploaded image or MP4/WebM video; reorder, active switch, "skipped now" marker from the live playlist; Outlet Admin read-only (BR-ADM-10, MOTION.md §6)
- [x] Display app (TV, dark, display surface): per-slide dwell (default 8 s), 1 s transitions, preloads the next slide's images 1 s ahead, scales a 1920×1080 stage to any screen. Runs **without a session**: the playlist is public (TR-AUTH-08 updated, TR-RT-01)
- [x] Templates: Hero Promo, Featured Product (Ken Burns, steam overlay), Happy Hour Countdown, Category Spotlight, Brand Idle
- [x] Live updates on `display` and `menu` topics without reload; new playlist applied at the next transition (BR-PD-02, TR-DM-10)
- [x] Skip Hidden / Archived / fully Sold Out featured Products; partly Sold Out Variants marked; never mid-slide (BR-PD-03)
- [x] Reduced-motion fallback: cross-fades only, videos paused on their first frame
- [x] Seed: playlist of 7 slides, Sakura Latte slide skipped while Hidden (DEMO-AND-SEED §5)

**Done when**
- [x] Display runs 30 min at 1920×1080 without frame drops or memory growth (`.tools/soak-display.mjs` on the production build: heap 2.9 → 4.7 MB, 60 fps at every 5-minute sample, worst frame 19 ms). A first run against the Vite dev server crashed when `pnpm gen` hot-reloaded the entry module twice; the built app has no hot reload
- [x] Hiding a Product in Admin removes it from rotation at the next transition (`.tools/flow-display.mjs`)
- [x] Happy Hour slide counts down correctly inside and outside the window (`display` tests: active window, next window, overnight, weekend skip)

---

## Phase 3 — Client Menu

**Done 2026-10-27**, except the TalkBack pass (needs a real Android device; noted below).

**Joining a Table** (OP-01)
- [x] Migration 00004: `bill`, `bill_table` (open-Bill partial unique index), `customer_entry`, `guest_token`, `orders`, `order_item`, `order_item_modifier`, `order_item_dietary_warning`, `waiter_call`
- [x] QR scan → Table state; open or join Bill inside `BEGIN IMMEDIATE` (BR-CM-01, TR-DM-01)
- [x] Occupied prompt with full names: Rejoin / join as new / "not my table" (BR-CM-12, G-02)
- [x] Customer bound to Bill; latest device wins; old device shows "continued on another device" (BR-CM-13, TR-AUTH-07). Guest token per browser tab (`X-Guest-Token`), so one laptop can play several Customers
- [x] Name + Dietary Restriction + Dietary Note with explicit consent; without consent the dietary data is discarded (BR-CM-02, BR-P-02)
- [x] Encrypted customer fields with daily keys in `data/keys.json` (`internal/vault`, AES-256-GCM, master key `SL_MASTER_KEY`); key destruction and Anonymisation jobs (TR-RET-01, TR-RET-04)
- [x] Empty Bill auto-close job (BR-R-26, G-01)

**Ordering** (OP-02)
- [x] Menu: Active Products only, Sold Out greyed, ID/EN toggle following the phone language (BR-CM-03, BR-CM-04, TR-I18N-01); current promo prices from `pricing`
- [x] ProductCard, ProductDetailSheet, VariantPicker, ModifierGroup, RestrictionPicker, CartSheet, OccupiedTablePrompt, OrderStatusTracker (COMPONENTS.md §2); Dietary Tag icons in `ui`
- [x] Dietary Warning badge + acknowledgement; never blocks; the server rejects unacknowledged warnings (BR-CM-10, BR-CM-11, BR-R-D1..D4)
- [x] Cart priced by a server quote; submit with Idempotency-Key; server re-check of Active / Sold Out / Modifier rules with per-line reasons; snapshots; warnings computed on the server (TR-DM-02, TR-DM-05)
- [x] Edit or Cancel own Order before Claim; dropped lines become Cancelled (Customer Request) (BR-CM-14, G-03)
- [x] Settling lock message and disabled "Kirim pesanan" (BR-CM-15) — wired now, triggered from Phase 4
- [x] Call Waiter button, one open call per Customer (BR-CM-07); `waiters` event emitted
- [x] Order status + cup-fill Estimated Wait screen (BR-CM-08, MOTION M8–M10); Estimated Wait formula in `internal/kitchen` with tests (TR-KT-03). The wait appears once Orders are confirmed (Phase 4)
- [x] Promo edge job publishes `menu` when scheduled discounts start or stop

**Motion and polish**
- [x] Signature moments M1–M12 (MOTION.md §5); reduced-motion fallbacks (fades, no steam, still cup)
- [x] Performance budget: LCP on the production build with 4× CPU and ~9 Mbps/170 ms — join screen 1.5 s, menu 2.3 s (`.tools/perf-client.mjs`) (TR-NF-02). The demo launcher is lazy-loaded

**Seed**
- [x] Meja 2 (empty Bill −25m), Meja 3 (Raka avoids Dairy, Laras), Meja 4 (Bima, Pending Order, Waiter call −6m), Meja 11 (Ayu's own Cancel) customer-side states (DEMO-AND-SEED §6.1). Meja 5–10 Customers arrive with the phases that drive their states
- [x] Demo launcher `/demo` with Table QR codes, scenario labels and "Open as Customer" links (`GET /demo/tables`)
- [x] BR-PC-08 price snapshot test (moved from Phase 1)

**Done when**
- [x] Two phones scanning the same QR at the same moment produce one Bill (`ordering` test: 8 concurrent joins → one Bill, one first scan)
- [x] Rejoin on a second tab signs out the first tab (`.tools/flow-client.mjs`)
- [x] A Dietary Warning Order is created with the warning stored on the Order Item (`ordering` test + browser flow)
- [ ] Client Menu passes the accessibility checklist with TalkBack — partially: every control is a labelled button/input, cards are single buttons with name and price, progress is a `progressbar` with text, status changes are `aria-live`. The TalkBack pass needs an Android phone; the automated axe run is in Phase 8

---

## Phase 4 — Waiter / Server

**Done 2026-10-27**, except the BLE hardware spike (no printer on hand); today's closed Bills are seeded in Phase 6.

**Claim** (OP-03)
- [x] ClaimNotification with sound and vibration; first-writer-wins Claim (BR-WT-01, TR-SM-03) — chime after a tap enables sound (browser autoplay rule)
- [x] Re-notify after 2 min, escalate to Outlet Admin after 5 min, for Orders and Waiter Calls (BR-WT-10, G-05)
- [x] Takeover and reassign with `claim_transfer` + audit (BR-WT-11, BR-R-34, G-06)
- [x] Review and confirm Order; Dietary Warnings and Sold Out/unavailable flags highlighted; flagged items must be Cancelled before confirm (BR-WT-02, BR-WT-05, BR-R-35, G-23)
- [x] Add lines while reviewing; direct entry with optional name and restriction, opening the Bill if needed (BR-WT-03)
- [x] Cancel with Cancel Reasons; Out of Stock → Sold Out cascade and flags; manual Sold Out flags too (BR-WT-04, BR-R-10..13, TR-SM-04)

**Serving**
- [x] Ready notifications, Served, pickup reminder after 3 min (BR-WT-09, G-09) — Ready comes from the Kitchen in Phase 5
- [x] Remake with Outlet Admin PIN (BR-WT-12, BR-R-33, G-08)
- [x] Void with PIN Override on unpaid Bill or Sub-bill; a closed Sub-bill's items need a Refund (BR-R-08, G-10)

**Bills and payment** (OP-09, OP-10)
- [x] TableMap and BillSummary (totals from `pricing`)
- [x] Move (blocked onto an occupied Table), Merge, Split by item / equally (blocked after Payment) (BR-WT-06, BR-R-03, BR-R-04, BR-R-28, BR-R-29)
- [x] Settling lock and unlock, price fixed while Settling (BR-R-27, BR-WT-15, G-04, TR-DM-11)
- [x] PaymentPad: cash with rounding and change, QRIS verify / pending with PIN, card (BR-R-18, BR-R-19, BR-R-30, TR-PAY-05)
- [x] Waiter cash recorded as Held by Waiter (BR-WT-14)
- [x] Pay now, serve later confirmation (BR-R-31, G-13)
- [x] Unpaid Bill with Outlet Admin PIN (BR-R-32, G-14)
- [x] Manual Bill discount with Management PIN (BR-R-17) — manual item-level discounts are not offered yet
- [x] Close Bill / Sub-bill; totals frozen
- [x] Daily sales report (moved from Phase 1, BR-ADM-11)

**Receipts** (TRD §10)
- [x] `web/packages/printing`: 32-column layout shared by printer and preview; ESC/POS encoder; tests (BR-R-21, TR-PRN-05)
- [x] Web Bluetooth transport, chunked writes, retry; Web Serial fallback on Windows (TR-PRN-01..07) — untested on hardware
- [x] ReceiptPreview when no printer is paired (demo)
- [x] Reprint marked REPRINT + audit (BR-WT-13, BR-R-37, G-15)
- [-] BLE printer hardware spike (TR-TST-04) — no printer on hand; the demo uses the preview

**Seed**
- [x] Meja 3–11 waiter- and kitchen-side states: Claim with flags (Meja 5), merged Settling Bill with Held by Waiter cash (Meja 8+9), split Bill with paid Sub-bill and Void (Meja 10) — `seed_test.go` checks every Table at T0 (DEMO-AND-SEED §6.1)
- [-] Today's closed Bills (§6.3) — moved to Phase 6: they include Refunds, Cash Handovers and the Shift Closing figures

**Done when**
- [x] Claim race integration test: exactly one winner (`TestClaimRaceHasOneWinner` + browser race)
- [x] Out of Stock Cancel greys the Variant on every open menu within 1 s (141 ms in `.tools/flow-waiter.mjs`)
- [x] Split by item and equally match `pricing` totals to the Rupiah (`TestSplitMatchesPricing`)
- [x] Waiter flow is fully usable one-handed on an 8″ portrait tablet — bottom tab bar, 56–64 px primary buttons in the lower half (screenshots at 800×1280)

---

## Phase 5 — Kitchen

**Done 2026-10-27**, except the physical 3 m readability test.

- [x] Kitchen and Bar displays (screen mode, dark, kitchen surface) at `/staff/kitchen/kitchen` and `/staff/kitchen/bar`; a Kitchen/Bar screen session opens its display directly (BR-KT-01, TR-AUTH-08)
- [x] KitchenTicket + PrepTimer: on time / amber ≥ 80 % / TERLAMBAT, timed on the server clock (BR-KT-02, TR-KT-01)
- [x] Start → Preparing → Ready with 64 px targets (BR-KT-03)
- [x] Cancel from Confirmed or Preparing with reason; Out of Stock cascade (BR-KT-04, G-07)
- [x] Dietary Warning strip (solid Plum), Dietary Note and item notes prominent (BR-KT-05)
- [x] REMAKE tickets first; Ready column with "Menunggu diambil" after the pickup reminder time; "+N lagi" when tickets overflow (BR-KT-06, LAYOUT.md §3)
- [x] Estimated Wait recalculated and pushed on `bill:{id}` to every Bill waiting on the Prep Area on every queue change (TR-KT-03, TR-KT-04)
- [x] Prep Time performance report on the Reports page (TR-KT-05)
- [x] Seed: Meja 6 timers (on time / amber / late, Bar), Meja 7 Remake and waiting pickup, Meja 11 Kitchen cancels

**Done when**
- [ ] Ticket readable from 3 m in a kitchen-lighting test (ACCESSIBILITY.md §4) — partially: item text 24 px, timers 48 px, status in colour **and** words (TERLAMBAT, REMAKE); the physical test needs the venue screen
- [x] Customer cup-fill progress moves when the Kitchen marks items Ready (`.tools/flow-kitchen.mjs`: 95 → 100 %, "Siap!"; `TestEstimatedWaitMovesWithKitchen`)

---

## Phase 6 — Cashier

**Done 2026-10-27.** Verified by `cash_test.go`, `seed_test.go` (Cash Drawer, Takeaways, today's and 14 days of history) and `.tools/flow-cashier.mjs` (17/17 at 1280×800); the Waiter, Bills and Kitchen flows still pass.

- [x] Cashier landscape split view at `/staff/cashier`: Bills list (40 %) + Bill detail with the PaymentPad pinned to the bottom (60 %); tabs Tagihan · Laci kas · Pasca-gangguan (LAYOUT.md §3)
- [x] All Waiter Bill and Payment capabilities — the Waiter's BillPage reused with Cashier links; counter cash goes straight into the drawer (BR-CS-01)
- [x] Takeaway Bills paid before the Kitchen: one held Order, no manual Confirm, no direct Order on the Bill (`409 takeaway_unpaid`); full payment queues it and closes the Bill; its Receipt names the Takeaway (BR-CS-02, TR-PAY-04, G-18)
- [x] Refunds within / above the cap, paid in cash from the drawer; the dialog asks the approver the cap requires (BR-CS-03, BR-R-20, G-17)
- [x] Cash Drawer: Opening Float per drawer session; counter cash refused while the drawer is closed (BR-CS-04, G-19)
- [x] Cash Handover with the difference recorded against the Waiter (BR-CS-05, BR-R-36, G-20)
- [x] Shift Closing for the drawer; pending QRIS verified or written off first (BR-CS-06, BR-CS-07, TR-PAY-01..03)
- [x] Post-outage Entry form: original times, Crew ID from the paper Bill, items, Payments; the API answers with its total when the Payments fall short (BR-CS-08, G-21)
- [x] Reports: payment methods and best sellers (Phase 4) plus the Operations section — Shift Closings, Unpaid Bills, Remakes, Refunds, Post-outage Entries (BR-ADM-11)
- [x] Seed: TA-1..TA-3, today's drawer (C001 −5h, Rp 500.000, W001 exact, W003 −Rp 20.000, W002 holding Rp 200.000, pending QRIS Rp 94.300, Refunds Rp 52.000 / Rp 560.000), today's closed Bills (§6.3), 14-day history (45–70 Bills a day, ~90 % on time, QRIS/cash/card mix, daily Shift Closings, yesterday's two Post-outage Entries, one Unpaid Bill, −47h / −49h anonymisation), Ebi Tempura archived a week ago (DEMO-AND-SEED §6.2, §7, §8) — today's story is compressed into the business day when T0 is early in the morning

**Done when**
- [x] Expected cash equals Float + handovers + counter cash − change − cash refunds (`TestExpectedCashAndShiftClosing`, `seed_test.go`, Cashier flow)
- [x] Shift Closing is refused while a pending QRIS exists (`pending_qris`; the button is disabled with "Selesaikan 1 QRIS dulu.")

---

## Phase 7 — HRIS

**Done 2026-10-27.** Verified by `hris_test.go`, `TestClockOutNeedsHandover`, `seed_test.go` (at 15.00 and at the 03.00 nightly reset) and `.tools/flow-hris.mjs` (15/15).

- [x] Employee records beside the Crew account: employment type, start date, phone, emergency contact, notes (BR-HR-01) — Management, `/staff/hris/employees`
- [x] Shift scheduling: weekly grid, add / move / delete, Pagi and Malam presets, no overlaps, ≤ 16 h, worked shifts cannot be deleted (BR-HR-02)
- [x] ClockInPad on the shared HRIS tablet only: screen mode → `/staff/clock`; large clock, Crew ID + PinPad, result card with shift and lateness; the PIN shares the sign-in lockout (BR-HR-03, TR-AUTH-08, TR-HR-01, TR-HR-02)
- [x] Clock-out blockers: held cash refused with "Setor Rp X ke kasir dulu."; open Claims listed, then clock-out on acknowledgement (BR-HR-07, BR-WT-11, TR-HR-03)
- [x] Auto clock-out at shift end + 2 h every 5 min, flagged "auto"; correction with reason, audit-logged (BR-HR-06, G-22, TR-HR-04)
- [x] Leave requests from the Crew member's own page; approve / reject with reason by someone else; approved leave on the schedule and the board (BR-HR-04)
- [x] Attendance board per day (late, absent, working, leave, auto) and CSV export for payroll (BR-HR-05)
- [x] Seed: employee records; shifts for the past week, today (around T0) and the next 7 days; attendance played on the pad; W003 late, W004 not in; K002's auto clock-out last night; W004 pending, K001 approved, C002 rejected leave; W005 locked out last (DEMO-AND-SEED §8)

**Done when**
- [x] W002 cannot clock out until the Cashier receives the handover (`TestClockOutNeedsHandover`, seed test, HRIS flow)
- [x] Attendance export matches the seeded shifts (`TestExportMatchesShifts`; the flow's 7-day export has one row per seeded shift: 68 = 68)

---

## Phase 8 — Demo polish

**Done 2026-10-27.** Verified by the Go suite (including the seed at 15.00 and at the 03.00 reset), `.tools/tours.mjs` (6/6 on one reset), `.tools/flow-reset.mjs`, `.tools/flow-e2e.mjs`, `.tools/a11y.mjs` and `.tools/load.mjs`; every earlier flow re-run green.

- [x] Anonymisation job + key destruction; −47 h / −49 h showcase: the seed anonymises and destroys keys older than three days like the nightly jobs; Reports → "Tagihan ditutup" shows names, or "Customer" with a Dianonimkan chip (TR-RET-01, TR-RET-04, G-24)
- [x] Reset demo: Admin → Pengaturan → Reset demo with Management PIN (`demo_reset`), nightly 03.00 in Demo mode; `demo.resetting` shows a notice on every app, the file swaps in ~15 ms, `demo.reset` sends Staff to sign-in, the phone to the start and the TV round again (DEMO-AND-SEED §9) — the whole reset takes ~8–10 s (the 14-day build), recorded in §9
- [x] Guided demo tours 1–6 on the launcher (`/demo`), with deep links to each screen, Indonesian and English (DEMO-AND-SEED §10)
- [x] Walk the scenario catalogue: `checkCatalogue` in `seed_test.go` covers the rows not already asserted (takeover, reprint and Move audits, G-11, G-12, G-13, Sold Out cascade, Display skipping the Hidden Sakura) (DEMO-AND-SEED §11)
- [x] Accessibility pass on every surface: axe (WCAG 2.1 AA) on 40 routes, 0 serious/critical — fixed contrast from `opacity` dimming (inactive cards now dashed), Kitchen ticket headers and Siap buttons on `on-solid` tokens, product-tile names, the cup progressbar name, the photo focus button, a keyboard-reachable history table (ACCESSIBILITY.md §4)
- [x] End-to-end Playwright flow: scan → order → Claim → Bar → Ready → Served → card payment → Receipt, each on its own screen (TR-TST-03)
- [x] Load test: 3 000 Orders/hour (10× the 2× peak) for 2 min with 30 Client Menu sessions and 15 staff devices: reads p95 45 ms, writes p95 22 ms, commit → screen p95 18 ms, no 5xx (TR-TST-05, TR-NF-01, TR-RT-05)
- [x] Style guide regenerated (`python tools/design/build.py`): no changes, the shipped tokens match
- [x] CSV export on every report (`?format=csv`, links on the Reports page) (TRD §6.1)
- [x] Crew portrait upload in the Crew dialog

**Done when**
- [x] A presenter can run all six tours back to back after one reset without touching the database (`.tools/tours.mjs`: 6/6)

**Still open (need the venue or hardware):** the 58 mm BLE printer spike (TR-TST-04) and the 3 m Kitchen readability test (ACCESSIBILITY.md §4).
