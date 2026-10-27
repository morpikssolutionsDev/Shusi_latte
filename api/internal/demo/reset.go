package demo

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/seed"
	"sushilatte/api/internal/vault"
)

// Topic reaches every app: it is public, like the menu, and carries no data (DEMO-AND-SEED §9).
const Topic = "demo"

// Resetter rebuilds the demo under the running API (DEMO-AND-SEED §9): the next database and key
// file are built beside the live ones while the demo keeps running, then the file is swapped in
// about a second (writes get 503 demo_resetting meanwhile), the keys reload, and every app is
// told to start over. Sessions live in the database, so they are gone with it.
type Resetter struct {
	Config  config.Config
	DB      *dbx.DB
	Vault   *vault.Vault
	Log     *slog.Logger
	running atomic.Bool
}

var errRunning = httpx.NewProblem(http.StatusConflict, "reset_running", "Reset demo sedang berjalan")

// Reset runs one reset to the end; a second call while one runs is refused.
func (r *Resetter) Reset(ctx context.Context, by string) error {
	if !r.running.CompareAndSwap(false, true) {
		return errRunning
	}
	defer r.running.Store(false)
	bus := r.DB.Bus()
	bus.Publish(realtime.Event{Topic: Topic, Type: "demo.resetting"})
	started := time.Now()
	next, err := seed.BuildNext(ctx, r.Config, time.Now().Truncate(time.Minute))
	if err != nil {
		bus.Publish(realtime.Event{Topic: Topic, Type: "demo.reset_failed"})
		return err
	}
	built := time.Since(started)
	if err := r.DB.Replace(ctx, func() error { return next.Install(r.Config) }); err != nil {
		bus.Publish(realtime.Event{Topic: Topic, Type: "demo.reset_failed"})
		return err
	}
	if err := r.Vault.Reload(); err != nil {
		return err
	}
	r.Log.Info("demo reset", "by", by, "build", built.Round(time.Millisecond), "total", time.Since(started).Round(time.Millisecond))
	bus.Publish(realtime.Event{Topic: Topic, Type: "demo.reset"})
	return nil
}

// handle is POST /demo/reset: Management with their PIN (demo_reset). It answers 202 at once and
// resets in the background; apps follow the `demo` topic.
func (r *Resetter) handle(w http.ResponseWriter, req *http.Request) error {
	p := auth.FromContext(req.Context())
	token := auth.OverrideToken(req)
	err := r.DB.Write(req.Context(), func(tx *dbx.Tx) error {
		_, err := auth.ConsumeOverride(req.Context(), tx, time.Now(), p, token, auth.ActionDemoReset,
			audit.Entry{Action: "demo.reset", Entity: "demo", EntityID: "demo"})
		return err
	})
	if err != nil {
		return err
	}
	if r.running.Load() {
		return errRunning
	}
	go func() {
		if err := r.Reset(context.Background(), p.CrewID); err != nil {
			r.Log.Error("demo reset failed", "err", err)
		}
	}()
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"status": "resetting", "at": platform.Millis(time.Now())})
	return nil
}
