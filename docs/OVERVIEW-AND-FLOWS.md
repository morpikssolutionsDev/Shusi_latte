# Sushi Latte — Overview and Flows

This document explains the project's documentation at a high level and maps every operational and backend flow as a chart. Its purpose is to test the spec: every branch in a chart is backed by a rule in the [BRD](BRD.md) or [TRD](TRD.md). Branches that came from the gap review are tagged **✅ G-nn** and listed in the [Gap resolution log](#6-gap-resolution-log). If you find a branch with no rule, the spec needs a revision.

Charts use Mermaid. They render on GitHub and in VS Code with a Mermaid-enabled Markdown preview (e.g. the *Markdown Preview Mermaid Support* extension).

**Contents**

1. [Document map](#1-document-map)
2. [System at a glance](#2-system-at-a-glance)
3. [Operational workflows](#3-operational-workflows)
4. [State machines](#4-state-machines)
5. [Backend flows](#5-backend-flows)
6. [Gap resolution log](#6-gap-resolution-log)

---

## 1. Document map

| Document | Answers | Audience |
|---|---|---|
| `project goals.md` | What the owner originally wanted (kept as-is) | Owner |
| [GLOSSARY.md](../GLOSSARY.md) | What each word means: Bill, Order, Claim, Void, Product… | Everyone |
| [BRD.md](BRD.md) | *What* the business needs: scope, roles, requirements (BR-*), business rules, privacy | Owner, developers |
| [TRD.md](TRD.md) | *How* it is built: architecture, data model, API, realtime, security, printing, jobs (TR-*) | Developers |
| [adr/](adr/) | *Why* hard-to-reverse decisions were made | Developers, future maintainers |
| This file | *How it all flows*, and the record of gaps resolved | Owner, developers |

```mermaid
flowchart LR
  PG["project goals.md<br/>(original intent)"] --> GL[GLOSSARY.md]
  PG --> BRD[BRD.md]
  GL --> BRD
  GL --> TRD[TRD.md]
  BRD -->|"BR-* requirements"| TRD
  TRD --> ADR["ADRs 0001–0004"]
  BRD --> FLOW[Overview and Flows]
  TRD --> FLOW
  FLOW -->|"new gaps"| BRD
  FLOW -->|"new gaps"| TRD
```

## 2. System at a glance

One Business, one outlet, online-only except printing ([ADR-0001](adr/0001-single-business-single-outlet.md), [ADR-0003](adr/0003-online-only-with-future-local-server.md)). Every Module reads the same **Product catalog** and works on the same **Bills**. All cash ends up in one **Cash Drawer**.

```mermaid
flowchart TB
  subgraph People
    CU([Customer])
    WA([Waiter])
    CA([Cashier])
    KI([Kitchen Crew])
    OA([Outlet Admin])
    MG([Management])
  end

  subgraph Modules
    CM[Client Menu]
    PD[Product Display]
    WM[Waiter]
    CS[Cashier]
    KD["Kitchen<br/>(one display per Prep Area)"]
    AP[Admin Panel]
    HR["HRIS<br/>(shared clock-in tablet)"]
  end

  subgraph Core["Shared core"]
    PC[("Product catalog<br/>Products · Variants · Modifiers<br/>Dietary Tags · Sold Out")]
    BL[("Bills · Orders · Order Items<br/>Payments · Receipts")]
    CD[("Cash Drawer<br/>Float · Handovers · Shift Closing")]
    AU[(Audit log)]
  end

  CU --> CM
  CU -.sees.-> PD
  WA --> WM
  CA --> CS
  KI --> KD
  OA --> AP
  MG --> AP
  WA & CA & KI & OA & MG --> HR

  CM --> PC
  CM --> BL
  PD --> PC
  WM --> PC
  WM --> BL
  CS --> PC
  CS --> BL
  CS --> CD
  WM -.cash handover.-> CD
  KD --> BL
  AP --> PC
  AP --> AU
  BL --> AU
  CD --> AU
  WM -.Bluetooth.-> PR1[[58 mm printer]]
  CS -.Bluetooth.-> PR2[[58 mm printer]]
```

---

## 3. Operational workflows

Legend: rounded boxes are start/end, diamonds are decisions, hexagons jump to another workflow, **✅ G-nn** marks a branch added by the gap review.

### OP-01 Customer arrival, joining and Rejoining the Bill

Rules: BR-CM-01, BR-CM-02, BR-CM-12, BR-CM-13, BR-R-01, BR-R-26, BR-P-02, BR-P-07.

```mermaid
flowchart TD
  A([Person scans Table QR]) --> C{"QR token valid<br/>and Table active?"}
  C -->|No| C1["Show 'please ask staff'<br/>(QR regenerated or Table inactive)"]
  C -->|Yes| D{"Table has an<br/>open Bill?"}
  D -->|No| F[Open new Bill for this Table]
  F --> G[Enter full name — required]
  D -->|"Yes (incl. merged Bill)"| P["✅ G-02 Prompt:<br/>'This table is occupied by Andi, Budi.<br/>Is one of these you?'"]
  P --> P1{Choice}
  P1 -->|"I'm Andi"| R["Rejoin as Andi<br/>name, Dietary Restriction, Orders restored"]
  R --> R1["Previous device of Andi signed out:<br/>'You continued on another device'"]
  R1 --> K
  P1 -->|"I'm joining this table"| G
  P1 -->|"This isn't my table"| N["Show 'please ask staff'<br/>notify on-duty Waiters"]
  G --> H{"Ticks Dietary Restriction<br/>or writes Dietary Note?"}
  H -->|No| K
  H -->|Yes| I{Consent given?}
  I -->|No| I1[Dietary data discarded<br/>continue without it]
  I -->|Yes| J[Save restriction and note<br/>on this Customer]
  I1 --> K
  J --> K[Show Client Menu in ID or EN]
  K --> L{{"OP-02 Ordering"}}
  K -.any time.-> W[Call Waiter button] --> W1{{"OP-03 notification"}}
  F -.-> E1["✅ G-01 No Order within 30 min<br/>→ Bill auto-closes, Table free"]
```

### OP-02 Customer ordering, including adding Orders while the Bill is open

Rules: BR-CM-03 to BR-CM-15, BR-PC-04, BR-R-D1 to BR-R-D3, BR-R-06, BR-R-27.

```mermaid
flowchart TD
  A([Customer on Client Menu]) --> S{"✅ G-04 Bill Settling?"}
  S -->|Yes| S1["'Bill is being settled —<br/>please ask the waiter'<br/>ordering blocked"]
  S -->|No| B["Browse Categories<br/>(only Active Products shown)"]
  B --> C{"All Variants of<br/>Product Sold Out?"}
  C -->|Yes| C1[Product greyed out] --> B
  C -->|No| D["Choose Variant<br/>(Sold Out Variants greyed)"]
  D --> E[Choose Modifiers, quantity, note]
  E --> F{"Product tags ∪ Modifier tags<br/>∩ Customer's restriction<br/>not empty?"}
  F -->|No| H[Add to cart]
  F -->|Yes| G["Show Dietary Warning<br/>e.g. 'Contains Dairy'"]
  G --> G1{Customer acknowledges?}
  G1 -->|No| B
  G1 -->|Yes| H
  H --> I{Add more items?}
  I -->|Yes| B
  I -->|No| J[Submit Order]
  J --> K{"Server re-check: Bill not Settling,<br/>Product Active, Variant not Sold Out?"}
  K -->|"No for some items"| K1[Reject those items<br/>show which and why] --> H
  K -->|Yes| L["Order created — Pending<br/>Dietary Warnings copied to Order Items"]
  L --> M{{"OP-03 Claim and confirm"}}
  L --> X{"✅ G-03 Customer wants to<br/>change or cancel it?"}
  X -->|"Yes, not yet Claimed"| X1[Edit or Cancel own Order] --> B
  X -->|"Yes, already Claimed"| X2[Ask the Waiter]
  L --> O["Customer sees Order status<br/>and Estimated Wait"]
  O --> P{"Customer wants<br/>more later?"}
  P -->|Yes| Q{Bill still open?}
  Q -->|Yes| A
  Q -->|"No — Bill was closed"| R[Next scan opens a new Bill]
  P -->|No| Z([Wait for food])
```

### OP-03 Waiter Claim, escalation and confirmation

Rules: BR-WT-01, BR-WT-02, BR-WT-05, BR-WT-10, BR-WT-11, BR-R-06, BR-R-34, TR-SM-03, TR-SM-05.

```mermaid
flowchart TD
  A([New Pending Order or Waiter call]) --> B["Notify all on-duty Waiters<br/>sound + vibration"]
  B --> C{Claimed within 2 min?}
  C -->|No| C1["✅ G-05 Re-notify all Waiters"]
  C1 --> C2{Claimed within 5 min?}
  C2 -->|No| C3["✅ G-05 Alert Outlet Admin"]
  C2 -->|Yes| D
  C3 --> D
  C -->|Yes| D{First to Claim?}
  D -->|No| D1[409 — notification withdrawn]
  D -->|Yes| E[Waiter owns the Order]
  E --> T{"✅ G-06 Waiter busy, on break<br/>or clocking out?"}
  T -->|Yes| T1["Another Waiter takes over<br/>or Outlet Admin reassigns<br/>audit-logged"] --> E
  T -->|No| F["Go to Table, review with Customer<br/>Dietary Warnings and Sold Out/unavailable<br/>flags highlighted"]
  F --> G{Customer at Table?}
  G -->|"No — prank or wrong Table"| G1["Cancel all items<br/>reason Other"] --> G2([Order closed empty])
  G -->|Yes| H{Changes needed?}
  H -->|Remove item| H1["Cancel item<br/>Customer Request or Wrong Order"] --> H
  H -->|"Item flagged Sold Out<br/>or Product Hidden"| H2["Inform Customer<br/>Cancel, offer alternative"] --> H
  H -->|Add item or change qty| H3[Edit Order before confirm] --> H
  H -->|No more changes| I{Any items left?}
  I -->|No| G2
  I -->|Yes| J[Confirm Order]
  J --> K["Items → Confirmed<br/>routed to Prep Area by Category"]
  K --> L{{"OP-05 Kitchen"}}
```

### OP-04 Waiter or Cashier direct entry

Rules: BR-WT-03, BR-CS-01.

```mermaid
flowchart TD
  A([Customer orders verbally]) --> B[Waiter selects Table]
  B --> C{Table has open Bill?}
  C -->|Yes| D[Use that Bill]
  C -->|No| E[Open new Bill]
  D --> F
  E --> F["Optional: Customer name<br/>Optional: tick Dietary Restriction for this Order"]
  F --> G["Pick Products — Sold Out greyed<br/>Dietary Warnings shown if restriction ticked"]
  G --> H[Submit]
  H --> I{"Server re-check<br/>Active and not Sold Out?"}
  I -->|No| I1[Reject those items] --> G
  I -->|Yes| J["Order created directly as Confirmed<br/>(no Claim step) — Waiter owns it"]
  J --> K{{"OP-05 Kitchen"}}
```

### OP-05 Kitchen processing

Rules: BR-KT-01 to BR-KT-06, BR-R-07, TR-KT-01.

```mermaid
flowchart TD
  A([Order Item Confirmed]) --> B["Appears on its Prep Area display<br/>REMAKE items first, then by confirmation time<br/>with notes and Dietary Warnings"]
  B --> C["Timer against Prep Time<br/>amber at 80% — red at 100%"]
  C --> D{"Can the Kitchen<br/>make it?"}
  D -->|"No — known before starting"| X["✅ G-07 Cancel from Confirmed<br/>with Cancel Reason"]
  D -->|Yes| E[Tap Start → Preparing]
  E --> F{Completed?}
  F -->|Yes| G[Tap Ready → Ready]
  F -->|"No — ran out mid-prep"| Y[Cancel from Preparing<br/>with Cancel Reason]
  X --> H1{Reason = Out of Stock?}
  Y --> H1
  H1 -->|Yes| H2{{"OP-08 Sold Out"}}
  H1 -->|No| H3[Owning Waiter notified of Cancel]
  H2 --> H3
  G --> I["Recalculate Estimated Wait<br/>for open Orders in this Prep Area"]
  G --> J{{"OP-06 Serving"}}
  H3 --> I
```

### OP-06 Serving and Remake

Rules: BR-WT-09, BR-WT-12, BR-KT-06, BR-R-09, BR-R-33.

```mermaid
flowchart TD
  A([Order Item Ready]) --> B[Owning Waiter notified]
  B --> P{Picked up within 3 min?}
  P -->|No| P1["✅ G-09 Re-notify Waiter<br/>Kitchen shows 'waiting pickup'"] --> P
  P -->|Yes| C[Waiter delivers to Table]
  C --> D{Correct and good?}
  D -->|Yes| E[Mark Served]
  D -->|"No — wrong or bad"| R["✅ G-08 Remake:<br/>Outlet Admin PIN + reason"]
  R --> R1["No-charge copy flagged REMAKE<br/>top of Kitchen queue<br/>original stays on Bill"]
  R1 --> K{{"OP-05 Kitchen"}}
  E --> F{"All live items<br/>in the Order Served?"}
  F -->|Yes| G[Order shows Served on Client Menu]
  F -->|No| H[Keep waiting for remaining items]
```

### OP-07 Cancel or Void — which one applies

Rules: BR-R-07, BR-R-08, BR-R-10, TR-AUTH-06.

```mermaid
flowchart TD
  A([Need to remove an Order Item]) --> B{Item status?}
  B -->|"Pending, not Claimed"| C0["Customer may Cancel own item<br/>— or Waiter Cancels"]
  B -->|"Pending (Claimed) or Confirmed"| C["Waiter or Cashier Cancels<br/>Cancel Reason — no PIN"]
  B -->|"Confirmed or Preparing"| D["Kitchen Cancels<br/>Cancel Reason — no PIN"]
  B -->|"Ready or Served"| E{"✅ G-10 Is the item's Bill<br/>or Sub-bill already paid?"}
  E -->|No| F["Void — PIN Override<br/>Outlet Admin or Management<br/>reason required"]
  E -->|Yes| G{{"OP-11 Refund"}}
  C0 --> J
  C --> H{Reason = Out of Stock?}
  D --> H
  H -->|Yes| I{{"OP-08 Sold Out"}}
  H -->|No| J[Item excluded from totals]
  I --> J
  F --> K[Audit log entry] --> J
```

### OP-08 Sold Out and unavailable Products

Rules: BR-R-11 to BR-R-13, BR-R-35, BR-ADM-07, TR-SM-04, TR-SM-06.

```mermaid
flowchart TD
  A1([Outlet Admin or Management<br/>marks Variant Sold Out]) --> B
  A2([Cancel with reason Out of Stock]) --> B
  A3(["✅ G-23 Management sets Product<br/>Hidden or Archived"]) --> U[Product off all menus]
  B[Variant Sold Out] --> C["Broadcast to all Modules<br/>Variant greyed everywhere"]
  C --> D{"All Variants of the<br/>Product Sold Out?"}
  D -->|Yes| D1[Product greyed out]
  D -->|No| D2[Only that Variant greyed]
  B --> E{"Open Order Items affected<br/>in Pending or Confirmed?"}
  U --> E
  E -->|No| F([Done])
  E -->|Yes| G{Order claimed?}
  G -->|Yes| G1[Flag to owning Waiter]
  G -->|No| G2[Flag to all on-duty Waiters]
  G1 --> H["Waiter informs Customer<br/>Customer picks alternative or drops it<br/>Waiter Cancels — nothing auto-cancelled"]
  G2 --> H
  B --> I{"In a Customer's<br/>unsubmitted cart?"}
  I -->|Yes| I1[Rejected at submit — OP-02]
  B --> R["Later: Outlet Admin or Management<br/>re-enables Variant"]
  R --> R1[Audit log: who and when] --> R2[Orderable again everywhere]
```

### OP-09 Bill changes: Move, Merge, Split

Rules: BR-WT-06, BR-R-03, BR-R-04, BR-R-28, BR-R-29, TR-DM-03, TR-DM-06, TR-PR-04.

```mermaid
flowchart TD
  A([Bill change requested]) --> B{Which change?}

  B -->|Move| M1{Target Table free?}
  M1 -->|Yes| M2[Re-point Bill to target Table<br/>old Table becomes free]
  M1 -->|"No — has open Bill"| M3["✅ G-11 Blocked:<br/>'Table occupied — use Merge'"]

  B -->|Merge| G1[Select Tables with open Bills]
  G1 --> G2["One Bill survives, covering all Tables<br/>others closed as 'merged'"]
  G2 --> G3[Irreversible — use Split to separate payment]

  B -->|Split| S0{"Any Payment already<br/>recorded on the Bill?"}
  S0 -->|Yes| S00["✅ G-12 Blocked:<br/>decide the split before paying"]
  S0 -->|No| S1{Split method?}
  S1 -->|By item| S2[Assign each Order Item to a Sub-bill]
  S2 --> S2a{All items assigned?}
  S2a -->|No| S2
  S2a -->|Yes| S4
  S1 -->|Equally| S3["Choose N — remainder Rupiah<br/>to the last Sub-bill"]
  S3 --> S4["Each Sub-bill gets its own discount share,<br/>service, tax, Payments and Receipts"]
  S4 --> P{{"OP-10 Payment"}}
  B -.-> AUD[All three are audit-logged]
```

### OP-10 Payment and closing the Bill

Rules: BR-WT-07, BR-WT-08, BR-WT-13 to BR-WT-15, BR-R-14 to BR-R-21, BR-R-27, BR-R-30 to BR-R-32, BR-R-36, BR-R-37.

```mermaid
flowchart TD
  A([Customer asks for the Bill]) --> L0["✅ G-04 Bill → Settling<br/>Customer ordering locked"]
  L0 --> L1{Customer wants to add more?}
  L1 -->|Yes| L2[Waiter unlocks → Open] --> Z0{{"OP-02 / OP-04"}}
  L1 -->|No| B{"Items still Pending,<br/>Confirmed or Preparing?"}
  B -->|Yes| B1{"✅ G-13 Staff confirms<br/>'pay now, serve later'?"}
  B1 -->|No| B2[Wait or Cancel remaining items] --> B
  B1 -->|Yes| C
  B -->|No| C["Show Bill summary<br/>best automatic discounts applied"]
  C --> D{Manual discount requested?}
  D -->|Yes| D1[Management PIN Override] --> C
  D -->|No| E{Split needed?}
  E -->|Yes| E1{{"OP-09 Split"}}
  E -->|No| F
  E1 --> F[For each Bill or Sub-bill]
  F --> G{Payment method}
  G -->|Cash| G1["Tendered, round to Rp 100, change"]
  G1 --> G1a{Taken by?}
  G1a -->|Waiter| G1b["✅ G-20 Recorded as<br/>Held by Waiter → OP-13 handover"]
  G1a -->|Cashier| G1c[Straight into Cash Drawer]
  G -->|QRIS| G2{"Staff can verify on<br/>merchant app?"}
  G2 -->|Yes| G2a[Record QRIS]
  G2 -->|No| G2b{"✅ G-16 Customer pays<br/>another way?"}
  G2b -->|Yes| G
  G2b -->|No| G2c["Outlet Admin PIN:<br/>QRIS — pending verification<br/>resolved at Shift Closing"]
  G -->|Card| G3[Customer pays on EDC<br/>staff records amount]
  G1b --> H
  G1c --> H
  G2a --> H
  G2c --> H
  G3 --> H{Paid in full?}
  H -->|"No — balance left"| G
  H -->|Yes| I[Close Bill or Sub-bill]
  I --> J["Print Customer copy (with name)<br/>and internal copy (Bill no. + Table)"]
  J --> K{Print OK?}
  K -->|No| K1["Retry — data held on device<br/>works without internet"] --> K
  K -->|Yes| M{All Sub-bills closed?}
  M -->|No| F
  M -->|Yes| N([Bill closed — Table free])
  N --> N1{{"OP-17 Data lifecycle"}}
  N -.later.-> RP["✅ G-15 Reprint by any Waiter/Cashier<br/>marked REPRINT, audit-logged"]
  A -.-> W1{"✅ G-14 Customer left<br/>without paying?"}
  W1 -->|Yes| W2["Outlet Admin PIN + reason<br/>Bill closed as Unpaid<br/>reported as loss"]
```

### OP-11 Refund

Rules: BR-R-20, BR-CS-03, BR-ADM-06, TR-AUTH-06.

```mermaid
flowchart TD
  A([Problem found after Bill is paid]) --> B[Cashier opens closed Bill]
  B --> C[Enter Refund amount and reason]
  C --> D{"Amount ≤ Refund cap<br/>set by Management?"}
  D -->|Yes| E[PIN Override by Outlet Admin or Management]
  D -->|No| F[PIN Override by Management only]
  E --> G[Record Refund]
  F --> G
  G --> H["✅ G-17 Paid in cash from the Cash Drawer<br/>whatever the original method"]
  H --> I[Reduces expected cash at Shift Closing]
  G --> J[Audit log]
```

### OP-12 Takeaway

Rules: BR-CS-02, TR-PAY-04.

```mermaid
flowchart TD
  A([Walk-in Customer at counter]) --> B[Cashier opens Takeaway Bill<br/>with Customer name]
  B --> C[Add items — Sold Out greyed<br/>optional Dietary Restriction]
  C --> D["✅ G-18 Customer pays first<br/>(OP-10 payment steps)"]
  D --> E{Paid in full?}
  E -->|No| D
  E -->|Yes| F[Order Confirmed → Kitchen]
  F --> G{{"OP-05 Kitchen"}}
  G --> H[All items Ready]
  H --> I[Call out Customer name]
  I --> J([Handed over — Served])
```

### OP-13 Cash Drawer: float, handover and Shift Closing

Rules: BR-CS-04 to BR-CS-07, BR-WT-14, BR-HR-07, TR-PAY-01 to TR-PAY-03.

```mermaid
flowchart TD
  A(["Cashier shift starts"]) --> B["✅ G-19 Cashier enters Opening Float<br/>drawer session opens"]
  B --> C[Service runs]
  C --> W([Waiter holds cash from Table Payments])
  W --> W1["✅ G-20 Waiter hands cash to Cashier"]
  W1 --> W2[Cashier enters amount received]
  W2 --> W3{Matches Held by Waiter total?}
  W3 -->|Yes| W4[Payments settled into drawer]
  W3 -->|No| W5[Difference recorded against Waiter<br/>audit-logged] --> W4
  C --> E(["Cashier shift ends<br/>or Cashier changes mid-day"])
  E --> Q{"Any QRIS pending<br/>verification?"}
  Q -->|Yes| Q1[Verify now, or write off with reason] --> Q
  Q -->|No| F["Expected = Float + handovers + counter cash<br/>− change − cash Refunds"]
  F --> G[Cashier counts drawer]
  G --> H[Enter counted amount → difference recorded]
  H --> I{Cashier changing mid-day?}
  I -->|Yes| B
  I -->|No| J([Drawer closed for the day])
```

### OP-14 Internet outage

Rules: BR-NF-05, BR-CS-08, TR-NF-07, ADR-0003.

```mermaid
flowchart TD
  A([Internet drops]) --> B["Staff app banner:<br/>'No connection — use paper'"]
  B --> C[Reprint of held Receipts still works]
  B --> D[New Orders, kitchen tickets and Payments on paper]
  B -.-> I["Client Menu unreachable<br/>for Customers"]
  D --> G([Internet returns])
  G --> H[Apps reconnect and refetch]
  H --> P["✅ G-21 Cashier or Outlet Admin enters<br/>paper Bills as Post-outage Entries"]
  P --> P1["Original times, items, Payments,<br/>serving Crew — no Kitchen routing"]
  P1 --> P2["Marked 'outage' in reports<br/>cash included in drawer"]
```

### OP-15 HRIS

Rules: BR-HR-01 to BR-HR-07, BR-WT-11, TR-AUTH-08.

```mermaid
flowchart TD
  subgraph Attendance
    A([Crew arrives]) --> B["✅ T-1 Shared HRIS tablet only<br/>enter Crew ID + PIN"]
    B --> C{Locked out?}
    C -->|Yes| C1[Wait 15 min or Management resets PIN]
    C -->|No| D{PIN correct?}
    D -->|No| D1["Failed attempt counted<br/>5 fails → 15 min lock"]
    D -->|Yes| E{Currently clocked in?}
    E -->|No| F[Clock in]
    E -->|Yes| H{"✅ G-20 Holding cash<br/>not handed over?"}
    H -->|Yes| H1["Blocked: 'Hand over Rp X<br/>to the Cashier first'"]
    H -->|No| H2{"✅ G-06 Open Claims?"}
    H2 -->|Yes| H3[List Claims to hand over] --> G
    H2 -->|No| G[Clock out]
    F -.-> AU["✅ G-22 Still clocked in at shift end + 2 h<br/>→ auto clock-out, flagged<br/>Outlet Admin corrects with reason"]
  end
  subgraph Leave
    L1([Crew requests leave]) --> L2[Outlet Admin or Management reviews]
    L2 --> L3{Approve?}
    L3 -->|Yes| L4[Approved — shift schedule shows absence]
    L3 -->|No| L5[Rejected with reason]
  end
  subgraph Export
    X1([Payroll period ends]) --> X2[Management exports attendance]
  end
```

### OP-16 Catalog and Product Display changes

Rules: BR-PC-*, BR-ADM-01 to BR-ADM-10, BR-PD-*, BR-R-35, TR-PC-05.

```mermaid
flowchart TD
  A([Management edits catalog or slides]) --> B{What changed?}
  B -->|"Price / name / Modifiers"| C[Applies to new Orders only<br/>existing Order Items keep snapshot]
  B -->|"Product Status → Hidden/Archived"| D["Removed from menus<br/>✅ G-23 open items flagged — OP-08"]
  B -->|Dietary Tags| E[Warnings apply to new orders]
  B -->|Display slides| F[Product Display updates live]
  C --> G[Audit log]
  D --> G
  E --> G
  G --> H["Broadcast — all open menus refresh"]
```

### OP-17 Customer data lifecycle

Rules: BR-P-01 to BR-P-07, ADR-0002.

```mermaid
flowchart LR
  A([Customer joins Bill]) --> B["Name + Dietary Restriction + Note<br/>stored encrypted with today's key"]
  B --> C[Used during visit:<br/>warnings, Waiter, Kitchen, Rejoin prompt]
  C --> D([Bill closed or Unpaid])
  D --> E["+48 hours: Anonymisation job"]
  E --> F["Name → 'Customer'<br/>Restriction and Note deleted"]
  F --> G["Kept 10 years: totals, Payments,<br/>Order Items, Product-side warnings"]
  E --> H["✅ G-24 Old daily keys destroyed<br/>→ backups unreadable for these fields"]
```

---

## 4. State machines

### Order Item

```mermaid
stateDiagram-v2
  [*] --> Pending : Customer places Order
  [*] --> Confirmed : direct entry, or Remake copy
  Pending --> Confirmed : Claim + confirm
  Pending --> Cancelled : Customer before Claim, or Waiter
  Confirmed --> Preparing : Kitchen starts
  Confirmed --> Cancelled : Waiter, Cashier or Kitchen
  Preparing --> Ready : Kitchen done
  Preparing --> Cancelled : Kitchen
  Ready --> Served : Waiter serves
  Ready --> Voided : PIN Override, unpaid
  Served --> Voided : PIN Override, unpaid
  Cancelled --> [*]
  Voided --> [*]
  Served --> [*]
```

A Remake does not change the original item's state; it creates a new no-charge item that starts at Confirmed.

### Order (derived from its items)

```mermaid
stateDiagram-v2
  [*] --> Pending : placed by Customer
  [*] --> Confirmed : direct entry
  Pending --> Pending : Customer edits before Claim
  Pending --> Claimed : Waiter Claims
  Claimed --> Claimed : takeover or reassign
  Claimed --> Confirmed : Waiter confirms
  Pending --> Closed : all items Cancelled
  Claimed --> Closed : all items Cancelled
  Confirmed --> Ready : all live items Ready
  Ready --> Served : all live items Served
  Confirmed --> Closed : all items Cancelled
  Served --> [*]
  Closed --> [*]
```

### Bill

```mermaid
stateDiagram-v2
  [*] --> Open : first scan or first direct Order
  Open --> Open : new Orders, Move, Split, Rejoin
  Open --> Closed : no Orders after 30 min
  Open --> Settling : payment started
  Settling --> Open : Waiter unlocks
  Settling --> Closed : fully paid
  Open --> Closed : fully paid
  Open --> Unpaid : Outlet Admin PIN + reason
  Settling --> Unpaid : Outlet Admin PIN + reason
  Open --> Merged : absorbed into another Bill
  Closed --> Closed : Refund or reprint
  Closed --> Anonymised : 48 h later
  Unpaid --> Anonymised : 48 h later
  Merged --> [*]
  Anonymised --> [*]
```

### Payment

```mermaid
stateDiagram-v2
  [*] --> Settled : Cashier cash, card, verified QRIS
  [*] --> HeldByWaiter : Waiter cash
  [*] --> PendingVerification : unverified QRIS + PIN
  HeldByWaiter --> Settled : Cash Handover received
  PendingVerification --> Settled : verified
  PendingVerification --> WrittenOff : at Shift Closing, with reason
  Settled --> [*]
  WrittenOff --> [*]
```

### Product and Variant

```mermaid
stateDiagram-v2
  state Product {
    [*] --> Active
    Active --> Hidden
    Hidden --> Active
    Active --> Archived
    Hidden --> Archived
  }
  state Variant {
    [*] --> Available
    Available --> SoldOut : manual or Cancel Out of Stock
    SoldOut --> Available : Outlet Admin or Management re-enables
  }
```

---

## 5. Backend flows

### BE-01 Sign-in and lockout

```mermaid
sequenceDiagram
  participant D as Device
  participant API as Go API
  participant DB as PostgreSQL
  D->>API: POST /auth/login (crew_id, pin)
  API->>DB: load crew row
  alt locked_until in the future
    API-->>D: 423 Locked (retry after)
  else PIN wrong
    API->>DB: failed_attempts + 1, lock 15 min at 5
    API-->>D: 401
  else PIN correct
    API->>DB: reset failed_attempts, create session (ends at shift end or 12 h)
    API-->>D: 200 + HttpOnly session cookie
  end
```

### BE-02 QR scan: join or Rejoin

```mermaid
sequenceDiagram
  participant C as Client Menu (new device)
  participant O as Old device
  participant API as Go API
  participant DB as PostgreSQL
  participant HUB as WebSocket hub
  C->>API: GET /public/tables/{qr_token}
  API->>DB: find open Bill for Table
  alt no open Bill
    API-->>C: free
    C->>API: POST join (name, tags, note, consent)
    API->>DB: BEGIN, lock Table, INSERT bill + customer_entry (encrypted), COMMIT
    API-->>C: guest token
  else open Bill
    API->>DB: decrypt Customer names on Bill
    API-->>C: occupied + names
    alt "I'm Andi"
      C->>API: POST rejoin (customer_entry_id)
      API->>DB: set active_device_id, revoke old token
      API->>DB: NOTIFY customer:{id}
      DB-->>HUB: event
      HUB-->>O: You continued on another device
      API-->>C: guest token for Andi
    else "I'm joining this table"
      C->>API: POST join
      API->>DB: INSERT customer_entry on existing Bill
      API-->>C: guest token
    else "This isn't my table"
      C->>API: POST not-mine
      API->>DB: NOTIFY waiters
      API-->>C: please ask staff
    end
  end
```

### BE-03 Place Order → notify Waiters

```mermaid
sequenceDiagram
  participant C as Client Menu
  participant API as Go API
  participant DB as PostgreSQL
  participant HUB as WebSocket hub
  participant W as Waiter devices
  C->>API: POST /public/orders (Idempotency-Key)
  API->>DB: BEGIN
  API->>DB: verify Bill open and not settling
  API->>DB: verify Products Active, Variants not Sold Out
  API->>DB: INSERT order (Pending), order_items with snapshots
  API->>DB: compute Dietary Warnings → order_item_dietary_warning
  API->>DB: NOTIFY waiters + bill:{id}
  API->>DB: COMMIT
  API-->>C: 201 Order
  DB-->>HUB: LISTEN event
  HUB-->>W: order.pending (id only)
  W->>API: GET order (refetch)
```

### BE-04 Claim race, escalation and takeover

```mermaid
sequenceDiagram
  participant W1 as Waiter A
  participant W2 as Waiter B
  participant J as Escalation job
  participant API as Go API
  participant DB as PostgreSQL
  participant HUB as WebSocket hub
  J->>DB: Orders unclaimed past 2 min / 5 min
  J->>DB: NOTIFY waiters (re-notify) or outlet-admin (escalate)
  W1->>API: POST /orders/{id}/claim
  W2->>API: POST /orders/{id}/claim
  API->>DB: UPDATE order SET claimed_by=A WHERE claimed_by IS NULL
  DB-->>API: 1 row
  API-->>W1: 200 Claimed
  API->>DB: same UPDATE for B
  DB-->>API: 0 rows
  API-->>W2: 409 Already claimed
  API->>DB: NOTIFY waiters order.claimed
  DB-->>HUB: event
  HUB-->>W2: withdraw notification
  W2->>API: POST /orders/{id}/takeover (later)
  API->>DB: claimed_by=B, INSERT claim_transfer, audit_log
  API->>DB: NOTIFY waiters
  HUB-->>W1: Claim taken over by B
```

### BE-05 Confirm → Kitchen → Ready (with Estimated Wait)

```mermaid
sequenceDiagram
  participant W as Waiter
  participant API as Go API
  participant DB as PostgreSQL
  participant HUB as WebSocket hub
  participant K as Prep Area display
  participant C as Client Menu
  W->>API: POST /orders/{id}/confirm
  API->>DB: items Pending → Confirmed (WHERE status = Pending)
  API->>DB: NOTIFY prep-area:{id} for each Prep Area involved
  DB-->>HUB: events
  HUB-->>K: new items (Remakes sorted first)
  K->>API: transition → Preparing, later → Ready
  API->>DB: optimistic status update, set ready_at
  API->>API: recompute Estimated Wait for Prep Area queue
  API->>DB: NOTIFY bill:{id}, waiters
  HUB-->>C: status + Estimated Wait
  HUB-->>W: item Ready
```

### BE-06 Cancel with Out of Stock → Sold Out cascade

```mermaid
sequenceDiagram
  participant K as Kitchen or Waiter
  participant API as Go API
  participant DB as PostgreSQL
  participant HUB as WebSocket hub
  K->>API: POST /order-items/{id}/cancel (reason out_of_stock)
  API->>DB: BEGIN
  API->>DB: item → Cancelled (guarded by role and status)
  API->>DB: variant.sold_out = true, source cancel_out_of_stock
  API->>DB: flag open Pending/Confirmed items with same Variant
  API->>DB: INSERT audit_log
  API->>DB: NOTIFY menu, waiters, prep-area
  API->>DB: COMMIT
  DB-->>HUB: events
  HUB-->>HUB: fan out to all menus (grey out) and owning Waiters (flags)
```

### BE-07 PIN Override (Void, Remake, Unpaid, manual discount, Refund, pending QRIS)

```mermaid
sequenceDiagram
  participant S as Staff device
  participant A as Approver (on same device)
  participant API as Go API
  participant DB as PostgreSQL
  S->>A: hand device for approval
  A->>API: POST /auth/override (approver PIN, action)
  API->>DB: verify approver role allows action (and Refund cap)
  API-->>S: one-time override token (60 s)
  S->>API: POST action endpoint + X-Override-Token
  API->>DB: BEGIN, apply action, audit_log (actor + approver), COMMIT
  API-->>S: 200
```

### BE-08 Settle, pay, close and print

```mermaid
sequenceDiagram
  participant S as Waiter/Cashier PWA
  participant API as Go API
  participant DB as PostgreSQL
  participant P as Bluetooth printer
  S->>API: POST /bills/{id}/settle
  API->>DB: status → settling, NOTIFY bill:{id}
  S->>API: GET /bills/{id}/summary
  API->>API: pricing engine (discounts → service → tax)
  API-->>S: totals + unfinished-item warning
  loop until fully paid
    S->>API: POST /bills/{id}/payments (method, amount, Idempotency-Key)
    API->>DB: INSERT payment (settled / held_by_waiter / pending_verification)
  end
  S->>API: POST /bills/{id}/close (pay_now_serve_later if needed)
  API->>DB: status → closed, closed_at
  API-->>S: Receipt data
  S->>S: hold Receipt data locally
  S->>P: ESC/POS over Web Bluetooth — Customer copy
  S->>P: ESC/POS — internal copy
  S->>API: record receipt_print rows
  alt print fails
    S->>S: show Retry (works offline)
  end
```

### BE-09 Cash Handover and Shift Closing

```mermaid
sequenceDiagram
  participant W as Waiter
  participant C as Cashier
  participant API as Go API
  participant DB as PostgreSQL
  C->>API: POST /drawer-sessions (opening_float)
  API->>DB: INSERT drawer_session
  W->>C: hands over cash
  C->>API: GET /crew/{waiter}/held-cash
  API-->>C: expected Rp X
  C->>API: POST /cash-handovers (waiter, received)
  API->>DB: INSERT cash_handover, payments → settled, difference audit-logged
  W->>API: POST /attendance/clock (clock out)
  API->>DB: check no held_by_waiter payments
  API-->>W: ok
  C->>API: POST /drawer-sessions/{id}/close (counted)
  API->>DB: refuse if pending_verification QRIS remain
  API->>DB: expected = float + handovers + counter cash − change − cash refunds
  API->>DB: store counted and difference
  API-->>C: Shift Closing summary
```

### BE-10 Realtime pipeline

```mermaid
flowchart LR
  TX["API transaction<br/>COMMIT"] --> N["pg NOTIFY<br/>(topic, entity id, version)"]
  N --> L[Go LISTEN goroutine]
  L --> H[WebSocket hub]
  H -->|"waiters"| WD[Waiter devices]
  H -->|"outlet-admin"| OA[Outlet Admin devices]
  H -->|"prep-area:id"| KD[Kitchen displays]
  H -->|"bill:id"| CM[Customer phones]
  H -->|"customer:id"| CD[Device revoked on Rejoin]
  H -->|"menu"| ALL[All menus]
  H -->|"display"| TV[Product Display]
  WD & OA & KD & CM & ALL & TV -->|"refetch via TanStack Query"| API[REST API]
```

### BE-11 Scheduled jobs

```mermaid
flowchart TD
  S([Scheduler — advisory lock]) --> J1["Every 15 s: Claim re-notify / escalate<br/>pickup reminders"]
  S --> J2["Every 1 min: close empty Bills > 30 min<br/>activate/deactivate promos"]
  S --> J3["Every 5 min: auto clock-out<br/>past shift end + 2 h"]
  S --> J4["Every 1 h: Anonymise Bills<br/>closed/unpaid > 48 h"]
  S --> J5["Daily: destroy expired data keys<br/>run backup"]
  J1 & J2 & J3 & J4 & J5 --> F{Failure?}
  F -->|Yes| A[Alert via monitoring]
  F -->|No| OK([Done])
```

---

## 6. Gap resolution log

All gaps found by the first version of these charts were resolved on 2026-10-26.

| ID | Gap | Decision | Spec reference |
|---|---|---|---|
| G-01 | Bill opened, no Order placed | Auto-close after 30 min | BR-R-26 |
| G-02 | Customer scans again | Prompt with full names: Rejoin as self, join as new, or "not my table"; bound to Bill, latest device wins | BR-CM-12, BR-CM-13, TR-AUTH-07 |
| G-03 | Customer edits own Order | Allowed until Claimed | BR-CM-14 |
| G-04 | Order during payment | Bill Settling blocks Customer Orders; Waiter can unlock | BR-R-27, BR-CM-15 |
| G-05 | Unclaimed Order or call | Re-notify at 2 min, alert Outlet Admin at 5 min | BR-WT-10 |
| G-06 | Claim handover | Takeover by Waiter or reassign by Outlet Admin; listed at clock-out | BR-WT-11, BR-R-34 |
| G-07 | Kitchen can't make Confirmed item | Kitchen may Cancel from Confirmed | BR-KT-04, BR-R-07 |
| G-08 | Wrong or bad item | Remake: Outlet Admin PIN, no-charge copy, prioritised | BR-WT-12, BR-R-33 |
| G-09 | Ready not picked up | Re-notify at 3 min, "waiting pickup" on Kitchen | BR-WT-09, BR-KT-06 |
| G-10 | Void on partly paid split | Void only on unpaid Sub-bills; else Refund | BR-R-08 |
| G-11 | Move onto occupied Table | Blocked; use Merge | BR-R-28 |
| G-12 | Split after Payment | Blocked | BR-R-29 |
| G-13 | Pay with unfinished items | Allowed with staff "pay now, serve later" | BR-R-31 |
| G-14 | Walkout | Unpaid Bill via Outlet Admin PIN + reason | BR-R-32 |
| G-15 | Reprint | Any Waiter/Cashier, marked REPRINT, audit-logged | BR-WT-13, BR-R-37 |
| G-16 | QRIS unverifiable | Another method, or pending verification with Outlet Admin PIN | BR-R-30, BR-CS-07 |
| G-17 | Refund method | Cash from the Cash Drawer | BR-R-20 |
| G-18 | Takeaway pay timing | Pay first | BR-CS-02 |
| G-19 | Opening float | Entered by Cashier per drawer session | BR-CS-04 |
| G-20 | Cash drawer | One drawer at Cashier; Waiter cash Held by Waiter until Cash Handover; clock-out blocked while holding cash | BR-CS-05, BR-CS-06, BR-WT-14, BR-HR-07, BR-R-36 |
| G-21 | Paper during outage | Post-outage Entry by Cashier or Outlet Admin | BR-CS-08 |
| G-22 | Forgot to clock out | Auto clock-out at shift end + 2 h, corrected by Outlet Admin | BR-HR-06 |
| G-23 | Product Hidden with Pending items | Flagged like Sold Out | BR-R-35 |
| G-24 | Backups beyond 48 h | Crypto-shredding | BR-P-06, TR-RET-04, ADR-0002 |
| T-1 | Clock-in location | Shared HRIS tablet only | BR-HR-03, TR-AUTH-08 |
| T-3 | Shared-device session length | Shift end or 12 h | BR-R-38, TR-AUTH-09 |
| T-4 | Kitchen parallel capacity | Kitchen 2, Bar 1, adjustable | TR-KT-02 |
