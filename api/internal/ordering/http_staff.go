package ordering

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/httpx"
)

// StaffRoutes mounts the Waiter and Cashier endpoints (TRD §6.1 Ordering). Authorisation
// follows BRD §5.1: Waiter or Cashier Station, Outlet Admin and Management everywhere.
func (h Handlers) StaffRoutes(r chi.Router) {
	floor := auth.Require(auth.Stations(auth.StationWaiter, auth.StationCashier))
	r.With(floor).Get("/waiter/feed", h.staff(h.feed))
	r.With(floor).Get("/floor", h.staff(h.floor))
	r.With(floor).Get("/bills/{id}", h.staff(h.bill))
	r.With(floor).Post("/orders", h.staff(h.direct))
	r.With(floor).Post("/orders/{id}/claim", h.staff(h.claim))
	r.With(floor).Post("/orders/{id}/takeover", h.staff(h.takeover))
	r.With(auth.Require(auth.Roles(auth.RoleOutletAdmin, auth.RoleManagement))).Post("/orders/{id}/reassign", h.staff(h.reassign))
	r.With(floor).Post("/orders/{id}/items", h.staff(h.addItems))
	r.With(floor).Post("/orders/{id}/confirm", h.staff(h.confirm))
	r.With(floor).Post("/order-items/{id}/cancel", h.staff(h.cancelItem))
	r.With(floor).Post("/order-items/serve", h.staff(h.serve))
	r.With(floor).Post("/order-items/{id}/remake", h.staff(h.remake))
	r.With(floor).Post("/order-items/{id}/void", h.staff(h.void))
	r.With(floor).Post("/waiter-calls/{id}/answer", h.staff(h.answerCall))

	// Bills and payment (OP-09, OP-10).
	r.With(floor).Get("/bills/{id}/billing", h.staff(h.billing))
	r.With(floor).Post("/bills/{id}/settle", h.staff(h.settle))
	r.With(floor).Post("/bills/{id}/unlock", h.staff(h.unlock))
	r.With(floor).Post("/bills/{id}/discount", h.staff(h.manualDiscount))
	r.With(floor).Post("/bills/{id}/payments", h.staff(h.pay))
	r.With(floor).Post("/bills/{id}/close", h.staff(h.closeBill))
	r.With(floor).Post("/bills/{id}/unpaid", h.staff(h.unpaid))
	r.With(floor).Post("/bills/{id}/move", h.staff(h.move))
	r.With(floor).Post("/bills/merge", h.staff(h.merge))
	r.With(floor).Post("/bills/{id}/split", h.staff(h.split))
	r.With(floor).Post("/bills/{id}/receipts", h.staff(h.receipt))
	r.With(auth.Require(auth.Roles(auth.RoleManagement))).Get("/reports/daily-sales", h.staff(h.dailySales))

	// Kitchen and Bar displays (OP-05): Kitchen Crew, or a Kitchen/Bar screen-mode session.
	kitchen := auth.Require(func(p *auth.Principal) bool {
		if p.Kind == "screen" {
			return p.Screen == auth.ScreenKitchen || p.Screen == auth.ScreenBar
		}
		return auth.Stations(auth.StationKitchen)(p)
	})
	r.With(kitchen).Get("/prep-areas/{id}/queue", h.staff(h.queue))
	r.With(kitchen).Post("/order-items/start", h.staff(h.startItems))
	r.With(kitchen).Post("/order-items/ready", h.staff(h.readyItems))
	r.With(kitchen).Post("/order-items/{id}/kitchen-cancel", h.staff(h.kitchenCancel))
	r.With(auth.Require(auth.Roles(auth.RoleManagement))).Get("/reports/prep-time", h.staff(h.prepTime))

	// Cashier (OP-11..OP-14): Cashier Station, Outlet Admin and Management.
	cashier := auth.Require(auth.Stations(auth.StationCashier))
	r.With(cashier).Get("/cashier/bills", h.staff(h.cashierBills))
	r.With(cashier).Get("/drawer", h.staff(h.drawer))
	r.With(cashier).Post("/drawer-sessions", h.staff(h.openDrawer))
	r.With(cashier).Post("/drawer-sessions/current/close", h.staff(h.closeDrawer))
	r.With(cashier).Post("/cash-handovers", h.staff(h.handover))
	r.With(floor).Get("/crew/{id}/held-cash", h.staff(h.heldCash))
	r.With(cashier).Post("/payments/{id}/verify", h.staff(h.verifyPayment))
	r.With(cashier).Post("/payments/{id}/write-off", h.staff(h.writeOff))
	r.With(cashier).Post("/bills/{id}/refunds", h.staff(h.refund))
	r.With(cashier).Post("/bills", h.staff(h.takeaway))
	r.With(cashier).Post("/post-outage/bills", h.staff(h.postOutage))
	r.With(auth.Require(auth.Roles(auth.RoleManagement))).Get("/reports/operations", h.staff(h.operations))
}

