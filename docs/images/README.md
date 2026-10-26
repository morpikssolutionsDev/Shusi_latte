# Sushi Latte — Demo Image Catalogue

Mockup photography for the prototype and showcase. Brand logos live in [../brand/](../brand/); colours and type in [../design/](../design/). This folder holds **photos only**.

All product photos are 1:1 JPEG, 1024×1024. They are originals: the app generates WebP variants (320/640/1024 px, plus 16:9 crops for Product Display) at seed time ([TRD TR-DM-09](../TRD.md)). Rules for imagery: [../design/ICONOGRAPHY.md §3](../design/ICONOGRAPHY.md#3-imagery).

## Products

Product data (category, price, Variants, Modifiers, Dietary Tags, Prep Time) is defined in [../DEMO-AND-SEED.md §3](../DEMO-AND-SEED.md#3-product-catalog).

### Food — Kitchen Prep Area (`products/food/`)

| File | Product |
|---|---|
| `salmon-aburi-nigiri.jpg` | Salmon Aburi Nigiri |
| `unagi-kabayaki-nigiri.jpg` | Unagi Kabayaki Nigiri |
| `dragon-roll-maki.jpg` | Dragon Roll |
| `spicy-salmon-roll.jpg` | Spicy Salmon Roll |
| `tuna-sashimi-platter.jpg` | Tuna & Salmon Sashimi Platter |
| `wagyu-beef-donburi.jpg` | Wagyu Beef Donburi |
| `chicken-katsu-curry.jpg` | Chicken Katsu Curry |
| `crispy-gyoza-dumplings.jpg` | Crispy Gyoza |
| `truffle-edamame.jpg` | Truffle Edamame |

### Beverages — Bar Prep Area (`products/beverages/`)

| File | Product |
|---|---|
| `signature-latte-art.jpg` | Signature Caffè Latte |
| `v60-pourover-coffee.jpg` | V60 Pour-Over |
| `japanese-cold-brew.jpg` | Kyoto Cold Brew |
| `iced-matcha-latte.jpg` | Uji Matcha Latte |
| `hojicha-caramel-latte.jpg` | Hojicha Caramel Latte |

### Desserts — Bar Prep Area (`products/desserts/`)

| File | Product |
|---|---|
| `matcha-mille-crepe.jpg` | Matcha Mille-Crêpe |
| `yuzu-rare-cheesecake.jpg` | Yuzu Rare Cheesecake |

### Products without a photo (branded placeholder)

| Product | Reason |
|---|---|
| Iced Espresso Tonic | Original image showed a third-party branded bottle; removed |
| Tonkotsu Ramen | Original image did not show ramen; removed |
| Sakura Latte (Hidden, seasonal) | Demonstrates a Hidden Product |
| Ebi Tempura Roll (Archived) | Appears only in historical Bills |

Replace a placeholder by dropping a matching 1:1 photo into the right folder and updating the seed mapping.

## Staff (`staff/`)

| File | Used for |
|---|---|
| `staff-store-manager.jpg` | Management and Outlet Admin demo accounts |
| `staff-waiter-male.jpg` | Male Waiter, Cashier and Kitchen demo accounts |
| `staff-waitress-female.jpg` | Female Waiter, Cashier and Kitchen demo accounts |

Portraits are reused across several fictional Crew members; the mapping is in [../DEMO-AND-SEED.md §4](../DEMO-AND-SEED.md#4-crew-and-demo-logins).

## Marketing (`marketing/`)

| File | Ratio | Used for |
|---|---|---|
| `hero-display-banner.jpg` | 16:9 | Product Display Hero Promo slide, Client Menu splash background |
| `cafe-interior-ambiance.jpg` | 16:9 | Demo launcher header, Brand Idle slide background |
| `ingredients-flatlay.jpg` | 3:2 | Category Spotlight background, empty states |
| `table-qr-stand.jpg` | 1:1 | Demo launcher "how to order" card, crew training |

## Removed (recoverable from git history)

- Two staff portraits that appeared to show real, identifiable people.
- `iced-espresso-tonic.jpg` (third-party branded product), `tonkotsu-ramen-bowl.jpg` (wrong subject).
- An earlier logo set, colour palette and preview page that conflicted with [../brand/](../brand/) and [../design/](../design/).
