// Package adminapi exposes the Admin Panel endpoints (Phase 1): catalog, Sold Out, settings,
// discounts, Tables, Crew, image upload and the audit log. Authorisation follows BRD §5.1.
package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/crew"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/display"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/media"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/store"
)

type Handlers struct {
	DB      *dbx.DB
	Catalog *catalog.Service
	Outlet  *outlet.Service
	Crew    *crew.Service
	Media   *media.Store
	Display *display.Service
}

var (
	management = auth.Require(auth.Roles(auth.RoleManagement))
	admins     = auth.Require(auth.Roles(auth.RoleManagement, auth.RoleOutletAdmin))
	staff      = auth.Require(nil)
)

func (h *Handlers) Routes(r chi.Router) {
	r.With(staff).Get("/catalog", httpx.Handler(h.getCatalog))
	r.With(management).Post("/catalog/categories", httpx.Handler(h.saveCategory))
	r.With(management).Put("/catalog/categories/{id}", httpx.Handler(h.saveCategory))
	r.With(management).Post("/catalog/products", httpx.Handler(h.saveProduct))
	r.With(management).Put("/catalog/products/{id}", httpx.Handler(h.saveProduct))
	r.With(management).Post("/catalog/dietary-tags", httpx.Handler(h.saveDietaryTag))
	r.With(management).Put("/catalog/dietary-tags/{id}", httpx.Handler(h.saveDietaryTag))
	r.With(management).Put("/catalog/prep-areas/{id}/capacity", httpx.Handler(h.setCapacity))
	r.With(admins).Put("/catalog/variants/{id}/sold-out", httpx.Handler(h.setSoldOut))

	r.With(staff).Get("/settings", httpx.Handler(h.getSettings))
	r.With(management).Put("/settings/pricing", httpx.Handler(h.updatePricing))
	r.With(management).Put("/settings/ops", httpx.Handler(h.updateOps))

	r.With(admins).Get("/discounts", httpx.Handler(h.listDiscounts))
	r.With(management).Post("/discounts", httpx.Handler(h.saveDiscount))
	r.With(management).Put("/discounts/{id}", httpx.Handler(h.saveDiscount))

	r.With(staff).Get("/tables", httpx.Handler(h.listTables))
	r.With(admins).Post("/tables", httpx.Handler(h.saveTable))
	r.With(admins).Put("/tables/{id}", httpx.Handler(h.saveTable))
	r.With(admins).Post("/tables/{id}/regenerate-qr", httpx.Handler(h.regenerateQR))

	r.With(management).Get("/crew", httpx.Handler(h.listCrew))
	r.With(management).Post("/crew", httpx.Handler(h.createCrew))
	r.With(management).Put("/crew/{id}", httpx.Handler(h.updateCrew))
	r.With(management).Post("/crew/{id}/reset-pin", httpx.Handler(h.resetPIN))

	r.With(management).Post("/images", httpx.Handler(h.uploadImage))
	r.With(management).Post("/videos", httpx.Handler(h.uploadVideo))
	r.With(admins).Get("/audit", httpx.Handler(h.searchAudit))

	// Product Display: the resolved playlist is public, like the menu (BR-PD-01).
	r.Get("/public/display/playlist", httpx.Handler(h.playlist))
	r.With(admins).Get("/display/slides", httpx.Handler(h.listSlides))
	r.With(management).Post("/display/slides", httpx.Handler(h.saveSlide))
	r.With(management).Put("/display/slides/{id}", httpx.Handler(h.saveSlide))
	r.With(management).Delete("/display/slides/{id}", httpx.Handler(h.deleteSlide))
	r.With(management).Put("/display/slide-order", httpx.Handler(h.reorderSlides))
}

func (h *Handlers) playlist(w http.ResponseWriter, r *http.Request) error {
	p, err := h.Display.Playlist(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, p)
	return nil
}

func (h *Handlers) listSlides(w http.ResponseWriter, r *http.Request) error {
	s, err := h.Display.Slides(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, s)
	return nil
}

