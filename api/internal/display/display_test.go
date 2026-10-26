package display

import "testing"

func TestValidate(t *testing.T) {
	if err := Validate(Slide{Template: TemplateHeroPromo, DurationSeconds: 10}); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Slide{Template: TemplateFeaturedProduct, DurationSeconds: 10}); err == nil {
		t.Fatal("featured_product without product_id should fail")
	}
	if err := Validate(Slide{Template: TemplateCategorySpot, DurationSeconds: 10}); err == nil {
		t.Fatal("category_spotlight without category_id should fail")
	}
	if err := Validate(Slide{Template: "unknown", DurationSeconds: 10}); err == nil {
		t.Fatal("unknown template should fail")
	}
	from, to := int64(2000), int64(1000)
	if err := Validate(Slide{Template: TemplateBrandIdle, DurationSeconds: 10, ActiveFrom: &from, ActiveTo: &to}); err == nil {
		t.Fatal("active_from after active_to should fail")
	}
}

func TestIsScheduled(t *testing.T) {
	from, to := int64(1000), int64(2000)
	sl := Slide{Active: true, ActiveFrom: &from, ActiveTo: &to}
	if sl.IsScheduled(500) || !sl.IsScheduled(1500) || sl.IsScheduled(2500) {
		t.Fatal("schedule window should be respected")
	}
	sl.Active = false
	if sl.IsScheduled(1500) {
		t.Fatal("inactive slide should not be scheduled")
	}
}
