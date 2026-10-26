# Typography

## 1. Typefaces

| Role | Family | Weights | Token | Why |
|---|---|---|---|---|
| **Display** | Poppins | 600, 700 | `font.family.display` | Continues the logo wordmark; friendly geometric shapes |
| **UI / body** | Plus Jakarta Sans | 400, 500, 600, 700, 800 | `font.family.ui` | Designed in Indonesia (Tokotype), excellent Indonesian text rendering, clear at small sizes, **tabular numerals** |
| **Mono** | JetBrains Mono | 400, 600 | `font.family.mono` | Bill numbers, codes, audit IDs, Receipt preview |

- All three are SIL Open Font License.
- **Self-hosted** via Fontsource packages (`@fontsource/poppins`, `@fontsource-variable/plus-jakarta-sans`, `@fontsource-variable/jetbrains-mono`), so they work offline and send no requests to Google.
- Subsets: `latin` and `latin-ext` only.
- `font-display: swap`; preload Plus Jakarta Sans 400/600 and, on customer surfaces, Poppins 700.
- Fallbacks: Segoe UI, Helvetica Neue, Arial, sans-serif (metrics close enough to avoid big layout shift).

### When to use which

| Use | Family |
|---|---|
| Product names, section titles and hero text on Client Menu and Product Display | Poppins |
| Page titles on staff surfaces | Plus Jakarta Sans 700 (Poppins is reserved for brand moments) |
| Everything else: labels, body, buttons, tables | Plus Jakarta Sans |
| Prices, totals, timers, quantities, counts | Plus Jakarta Sans with `font-variant-numeric: tabular-nums` |
| Bill numbers (`B-0427`), Crew IDs, audit IDs | JetBrains Mono |

## 2. Type scale

Named steps are shared; the pixel value depends on the surface ([tokens/surface.*.json](tokens/)). Sizes follow a ≈1.25 ratio rounded to the 2 px grid.

| Step | Customer (phone) | Staff (tablet) | Admin (desktop) | Kitchen | Display (TV) |
|---|---|---|---|---|---|
| `xs` | 12 | 12 | 12 | — | — |
| `sm` | 14 | 14 | 13 | 20 | — |
| `base` | **16** | **16** | **14** | **24** | **32** |
| `lg` | 20 | 20 | 16 | 30 | 40 |
| `xl` | 24 | 24 | 20 | 36 | 56 |
| `2xl` | 32 | 28 | 24 | 48 (timers) | 72 |
| `3xl` | — | — | 28 | — | 96 |
| `4xl` | — | — | — | — | 120 |
| `amountMd` / `amountLg` / `amountXl` | — | 24 / 32 / 40 | — | — | — |

### Text styles

| Style | Family / weight | Size step | Line height | Letter spacing | Example |
|---|---|---|---|---|---|
| `display` | Poppins 700 | `2xl`+ | tight 1.1 | tighter −0.02em | "Mau makan apa hari ini?" |
| `title` | Poppins 600 (customer) / Jakarta 700 (staff) | `xl` | snug 1.25 | tight −0.01em | Category header |
| `heading` | Jakarta 700 | `lg` | snug 1.25 | tight | Card title, ticket header |
| `body` | Jakarta 400 | `base` | normal 1.5 | normal | Descriptions |
| `bodyStrong` | Jakarta 600 | `base` | normal 1.5 | normal | Product name in lists |
| `label` | Jakarta 600 | `sm` | snug 1.25 | normal | Buttons, form labels, chips |
| `caption` | Jakarta 500 | `xs` | normal 1.5 | normal | Helper text, timestamps |
| `overline` | Jakarta 700 | `xs` | snug | caps 0.12em, uppercase | "SUSHI & COFFEE", section kickers |
| `price` | Jakarta 700, tabular | `lg` | snug | normal | Rp 38.000 |
| `amount` | Jakarta 800, tabular | `amountLg` | tight | tight | Payment total |
| `timer` | Jakarta 800, tabular | Kitchen `2xl` | tight | normal | 07:42 |
| `code` | JetBrains Mono 600 | `sm` | normal | normal | B-0427 |

## 3. Rules

1. **Body text never below 16 px on phones and tablets**, 14 px on desktop, 24 px on Kitchen.
2. **Numbers that change or compare are tabular**: prices, totals, timers, quantities, countdowns. They must not shift width while ticking.
3. Line length 45–75 characters for descriptions; cards truncate Product descriptions to 2 lines on the menu, full text in the detail sheet.
4. Indonesian strings run ~20–30 % longer than English. Every label must survive ID at the same size: allow wrapping or 2-line buttons on staff surfaces rather than shrinking text.
5. Uppercase only for `overline` and the "REMAKE"/"REPRINT"/"INTERNAL" markers.
6. Text scales with the user's browser font size (rem-based output); layouts must survive 200 % zoom.
7. Receipts (58 mm, 32 characters per line) use the printer's Font A; brand typefaces apply only to the on-screen Receipt preview.
