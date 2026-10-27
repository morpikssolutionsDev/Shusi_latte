# Sushi Latte — Technical Requirements Document (TRD)

| | |
|---|---|
| Product | Sushi Latte |
| Document | Technical Requirements Document |
| Version | 1.1 (draft) — gap resolutions 2026-10-26 |
| Date | 2026-10-26 |
| Implements | [BRD.md](BRD.md) |
| Decisions | [ADRs](adr/) |

Domain terms follow [GLOSSARY.md](../GLOSSARY.md). Requirement IDs `BR-*` refer to the BRD; `TR-*` are defined here.

---

## 1. Architecture overview

The prototype runs on **one machine** (the demo host, e.g. a laptop) on the outlet's or presenter's Wi-Fi. One Go binary serves the API, the WebSocket hub, scheduled jobs and the three browser apps; data lives in a single SQLite file ([ADR-0005](adr/0005-sqlite-for-demo.md)). Phones, tablets and screens connect over the local network. Receipt printing goes from each staff device straight to its paired Bluetooth printer.

```
                     ┌──────────────────────── Demo host (one machine, LAN) ─────────────────────────┐
 Customer phone ───▶ │  Caddy (local HTTPS via internal CA, static files)                             │
  (Client Menu)      │     ├── /            → Client Menu SPA                                         │
                     │     ├── /staff       → Staff PWA (Admin, Waiter, Cashier, Kitchen, HRIS)       │
 Staff tablet/PC ──▶ │     ├── /display     → Product Display SPA                                     │
  (Staff PWA)        │     ├── /demo        → Demo launcher (Demo mode only)                          │
        │            │     ├── /api/v1      → Go API ──┐                                              │
        │ Bluetooth  │     └── /ws          → Go API ──┤  in-process event bus → WebSocket hub       │
        ▼            │                                 ▼                                              │
  58 mm printer      │                        SQLite file (WAL)  ·  key file (crypto-shredding)       │
                     │                        In-process scheduler (escalations, jobs, nightly reset)  │
 Kitchen / TV ─────▶ │                                                                                │
                     └────────────────────────────────────────────────────────────────────────────────┘
```

- **TR-ARC-01** One Go binary hosts the REST API, the WebSocket hub, the event bus and scheduled jobs. It is the only process that opens the database file.
- **TR-ARC-02** SQLite (WAL mode) is the single source of truth; no cache or message broker is needed at the BRD-NF-01 scale.
- **TR-ARC-03** Every state change that other screens must see is committed first, then published on the **in-process event bus**; the WebSocket hub fans it out to subscribed clients. Events are never published for a rolled-back transaction.
- **TR-ARC-04** The system needs the local network for everything except printing; it does not need the internet ([ADR-0003](adr/0003-online-only-with-future-local-server.md), [ADR-0005](adr/0005-sqlite-for-demo.md)).
- **TR-ARC-05** All devices reach the host over **HTTPS**, because Web Bluetooth, PWA install and the Service Worker require a secure context. Caddy issues certificates from its internal CA (`tls internal`); the CA root is installed once on each demo device, or the host is exposed through an HTTPS tunnel when devices cannot be configured.

## 2. Technology stack

| Layer | Choice | Reason |
|---|---|---|
| Backend language | Go (current stable) | Fast, single static binary, strong concurrency for WebSockets |
| HTTP router | `go-chi/chi` | Thin, idiomatic `net/http` |
| DB access | `sqlc` (SQLite engine) + `database/sql` | Type-safe SQL without an ORM |
| Migrations | `goose` | Plain SQL migrations, embeddable |
| WebSocket | `coder/websocket` | Maintained, context-aware |
| Database | SQLite (WAL) via `modernc.org/sqlite` | One file, pure Go (no CGO on Windows), transactions, partial unique indexes, JSON functions ([ADR-0005](adr/0005-sqlite-for-demo.md)) |
| Event bus | In-process publish/subscribe (Go channels) | Replaces Postgres `LISTEN/NOTIFY` in a single-process deployment |
| Images | `gen2brain/webp` (no cgo) + `golang.org/x/image/draw` | WebP variants 320/640/1024 px and 16:9 display crops, generated in the API on upload and at seed time |
| Frontend | React + Vite + TypeScript 5.9 | Fast SPA builds; no server rendering needed for QR menu or kiosk screens. TypeScript is pinned to 5.9: TypeScript 7 has no JavaScript compiler API, which `openapi-typescript` needs |
| Routing / data | TanStack Router, TanStack Query | Type-safe routes; server-state caching and WebSocket-driven invalidation |
| UI | Tailwind CSS, shadcn/ui, Motion (formerly Framer Motion) | Modern, accessible components and animation |
| PWA | `vite-plugin-pwa` | Installable Staff app, cached shell |
| i18n | `i18next` | ID/EN Client Menu; ID staff |
| Monorepo | pnpm workspaces + Turborepo | Shared UI, types and API client across apps |
| API contract | OpenAPI 3 | Generated TypeScript client for all apps |
| Edge | Caddy | Local HTTPS (internal CA), static serving, reverse proxy |
| Runtime | Native binaries on the demo host (Go API + Caddy), started by `pnpm demo` (`scripts/run.mjs`); sqlc and Caddy are pinned in `tools/fetch-tools.ps1` | No Docker needed |

Next.js was considered and not chosen: none of the apps need server rendering or SEO, and an extra Node server adds an operational component without benefit.

## 3. Repository structure