func (h *Handlers) saveSlide(w http.ResponseWriter, r *http.Request) error {
	var in display.Slide
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.ID = chi.URLParam(r, "id")
	out, err := h.Display.Save(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	if out.ImageKey != "" {
		if err := h.Media.EnsureWide(out.ImageKey, out.ImageFocus.X, out.ImageFocus.Y); err != nil {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handlers) deleteSlide(w http.ResponseWriter, r *http.Request) error {
	if err := h.Display.Delete(r.Context(), actor(r), chi.URLParam(r, "id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handlers) reorderSlides(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Display.Reorder(r.Context(), actor(r), in.IDs); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func actor(r *http.Request) string { return auth.FromContext(r.Context()).ID }

// ---------------------------------------------------------------- catalog

func (h *Handlers) getCatalog(w http.ResponseWriter, r *http.Request) error {
	c, err := h.Catalog.Catalog(r.Context(), r.URL.Query().Get("archived") == "true")
	if err != nil {
		return err
	}
	httpx.WriteJSONWithETag(w, r, c)
	return nil
}

func (h *Handlers) saveCategory(w http.ResponseWriter, r *http.Request) error {
	var in catalog.Category
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.ID = chi.URLParam(r, "id")
	if in.ID == "" {
		in.Active = true
	}
	out, err := h.Catalog.SaveCategory(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handlers) saveProduct(w http.ResponseWriter, r *http.Request) error {
	var in catalog.Product
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.ID = chi.URLParam(r, "id")
	out, err := h.Catalog.SaveProduct(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	if out.ImageKey != "" {
		// The 16:9 Product Display crop follows the focal point (TR-DM-09).
		if err := h.Media.EnsureWide(out.ImageKey, out.ImageFocus.X, out.ImageFocus.Y); err != nil {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handlers) saveDietaryTag(w http.ResponseWriter, r *http.Request) error {
	var in catalog.DietaryTag
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.ID = chi.URLParam(r, "id")
	out, err := h.Catalog.SaveDietaryTag(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handlers) setCapacity(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ParallelCapacity int64 `json:"parallelCapacity"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Catalog.SetPrepAreaCapacity(r.Context(), actor(r), chi.URLParam(r, "id"), in.ParallelCapacity); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handlers) setSoldOut(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		SoldOut bool `json:"soldOut"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.Catalog.SetSoldOut(r.Context(), actor(r), chi.URLParam(r, "id"), in.SoldOut)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// ---------------------------------------------------------------- settings & discounts

func (h *Handlers) getSettings(w http.ResponseWriter, r *http.Request) error {
	s, err := h.Outlet.Settings(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, s)
	return nil
}

func (h *Handlers) updatePricing(w http.ResponseWriter, r *http.Request) error {
	var in outlet.PricingSettings
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	s, err := h.Outlet.UpdatePricing(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, s)
	return nil
}

func (h *Handlers) updateOps(w http.ResponseWriter, r *http.Request) error {
	var in outlet.OpsSettings
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	s, err := h.Outlet.UpdateOps(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, s)
	return nil
}

func (h *Handlers) listDiscounts(w http.ResponseWriter, r *http.Request) error {
	d, err := h.Outlet.Discounts(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

func (h *Handlers) saveDiscount(w http.ResponseWriter, r *http.Request) error {
	var in outlet.Discount
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.ID = chi.URLParam(r, "id")
	out, err := h.Outlet.SaveDiscount(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// ---------------------------------------------------------------- tables

func (h *Handlers) listTables(w http.ResponseWriter, r *http.Request) error {
	t, err := h.Outlet.Tables(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, t)
	return nil
}

func (h *Handlers) saveTable(w http.ResponseWriter, r *http.Request) error {
	var in outlet.Table
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.ID = chi.URLParam(r, "id")
	out, err := h.Outlet.SaveTable(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handlers) regenerateQR(w http.ResponseWriter, r *http.Request) error {
	out, err := h.Outlet.RegenerateQR(r.Context(), actor(r), chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// ---------------------------------------------------------------- crew

func (h *Handlers) listCrew(w http.ResponseWriter, r *http.Request) error {
	m, err := h.Crew.List(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}

func (h *Handlers) createCrew(w http.ResponseWriter, r *http.Request) error {
	var in crew.Input
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	in.Active = true
	m, err := h.Crew.Create(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
	return nil
}

func (h *Handlers) updateCrew(w http.ResponseWriter, r *http.Request) error {
	var in crew.Input
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	m, err := h.Crew.Update(r.Context(), actor(r), chi.URLParam(r, "id"), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}

func (h *Handlers) resetPIN(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		PIN string `json:"pin"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Crew.ResetPIN(r.Context(), actor(r), chi.URLParam(r, "id"), in.PIN); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---------------------------------------------------------------- media & audit

func (h *Handlers) uploadImage(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUpload+1<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		return httpx.NewProblem(http.StatusBadRequest, "file_required", "Pilih file gambar")
	}
	defer file.Close()
	key, err := h.Media.Save(file)
	if errors.Is(err, media.ErrUnsupported) {
		return httpx.NewProblem(http.StatusUnsupportedMediaType, "unsupported_image", "Gunakan gambar JPEG, PNG, atau WebP")
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"imageKey": key})
	return nil
}

func (h *Handlers) uploadVideo(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxVideoUpload+1<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		return httpx.NewProblem(http.StatusBadRequest, "file_required", "Pilih file video (maks. 40 MB)")
	}
	defer file.Close()
	key, err := h.Media.SaveVideo(file)
	if errors.Is(err, media.ErrUnsupportedVideo) {
		return httpx.NewProblem(http.StatusUnsupportedMediaType, "unsupported_video", "Gunakan video MP4 atau WebM")
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"videoKey": key})
	return nil
}

type auditEntry struct {
	ID       string          `json:"id"`
	At       int64           `json:"at"`
	Actor    string          `json:"actorId,omitempty"`
	Approver string          `json:"approverId,omitempty"`
	Action   string          `json:"action"`
	Entity   string          `json:"entity"`
	EntityID string          `json:"entityId,omitempty"`
	Before   json.RawMessage `json:"before,omitempty"`
	After    json.RawMessage `json:"after,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

func (h *Handlers) searchAudit(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	opt := func(key string) any {
		if v := q.Get(key); v != "" {
			return v
		}
		return nil
	}
	optInt := func(key string) any {
		if v, err := strconv.ParseInt(q.Get(key), 10, 64); err == nil {
			return v
		}
		return nil
	}
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 64)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := h.DB.Read().SearchAudit(r.Context(), store.SearchAuditParams{Entity: opt("entity"), ActionPrefix: opt("action"),
		Actor: opt("actor"), FromMs: optInt("from"), BeforeMs: optInt("before"), RowLimit: limit})
	if err != nil {
		return err
	}
	out := make([]auditEntry, 0, len(rows))
	for _, a := range rows {
		e := auditEntry{ID: a.ID, At: a.At, Actor: a.ActorCrewID.String, Approver: a.OverrideCrewID.String, Action: a.Action,
			Entity: a.Entity, EntityID: a.EntityID.String, Reason: a.Reason.String}
		if a.BeforeJson.Valid {
			e.Before = json.RawMessage(a.BeforeJson.String)
		}
		if a.AfterJson.Valid {
			e.After = json.RawMessage(a.AfterJson.String)
		}
		out = append(out, e)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}
