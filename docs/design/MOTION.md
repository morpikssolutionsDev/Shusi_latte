# Motion

Motion explains what changed and, on customer surfaces, makes the food feel alive. It never slows anyone down.

## 1. Two profiles

| | **Expressive** | **Functional** |
|---|---|---|
| Surfaces | Client Menu, Product Display | Waiter, Cashier, Kitchen, Admin, HRIS |
| Durations | 300–900 ms (`moderate` → `hero`), celebrations 1200 ms | 120–200 ms (`fast`, `base`) |
| Easing | `expressive`, `overshoot`, springs `gentle` / `bouncy` | `standard`, `decelerate` (enter), `accelerate` (exit), spring `snappy` |
| Techniques | Springs, staggered reveals, shared-element transitions, parallax, ambient loops, celebrations | Fades, short slides, height changes, one attention pulse |
| Ambient loops | Yes (steam, sparkles, floating beans), paused off-screen | Never |
| Success feedback | Many small delights | Only Claim and Payment success |

## 2. Tokens

| Duration | ms | Use |
|---|---|---|
| `instant` | 80 | Press feedback, toggles |
| `fast` | 150 | Hover, chip change, small fades |
| `base` | 200 | Staff panel/sheet transitions |
| `moderate` | 300 | Customer card reveal, tab indicator |
| `slow` | 450 | Customer sheets, page transitions |
| `expressive` | 600 | Shared-element Product photo |
| `hero` | 900 | Splash logo, nigiri drop |
| `celebrate` | 1200 | Order sent, Ready |
| `loop` | 2800 | Steam and ambient cycles |
| `slide` | 8000 | Product Display slide dwell |

| Easing | Curve | Use |
|---|---|---|
| `standard` | `cubic-bezier(0.2, 0, 0, 1)` | Default for both profiles |
| `decelerate` | `cubic-bezier(0.05, 0.7, 0.1, 1)` | Elements entering |
| `accelerate` | `cubic-bezier(0.3, 0, 0.8, 0.15)` | Elements leaving |
| `expressive` | `cubic-bezier(0.22, 1, 0.36, 1)` | Customer slides and reveals |
| `overshoot` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | Pops, bounces (logo, cart badge) |

| Spring (Motion library) | Stiffness / damping / mass | Use |
|---|---|---|
| `gentle` | 170 / 26 / 1 | Sheets, shared elements |
| `bouncy` | 300 / 18 / 1 | Add-to-cart, badges, celebrations |
| `snappy` | 500 / 35 / 1 | Staff: drag, toggles, list reorder |

Stagger: 40 ms between siblings (customer), max 8 staggered items, then the rest appear together.

## 3. Technology

- **Motion** (formerly Framer Motion) for component animation, layout and shared-element transitions (`layoutId`), `AnimatePresence` for enter/exit.
- **Inline SVG + CSS keyframes** for brand illustrations and ambient loops (same technique as the animated logo). No Lottie.
- Animate only `transform` and `opacity` (plus `clip-path` / SVG `stroke-dashoffset` for reveals). Never animate layout properties directly; use Motion's `layout`.
- Ambient loops pause with `IntersectionObserver` when off-screen and on `visibilitychange`.
- **Budget:** 60 fps on a mid-range Android phone (Chrome); at most 3 ambient loops visible at once; total customer JS for motion ≤ 40 kB gzip.

## 4. Reduced motion

When `prefers-reduced-motion: reduce` is set, or the in-app "Reduce motion" setting is on:

- Expressive profile → 150 ms opacity fades only; no springs, parallax, flying items or ambient loops.
- Celebrations show their final still frame with the message.
- Product Display uses cross-fades and no Ken Burns zoom.
- Functional profile is unchanged except the attention pulse becomes a static border.

## 5. Client Menu signature moments

