package ordering

import (
	"encoding/json"
	"net/http"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/catalog"
)

// Handler wires Phase 2a public Client Menu endpoints. These are
// public by design: scanning a QR is the Customer's authentication,
// and guest tokens authenticate subsequent Customer requests.
// Staff auth middleware must allow /public/ paths through.
type Handler struct {
	Store   *Store
	Catalog catalog.Store
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
	mux.HandleFunc("GET /public/tables/{qr_token}", h.scanTable)
	mux.HandleFunc("POST /public/tables/{qr_token}/join", h.join)
	mux.HandleFunc("POST /public/tables/{qr_token}/rejoin", h.rejoin)
	mux.HandleFunc("POST /public/tables/{qr_token}/not-mine", h.notMine)
	mux.HandleFunc("POST /public/waiter-calls", h.waiterCall)
	mux.HandleFunc("GET /public/catalog", h.publicCatalog)
}

func (h *Handler) scanTable(w http.ResponseWriter, r *http.Request) {
	scan, err := h.Store.ScanTable(r.PathValue("qr_token"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

func (h *Handler) join(w http.ResponseWriter, r *http.Request) {
	scan, err := h.Store.ScanTable(r.PathValue("qr_token"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	var req struct {
		Name            string   `json:"name"`
		RestrictionTags []string `json:"restriction_tag_ids"`
		DietaryNote     *string  `json:"dietary_note"`
		Consent         bool     `json:"consent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	result, err := h.Store.Join(scan.TableID, req.Name, req.RestrictionTags, req.DietaryNote, req.Consent)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_join", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) rejoin(w http.ResponseWriter, r *http.Request) {
	if _, err := h.Store.ScanTable(r.PathValue("qr_token")); err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	var req struct {
		CustomerEntryID string `json:"customer_entry_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	result, err := h.Store.Rejoin(req.CustomerEntryID)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_rejoin", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) notMine(w http.ResponseWriter, r *http.Request) {
	scan, err := h.Store.ScanTable(r.PathValue("qr_token"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	if err := h.Store.RecordWaiterCall(scan.TableID, "not_mine", scan.BillID, nil); err != nil {
		writeProblem(w, http.StatusInternalServerError, "not_mine_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "This isn't your table. Please ask staff for help — a Waiter has been notified.",
	})
}

func (h *Handler) waiterCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		QRToken         string  `json:"qr_token"`
		CustomerEntryID *string `json:"customer_entry_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	scan, err := h.Store.ScanTable(req.QRToken)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "table_not_found", err.Error())
		return
	}
	if err := h.Store.RecordWaiterCall(scan.TableID, "waiter_call", scan.BillID, req.CustomerEntryID); err != nil {
		writeProblem(w, http.StatusInternalServerError, "waiter_call_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "A Waiter has been called.",
	})
}

// publicCatalog returns only Active Products (BR-CM-03), with
// Sold Out Variants marked by the catalog's derived orderable
// state. Dietary Warnings are computed by the Client Menu from
// the Customer's own restriction and the Product tags returned
// here; the server result at order time is authoritative.
func (h *Handler) publicCatalog(w http.ResponseWriter, r *http.Request) {
	if h.Catalog == nil {
		writeJSON(w, http.StatusOK, catalog.Catalog{})
		return
	}
	cat := h.Catalog.Catalog()
	active := []catalog.Product{}
	for _, p := range cat.Products {
		if p.Status == catalog.StatusActive {
			active = append(active, p)
		}
	}
	cat.Products = active
	writeJSON(w, http.StatusOK, cat)
}
