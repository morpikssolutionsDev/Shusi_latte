// Command server runs the Sushi Latte API, WebSocket hub and scheduled jobs (TR-ARC-01).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // Asia/Jakarta on Windows hosts without a zoneinfo database

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/demo"
	"sushilatte/api/internal/hris"
	"sushilatte/api/internal/jobs"
	"sushilatte/api/internal/ordering"
	"sushilatte/api/internal/outlet"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/server"
	"sushilatte/api/internal/vault"
)

func main() {
	cfg := config.Load()
	log := newLogger(cfg.LogLevel)
	if err := run(cfg, log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	bus := realtime.NewBus()
	db, err := dbx.Open(ctx, cfg.DatabasePath(), bus)
	if err != nil {
		return err
	}
	defer db.Close()

	clock := platform.SystemClock{}
	authSvc := auth.NewService(db, clock)
	if cfg.MasterKey == "" {
		log.Warn("SL_MASTER_KEY not set; using the prototype key file", "path", cfg.MasterKeyPath())
	}
	masterKey, err := vault.LoadMasterKey(cfg.MasterKey, cfg.MasterKeyPath())
	if err != nil {
		return err
	}
	keys, err := vault.Open(cfg.KeysPath(), masterKey)
	if err != nil {
		return err
	}

	hr := hris.NewService(db, clock, authSvc)
	all := append(maintenanceJobs(db, clock), customerJobs(db, clock, keys, log)...)
	// Crew still clocked in 2 h after their shift ends are clocked out and flagged (BR-HR-06).
	all = append(all, jobs.Every("attendance.auto_clock_out", 5*time.Minute, func(ctx context.Context) error {
		n, err := hr.AutoClockOut(ctx)
		if n > 0 {
			log.Info("auto clock-out", "count", n)
		}
		return err
	}))
	reset := &demo.Resetter{Config: cfg, DB: db, Vault: keys, Log: log}
	if cfg.DemoMode {
		// The demo starts over every night with T0 = 03.00 (DEMO-AND-SEED §9).
		all = append(all, jobs.DailyAt("demo.reset", 3, 0, func(ctx context.Context) error { return reset.Reset(ctx, "nightly") }))
	}
	sched := jobs.NewScheduler(log, all...)
	sched.Start(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(server.Deps{Config: cfg, DB: db, Auth: authSvc, Vault: keys, Clock: clock, Log: log, Reset: reset}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "data", cfg.DataDir, "demo", cfg.DemoMode)
		errc <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	sched.Wait()
	return err
}

func maintenanceJobs(db *dbx.DB, clock platform.Clock) []jobs.Job {
	return []jobs.Job{
		jobs.Every("cleanup.sessions", time.Hour, func(ctx context.Context) error {
			return db.Write(ctx, func(tx *dbx.Tx) error {
				now := platform.Millis(clock.Now())
				if _, err := tx.DeleteExpiredSessions(ctx, now); err != nil {
					return err
				}
				_, err := tx.DeleteExpiredOverrideTokens(ctx, now)
				return err
			})
		}),
		jobs.Every("cleanup.idempotency", time.Hour, func(ctx context.Context) error {
			return db.Write(ctx, func(tx *dbx.Tx) error {
				_, err := tx.DeleteExpiredIdempotencyKeys(ctx, platform.Millis(clock.Now().Add(-24*time.Hour)))
				return err
			})
		}),
	}
}

// customerJobs are the TRD §19 jobs for Bills and Customer data.
func customerJobs(db *dbx.DB, clock platform.Clock, keys *vault.Vault, log *slog.Logger) []jobs.Job {
	orders := ordering.NewService(db, clock, keys)
	discounts := outlet.NewService(db, clock)
	lastPromos := ""
	return []jobs.Job{
		jobs.Every("orders.escalate", 15*time.Second, orders.Escalate),
		jobs.Every("bills.close_empty", time.Minute, func(ctx context.Context) error {
			closed, err := orders.CloseEmptyBills(ctx)
			if len(closed) > 0 {
				log.Info("closed empty bills", "count", len(closed))
			}
			return err
		}),
		// Scheduled discounts start and stop without a write, so menus are told at each edge.
		jobs.Every("promos.edges", time.Minute, func(ctx context.Context) error {
			all, err := discounts.Discounts(ctx)
			if err != nil {
				return err
			}
			var ids []string
			for _, d := range outlet.ActiveDiscounts(all, clock.Now()) {
				ids = append(ids, d.ID)
			}
			now := strings.Join(ids, ",")
			if now != lastPromos {
				lastPromos = now
				db.Bus().Publish(realtime.Event{Topic: "menu", Type: "promo.changed"})
			}
			return nil
		}),
		jobs.Every("customers.anonymise", time.Hour, func(ctx context.Context) error {
			n, err := orders.Anonymise(ctx)
			if n > 0 {
				log.Info("anonymised bills", "count", n)
			}
			return err
		}),
		jobs.DailyAt("keys.destroy", 4, 30, func(context.Context) error {
			gone, err := keys.DestroyBefore(clock.Now())
			if len(gone) > 0 {
				log.Info("destroyed data keys", "keys", gone)
			}
			return err
		}),
	}
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	_ = l.UnmarshalText([]byte(level))
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
