package ordering

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
)

// CashierHandler adds Cashier endpoints to the StaffHandler.
func (h *StaffHandler) RegisterCashier(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/bills/takeaway", h.createTakeaway)
	mux.HandleFunc("POST /api/v1/bills/takeaway/{id}/orders", h.createTakeawayOrder)
	mux.HandleFunc("POST /api/v1/bills/takeaway/{id}/confirm-orders", h.confirmTakeawayOrders)
	mux.HandleFunc("GET /api/v1/bills/{id}/payments", h.billPayments)
	mux.HandleFunc("POST /api/v1/bills/{id}/payments", h.recordPayment)
	mux.HandleFunc("POST /api/v1/payments/{id}/verify", h.verifyPayment)
	mux.HandleFunc("POST /api/v1/payments/{id}/write-off", h.writeOffPayment)
	mux.HandleFunc("POST /api/v1/bills/{id}/close", h.closeBill)
	mux.HandleFunc("POST /api/v1/bills/{id}/close-unpaid", h.closeUnpaid)
	mux.HandleFunc("POST /api/v1/bills/{id}/refunds", h.refund)
	mux.HandleFunc("POST /api/v1/drawer-sessions", h.openDrawer)
	mux.HandleFunc("POST /api/v1/drawer-sessions/{id}/close", h.closeDrawer)
	mux.HandleFunc("POST /api/v1/cash-handovers", h.recordHandover)
	mux.HandleFunc("POST /api/v1/cash-handovers/{id}/confirm", h.confirmHandover)
	mux.HandleFunc("GET /api/v1/crew/{id}/held-cash", h.heldCash)
	mux.HandleFunc("POST /api/v1/post-outage/bills", h.postOutage)
	mux.HandleFunc("POST /api/v1/bills/{id}/apply-discount", h.applyDiscount)
}

