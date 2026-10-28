# Sushi Latte — Business Requirements Document (BRD)

| | |
|---|---|
| Product | Sushi Latte |
| Document | Business Requirements Document |
| Version | 1.1 (draft) — gap resolutions 2026-10-26 |
| Date | 2026-10-26 |
| Source | `project goals.md`, design interview, [GLOSSARY.md](../GLOSSARY.md) |
| Companion | [TRD.md](TRD.md), [ADRs](adr/) |

Capitalised terms (Bill, Order, Claim, Void, …) are defined in [GLOSSARY.md](../GLOSSARY.md) and carry exactly that meaning here.

---

## 1. Background

Sushi Latte is a web platform for a single HORECA (hotel, restaurant, café) Business operating one outlet. Today, orders are taken on paper notes, which are slow, easy to lose, hard to read in the kitchen, and impossible to report on. The platform replaces paper with a set of Modules covering the customer menu, front-of-house service, kitchen, payment, management configuration and staff administration.

## 2. Problem statement

1. Paper order notes cause lost, misread and duplicated orders.
2. Customers wait for a Waiter before they can even see or place an order.
3. The kitchen has no shared view of the queue and no measure of lateness.
4. Payments, voids and refunds are not traceable to a person.
5. Management has no reliable sales, best-seller or cash reconciliation data.
6. Attendance, shifts and leave are tracked manually.

## 3. Objectives and success metrics

| # | Objective | Metric | Target |
|---|---|---|---|
| O1 | Shift ordering to self-service | Share of dine-in Orders placed via QR Client Menu | ≥ 80% within 3 months of ordering going live |
| O2 | Fast front-of-house response | Average time from Customer placing an Order to Waiter Claim | < 2 minutes |
| O3 | Predictable kitchen | Order Items Ready within their Prep Time | ≥ 90% |
| O4 | Eliminate paper | Paper order notes in use | Zero |

## 4. Scope

### 4.1 In scope

| Module | Audience | Purpose |
|---|---|---|
| Admin Panel | Management, Outlet Admin | Configure menu, prices, discounts, tax, tables, crew; reports |
| Client Menu | Customer | Browse menu and place Orders by scanning a Table QR |
| Product Display | Customer (in-outlet TV) | Slideshow of banners and featured Products |
| Waiter | Crew (Waiter Station) | Claim and enter Orders, manage Bills, take Payment, print Receipts |
| Kitchen | Crew (Kitchen Station) | Per-Prep-Area order queue with Prep Time tracking |
| Cashier | Crew (Cashier Station) | Takeaway Bills, Payment, Shift Closing |
| HRIS | All roles | Employee records, shifts, attendance, leave |

Modules are delivered in this order: Admin Panel → Product Display → Client Menu → Waiter → Kitchen → Cashier → HRIS (tracked in [todolist.md](../todolist.md)).

### 4.2 Out of scope

- Multiple businesses or multiple outlets ([ADR-0001](adr/0001-single-business-single-outlet.md)).
- Payment gateway or online payment; QRIS is a static code verified by staff.
- Customer accounts, loyalty, or saved preferences.
- Inventory or ingredient stock tracking; bundles/combos.
- Variants or Modifiers that remove Dietary Tags (e.g. oat milk removing Dairy); warnings may over-warn in the MVP.
- Payroll (HRIS exports attendance only).
- Offline operation beyond printing ([ADR-0003](adr/0003-online-only-with-future-local-server.md)).
- Delivery platforms integration.

## 5. Stakeholders and roles

| Role | Description |
|---|---|
| **Management** | Sets the rules: menu, prices, Prep Time, discounts, tax and service charge, Refund cap, staff records; sees all reports. |
| **Outlet Admin** | Runs the day: Sold Out, Tables, shifts, leave approval, Claim reassignment, Voids, Remakes, Unpaid Bills, Refunds up to the cap. |
| **Crew** | Works one or more Stations: **Waiter**, **Cashier**, **Kitchen**. |
| **Customer** | Orders at a Table via the Client Menu; no account. |

### 5.1 Permission matrix

