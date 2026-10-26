package seed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/media"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
)

// Catalog data mirrors DEMO-AND-SEED.md §3 and §5. Prices are Rupiah excluding tax and service.

type tagDef struct{ code, nameID, nameEN string }

var dietaryTags = []tagDef{
	{"dairy", "Susu", "Dairy"}, {"gluten", "Gluten", "Gluten"}, {"egg", "Telur", "Egg"}, {"fish", "Ikan", "Fish"},
	{"raw_fish", "Ikan mentah", "Raw fish"}, {"shellfish", "Udang & kerang", "Shellfish"}, {"nuts", "Kacang pohon", "Tree nuts"},
	{"peanut", "Kacang tanah", "Peanut"}, {"soy", "Kedelai", "Soy"}, {"sesame", "Wijen", "Sesame"}, {"pork", "Babi", "Pork"},
	{"alcohol", "Alkohol", "Alcohol"}, {"spicy", "Pedas", "Spicy"},
}

type categoryDef struct{ key, nameID, nameEN, area string }

var categories = []categoryDef{
	{"nigiri", "Nigiri", "Nigiri", "kitchen"}, {"rolls", "Roll", "Rolls", "kitchen"}, {"sashimi", "Sashimi", "Sashimi", "kitchen"},
	{"hot", "Hidangan Panas", "Hot Dishes", "kitchen"}, {"small", "Camilan", "Small Plates", "kitchen"},
	{"coffee", "Kopi", "Coffee", "bar"}, {"tea", "Teh", "Tea", "bar"}, {"dessert", "Hidangan Penutup", "Dessert", "bar"},
}

type variantDef struct {
	nameID, nameEN string
	price          int64
}

type modDef struct {
	nameID, nameEN string
	price          int64
	tags           []string
}

type groupDef struct {
	nameID, nameEN string
	min, max       int64
	mods           []modDef
}

var (
	gMilk         = groupDef{"Susu", "Milk", 1, 1, []modDef{{"Susu segar", "Fresh milk", 0, []string{"dairy"}}, {"Susu oat", "Oat milk", 6000, nil}, {"Susu kedelai", "Soy milk", 5000, []string{"soy"}}}}
	gCoffeeExtras = groupDef{"Tambahan kopi", "Coffee extras", 0, 3, []modDef{{"Extra shot", "Extra shot", 8000, nil}, {"Sirup vanila", "Vanilla syrup", 5000, nil}, {"Kurangi gula", "Less sugar", 0, nil}}}
	gSushiExtras  = groupDef{"Tambahan sushi", "Sushi extras", 0, 3, []modDef{{"Tanpa wasabi", "No wasabi", 0, nil}, {"Extra gari", "Extra gari", 3000, nil}, {"Mayo pedas", "Spicy mayo", 5000, []string{"egg", "spicy"}}}}
	gRamenExtras  = groupDef{"Tambahan ramen", "Ramen extras", 0, 2, []modDef{{"Extra chashu", "Extra chashu", 18000, []string{"pork"}}, {"Telur ajitama", "Ajitama egg", 8000, []string{"egg"}}}}
	gSpice        = groupDef{"Tingkat pedas", "Spice level", 0, 1, []modDef{{"Tidak pedas", "Mild", 0, nil}, {"Sedang", "Medium", 0, []string{"spicy"}}, {"Pedas", "Hot", 0, []string{"spicy"}}}}
	gDonburi      = groupDef{"Tambahan donburi", "Donburi extras", 0, 2, []modDef{{"Telur onsen", "Onsen egg", 8000, []string{"egg"}}, {"Extra nasi", "Extra rice", 6000, nil}}}
	gGyozaDip     = groupDef{"Saus gyoza", "Gyoza dip", 0, 1, []modDef{{"Ponzu", "Ponzu", 0, nil}, {"Minyak cabai", "Chilli oil", 0, []string{"spicy", "sesame"}}}}
	gDessert      = groupDef{"Tambahan", "Add-on", 0, 1, []modDef{{"Es krim matcha", "Matcha ice cream", 15000, []string{"dairy"}}}}
)

