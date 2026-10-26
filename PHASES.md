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
drawer_session tables that are created by the Waiter, Kitchen and
Cashier phases, so they return empty rows until then; their query
logic is tested against TRD-schema sample data.

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

Next phase: Product Display (Phase 3). Each phase ends with a PR for
review; nothing is merged unless the owner asks for a merge.