| Capability | Management | Outlet Admin | Waiter | Cashier | Kitchen |
|---|:-:|:-:|:-:|:-:|:-:|
| Product catalog: Products, Variants, Modifiers, prices, Prep Time, Prep Areas, Product Status | ✔ | | | | |
| Dietary Tag list and Product/Modifier Dietary Tags | ✔ | | | | |
| Tax, service charge, discounts, promos, Refund cap | ✔ | | | | |
| Staff records | ✔ | | | | |
| Toggle / re-enable Sold Out | ✔ | ✔ | | | |
| Tables and QR codes | ✔ | ✔ | | | |
| Shift schedules, leave approval | ✔ | ✔ | | | |
| Claim / enter Orders | | | ✔ | ✔ | |
| Take over a Claim from another Waiter | | | ✔ | ✔ | |
| Reassign a Claim | ✔ | ✔ | | | |
| Cancel Order Item (Pending, Confirmed) | | | ✔ | ✔ | |
| Cancel Order Item (Confirmed, Preparing) | | | | | ✔ |
| Advance Order Item status (Preparing, Ready) | | | | | ✔ |
| Move / Merge / Split Bill | | | ✔ | ✔ | |
| Record Payment, close Bill, print and reprint Receipt | | | ✔ | ✔ | |
| Unlock a Settling Bill | | | ✔ | ✔ | |
| Open Takeaway Bill | | | | ✔ | |
| Void, Remake, close as Unpaid, record unverified QRIS (via PIN Override) | ✔ | ✔ | | | |
| Manual discount (via PIN Override) | ✔ | | | | |
| Refund ≤ cap / > cap | ✔ / ✔ | ✔ / | | | |
| Opening Float, receive Cash Handover, Shift Closing | ✔ | ✔ | | ✔ | |
| Post-outage Entry | ✔ | ✔ | | ✔ | |
| Reports | ✔ | | | | |
| Own attendance and leave requests | ✔ | ✔ | ✔ | ✔ | ✔ |

A **PIN Override** lets a Crew member perform a restricted action on their device when an authorised person enters their own PIN.

## 6. Business requirements by module

Requirement IDs are stable references for the TRD and tests. Priority: **M** = must (MVP), **S** = should.

### 6.0 Product catalog (shared baseline)

The Product catalog is the single definition of what the Business sells. Every Module reads from it: the Client Menu and Product Display show it, Waiter and Cashier order from it, Kitchen routes and times by it, and reports aggregate by it. No Module keeps its own copy of Product data.

| ID | Requirement | P |
|---|---|---|
| BR-PC-01 | A Product has: name (ID/EN), description (ID/EN), photo, Category, Prep Time, Dietary Tags, Product Status, and one or more Variants. | M |
| BR-PC-02 | A Variant has: name (ID/EN), price and Sold Out flag. A Product with no options has exactly one default Variant, so price and Sold Out always live on a Variant. | M |
| BR-PC-03 | Product Status is **Active** (on the menu), **Hidden** (off the menu, e.g. seasonal) or **Archived** (retired; kept because past Bills refer to it). Products are never deleted once ordered. | M |
| BR-PC-04 | A Product is shown greyed out and unorderable on every Module when all its Variants are Sold Out; individual Sold Out Variants are greyed out within the Product. | M |
| BR-PC-05 | Modifiers belong to Modifier groups on a Product, with name (ID/EN), price and optional Dietary Tags they add. | M |
| BR-PC-06 | Management maintains one list of Dietary Tags. Initial list: Dairy/Lactose, Gluten, Egg, Fish, Raw Fish, Shellfish, Nuts, Peanut, Soy, Sesame, Pork, Alcohol, Spicy. | M |
| BR-PC-07 | Each Category is assigned to one Prep Area (initially Kitchen and Bar). | M |
| BR-PC-08 | Price and Product changes apply to new Orders only; existing Order Items keep the price and details they were ordered with. | M |

### 6.1 Admin Panel

