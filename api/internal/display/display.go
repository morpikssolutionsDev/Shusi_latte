// Package display owns the Product Display playlist (BR-ADM-10, BR-PD-01..03, MOTION.md §6):
// slide administration for Management and the resolved, public playlist for the TV.
package display

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/pricing"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// Topic carries playlist changes to every Product Display.
const Topic = "display"

const (
	HeroPromo         = "hero_promo"
	FeaturedProduct   = "featured_product"
	HappyHour         = "happy_hour"
	CategorySpotlight = "category_spotlight"
	BrandIdle         = "brand_idle"
	Uploaded          = "uploaded"
)

var templates = []string{HeroPromo, FeaturedProduct, HappyHour, CategorySpotlight, BrandIdle, Uploaded}

type Bilingual struct {
	ID string `json:"id"`
	EN string `json:"en"`
}

type Slide struct {
	ID              string        `json:"id"`
	Template        string        `json:"template"`
	Sort            int64         `json:"sort"`
	Active          bool          `json:"active"`
	Headline        Bilingual     `json:"headline"`
	Subline         Bilingual     `json:"subline"`
	ImageKey        string        `json:"imageKey,omitempty"`
	VideoKey        string        `json:"videoKey,omitempty"` // uploaded template only
	ImageFocus      catalog.Focus `json:"imageFocus"`
	ProductID       string        `json:"productId,omitempty"`
	DiscountID      string        `json:"discountId,omitempty"`
	CategoryIDs     []string      `json:"categoryIds"`
	Steam           bool          `json:"steam"`
	DurationSeconds int64         `json:"durationSeconds"`
	ActiveFrom      *int64        `json:"activeFrom,omitempty"`
	ActiveTo        *int64        `json:"activeTo,omitempty"`
}

type Service struct {
	db     *dbx.DB
	clock  platform.Clock
	outlet *outlet.Service
}

func NewService(db *dbx.DB, clock platform.Clock) *Service {
	return &Service{db: db, clock: clock, outlet: outlet.NewService(db, clock)}
}

func invalid(detail string) error {
	p := httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_slide", "Slide tidak valid")
	p.Detail = detail
	return p
}

func toSlide(r store.DisplaySlide) Slide {
	s := Slide{ID: r.ID, Template: r.Template, Sort: r.Sort, Active: r.Active == 1,
		Headline: Bilingual{r.HeadlineID, r.HeadlineEn}, Subline: Bilingual{r.SublineID, r.SublineEn},
		ImageKey: r.ImageKey.String, VideoKey: r.VideoKey.String, ImageFocus: catalog.Focus{X: r.ImageFocusX, Y: r.ImageFocusY},
		ProductID: r.ProductID.String, DiscountID: r.DiscountID.String, Steam: r.Steam == 1, DurationSeconds: r.DurationSeconds,
		CategoryIDs: []string{}}
	_ = json.Unmarshal([]byte(r.CategoryIds), &s.CategoryIDs)
	if r.ActiveFrom.Valid {
		v := r.ActiveFrom.Int64
		s.ActiveFrom = &v
	}
	if r.ActiveTo.Valid {
		v := r.ActiveTo.Int64
		s.ActiveTo = &v
	}
	return s
}

func (s *Service) Slides(ctx context.Context) ([]Slide, error) {
	rows, err := s.db.Read().ListSlides(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Slide, 0, len(rows))
	for _, r := range rows {
		out = append(out, toSlide(r))
	}
	return out, nil
}

func validate(in *Slide) error {
	if !slices.Contains(templates, in.Template) {
		return invalid("Template tidak dikenal")
	}
	if in.DurationSeconds == 0 {
		in.DurationSeconds = 8
	}
	if in.DurationSeconds < 4 || in.DurationSeconds > 60 {
		return invalid("Durasi slide 4–60 detik")
	}
	if in.ImageFocus == (catalog.Focus{}) {
		in.ImageFocus = catalog.Focus{X: 50, Y: 50}
	}
	if in.CategoryIDs == nil {
		in.CategoryIDs = []string{}
	}
	switch in.Template {
	case HeroPromo:
		if strings.TrimSpace(in.Headline.ID) == "" || in.ImageKey == "" {
			return invalid("Hero Promo butuh judul dan gambar")
		}
	case FeaturedProduct:
		if in.ProductID == "" {
			return invalid("Pilih produk unggulan")
		}
	case HappyHour:
		if in.DiscountID == "" {
			return invalid("Pilih diskon terjadwal")
		}
	case CategorySpotlight:
		if len(in.CategoryIDs) == 0 {
			return invalid("Pilih minimal satu kategori")
		}
	case Uploaded:
		if in.ImageKey == "" && in.VideoKey == "" {
			return invalid("Unggah gambar atau video")
		}
	}
	if in.Template != Uploaded {
		in.VideoKey = ""
	}
	if in.VideoKey != "" && !strings.HasPrefix(in.VideoKey, "vid/") {
		return invalid("Video tidak dikenal")
	}
	if in.ActiveFrom != nil && in.ActiveTo != nil && *in.ActiveTo <= *in.ActiveFrom {
		return invalid("Waktu selesai harus setelah waktu mulai")
	}
	return nil
}

