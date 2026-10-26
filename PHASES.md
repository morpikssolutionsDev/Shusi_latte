# Build phases — Sushi Latte

Working branch: `work/sushi-latte`
Source of truth: `docs/BRD.md`, `docs/TRD.md`, `GLOSSARY.md`.
Rule: when code and spec disagree, ask the owner, record the answer in the
BRD/TRD, and add an ADR for hard-to-reverse choices. Tests and PR
descriptions cite the `BR-*` / `TR-*` IDs they implement.

## Phase order (BRD §4.1)

1. **Admin Panel** — BR-ADM-01 to BR-ADM-13
   - Product catalog: Categories, Products, Variants, Modifiers, Dietary Tags,
     photos, prices, Prep Time, Product Status, Prep Areas
   - Tax (PB1, default 10%) and service charge, price display mode
   - Discounts: percentage or nominal, item-level or Bill-level, automatic
     promos with schedule windows, manual discounts
   - Refund cap per Bill, Sold Out toggle and re-enable audit, Tables and QR codes
   - Crew records, roles, Stations, Crew IDs and PIN resets
   - Product Display content management, reports, audit log view,
     escalation timers
   - Gate: catalog and config can be created, read, updated and audited;
     no ordering, payment or HRIS logic is claimed in this phase.
2. **Client Menu** — QR per Table, browse and order, dietary warnings, SLA view
3. **Product Display** — TV slideshow of banners and featured Products
4. **Waiter** — Claim and enter Orders, manage Bills, take Payment, print Receipts
5. **Kitchen** — Per-Prep-Area queue, Prep Time tracking, SLA
6. **Cashier** — Takeaway Bills, Payment, Shift Closing
7. **HRIS** — Employee records, shifts, attendance, leave

## Phase 1 status: complete (API)

Phase 1 is complete on the API side, covering BR-ADM-01 to BR-ADM-13:
catalog, pricing and tax, Tables and QR codes, crew records, Product
Display content, reports, auth with role enforcement, ops settings,
and the audit log view. The Staff PWA frontend has not been started;
all Phase 1 work so far is the Go API, migrations and docs.

Migrations 0001 to 0006 cover catalog, pricing, Tables, crew and
sessions, display slides and ops settings. Audit rows now record the
authenticated Crew ID as actor, with the actor's full name resolved
in the audit view. Reports query bill, payment, order_item and
drawer_session tables; those tables are now created by the
Waiter, Kitchen and Cashier phases (migrations 0007 to 0010),
so the reports read live data from Phase 6 onward.

Gate met: catalog and config can be created, read, updated and
audited; no ordering, payment or HRIS logic is claimed in this phase.

Phase 2 status: complete (API)

Phase 2 is complete on the API side, covering BR-CM-01 to BR-CM-15:
table scan free/occupied, join with name and explicit consent,
single open Bill per Table, rejoin with guest-token revocation,
not-mine and waiter calls, public catalog of Active Products only,
customer Orders with Variants, Modifiers, quantity and notes,
server-side Dietary Warnings with acknowledgement, edit and
cancel of a Customer's own Pending unclaimed Order, settling
blocks new Orders with 409, and the Customer bill view with
pricing totals and Estimated Wait. Customer names and dietary
data are AES-256-GCM encrypted with per-day keys (TR-RET-04).
The Client Menu web app itself has not been built; this phase
is the API it will use. Waiter Claim and confirmation, Kitchen
queue, payment and HRIS are later phases.

Phase 3 status: complete (API)

Phase 3 is complete on the API side, covering BR-PD-01 to BR-PD-03
and TR-AUTH-08. Display content and the public feed already
existed from Phase 1; this phase adds POST /auth/screen-login,
which signs a display, kitchen or hris screen in once per day by
an Outlet Admin or Management PIN, expiring at the end of the
Jakarta day (screen_session table, migration 0009), and
GET /public/display/version, which returns the feed version
(latest slide change) and slide count so the TV app polls it
and refetches the feed only when it changes, satisfying
BR-PD-02 without manual reload. Sold Out featured Products
remain marked in the feed (BR-PD-03). The TV slideshow web app
itself has not been built; this phase is the API and screen
session it will use.

Phase 4 status: complete (API)

Phase 4 is complete on the API side, covering BR-WT-01 to
BR-WT-06, BR-WT-09, BR-WT-11 to BR-WT-13, BR-WT-15 and
TR-SM-01 to TR-SM-05. Claim is first-writer-wins (409 on second
claim), takeover by another Waiter and reassign by Outlet Admin
or Management are audit-logged, confirm sends a Claimed Pending
Order to Confirmed (with optional item adjustment), and staff
direct Orders for a Table are created Confirmed with optional
name and Waiter-ticked restriction stored on the Order. Cancel
item (Pending/Confirmed, reason required, out_of_stock sets
Variant Sold Out and flags other open items), serve item
(Ready to Served), void item (Ready/Served, unpaid Bill, PIN
Override) and remake item (Served, PIN Override, no-charge
Confirmed copy) are included. Bill summary, settle (blocks
Customer Orders with 409), unlock, move, merge and split by
item or equally, plus receipt reprint recording, are included.
Payment recording and Bill closing are Phase 6, shared by
Waiter and Cashier (BR-CS-01).