| ID | Requirement | P |
|---|---|---|
| BR-ADM-01 | Management maintains the Product catalog (BR-PC-01 to BR-PC-07): Categories, Products, Variants, Modifiers, Dietary Tags, photos and prices. | M |
| BR-ADM-02 | Management assigns each Category to a Prep Area. | M |
| BR-ADM-03 | Management sets a Prep Time per Product and sets Product Status. | M |
| BR-ADM-04 | Management configures tax (PB1, default 10%) and service charge rates, and the price display mode (default: excluding tax and service; tax-inclusive available later). | M |
| BR-ADM-05 | Management defines discounts: percentage or nominal; item-level or Bill-level; automatic promos with schedule windows; manual discounts. | M |
| BR-ADM-06 | Management sets the Refund cap per Bill (e.g. Rp 500,000). | M |
| BR-ADM-07 | Outlet Admin and Management toggle Sold Out per Variant and re-enable Sold Out Variants; every re-enable is audit-logged with person and time. | M |
| BR-ADM-08 | Outlet Admin and Management manage Tables and print each Table's QR code. The code is static: it opens `/table-NN` from the Table number and never changes (owner decision 2026-10-28, [ADR-0006](adr/0006-static-table-qr-codes.md)). | M |
| BR-ADM-09 | Management manages crew records, roles, Stations, Crew IDs and PIN resets. | M |
| BR-ADM-10 | Management manages Product Display content (banners, featured Products, ordering, schedule). | M |
| BR-ADM-11 | Reports: daily sales, sales by payment method, best sellers, Prep Time performance, Shift Closing, Unpaid Bills, Remakes by reason, and Post-outage Entries. | M |
| BR-ADM-12 | Audit log view of Voids, Remakes, Refunds, PIN Overrides, Claim takeovers, reprints, Sold Out re-enables and price changes. | M |
| BR-ADM-13 | Management sets the escalation timers: Claim re-notify (default 2 min), Outlet Admin alert (default 5 min), pickup reminder (default 3 min), empty-Bill auto-close (default 30 min). | M |

### 6.2 Client Menu

| ID | Requirement | P |
|---|---|---|
| BR-CM-01 | Scanning a Table QR opens the Client Menu bound to that Table, and opens or joins that Table's single open Bill. | M |
| BR-CM-02 | Before ordering, the Customer enters a full name (required) and may tick a Dietary Restriction from the Dietary Tag list and add a Dietary Note; explicit consent is collected before either is saved. | M |
| BR-CM-03 | The menu is available in Indonesian and English and shows only Active Products. | M |
| BR-CM-04 | Sold Out Variants are shown greyed out and cannot be ordered; a Product whose Variants are all Sold Out is greyed out entirely. | M |
| BR-CM-05 | The Customer builds an Order with Variants, Modifiers, quantity and notes, and submits it; the Order is Pending until Claimed. | M |
| BR-CM-06 | Several Customers at the same Table may each order on the same Bill; each Order records the name of who placed it. | M |
| BR-CM-07 | The Customer can call a Waiter from the menu (notification to all on-duty Waiters). | M |
| BR-CM-08 | The Customer sees the status of their Orders and an Estimated Wait. | M |
| BR-CM-09 | The Customer sees prices excluding tax and service, with a note that both are added. | M |
| BR-CM-10 | Products that contain a Dietary Tag in the Customer's own Dietary Restriction show a Dietary Warning badge naming the tag (e.g. "Contains Dairy"). | M |
| BR-CM-11 | Adding a Product with a Dietary Warning (including tags added by chosen Modifiers) asks the Customer to acknowledge it; the Product can still be ordered. | M |
| BR-CM-12 | Scanning a Table that has an open Bill shows "This table is occupied by [full names]. Is one of these you?" with three choices: **I'm [name]** (Rejoin as that Customer, restoring name, Dietary Restriction and Orders), **I'm joining this table** (new Customer on the same Bill), **This isn't my table** (message to ask staff; Waiters notified). | M |
| BR-CM-13 | A Customer is bound to the Bill, not to a device. Only one device is active per Customer: the latest Rejoin wins, and the previous device shows "You continued on another device". | M |
| BR-CM-14 | A Customer may edit or Cancel their own Order while it is Pending and unclaimed; after Claim they must ask the Waiter. | M |
| BR-CM-15 | While a Bill is Settling, the Client Menu blocks new Orders and shows "Bill is being settled — please ask the waiter". | M |

### 6.3 Product Display

| ID | Requirement | P |
|---|---|---|
| BR-PD-01 | A TV screen shows a looping slideshow of banners and featured Products managed in the Admin Panel. | M |
| BR-PD-02 | Content changes appear on the screen without manual reload. | M |
| BR-PD-03 | Sold Out featured Products are hidden or marked. | S |

### 6.4 Waiter

