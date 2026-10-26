package crew

import (
	"encoding/json"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"net/http"
)

// Handler wires crew records to the Admin Panel endpoints.
// Responses never include a PIN or a PIN hash. PIN resets are
// audit-logged; role enforcement for who may reset a PIN arrives
// with the auth phase (TR-AUTH-05).
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
	mux.HandleFunc("GET /api/v1/crew", h.listCrew)
	mux.HandleFunc("POST /api/v1/crew", h.createCrew)
	mux.HandleFunc("GET /api/v1/crew/{id}", h.getCrew)
	mux.HandleFunc("PUT /api/v1/crew/{id}", h.updateCrew)
	mux.HandleFunc("POST /api/v1/crew/{id}/pin-reset", h.resetPIN)
}

func (h *Handler) listCrew(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	writeJSON(w, http.StatusOK, h.Store.ListCrew(includeInactive))
}

func (h *Handler) createCrew(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		CrewID   string   `json:"crew_id"`
		FullName string   `json:"full_name"`
		Role     string   `json:"role"`
		Stations []string `json:"stations"`
		PIN      string   `json:"pin"`
		ImageKey *string  `json:"image_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	c, err := h.Store.CreateCrew(req.CrewID, req.FullName, req.Role, req.Stations, req.PIN, req.ImageKey)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_crew", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) getCrew(w http.ResponseWriter, r *http.Request) {
	c, err := h.Store.GetCrew(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "crew_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) updateCrew(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		FullName string   `json:"full_name"`
		Role     string   `json:"role"`
		Stations []string `json:"stations"`
		Active   bool     `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	c, err := h.Store.UpdateCrew(r.PathValue("id"), req.FullName, req.Role, req.Stations, req.Active)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_crew", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) resetPIN(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		NewPIN string `json:"new_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	c, err := h.Store.ResetPIN(r.PathValue("id"), req.NewPIN)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_pin", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// withActor returns a handler copy whose store records the
// authenticated crew as the audit actor.
func (h *Handler) withActor(r *http.Request) *Handler {
	copyStore := *h.Store
	copyStore.Actor = actor.FromRequest(r.Context())
	return &Handler{Store: &copyStore}
}
