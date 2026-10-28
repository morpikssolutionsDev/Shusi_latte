// Package demo serves the Demo mode helpers (DEMO-AND-SEED §2). It is mounted only when
// SL_DEMO_MODE is on.
package demo

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/seed"
)

type account struct {
	seed.DemoAccount
	ImageKey string `json:"imageKey,omitempty"`
}

func Routes(db *dbx.DB, reset *Resetter) func(chi.Router) {
	return func(r chi.Router) {
		if reset != nil {
			r.With(auth.Require(auth.Roles(auth.RoleManagement))).Post("/reset", httpx.Handler(reset.handle))
		}
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
		// What the seed built: the clean start, or the story the tours are written for (§9).
		r.Get("/state", httpx.Handler(func(w http.ResponseWriter, req *http.Request) error {
			httpx.WriteJSON(w, http.StatusOK, demoState(req.Context(), db))
			return nil
		}))
		// Launcher cards: every Table with its code (its QR opens /table-01) and, in the story, its
		// scenario label (DEMO-AND-SEED §2).
		r.Get("/tables", httpx.Handler(func(w http.ResponseWriter, req *http.Request) error {
			tables, err := outlet.NewService(db, platform.SystemClock{}).Tables(req.Context())
			if err != nil {
				return err
			}
			story := demoState(req.Context(), db).Story
			out := make([]launcherTable, 0, len(tables))
			for _, t := range tables {
				row := launcherTable{Number: t.Number, Label: t.Label, Code: t.Code, Active: t.Active}
				if story {
					row.Scenario = seed.TableScenarios[t.Number]
				}
				out = append(out, row)
			}
			httpx.WriteJSON(w, http.StatusOK, out)
			return nil
		}))
	}
}

type state struct {
	Story bool  `json:"story"`
	T0    int64 `json:"t0,omitempty"`
}

func demoState(ctx context.Context, db *dbx.DB) state {
	s, err := db.Read().GetDemoSeed(ctx)
	if err != nil {
		return state{} // not seeded by the demo seed: treat as clean
	}
	return state{Story: s.Story == 1, T0: s.T0}
}

type launcherTable struct {
	Number   int64  `json:"number"`
	Label    string `json:"label"`
	Code     string `json:"code"`
	Active   bool   `json:"active"`
	Scenario string `json:"scenario"`
}
