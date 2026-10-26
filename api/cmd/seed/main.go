// Command seed rebuilds the demo database anchored at "now" (DEMO-AND-SEED §9).
// Stop the API first: the live file is replaced.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	_ "time/tzdata"

	"sushilatte/api/internal/config"
	"sushilatte/api/internal/seed"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		fail(err)
	}
	t0 := time.Now().Truncate(time.Minute)
	next := filepath.Join(cfg.DataDir, "sushi-latte.next.db")
	started := time.Now()
	opts := seed.Options{DBPath: next, ImagesDir: cfg.ImagesDir(), AssetsDir: cfg.AssetsDir, T0: t0}
	if err := seed.Build(context.Background(), opts); err != nil {
		fail(err)
	}
	if err := seed.Install(next, cfg.DatabasePath()); err != nil {
		fail(err)
	}
	fmt.Printf("seeded %s (T0 %s) in %s\n", cfg.DatabasePath(), t0.Format(time.RFC3339), time.Since(started).Round(time.Millisecond))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "seed:", err)
	os.Exit(1)
}
