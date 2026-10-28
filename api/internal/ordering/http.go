package ordering

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/httpx"
)

// GuestHeader carries the Customer's guest token. It is a header rather than a cookie so each
// browser tab can be a different Customer (several Customers on one laptop in the demo).
const GuestHeader = "X-Guest-Token"

type guestKey struct{}

type guestResult struct {
	guest *Guest
	err   error
}

// Middleware resolves the guest token, if one is sent. Handlers that need a Customer call
// GuestFrom; others ignore it.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(GuestHeader)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		g, err := s.Authenticate(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), guestKey{}, guestResult{g, err})))
	})
}

// GuestFrom returns the authenticated Customer, or the reason there is none.
func GuestFrom(ctx context.Context) (*Guest, error) {
	res, ok := ctx.Value(guestKey{}).(guestResult)
	if !ok {
		return nil, httpx.NewProblem(http.StatusUnauthorized, "guest_required", "Pindai QR meja untuk mulai memesan")
	}
	return res.guest, res.err
}

type Handlers struct{ Svc *Service }

func (h Handlers) Routes(r chi.Router) {
	r.Get("/public/tables/{code}", httpx.Handler(h.lookup))
	r.Post("/public/tables/{code}/join", httpx.Handler(h.join))
	r.Post("/public/tables/{code}/rejoin", httpx.Handler(h.rejoin))
	r.Post("/public/tables/{code}/not-mine", httpx.Handler(h.notMine))
	r.Get("/public/catalog", httpx.Handler(h.menu))
	r.Get("/public/bills/me", h.guest(h.me))
	r.Post("/public/cart/quote", h.guest(h.quote))
	r.Post("/public/orders", h.guest(h.placeOrder))
	r.Patch("/public/orders/{id}", h.guest(h.editOrder))
	r.Delete("/public/orders/{id}", h.guest(h.cancelOrder))
	r.Post("/public/waiter-calls", h.guest(h.callWaiter))
}

func (h Handlers) guest(fn func(http.ResponseWriter, *http.Request, *Guest) error) http.HandlerFunc {
	return httpx.Handler(func(w http.ResponseWriter, r *http.Request) error {
		g, err := GuestFrom(r.Context())
		if err != nil {
			return err
		}
		return fn(w, r, g)
	})
}

func (h Handlers) lookup(w http.ResponseWriter, r *http.Request) error {
	st, err := h.Svc.Lookup(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, st)
	return nil
}

func (h Handlers) join(w http.ResponseWriter, r *http.Request) error {
	var in JoinInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	s, err := h.Svc.Join(r.Context(), chi.URLParam(r, "code"), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, s)
	return nil
}

func (h Handlers) rejoin(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CustomerID string `json:"customerId"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	s, err := h.Svc.Rejoin(r.Context(), chi.URLParam(r, "code"), in.CustomerID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, s)
	return nil
}

func (h Handlers) notMine(w http.ResponseWriter, r *http.Request) error {
	if err := h.Svc.NotMine(r.Context(), chi.URLParam(r, "code")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h Handlers) menu(w http.ResponseWriter, r *http.Request) error {
	m, err := h.Svc.Menu(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSONWithETag(w, r, m)
	return nil
}

func (h Handlers) me(w http.ResponseWriter, r *http.Request, g *Guest) error {
	m, err := h.Svc.Me(r.Context(), g)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}

type itemsBody struct {
	Items []ItemInput `json:"items"`
}

func (h Handlers) quote(w http.ResponseWriter, r *http.Request, g *Guest) error {
	var in itemsBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	q, err := h.Svc.Quote(r.Context(), g, in.Items)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, q)
	return nil
}

func (h Handlers) placeOrder(w http.ResponseWriter, r *http.Request, g *Guest) error {
	var in itemsBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.Svc.PlaceOrder(r.Context(), g, in.Items)
	if err != nil {
		return err
	}
	return h.respondMe(w, r, g, http.StatusCreated, id)
}

func (h Handlers) editOrder(w http.ResponseWriter, r *http.Request, g *Guest) error {
	var in itemsBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id := chi.URLParam(r, "id")
	if err := h.Svc.EditOrder(r.Context(), g, id, in.Items); err != nil {
		return err
	}
	return h.respondMe(w, r, g, http.StatusOK, id)
}

func (h Handlers) cancelOrder(w http.ResponseWriter, r *http.Request, g *Guest) error {
	id := chi.URLParam(r, "id")
	if err := h.Svc.CancelOrder(r.Context(), g, id); err != nil {
		return err
	}
	return h.respondMe(w, r, g, http.StatusOK, id)
}

// respondMe answers an Order change with the Customer's refreshed view, so the phone renders
// exactly what the server decided.
func (h Handlers) respondMe(w http.ResponseWriter, r *http.Request, g *Guest, status int, orderID string) error {
	m, err := h.Svc.Me(r.Context(), g)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, status, struct {
		OrderID string `json:"orderId"`
		Me
	}{orderID, m})
	return nil
}

func (h Handlers) callWaiter(w http.ResponseWriter, r *http.Request, g *Guest) error {
	c, err := h.Svc.CallWaiter(r.Context(), g)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
	return nil
}
