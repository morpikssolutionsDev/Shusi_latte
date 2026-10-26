# Sushi Latte — Brand

## Concept

A salmon nigiri resting on a steaming latte cup, inside a round badge. Sushi and coffee are both shown literally, so the logo reads at a glance:

| Element | Meaning |
|---|---|
| Salmon nigiri with nori band | Sushi |
| Latte cup on saucer, crema surface, rising steam | Coffee |
| Salmon band with rice-dot pattern on the cup | The two worlds joined |
| Coffee beans, black sesame, salmon sparkles | Decoration (sushi + coffee ingredients) |
| Cream badge with dotted ring | Stamp / café seal |

## Files

| File | Animated | Use |
|---|:-:|---|
| [sushi-latte-logo-animated.svg](sushi-latte-logo-animated.svg) | ✔ | Brand mark for splash screen, login, Product Display idle slide |
| [sushi-latte-lockup-animated.svg](sushi-latte-lockup-animated.svg) | ✔ | Mark + wordmark + tagline for wide hero areas, showcase landing page |
| [sushi-latte-logo.svg](sushi-latte-logo.svg) | | Static mark: headers, docs, anywhere motion is distracting |
| [sushi-latte-lockup.svg](sushi-latte-lockup.svg) | | Static lockup |
| [sushi-latte-app-icon.svg](sushi-latte-app-icon.svg) | | Nori tile without small decorations: PWA icon, favicon, dark backgrounds |
| [sushi-latte-mono.svg](sushi-latte-mono.svg) | | Black and white for 58 mm thermal Receipts; recolour via CSS `color` |

The static and lockup files are generated from `sushi-latte-logo-animated.svg` (the artwork between `<!-- ART START -->` and `<!-- ART END -->`). Edit the artwork there first, then regenerate the others.

## Animation

Pure CSS inside the SVG, no JavaScript, so it plays when used as `<img src>`, CSS `background-image`, or inline.

| Time | Intro (plays once) |
|---|---|
| 0.0 s | Badge pops in |
| 0.25 s | Cup and saucer rise |
| 0.7 s | Nigiri drops onto the cup with a bounce |
| 1.1 s | Wordmark slides in (lockup only) |
| 1.35 s | Tagline slides in, rule draws, dots pop (lockup only) |
| 1.5 s | Coffee beans and sesame seeds pop in |

| Loop | |
|---|---|
| Steam | Three curls rise and fade, staggered, 2.8 s cycle |
| Coffee beans | Gentle float, 3.2 s cycle |
| Sparkles | Twinkle and turn, 2.4 s cycle |
| Dotted ring | Slow rotation, 40 s per turn |

`prefers-reduced-motion: reduce` disables all animation and shows the finished still frame.

To replay the intro in the app, re-mount the element (e.g. change the `<img>` `src` query string or `key`).

## Palette

| Name | Hex | Role |
|---|---|---|
| Nori | `#1E2B25` | Primary dark: cup, nori, outlines, text |
| Nori Light | `#2E4238` | Highlights on dark surfaces |
| Rice | `#F7F0E4` | Light surface, badge |
| Rice White | `#FFFCF5` | Sushi rice |
| Steam | `#C9B79C` | Steam, muted lines |
| Espresso | `#6F4428` | Coffee, beans, "latte" in the wordmark |
| Crema | `#8C5634` | Coffee highlight |
| Salmon | `#F2885B` | Accent: salmon, cup band, tagline, sparkles |
| Salmon Fat | `#FFD9C7` | Salmon stripes |

Do not reuse Salmon for Dietary Warnings in the UI; give warnings their own colour so they never look decorative.

## Typography

The wordmark is set lowercase in a bold geometric sans: **Poppins Bold** (SIL Open Font License) is the intended face, with Nunito, Segoe UI, Helvetica Neue and Arial as fallbacks. `textLength` keeps the lockup proportions identical whichever font renders.

Before using the lockups outside the app (print, merch, signage), convert the text to outlines, e.g. Inkscape *Path → Object to Path*.

## Usage

- Clear space: at least the height of the nigiri on every side.
- Minimum size: badge 64 px (below that use the app icon); lockup 240 px wide.
- On dark or photographic backgrounds use the app icon tile.
- On Receipts, print the mono mark about 120 px wide above the outlet header.
- Do not recolour layers, separate the nigiri from the cup, or crop the badge ring.