func (s *Service) Save(ctx context.Context, actor string, in Slide) (Slide, error) {
	if err := validate(&in); err != nil {
		return in, err
	}
	cats, _ := json.Marshal(in.CategoryIDs)
	now := platform.Millis(s.clock.Now())
	err := s.db.Write(ctx, func(tx *dbx.Tx) error {
		action := "display_slide.updated"
		if in.ID == "" {
			in.ID = platform.NewID()
			action = "display_slide.created"
			if in.Sort == 0 {
				rows, err := tx.ListSlides(ctx)
				if err != nil {
					return err
				}
				in.Sort = int64(len(rows)+1) * 10
			}
			if err := tx.CreateSlide(ctx, store.CreateSlideParams{ID: in.ID, Template: in.Template, Sort: in.Sort, Active: boolInt(in.Active),
				HeadlineID: in.Headline.ID, HeadlineEn: in.Headline.EN, SublineID: in.Subline.ID, SublineEn: in.Subline.EN,
				ImageKey: nullString(in.ImageKey), VideoKey: nullString(in.VideoKey), ImageFocusX: in.ImageFocus.X, ImageFocusY: in.ImageFocus.Y,
				ProductID: nullString(in.ProductID), DiscountID: nullString(in.DiscountID), CategoryIds: string(cats),
				Steam: boolInt(in.Steam), DurationSeconds: in.DurationSeconds, ActiveFrom: nullPtr(in.ActiveFrom),
				ActiveTo: nullPtr(in.ActiveTo), CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
		} else {
			if _, err := tx.GetSlide(ctx, in.ID); errors.Is(err, sql.ErrNoRows) {
				return httpx.NewProblem(http.StatusNotFound, "not_found", "Slide tidak ditemukan")
			} else if err != nil {
				return err
			}
			if err := tx.UpdateSlide(ctx, store.UpdateSlideParams{Template: in.Template, Sort: in.Sort, Active: boolInt(in.Active),
				HeadlineID: in.Headline.ID, HeadlineEn: in.Headline.EN, SublineID: in.Subline.ID, SublineEn: in.Subline.EN,
				ImageKey: nullString(in.ImageKey), VideoKey: nullString(in.VideoKey), ImageFocusX: in.ImageFocus.X, ImageFocusY: in.ImageFocus.Y,
				ProductID: nullString(in.ProductID), DiscountID: nullString(in.DiscountID), CategoryIds: string(cats),
				Steam: boolInt(in.Steam), DurationSeconds: in.DurationSeconds, ActiveFrom: nullPtr(in.ActiveFrom),
				ActiveTo: nullPtr(in.ActiveTo), UpdatedAt: now, ID: in.ID}); err != nil {
				return err
			}
		}
		tx.Publish(realtime.Event{Topic: Topic, Type: "playlist.changed", ID: in.ID})
		return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: action, Entity: "display_slide", EntityID: in.ID, After: in})
	})
	return in, err
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		row, err := tx.GetSlide(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.NewProblem(http.StatusNotFound, "not_found", "Slide tidak ditemukan")
		}
		if err != nil {
			return err
		}
		if err := tx.DeleteSlide(ctx, id); err != nil {
			return err
		}
		tx.Publish(realtime.Event{Topic: Topic, Type: "playlist.changed", ID: id})
		return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: "display_slide.deleted", Entity: "display_slide", EntityID: id, Before: toSlide(row)})
	})
}

