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

## Phase 1 starting point

This commit is a scaffold only, not a completed Admin Panel:
- `api/go.mod` — Go module for the single API binary (TR-ARC-01)
- `api/cmd/server/main.go` — stdlib HTTP server with a health endpoint only

Next step in Phase 1: catalog data model and migrations, then catalog
configuration endpoints, following TRD §4 and the Admin Panel requirements
above. Each phase ends with a PR for review; nothing is merged unless the
owner asks for a merge.
