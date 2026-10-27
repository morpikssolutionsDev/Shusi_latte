// Package demo serves the Demo mode helpers (DEMO-AND-SEED §2). It is mounted only when
// SL_DEMO_MODE is on.
package demo

import (
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
		// Launcher cards: every Table with its QR token and scenario label (DEMO-AND-SEED §2).
		r.Get("/tables", httpx.Handler(func(w http.ResponseWriter, req *http.Request) error {
			tables, err := outlet.NewService(db, platform.SystemClock{}).Tables(req.Context())
			if err != nil {
				return err
			}
			out := make([]launcherTable, 0, len(tables))
			for _, t := range tables {
				out = append(out, launcherTable{Number: t.Number, Label: t.Label, QRToken: t.QRToken, Active: t.Active, Scenario: seed.TableScenarios[t.Number]})
			}
			httpx.WriteJSON(w, http.StatusOK, out)
			return nil
		}))
	}
}

type launcherTable struct {
	Number   int64  `json:"number"`
	Label    string `json:"label"`
	QRToken  string `json:"qrToken"`
	Active   bool   `json:"active"`
	Scenario string `json:"scenario"`
}
