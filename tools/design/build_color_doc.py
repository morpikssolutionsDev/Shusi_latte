import json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import palette as p

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
S = p.SCALES
W = '#FFFFFF'
steps = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]
roles = {'nori': 'Brand dark: staff primary, text, logo', 'neutral': 'Staff surfaces, borders, secondary text',
         'rice': 'Customer surfaces, warm light', 'espresso': 'Coffee, prices, "latte" wordmark',
         'salmon': 'Brand accent; 600 = Salmon Deep (customer primary)', 'amber': 'Pending, Prep Time warning, feedback warning',
         'blue': 'Confirmed, Settling, info, focus, links', 'indigo': 'Preparing', 'green': 'Ready, on time, success',
         'red': 'Voided, late, Unpaid, danger', 'plum': 'Dietary Warning only'}
scale_rows = '\n'.join(f"| **{n}** | " + ' | '.join(f'`{S[n][s]}`' for s in steps) + f" | {roles[n]} |" for n in S)


def row(label, fg, bg, need):
    c = p.contrast(fg, bg)
    return f"| {label} | `{fg}` on `{bg}` | {c:.2f} | {need}:1 | {'✅' if c >= need else '❌'} |"


L = [row('Text primary on white', S['nori'][900], W, 4.5), row('Text primary on Rice 50', S['nori'][900], S['rice'][50], 4.5),
     row('Text secondary on Rice 50', S['neutral'][700], S['rice'][50], 4.5), row('Text muted on Rice 50', S['neutral'][600], S['rice'][50], 4.5),
     row('Text muted on white', S['neutral'][600], W, 4.5), row('Price (text.brand) on Rice 50', S['espresso'][700], S['rice'][50], 4.5),
     row('Link on white', S['blue'][700], W, 4.5), row('Input border on white', S['neutral'][500], W, 3),
     row('Focus ring on white', S['blue'][600], W, 3), row('Customer primary label', W, S['salmon'][600], 4.5),
     row('Staff primary label', W, S['nori'][900], 4.5), row('Danger label', W, S['red'][600], 4.5)]
for k, h in [('Pending', 'amber'), ('Confirmed', 'blue'), ('Preparing', 'indigo'), ('Ready', 'green'),
             ('Served/Cancelled', 'neutral'), ('Voided', 'red'), ('Dietary Warning', 'plum')]:
    L.append(row(f'{k} chip text', S[h][800], S[h][100], 4.5))
    L.append(row(f'{k} solid label', W, S[h][600], 4.5))
D = [row('Text primary on Nori 950', S['rice'][50], S['nori'][950], 4.5), row('Text primary on Nori 900', S['rice'][50], S['nori'][900], 4.5),
     row('Text secondary on Nori 900', S['neutral'][300], S['nori'][900], 4.5), row('Text muted on Nori 900', S['neutral'][400], S['nori'][900], 4.5),
     row('Input border on Nori 900', S['neutral'][500], S['nori'][900], 3), row('Focus ring on Nori 900', S['blue'][400], S['nori'][900], 3),
     row('Staff primary label (dark)', S['nori'][950], S['rice'][100], 4.5),
     row('Customer/Display primary label (dark)', S['nori'][950], S['salmon'][400], 4.5)]
for k, h in [('Pending', 'amber'), ('Confirmed', 'blue'), ('Preparing', 'indigo'), ('Ready', 'green'),
             ('Voided / late', 'red'), ('Dietary Warning', 'plum')]:
    D.append(row(f'{k} chip text', S[h][200], S[h][950], 4.5))
    D.append(row(f'{k} ticket header label', S[h][950], S[h][400], 4.5))
F = [row('Logo Salmon as text on white (forbidden)', S['salmon'][400], W, 4.5),
     row('Neutral 500 as text on white (forbidden)', S['neutral'][500], W, 4.5),
     row('Neutral 400 as input border (forbidden)', S['neutral'][400], W, 3)]
hdr = '| Pair | Colours | Ratio | Needs | Pass |\n|---|---|---|---|---|\n'
NL = '\n'

doc = f'''# Colour

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
{scale_rows}

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

{hdr}{NL.join(L)}

### Dark (Kitchen, Product Display, dark staff theme)

{hdr}{NL.join(D)}

### Forbidden combinations

{hdr}{NL.join(F)}

## 6. Rules

1. Logo Salmon `#F2885B` is decoration only: illustrations, sparkles, Remake ticket border, tagline on dark. Never text or a button label on light.
2. Plum = Dietary Warning. Nothing else may be purple or pink.
3. Customer surfaces use Rice, not pure white, as the canvas; cards are white so food pops.
4. Kitchen and Product Display are dark by default; status colours use the dark tone set.
5. Prices are Espresso on customer surfaces and Nori (`text.primary`) on staff surfaces.
6. Images of Sold Out items are desaturated, never hidden, so Customers learn what exists.
'''
open(os.path.join(ROOT, 'docs', 'design', 'COLOR.md'), 'w', encoding='utf-8', newline='\n').write(doc)
print('written')
