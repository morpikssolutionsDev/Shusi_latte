// Package display implements Product Display content management
// from BR-ADM-10 and BR-PD-01 to BR-PD-03, with the display_slide
// table from TRD §4.5.
//
// Management manages banners, featured Products, ordering and
// schedule in the Admin Panel. This step is that content API, not
// the TV slideshow app itself, which is Phase 3. The public feed
// endpoint is included here because it is the contract the TV app
// will read: currently scheduled, active slides in sort order,
// with Sold Out featured Products marked (BR-PD-03), and content
// changes visible to a polling TV without manual reload (BR-PD-02)
// because the feed is read live from the database on every request.
//
// A slide's headline is bilingual (headline_id, headline_en) per
// TR-I18N-01. Its image is an image_key, not a file upload: image
// variants and the 1920x1080 display crop are generated at upload
// and seed time per TR-DM-09, in a later step.
package display

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TemplateHeroPromo       = "hero_promo"
	TemplateFeaturedProduct = "featured_product"
	TemplateHappyHour       = "happy_hour"
	TemplateCategorySpot    = "category_spotlight"
	TemplateBrandIdle       = "brand_idle"
	TemplateUploaded        = "uploaded"

	DefaultDurationSeconds = 10
)

// Slide is a display_slide row. SoldOut is computed at read time
// for featured_product slides and is not stored.
type Slide struct {
	ID              string  `json:"id"`
	Template        string  `json:"template"`
	ImageKey        *string `json:"image_key"`
	ImageFocus      *string `json:"image_focus"`
	ProductID       *string `json:"product_id"`
	DiscountID      *string `json:"discount_id"`
	CategoryID      *string `json:"category_id"`
	HeadlineID      *string `json:"headline_id"`
	HeadlineEN      *string `json:"headline_en"`
	Sort            int     `json:"sort"`
	Active          bool    `json:"active"`
	ActiveFrom      *int64  `json:"active_from"`
	ActiveTo        *int64  `json:"active_to"`
	DurationSeconds int     `json:"duration_seconds"`
	SoldOut         bool    `json:"sold_out"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hexEncode(b)
}

func hexEncode(b []byte) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	for _, c := range b {
		sb.WriteByte(digits[c>>4])
		sb.WriteByte(digits[c&0x0f])
	}
	return sb.String()
}

func Validate(s Slide) error {
	switch s.Template {
	case TemplateHeroPromo, TemplateFeaturedProduct, TemplateHappyHour, TemplateCategorySpot, TemplateBrandIdle, TemplateUploaded:
	default:
		return fmt.Errorf("unknown display template: %s", s.Template)
	}
	if s.Template == TemplateFeaturedProduct && s.ProductID == nil {
		return errors.New("featured_product slides require a product_id")
	}
	if s.Template == TemplateCategorySpot && s.CategoryID == nil {
		return errors.New("category_spotlight slides require a category_id")
	}
	if s.Sort < 0 {
		return errors.New("sort must be 0 or greater")
	}
	if s.DurationSeconds <= 0 {
		return errors.New("duration_seconds must be greater than 0")
	}
	if s.ActiveFrom != nil && s.ActiveTo != nil && *s.ActiveFrom >= *s.ActiveTo {
		return errors.New("active_from must be before active_to")
	}
	return nil
}

// IsScheduled reports whether the slide is inside its schedule at now.
func (s Slide) IsScheduled(nowMillis int64) bool {
	if !s.Active {
		return false
	}
	if s.ActiveFrom != nil && nowMillis < *s.ActiveFrom {
		return false
	}
	if s.ActiveTo != nil && nowMillis >= *s.ActiveTo {
		return false
	}
	return true
}