// Reorder sets the playlist order to the given slide ids.
func (s *Service) Reorder(ctx context.Context, actor string, ids []string) error {
	return s.db.Write(ctx, func(tx *dbx.Tx) error {
		now := platform.Millis(s.clock.Now())
		for i, id := range ids {
			if err := tx.SetSlideSort(ctx, store.SetSlideSortParams{Sort: int64(i+1) * 10, UpdatedAt: now, ID: id}); err != nil {
				return err
			}
		}
		tx.Publish(realtime.Event{Topic: Topic, Type: "playlist.changed"})
		return audit.Write(ctx, tx, s.clock.Now(), audit.Entry{Actor: actor, Action: "display_slide.reordered", Entity: "display_slide", After: ids})
	})
}

// ---------------------------------------------------------------- public playlist

type TagView struct {
	Code   string `json:"code"`
	NameID string `json:"nameId"`
	NameEN string `json:"nameEn"`
}

type ProductView struct {
	ID            string        `json:"id"`
	NameID        string        `json:"nameId"`
	NameEN        string        `json:"nameEn"`
	DescriptionID string        `json:"descriptionId"`
	DescriptionEN string        `json:"descriptionEn"`
	ImageKey      string        `json:"imageKey,omitempty"`
	ImageFocus    catalog.Focus `json:"imageFocus"`
	Price         int64         `json:"price"`     // lowest price among available Variants
	PriceFrom     bool          `json:"priceFrom"` // other Variants cost more
	SoldOut       bool          `json:"soldOut"`   // every Variant Sold Out
	SoldOutSome   []Bilingual   `json:"soldOutVariants"`
	DietaryTags   []TagView     `json:"dietaryTags"`
}

type DiscountView struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Value     int64  `json:"value"`
	ActiveNow bool   `json:"activeNow"`
	StartsAt  int64  `json:"startsAt"` // current or next window, UTC ms
	EndsAt    int64  `json:"endsAt"`
	// DiscountedPrice is the featured Product's price with the discount applied (0 when none).
	DiscountedPrice int64 `json:"discountedPrice"`
}

type PlaylistSlide struct {
	ID              string        `json:"id"`
	Template        string        `json:"template"`
	DurationSeconds int64         `json:"durationSeconds"`
	Headline        Bilingual     `json:"headline"`
	Subline         Bilingual     `json:"subline"`
	ImageKey        string        `json:"imageKey,omitempty"`
	VideoKey        string        `json:"videoKey,omitempty"`
	ImageFocus      catalog.Focus `json:"imageFocus"`
	Steam           bool          `json:"steam"`
	Product         *ProductView  `json:"product,omitempty"`
	Products        []ProductView `json:"products,omitempty"`
	Discount        *DiscountView `json:"discount,omitempty"`
}

type Playlist struct {
	GeneratedAt int64           `json:"generatedAt"`
	Slides      []PlaylistSlide `json:"slides"`
}