Phase 5 status: complete (API)

Phase 5 is complete on the API side, covering BR-KT-01 to
BR-KT-06 and TR-KT-01 to TR-KT-05. GET /api/v1/prep-areas/{id}/queue
returns Confirmed and Preparing items for that Prep Area,
Remakes first then by confirmation time, with elapsed time
against Prep Time (amber at 80%, red at 100%), Dietary Warnings,
item notes, Table labels and PlacedBy, plus Ready items waiting
pickup past the pickup reminder time. Transition moves
Confirmed to Preparing to Ready (ready_at recorded),
kitchen-cancel handles Confirmed/Preparing with reason and
out-of-stock Sold Out flagging, and GET /api/v1/orders/{id}/estimated-wait
implements TR-KT-03 (items ahead / parallel capacity + longest
own Prep Time, rounded up to minutes). Serving remains a
Waiter action from Phase 4. The Kitchen display web app itself
has not been built; this phase is the API it will use.

Phase 6 status: complete (API)

Phase 6 is complete on the API side, covering BR-CS-01 to
BR-CS-08, BR-R-05, BR-R-29, BR-R-30, BR-R-31, BR-R-32 and
TR-PR-05. Migration 0010 adds payment, refund, cash_drawer,
drawer_session, cash_handover and applied_discount. Payments:
cash (rounded to nearest Rp 100), QRIS (starts pending, verify
or write off with reason) and card, with pay-now-serve-later
gate for unfinished items, settling on first payment, and
close only when fully paid with no pending QRIS. Refunds are
cash from the Drawer with PIN Override and cap check (over-cap
needs Management). Drawer sessions: one open at a time,
opening float, expected cash = float + confirmed handovers +
cash taken - refunds, counted cash and difference recorded at
Shift Closing. Cash handovers are recorded as Held by Waiter
and confirmed with difference. Takeaway Bills: open with
customer name, Orders created Pending, confirmed to Kitchen
only after full payment. Post-outage Bills are created closed
with source post_outage, bypassing the Kitchen. Manual
discounts require a Management override and are recorded in
applied_discount, reducing BillPaymentSummary totals. The
Cashier web app itself has not been built; this phase is the
API it will use.

Phase 7 status: complete (API)

Phase 7 is complete on the API side, the last phase, covering
BR-HR-01 to BR-HR-07 and TRD §4.5. Migration 0011 adds
employee (extending Phase 1 crew records), shift, attendance
and leave_request. Employees are Management-only for
create/update. Shifts are Outlet Admin/Management, with an
overlap check per crew. Attendance clock-in/out is only from
an HRIS screen-mode session (X-Screen-Token for screen hris)
plus Crew ID and PIN, so personal devices cannot clock in;
approved leave today blocks clock-in, and clock-out is
blocked while a Waiter holds cash, with the message Hand over
Rp X to the Cashier first. Attendance correction is Outlet
Admin/Management with a reason, audit-logged. Auto clock-out
closes open attendance past shift end + ops grace (default
7200 seconds), flagged auto, via an endpoint and a 5-minute
server ticker. Leave can be requested and approved or
rejected by Outlet Admin/Management. Attendance export is
CSV for external payroll. The HRIS web app itself has not
been built; this phase is the API it will use. Also fixed in
this phase: re-running migrations on an existing database
no longer fails on ALTER TABLE ADD COLUMN created_by, so a
server restart works.

All 7 phases are now complete on the API side, and the
frontends are now built too (commit 72c5e346).

Frontends status: complete (first pass)

Three React + Vite + TypeScript apps under web/apps/,
served by the Go API server at /client-menu/, /staff/ and
/display/ (build each app with npm run build first; the
server serves dist if present, otherwise a
build-instructions page). Client Menu: table scan, join
with name/restriction/consent, catalog with variants and
modifiers, cart, dietary-warning acknowledgement, bill
view, waiter call, rejoin and not-mine. Staff: sign-in
(Crew ID + PIN), Waiter, Kitchen, Cashier, Admin, HRIS and
HRIS Clock screens. Display: TV slideshow polling
/public/display/version every 10 seconds, refetching only
on change, looping by duration_seconds, marking Sold Out
featured Products. Verified: tsc + vite build for all
three apps, Go vet/build, and live server serving all
three index pages plus a JS asset (200). Deviations are
documented in web/README.md: plain CSS instead of
Tailwind/shadcn, plain fetch instead of TanStack, no PWA
manifest or Motion, receipts via browser print not Web
Bluetooth, and QR codes still point to the API scan
endpoint. Realtime WebSocket push has not been built.
Nothing is merged unless the owner asks for a merge.



