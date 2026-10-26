# Sushi Latte — Brand Assets & Media Catalog

This directory contains the brand identity vectors, photorealistic product prototype images, staff & management portraits, and marketing/operational visual resources for the **Sushi Latte** HORECA platform.

---

## 1. Brand Logo Suite (Vector / SVG)

All brand vector assets are crafted in pure SVG with semantic layer IDs, allowing interactive animations (e.g. Framer Motion, GSAP, CSS hover transitions) such as rising steam wisps, heart pulses, and enso rotation.

| Asset File | Format | Resolution / ViewBox | Target Application / Module | Description |
|---|---|---|---|---|
| [`sushi-latte-mark.svg`](sushi-latte-mark.svg) | SVG | `500 × 500` | App icon, Favicon, PWA Splash, Avatar | Standalone fusion emblem: Zen Enso + Nori Ring + Ceramic Cup + Salmon Heart / Latte Art Core + Rising Steam |
| [`sushi-latte-horizontal.svg`](sushi-latte-horizontal.svg) | SVG | `850 × 220` | Client Menu Navbar, Admin Panel Header, TV Top-Bar | Full horizontal lockup featuring the mark, Japanese script (`鮨 × 珈琲・ラテ`), modern serif wordmark, and tagline |
| [`sushi-latte-vertical.svg`](sushi-latte-vertical.svg) | SVG | `450 × 560` | Table QR Cards, Menu Covers, Posters, Mobile Splash | Stacked badge with centered typography, geometric divider, and service platform badge |
| [`sushi-latte-dark.svg`](sushi-latte-dark.svg) | SVG | `850 × 220` | Kitchen Station KDS TV, Dark UI Modes | High-contrast dark mode vector with subtle coral luminous glow |
| [`sushi-latte-thermal-monochrome.svg`](sushi-latte-thermal-monochrome.svg) | SVG | `384 × 200` (58mm ESC/POS) | Waiter & Cashier Bluetooth Thermal Printers | Pure 1-bit high-contrast monochrome vector engineered specifically for 58 mm thermal receipt printing (BRD-WT-08, BRD-CS-01, BRD-R-21) |
| [`logo-interactive-preview.html`](logo-interactive-preview.html) | HTML5 | Responsive | Development & Design Testing | Live interactive testbed with toggleable steam animations, dark/light theme switch, and order pulse wave simulations |

### Interactive Layer IDs for Animation / Framer Motion
- `#sl-steam-paths` / `#sl-steam-1`, `#sl-steam-2`, `#sl-steam-3`: Steam wave elevation & opacity pulses.
- `#sl-enso-ring`: Outer nori curve for rotation or stroke dash offset draws.
- `#sl-core-heart`: Center salmon-latte heart for subtle pulse or bounce on order interactions.
- `#sl-cup-vessel`: Ceramic cup base.

---

## 2. Staff & Management Team Portraits

Photorealistic character portraits designed for the **HRIS Module** (staff directory, shift schedules, clock-in avatars), **Waiter/Server Module** (server profile), and **Admin Panel** (manager override authorization).

| Portrait File | Role / Station | Uniform & Attire | Pose & Expression | Target Application |
|---|---|---|---|---|
| [**`staff-waiter-male.jpg`**](staff-waiter-male.jpg) | Waiter / Service Crew | Modern charcoal cafe apron over a crisp beige linen shirt | Friendly welcoming smile directly facing camera, holding an order tablet in hand | Waiter profile avatar, order claim badge, HRIS attendance |
| [**`staff-waitress-female.jpg`**](staff-waitress-female.jpg) | Waitress / Cashier Crew | Minimalist cafe barista apron over service shirt | Cheerful happy smile facing camera, holding a hot coffee cup | Waiter/Cashier profile avatar, shift schedule card, HRIS |
| [**`staff-store-manager.jpg`**](staff-store-manager.jpg) | Outlet Admin / Store Manager | Tailored black business suit, crisp collared shirt & tie | Confident, poised, approachable professional management posture | Admin Panel header, PIN Override prompt, Management reports |

---

## 3. Photorealistic Dummy Product Menu Images (18 Items)

All images are formatted in `1:1` square aspect ratio (`1000 × 1000`) for the **Client Menu**, **Admin Panel Catalog**, and **Product Display Showcase**. Tagged according to BRD Prep Areas (**Kitchen** vs **Bar**) and Dietary Tags (**BRD-PC-06**).

