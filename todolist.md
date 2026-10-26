# Sushi Latte — Development Todo List

Phase-by-phase tracking for the prototype. Each task cites the spec ID it implements (`BR-*` in [BRD](docs/BRD.md), `TR-*` in [TRD](docs/TRD.md), `G-*` gap decisions in [OVERVIEW-AND-FLOWS §6](docs/OVERVIEW-AND-FLOWS.md#6-gap-resolution-log)). A phase is done when every box is ticked **and** its "Done when" checks pass.

**Legend:** `[ ]` todo · `[~]` in progress · `[x]` done · `[-]` dropped (say why)

| Phase | Module | Status |
|---|---|---|
| 0 | Foundation | Done |
| 1 | Admin Panel | Done (2 checks deferred) |
| 2 | Product Display Showcase | Not started |
| 3 | Client Menu | Not started |
| 4 | Waiter / Server | Not started |
| 5 | Kitchen | Not started |
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
- [ ] Management edits a price; the change applies to new Orders only (BR-PC-08) — **moves to Phase 3**, where Orders snapshot prices (TR-DM-02). Phase 1 verifies the price change and its audit entry
- [x] Outlet Admin can toggle Sold Out but cannot edit prices (`server` permission test + browser check)
- [x] Pricing tests pass, including "best discount wins" and cash rounding
- [ ] Admin Panel passes the accessibility checklist — partially: labelled fields, focus rings, keyboard-reachable controls, colour + label on every status. The automated axe run and the keyboard-only pass happen with the Playwright suite in Phase 8

---

## Phase 2 — Product Display Showcase

- [ ] Migration: `display_slide` (template, image, focus, refs, schedule)
- [ ] Admin: playlist editor with the 5 templates + uploaded image/video (BR-ADM-10, MOTION.md §6)
- [ ] Display app (TV, dark, display surface): screen-mode login, 8 s dwell, 1 s transitions, preloads the next slide
- [ ] Templates: Hero Promo, Featured Product (Ken Burns, steam overlay), Happy Hour Countdown, Category Spotlight, Brand Idle
- [ ] Live updates on `display` and `menu` topics without reload (BR-PD-02)
- [ ] Skip Hidden Products; mark/skip Sold Out between slides, never mid-slide (BR-PD-03)
- [ ] Reduced-motion fallback: cross-fades only
- [ ] Seed: playlist of 7 slides (DEMO-AND-SEED §5)

**Done when**
- [ ] Display runs 30 min at 1920×1080 without frame drops or memory growth
- [ ] Hiding a Product in Admin removes it from rotation at the next transition
- [ ] Happy Hour slide counts down correctly inside and outside the window

---

## Phase 3 — Client Menu

**Joining a Table** (OP-01)
- [ ] Migrations: `bill`, `bill_table` (open-Bill partial unique index), `customer_entry`, `orders`, `order_item`, `order_item_modifier`, `order_item_dietary_warning`
- [ ] QR scan → Table state; open or join Bill inside `BEGIN IMMEDIATE` (BR-CM-01, TR-DM-01)
- [ ] Occupied prompt with full names: Rejoin / join as new / "not my table" (BR-CM-12, G-02)
- [ ] Customer bound to Bill; latest device wins; old device shows "continued on another device" (BR-CM-13, TR-AUTH-07)
- [ ] Name + Dietary Restriction + Dietary Note with explicit consent (BR-CM-02, BR-P-02)
- [ ] Encrypted customer fields with daily keys in `data/keys.json` (TR-RET-04)
- [ ] Empty Bill auto-close job (BR-R-26, G-01)

**Ordering** (OP-02)
- [ ] Menu: Active Products only, Sold Out greyed, ID/EN toggle (BR-CM-03, BR-CM-04)
- [ ] ProductCard, ProductDetailSheet, VariantPicker, ModifierGroup (COMPONENTS.md §2)
- [ ] Dietary Warning badge + acknowledgement; never blocks (BR-CM-10, BR-CM-11, BR-R-D1..D4)
- [ ] Cart and submit; server re-check of Active / Sold Out; snapshots; warnings computed on the server (TR-DM-02, TR-DM-05)
- [ ] Edit or Cancel own Order before Claim (BR-CM-14, G-03)
- [ ] Settling lock message (BR-CM-15) — wired now, triggered from Phase 4
- [ ] Call Waiter button (BR-CM-07); `waiters` event emitted
- [ ] Order status + cup-fill Estimated Wait screen (BR-CM-08, MOTION M8–M10); Estimated Wait formula (TR-KT-03)

**Motion and polish**
- [ ] Signature moments M1–M7, M11, M12 (MOTION.md §5); reduced-motion fallbacks
- [ ] Performance budget: LCP ≤ 2.5 s on a mid-range phone, 60 fps animations (TR-NF-02, MOTION.md §3)

**Seed**
- [ ] Meja 1–3 and Meja 11 customer-side states; Customers with restrictions (DEMO-AND-SEED §6.1)
- [ ] Demo launcher `/demo` with Table QR codes and "Open as Customer" links

**Done when**
- [ ] Two phones scanning the same QR at the same moment produce one Bill (integration test)
- [ ] Rejoin on a second tab signs out the first tab
- [ ] A Dietary Warning Order is created with the warning stored on the Order Item
- [ ] Client Menu passes the accessibility checklist with TalkBack

---

## Phase 4 — Waiter / Server

**Claim** (OP-03)
- [ ] ClaimNotification with sound and vibration; first-writer-wins Claim (BR-WT-01, TR-SM-03)
- [ ] Re-notify after 2 min, escalate to Outlet Admin after 5 min (BR-WT-10, G-05)
- [ ] Takeover and reassign with `claim_transfer` + audit (BR-WT-11, BR-R-34, G-06)
- [ ] Review and confirm Order; Dietary Warnings and Sold Out/unavailable flags highlighted (BR-WT-02, BR-WT-05, BR-R-35, G-23)
- [ ] Direct entry with optional name and restriction (BR-WT-03)
- [ ] Cancel with Cancel Reasons; Out of Stock → Sold Out cascade and flags (BR-WT-04, BR-R-10..13, TR-SM-04)

**Serving**
- [ ] Ready notifications, Served, pickup reminder after 3 min (BR-WT-09, G-09)
- [ ] Remake with Outlet Admin PIN (BR-WT-12, BR-R-33, G-08)
- [ ] Void with PIN Override on unpaid Bill or Sub-bill (BR-R-08, G-10)

**Bills and payment** (OP-09, OP-10)
- [ ] TableMap and BillSummary
- [ ] Move (blocked onto an occupied Table), Merge, Split by item / equally (blocked after Payment) (BR-WT-06, BR-R-03, BR-R-04, BR-R-28, BR-R-29)
- [ ] Settling lock and unlock (BR-R-27, BR-WT-15, G-04)
- [ ] PaymentPad: cash with rounding and change, QRIS verify / pending with PIN, card (BR-R-18, BR-R-19, BR-R-30)
- [ ] Waiter cash recorded as Held by Waiter (BR-WT-14)
- [ ] Pay now, serve later confirmation (BR-R-31, G-13)
- [ ] Unpaid Bill with Outlet Admin PIN (BR-R-32, G-14)
- [ ] Close Bill / Sub-bill

**Receipts** (TRD §10)
- [ ] ESC/POS encoder, 58 mm layout; Customer and Internal copies (BR-R-21, TR-PRN-05)
- [ ] Web Bluetooth transport, chunked writes, retry; Web Serial fallback on Windows (TR-PRN-01..07)
- [ ] ReceiptPreview when no printer is paired (demo)
- [ ] Reprint marked REPRINT + audit (BR-WT-13, BR-R-37, G-15)
- [ ] BLE printer hardware spike (TR-TST-04) — only if real printing is shown

**Seed**
- [ ] Meja 3–10 waiter-side states, merged/Settling Bill, split Bill with Void, today's closed Bills (DEMO-AND-SEED §6.1, §6.3)

**Done when**
- [ ] Claim race integration test: exactly one winner
- [ ] Out of Stock Cancel greys the Variant on every open menu within 1 s
- [ ] Split by item and equally match `pricing` totals to the Rupiah
- [ ] Waiter flow is fully usable one-handed on an 8″ portrait tablet

---

## Phase 5 — Kitchen

- [ ] Kitchen and Bar displays (screen mode, dark, kitchen surface) (BR-KT-01)
- [ ] KitchenTicket + PrepTimer: on time / amber ≥ 80 % / late (BR-KT-02, TR-KT-01)
- [ ] Start → Preparing → Ready with 64 px targets (BR-KT-03)
- [ ] Cancel from Confirmed or Preparing with reason; Out of Stock cascade (BR-KT-04, G-07)
- [ ] Dietary Warning strip and notes prominent (BR-KT-05)
- [ ] REMAKE tickets first; "waiting pickup" marker (BR-KT-06)
- [ ] Estimated Wait recalculated and pushed on every queue change (TR-KT-03, TR-KT-04)
- [ ] Prep Time performance report (TR-KT-05)
- [ ] Seed: Meja 6 timers, Meja 7 Remake and waiting pickup, Meja 11 kitchen cancels

**Done when**
- [ ] Ticket readable from 3 m in a kitchen-lighting test (ACCESSIBILITY.md §4)
- [ ] Customer cup-fill progress moves when the Kitchen marks items Ready

---

## Phase 6 — Cashier

- [ ] Cashier landscape split view: Bills list + Bill detail with PaymentPad (LAYOUT.md §3)
- [ ] All Waiter Bill and Payment capabilities (BR-CS-01)
- [ ] Takeaway Bills paid before the Kitchen (BR-CS-02, TR-PAY-04, G-18)
- [ ] Refunds within / above the cap, paid in cash from the drawer (BR-CS-03, BR-R-20, G-17)
- [ ] Cash Drawer: Opening Float per drawer session (BR-CS-04, G-19)
- [ ] Cash Handover with difference against the Waiter (BR-CS-05, BR-R-36, G-20)
- [ ] Shift Closing for the drawer; pending QRIS must be resolved first (BR-CS-06, BR-CS-07, TR-PAY-01..03)
- [ ] Post-outage Entry mode (BR-CS-08, G-21)
- [ ] Reports: payment methods, best sellers, Shift Closing, Unpaid Bills, Remakes, post-outage (BR-ADM-11)
- [ ] Seed: TA-1..TA-3, Cash Drawer state, refunds, pending QRIS, post-outage history, 14-day history (DEMO-AND-SEED §6.2, §7, §8)

**Done when**
- [ ] Expected cash equals Float + handovers + counter cash − change − cash refunds (integration test)
- [ ] Shift Closing is refused while a pending QRIS exists

---

## Phase 7 — HRIS

- [ ] Employee records extending Crew (BR-HR-01)
- [ ] Shift scheduling (BR-HR-02)
- [ ] ClockInPad on the shared HRIS tablet only (BR-HR-03, TR-AUTH-08)
- [ ] Clock-out blockers: held cash, open Claims (BR-HR-07, BR-WT-11)
- [ ] Auto clock-out at shift end + 2 h; correction with reason (BR-HR-06, G-22)
- [ ] Leave requests and approval (BR-HR-04)
- [ ] Attendance export (BR-HR-05)
- [ ] Seed: shifts, attendance, K002 auto clock-out, leave requests (DEMO-AND-SEED §8)

**Done when**
- [ ] W002 cannot clock out until the Cashier receives the handover
- [ ] Attendance export matches the seeded shifts

---

## Phase 8 — Demo polish

- [ ] Anonymisation job + key destruction; −47 h / −49 h showcase (TR-RET-01, TR-RET-04, G-24)
- [ ] Reset demo: Admin button with Management PIN, nightly 03:00, `demo.reset` reload (DEMO-AND-SEED §9)
- [ ] Guided demo tours 1–6 on the launcher (DEMO-AND-SEED §10)
- [ ] Walk the scenario catalogue: every row visible right after a reset (DEMO-AND-SEED §11)
- [ ] Accessibility pass on every surface; axe in Playwright (ACCESSIBILITY.md §4)
- [ ] End-to-end Playwright flow: scan → order → Claim → Kitchen → Ready → pay → receipt (TR-TST-03)
- [ ] Load test at 300 Orders/hour (TR-TST-05)
- [ ] Style guide regenerated and matches the shipped tokens

**Done when**
- [ ] A presenter can run all six tours back to back after one reset without touching the database
