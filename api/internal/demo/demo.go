// Package demo serves the Demo mode helpers (DEMO-AND-SEED §2). It is mounted only when
// SL_DEMO_MODE is on.
package demo

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/seed"
)

type account struct {
	seed.DemoAccount
	ImageKey string `json:"imageKey,omitempty"`
}

func Routes(db *dbx.DB) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/accounts", httpx.Handler(func(w http.ResponseWriter, req *http.Request) error {
			crew, err := db.Read().ListAllCrew(req.Context())
			if err != nil {
				return err
			}
			keys := map[string]string{}
			for _, c := range crew {
				keys[c.CrewID] = c.ImageKey.String
			}
			out := make([]account, 0, len(seed.DemoCrew))
			for _, a := range seed.DemoCrew {
				out = append(out, account{DemoAccount: a, ImageKey: keys[a.CrewID]})
			}
			httpx.WriteJSON(w, http.StatusOK, out)
			return nil
		}))
	}
}
