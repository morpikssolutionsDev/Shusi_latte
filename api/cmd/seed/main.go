// Command seed rebuilds the demo database anchored at "now" (DEMO-AND-SEED §9).
// Stop the API first: the live file is replaced. A running API resets itself instead
// (Admin → Settings → Reset demo, or the nightly job).
package main

import (
	"context"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"sushilatte/api/internal/config"
	"sushilatte/api/internal/seed"
)

func main() {
	cfg := config.Load()
	t0 := time.Now().Truncate(time.Minute)
	started := time.Now()
	next, err := seed.BuildNext(context.Background(), cfg, t0)
	if err != nil {
		fail(err)
	}
	if err := next.Install(cfg); err != nil {
		fail(err)
	}
	fmt.Printf("seeded %s (T0 %s) in %s\n", cfg.DatabasePath(), t0.Format(time.RFC3339), time.Since(started).Round(time.Millisecond))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "seed:", err)
	os.Exit(1)
}
