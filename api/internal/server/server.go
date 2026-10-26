// Package server wires routes, middleware and the WebSocket hub into one http.Handler.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"sushilatte/api/internal/adminapi"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/catalog"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/crew"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/demo"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/media"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

type Deps struct {
	Config config.Config
	DB     *dbx.DB
	Auth   *auth.Service
	Clock  platform.Clock
	Log    *slog.Logger
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "demo": d.Config.DemoMode})
	})

	hub := realtime.NewHub(d.DB.Bus(), func(req *http.Request) (*realtime.Subscriber, error) {
		c, err := req.Cookie(auth.CookieName)
		if err != nil {
			return nil, httpx.ErrUnauthorised
		}
		p, err := d.Auth.Authenticate(req.Context(), c.Value)
		if err != nil {
			return nil, err
		}
		return &realtime.Subscriber{ID: p.CrewID, Allowed: realtime.TopicMatcher(topicsFor(p)...)}, nil
	}, d.Config.OriginPatterns, d.Log)
	r.Handle("/ws", hub)

	// Uploaded images are content-addressed (img/<hash>) and never change in place (TR-DM-09).
	images := http.StripPrefix("/media/images/", http.FileServer(http.Dir(d.Config.ImagesDir())))
	r.Get("/media/images/*", func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(chi.URLParam(req, "*"), "img/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		images.ServeHTTP(w, req)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(d.Auth.Middleware)
		r.Use(httpx.Idempotency(idempotencyStore{d.DB}, idempotencyScope, d.Clock.Now))
		r.Route("/auth", auth.NewHandlers(d.Auth, d.Config.SecureCookie, d.Clock).Routes)
		admin := &adminapi.Handlers{
			DB:      d.DB,
			Catalog: catalog.NewService(d.DB, d.Clock),
			Outlet:  outlet.NewService(d.DB, d.Clock),
			Crew:    crew.NewService(d.DB, d.Clock),
			Media:   media.NewStore(d.Config.ImagesDir()),
		}
		admin.Routes(r)
		if d.Config.DemoMode {
			r.Route("/demo", demo.Routes(d.DB))
		}
	})
	return r
}

// topicsFor decides which realtime topics a principal may subscribe to (TR-RT-02).
func topicsFor(p *auth.Principal) []string {
	if p.Kind == "screen" {
		switch p.Screen {
		case auth.ScreenKitchen, auth.ScreenBar:
			return []string{"prep-area:*", "menu"}
		case auth.ScreenDisplay:
			return []string{"display", "menu"}
		case auth.ScreenHRIS:
			return []string{"hris"}
		}
		return nil
	}
	if p.HasRole(auth.RoleManagement, auth.RoleOutletAdmin) {
		return []string{"*"}
	}
	topics := []string{"menu"}
	if p.HasStation(auth.StationWaiter) || p.HasStation(auth.StationCashier) {
		topics = append(topics, "waiters", "bill:*", "cash")
	}
	if p.HasStation(auth.StationKitchen) {
		topics = append(topics, "prep-area:*")
	}
	return topics
}

func idempotencyScope(r *http.Request) string {
	if p := auth.FromContext(r.Context()); p != nil {
		return "session:" + p.SessionID
	}
	return "ip:" + r.RemoteAddr
}

type idempotencyStore struct{ db *dbx.DB }

func (s idempotencyStore) Get(r *http.Request, scope, key string) (*store.IdempotencyKey, error) {
	row, err := s.db.Read().GetIdempotentResponse(r.Context(), store.GetIdempotentResponseParams{Scope: scope, Key: key})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s idempotencyStore) Save(r *http.Request, row store.SaveIdempotentResponseParams) error {
	return s.db.Write(context.WithoutCancel(r.Context()), func(tx *dbx.Tx) error {
		return tx.SaveIdempotentResponse(r.Context(), row)
	})
}