| ID | Requirement | P |
|---|---|---|
| BR-WT-01 | All on-duty Waiters are notified (sound and vibration) of new Customer Orders and waiter calls; the first to Claim owns the Order. | M |
| BR-WT-02 | The Waiter confirms the Claimed Order with the Customer, may adjust it, then confirms it to send it to the Kitchen. Order Items with Dietary Warnings are highlighted during confirmation. | M |
| BR-WT-03 | The Waiter can enter Orders directly for a Table; Customer name is optional, and the Waiter may tick a Dietary Restriction for that Order, which raises Dietary Warnings the same way. | M |
| BR-WT-04 | The Waiter can Cancel Order Items that are Pending or Confirmed, choosing a Cancel Reason. | M |
| BR-WT-05 | The Waiter sees flags on Order Items whose Variant became Sold Out and must inform the Customer and Cancel them; nothing is cancelled automatically. | M |
| BR-WT-06 | The Waiter can move a Bill to another Table, Merge Bills of several Tables, and split a Bill into Sub-bills by item or equally. | M |
| BR-WT-07 | The Waiter can show the Bill summary, record Payments (cash, QRIS, card), and close the Bill. | M |
| BR-WT-08 | On close, two Receipts are printed on the Waiter's paired Bluetooth printer: a Customer copy and an internal copy. | M |
| BR-WT-09 | Ready Order Items are signalled to the owning Waiter for serving; the Waiter marks them Served. If not picked up within the pickup reminder time, the Waiter is re-notified and the Kitchen display marks the item "waiting pickup". | M |
| BR-WT-10 | Unclaimed Orders and unanswered Waiter calls re-notify all on-duty Waiters after the re-notify time, and alert the Outlet Admin after the escalation time. | M |
| BR-WT-11 | Another Waiter can take over a Claim, and the Outlet Admin can reassign one; both are audit-logged. At clock-out, the Waiter sees their open Claims to hand over. | M |
| BR-WT-12 | The Waiter can Remake a served Order Item that was wrong or bad: an Outlet Admin PIN Override and a reason create a no-charge copy flagged "REMAKE" and prioritised on the Kitchen display; the original stays on the Bill. | M |
| BR-WT-13 | Any Waiter or Cashier may reprint a Receipt; reprints are marked "REPRINT" and audit-logged. | M |
| BR-WT-14 | Cash taken by a Waiter is recorded as **Held by Waiter** until the Cashier confirms the Cash Handover. | M |
| BR-WT-15 | Starting payment puts the Bill into Settling; the Waiter or Cashier can unlock it to accept new Customer Orders. | M |

### 6.5 Kitchen

| ID | Requirement | P |
|---|---|---|
| BR-KT-01 | Each Prep Area has its own large display showing its queue of Confirmed and Preparing Order Items in order of confirmation. | M |
| BR-KT-02 | Each Order Item shows elapsed time against its Prep Time, highlighted when late. | M |
| BR-KT-03 | Kitchen Crew mark Order Items Preparing and Ready with minimal input. | M |
| BR-KT-04 | Kitchen Crew can Cancel Order Items that are Confirmed or Preparing, choosing a Cancel Reason. | M |
| BR-KT-05 | Dietary Warnings (e.g. "⚠ Dairy — customer avoids"), Dietary Notes and item notes are prominently shown on the Order Item. | M |
| BR-KT-06 | Remake Order Items appear at the top of the queue flagged "REMAKE"; Ready items not yet picked up are marked "waiting pickup". | M |

### 6.6 Cashier

| ID | Requirement | P |
|---|---|---|
| BR-CS-01 | The Cashier has all Bill and Payment capabilities of the Waiter module (BR-WT-03 to BR-WT-08). | M |
| BR-CS-02 | The Cashier opens Takeaway Bills with a Customer name for pickup call-out. Takeaway Bills are paid first; the Order goes to the Kitchen only after full Payment. | M |
| BR-CS-03 | Voids and Refunds are performed at the Cashier with a PIN Override. Refunds are paid in cash from the Cash Drawer. | M |
| BR-CS-04 | The outlet has one Cash Drawer at the Cashier. At shift start the Cashier enters the Opening Float. | M |
| BR-CS-05 | The Cashier confirms each Cash Handover from a Waiter by entering the amount received; any difference is recorded against that Waiter. | M |
| BR-CS-06 | Shift Closing is done by the Cashier for the Cash Drawer: expected cash = Opening Float + received Cash Handovers + cash taken at the counter − change given − cash Refunds; counted cash and difference are recorded. A Cashier change mid-day triggers a Shift Closing and a new Opening Float. | M |
| BR-CS-07 | QRIS Payments pending verification (BR-R-30) are listed at Shift Closing and must be verified or written off with a reason. | M |
| BR-CS-08 | After an internet outage, the Cashier or Outlet Admin enters paper Bills as Post-outage Entries with original times, items, Payments and serving Crew; these bypass the Kitchen and are marked "outage" in reports. | M |

