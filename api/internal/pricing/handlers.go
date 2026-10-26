package pricing

import (
	"encoding/json"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"net/http"
)

// Handler wires pricing settings, discounts and calculation to the
// Admin Panel endpoints. Manual discounts still require a Management
// PIN Override at the auth layer (BR-R-17); this handler records the
// discount definition, it does not grant an override.
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
	mux.HandleFunc("GET /api/v1/settings/pricing", h.getSettings)
	mux.HandleFunc("PUT /api/v1/settings/pricing", h.updateSettings)
	mux.HandleFunc("GET /api/v1/discounts", h.listDiscounts)
	mux.HandleFunc("POST /api/v1/discounts", h.createDiscount)
	mux.HandleFunc("POST /api/v1/pricing/calculate", h.calculate)
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
	h = h.withActor(r)
	var st Settings
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	out, err := h.Store.UpdateSettings(st)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_settings", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listDiscounts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Store.ListDiscounts())
}

func (h *Handler) createDiscount(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var d Discount
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	out, err := h.Store.CreateDiscount(d)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_discount", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) calculate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items           []ItemInput `json:"items"`
		BillDiscountID  *string     `json:"bill_discount_id"`
		BillDiscount    *Discount   `json:"bill_discount"`
		PaymentMethod   string      `json:"payment_method"`
		UseCurrentRates bool        `json:"use_current_rates"`
		ServiceRateBP   *int        `json:"service_rate_bp"`
		TaxRateBP       *int        `json:"tax_rate_bp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	settings, err := h.Store.GetSettings()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "settings_unavailable", err.Error())
		return
	}
	if !req.UseCurrentRates {
		if req.ServiceRateBP != nil {
			settings.ServiceRateBP = *req.ServiceRateBP
		}
		if req.TaxRateBP != nil {
			settings.TaxRateBP = *req.TaxRateBP
		}
	}
	var billDiscount *Discount
	if req.BillDiscount != nil {
		billDiscount = req.BillDiscount
	} else if req.BillDiscountID != nil {
		d, err := h.Store.GetDiscount(*req.BillDiscountID)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "discount_not_found", err.Error())
			return
		}
		billDiscount = &d
	}
	out, err := Calculate(req.Items, billDiscount, settings, req.PaymentMethod)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_calculation", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// withActor returns a handler copy whose store records the
// authenticated crew as the audit actor.
func (h *Handler) withActor(r *http.Request) *Handler {
	copyStore := *h.Store
	copyStore.Actor = actor.FromRequest(r.Context())
	return &Handler{Store: &copyStore}
}
