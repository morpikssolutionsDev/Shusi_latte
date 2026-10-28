# Layout, Spacing, Shape and Elevation

## 1. Spacing

4 px base. Use only these steps (`space.*`):

| Token | px | Typical use |
|---|---|---|
| `space.1` | 4 | Icon-to-label gap in chips |
| `space.2` | 8 | Tight stacks, chip gaps |
| `space.3` | 12 | Inside compact cards, PIN pad gap |
| `space.4` | 16 | **Default** padding, phone gutter |
| `space.5` | 20 | Customer card padding |
| `space.6` | 24 | Section gaps, tablet gutter |
| `space.8` | 32 | Between major sections |
| `space.10` | 40 | Customer hero spacing |
| `space.12` | 48 | Desktop page padding |
| `space.16` | 64 | Display safe area (TV) |
| `space.20` / `space.24` | 80 / 96 | Display hero layouts |

## 2. Breakpoints and devices

| Token | Min width | Target |
|---|---|---|
| `breakpoint.phone` | 360 | Client Menu baseline |
| `breakpoint.phoneLg` | 430 | Large phones; menu max content width 480 |
| `breakpoint.tablet` | 768 | Waiter (8″ portrait) |
| `breakpoint.tabletLg` | 1024 | Cashier landscape, Admin on tablet |
| `breakpoint.desktop` | 1280 | Admin Panel, HRIS |
| `breakpoint.wide` | 1920 | Kitchen display, Product Display |

## 3. Layout per surface

### Client Menu — phone, portrait
- Single column, 16 px gutters, max content width 480 px (centred on larger screens).
- Sticky header: Table label + language toggle + call-Waiter button.
- Sticky category bar under the header.
- Product grid: 2 columns of cards on ≥ 360 px; 1 column "feature" card for the first item of a Category.
- Cart button floats bottom-right above the safe-area inset; becomes a full-width bar when the cart has items.
- Sheets (Product detail, Cart, Rejoin prompt) slide up from the bottom with a 28 px top radius.

### Waiter — 8″ tablet, portrait, one hand
- Bottom tab bar (thumb reach): **Notifications · Tables · Orders · Bills**.
- Primary actions sit in the lower half of the screen; destructive actions never in the bottom corners.
- Lists: 64 px rows minimum.
- Notifications stack as cards with a Claim button 56 px tall.

### Cashier — 10–11″ tablet or PC, landscape
- Split view: **Bills list (left, 40 %)** · **Bill detail + PaymentPad (right, 60 %)**.
- Totals pinned at the bottom of the right pane; PaymentPad keys 64 px tall.

### Kitchen — 1920×1080 landscape, one screen per Prep Area
- Horizontal flow of KitchenTicket columns (360 px wide, 24 px gaps), oldest left; REMAKE tickets first.
- Top bar 72 px: Prep Area name, counts by status, clock.
- No scrolling inside a ticket; overflow tickets scroll horizontally with a visible "+N more" indicator.

### Admin Panel / HRIS — desktop ≥ 1280 px
- Left navigation rail 248 px (collapsible to 72 px), content max 1440 px with 48 px padding.
- 12-column grid, 24 px gutters; forms max 720 px wide.
- Tables: 48 px rows, sticky header, tabular numbers right-aligned.

### Product Display — TV 1920×1080 (4K safe)
- Slides are laid out on a stage whose short side is 1080 px, scaled to the screen. The long side stretches from 1440 to 2560, so 4:3 through 21:9 screens fill edge to edge with no letterbox; narrower landscape stages (< 1800) step the type down a size.
- A screen taller than it is wide (vertical TV, phone) gets portrait layouts: photo above text, Category Spotlight in 2 columns, uploaded media shown whole over a blurred copy.
- 64 px safe area on every edge (overscan).
- Layout grid: 12 columns, 32 px gutters. Templates in [MOTION.md §6](MOTION.md#6-product-display-banners).

## 4. Density

| Profile | Surface | Row height | Control height | Card padding |
|---|---|---|---|---|
| Comfortable | Customer | 56 | 48 | 20 |
| Compact | Waiter, Cashier | 64 (touch) | 48 | 16 |
| Dense | Admin, HRIS | 48 | 40 | 16 |
| Spacious | Kitchen | 72 | 64 | 20 |

## 5. Shape (radius)

| Token | px | Customer | Staff | Admin | Kitchen | Display |
|---|---|---|---|---|---|---|
| `radius.control` | | pill | 10 | 8 | 8 | pill |
| `radius.card` | | 20 | 12 | 12 | 8 | 28 |
| `radius.sheet` | | 28 | 20 | 20 | 12 | 28 |
| `radius.chip` | | pill | pill | pill | pill | pill |

Images inside cards inherit the card radius on their top corners. Avatars and the logo badge are always circles.

## 6. Elevation

Shadows are warm (Nori-tinted), never neutral grey.

| Token | Use | Surfaces |
|---|---|---|
| `shadow.sm` | Inputs on focus, chips that float | All |
| `shadow.md` | Product cards, floating cart button | Customer |
| `shadow.lg` | Sheets, popovers, toasts | All |
| `shadow.xl` | Dialogs, PIN Override | All |
| `shadow.glow` | Celebration moments only (Order sent, Ready) | Customer |

Staff surfaces are mostly flat: cards use `border.subtle` instead of shadows; only overlays (sheet, dialog, toast) cast shadows. In dark mode, elevation is shown by a lighter surface (`bg.surface` → `bg.raised`), not by shadow.

## 7. Z-index

`base 0 · sticky 10 · header 20 · overlay 40 · sheet 50 · dialog 60 · toast 70 · notification 80`. Claim notifications sit above dialogs so a Waiter never misses one; PIN Override dialogs are modal and block everything except notifications.
