# Sushi Latte — Design Philosophy

> **Warm front, calm back.**
> Customers should feel invited to eat. Crew should feel in control during the rush.

Sushi Latte serves two audiences in very different states of mind. A Customer at a Table is relaxed, curious, and deciding what to eat. A Waiter carrying three plates, a Kitchen crew member with wet hands, and a Cashier at closing time are busy, interrupted, and must not make mistakes. One design language serves both by changing **how loudly it speaks**, not what it is made of.

## 1. Principles

### Appetite first
On customer surfaces the food is the hero. Imagery is large, warm and generous; interface chrome steps back. Motion makes food feel fresh: steam rises, sauces shine, the cup fills.

### Glanceable under pressure
On staff surfaces every screen answers "what needs me now?" in under a second. Status is shown with colour **and** icon **and** label. Numbers are large and tabular. Nothing decorative competes with a late ticket.

### One source of truth
Every colour, size, radius and timing comes from design tokens ([TOKENS.md](TOKENS.md)). Every word for a domain concept comes from the [Glossary](../../GLOSSARY.md). If the Kitchen calls it "Ready", the Waiter sees "Ready" in the same green with the same icon.

### Kind to mistakes
People tap the wrong thing. Actions that can be undone are undone easily; actions that cannot (Void, Unpaid Bill, Refund) ask for a PIN Override and say exactly what will happen. Error messages explain what to do next, never who is at fault. Dietary Warnings inform, never alarm or block.

## 2. Two levels of expression

The same tokens, components and grid are used everywhere. What changes is the **surface profile** ([tokens/surface.*.json](tokens/)):

| | Expressive (front) | Functional (back) |
|---|---|---|
| Surfaces | Client Menu, Product Display | Waiter, Cashier, Kitchen, Admin Panel, HRIS |
| Mood | Warm, crafted, playful, appetising | Calm, clear, dense, confident |
| Background | Rice (cream) on phone; Nori dark on TV | Neutral light; Nori dark on Kitchen |
| Primary action | Salmon Deep | Nori |
| Corners | Very soft: 20–28 px cards, pill buttons | Firm: 8–12 px cards, 10 px buttons |
| Imagery | Large illustrations/photos, ambient decoration | Only on login, empty states, success |
| Motion | Rich: springs, staggers, shared elements, ambient loops, celebrations | Functional: 120–200 ms, one attention pulse, success feedback only for Claim and Payment |
| Density | Comfortable | Compact (tablet), dense (desktop), spacious (Kitchen) |

## 3. Brand character

Taken from the logo ([../brand/](../brand/)): a salmon nigiri resting on a latte cup inside a café badge.

- **Crafted, not corporate.** Rounded shapes, warm off-white instead of pure white on customer surfaces, hand-feeling decoration (beans, sesame, sparkles, steam).
- **Two worlds, one table.** Sushi and coffee appear side by side: Nori and Rice for the sushi bar, Espresso and Crema for the coffee bar, Salmon as the bridge.
- **Playful with restraint.** Decoration lives at the edges and in moments of delight; it never covers content or controls.

## 4. Surfaces at a glance

| Surface | Device | Mode | Profile | Key feeling |
|---|---|---|---|---|
| Client Menu | Phone, portrait | Light only | Expressive | "I want to try that." |
| Product Display | TV 1920×1080 | Dark | Expressive | "That looks good — I'll order it." |
| Waiter | 8″ tablet, portrait, one hand | Light (dark available) | Functional | "I know what to do next." |
| Cashier | 10–11″ tablet or PC, landscape | Light (dark available) | Functional | "The numbers add up." |
| Kitchen | 1920×1080 screen | Dark | Functional | "I can see what's late from here." |
| Admin Panel, HRIS | Desktop ≥1280 px | Light (dark available) | Functional | "Everything is where I expect it." |

## 5. What we never do

- Use colour as the only signal of state.
- Use Salmon `#F2885B` for text or button labels (contrast fails); use Salmon Deep.
- Use Plum for anything other than Dietary Warnings.
- Animate anything on staff surfaces just because it is pretty.
- Block ordering because of a Dietary Warning.
- Hide a price, tax or service charge until the end.
- Show decoration on the Kitchen display.

## 6. Documents

| Document | Contents |
|---|---|
| [TOKENS.md](TOKENS.md) | Token architecture, naming, pipeline |
| [COLOR.md](COLOR.md) | Scales, semantic roles, status colours, contrast proof |
| [TYPOGRAPHY.md](TYPOGRAPHY.md) | Typefaces, type scales per surface, numerals |
| [LAYOUT.md](LAYOUT.md) | Spacing, grid, breakpoints, density, radius, elevation |
| [MOTION.md](MOTION.md) | Motion profiles, timing, signature moments, Product Display animation |
| [ICONOGRAPHY.md](ICONOGRAPHY.md) | Icons and the illustration kit |
| [COMPONENTS.md](COMPONENTS.md) | Component specifications |
| [ACCESSIBILITY.md](ACCESSIBILITY.md) | Targets and checklists |
| [CONTENT.md](CONTENT.md) | Voice, tone, ID/EN writing, formats |
| [styleguide.html](styleguide.html) | Living visual reference of tokens and key components |