func (h *StaffHandler) createTakeaway(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		CustomerName string `json:"customer_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	bill, err := h.Store.CreateTakeawayBill(c.CrewID, req.CustomerName)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_takeaway", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, bill)
}

func (h *StaffHandler) createTakeawayOrder(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		RestrictionTags []string         `json:"restriction_tag_ids"`
		Items           []OrderItemInput `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	order, err := h.Store.CreateTakeawayOrder(c.CrewID, r.PathValue("id"), req.RestrictionTags, req.Items)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_order", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (h *StaffHandler) confirmTakeawayOrders(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	orders, err := h.Store.ConfirmTakeawayOrders(c.CrewID, r.PathValue("id"), h.Pricing)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "confirm_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (h *StaffHandler) billPayments(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	summary, err := h.Store.BillPaymentSummary(r.PathValue("id"), h.Pricing)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "bill_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *StaffHandler) recordPayment(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		Method           string  `json:"method"`
		Amount           int64   `json:"amount"`
		SubBillID        *string `json:"sub_bill_id"`
		PayNowServeLater bool    `json:"pay_now_serve_later"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	pay, err := h.Store.RecordPayment(c.CrewID, r.PathValue("id"), req.Method, req.Amount, req.SubBillID, req.PayNowServeLater)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "payment_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, pay)
}

func (h *StaffHandler) verifyPayment(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	pay, err := h.Store.VerifyPayment(c.CrewID, r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "verify_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pay)
}

func (h *StaffHandler) writeOffPayment(w http.ResponseWriter, r *http.Request) {
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
	pay, err := h.Store.WriteOffPayment(c.CrewID, r.PathValue("id"), req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "write_off_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pay)
}

func (h *StaffHandler) closeBill(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	bill, err := h.Store.CloseBill(c.CrewID, r.PathValue("id"), h.Pricing)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "close_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (h *StaffHandler) closeUnpaid(w http.ResponseWriter, r *http.Request) {
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
	approver, ok := h.Auth.ConsumeOverride(r.Header.Get("X-Override-Token"), "unpaid_bill")
	if !ok {
		writeProblem(w, http.StatusForbidden, "override_required", "closing as unpaid requires an outlet admin or management PIN override")
		return
	}
	bill, err := h.Store.CloseUnpaidBill(c.CrewID, approver.CrewID, r.PathValue("id"), req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "close_unpaid_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bill)
}

func (h *StaffHandler) refund(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	// Refund cap from pricing settings.
	settings, err := h.Pricing.GetSettings()
	if err != nil {
		settings = pricing.DefaultSettings()
	}
	// Over-cap requires a Management override for refund_gt_cap;
	// under-cap requires Outlet Admin/Management for refund_le_cap.
	// Try the over-cap override first, then the under-cap one.
	approver, overCap := h.Auth.ConsumeOverride(r.Header.Get("X-Override-Token"), "refund_gt_cap")
	if !overCap {
		approver, ok = h.Auth.ConsumeOverride(r.Header.Get("X-Override-Token"), "refund_le_cap")
		if !ok {
			writeProblem(w, http.StatusForbidden, "override_required", "a refund requires an outlet admin or management PIN override")
			return
		}
	}
	approverID := approver.CrewID
	if err := h.Store.RefundBill(c.CrewID, approverID, r.PathValue("id"), req.Amount, req.Reason, settings.RefundCap, overCap); err != nil {
		writeProblem(w, http.StatusBadRequest, "refund_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "refund recorded, paid in cash from the drawer"})
}

func (h *StaffHandler) openDrawer(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		OpeningFloat int64 `json:"opening_float"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	ds, err := h.Store.OpenDrawerSession(c.CrewID, req.OpeningFloat)
	if err != nil {
		writeProblem(w, http.StatusConflict, "drawer_open_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ds)
}

func (h *StaffHandler) closeDrawer(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		CountedCash int64 `json:"counted_cash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	ds, err := h.Store.CloseDrawerSession(c.CrewID, r.PathValue("id"), req.CountedCash)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "drawer_close_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

func (h *StaffHandler) recordHandover(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		WaiterCrewID    string  `json:"waiter_crew_id"`
		Amount          int64   `json:"amount"`
		DrawerSessionID *string `json:"drawer_session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	if err := h.Store.RecordHandover(req.WaiterCrewID, req.Amount, req.DrawerSessionID); err != nil {
		writeProblem(w, http.StatusBadRequest, "handover_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "handover recorded, pending confirmation"})
}

func (h *StaffHandler) confirmHandover(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		ConfirmedAmount int64 `json:"confirmed_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	if err := h.Store.ConfirmHandover(c.CrewID, r.PathValue("id"), req.ConfirmedAmount); err != nil {
		writeProblem(w, http.StatusBadRequest, "handover_confirm_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "handover confirmed"})
}

func (h *StaffHandler) heldCash(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.crewMember(r); !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"crew_id": r.PathValue("id"), "held_cash": h.Store.HeldCash(r.PathValue("id"))})
}

func (h *StaffHandler) postOutage(w http.ResponseWriter, r *http.Request) {
	c, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		BillNumber string              `json:"bill_number"`
		Items      []OrderItemInput    `json:"items"`
		Payments   []PostOutagePayment `json:"payments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	bill, err := h.Store.CreatePostOutageBill(c.CrewID, req.BillNumber, req.Items, req.Payments)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "post_outage_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, bill)
}

func (h *StaffHandler) applyDiscount(w http.ResponseWriter, r *http.Request) {
	_, ok := h.crewMember(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	var req struct {
		DiscountID  string  `json:"discount_id"`
		OrderItemID *string `json:"order_item_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	// Manual discounts require a Management override (BR-R-17).
	approver, ok := h.Auth.ConsumeOverride(r.Header.Get("X-Override-Token"), "manual_discount")
	if !ok {
		writeProblem(w, http.StatusForbidden, "override_required", "a manual discount requires a management PIN override")
		return
	}
	// Amount: for a Bill discount, compute from the Bill total;
	// for an item discount, from that item's line total. This
	// uses the pricing engine's DiscountAmount via a direct
	// query of the discount row.
	var kind, scope string
	var value int64
	err := h.Store.db.QueryRow(`SELECT kind, scope, value FROM discount WHERE id = ?`, req.DiscountID).Scan(&kind, &scope, &value)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "discount_not_found", "discount not found")
		return
	}
	var base int64
	var billID *string
	if req.OrderItemID != nil {
		var unit int64
		var qty int
		if err := h.Store.db.QueryRow(`SELECT unit_price, quantity FROM order_item WHERE id = ?`, *req.OrderItemID).Scan(&unit, &qty); err != nil {
			writeProblem(w, http.StatusNotFound, "order_item_not_found", "order item not found")
			return
		}
		base = unit * int64(qty)
	} else {
		summary, err := h.Store.BillSummary(r.PathValue("id"), h.Pricing)
		if err != nil {
			writeProblem(w, http.StatusNotFound, "bill_not_found", err.Error())
			return
		}
		base = summary.Total
		bid := r.PathValue("id")
		billID = &bid
	}
	var amount int64
	if kind == "percent" {
		amount = (base*value + 50) / 100
	} else {
		amount = value
	}
	if amount > base {
		amount = base
	}
	_ = scope
	if _, err := h.Store.db.Exec(`INSERT INTO applied_discount (id, order_item_id, bill_id, discount_id, amount, override_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, NewID(), req.OrderItemID, billID, req.DiscountID, amount, approver.CrewID, timeNowMillis()); err != nil {
		writeProblem(w, http.StatusBadRequest, "apply_discount_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"discount_id": req.DiscountID, "amount": amount, "approver_crew_id": approver.CrewID})
}

func timeNowMillis() int64 {
	return time.Now().UnixMilli()
}