```
/
├── GLOSSARY.md
├── docs/ (BRD, TRD, adr/)
├── api/                      Go module
│   ├── cmd/server/           main
│   ├── cmd/seed/             demo seed generator (see docs/DEMO-AND-SEED.md)
│   ├── internal/
│   │   ├── auth/             Crew ID + PIN, sessions, PIN Override, lockout
│   │   ├── catalog/          Product catalog: Categories, Products, Variants, Modifiers, Dietary Tags, Sold Out, Prep Areas
│   │   ├── ordering/         Tables, Bills, Sub-bills, Orders, Order Items, Claim
│   │   ├── pricing/          discount, service, tax, rounding (pure functions)
│   │   ├── payment/          Payments, Refunds, Shift Closing
│   │   ├── kitchen/          queues, Estimated Wait
│   │   ├── display/          Product Display content
│   │   ├── hris/             employees, shifts, attendance, leave
│   │   ├── audit/            audit log
│   │   ├── realtime/         event bus, WebSocket hub
│   │   ├── demo/             Demo mode: launcher data, reset
│   │   └── jobs/             scheduler: escalations, Anonymisation, promos, nightly demo reset
│   ├── db/migrations/
│   └── db/queries/           sqlc
├── web/
│   ├── apps/
│   │   ├── client-menu/
│   │   ├── staff/            Admin Panel, Waiter, Cashier, Kitchen, HRIS (role-gated routes)
│   │   └── display/
│   └── packages/
│       ├── ui/               shadcn/ui components, theme
│       ├── api-client/       generated from OpenAPI
│       └── printing/         ESC/POS encoder + Web Bluetooth transport
├── deploy/                   Caddyfile, demo start script, backup script
└── data/                     (git-ignored) sushi-latte.db, keys file, uploaded and processed images
```

## 4. Data model

Types follow SQLite ([ADR-0005](adr/0005-sqlite-for-demo.md)):

- Money: `INTEGER` whole Rupiah.
- Timestamps: `INTEGER` UTC Unix milliseconds, converted to Asia/Jakarta for display in Go.
- Primary keys: UUIDv7 generated in Go, stored as `TEXT`.
- Enumerations: `TEXT` with `CHECK (… IN (…))` constraints.
- JSON: `TEXT` with SQLite JSON functions.
- Foreign keys: enforced (`PRAGMA foreign_keys = ON` on every connection).
- Table names are singular, except `orders`: `ORDER` is a reserved SQL keyword. The domain term stays **Order**.
- Query parameters use SQLite `?` placeholders.

There is no tenant or outlet column ([ADR-0001](adr/0001-single-business-single-outlet.md)).

### 4.1 People and access

| Table | Key columns |
|---|---|
| `crew` | id, crew_id (unique, human-readable), full_name, role (`management`, `outlet_admin`, `crew`), image_key (nullable; initials avatar when empty), pin_hash, failed_attempts, locked_until, active |
| `crew_station` | crew_id, station (`waiter`, `cashier`, `kitchen`) |
| `session` | id, crew_id, device_label, created_at, last_seen_at, expires_at |

### 4.2 Product catalog (shared baseline)

The Product catalog is the one source of Product data for every Module (BRD §6.0). Modules read it through the `catalog` package and never store their own copies; Order Items snapshot what they need at order time (TR-DM-02).

```
category ──1:N── product ──1:N── variant          (price, Sold Out)
   │                │  └──1:N── modifier_group ──1:N── modifier ──N:M── dietary_tag (adds)
   │                └──N:M── dietary_tag (contains)
   └── prep_area
```

| Table | Key columns |
|---|---|
| `prep_area` | id, name (Kitchen, Bar), parallel_capacity |
| `category` | id, name_id, name_en, sort, prep_area_id |
| `product` | id, category_id, name_id, name_en, description_id, description_en, image_key (nullable; WebP variants generated from the original), image_focus_x, image_focus_y (focal point in %, default 50/50, used for 16:9 crops), prep_time_seconds, status (`active`, `hidden`, `archived`), sort, created_at, updated_at |
| `variant` | id, product_id, name_id, name_en, price, is_default, sort, sold_out, sold_out_at, sold_out_by, sold_out_source (`manual`, `cancel_out_of_stock`) |
| `modifier_group` | id, product_id, name_id, name_en, min_select, max_select, sort |
| `modifier` | id, modifier_group_id, name_id, name_en, price, sort |
| `dietary_tag` | id, code (e.g. `dairy`, `raw_fish`, `pork`), name_id, name_en, icon, sort, active |
| `product_dietary_tag` | product_id, dietary_tag_id — what the Product contains |
| `modifier_dietary_tag` | modifier_id, dietary_tag_id — tags a Modifier adds |

Rules:
- **TR-PC-01** Every Product has at least one Variant; a Product with no options has one `is_default` Variant. Price and Sold Out therefore always live on `variant`.
- **TR-PC-02** A Product is orderable when `status = 'active'` and at least one Variant has `sold_out = false`. Derived, not stored: a Product's greyed-out state is computed from its Variants.
- **TR-PC-03** A Product that has ever been ordered cannot be deleted, only Archived; Variants and Modifiers likewise are soft-retired (`archived_at`), never deleted.
- **TR-PC-04** Dietary Tags are not removable by Variants or Modifiers in the MVP (BR-R-D4). The effective tags of an Order Item are `product tags ∪ chosen modifier tags`.
- **TR-PC-05** Catalog reads used by the Client Menu, Waiter and Cashier are served from one endpoint returning the full active catalog (Categories, Products, Variants with Sold Out, Modifiers, Dietary Tags) with an `ETag`; any catalog change publishes on the `menu` topic and clients refetch.
- **TR-PC-06** Catalog changes are audit-logged (TR-AUD-01).

### 4.3 Ordering

