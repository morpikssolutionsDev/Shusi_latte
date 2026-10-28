// Command seed rebuilds the demo database in PostgreSQL (POSTGRE_URL), anchored at "now"
// (DEMO-AND-SEED §9). By default it builds the clean start: menu, Crew, Tables, discounts,
// Product Display and settings. -story adds the scripted demo (busy Tables, 14 days of history).
// With the API running, use Admin → Pengaturan → Reset demo instead: the API must reload its keys.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/seed"
)

func main() {
	story := flag.Bool("story", false, "add the scripted demo story")
	ifEmpty := flag.Bool("if-empty", false, "only seed when the live schema has no data yet")
	flag.Parse()
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		fail(fmt.Errorf("POSTGRE_URL is not set"))
	}
	ctx := context.Background()
	if *ifEmpty {
		seeded, err := dbx.SchemaHasData(ctx, cfg.DatabaseURL, dbx.LiveSchema)
		if err != nil {
			fail(err)
		}
		if seeded {
			return
		}
	}
	t0 := time.Now().Truncate(time.Minute)
	started := time.Now()
	next, err := seed.BuildNext(ctx, cfg, t0, *story)
	if err != nil {
		fail(err)
	}
	if err := next.Install(cfg); err != nil {
		fail(err)
	}
	kind := "clean start"
	if *story {
		kind = "with the demo story"
	}
	fmt.Printf("seeded schema %q, %s (T0 %s) in %s\n", dbx.LiveSchema, kind, t0.Format(time.RFC3339), time.Since(started).Round(time.Millisecond))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "seed:", err)
	os.Exit(1)
}
