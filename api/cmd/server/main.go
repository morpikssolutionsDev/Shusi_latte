// Command server runs the Sushi Latte API, WebSocket hub and scheduled jobs (TR-ARC-01).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Asia/Jakarta on Windows hosts without a zoneinfo database

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/jobs"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/server"
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

	sched := jobs.NewScheduler(log, maintenanceJobs(db, clock)...)
	sched.Start(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(server.Deps{Config: cfg, DB: db, Auth: authSvc, Clock: clock, Log: log}),
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

func newLogger(level string) *slog.Logger {
	var l slog.Level
	_ = l.UnmarshalText([]byte(level))
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
