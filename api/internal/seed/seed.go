// Package seed builds the demo database (docs/DEMO-AND-SEED.md). It writes through the
// domain packages so every seeded record obeys the same rules as live data, and anchors
// all live timestamps to T0, the moment the seed runs.
package seed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sushilatte/api/internal/auth"
	"sushilatte/api/internal/config"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/media"
	"sushilatte/api/internal/platform"
	"sushilatte/api/internal/store"
	"sushilatte/api/internal/vault"
)

// Rand is the fixed seed for generated history, so reports look the same after every reset.
const Rand = 2026

// DemoAccount is one demo login (DEMO-AND-SEED §4). PINs are public by design in Demo mode.
type DemoAccount struct {
	CrewID   string   `json:"crewId"`
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	Stations []string `json:"stations"`
	PIN      string   `json:"pin"`
	Portrait string   `json:"portrait"`
	Note     string   `json:"note,omitempty"`
}

const (
	portraitManager = "staff/staff-store-manager.jpg"
	portraitMale    = "staff/staff-waiter-male.jpg"
	portraitFemale  = "staff/staff-waitress-female.jpg"
)

// DemoCrew is the seeded Crew. Keep in sync with DEMO-AND-SEED.md §4.
var DemoCrew = []DemoAccount{
	{"M001", "Hendra Wijaya", auth.RoleManagement, []string{}, "111111", portraitManager, "Manual discounts, Refunds above cap, Reset demo"},
	{"A001", "Rina Kusuma", auth.RoleOutletAdmin, []string{}, "222222", portraitFemale, "Escalations, Voids, Remakes, Unpaid Bills"},
	{"W001", "Andi Pratama", auth.RoleCrew, []string{auth.StationWaiter}, "333333", portraitMale, ""},
	{"W002", "Dimas Saputra", auth.RoleCrew, []string{auth.StationWaiter}, "333333", portraitMale, ""},
	{"W003", "Putri Lestari", auth.RoleCrew, []string{auth.StationWaiter}, "333333", portraitFemale, ""},
	{"W004", "Nadia Rahma", auth.RoleCrew, []string{auth.StationWaiter}, "333333", portraitFemale, ""},
	{"W005", "Eko Prasetyo", auth.RoleCrew, []string{auth.StationWaiter}, "333333", portraitMale, "Locked out: shows the PIN lockout"},
	{"C001", "Maya Anggraini", auth.RoleCrew, []string{auth.StationCashier}, "444444", portraitFemale, ""},
	{"C002", "Fajar Nugroho", auth.RoleCrew, []string{auth.StationCashier, auth.StationWaiter}, "444444", portraitMale, ""},
	{"K001", "Budi Santoso", auth.RoleCrew, []string{auth.StationKitchen}, "555555", portraitMale, "Kitchen display"},
	{"K002", "Agus Setiawan", auth.RoleCrew, []string{auth.StationKitchen}, "555555", portraitMale, "Kitchen display"},
	{"K003", "Lia Permata", auth.RoleCrew, []string{auth.StationKitchen}, "555555", portraitFemale, "Bar display"},
}

const lockedOutCrew = "W005"

// Options locate the seed's inputs and outputs.
type Options struct {
	DBPath    string    // database file to create
	ImagesDir string    // served images (data/images); processed variants are cached here
	AssetsDir string    // demo photos (docs/images)
	T0        time.Time // anchor for every live timestamp
	// Vault encrypts seeded Customer data. Its key file is built next to the database and
	// installed with it, so a reset also rotates the data keys (DEMO-AND-SEED §9).
	Vault *vault.Vault
}