func (h Handlers) cashierBills(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	b, err := h.Svc.CashierBills(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, b)
	return nil
}

func (h Handlers) drawer(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	d, err := h.Svc.Drawer(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

func (h Handlers) openDrawer(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		OpeningFloat int64 `json:"openingFloat"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.OpenDrawer(r.Context(), p, in.OpeningFloat); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) closeDrawer(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		CountedCash int64 `json:"countedCash"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	c, err := h.Svc.CloseDrawer(r.Context(), p, in.CountedCash)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, c)
	return nil
}

func (h Handlers) handover(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		WaiterID       string `json:"waiterId"`
		ReceivedAmount int64  `json:"receivedAmount"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	ho, err := h.Svc.ReceiveHandover(r.Context(), p, in.WaiterID, in.ReceivedAmount)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, ho)
	return nil
}

func (h Handlers) heldCash(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	held, err := h.Svc.HeldBy(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, held)
	return nil
}

func (h Handlers) verifyPayment(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.VerifyPayment(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) writeOff(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in reasonBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.WriteOffPayment(r.Context(), p, chi.URLParam(r, "id"), in.Reason); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) refund(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Refund(r.Context(), p, chi.URLParam(r, "id"), in.Amount, in.Reason, auth.OverrideToken(r)); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) takeaway(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in TakeawayInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.Svc.CreateTakeaway(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"billId": id})
	return nil
}

func (h Handlers) postOutage(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in OutageInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.Svc.PostOutage(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"billId": id})
	return nil
}

func (h Handlers) operations(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	o, err := h.Svc.Operations(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, o)
	return nil
}

func (h Handlers) queue(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	q, err := h.Svc.Queue(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, q)
	return nil
}

type itemIDsBody struct {
	ItemIDs []string `json:"itemIds"`
}

func (h Handlers) startItems(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in itemIDsBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.StartItems(r.Context(), p, in.ItemIDs); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) readyItems(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in itemIDsBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.ReadyItems(r.Context(), p, in.ItemIDs); err != nil {
		return err
	}
	return noContent(w)
}

// The Kitchen Cancels Confirmed or Preparing items (BR-KT-04, G-07).
func (h Handlers) kitchenCancel(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.CancelItem(r.Context(), p, chi.URLParam(r, "id"), in.Reason, in.Note, []string{"confirmed", "preparing"}); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) prepTime(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	rep, err := h.Svc.PrepTime(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
	return nil
}

func (h Handlers) billing(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	b, err := h.Svc.Billing(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, b)
	return nil
}

func (h Handlers) settle(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.Settle(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) unlock(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.Unlock(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) manualDiscount(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		DiscountID string `json:"discountId"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.ApplyManualDiscount(r.Context(), p, chi.URLParam(r, "id"), in.DiscountID, auth.OverrideToken(r)); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) pay(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in PayInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	res, err := h.Svc.Pay(r.Context(), p, chi.URLParam(r, "id"), in, auth.OverrideToken(r))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
	return nil
}

func (h Handlers) closeBill(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		SubBillID        string `json:"subBillId"`
		PayNowServeLater bool   `json:"payNowServeLater"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Close(r.Context(), p, chi.URLParam(r, "id"), in.SubBillID, in.PayNowServeLater); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) unpaid(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in reasonBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.CloseUnpaid(r.Context(), p, chi.URLParam(r, "id"), in.Reason, auth.OverrideToken(r)); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) move(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		FromTableID string `json:"fromTableId"`
		ToTableID   string `json:"toTableId"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Move(r.Context(), p, chi.URLParam(r, "id"), in.FromTableID, in.ToTableID); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) merge(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		IntoBillID string   `json:"intoBillId"`
		BillIDs    []string `json:"billIds"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Merge(r.Context(), p, in.IntoBillID, in.BillIDs); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) split(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in SplitInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Split(r.Context(), p, chi.URLParam(r, "id"), in); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) receipt(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		Copy      string `json:"copy"`
		SubBillID string `json:"subBillId"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	rc, err := h.Svc.Receipt(r.Context(), p, chi.URLParam(r, "id"), in.SubBillID, in.Copy)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, rc)
	return nil
}