type productDef struct {
	key, category  string
	nameID, nameEN string
	descID, descEN string
	variants       []variantDef
	prepMinutes    int64
	tags           []string
	photo          string // relative to docs/images
	status         string
	groups         []groupDef
}

var hotIced = func(hot, iced int64) []variantDef {
	return []variantDef{{"Panas", "Hot", hot}, {"Dingin", "Iced", iced}}
}

var products = []productDef{
	{"salmon-aburi", "nigiri", "Salmon Aburi Nigiri", "Salmon Aburi Nigiri",
		"Salmon dibakar sekilas dengan mayo kewpie dan saus tare manis.", "Torched salmon with kewpie mayo and sweet tare glaze.",
		[]variantDef{{"2 pcs", "2 pcs", 48000}, {"4 pcs", "4 pcs", 89000}}, 6, []string{"raw_fish", "fish", "egg", "soy"},
		"products/food/salmon-aburi-nigiri.jpg", "", []groupDef{gSushiExtras}},
	{"unagi", "nigiri", "Unagi Kabayaki Nigiri", "Unagi Kabayaki Nigiri",
		"Belut panggang berlapis saus tare karamel, diikat nori.", "Grilled eel brushed with caramelised tare, wrapped in nori.",
		[]variantDef{{"2 pcs", "2 pcs", 58000}, {"4 pcs", "4 pcs", 109000}}, 6, []string{"fish", "soy", "gluten"},
		"products/food/unagi-kabayaki-nigiri.jpg", "", []groupDef{gSushiExtras}},
	{"dragon", "rolls", "Dragon Roll", "Dragon Roll",
		"Roll isi unagi berselimut alpukat, tobiko, dan saus kabayaki.", "Unagi roll under avocado scales, tobiko and kabayaki drizzle.",
		[]variantDef{{"8 pcs", "8 pcs", 88000}}, 10, []string{"fish", "shellfish", "soy", "gluten"},
		"products/food/dragon-roll-maki.jpg", "", []groupDef{gSushiExtras}},
	{"spicy-salmon", "rolls", "Spicy Salmon Roll", "Spicy Salmon Roll",
		"Salmon pedas dengan mayo sriracha, wijen hitam, dan tempura renyah.", "Spicy salmon with sriracha mayo, black sesame and tempura crunch.",
		[]variantDef{{"8 pcs", "8 pcs", 68000}}, 9, []string{"raw_fish", "fish", "egg", "soy", "sesame", "gluten", "spicy"},
		"products/food/spicy-salmon-roll.jpg", "", []groupDef{gSushiExtras}},
	{"ebi-tempura", "rolls", "Ebi Tempura Roll", "Ebi Tempura Roll",
		"Udang tempura renyah dalam roll nori.", "Crispy prawn tempura rolled in nori.",
		[]variantDef{{"8 pcs", "8 pcs", 72000}}, 10, []string{"shellfish", "gluten", "egg", "soy"},
		"", catalog.StatusArchived, nil},
	{"sashimi", "sashimi", "Sashimi Tuna & Salmon", "Tuna & Salmon Sashimi Platter",
		"Irisan tuna akami dan salmon segar dengan wasabi dan daun shiso.", "Thick-cut tuna akami and salmon with fresh wasabi and shiso.",
		[]variantDef{{"Reguler, 9 iris", "Regular, 9 slices", 125000}, {"Premium, 15 iris", "Premium, 15 slices", 195000}}, 8,
		[]string{"raw_fish", "fish"}, "products/food/tuna-sashimi-platter.jpg", "", []groupDef{gSushiExtras}},
	{"wagyu", "hot", "Wagyu Beef Donburi", "Wagyu Beef Donburi",
		"Wagyu medium-rare di atas nasi Jepang, telur onsen, dan keripik bawang putih.", "Medium-rare wagyu over Japanese rice with onsen egg and garlic chips.",
		[]variantDef{{"Reguler", "Regular", 135000}, {"Besar", "Large", 165000}}, 12, []string{"egg", "soy", "gluten"},
		"products/food/wagyu-beef-donburi.jpg", "", []groupDef{gDonburi}},
	{"katsu", "hot", "Chicken Katsu Curry", "Chicken Katsu Curry",
		"Ayam panko renyah dengan kari Jepang yang harum dan nasi pulen.", "Crispy panko chicken with fragrant Japanese curry and pearl rice.",
		[]variantDef{{"Satu porsi", "One size", 72000}}, 14, []string{"gluten", "egg", "soy"},
		"products/food/chicken-katsu-curry.jpg", "", []groupDef{gSpice}},
	{"tonkotsu", "hot", "Tonkotsu Ramen", "Tonkotsu Ramen",
		"Kuah tulang babi kental dengan chashu, telur ramen, menma, dan nori.", "Rich pork-bone broth with chashu, ramen egg, menma and nori.",
		[]variantDef{{"Satu porsi", "One size", 78000}}, 12, []string{"pork", "egg", "gluten", "soy", "sesame"},
		"", "", []groupDef{gRamenExtras, gSpice}},
	{"gyoza", "small", "Gyoza Renyah", "Crispy Gyoza",
		"Gyoza goreng dengan rok renyah keemasan dan saus ponzu.", "Pan-fried dumplings with a golden lace skirt and ponzu.",
		[]variantDef{{"5 pcs", "5 pcs", 42000}, {"10 pcs", "10 pcs", 78000}}, 10, []string{"pork", "gluten", "soy", "sesame"},
		"products/food/crispy-gyoza-dumplings.jpg", "", []groupDef{gGyozaDip}},
	{"edamame", "small", "Edamame Truffle", "Truffle Edamame",
		"Edamame kukus dengan minyak truffle hitam dan garam laut.", "Steamed edamame tossed in black truffle oil and sea salt.",
		[]variantDef{{"Satu porsi", "One size", 38000}}, 5, []string{"soy"}, "products/food/truffle-edamame.jpg", "", nil},
	{"latte", "coffee", "Signature Caffè Latte", "Signature Caffè Latte",
		"Espresso dengan susu velvety dan latte art di cangkir keramik.", "Espresso with velvety milk and latte art in a ceramic cup.",
		hotIced(38000, 41000), 4, []string{"dairy"}, "products/beverages/signature-latte-art.jpg", "", []groupDef{gMilk, gCoffeeExtras}},
	{"v60", "coffee", "V60 Pour-Over", "V60 Pour-Over",
		"Kopi single-origin diseduh manual dengan dripper V60.", "Single-origin coffee hand-brewed through a V60 dripper.",
		[]variantDef{{"Gayo Aceh", "Gayo Aceh", 45000}, {"Kintamani Bali", "Kintamani Bali", 45000}, {"Toraja", "Toraja", 48000}}, 6,
		nil, "products/beverages/v60-pourover-coffee.jpg", "", nil},
	{"cold-brew", "coffee", "Kyoto Cold Brew", "Kyoto Cold Brew",
		"Cold brew tetes lambat dengan bola es bening dan kulit jeruk.", "Slow-drip cold brew over a clear ice sphere with an orange twist.",
		[]variantDef{{"Satu ukuran", "One size", 42000}}, 2, nil, "products/beverages/japanese-cold-brew.jpg", "", []groupDef{gCoffeeExtras}},
	{"tonic", "coffee", "Iced Espresso Tonic", "Iced Espresso Tonic",
		"Double espresso di atas tonic dingin, es, dan jeruk.", "Double espresso over chilled tonic, ice and citrus.",
		[]variantDef{{"Satu ukuran", "One size", 45000}}, 3, nil, "", "", []groupDef{gCoffeeExtras}},
	{"sakura", "coffee", "Sakura Latte", "Sakura Latte",
		"Latte musiman dengan sirup sakura.", "Seasonal latte with sakura syrup.",
		hotIced(42000, 45000), 4, []string{"dairy"}, "", catalog.StatusHidden, []groupDef{gMilk}},
	{"matcha", "tea", "Uji Matcha Latte", "Uji Matcha Latte",
		"Matcha seremonial dari Uji dengan susu, berlapis cantik.", "Ceremonial Uji matcha layered with milk.",
		hotIced(42000, 45000), 4, []string{"dairy"}, "products/beverages/iced-matcha-latte.jpg", "", []groupDef{gMilk}},
	{"hojicha", "tea", "Hojicha Caramel Latte", "Hojicha Caramel Latte",
		"Teh hijau sangrai hojicha dengan susu dan gula kuromitsu.", "Roasted hojicha with steamed milk and kuromitsu brown sugar.",
		hotIced(40000, 43000), 4, []string{"dairy"}, "products/beverages/hojicha-caramel-latte.jpg", "", []groupDef{gMilk}},
	{"crepe", "dessert", "Matcha Mille-Crêpe", "Matcha Mille-Crêpe",
		"Kue krep berlapis dengan krim ringan dan taburan matcha.", "Layered crêpe cake with light cream and a matcha dusting.",
		[]variantDef{{"Satu potong", "One slice", 55000}}, 3, []string{"dairy", "gluten", "egg"},
		"products/desserts/matcha-mille-crepe.jpg", "", []groupDef{gDessert}},
	{"cheesecake", "dessert", "Yuzu Rare Cheesecake", "Yuzu Rare Cheesecake",
		"Cheesecake lembut dengan yuzu segar dan dasar biskuit mentega.", "Silky cheesecake with bright yuzu on a buttery base.",
		[]variantDef{{"Satu potong", "One slice", 52000}}, 3, []string{"dairy", "gluten", "egg"},
		"products/desserts/yuzu-rare-cheesecake.jpg", "", []groupDef{gDessert}},
}

