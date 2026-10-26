package ordering

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// KitchenHandler adds Kitchen queue endpoints to the StaffHandler.
func (h *StaffHandler) RegisterKitchen(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/prep-areas/{id}/queue", h.kitchenQueue)
	mux.HandleFunc("POST /api/v1/order-items/{id}/transition", h.transitionItem)
	mux.HandleFunc("POST /api/v1/order-items/{id}/kitchen-cancel", h.kitchenCancel)
	mux.HandleFunc("GET /api/v1/orders/{id}/estimated-wait", h.estimatedWait)
}

func (h *StaffHandler) kitchenQueue(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	pickupReminder := 180
	if v := r.URL.Query().Get("pickup_reminder_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pickupReminder = n
		}
	}
	q, err := h.Store.KitchenQueue(r.PathValue("id"), pickupReminder)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "prep_area_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *StaffHandler) transitionItem(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		ToStatus string `json:"to_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	order, err := h.Store.TransitionItem(c.CrewID, r.PathValue("id"), req.ToStatus)
	if err != nil {
		writeProblem(w, http.StatusConflict, "transition_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) kitchenCancel(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	order, err := h.Store.KitchenCancelItem(c.CrewID, r.PathValue("id"), req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) estimatedWait(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	minutes, err := h.Store.EstimatedWaitForOrder(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "order_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order_id": r.PathValue("id"), "estimated_wait_minutes": minutes})
}