// Build creates a fresh demo database, anchored at opts.T0.
func Build(ctx context.Context, opts Options) error {
	if err := removeDatabase(opts.DBPath); err != nil {
		return err
	}
	db, err := dbx.Open(ctx, opts.DBPath, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	crewIDs, err := seedCrew(ctx, db, opts)
	if err != nil {
		return err
	}
	ids, err := seedCatalog(ctx, db, opts, crewIDs["M001"], crewIDs["A001"])
	if err != nil {
		return err
	}
	if err := seedOutlet(ctx, db, opts, crewIDs["M001"], ids); err != nil {
		return err
	}
	if err := seedDisplay(ctx, db, opts, crewIDs["M001"], ids); err != nil {
		return err
	}
	if err := seedHRIS(ctx, db, opts, crewIDs); err != nil {
		return err
	}
	if err := seedCustomers(ctx, db, opts, ids, crewIDs); err != nil {
		return err
	}
	if err := lockOut(ctx, db, opts, crewIDs[lockedOutCrew]); err != nil {
		return err
	}
	return db.Checkpoint(ctx)
}

// lockOut shows the PIN lockout on W005 until T0 +15m (DEMO-AND-SEED §4). It comes last: a
// locked Crew member could not have clocked in during the seeded week.
func lockOut(ctx context.Context, db *dbx.DB, opts Options, id string) error {
	return db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.SetCrewLockout(ctx, store.SetCrewLockoutParams{
			FailedAttempts: 5,
			LockedUntil:    nullMs(opts.T0.Add(15 * time.Minute)),
			UpdatedAt:      platform.Millis(opts.T0),
			ID:             id,
		})
	})
}

func seedCrew(ctx context.Context, db *dbx.DB, opts Options) (map[string]string, error) {
	images := media.NewStore(opts.ImagesDir)
	portraits := map[string]string{}
	for _, a := range DemoCrew {
		if _, ok := portraits[a.Portrait]; ok {
			continue
		}
		key, err := saveImage(images, filepath.Join(opts.AssetsDir, filepath.FromSlash(a.Portrait)))
		if err != nil {
			return nil, fmt.Errorf("portrait %s: %w", a.Portrait, err)
		}
		portraits[a.Portrait] = key
	}
	ids := map[string]string{}
	err := db.Write(ctx, func(tx *dbx.Tx) error {
		for _, a := range DemoCrew {
			id, err := auth.CreateCrew(ctx, tx, opts.T0, auth.NewCrew{
				CrewID: a.CrewID, FullName: a.Name, Role: a.Role, Stations: a.Stations, PIN: a.PIN, ImageKey: portraits[a.Portrait],
			})
			if err != nil {
				return fmt.Errorf("crew %s: %w", a.CrewID, err)
			}
			ids[a.CrewID] = id
		}
		return nil
	})
	return ids, err
}

// Install replaces the live database and key file with freshly built ones. The API must not
// hold the live file open (stop the server, or use the in-app reset once it exists).
func Install(next, live, nextKeys, liveKeys string) error {
	if err := removeDatabase(live); err != nil {
		return fmt.Errorf("remove live database (is the API still running?): %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(next + suffix)
	}
	if err := os.Rename(next, live); err != nil {
		return err
	}
	if _, err := os.Stat(nextKeys); errors.Is(err, os.ErrNotExist) {
		return os.Remove(liveKeys) // nothing was encrypted; old keys are useless anyway
	}
	return os.Rename(nextKeys, liveKeys)
}

func removeDatabase(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Next is a freshly built demo database and key file waiting next to the live ones.
type Next struct{ DB, Keys string }

// BuildNext builds the demo database anchored at t0 into data/sushi-latte.next.db with its own
// key file (DEMO-AND-SEED §9 step 1). The live files are untouched until Install.
func BuildNext(ctx context.Context, cfg config.Config, t0 time.Time) (Next, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return Next{}, err
	}
	n := Next{DB: filepath.Join(cfg.DataDir, "sushi-latte.next.db"), Keys: filepath.Join(cfg.DataDir, "keys.next.json")}
	if err := os.Remove(n.Keys); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Next{}, err
	}
	masterKey, err := vault.LoadMasterKey(cfg.MasterKey, cfg.MasterKeyPath())
	if err != nil {
		return Next{}, err
	}
	keys, err := vault.Open(n.Keys, masterKey)
	if err != nil {
		return Next{}, err
	}
	opts := Options{DBPath: n.DB, ImagesDir: cfg.ImagesDir(), AssetsDir: cfg.AssetsDir, T0: t0, Vault: keys}
	return n, Build(ctx, opts)
}

// Install moves the built files over the live ones.
func (n Next) Install(cfg config.Config) error {
	return Install(n.DB, cfg.DatabasePath(), n.Keys, cfg.KeysPath())
}