### A. Sushi, Sashimi & Rolls (Kitchen Prep Area)
| Product Name | File | Category | Prep Area | Dietary Tags | Description |
|---|---|---|---|---|---|
| **Salmon Aburi Nigiri** | [`salmon-aburi-nigiri.jpg`](salmon-aburi-nigiri.jpg) | Sushi | Kitchen | `Raw Fish`, `Egg`, `Soy` | Seared salmon nigiri torched with kewpie mayo, sweet tare glaze, and fresh microgreens on dark slate |
| **Dragon Roll Maki** | [`dragon-roll-maki.jpg`](dragon-roll-maki.jpg) | Sushi & Rolls | Kitchen | `Fish`, `Shellfish`, `Soy`, `Gluten` | Fresh sliced avocado scales, grilled unagi eel interior, orange tobiko roe, and kabayaki drizzle |
| **Spicy Salmon Roll** | [`spicy-salmon-roll.jpg`](spicy-salmon-roll.jpg) | Sushi & Rolls | Kitchen | `Raw Fish`, `Egg`, `Soy`, `Spicy` | Spicy fresh salmon maki drizzled with sriracha mayo, black sesame seeds, and golden tempura crunch |
| **Unagi Kabayaki Nigiri** | [`unagi-kabayaki-nigiri.jpg`](unagi-kabayaki-nigiri.jpg) | Sushi | Kitchen | `Fish`, `Soy`, `Gluten` | Grilled freshwater eel brushed with caramelized tare glaze, wrapped with crisp nori belt |
| **Tuna Sashimi Platter** | [`tuna-sashimi-platter.jpg`](tuna-sashimi-platter.jpg) | Sashimi | Kitchen | `Raw Fish` | Thick, glistening bluefin tuna akami and salmon sashimi with fresh wasabi and shiso over crushed ice |

### B. Japanese Hot Kitchen & Appetizers (Kitchen Prep Area)
| Product Name | File | Category | Prep Area | Dietary Tags | Description |
|---|---|---|---|---|---|
| **Wagyu Beef Donburi** | [`wagyu-beef-donburi.jpg`](wagyu-beef-donburi.jpg) | Hot Dishes | Kitchen | `Egg`, `Soy`, `Gluten` | Medium-rare sliced Wagyu steak over steamed Japanese rice, topped with golden onsen egg and garlic chips |
| **Chicken Katsu Curry** | [`chicken-katsu-curry.jpg`](chicken-katsu-curry.jpg) | Hot Dishes | Kitchen | `Gluten`, `Egg`, `Soy` | Panko breaded chicken cutlet with rich Japanese aromatic curry gravy over pearl rice |
| **Tonkotsu Ramen Bowl** | [`tonkotsu-ramen-bowl.jpg`](tonkotsu-ramen-bowl.jpg) | Hot Dishes | Kitchen | `Pork`, `Egg`, `Gluten`, `Soy` | Signature rich pork bone broth ramen with tender rolled chashu, ramen egg, menma bamboo shoots, and nori |
| **Crispy Gyoza Dumplings** | [`crispy-gyoza-dumplings.jpg`](crispy-gyoza-dumplings.jpg) | Appetizers | Kitchen | `Pork`, `Gluten`, `Soy`, `Sesame` | Pan-fried Japanese dumplings with golden crispy lace skirt, served with seasoned ponzu dipping sauce |
| **Truffle Edamame** | [`truffle-edamame.jpg`](truffle-edamame.jpg) | Appetizers | Kitchen | `Soy` | Steamed organic edamame pods tossed in fragrant black truffle oil and sea salt crystals |

