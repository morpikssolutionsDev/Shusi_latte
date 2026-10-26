package ordering

import (
	"encoding/json"
	"net/http"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/auth"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
)

// StaffHandler adds Waiter endpoints to the ordering Handler.
// It is registered separately so the public Handler fields stay
// unchanged; both share the same Store.
type StaffHandler struct {
	Store   *Store
	Auth    *auth.Store
	Pricing *pricing.Store
}

func (h *StaffHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orders", h.listOrders)
	mux.HandleFunc("GET /api/v1/orders/{id}", h.getOrderStaff)
	mux.HandleFunc("POST /api/v1/orders", h.createStaffOrder)
	mux.HandleFunc("POST /api/v1/orders/{id}/claim", h.claim)
	mux.HandleFunc("POST /api/v1/orders/{id}/takeover", h.takeover)
	mux.HandleFunc("POST /api/v1/orders/{id}/reassign", h.reassign)
	mux.HandleFunc("POST /api/v1/orders/{id}/confirm", h.confirm)
	mux.HandleFunc("POST /api/v1/order-items/{id}/cancel", h.cancelItem)
	mux.HandleFunc("POST /api/v1/order-items/{id}/serve", h.serveItem)
	mux.HandleFunc("POST /api/v1/order-items/{id}/void", h.voidItem)
	mux.HandleFunc("POST /api/v1/order-items/{id}/remake", h.remakeItem)
	mux.HandleFunc("GET /api/v1/waiters/me/claims", h.myClaims)
	mux.HandleFunc("GET /api/v1/bills/{id}/summary", h.billSummary)
	mux.HandleFunc("POST /api/v1/bills/{id}/settle", h.settle)
	mux.HandleFunc("POST /api/v1/bills/{id}/unlock", h.unlock)
	mux.HandleFunc("POST /api/v1/bills/{id}/move", h.move)
	mux.HandleFunc("POST /api/v1/bills/merge", h.merge)
	mux.HandleFunc("POST /api/v1/bills/{id}/split", h.split)
	mux.HandleFunc("POST /api/v1/bills/{id}/receipts/reprint", h.reprint)
}

func (h *StaffHandler) crewMember(r *http.Request) (crew.Crew, bool) {
	return auth.CrewFromContext(r)
}

func (h *StaffHandler) listOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	claimedBy := q.Get("claimed_by")
	rows, err := h.Store.db.Query(`SELECT id FROM orders ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, http.StatusOK, []Order{})
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []Order{}
	for _, id := range ids {
		if o, err := h.Store.loadOrder(id); err == nil {
			if status != "" && o.Status != status {
				continue
			}
			if claimedBy != "" && (o.ClaimedBy == nil || *o.ClaimedBy != claimedBy) {
				continue
			}
			out = append(out, o)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *StaffHandler) getOrderStaff(w http.ResponseWriter, r *http.Request) {
	o, err := h.Store.loadOrder(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "order_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (h *StaffHandler) createStaffOrder(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		TableID         string           `json:"table_id"`
		CustomerName    *string          `json:"customer_name"`
		RestrictionTags []string         `json:"restriction_tag_ids"`
		Items           []OrderItemInput `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	// Resolve table_id to the Table's internal id if a QR token or
	// label was given? No: staff API takes the Table id directly.
	order, err := h.Store.StaffCreateOrder(c.CrewID, req.TableID, req.CustomerName, req.RestrictionTags, req.Items)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_order", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (h *StaffHandler) claim(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	order, err := h.Store.ClaimOrder(c.CrewID, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusConflict, "claim_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) takeover(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	order, err := h.Store.TakeoverOrder(c.CrewID, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusConflict, "takeover_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) reassign(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if c.Role != crew.RoleOutletAdmin && c.Role != crew.RoleManagement {
		writeProblem(w, http.StatusForbidden, "forbidden", "only outlet admin or management may reassign a claim")
		return
	}
	var req struct {
		ToCrewID string `json:"to_crew_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	order, err := h.Store.ReassignOrder(c.CrewID, r.PathValue("id"), req.ToCrewID)
	if err != nil {
		writeProblem(w, http.StatusConflict, "reassign_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) confirm(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		Items *[]OrderItemInput `json:"items"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	order, err := h.Store.ConfirmOrder(c.CrewID, r.PathValue("id"), req.Items)
	if err != nil {
		writeProblem(w, http.StatusConflict, "confirm_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) cancelItem(w http.ResponseWriter, r *http.Request) {
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
	order, err := h.Store.CancelItem(c.CrewID, r.PathValue("id"), req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) serveItem(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	order, err := h.Store.ServeItem(c.CrewID, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "serve_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) voidItem(w http.ResponseWriter, r *http.Request) {
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
	approver, ok := h.Auth.ConsumeOverride(r.Header.Get("X-Override-Token"), "void")
	if !ok {
		writeProblem(w, http.StatusForbidden, "override_required", "a void requires an outlet admin or management PIN override")
		return
	}
	order, err := h.Store.VoidItem(c.CrewID, approver.CrewID, r.PathValue("id"), req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "void_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) remakeItem(w http.ResponseWriter, r *http.Request) {
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
	approver, ok := h.Auth.ConsumeOverride(r.Header.Get("X-Override-Token"), "remake")
	if !ok {
		writeProblem(w, http.StatusForbidden, "override_required", "a remake requires an outlet admin or management PIN override")
		return
	}
	order, err := h.Store.RemakeItem(c.CrewID, approver.CrewID, r.PathValue("id"), req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "remake_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *StaffHandler) myClaims(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	writeJSON(w, http.StatusOK, h.Store.ListOpenClaims(c.CrewID))
}

func (h *StaffHandler) billSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if h.Pricing == nil {
		writeProblem(w, http.StatusInternalServerError, "pricing_unavailable", "pricing store is not wired")
		return
	}
	summary, err := h.Store.BillSummary(r.PathValue("id"), h.Pricing)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "bill_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *StaffHandler) settle(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	bill, err := h.Store.SettleBill(c.CrewID, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusConflict, "settle_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (h *StaffHandler) unlock(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	bill, err := h.Store.UnlockBill(c.CrewID, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusConflict, "unlock_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (h *StaffHandler) move(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		TableID string `json:"table_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	bill, err := h.Store.MoveBill(c.CrewID, r.PathValue("id"), req.TableID)
	if err != nil {
		writeProblem(w, http.StatusConflict, "move_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (h *StaffHandler) merge(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		SurvivingBillID string   `json:"surviving_bill_id"`
		AbsorbedBillIDs []string `json:"absorbed_bill_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	bill, err := h.Store.MergeBills(c.CrewID, req.SurvivingBillID, req.AbsorbedBillIDs)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "merge_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (h *StaffHandler) split(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		Method      string              `json:"method"`
		ShareCount  int                 `json:"share_count"`
		Assignments map[string][]string `json:"assignments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	subs, err := h.Store.SplitBill(c.CrewID, r.PathValue("id"), req.Method, req.ShareCount, req.Assignments)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "split_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *StaffHandler) reprint(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	if err := h.Store.ReprintReceipt(c.CrewID, r.PathValue("id")); err != nil {
		writeProblem(w, http.StatusBadRequest, "reprint_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "REPRINT recorded"})
}
