package tables

import (
	"encoding/json"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

// Handler wires Table management to the Admin Panel endpoints.
// The QR PNG endpoint returns a 256px PNG encoding the Client Menu
// URL for the Table, suitable for printing on a Table stand.
type Handler struct {
	Store      *Store
	PublicBase string
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

type tableResponse struct {
	Table
	QRURL string `json:"qr_url"`
}

func (h *Handler) respond(w http.ResponseWriter, status int, t Table) {
	writeJSON(w, status, tableResponse{Table: t, QRURL: QRURL(h.PublicBase, t)})
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tables", h.listTables)
	mux.HandleFunc("POST /api/v1/tables", h.createTable)
	mux.HandleFunc("GET /api/v1/tables/{id}", h.getTable)
	mux.HandleFunc("PUT /api/v1/tables/{id}", h.updateTable)
	mux.HandleFunc("POST /api/v1/tables/{id}/regenerate-qr", h.regenerateQR)
	mux.HandleFunc("POST /api/v1/tables/{id}/regenerate-slug", h.regenerateSlug)
	mux.HandleFunc("GET /api/v1/tables/{id}/qr.png", h.qrPNG)
	// Public: resolve a static order slug to its Table, so the
	// client-menu app at /order/{slug} can bind to the Table.
	// This path is public in the auth middleware ("/public/" prefix).
	mux.HandleFunc("GET /public/tables/by-slug/{slug}", h.bySlug)
}

func (h *Handler) listTables(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	tables := h.Store.ListTables(includeInactive)
	out := make([]tableResponse, 0, len(tables))
	for _, t := range tables {
		out = append(out, tableResponse{Table: t, QRURL: QRURL(h.PublicBase, t)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createTable(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	t, err := h.Store.CreateTable(req.Label)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_table", err.Error())
		return
	}
	h.respond(w, http.StatusCreated, t)
}

func (h *Handler) getTable(w http.ResponseWriter, r *http.Request) {
	t, err := h.Store.GetTable(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	h.respond(w, http.StatusOK, t)
}

func (h *Handler) updateTable(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		Label  string `json:"label"`
		Active bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	t, err := h.Store.UpdateTable(r.PathValue("id"), req.Label, req.Active)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_table", err.Error())
		return
	}
	h.respond(w, http.StatusOK, t)
}

func (h *Handler) regenerateQR(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	t, err := h.Store.RegenerateQRToken(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	h.respond(w, http.StatusOK, t)
}

func (h *Handler) bySlug(w http.ResponseWriter, r *http.Request) {
	tbl, err := h.Store.GetBySlug(r.PathValue("slug"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	h.respond(w, http.StatusOK, tbl)
}

func (h *Handler) regenerateSlug(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	tbl, err := h.Store.RegenerateSlug(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	h.respond(w, http.StatusOK, tbl)
}

func (h *Handler) qrPNG(w http.ResponseWriter, r *http.Request) {
	t, err := h.Store.GetTable(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	png, err := qrcode.Encode(QRURL(h.PublicBase, t), qrcode.Medium, 256)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "qr_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// withActor returns a handler copy whose store records the
// authenticated crew as the audit actor.
func (h *Handler) withActor(r *http.Request) *Handler {
	copyStore := *h.Store
	copyStore.Actor = actor.FromRequest(r.Context())
	return &Handler{Store: &copyStore, PublicBase: h.PublicBase}
}
