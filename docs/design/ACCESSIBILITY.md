# Accessibility

Target: **WCAG 2.2 level AA** on every surface. Staff surfaces add stricter ergonomics because people use them while carrying plates, wearing gloves, or with wet hands.

## 1. Targets

| Area | Requirement |
|---|---|
| Text contrast | ≥ 4.5:1 (≥ 3:1 only for text ≥ 24 px regular / 19 px bold). Proven in [COLOR.md §5](COLOR.md#5-contrast-proof) |
| Non-text contrast | Icons, input borders, focus rings, chart marks ≥ 3:1 |
| Touch targets | Phone ≥ 44 px · Tablet ≥ 48 px · Kitchen ≥ 64 px · Desktop ≥ 40 px (WCAG minimum is 24 px) |
| Spacing between targets | ≥ 8 px on touch surfaces |
| Colour independence | Every status = colour + icon + text label |
| Focus | Visible 2 px `border.focus` ring with 2 px offset on every focusable element; never removed |
| Keyboard | Admin, HRIS and Cashier (PC) fully operable by keyboard; logical tab order; Esc closes sheets/dialogs |
| Screen readers | Semantic HTML first; shadcn/Radix primitives for dialogs, menus, tabs; live regions for status changes |
| Zoom and text size | Layout survives 200 % zoom and browser font size up to 20 px on phone |
| Motion | `prefers-reduced-motion` respected ([MOTION.md §4](MOTION.md#4-reduced-motion)); no flashing > 3 per second |
| Language | `lang="id"` or `lang="en"` on `<html>`, switched with the language toggle; mixed-language strings marked with `lang` |
| Time limits | PIN Override token (60 s) and session expiry warn 30 s ahead and can be extended |

## 2. Surface-specific rules

### Client Menu
- Product images have `alt` text from the Product name ("Salmon nigiri, 2 pieces"); decorative illustrations `alt=""`.
- Dietary Warnings are announced (`role="status"`) and repeated in the acknowledgement dialog as text, not only as an icon.
- Sold Out items remain in the reading order with "Habis / Sold out" read before the name.
- Cart count changes announce politely ("3 item di keranjang").
- Estimated Wait updates announce at most once per minute.

### Waiter / Cashier
- New Claim notifications use sound + vibration + visual; sound volume and vibration can be adjusted, never fully muted by default.
- PIN pads randomise nothing (muscle memory matters) and accept hardware number keys on PC.
- Amounts are read with currency ("Rp tiga puluh delapan ribu") via `aria-label`.

### Kitchen
- Readable from 3 m: 24 px minimum text, 48 px timers, high-contrast dark theme.
- Late state = red border + "TERLAMBAT" label + timer colour, never colour alone.
- No reliance on sound alone; sound is optional per Prep Area.

### Product Display
- Text on imagery sits on a scrim (`bg.scrim` at `opacity.scrim`) to keep ≥ 4.5:1.
- Slides dwell ≥ 8 s so text can be read.

## 3. Checklist (per screen, before merge)

- [ ] All text and controls meet contrast (automated axe check passes).
- [ ] Every interactive element is reachable and operable by keyboard (PC surfaces) and has a visible focus ring.
- [ ] Touch targets meet the surface minimum.
- [ ] Status shown with icon + label, not colour only.
- [ ] Images have correct `alt`; decorative ones are hidden.
- [ ] Dynamic updates use appropriate live regions without spamming.
- [ ] Works in ID and EN without truncating meaning.
- [ ] Works with reduced motion.
- [ ] Works at 200 % zoom (desktop) / 20 px font size (phone).
- [ ] Forms: labels bound to inputs, errors described in text and linked with `aria-describedby`.

## 4. Testing

- Automated: axe-core in Playwright end-to-end tests for every route.
- Manual per release: TalkBack on Android Chrome (Client Menu, Waiter), NVDA on Windows (Admin, Cashier PC), keyboard-only pass on Admin.
- Kitchen readability test: view the display from 3 m in kitchen lighting.