// Playlist resolves the active slides against the live catalog and discounts. Slides whose
// content is unavailable are skipped (BR-PD-03): a Hidden, Archived or fully Sold Out featured
// Product, an inactive scheduled discount, or a spotlight with no visible Products.
func (s *Service) Playlist(ctx context.Context) (Playlist, error) {
	now := s.clock.Now()
	slides, err := s.Slides(ctx)
	if err != nil {
		return Playlist{}, err
	}
	cat, err := catalog.Load(ctx, s.db.Read(), false)
	if err != nil {
		return Playlist{}, err
	}
	discounts, err := s.outlet.Discounts(ctx)
	if err != nil {
		return Playlist{}, err
	}
	products := map[string]catalog.Product{}
	for _, p := range cat.Products {
		products[p.ID] = p
	}
	tags := map[string]catalog.DietaryTag{}
	for _, t := range cat.DietaryTags {
		tags[t.ID] = t
	}
	view := func(p catalog.Product) ProductView { return productView(p, tags) }

	out := Playlist{GeneratedAt: platform.Millis(now), Slides: []PlaylistSlide{}}
	nowMs := platform.Millis(now)
	for _, sl := range slides {
		if !sl.Active || (sl.ActiveFrom != nil && nowMs < *sl.ActiveFrom) || (sl.ActiveTo != nil && nowMs >= *sl.ActiveTo) {
			continue
		}
		ps := PlaylistSlide{ID: sl.ID, Template: sl.Template, DurationSeconds: sl.DurationSeconds, Headline: sl.Headline,
			Subline: sl.Subline, ImageKey: sl.ImageKey, VideoKey: sl.VideoKey, ImageFocus: sl.ImageFocus, Steam: sl.Steam}
		switch sl.Template {
		case FeaturedProduct:
			p, ok := products[sl.ProductID]
			if !ok || !p.Orderable() {
				continue
			}
			v := view(p)
			ps.Product = &v
		case HappyHour:
			idx := slices.IndexFunc(discounts, func(d outlet.Discount) bool { return d.ID == sl.DiscountID })
			if idx < 0 || !discounts[idx].Active || discounts[idx].Trigger != "auto" {
				continue
			}
			d := discounts[idx]
			start, end, active, ok := NextWindow(outlet.ToPricing(d).Schedule, now, platform.Jakarta)
			if !ok {
				continue
			}
			dv := DiscountView{Name: d.Name, Kind: d.Kind, Value: d.Value, ActiveNow: active,
				StartsAt: platform.Millis(start), EndsAt: platform.Millis(end)}
			if p, ok := products[sl.ProductID]; ok && p.Orderable() {
				v := view(p)
				ps.Product = &v
				line := pricing.Line{CategoryID: p.CategoryID, ProductID: p.ID, UnitPrice: v.Price, Quantity: 1}
				r := pricing.Calculate(pricing.Input{Lines: []pricing.Line{line}, Auto: []pricing.Discount{outlet.ToPricing(d)}})
				dv.DiscountedPrice = r.Lines[0].Net
			}
			ps.Discount = &dv
		case CategorySpotlight:
			for _, p := range cat.Products {
				if slices.Contains(sl.CategoryIDs, p.CategoryID) && p.Status == catalog.StatusActive && p.ImageKey != "" {
					ps.Products = append(ps.Products, view(p))
				}
			}
			if len(ps.Products) == 0 {
				continue
			}
			if len(ps.Products) > 6 {
				ps.Products = ps.Products[:6]
			}
		}
		out.Slides = append(out.Slides, ps)
	}
	return out, nil
}

func productView(p catalog.Product, tags map[string]catalog.DietaryTag) ProductView {
	v := ProductView{ID: p.ID, NameID: p.NameID, NameEN: p.NameEN, DescriptionID: p.DescriptionID, DescriptionEN: p.DescriptionEN,
		ImageKey: p.ImageKey, ImageFocus: p.ImageFocus, SoldOut: p.SoldOut(), SoldOutSome: []Bilingual{}, DietaryTags: []TagView{}}
	first := true
	for _, vr := range p.Variants {
		if vr.SoldOut {
			v.SoldOutSome = append(v.SoldOutSome, Bilingual{vr.NameID, vr.NameEN})
			continue
		}
		if first || vr.Price < v.Price {
			if !first {
				v.PriceFrom = true
			}
			v.Price = vr.Price
		} else if vr.Price != v.Price {
			v.PriceFrom = true
		}
		first = false
	}
	for _, id := range p.DietaryTagIDs {
		if t, ok := tags[id]; ok {
			v.DietaryTags = append(v.DietaryTags, TagView{Code: t.Code, NameID: t.NameID, NameEN: t.NameEN})
		}
	}
	return v
}

// NextWindow returns the discount window containing now, or the next one within 8 days.
// A schedule without hours is a whole-day window on each allowed day.
func NextWindow(s pricing.Schedule, now time.Time, loc *time.Location) (start, end time.Time, active, ok bool) {
	local := now.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	for i := -1; i <= 8; i++ {
		d := day.AddDate(0, 0, i)
		weekday := (int(d.Weekday()) + 6) % 7
		if s.DaysMask != 0 && s.DaysMask != 127 && s.DaysMask&(1<<weekday) == 0 {
			continue
		}
		st, en := d, d.AddDate(0, 0, 1)
		if s.StartMinute != nil && s.EndMinute != nil {
			st = d.Add(time.Duration(*s.StartMinute) * time.Minute)
			en = d.Add(time.Duration(*s.EndMinute) * time.Minute)
			if !en.After(st) {
				en = en.AddDate(0, 0, 1)
			}
		}
		if s.ValidFrom != nil && en.Before(*s.ValidFrom) || s.ValidTo != nil && !st.Before(*s.ValidTo) {
			continue
		}
		if !now.Before(st) && now.Before(en) {
			return st, en, true, true
		}
		if st.After(now) {
			return st, en, false, true
		}
	}
	return time.Time{}, time.Time{}, false, false
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullPtr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}