### 6.7 HRIS

| ID | Requirement | P |
|---|---|---|
| BR-HR-01 | Management maintains employee records. | M |
| BR-HR-02 | Outlet Admin and Management schedule shifts. | M |
| BR-HR-03 | Crew clock in and out with Crew ID and PIN only on the shared HRIS tablet, opened in clock-in mode by an Outlet Admin; personal devices cannot clock in. | M |
| BR-HR-04 | Crew request leave; Outlet Admin or Management approve or reject. | M |
| BR-HR-05 | Attendance data can be exported for external payroll. | M |
| BR-HR-06 | A Crew member still clocked in 2 hours after their scheduled shift end is clocked out automatically and flagged "auto"; the Outlet Admin corrects it with a reason, audit-logged. | M |
| BR-HR-07 | A Waiter cannot clock out while holding cash not yet handed over; the screen shows "Hand over Rp X to the Cashier first". | M |

## 7. Business rules

### 7.1 Bill lifecycle

- **BR-R-01** A Table has at most one open Bill. It opens when the Table is first used (first QR scan or first Waiter Order) and closes when fully paid.
- **BR-R-02** A Bill holds many Orders; an Order holds many Order Items.
- **BR-R-03** Merge combines the Bills of several Tables into one Bill covering all of them. Merge cannot be undone; use Split instead.
- **BR-R-04** Split produces Sub-bills, by item or equally. Each Sub-bill carries its own discount share, service charge, tax, Payments and two-copy Receipt.
- **BR-R-05** Either Waiter or Cashier may record Payment and close a Bill or Sub-bill.
- **BR-R-26** An open Bill with no Orders closes automatically after the empty-Bill time (default 30 min), freeing the Table.
- **BR-R-27** Starting payment puts the Bill into **Settling**: new Customer Orders are blocked until a Waiter or Cashier unlocks it.
- **BR-R-28** A Bill cannot be moved onto a Table that has an open Bill; staff must Merge instead.
- **BR-R-29** A Bill cannot be split once any Payment has been recorded on it.
- **BR-R-31** A Bill with items still Pending, Confirmed or Preparing can be paid and closed only after staff confirm "pay now, serve later"; the remaining items continue to the Kitchen and are served as normal.
- **BR-R-32** If a Customer leaves without paying, the Outlet Admin closes the Bill as an **Unpaid Bill** with a PIN Override and a reason; it is audit-logged and shown in reports as a loss.

### 7.2 Order Item lifecycle

```
Pending ──Claim+confirm──▶ Confirmed ──▶ Preparing ──▶ Ready ──▶ Served
   │                          │              │           │          │
   └─ Cancel (Customer before Claim, Waiter) │           └── Void ──┘
                 Cancel (Kitchen: Confirmed or Preparing)  (PIN Override, unpaid only)
Served ──Remake (PIN Override)──▶ new no-charge Order Item, Confirmed, prioritised
```

- **BR-R-06** Customer-placed Orders stay Pending until a Waiter Claims and confirms them; they never reach the Kitchen unconfirmed.
- **BR-R-07** **Cancel** applies before Ready: Customer (own Pending Order before Claim), Waiter (Pending, Confirmed) or Kitchen (Confirmed, Preparing). It needs a Cancel Reason and no PIN.
- **BR-R-08** **Void** applies to Ready or Served Order Items on an unpaid Bill or unpaid Sub-bill. Items on a paid Sub-bill go through Refund. Void always needs a PIN Override (Outlet Admin or Management) and a reason, and is audit-logged.
- **BR-R-09** An Order is Ready when all its non-cancelled Order Items are Ready.
- **BR-R-33** **Remake** applies to a Served Order Item that was wrong or bad. It needs an Outlet Admin or Management PIN Override and a reason, creates a no-charge copy at the top of the Kitchen queue, and keeps the original on the Bill.
- **BR-R-34** A Claim may be taken over by another Waiter or reassigned by the Outlet Admin; both are audit-logged.

### 7.3 Sold Out

