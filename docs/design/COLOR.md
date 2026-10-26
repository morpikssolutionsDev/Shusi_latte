# Colour

Colours are defined in [tokens/primitive.json](tokens/primitive.json) and given meaning in [tokens/semantic.light.json](tokens/semantic.light.json) / [tokens/semantic.dark.json](tokens/semantic.dark.json). Scales are generated in OKLCH so equal steps look equally far apart, with the exact logo colours locked to their nearest step.

## 1. Brand colours

| Name | Hex | Token | Use |
|---|---|---|---|
| Nori | `#1E2B25` | `color.nori.900` | Staff primary, text, cup, nori |
| Nori Light | `#2E4238` | `color.nori.800` | Highlights on dark |
| Rice | `#F7F0E4` | `color.rice.100` | Customer surfaces, badge |
| Espresso | `#6F4428` | `color.espresso.700` | Prices, "latte", coffee |
| Crema | `#8C5634` | `color.espresso.600` | Coffee highlight |
| Salmon | `#F2885B` | `color.salmon.400` | **Decorative only** (fails text contrast on light) |
| Salmon Deep | `#B9512B` | `color.salmon.600` | Customer primary action, salmon text |
| Steam | `#C9B79C` | `color.brand.steam` | Steam, muted illustration lines |

## 2. Scales

| Scale | 50 | 100 | 200 | 300 | 400 | 500 | 600 | 700 | 800 | 900 | 950 | Role |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **nori** | `#F2F9F5` | `#E5F0EA` | `#CBDFD5` | `#ABC7BA` | `#8BAC9C` | `#6F9181` | `#57796A` | `#436154` | `#2E4238` | `#1E2B25` | `#12221B` | Brand dark: staff primary, text, logo |
| **neutral** | `#F5F7F6` | `#EAEEEC` | `#D5DAD8` | `#BAC1BE` | `#9DA5A1` | `#818A86` | `#6A726E` | `#545B58` | `#3F4542` | `#2B302E` | `#1B1F1D` | Staff surfaces, borders, secondary text |
| **rice** | `#FAF6F1` | `#F7F0E4` | `#E2D8C7` | `#CBBEA6` | `#B1A186` | `#968569` | `#7E6E52` | `#65573E` | `#4E422D` | `#372E1D` | `#241D10` | Customer surfaces, warm light |
| **espresso** | `#FFF4EE` | `#FFE8DA` | `#F7D0B8` | `#E8B291` | `#D2926B` | `#B7764C` | `#8C5634` | `#6F4428` | `#653614` | `#492408` | `#321502` | Coffee, prices, "latte" wordmark |
| **salmon** | `#FFF4F0` | `#FFE7DE` | `#FFCCB8` | `#FFA681` | `#F2885B` | `#D4602A` | `#B9512B` | `#953700` | `#732900` | `#521B00` | `#380F00` | Brand accent; 600 = Salmon Deep (customer primary) |
| **amber** | `#FFF5E8` | `#FFE9CC` | `#FCD197` | `#EEB35C` | `#D99302` | `#B57A00` | `#976500` | `#795000` | `#5D3C00` | `#422900` | `#2C1A00` | Pending, Prep Time warning, feedback warning |
| **blue** | `#F2F7FF` | `#E1EEFF` | `#C0DCFF` | `#93C3FF` | `#5DA5FF` | `#3B88E5` | `#1F6FCA` | `#0758AA` | `#004285` | `#002E60` | `#001D42` | Confirmed, Settling, info, focus, links |
| **indigo** | `#F5F6FF` | `#E9EBFF` | `#D1D6FF` | `#B2B9FF` | `#9296FE` | `#7779E5` | `#6160CA` | `#4C4AA9` | `#393687` | `#272464` | `#181546` | Preparing |
| **green** | `#EBFCED` | `#D7F7DC` | `#B3E9BD` | `#86D597` | `#59BB72` | `#30A054` | `#00873C` | `#006C2F` | `#005322` | `#003A16` | `#00260C` | Ready, on time, success |
| **red** | `#FFF4F2` | `#FFE6E3` | `#FFCBC4` | `#FFA398` | `#FF6F63` | `#E34B43` | `#C72E2B` | `#A61619` | `#83020A` | `#5F0005` | `#410002` | Voided, late, Unpaid, danger |
| **plum** | `#FFF2FC` | `#FFE3F8` | `#FCC5EF` | `#EEA2DD` | `#D87FC6` | `#BD61AB` | `#A34892` | `#863478` | `#69235D` | `#4C1543` | `#34092D` | Dietary Warning only |

## 3. Semantic roles