| # | Moment | Motion | Duration | Notes |
|---|---|---|---|---|
| M1 | First open | Animated logo splash (badge pop → cup rise → nigiri drop) then cross-fade to menu | ≤ 1.5 s, tap to skip | Only on first scan of a Bill; Rejoin skips it |
| M2 | Category bar | Pill indicator slides between categories (spring `gentle`); follows scroll position | 300 ms | Scroll-spy |
| M3 | Section reveal | Cards rise 16 px + fade, staggered 40 ms | 300 ms | Once per section per session |
| M4 | Product card → detail | Shared-element: photo/illustration expands into the sheet header (`layoutId`), sheet slides up | 600 ms | Hot drinks show the steam loop in the header |
| M5 | Choosing Variant/Modifiers | Price ticks to the new value (number roll); selected chip fills | 200 ms | |
| M6 | Add to order | Thumbnail flies along a curve into the cart button; cart count pops (`overshoot`) | 600 ms | Haptic tick (`navigator.vibrate(10)`) where supported |
| M7 | Dietary Warning | Plum badge eases in (fade + 4 px rise); dialog slides up calmly | 300 ms | No shake, no red, no bounce |
| M8 | Order sent | Cup fills with latte from the bottom; sparkles burst; "Pesanan terkirim!" | 1.2 s | Glow shadow |
| M9 | Waiting | **Cup-fill progress**: latte level = elapsed ÷ Estimated Wait; steam grows as Ready nears; Order Item chips change status with a cross-fade | Continuous | Level never exceeds 95 % until Ready |
| M10 | Ready | Nigiri drops onto the cup + "Siap! / Ready!" | 900 ms | Vibration pattern [30, 60, 30] |
| M11 | Sold Out while browsing | Card desaturates over 450 ms and the "Habis" label slides in | 450 ms | |
| M12 | Rejoin prompt, Settling, "continued on another device" | Calm sheet slide-up, no decoration | 450 ms | Serious moments are never playful |

## 6. Product Display banners

Template-driven slides render live data (price, Sold Out, promo times) with animated layers. Uploaded images/videos are also supported and get the same transitions.

| Template | Layers and motion |
|---|---|
| **Hero Promo** | Background illustration with slow parallax drift; headline slides in (`expressive`, 600 ms); price tag pops (`overshoot`) at 900 ms; salmon sparkles twinkle |
| **Featured Product** | Ken Burns: image scales 1.0 → 1.08 over the 8 s dwell; steam overlay on hot drinks; name + price slide up from the bottom third; Dietary Tag icons fade in |
| **Happy Hour Countdown** | Live countdown with tabular digits flipping per second; discounted price strikes through the old price at 1.2 s |
| **Category Spotlight** | 4–6 Product tiles reveal in a staggered grid (80 ms), each with a subtle float loop |
| **Brand Idle** | Full animated logo lockup, ambient loops only |

- Dwell 8 s (`duration.slide`), transition 1 s: cross-fade + 40 px slide in the direction of travel.
- A Product that becomes Sold Out is removed from rotation on the next transition (BR-PD-03), never mid-slide.
- Content updates arrive live; the current slide finishes before the new playlist starts.
- TV performance: pre-render the next slide off-screen 1 s before its transition; one ambient loop per slide maximum.

## 7. Functional profile patterns (staff)

| Pattern | Motion |
|---|---|
| New Claim notification | Slides in from the top (200 ms `decelerate`), then **one attention pulse** (border glow ×2, 1.2 s total). Sound + vibration. Re-notification at 2 min repeats the pulse once |
| Claim success | Card collapses into the Orders tab icon (200 ms) with a check tick |
| Another Waiter claimed it | Card fades out (150 ms) with "Diklaim oleh Budi" for 1.5 s |
| Kitchen new ticket | Slides in from the right (200 ms) and shows a 3 s highlight border |
| Kitchen late | Ticket border turns red; timer pulses once per minute (no continuous flashing) |
| Status change | Chip cross-fade 150 ms |
| Payment complete | Total ticks to "Lunas", check mark draws (stroke 300 ms), Receipt printing indicator |
| PIN entry error | Dots shake 4 px horizontally ×2 (the only shake allowed in the product) |
| Sheets / dialogs | 200 ms slide/fade; scrim fades 150 ms |

Never on staff surfaces: parallax, ambient loops, staggered reveals of lists longer than 5 rows, confetti.