- **BR-R-10** Cancel Reasons: Out of Stock, Customer Request, Wrong Order, Other (free text).
- **BR-R-11** A Cancel with reason **Out of Stock** marks that Order Item's **Variant** Sold Out immediately, on every Module.
- **BR-R-12** Sold Out Variants cannot be ordered from any Module until Outlet Admin or Management re-enables them; each re-enable is audit-logged.
- **BR-R-13** When a Variant becomes Sold Out, open Order Items with that Variant in Pending or Confirmed are flagged to their Waiter (or to all on-duty Waiters if unclaimed). They are not cancelled automatically.
- **BR-R-35** When a Product is set to Hidden or Archived, its open Pending or Confirmed Order Items are flagged the same way as Sold Out.

### 7.4 Dietary Warnings

- **BR-R-D1** A Dietary Restriction belongs to one Customer (each person who scans and enters a name), or to one Waiter-entered Order. Warnings on a Customer's phone follow only their own Dietary Restriction.
- **BR-R-D2** A Dietary Warning is raised when the Product's Dietary Tags, plus Dietary Tags added by its chosen Modifiers, include any tag in the applicable Dietary Restriction.
- **BR-R-D3** Dietary Warnings never block ordering. They are copied onto the Order Item at order time and shown to the Waiter (at Claim and confirmation) and the Kitchen.
- **BR-R-D4** Variants and Modifiers cannot remove Dietary Tags in the MVP, so warnings may over-warn but never under-warn.

### 7.5 Pricing

- **BR-R-14** Calculation order for a Bill or Sub-bill:
  1. Subtotal = Σ (Variant price + Modifier prices) × quantity, excluding Cancelled and Voided items
  2. − item-level discounts
  3. − Bill-level discount
  4. + service charge on (2–3 result)
  5. + PB1 tax on (result of step 4)
- **BR-R-15** Menu prices are displayed excluding tax and service charge.
- **BR-R-16** At most one item-level discount per Order Item; when several apply, the best one for the Customer applies automatically. At most one Bill-level discount per Bill.
- **BR-R-17** Manual discounts require a Management PIN Override.
- **BR-R-18** Only cash Payments are rounded, to the nearest Rp 100; the rounding difference is recorded.

### 7.6 Payment and Refund

- **BR-R-19** Payment methods: cash (amount tendered, change shown), static QRIS (verified by staff on the merchant app), card via external EDC terminal. A Bill may be settled with several methods.
- **BR-R-20** Outlet Admin may Refund up to the Management-set cap per Bill; above the cap requires Management. Every Refund is audit-logged. Refunds are paid in cash from the Cash Drawer, whatever the original method.
- **BR-R-30** A QRIS Payment is recorded only once staff verify it on the merchant app. If it cannot be verified, staff ask for another method, or record it as **QRIS — pending verification** with an Outlet Admin PIN Override; pending QRIS is resolved at Shift Closing.
- **BR-R-36** All cash goes into the single Cash Drawer. Cash taken by a Waiter is **Held by Waiter** until the Cashier confirms the Cash Handover; differences are recorded against the Waiter.

### 7.7 Receipts

- **BR-R-21** Two 58 mm Receipts per closed Bill or Sub-bill. The **Customer copy** shows the Customer name; the **internal copy** shows only Bill number and Table, to keep personal data off long-stored paper.
- **BR-R-37** Reprints are allowed to any Waiter or Cashier, marked "REPRINT", and audit-logged.

### 7.8 SLA

- **BR-R-22** Kitchen lateness is measured per Order Item against its Prep Time, from confirmation to Ready.
- **BR-R-23** Customers see a queue-based Estimated Wait per Order.

### 7.9 Access

- **BR-R-24** Everyone signs in with Crew ID and a 6-digit PIN. An account locks for 15 minutes after 5 failed attempts.
- **BR-R-25** Management sessions on the Admin Panel expire after 30 minutes of inactivity.
- **BR-R-38** Waiter and Cashier sessions end at shift end or after 12 hours, whichever comes first; a quick PIN re-entry switches user on a shared device.

## 8. Data privacy and retention

See [ADR-0002](adr/0002-two-day-customer-data-retention.md).