| Role | Light | Dark |
|---|---|---|
| `bg.canvas` (staff) | Neutral 50 | Nori 950 |
| `bg.canvasWarm` (customer) | Rice 50 | Nori 950 |
| `bg.surface` | White | Nori 900 |
| `bg.raised` | White + shadow | Nori 800 |
| `bg.sunken` | Neutral 100 | Neutral 950 |
| `text.primary` | Nori 900 | Rice 50 |
| `text.secondary` | Neutral 700 | Neutral 300 |
| `text.muted` | Neutral 600 (lightest allowed for text) | Neutral 400 |
| `text.brand` | Espresso 700 | Espresso 300 |
| `text.link` | Blue 700 | Blue 300 |
| `border.subtle` / `default` / `input` | Neutral 200 / 300 / 500 | Nori 800 / Neutral 700 / 500 |
| `border.focus` | Blue 600 | Blue 400 |
| `action.brand` (customer, display) | Salmon 600 → hover 700 → pressed 800, white label | Salmon 400, Nori 950 label |
| `action.neutral` (staff) | Nori 900 → 800 → 950, white label | Rice 100, Nori 950 label |
| `action.secondary` | Neutral 100, Nori 900 label | Nori 800, Rice 50 label |
| `action.danger` | Red 600, white label | Red 400, Red 950 label |

## 4. Status and domain colours

Colour is **never** the only signal: every status also has an icon ([ICONOGRAPHY.md](ICONOGRAPHY.md)) and a text label.

| Meaning | Hue | Icon | Notes |
|---|---|---|---|
| Order Item **Pending** | Amber | `clock` | Waiting for Claim |
| **Confirmed** | Blue | `check` | Sent to Kitchen |
| **Preparing** | Indigo | `flame` | Kitchen working |
| **Ready** | Green | `bell-ring` | Waiting pickup |
| **Served** | Neutral | `utensils` | |
| **Cancelled** | Neutral, label struck through | `x` | |
| **Voided** | Red | `ban` | Audit-logged |
| Prep Time on time / ≥80 % / late | Green / Amber / Red | `timer` | Kitchen ticket border + timer |
| **Dietary Warning** | **Plum** | custom tag icon + `alert-circle` | Plum is used for nothing else |
| **Sold Out** | Greyscale image at 45 % opacity + Nori label "Habis · Sold out" | `circle-slash` | |
| Bill **Settling** | Blue outline | `hourglass` | |
| **Unpaid Bill** | Red outline | `receipt-x` (custom) | |
| Remake | Salmon 400 ticket border + "REMAKE" label | `rotate-ccw` | Decorative salmon allowed: the label carries the meaning |
| Feedback success / warning / danger / info | Green / Amber / Red / Blue | `check-circle` / `alert-triangle` / `x-circle` / `info` | Toasts, banners |

