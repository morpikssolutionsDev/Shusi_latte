package ops

import (
	"encoding/json"
	"net/http"
)

// Handler wires ops settings to the Admin Panel endpoints.
// Auth middleware restricts these to Management (BR-ADM-13).
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
	mux.HandleFunc("GET /api/v1/settings/ops", h.getSettings)
	mux.HandleFunc("PUT /api/v1/settings/ops", h.updateSettings)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.GetSettings()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "settings_unavailable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var st Settings
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	updated, err := h.Store.UpdateSettings(st)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_ops_settings", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