### C. Specialty Coffee & Beverages (Bar Prep Area)
| Product Name | File | Category | Prep Area | Dietary Tags | Description |
|---|---|---|---|---|---|
| **Signature Caffe Latte** | [`signature-latte-art.jpg`](signature-latte-art.jpg) | Coffee | Bar | `Dairy/Lactose` | Handcrafted hot latte with swan latte art poured on golden-brown crema in a ceramic Japanese cup |
| **V60 Pour-Over Coffee** | [`v60-pourover-coffee.jpg`](v60-pourover-coffee.jpg) | Coffee | Bar | *(None)* | Single-origin specialty coffee hand-brewed via manual pour-over dripper into a clear serving carafe |
| **Kyoto Cold Brew** | [`japanese-cold-brew.jpg`](japanese-cold-brew.jpg) | Coffee | Bar | *(None)* | Slow-drip specialty cold brew served in a cut-crystal rocks glass with a carved clear ice sphere and orange twist |
| **Iced Espresso Tonic** | [`iced-espresso-tonic.jpg`](iced-espresso-tonic.jpg) | Coffee | Bar | *(None)* | Chilled double espresso layered over effervescent craft tonic water with ice and citrus |
| **Iced Uji Matcha Latte** | [`iced-matcha-latte.jpg`](iced-matcha-latte.jpg) | Specialty Tea | Bar | `Dairy/Lactose` | Two-toned layered iced latte with emerald ceremonial Uji matcha poured over cold oat milk with clear ice |
| **Hojicha Caramel Latte** | [`hojicha-caramel-latte.jpg`](hojicha-caramel-latte.jpg) | Specialty Tea | Bar | `Dairy/Lactose` | Roasted green tea hojicha with velvety steamed milk and Japanese kuromitsu brown sugar drizzle |

### D. Japanese Artisanal Desserts (Bar / Kitchen Prep Area)
| Product Name | File | Category | Prep Area | Dietary Tags | Description |
|---|---|---|---|---|---|
| **Matcha Mille-Crêpe** | [`matcha-mille-crepe.jpg`](matcha-mille-crepe.jpg) | Dessert | Bar | `Dairy/Lactose`, `Gluten`, `Egg` | Multi-layered thin crepe cake with light whipped cream and ceremonial matcha dusting |
| **Yuzu Rare Cheesecake** | [`yuzu-rare-cheesecake.jpg`](yuzu-rare-cheesecake.jpg) | Dessert | Bar | `Dairy/Lactose`, `Gluten`, `Egg` | Japanese baked citrus cheesecake with vibrant yuzu infusion and buttery crust |

---

## 4. Other Project Resources

Visual assets for brand storytelling, digital signage, customer onboarding, and physical collateral mockups.

| Resource Name | File | Aspect Ratio | Target Placement | Description |
|---|---|---|---|---|
| **Product Display Hero Banner** | [`hero-display-banner.jpg`](hero-display-banner.jpg) | `16:9` | In-outlet TV Display (BRD-PD-01) & Client Menu Header | Ultra-wide cinematic shot pairing freshly rolled sushi and steaming artisanal latte in warm ambient lighting |
| **Cafe Interior Ambiance** | [`cafe-interior-ambiance.jpg`](cafe-interior-ambiance.jpg) | `16:9` | Brand Landing Page, HRIS Onboarding, About Screens | Modern Japandi aesthetic interior showcasing light natural oak, sushi prep counter, espresso bar, and seating |
| **Table QR Stand Mockup** | [`table-qr-stand.jpg`](table-qr-stand.jpg) | `1:1` | Client Menu Tutorial & Crew Training (BRD-CM-01) | Sleek light oak wooden tent stand with 'Table 08 - Scan to Order' and QR code on cafe table |
| **Craft Ingredients Flatlay** | [`ingredients-flatlay.jpg`](ingredients-flatlay.jpg) | `3:2` | Storytelling, Marketing Cards, Hero Dividers | Flatlay showcasing fresh salmon sashimi, sushi rice, matcha bamboo chasen whisk, and coffee beans |

---

## 5. Brand Color System

```css
:root {
  /* Warm Crema & Coffee */
  --sl-crema-light:    #FAF5EE;
  --sl-crema:          #DDB892;
  --sl-crema-dark:     #B08968;
  --sl-espresso:       #5C3D2E;
  --sl-espresso-dark:  #2D1A12;

  /* Fresh Japanese Accents */
  --sl-salmon-light:   #FF8A65;
  --sl-salmon:         #FF6E40;
  --sl-salmon-deep:    #D84315;
  --sl-matcha-light:   #81A263;
  --sl-matcha-deep:    #4C6B38;

  /* Japanese Charcoal & Porcelain */
  --sl-nori-charcoal:  #1A1D20;
  --sl-nori-dark:      #111315;
  --sl-ceramic-white:  #FAF8F5;
  --sl-ceramic-border: #E8E0D2;
}
```
