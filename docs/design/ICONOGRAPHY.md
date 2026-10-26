# Iconography and Illustration

## 1. UI icons: Lucide

- Library: **Lucide** (`lucide-react`), the shadcn/ui default.
- Grid 24 px, stroke 2 px, round caps and joins. Rendered at `size.icon.*`: 14 / 16 / 20 / 24 / 32 / 48 px.
- Stroke scales with size: 1.75 px at ≤ 16 px, 2 px at 20–24 px, 2.25 px at ≥ 32 px (Kitchen, Display).
- Colour: `currentColor`; status icons take `color.status.*.icon`.
- Icon-only buttons always have an accessible name (`aria-label`) and a tooltip on desktop.

### Domain mapping

| Concept | Lucide icon |
|---|---|
| Pending | `clock` |
| Confirmed | `check` |
| Preparing | `flame` |
| Ready | `bell-ring` |
| Served | `utensils` |
| Cancelled | `x` |
| Voided | `ban` |
| Sold Out | `circle-slash` |
| Settling | `hourglass` |
| Call Waiter | `hand` |
| Claim | `hand-helping` |
| Table | `armchair` |
| Bill | `receipt` |
| Split | `split` |
| Merge | `merge` |
| Move | `arrow-right-left` |
| Cash | `banknote` |
| QRIS | `qr-code` |
| Card | `credit-card` |
| Refund | `undo-2` |
| Remake | `rotate-ccw` |
| Reprint | `printer` |
| PIN Override | `shield-check` |
| Shift Closing | `vault` |
| Cash Handover | `hand-coins` |
| Clock in/out | `log-in` / `log-out` |
| Leave | `calendar-off` |
| Timer / Prep Time | `timer` |
| Language | `languages` |
| Offline | `wifi-off` |

## 2. Custom domain icons

Drawn on the same 24 px grid with 2 px rounded strokes so they sit beside Lucide without looking foreign. Stored as React components in `web/packages/ui/icons/` and as SVG in `docs/design/icons/` (to be drawn during implementation).

| Set | Icons |
|---|---|
| **Dietary Tags (13)** | dairy (milk carton), gluten (wheat ear), egg, fish, raw-fish (fish + snowflake), shellfish (shrimp), nuts, peanut, soy (soy bean pod), sesame (three seeds), pork (pig snout), alcohol (sake cup), spicy (chilli) |
| **Prep Areas** | kitchen (knife + board), bar (coffee cup + steam) |
| **Domain extras** | nigiri, maki, latte-cup, receipt-x (Unpaid Bill), table-occupied |

Dietary icons appear inside the Plum DietaryBadge when warning, and in neutral grey on Product detail ("Contains: …").

## 3. Imagery

### 3.1 Product and brand photography (primary)

The demo uses the mockup photos in [../images/](../images/) (catalogue in [../images/README.md](../images/README.md)); a live café would upload its own through the Admin Panel.

| Use | Ratio | Delivered as | Treatment |
|---|---|---|---|
| ProductCard, ProductDetailSheet, Admin catalog | **1:1** | WebP 320 / 640 / 1024 px, `srcset` + `sizes` | `object-fit: cover`, card top radius; lazy-loaded below the fold |
| Product Display slides | **16:9** | WebP 1920×1080 crop (centre-weighted, adjustable focal point per image) | Ken Burns zoom, scrim under text |
| Staff avatars | 1:1 circle | WebP 96 / 192 px | Face-centred crop; Crew without a portrait get an **initials avatar** (Poppins 600 on a Nori/Espresso/Salmon Deep tile picked from the Crew ID) |
| Marketing (hero, interior, flatlay, QR stand) | Original | WebP 1280 / 1920 px | Splash, demo launcher, Brand Idle slide |

Rules:
- Warm, natural light; dark slate or light oak surfaces; no visible third-party brand logos or real, identifiable people.
- Sold Out images are desaturated (greyscale, 45 % opacity), never removed.
- Images never carry text; prices and names are rendered by the UI.
- Every image has a Product-name `alt` text in the active language.

### 3.2 Illustration kit (fallback and decoration)

Used when a Product has no photo (branded placeholder: logo badge on Rice), for empty states and error pages, and for ambient decoration on customer surfaces.

### Style
- Flat shapes, no outlines except Nori-coloured details; soft inner shadows as darker tints of the fill (no gradients except the latte surface).
- Palette from tokens only: Nori, Rice, Espresso/Crema, Salmon scale, plus food tints (matcha green `#7FA65A`, tamago yellow `#F2C14E`, tuna red `#C7463F`, avocado `#9BBF5A`, cucumber `#5E9E6B`, chocolate `#5A3622`) added to the kit as `illustration.*` colours.
- Viewed from three-quarter above, centred on a Rice-coloured plate or saucer, 4:3 frame, 10 % padding.
- Each illustration is an SVG with named layers so motion can target them (`#steam`, `#shine`, `#garnish`).

### Kit parts (composable)
| Group | Parts |
|---|---|
| Vessels | round plate, long sushi plate (geta), ramen bowl, rice bowl, latte cup, glass tumbler, iced glass, dessert plate |
| Sushi | nigiri (salmon, tuna, tamago, ebi), maki roll cross-section (salmon, cucumber, avocado), uramaki with sesame, temaki cone, gunkan |
| Hot food | ramen (noodles, chashu, egg half, nori sheet, scallion), donburi toppings, gyoza |
| Drinks | latte surface with art (heart, tulip, rosetta), cappuccino foam, matcha latte, iced coffee with ice cubes, cold brew, ocha |
| Desserts | mochi, matcha cake slice, dorayaki, cheesecake |
| Garnish | wasabi dot, pickled ginger, sesame, scallion rings, chilli flakes, steam curls, sparkles, coffee beans |

### Motion hooks
| Layer | Loop |
|---|---|
| `#steam` | Rise and fade (hot drinks, ramen) — same keyframes as the logo |
| `#shine` | Soft highlight sweep every 6 s (sushi, glazed items) |
| `#ice` | Gentle bob (iced drinks) |
| `#garnish` | Pop-in on the detail sheet only |

## 4. Brand assets

Logo files and usage live in [../brand/](../brand/). Decorations from the logo (beans, sesame, sparkles, steam, dotted ring) are reusable on customer surfaces: empty states, splash, celebrations, Product Display backgrounds. Never on staff work screens; allowed on the staff login screen and empty states.