| Table | Key columns |
|---|---|
| `dining_table` | id, label, qr_token (random, regenerable), active |
| `bill` | id, business_date + number (daily sequence shown as `B-0427`; the business day turns over at 04:00 WIB), type (`dine_in`, `takeaway`), source (`live`, `post_outage`), status (`open`, `settling`, `closed`, `unpaid`, `merged`), key_id + takeaway_name_enc, pay_now_serve_later_by, unpaid_reason, unpaid_override_by, merged_into_id, opened_at, closed_at, anonymised_at |
| `bill_table` | bill_id, table_id, is_open (many tables per Bill after Merge) |
| `customer_entry` | id, bill_id, key_id, name_enc, dietary_note_enc, restriction_tags_enc (Dietary Restriction tag ids), consent_at, active_device_id (the current `guest_token`), joined_at, rejoined_at — one per person at the Table, bound to the Bill not the device |
| `guest_token` | id, customer_entry_id, token_hash (SHA-256; the token itself is never stored), created_at, revoked_at — one per device a Customer used; revoked rows let the old device show "You continued on another device" |
| `orders` | id, bill_id, seq (1, 2, 3… within the Bill), customer_entry_id (nullable), source (`client_menu`, `waiter`, `cashier`), status (`pending`, `claimed`, `confirmed`, `cancelled`; Ready/Served are derived from items), placed_by, claimed_by, claimed_at, confirmed_by, confirmed_at, key_id, restriction_tags_enc (Dietary Restriction ticked by a Waiter on a direct-entry Order), renotified_at, escalated_at |
| `order_item` | id, order_id, sort, product_id, variant_id, category_id, prep_area_id, product_name_id/en, variant_name_id/en, unit_price, prep_time_seconds (snapshots), quantity, note, status, cancel_reason, cancel_note, cancelled_by (NULL with a reason = the Customer), cancelled_at, void_reason, voided_by, voided_override_by, voided_at, remake_of_id (nullable), remake_reason, remake_override_by, preparing_at, ready_at, served_at, served_by, sub_bill_id (nullable, Phase 6), sold_out_flag, unavailable_flag |
| `waiter_call` | id, kind (`call`, `not_mine`), table_id, bill_id, customer_entry_id, created_at, renotified_at, escalated_at, answered_by, answered_at — Call Waiter (BR-CM-07) and "This isn't my table" (BR-CM-12); a Customer has at most one unanswered call |
| `claim_transfer` | id, order_id, from_crew_id, to_crew_id, kind (`takeover`, `reassign`), by_crew_id, at |
| `order_item_modifier` | order_item_id, modifier_id, name_snapshot, price |
| `order_item_dietary_warning` | order_item_id, dietary_tag_id, acknowledged_at — Product-side tag only, no link to the person |
| `sub_bill` | id, bill_id, method (`by_item`, `equal`), share_index, share_count, status (`open`, `closed`), frozen subtotal / item_discount / bill_discount / service / tax / total, created_at, closed_at — Order Items carry `sub_bill_id` when split by item |

Constraints:
- **TR-DM-01** One open Bill per Table: `bill_table` carries a denormalised `is_open` flag kept in step with `bill.status` and a partial unique index `UNIQUE (table_id) WHERE is_open = 1`. The open/join runs inside `BEGIN IMMEDIATE`, which takes SQLite's write lock up front so two scans of the same QR cannot both open a Bill.
- **TR-DM-02** `order_item.unit_price`, modifier prices and Product/Variant/Modifier names are copied at order time; later catalog changes never alter existing Orders (BR-PC-08).
- **TR-DM-05** Dietary Warnings are computed server-side when an Order Item is created: (Product tags ∪ chosen Modifier tags) ∩ applicable Dietary Restriction, where the applicable restriction is the ordering Customer's `restriction_tags_enc` or the Order's `restriction_tags_enc` (decrypted in memory). Each matching tag becomes an `order_item_dietary_warning` row. The Client Menu computes the same match locally for badges, but the server result is authoritative: a Customer Order line whose warnings are not all in its `acknowledgedTagIds` is rejected with `dietary_ack_required` naming the tags (BR-CM-11), and nothing in that Order is saved. Every failing line of an Order is reported at once (`409 order_rejected` with per-line codes `unavailable`, `sold_out`, `modifier_rule`, …).
- **TR-DM-03** Merge re-points `bill_table` and `orders` rows to the surviving Bill and closes the absorbed Bill as `merged`; there is no unmerge operation.
- **TR-DM-06** Move fails with `409 table_occupied` if the target Table has an open Bill (BR-R-28). Split fails with `409 bill_has_payments` if any Payment exists (BR-R-29).
- **TR-DM-07** A Remake is a new `order_item` with `remake_of_id` set, `unit_price = 0`, status `confirmed`, sorted first in its Prep Area queue (BR-R-33).
- **TR-DM-08** Closing a Bill with items not yet Ready requires `pay_now_serve_later_by` (the confirming staff member); those items keep flowing to the Kitchen after close (BR-R-31).
- **TR-DM-11** Settling stamps `bill.priced_at`; while it is set the Bill is priced with the discounts active at that moment, so a promo window ending mid-payment cannot change what is owed. Unlock clears it. On close (or Unpaid) the totals are frozen on the Bill (`subtotal`, `item_discount`, `bill_discount`, `service`, `tax`, `total`) and on each closed Sub-bill; reports read the frozen figures. A manual Bill discount is stored as `bill.manual_discount_id` with its approver.
- **TR-PAY-05** A Payment credits `amount − rounding` to the Bill. Cash is rounded to Rp 100 only when it settles the remaining balance (the API returns that rounded figure as `cashDue`); a smaller cash amount is a partial Payment without rounding. QRIS and card cannot exceed the balance.

### 4.4 Pricing and payment

