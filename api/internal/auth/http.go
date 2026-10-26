package auth

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
)

const CookieName = "sl_session"

type ctxKey struct{}

// FromContext returns the request's Principal, or nil for anonymous requests.
func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}

// Middleware resolves the session cookie, if any, into a Principal on the context.
// Missing or invalid sessions leave the request anonymous; Require decides access.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err == nil && c.Value != "" {
			if p, err := s.Authenticate(r.Context(), c.Value); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, p))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Require allows only signed-in principals passing check (TR-AUTH-05). Authorisation is
// enforced here on the server; the UI only hides what a user cannot do.
func Require(check func(*Principal) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := FromContext(r.Context())
			if p == nil {
				httpx.WriteError(w, httpx.ErrUnauthorised)
				return
			}
			if check != nil && !check(p) {
				httpx.WriteError(w, httpx.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Roles builds a check that passes for any of the given roles.
func Roles(roles ...string) func(*Principal) bool {
	return func(p *Principal) bool { return p.Kind == "crew" && p.HasRole(roles...) }
}

// Stations builds a check that passes for Crew working any of the given Stations.
// Management and Outlet Admin pass too: they may act at any Station.
func Stations(stations ...string) func(*Principal) bool {
	return func(p *Principal) bool {
		if p.Kind != "crew" {
			return false
		}
		if p.HasRole(RoleManagement, RoleOutletAdmin) {
			return true
		}
		for _, st := range stations {
			if p.HasStation(st) {
				return true
			}
		}
		return false
	}
}

// Handlers exposes /auth endpoints.
type Handlers struct {
	svc          *Service
	limiter      *ipLimiter
	secureCookie bool
	clock        platform.Clock
}

func NewHandlers(svc *Service, secureCookie bool, clock platform.Clock) *Handlers {
	return &Handlers{svc: svc, limiter: newIPLimiter(20, time.Minute), secureCookie: secureCookie, clock: clock}
}

func (h *Handlers) Routes(r chi.Router) {
	r.Post("/login", httpx.Handler(h.login))
	r.Group(func(r chi.Router) {
		r.Use(Require(nil))
		r.Get("/me", httpx.Handler(h.me))
		r.Post("/logout", httpx.Handler(h.logout))
		r.Post("/override", httpx.Handler(h.override))
		r.Post("/screen-mode", httpx.Handler(h.screenMode))
	})
}

type loginRequest struct {
	CrewID      string `json:"crewId"`
	PIN         string `json:"pin"`
	DeviceLabel string `json:"deviceLabel,omitempty"`
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) error {
	if !h.limiter.allow(clientIP(r), h.clock.Now()) {
		return httpx.NewProblem(http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan, coba lagi sebentar")
	}
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	token, p, err := h.svc.Login(r.Context(), req.CrewID, req.PIN, req.DeviceLabel)
	if err != nil {
		return err
	}
	h.setCookie(w, token, p.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, p)
	return nil
}

func (h *Handlers) me(w http.ResponseWriter, r *http.Request) error {
	httpx.WriteJSON(w, http.StatusOK, FromContext(r.Context()))
	return nil
}

func (h *Handlers) logout(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.Logout(r.Context(), FromContext(r.Context())); err != nil {
		return err
	}
	h.setCookie(w, "", 0)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type overrideRequest struct {
	CrewID string `json:"crewId"`
	PIN    string `json:"pin"`
	Action string `json:"action"`
}

type overrideResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (h *Handlers) override(w http.ResponseWriter, r *http.Request) error {
	if !h.limiter.allow(clientIP(r), h.clock.Now()) {
		return httpx.NewProblem(http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan, coba lagi sebentar")
	}
	var req overrideRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	token, expires, err := h.svc.IssueOverride(r.Context(), FromContext(r.Context()), req.CrewID, req.PIN, req.Action)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, overrideResponse{Token: token, ExpiresAt: expires})
	return nil
}

type screenModeRequest struct {
	Screen      string `json:"screen"`
	DeviceLabel string `json:"deviceLabel,omitempty"`
}

func (h *Handlers) screenMode(w http.ResponseWriter, r *http.Request) error {
	var req screenModeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	token, p, err := h.svc.EnterScreenMode(r.Context(), FromContext(r.Context()), req.Screen, req.DeviceLabel)
	if err != nil {
		return err
	}
	h.setCookie(w, token, p.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, p)
	return nil
}

func (h *Handlers) setCookie(w http.ResponseWriter, token string, expiresAt int64) {
	c := &http.Cookie{
		Name: CookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	}
	if token == "" {
		c.MaxAge = -1
	} else {
		c.Expires = time.UnixMilli(expiresAt)
	}
	http.SetCookie(w, c)
}

// OverrideToken reads the X-Override-Token header (TR-API-03).
func OverrideToken(r *http.Request) string { return r.Header.Get("X-Override-Token") }

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipLimiter is a fixed-window counter per client IP for PIN attempts (TR-AUTH-02).
type ipLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newIPLimiter(limit int, per time.Duration) *ipLimiter {
	return &ipLimiter{limit: limit, window: per, hits: map[string]*window{}}
}

func (l *ipLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.hits[ip]
	if !ok || now.Sub(w.start) >= l.window {
		l.hits[ip] = &window{start: now, count: 1}
		return true
	}
	w.count++
	return w.count <= l.limit
}