- **BR-P-01** Customer name, Dietary Restriction and Dietary Note are stored on the Bill only. No Customer accounts.
- **BR-P-02** Dietary Restrictions and Dietary Notes are health data, classified as **specific personal data** under UU PDP (Law 27/2022) Art. 4; explicit consent is collected before entry.
- **BR-P-03** Two days after a Bill closes, it is Anonymised: names become "Customer", and Dietary Restrictions and Dietary Notes are deleted. Dietary Warnings on Order Items keep only the Product-side tag ("contained Dairy"), not who avoided it. This applies UU PDP Art. 43 (delete data no longer needed for its purpose). The 2-day period is the Business's own necessity judgement; the law sets no fixed number.
- **BR-P-04** Sales, Payment and Receipt figures are retained for 10 years per UU KUP Art. 28(11); they contain no Customer personal data after Anonymisation.
- **BR-P-05** Data is hosted in a cloud region in Jakarta for a real deployment. The prototype/demo runs on a single local host with fictional data only ([ADR-0007](adr/0007-postgresql.md)).
- **BR-P-06** Database backups must not keep Customer names or dietary data readable beyond the 2 days: these fields are encrypted with a daily key, and keys older than 48 hours are destroyed ([ADR-0002](adr/0002-two-day-customer-data-retention.md)).
- **BR-P-07** While a Bill is open, anyone who scans its Table QR sees the full names of the Customers on it (BR-CM-12). This is an accepted owner decision; see §11.

## 9. Non-functional business requirements

| ID | Requirement |
|---|---|
| BR-NF-01 | Capacity: ~30 Tables, ~15 Crew on shift, ~150 Orders per hour at peak. |
| BR-NF-02 | Availability: 99.5% during opening hours. |
| BR-NF-03 | Language: Client Menu in Indonesian and English; staff Modules in Indonesian. |
| BR-NF-04 | Devices: Android tablets and Windows PCs running Chrome; Kitchen and Product Display on large screens. |
| BR-NF-05 | Printing works without the server (local Bluetooth to the device); all other functions require a connection to the server (internet in production, the local network in the demo). |
| BR-NF-06 | Currency: Indonesian Rupiah, no decimal places. Time zone: WIB (Asia/Jakarta). |

## 10. Assumptions and constraints

- One Business, one outlet, permanently ([ADR-0001](adr/0001-single-business-single-outlet.md)).
- The outlet has reliable internet (production) or the demo host is on the same Wi-Fi as all devices (demo); during outages, staff fall back to paper ([ADR-0003](adr/0003-online-only-with-future-local-server.md)).
- Customers use their own phones on mobile data or outlet Wi-Fi.
- Receipt printers are 58 mm Bluetooth thermal printers compatible with browser printing ([ADR-0004](adr/0004-browser-bluetooth-printing.md)).
- Staff devices run Chrome; iPads/Safari are not supported.

## 11. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Internet outage stops ordering | Service falls back to paper | Paper fallback procedure; future local server (ADR-0003) |
| Bought printers incompatible with browser Bluetooth | No Receipts | Verify printer compatibility before purchase (ADR-0004) |
| QR scanned remotely to place prank Orders | Wasted kitchen work | Waiter Claim and confirmation before Kitchen |
| PIN-only sign-in is guessable | Unauthorised actions | 6 digits, lockout, audit log, Management session timeout |
| Staff resist QR ordering | O1 missed | Waiters coach Customers; Waiter-entered Orders still supported |
| Rejoin shows full names and needs no verification, and Table URLs are guessable (`/table-02`): anyone, even off-site, can see who is seated, Rejoin as them, and see their Dietary Warnings | Personal and health data exposure; Orders placed in another person's name | Accepted by owner (2026-10-28, ADR-0006). Names visible only while the Bill is open; every Order still needs Waiter Claim; Rejoin signs out the previous device, so the real Customer notices; a misused Table can be deactivated |
| Cash held by Waiters goes missing | Cash loss | Held by Waiter tracking, Cash Handover confirmation, clock-out blocked until handed over |

## 11a. Prototype and demo

The current build is a **prototype and showcase**, not a live café system. It runs on one local machine with PostgreSQL ([ADR-0007](adr/0007-postgresql.md)), uses only fictional data, and adds a Demo mode (seeded scenarios, demo launcher, demo logins, receipt preview, nightly reset) described in [DEMO-AND-SEED.md](DEMO-AND-SEED.md). All business rules in this document apply unchanged in the demo.

## 12. Open items

None. All gaps found in [OVERVIEW-AND-FLOWS.md](OVERVIEW-AND-FLOWS.md) were resolved on 2026-10-26.