| Table | Key columns |
|---|---|
| `pricing_setting` | single row: service_rate_bp, tax_rate_bp (basis points), price_mode (`exclusive`, `inclusive`), refund_cap |
| `discount` | id, name, kind (`percent` value in basis points, `nominal` value in Rupiah), value, scope (`item`, `bill`), trigger (`auto`, `manual`), days_mask (bit 0 = Monday), start_minute, end_minute (WIB minutes; start > end crosses midnight), valid_from, valid_to, min_subtotal (Bill discounts), active |
| `discount_target` | discount_id, target_type (`category`, `product`, `variant`), target_id — none means every Product |
| `applied_discount` | id, order_item_id or bill_id or sub_bill_id, discount_id, amount, override_by |
| `payment` | id, bill_id, sub_bill_id, method (`cash`, `qris`, `card`), status (`settled`, `held_by_waiter`, `pending_verification`, `written_off`), amount (credited to the Bill; for cash, after rounding), tendered, change_given, rounding (rounded − exact, cash only), taken_by, taken_at, cash_handover_id, override_by (pending QRIS approver), verified_by, verified_at |
| `refund` | id, bill_id, amount, reason, approved_by, drawer_session_id, created_at — always paid in cash |
| `cash_drawer` | id, label — exactly one row in the MVP |
| `drawer_session` | id, cash_drawer_id, cashier_crew_id, opening_float, opened_at, expected_cash, counted_cash, difference, closed_at — one Shift Closing per session |
| `cash_handover` | id, drawer_session_id, waiter_crew_id, expected_amount, received_amount, difference, received_by, received_at |
| `receipt_print` | id, bill_id, sub_bill_id, copy (`customer`, `internal`), is_reprint, printed_by, printed_at |
| `ops_setting` | claim_renotify_seconds (120), claim_escalate_seconds (300), pickup_reminder_seconds (180), empty_bill_close_seconds (1800), auto_clock_out_grace_seconds (7200) |

- **TR-PAY-01** Expected cash for a `drawer_session` = opening_float + Σ received Cash Handovers + Σ counter cash Payments − Σ change − Σ cash Refunds; rounding is already inside Payment amounts.
- **TR-PAY-02** A Waiter's cash Payment is `held_by_waiter`; receiving a Cash Handover sets the included Payments to `settled` and links them. Waiter clock-out is refused while any `held_by_waiter` Payment exists (BR-HR-07).
- **TR-PAY-03** `pending_verification` QRIS Payments require an Outlet Admin override token and block closing their `drawer_session` until verified or `written_off` with reason.
- **TR-PAY-04** Takeaway Orders are created `confirmed` only after their Bill is fully paid; until then they are held as a draft on the Bill (BR-CS-02).

### 4.5 Display, HRIS, audit

| Table | Key columns |
|---|---|
| `display_slide` | id, template (`hero_promo`, `featured_product`, `happy_hour`, `category_spotlight`, `brand_idle`, `uploaded`), sort, active, headline_id/en, subline_id/en, image_key, video_key (`uploaded` only; image_key is then the poster), image_focus_x/y, product_id, discount_id, category_ids (JSON array), steam, duration_seconds (4–60, default 8), active_from, active_to |
| `shift` | id, crew_id, starts_at, ends_at |
| `attendance` | id, crew_id, clock_in_at, clock_out_at, auto_clocked_out, corrected_by, correction_reason |
| `leave_request` | id, crew_id, from_date, to_date, type, reason, status, decided_by, decided_at |
| `audit_log` | id, at, actor_crew_id, override_crew_id, action, entity, entity_id, before (JSON text), after (JSON text), reason |

- **TR-DM-04** `audit_log` is append-only: `BEFORE UPDATE` and `BEFORE DELETE` triggers raise `ABORT`. The only exception is the demo reset, which replaces the whole database file.
- **TR-DM-09** Images are content-addressed: `image_key` is `img/<sha256 prefix>` of the uploaded bytes, stored under `data/images/<image_key>/` as `original.<ext>`, `sq320.webp`, `sq640.webp`, `sq1024.webp` (centre crops, 1:1) and `wide-<x>-<y>.webp` (1920×1080 crop at the focal point, created when a Product is saved). Files never change in place, so `/media/images/img/*` is served `immutable` for a year; a new focal point produces a new file name. Re-uploading identical bytes reuses the key, which also makes repeated demo seeds fast. A Product without an image renders the branded placeholder (logo badge on Rice). Product Display videos (MP4/WebM, ≤ 40 MB) are stored as uploaded, without transcoding, at `vid/<sha256 prefix>.<ext>` under the same directory and served by the same immutable route.
- **TR-DM-10** The Product Display playlist is resolved on the server (`display.Playlist`): inactive slides and slides outside `active_from`/`active_to` are dropped; a Featured Product slide is skipped while its Product is not Orderable (Hidden, Archived, or every Variant Sold Out); a Happy Hour slide is skipped unless its discount is active and automatic, and carries the current or next window (`startsAt`, `endsAt`, `activeNow`) plus the discounted price from `pricing`; a Category Spotlight shows up to 6 Active Products with photos and is skipped when none remain (BR-PD-03). Every slide change, and every `menu` event, makes the screen refetch; the screen swaps in the new playlist only at the next transition, never mid-slide.

## 5. State machines

### 5.1 Order Item

| From | To | Actor | Guard |
|---|---|---|---|
| — | Pending | Customer (Client Menu) | Product Active; Variant not Sold Out; Dietary Warnings acknowledged |
| — | Confirmed | Waiter/Cashier direct entry | Product Active; Variant not Sold Out |
| Pending | Confirmed | Claiming Waiter | Order Claimed by this Waiter |
| Confirmed | Preparing | Kitchen | — |
| Preparing | Ready | Kitchen | — |
| Ready | Served | Waiter | — |
| Pending | Cancelled | Customer (own Order) | Order not yet Claimed |
| Pending, Confirmed | Cancelled | Waiter/Cashier | Cancel Reason required |
| Confirmed, Preparing | Cancelled | Kitchen | Cancel Reason required |
| Ready, Served | Voided | Any staff + PIN Override (Outlet Admin/Management) | Item's Bill or Sub-bill unpaid; reason required |
| Served | (new item, Confirmed) | Waiter + PIN Override (Outlet Admin/Management) | Remake reason required |

