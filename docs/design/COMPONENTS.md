# Components

Base components are **shadcn/ui** (Radix primitives) themed through tokens ([TOKENS.md §5](TOKENS.md#5-pipeline)). Domain components are built on top of them in `web/packages/ui/` and shared by all apps. Each spec lists **anatomy, states, tokens, motion and accessibility**.

Profiles: **E** = expressive (customer, display), **F** = functional (staff).

## 1. Base components (shadcn/ui)

Button, Input, Textarea, Select, Checkbox, RadioGroup, Switch, Tabs, Dialog, AlertDialog, Sheet, Popover, DropdownMenu, Tooltip, Toast (Sonner), Badge, Card, Table, Skeleton, Avatar, Separator, ScrollArea, Calendar, Command.

### Button

| Variant | Customer | Staff | Use |
|---|---|---|---|
| `primary` | Salmon Deep, pill | Nori, 10 px radius | One per view |
| `secondary` | Neutral 100 | Neutral 100 | Alternatives |
| `ghost` | Transparent | Transparent | Toolbar, low emphasis |
| `danger` | Red 600 | Red 600 | Void, Unpaid, Refund (always after a PIN Override) |
| `outline` | Border input | Border input | Filters |

Sizes `sm 36 / md 44 / lg 52 / xl 64` px height (`button.height.*`); never below the surface touch minimum on touch surfaces. States: default, hover, pressed (scale 0.98, `instant`), focus-visible ring, disabled (`opacity.disabled`), loading (spinner replaces icon, label stays).

## 2. Customer domain components

### ProductCard (E)
- **Anatomy:** product photo (**1:1**, `object-fit: cover`, top radius 20; branded placeholder when the Product has no image) · Sold Out overlay · Dietary badge slot (top-left) · name (`bodyStrong`, 2 lines max) · short description (`caption`, 2 lines) · price (`price`, Espresso) · add button (44 px circle, Salmon Deep, `plus`).
- **States:** default · pressed (scale 0.98) · Sold Out (image greyscale 45 %, "Habis" label, add button hidden) · has Dietary Warning (Plum badge) · in cart (quantity pill replaces add button).
- **Tokens:** `productCard.*`, `color.text.brand`, `shadow.md`, `radius.card`.
- **Motion:** M3 reveal, M4 shared element to detail, M11 Sold Out transition, `#shine`/`#steam` illustration loop only while in viewport.
- **A11y:** whole card is one button named "Salmon nigiri, Rp 38.000"; Sold Out read first; badge text read ("Mengandung susu").

### ProductDetailSheet (E)
- **Anatomy:** hero photo (1:1, shared element; steam overlay on hot drinks) · name (`title`) · description · "Contains" Dietary Tag icons (neutral) · VariantPicker · ModifierGroups · note field · quantity stepper · sticky footer: price + "Tambah ke pesanan".
- **States:** loading skeleton · Variant Sold Out (chip disabled + "Habis") · required Modifier group unmet (footer button disabled with reason text) · Dietary Warning (Plum banner above footer).
- **Motion:** M4, M5 price roll, M6 add-to-cart flight on confirm.

### VariantPicker (E/F)
Segmented chips (customer pill, staff 10 px). Exactly one selected. Sold Out Variants disabled with strike + "Habis". Price difference shown on each chip ("+Rp 5.000").

### ModifierGroup (E/F)
Title with rule ("Pilih 1", "Opsional, maks 3"). Checkboxes (multi) or radios (single). Price per option. Modifiers that add a Dietary Tag show the tag icon. Error text when min not met.

### DietaryBadge (E/F)
- **Anatomy:** Plum `bg` · tag icon · label "Susu" / "Dairy" · optional `alert-circle`.
- **Variants:** `compact` (icon + short label, on cards), `full` ("Mengandung susu — kamu menghindari susu"), `kitchen` (solid Plum header strip, `solid`/`onSolid`, 24 px text: "⚠ SUSU — pelanggan menghindari").
- **Tokens:** `dietaryBadge.*` (`color.dietary.warning.*`). Plum is reserved for this component family.
- **A11y:** never icon-only; `role="status"` when it appears as a result of a choice.

### DietaryWarningDialog (E)
Bottom sheet, calm: illustration-free, Plum icon, text from [CONTENT.md](CONTENT.md#3-writing-rules) rule 5, buttons "Tetap pesan" (primary) / "Lihat menu lain" (secondary). Lists every matching tag. Never red, never blocking.

### RestrictionPicker (E/F)
Grid of the 13 Dietary Tags as toggle chips with icons; free-text Dietary Note below; consent checkbox required before saving (BR-CM-02). Waiter variant is a compact chip row on the order screen.

### CartButton and CartSheet (E)
- Floating button (56 px, Salmon Deep, item count badge) → full-width bar with total when items exist.
- Sheet: grouped items with Variant/Modifiers, quantity steppers, swipe-to-remove (with undo toast), note, subtotal + "pajak & layanan dihitung saat bayar" line, "Kirim pesanan" primary.
- Disabled with explanation while Bill is Settling (BR-CM-15).

### OrderStatusTracker (E)
- **Anatomy:** animated latte cup (cup-fill = elapsed ÷ Estimated Wait, capped at 95 % until Ready) · Estimated Wait "± 12 mnt" (tabular) · WaiterIntro from Claim (first name + portrait, BR-CM-18) · list of the Customer's Orders, each with one Order StatusChip, an item StatusChip only where the item differs (BR-CM-16), and the expected-ready time (BR-CM-17) · "Ubah pesanan" while Pending and unclaimed (BR-CM-14).
- **Motion:** M8 on send, M9 continuous, M10 on Ready.
- **A11y:** progress exposed as `role="progressbar"` with text value; status changes announced politely.

### OccupiedTablePrompt (E)
Sheet listing full names as large buttons ("Saya Andi"), then "Saya baru bergabung" and "Ini bukan meja saya" (BR-CM-12). Plain style, no decoration (M12).

## 3. Staff domain components

### StatusChip (F, also E)
Pill: icon + label in status tone (`bg`/`fg`/`border`). Sizes sm (24) / md (28) / kitchen (40). Cancelled label struck through. Cross-fade on change (150 ms).

### ClaimNotification (F)
- **Anatomy:** type icon (new Order / Waiter call / Sold Out flag / not-my-table) · "Meja 5 · 3 item · 0:42 lalu" · Dietary Warning count badge · primary "Klaim" (56 px) · secondary "Lihat".
- **States:** new (attention pulse ×2) · re-notified (pulse again + "2 mnt" amber) · escalated (amber border, Outlet Admin only) · claimed by other (fade out with name) · claimed by me (moves to Orders).
- **A11y:** `role="alert"` for new; sound + vibration configurable.

### OrderReviewList (F)
Items of a Claimed Order with Variant/Modifiers, notes, Dietary Warnings highlighted (Plum strip), Sold Out/unavailable flags (Nori "Habis" chip). Swipe or menu: Cancel (reason sheet), edit quantity. Footer: "Konfirmasi ke dapur" primary.

### TableMap (F)
Grid of Table tiles (96 px min) showing label, Bill state (free / open / Settling / Unpaid outline), guest names count, longest-waiting item timer, Ready-items badge. Tap → Bill. Long-press → Move / Merge.

### BillSummary (F)
Header (Bill number mono, Tables, state chip) · Orders grouped by Customer name · per-item status · discounts lines · subtotal / discount / service / tax / **total** (`amount`) · payments made · remaining balance. Split view shows Sub-bill tabs.

### PaymentPad (F)
- **Anatomy:** method tabs (Tunai / QRIS / Kartu) · amount due (`amountXl`) · cash: keypad (64 px keys) + quick amounts (Uang pas, 50.000, 100.000) + tendered and change (rounding line shown) · QRIS: "Sudah dicek di aplikasi merchant" confirm + "Belum terverifikasi" (PIN) · card: amount confirm.
- **States:** partial (remaining balance), complete ("Lunas" + check draw), Held by Waiter notice when a Waiter takes cash.

### PinPad (F)
6 dots + 3×4 keypad (72 px keys, `pinPad.*`), backspace, no randomisation. Error: dot shake (the only shake in the product) + "PIN salah. Sisa 2 percobaan." Lockout state shows countdown.

### PinOverrideDialog (F)
Modal: action summary ("Void 1 Salmon Nigiri · Meja 5"), required reason field, approver Crew ID + PinPad, consequence line ("Tercatat di log audit"). Confirm button uses `danger` for Void/Unpaid/Refund. Expires with the 60 s token (countdown visible).

### ReceiptPreview (F)
On-screen 58 mm Receipt (32-char monospaced layout) with Customer / Internal copy toggle, REPRINT marker, print status (pairing, printing, failed → retry). Used when no printer is paired.

### SoldOutToggle (F)
Per Variant switch with "Habis" state; re-enable requires Outlet Admin/Management and shows who set it and why (manual vs Out of Stock Cancel).

### CashHandoverForm (F)
Waiter name, expected amount (from Held by Waiter), received amount keypad, difference highlighted (amber/red) with reason when ≠ 0.

### ShiftClosingSummary (F)
Opening Float, Cash Handovers, counter cash, change, cash Refunds → expected; counted entry; difference; pending QRIS list that must be resolved before closing.

### ClockInPad (F)
Shared HRIS tablet screen: large clock, Crew ID field + PinPad, result card ("Masuk 08.02 · Shift 08.00–16.00"), clock-out blockers (unhanded cash, open Claims).

## 4. Kitchen components

### KitchenTicket (F, kitchen surface)
- **Anatomy:** header strip in status `solid` (Table/Takeaway name, Order time, Waiter) · items (24 px, quantity first: "2× Salmon Nigiri") · Modifiers and notes indented · Dietary Warning strip (Plum `solid`) · PrepTimer · action buttons 64 px ("Mulai", "Siap", "Batalkan").
- **States:** new (3 s highlight) · Preparing · amber (≥ 80 %) · late (red border + "TERLAMBAT") · REMAKE (Salmon border + label, sorted first) · waiting pickup (Ready + elapsed since Ready).
- **Tokens:** `kitchenTicket.*`, dark status tones.

### PrepTimer (F)
`mm:ss` tabular, 48 px on Kitchen; ring progress against Prep Time; colour from `color.timer.*` with label change at thresholds.

## 5. Product Display components

### DisplaySlide templates (E, display surface)
Hero Promo · Featured Product · Happy Hour Countdown · Category Spotlight · Brand Idle — layers and motion in [MOTION.md §6](MOTION.md#6-product-display-banners). Each template takes data (Product ids, promo id, headline, background illustration) and renders within the 64 px safe area; uploaded image/video slides share the transition shell.

## 6. Feedback and empty states

| Pattern | Customer | Staff |
|---|---|---|
| Toast | Rounded 12, bottom, playful icon allowed | Top-right (PC) / top (tablet), status icon |
| Empty state | Logo decoration + friendly line + action | Lucide icon + one line + action |
| Loading | Skeletons shaped like cards; shimmer off under reduced motion | Skeleton rows |
| Offline | Full-width banner "Tidak ada koneksi" | Persistent banner "gunakan catatan kertas" (BR-NF-05) |
| Error page | Illustration of a fallen nigiri + retry | Plain message + retry + support code (mono) |