// fixedClock anchors every seeded timestamp at T0.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// catalogIDs lets later seed steps find seeded records by their definition key.
type catalogIDs struct {
	categories map[string]string
	products   map[string]catalog.Product
	tags       map[string]string
}

func seedCatalog(ctx context.Context, db *dbx.DB, opts Options, managementID, outletAdminID string) (catalogIDs, error) {
	ids := catalogIDs{categories: map[string]string{}, products: map[string]catalog.Product{}, tags: map[string]string{}}
	svc := catalog.NewService(db, fixedClock{opts.T0})
	images := media.NewStore(opts.ImagesDir)

	areas := map[string]string{}
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		for i, a := range []struct {
			code, nameID, nameEN string
			capacity             int64
		}{{"kitchen", "Dapur", "Kitchen", 2}, {"bar", "Bar", "Bar", 1}} {
			id := platform.NewID()
			areas[a.code] = id
			if err := tx.CreatePrepArea(ctx, store.CreatePrepAreaParams{ID: id, Code: a.code, NameID: a.nameID, NameEn: a.nameEN,
				ParallelCapacity: a.capacity, Sort: int64(i)}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return ids, err
	}

	for i, t := range dietaryTags {
		saved, err := svc.SaveDietaryTag(ctx, managementID, catalog.DietaryTag{Code: t.code, NameID: t.nameID, NameEN: t.nameEN, Icon: t.code, Sort: int64(i)})
		if err != nil {
			return ids, err
		}
		ids.tags[t.code] = saved.ID
	}
	for i, c := range categories {
		saved, err := svc.SaveCategory(ctx, managementID, catalog.Category{NameID: c.nameID, NameEN: c.nameEN, PrepAreaID: areas[c.area], Sort: int64(i * 10)})
		if err != nil {
			return ids, err
		}
		ids.categories[c.key] = saved.ID
	}

	for i, p := range products {
		in := catalog.Product{CategoryID: ids.categories[p.category], NameID: p.nameID, NameEN: p.nameEN,
			DescriptionID: p.descID, DescriptionEN: p.descEN, PrepTimeSeconds: p.prepMinutes * 60, Status: p.status,
			Sort: int64(i * 10), ImageFocus: catalog.Focus{X: 50, Y: 50}, DietaryTagIDs: tagIDs(ids.tags, p.tags)}
		if p.photo != "" {
			key, err := saveImage(images, filepath.Join(opts.AssetsDir, filepath.FromSlash(p.photo)))
			if err != nil {
				return ids, fmt.Errorf("%s photo: %w", p.key, err)
			}
			in.ImageKey = key
			if err := images.EnsureWide(key, 50, 50); err != nil {
				return ids, err
			}
		}
		for j, v := range p.variants {
			in.Variants = append(in.Variants, catalog.Variant{NameID: v.nameID, NameEN: v.nameEN, Price: v.price, IsDefault: j == 0})
		}
		for _, g := range p.groups {
			group := catalog.ModifierGroup{NameID: g.nameID, NameEN: g.nameEN, MinSelect: g.min, MaxSelect: g.max}
			for _, m := range g.mods {
				group.Modifiers = append(group.Modifiers, catalog.Modifier{NameID: m.nameID, NameEN: m.nameEN, Price: m.price, DietaryTagIDs: tagIDs(ids.tags, m.tags)})
			}
			in.ModifierGroups = append(in.ModifierGroups, group)
		}
		saved, err := svc.SaveProduct(ctx, managementID, in)
		if err != nil {
			return ids, fmt.Errorf("product %s: %w", p.key, err)
		}
		ids.products[p.key] = saved
	}

	// Unagi 4 pcs is Sold Out manually by the Outlet Admin (DEMO-AND-SEED §3.2).
	if _, err := svc.SetSoldOut(ctx, outletAdminID, ids.products["unagi"].Variants[1].ID, true); err != nil {
		return ids, err
	}
	return ids, nil
}

func tagIDs(byCode map[string]string, codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		out = append(out, byCode[c])
	}
	return out
}