- **TR-SM-01** Transitions are enforced server-side in one function per transition; invalid transitions return `409 Conflict`.
- **TR-SM-02** Status writes use optimistic concurrency (`WHERE status = ?` bound to the expected status) so two devices cannot double-advance or double-Claim.
- **TR-SM-03** Claim is first-writer-wins: `UPDATE orders SET claimed_by = ? WHERE id = ? AND claimed_by IS NULL`. The losing Waiter receives `409` and the notification is withdrawn from all devices.
- **TR-SM-04** A Cancel with reason `out_of_stock` sets `variant.sold_out = true` in the same transaction, then flags (`sold_out_flag = true`) all Pending/Confirmed Order Items with that Variant and notifies their owning Waiter or all on-duty Waiters if unclaimed (BR-R-11, BR-R-13).

- **TR-SM-05** Claim takeover (`POST /orders/{id}/takeover`) by another Waiter and reassignment by the Outlet Admin write `claim_transfer` and the audit log, and notify both Waiters.
- **TR-SM-06** Hiding or archiving a Product sets `unavailable_flag` on its open Pending/Confirmed Order Items and notifies Waiters, exactly like TR-SM-04 (BR-R-35).

### 5.2 Bill

| From | To | Trigger |
|---|---|---|
| — | `open` | First QR scan or first direct Order |
| `open` | `settling` | First Payment started or Bill summary requested for payment |
| `settling` | `open` | Waiter/Cashier unlocks |
| `open`, `settling` | `closed` | Fully paid (with `pay_now_serve_later_by` if items unfinished) |
| `open`, `settling` | `unpaid` | Outlet Admin override + reason |
| `open` | `merged` | Absorbed by Merge |
| `open` | `closed` | No Orders and older than `empty_bill_close_seconds` (job) |

A `closed` or `unpaid` Bill is immutable except for Refunds and reprints. `settling` blocks `POST /public/orders` with `409 bill_settling`.

## 6. API

REST, JSON, under `/api/v1`, described in an OpenAPI 3 spec from which the TypeScript client is generated.

### 6.1 Representative endpoints