func (h Handlers) dailySales(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	d, err := h.Svc.DailySales(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

func (h Handlers) staff(fn func(http.ResponseWriter, *http.Request, *auth.Principal) error) http.HandlerFunc {
	return httpx.Handler(func(w http.ResponseWriter, r *http.Request) error {
		return fn(w, r, auth.FromContext(r.Context()))
	})
}

func noContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h Handlers) feed(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	f, err := h.Svc.Feed(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, f)
	return nil
}

func (h Handlers) floor(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	f, err := h.Svc.Floor(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, f)
	return nil
}

func (h Handlers) bill(w http.ResponseWriter, r *http.Request, _ *auth.Principal) error {
	b, err := h.Svc.Bill(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, b)
	return nil
}

func (h Handlers) direct(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in DirectInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	source := "waiter"
	if p.HasStation(auth.StationCashier) && !p.HasStation(auth.StationWaiter) {
		source = "cashier"
	}
	id, err := h.Svc.DirectOrder(r.Context(), p, source, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"orderId": id})
	return nil
}

func (h Handlers) claim(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.Claim(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) takeover(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.TransferClaim(r.Context(), p, chi.URLParam(r, "id"), ""); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) reassign(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		ToCrewID string `json:"toCrewId"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if in.ToCrewID == "" {
		return invalid("Pilih pelayan tujuan.")
	}
	if err := h.Svc.TransferClaim(r.Context(), p, chi.URLParam(r, "id"), in.ToCrewID); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) addItems(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in itemsBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.AddItems(r.Context(), p, chi.URLParam(r, "id"), in.Items); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) confirm(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.Confirm(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}

// Waiters and Cashiers Cancel Pending or Confirmed items (BR-WT-04); the Kitchen has its own
// route for Confirmed or Preparing ones.
func (h Handlers) cancelItem(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.CancelItem(r.Context(), p, chi.URLParam(r, "id"), in.Reason, in.Note, []string{ItemPending, "confirmed"}); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) serve(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in struct {
		ItemIDs []string `json:"itemIds"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Serve(r.Context(), p, in.ItemIDs); err != nil {
		return err
	}
	return noContent(w)
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func (h Handlers) remake(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in reasonBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.Svc.Remake(r.Context(), p, chi.URLParam(r, "id"), in.Reason, auth.OverrideToken(r))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"itemId": id})
	return nil
}

func (h Handlers) void(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	var in reasonBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.Svc.Void(r.Context(), p, chi.URLParam(r, "id"), in.Reason, auth.OverrideToken(r)); err != nil {
		return err
	}
	return noContent(w)
}

func (h Handlers) answerCall(w http.ResponseWriter, r *http.Request, p *auth.Principal) error {
	if err := h.Svc.AnswerCall(r.Context(), p, chi.URLParam(r, "id")); err != nil {
		return err
	}
	return noContent(w)
}