Status tone roles (`bg`, `fg`, `border`, `icon`, `solid`, `onSolid`) are described in [TOKENS.md §4](TOKENS.md#4-status-tone-pattern).

## 5. Contrast proof

Computed with the WCAG 2.x relative-luminance formula from the token values. Text needs 4.5:1; UI components, icons and focus rings need 3:1.

### Light

| Pair | Colours | Ratio | Needs | Pass |
|---|---|---|---|---|
| Text primary on white | `#1E2B25` on `#FFFFFF` | 14.71 | 4.5:1 | ✅ |
| Text primary on Rice 50 | `#1E2B25` on `#FAF6F1` | 13.67 | 4.5:1 | ✅ |
| Text secondary on Rice 50 | `#545B58` on `#FAF6F1` | 6.47 | 4.5:1 | ✅ |
| Text muted on Rice 50 | `#6A726E` on `#FAF6F1` | 4.60 | 4.5:1 | ✅ |
| Text muted on white | `#6A726E` on `#FFFFFF` | 4.95 | 4.5:1 | ✅ |
| Price (text.brand) on Rice 50 | `#6F4428` on `#FAF6F1` | 7.70 | 4.5:1 | ✅ |
| Link on white | `#0758AA` on `#FFFFFF` | 7.03 | 4.5:1 | ✅ |
| Input border on white | `#818A86` on `#FFFFFF` | 3.55 | 3:1 | ✅ |
| Focus ring on white | `#1F6FCA` on `#FFFFFF` | 5.02 | 3:1 | ✅ |
| Customer primary label | `#FFFFFF` on `#B9512B` | 4.91 | 4.5:1 | ✅ |
| Staff primary label | `#FFFFFF` on `#1E2B25` | 14.71 | 4.5:1 | ✅ |
| Danger label | `#FFFFFF` on `#C72E2B` | 5.45 | 4.5:1 | ✅ |
| Pending chip text | `#5D3C00` on `#FFE9CC` | 8.42 | 4.5:1 | ✅ |
| Pending solid label | `#FFFFFF` on `#976500` | 5.03 | 4.5:1 | ✅ |
| Confirmed chip text | `#004285` on `#E1EEFF` | 8.44 | 4.5:1 | ✅ |
| Confirmed solid label | `#FFFFFF` on `#1F6FCA` | 5.02 | 4.5:1 | ✅ |
| Preparing chip text | `#393687` on `#E9EBFF` | 8.67 | 4.5:1 | ✅ |
| Preparing solid label | `#FFFFFF` on `#6160CA` | 5.21 | 4.5:1 | ✅ |
| Ready chip text | `#005322` on `#D7F7DC` | 8.06 | 4.5:1 | ✅ |
| Ready solid label | `#FFFFFF` on `#00873C` | 4.63 | 4.5:1 | ✅ |
| Served/Cancelled chip text | `#3F4542` on `#EAEEEC` | 8.38 | 4.5:1 | ✅ |
| Served/Cancelled solid label | `#FFFFFF` on `#6A726E` | 4.95 | 4.5:1 | ✅ |
| Voided chip text | `#83020A` on `#FFE6E3` | 8.94 | 4.5:1 | ✅ |
| Voided solid label | `#FFFFFF` on `#C72E2B` | 5.45 | 4.5:1 | ✅ |
| Dietary Warning chip text | `#69235D` on `#FFE3F8` | 8.80 | 4.5:1 | ✅ |
| Dietary Warning solid label | `#FFFFFF` on `#A34892` | 5.39 | 4.5:1 | ✅ |

### Dark (Kitchen, Product Display, dark staff theme)

| Pair | Colours | Ratio | Needs | Pass |
|---|---|---|---|---|
| Text primary on Nori 950 | `#FAF6F1` on `#12221B` | 15.36 | 4.5:1 | ✅ |
| Text primary on Nori 900 | `#FAF6F1` on `#1E2B25` | 13.67 | 4.5:1 | ✅ |
| Text secondary on Nori 900 | `#BAC1BE` on `#1E2B25` | 8.03 | 4.5:1 | ✅ |
| Text muted on Nori 900 | `#9DA5A1` on `#1E2B25` | 5.84 | 4.5:1 | ✅ |
| Input border on Nori 900 | `#818A86` on `#1E2B25` | 4.14 | 3:1 | ✅ |
| Focus ring on Nori 900 | `#5DA5FF` on `#1E2B25` | 5.81 | 3:1 | ✅ |
| Staff primary label (dark) | `#12221B` on `#F7F0E4` | 14.59 | 4.5:1 | ✅ |
| Customer/Display primary label (dark) | `#12221B` on `#F2885B` | 6.65 | 4.5:1 | ✅ |
| Pending chip text | `#FCD197` on `#2C1A00` | 11.72 | 4.5:1 | ✅ |
| Pending ticket header label | `#2C1A00` on `#D99302` | 6.47 | 4.5:1 | ✅ |
| Confirmed chip text | `#C0DCFF` on `#001D42` | 11.90 | 4.5:1 | ✅ |
| Confirmed ticket header label | `#001D42` on `#5DA5FF` | 6.61 | 4.5:1 | ✅ |
| Preparing chip text | `#D1D6FF` on `#181546` | 11.97 | 4.5:1 | ✅ |
| Preparing ticket header label | `#181546` on `#9296FE` | 6.49 | 4.5:1 | ✅ |
| Ready chip text | `#B3E9BD` on `#00260C` | 11.93 | 4.5:1 | ✅ |
| Ready ticket header label | `#00260C` on `#59BB72` | 6.84 | 4.5:1 | ✅ |
| Voided / late chip text | `#FFCBC4` on `#410002` | 11.91 | 4.5:1 | ✅ |
| Voided / late ticket header label | `#410002` on `#FF6F63` | 6.29 | 4.5:1 | ✅ |
| Dietary Warning chip text | `#FCC5EF` on `#34092D` | 11.75 | 4.5:1 | ✅ |
| Dietary Warning ticket header label | `#34092D` on `#D87FC6` | 6.35 | 4.5:1 | ✅ |

### Forbidden combinations

| Pair | Colours | Ratio | Needs | Pass |
|---|---|---|---|---|
| Logo Salmon as text on white (forbidden) | `#F2885B` on `#FFFFFF` | 2.49 | 4.5:1 | ❌ |
| Neutral 500 as text on white (forbidden) | `#818A86` on `#FFFFFF` | 3.55 | 4.5:1 | ❌ |
| Neutral 400 as input border (forbidden) | `#9DA5A1` on `#FFFFFF` | 2.52 | 3:1 | ❌ |

## 6. Rules

1. Logo Salmon `#F2885B` is decoration only: illustrations, sparkles, Remake ticket border, tagline on dark. Never text or a button label on light.
2. Plum = Dietary Warning. Nothing else may be purple or pink.
3. Customer surfaces use Rice, not pure white, as the canvas; cards are white so food pops.
4. Kitchen and Product Display are dark by default; status colours use the dark tone set.
5. Prices are Espresso on customer surfaces and Nori (`text.primary`) on staff surfaces.
6. Images of Sold Out items are desaturated, never hidden, so Customers learn what exists.