| Area | Endpoint |
|---|---|
| Auth | `POST /auth/login` (crew_id, pin) · `POST /auth/logout` · `POST /auth/override` (pin, action) → short-lived override token |
| Client Menu | `GET /public/tables/{qr_token}` → `free`, `occupied` with Customer names, or `unavailable` · `POST /public/tables/{qr_token}/join` (name, Dietary Restriction tag ids, Dietary Note, consent) → guest token for a new Customer; opens the Bill if the Table has none · `POST /public/tables/{qr_token}/rejoin` (customerId) → guest token, previous device revoked · `POST /public/tables/{qr_token}/not-mine` → notify Waiters · `GET /public/catalog` (Active Products, Sold Out Variants, current promo prices, price display settings; ETag) · `GET /public/bills/me` (Bill, own profile, own Orders with derived status and Estimated Wait) · `POST /public/cart/quote` (prices a cart and reports warnings and unavailable lines without saving) · `POST /public/orders` · `PATCH`/`DELETE /public/orders/{id}` (before Claim; lines left out of a PATCH are Cancelled with Customer Request) · `POST /public/waiter-calls` · Demo mode: `GET /demo/tables` (launcher) |
| Catalog | `GET /catalog` (staff; `?archived=true` includes Archived) · `POST /catalog/products`, `PUT /catalog/products/{id}` — the whole Product document (details, Dietary Tags, Variants, Modifier groups) in one transaction; children left out are archived · `POST`/`PUT /catalog/categories`, `/catalog/dietary-tags` · `PUT /catalog/prep-areas/{id}/capacity` · `PUT /catalog/variants/{id}/sold-out` · `POST /images` (multipart upload) |
| Admin | `GET /settings`, `PUT /settings/pricing`, `PUT /settings/ops` · `GET`/`POST /discounts`, `PUT /discounts/{id}` · `GET`/`POST /tables`, `PUT /tables/{id}`, `POST /tables/{id}/regenerate-qr` · `GET`/`POST /crew`, `PUT /crew/{id}`, `POST /crew/{id}/reset-pin` · `GET /audit` |
| Ordering | `GET /waiter/feed` (unclaimed Orders and Waiter Calls; the caller's own Orders) · `GET /floor` (TableMap) · `POST /orders` (direct entry, created Confirmed) · `POST /orders/{id}/claim` · `POST /orders/{id}/takeover` · `POST /orders/{id}/reassign` · `POST /orders/{id}/items` (add lines before confirming) · `POST /orders/{id}/confirm` · `POST /order-items/{id}/cancel` · `POST /order-items/serve` · `POST /order-items/{id}/void` · `POST /order-items/{id}/remake` · `POST /waiter-calls/{id}/answer` |
| Bills | `POST /bills` (takeaway) · `GET /bills/{id}` (BillSummary with billing) · `GET /bills/{id}/billing` · `POST /bills/{id}/move` · `POST /bills/merge` · `POST /bills/{id}/split` · `POST /bills/{id}/settle` · `POST /bills/{id}/unlock` · `POST /bills/{id}/discount` (manual, Management PIN) · `POST /bills/{id}/unpaid` · `POST /post-outage/bills` |
| Payment | `POST /bills/{id}/payments` · `POST /payments/{id}/verify` · `POST /bills/{id}/close` (Bill or Sub-bill; `payNowServeLater`) · `POST /bills/{id}/refunds` · `POST /bills/{id}/receipts` (one copy; the first print of a copy is the original, every later one a REPRINT) |
| Cash | `POST /drawer-sessions` (opening float) · `POST /cash-handovers` · `GET /crew/{id}/held-cash` · `POST /drawer-sessions/{id}/close` (Shift Closing) |
| Kitchen | `GET /prep-areas/{id}/queue` (id or code: `kitchen`, `bar`) · `POST /order-items/start` · `POST /order-items/ready` · `POST /order-items/{id}/kitchen-cancel` (Confirmed or Preparing) — Kitchen Crew or a Kitchen/Bar screen session |
| Display | `GET /public/display/playlist` (resolved, TR-DM-10) · `GET /display/slides` (Outlet Admin, Management) · `POST /display/slides`, `PUT`/`DELETE /display/slides/{id}`, `PUT /display/slide-order` (Management) · `POST /videos` (multipart, Management) |
| HRIS | `/employees`, `/shifts`, `POST /attendance/clock` (crew_id, pin; clock-in screen session only), `POST /attendance/{id}/correct`, `/leave-requests` |
| Reports | `/reports/daily-sales`, `/reports/payment-methods`, `/reports/best-sellers`, `/reports/prep-time`, `/reports/shift-closings`, `/reports/unpaid-bills`, `/reports/remakes`, `/reports/post-outage` (CSV export on each) |

### 6.2 Conventions

- **TR-API-01** Mutating endpoints accept an `Idempotency-Key` header; repeat requests within 24 h return the original result. This protects against double-taps and retries on flaky tablet Wi-Fi.
- **TR-API-02** Errors use RFC 9457 problem details with a stable `code`.
- **TR-API-03** Restricted actions require an override token from `/auth/override` in an `X-Override-Token` header, valid for one action and 60 seconds.

## 7. Realtime

- **TR-RT-01** One WebSocket per client at `/ws`. Clients subscribe to topics after authenticating. A connection without a session may subscribe only to the public topics `menu` and `display`, which carry no customer or staff data.
- **TR-RT-02** Topics: `waiters` (new Orders, waiter calls, claims, takeovers, Sold Out and unavailable flags, Ready items, re-notifications, "not my table" alerts), `outlet-admin` (escalations), `prep-area:{id}`, `bill:{id}` (Customer view and staff Bill screens, Settling state), `customer:{id}` (device revoked on Rejoin), `menu` (Sold Out changes, price updates), `display`.
- **TR-RT-03** Events carry the entity id and new version only; clients refetch through TanStack Query. This keeps one source of truth and survives missed events.
- **TR-RT-04** On reconnect, clients refetch all active queries. Heartbeat every 20 s; reconnect with exponential backoff capped at 10 s.
- **TR-RT-05** End-to-end latency from commit to screen update ≤ 1 s at p95 under peak load.

## 8. Authentication and authorisation

- **TR-AUTH-01** Sign-in is Crew ID + 6-digit PIN for every role (BR-R-24). PINs are hashed with Argon2id.
- **TR-AUTH-02** After 5 consecutive failed attempts, the account is locked for 15 minutes. Failed attempts are also rate-limited per client IP.
- **TR-AUTH-03** Sessions are opaque random tokens in `HttpOnly`, `Secure`, `SameSite=Strict` cookies, stored server-side.
- **TR-AUTH-04** Management sessions expire after 30 minutes of inactivity (BR-R-25).
- **TR-AUTH-05** Authorisation is checked server-side per endpoint against role and Stations, following the BRD §5.1 permission matrix. The UI hides what a user cannot do but is never the enforcement point.
- **TR-AUTH-06** PIN Override: the authorising person enters their PIN on the requesting device; the server verifies their role permits the action (manual discount → Management; Void, Remake, Unpaid Bill, unverified QRIS, Refund ≤ cap → Outlet Admin or Management; Refund > cap → Management) and records both actor and approver in `audit_log`.
- **TR-AUTH-07** Customers are bound to the Bill, not the device (BR-CM-13). Guest tokens travel in the `X-Guest-Token` header (and `?guest=` on `/ws`), not in a cookie, so each browser tab can be a different Customer; the Client Menu keeps the token in `sessionStorage`, with a per-Table copy in `localStorage` so a phone resumes after closing its tab. `join` creates a `customer_entry` and issues a guest token for it; `rejoin` issues a new token for an existing `customer_entry`, stores the new device in `active_device_id`, revokes the previous token and pushes `customer:{id}` so the old device shows "You continued on another device". Tokens expire when the Bill closes. No verification is required to Rejoin (owner decision, BRD §11). Regenerating a Table's `qr_token` invalidates printed QR codes for that Table.
- **TR-AUTH-08** The Kitchen and shared HRIS clock-in screens are signed in once per day by an Outlet Admin (Crew ID + PIN) into a screen-mode session limited to that screen, so no one types a PIN on a wall-mounted display. `POST /attendance/clock` is accepted only from an HRIS screen-mode session (BR-HR-03). The Product Display needs no session: it reads only public content (`/public/display/playlist`, the `menu` and `display` topics), so a TV that reboots overnight comes back without anyone signing it in. A `display` screen mode still exists for a future private screen.
- **TR-AUTH-09** Waiter and Cashier sessions end at the scheduled shift end or after 12 hours, whichever comes first; switching user on a shared device is a PIN re-entry (BR-R-38).

## 9. Pricing engine

`internal/pricing` is a pure package with no I/O, fully unit-tested.

- **TR-PR-01** Inputs: Order Items (unit price, modifiers, qty, status), applicable discounts at the time of calculation, tax setting. Output: line totals, item discounts, Bill discount, service, tax, grand total.
- **TR-PR-02** Order of operations follows BR-R-14. Percentages are applied in basis points with integer arithmetic; each step rounds half-up to whole Rupiah.
- **TR-PR-03** Best item-level discount selection: compute every eligible automatic discount for the item and pick the largest Rupiah amount; a manual item discount replaces the automatic one only if the Management override is present.
- **TR-PR-04** Split equally: compute the full Bill total, divide by N; the remainder Rupiah goes to the last Sub-bill. Split by item: each Sub-bill is priced independently from its assigned items, with the Bill-level discount prorated by subtotal.
- **TR-PR-05** Cash rounding to nearest Rp 100 is applied only at cash Payment and stored in `payment.rounding`; Bill totals stay exact.
- **TR-PR-06** Tax-inclusive mode (future, BR-ADM-04) back-calculates the net price; the engine supports it behind `price_mode` from the start.

## 10. Receipt printing

See [ADR-0004](adr/0004-browser-bluetooth-printing.md).

- **TR-PRN-01** The Staff PWA prints directly to a paired 58 mm thermal printer via the **Web Bluetooth API**, sending ESC/POS byte streams generated in `web/packages/printing`.
- **TR-PRN-02** Printers must expose a **Bluetooth Low Energy (BLE)** GATT write characteristic; Web Bluetooth cannot reach Bluetooth Classic (SPP)-only printers. Printer models are validated in a hardware spike before purchase.
- **TR-PRN-03** On Windows PCs, a fallback transport via the **Web Serial API** (printer paired as a Bluetooth COM port) is available for Classic-only printers.
- **TR-PRN-04** Receipt data is fetched before printing and held on the device, so a print or reprint succeeds even if internet drops after the Bill closes (BR-NF-05).
- **TR-PRN-05** Layout: 32 characters per line (58 mm, Font A). Customer copy: outlet header, Bill number, Table(s), date/time, Customer name(s), items with Variants/Modifiers, discounts, service, tax, total, Payments, change. Internal copy: same without Customer names, marked "INTERNAL".
- **TR-PRN-06** Writes are chunked to the characteristic's maximum size with flow control; failures surface a retry button and never lose the Bill state.
- **TR-PRN-07** Supported browsers: Chrome on Android and Windows. The Staff app detects missing Web Bluetooth and shows a clear unsupported-device message.
- **TR-PRN-08** Every print writes a `receipt_print` row; reprints print "REPRINT" under the header and are audit-logged (BR-R-37).

## 11. Kitchen queue and Estimated Wait

- **TR-KT-01** Each Prep Area queue lists Confirmed and Preparing Order Items with Remakes first, then by `confirmed_at`, showing elapsed time since confirmation against `prep_time_seconds`; items turn amber at 80% and red at 100%. Ready items not Served within `pickup_reminder_seconds` show "waiting pickup".
- **TR-KT-02** Each Prep Area has a configurable parallel capacity `c` (default Kitchen 2, Bar 1).
- **TR-KT-03** Estimated Wait for an Order = max over its Prep Areas of: (Σ remaining prep time of items ahead in that Prep Area ÷ c) + the longest Prep Time among the Order's items in that Prep Area. Remaining time for a Preparing item is `max(prep_time − elapsed, 0)`.
- **TR-KT-04** Estimated Wait is recalculated on every queue change and pushed on `bill:{id}`; it is shown rounded up to the next minute.
- **TR-KT-05** Prep Time performance report: share of Order Items with (Ready − Confirmed) ≤ Prep Time, by item, Category and Prep Area.

## 12. Customer data retention

See [ADR-0002](adr/0002-two-day-customer-data-retention.md).

- **TR-RET-01** A job runs every hour and Anonymises every Bill closed (or `unpaid`) more than 48 hours ago: `customer_entry.name_enc` → "Customer", `dietary_note_enc` and `restriction_tags_enc` → NULL on `customer_entry` and `orders`, `bill.takeaway_name` → "Customer", `bill.anonymised_at` set. `order_item_dietary_warning` rows are kept; they hold only the Product-side tag.
- **TR-RET-02** Application logs never contain Customer names or Dietary Notes; request logging redacts these fields.
- **TR-RET-03** Dietary Note consent timestamp is stored with the entry; no entry is saved without it.
- **TR-RET-04** Crypto-shredding (BR-P-06): `customer_entry.name`, `dietary_note`, `bill.takeaway_name` and the Dietary Restriction tag list are encrypted (AES-256-GCM) with a per-day data key identified by `key_id` on the row. Data keys live in a separate key file (`data/keys.json`) outside the database file and its backups, wrapped by a master key held in the environment (`SL_MASTER_KEY`, base64 32 bytes; the prototype falls back to a generated `data/master.key` and logs a warning). A data key's id is its WIB date; every encrypted field of one row uses the row's `key_id`. The demo seed builds a fresh key file next to the fresh database and installs both together, so a reset also rotates the keys. A daily job destroys data keys older than 3 days (48 h after the last Bill that could use them closes), so any backup older than that holds unreadable ciphertext. This is why Dietary Restrictions are stored as encrypted tag-id lists rather than join tables.

## 13. Audit log

- **TR-AUD-01** Logged actions: Void, Remake, Refund, Unpaid Bill, unverified QRIS and its resolution, Claim takeover and reassignment, Receipt reprint, Cash Handover differences, attendance corrections, Post-outage Entry, PIN Override, manual discount, Sold Out set and re-enable, Product Status changes, Product and Modifier Dietary Tag changes, price and tax changes, discount changes, Merge, Split, Bill move, Shift Closing, PIN reset, role changes.
- **TR-AUD-02** Each entry records actor, approver (if override), timestamp, entity, before/after snapshot and reason.
- **TR-AUD-03** Audit entries are retained 10 years with the sales records.

## 14. Internationalisation

- **TR-I18N-01** Menu content stores Indonesian and English fields; Client Menu language follows the phone language with a toggle, defaulting to Indonesian.
- **TR-I18N-02** Staff app strings are in Indonesian, kept in i18next resource files so English can be added without code changes.
- **TR-I18N-03** Currency formatted `Rp 25.000` (id-ID locale), no decimals.

## 15. Non-functional requirements

| ID | Requirement |
|---|---|
| TR-NF-01 | API p95 ≤ 300 ms for reads and ≤ 500 ms for writes at 150 Orders/hour plus 30 concurrent Client Menu sessions and 15 staff devices. |
| TR-NF-02 | Client Menu first load ≤ 2.5 s LCP on a mid-range Android phone over 4G; menu photos served as responsive WebP. |
| TR-NF-03 | Availability 99.5% during opening hours; deployments happen outside opening hours. For the demo, the host must stay awake and on the same network as the devices. |
| TR-NF-04 | Database backups daily with `VACUUM INTO` a timestamped file, retained 30 days; restore = replace the file and restart. RPO 24 h, RTO 15 min. In Demo mode backups are off (the demo resets nightly). |
| TR-NF-05 | TLS everywhere; HSTS; strict Content Security Policy; secrets via environment, never in the repo. |
| TR-NF-06 | Observability: structured JSON logs, request metrics, uptime check on `/healthz`, alert on WebSocket error rate and job failures. |
| TR-NF-07 | Staff app is installable as a PWA with a cached shell; when offline it shows a clear "no connection — use paper" banner and allows reprinting held Receipts only. |

## 16. Deployment

- **TR-DEP-01** The demo host (Windows, macOS or Linux) runs two native processes started by one script: Caddy (HTTPS, static files, reverse proxy) and the Go API (with `DEMO_MODE=true`). Data lives in `data/`.
- **TR-DEP-02** Environments: `dev` (Vite dev servers + API, fresh seed), `demo` (built apps, Demo mode, nightly reset). A future production deployment would follow ADR-0005's PostgreSQL path.
- **TR-DEP-03** CI builds the Go binary and the three web apps and runs all tests; a release is a zip with the binaries, built apps, Caddyfile and start script.
- **TR-DEP-04** Migrations run automatically on API start, are forward-only, and must be backward compatible with the previous release.

## 17. Testing

- **TR-TST-01** Unit tests for `pricing`, Estimated Wait, Dietary Warning matching and state-transition guards, table-driven, covering every BRD pricing and dietary rule.
- **TR-TST-02** Integration tests against real SQLite databases in temporary files (WAL, foreign keys on, same migrations) for concurrent writers (`BEGIN IMMEDIATE` contention), Claim races and takeovers, one-open-Bill-per-Table, Rejoin device revocation, Settling lock, Move onto occupied Table, Split after Payment, Merge, Sold Out on Out of Stock Cancel, Product Status visibility, Dietary Warnings carried onto Order Items, Remake, Unpaid Bill, Cash Handover and drawer expected cash, pending QRIS blocking Shift Closing, auto clock-out, Anonymisation and key destruction.
- **TR-TST-03** Frontend component tests with Vitest; end-to-end flows with Playwright: QR scan → Order → Claim → Kitchen → Ready → Payment → Receipt.
- **TR-TST-04** Printer spike with the shortlisted 58 mm printer models on an Android tablet and a Windows PC before procurement.
- **TR-TST-05** Load test at 2× peak (300 Orders/hour) before the Waiter module goes live.

## 18. Future architecture

The local outlet server model, which would keep all staff Modules working during internet outages, is recorded in [ADR-0003](adr/0003-online-only-with-future-local-server.md). The current design keeps that door open by: keeping the Go API as one binary with a file database that already runs on a mini-PC (ADR-0005), using no managed cloud-specific services, and using idempotent mutations that can later be replayed by a sync agent.

## 19. Scheduled jobs

| Job | Interval | Action |
|---|---|---|
| Claim escalation | 15 s | Re-notify `waiters` for Orders/calls unclaimed past `claim_renotify_seconds`; alert `outlet-admin` past `claim_escalate_seconds` (BR-WT-10) |
| Pickup reminder | 15 s | Re-notify owning Waiter for Ready items past `pickup_reminder_seconds` (BR-WT-09) |
| Empty Bill close | 1 min | Close open Bills with no Orders older than `empty_bill_close_seconds` (BR-R-26) |
| Promo activation | 1 min | Discounts are evaluated at pricing time, so nothing is written; the job publishes `menu` whenever the set of active automatic discounts changes, so menus and the Product Display refresh their promo prices |
| Auto clock-out | 5 min | Clock out attendance open past shift end + `auto_clock_out_grace_seconds`, flag `auto_clocked_out` (BR-HR-06) |
| Anonymisation | 1 h | TR-RET-01 |
| Key destruction | 04:30 WIB daily | TR-RET-04 |
| Backup | 1 day | TR-NF-04 (off in Demo mode) |
| Demo reset | 03:00 WIB daily | Demo mode only; see [DEMO-AND-SEED.md](DEMO-AND-SEED.md) |

All jobs are idempotent and run on the in-process scheduler (the API is the only process, so no distributed lock is needed); failures are logged and shown on the Admin Panel health card.

## 20. Open items

None. TRD open items and the gaps from [OVERVIEW-AND-FLOWS.md](OVERVIEW-AND-FLOWS.md) were resolved on 2026-10-26.
