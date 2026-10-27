// Command seed rebuilds the demo database anchored at "now" (DEMO-AND-SEED §9).
// Stop the API first: the live file is replaced.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	_ "time/tzdata"

	"sushilatte/api/internal/config"
	"sushilatte/api/internal/seed"
	"sushilatte/api/internal/vault"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		fail(err)
	}
	t0 := time.Now().Truncate(time.Minute)
	next := filepath.Join(cfg.DataDir, "sushi-latte.next.db")
	nextKeys := filepath.Join(cfg.DataDir, "keys.next.json")
	if err := os.Remove(nextKeys); err != nil && !errors.Is(err, os.ErrNotExist) {
		fail(err)
	}
	masterKey, err := vault.LoadMasterKey(cfg.MasterKey, cfg.MasterKeyPath())
	if err != nil {
		fail(err)
	}
	keys, err := vault.Open(nextKeys, masterKey)
	if err != nil {
		fail(err)
	}
	started := time.Now()
	opts := seed.Options{DBPath: next, ImagesDir: cfg.ImagesDir(), AssetsDir: cfg.AssetsDir, T0: t0, Vault: keys}
	if err := seed.Build(context.Background(), opts); err != nil {
		fail(err)
	}
	if err := seed.Install(next, cfg.DatabasePath(), nextKeys, cfg.KeysPath()); err != nil {
		fail(err)
	}
	fmt.Printf("seeded %s (T0 %s) in %s\n", cfg.DatabasePath(), t0.Format(time.RFC3339), time.Since(started).Round(time.Millisecond))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "seed:", err)
	os.Exit(1)
}