func saveImage(store *media.Store, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return store.Save(f)
}

func seedOutlet(ctx context.Context, db *dbx.DB, opts Options, managementID string, ids catalogIDs) error {
	svc := outlet.NewService(db, fixedClock{opts.T0})
	minutes := func(h, m int64) *int64 { v := h*60 + m; return &v }
	amount := func(v int64) *int64 { return &v }
	discounts := []outlet.Discount{
		{Name: "Happy Hour Kopi", Kind: "percent", Value: 1500, Scope: "item", Trigger: "auto", DaysMask: 127,
			StartMinute: minutes(14, 0), EndMinute: minutes(17, 0), Active: true,
			Categories: []string{ids.categories["coffee"], ids.categories["tea"]}},
		{Name: "Weekday Latte", Kind: "percent", Value: 1000, Scope: "item", Trigger: "auto", DaysMask: 0b0011111, Active: true,
			Products: []string{ids.products["latte"].ID}},
		{Name: "Paket Hemat", Kind: "nominal", Value: 25000, Scope: "bill", Trigger: "auto", DaysMask: 127, MinSubtotal: amount(350000), Active: true},
		{Name: "Diskon Manual 10%", Kind: "percent", Value: 1000, Scope: "bill", Trigger: "manual", DaysMask: 127, Active: true},
		{Name: "Diskon Manual Item 10%", Kind: "percent", Value: 1000, Scope: "item", Trigger: "manual", DaysMask: 127, Active: true},
	}
	for _, d := range discounts {
		if _, err := svc.SaveDiscount(ctx, managementID, d); err != nil {
			return fmt.Errorf("discount %s: %w", d.Name, err)
		}
	}
	for n := int64(1); n <= 12; n++ {
		t, err := svc.SaveTable(ctx, managementID, outlet.Table{Number: n, Active: true})
		if err != nil {
			return err
		}
		if n == 12 {
			// Meja 12: inactive with a regenerated QR, so its old printed code is invalid.
			if _, err := svc.RegenerateQR(ctx, managementID, t.ID); err != nil {
				return err
			}
			t.Active = false
			if _, err := svc.SaveTable(ctx, managementID, t); err != nil {
				return err
			}
		}
	}
	return nil
}
