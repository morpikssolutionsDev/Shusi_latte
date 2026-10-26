package display

import (
	"encoding/json"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"net/http"
	"time"
)

// Handler wires display content to the Admin Panel endpoints and
// the public feed the TV slideshow will read in Phase 3.
type Handler struct {
	Store *Store
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, code, title string) {
	writeJSON(w, status, map[string]any{
		"type": "about:blank", "title": title, "status": status, "code": code,
	})
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/display/slides", h.listSlides)
	mux.HandleFunc("POST /api/v1/display/slides", h.createSlide)
	mux.HandleFunc("GET /api/v1/display/slides/{id}", h.getSlide)
	mux.HandleFunc("PUT /api/v1/display/slides/{id}", h.updateSlide)
	mux.HandleFunc("DELETE /api/v1/display/slides/{id}", h.deleteSlide)
	mux.HandleFunc("GET /public/display/slides", h.publicSlides)
}

func decodeSlide(r *http.Request) (Slide, error) {
	var sl Slide
	if err := json.NewDecoder(r.Body).Decode(&sl); err != nil {
		return Slide{}, err
	}
	return sl, nil
}

func (h *Handler) listSlides(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Store.ListSlides())
}

func (h *Handler) createSlide(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	sl, err := decodeSlide(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	created, err := h.Store.CreateSlide(sl)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_slide", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) getSlide(w http.ResponseWriter, r *http.Request) {
	sl, err := h.Store.GetSlide(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "slide_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sl)
}

func (h *Handler) updateSlide(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	sl, err := decodeSlide(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	updated, err := h.Store.UpdateSlide(r.PathValue("id"), sl)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_slide", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteSlide(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	if err := h.Store.DeleteSlide(r.PathValue("id")); err != nil {
		writeProblem(w, http.StatusNotFound, "slide_not_found", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) publicSlides(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Store.PublicSlides(time.Now().UnixMilli()))
}

// withActor returns a handler copy whose store records the
// authenticated crew as the audit actor.
func (h *Handler) withActor(r *http.Request) *Handler {
	copyStore := *h.Store
	copyStore.Actor = actor.FromRequest(r.Context())
	return &Handler{Store: &copyStore}
}
