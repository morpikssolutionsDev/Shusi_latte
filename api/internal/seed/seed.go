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
	if err := seedCustomers(ctx, db, opts, ids, crewIDs); err != nil {
		return err
	}
	return db.Checkpoint(ctx)
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
			if a.CrewID == lockedOutCrew {
				if err := tx.SetCrewLockout(ctx, store.SetCrewLockoutParams{
					FailedAttempts: 5,
					LockedUntil:    nullMs(opts.T0.Add(15 * time.Minute)),
					UpdatedAt:      platform.Millis(opts.T0),
					ID:             id,
				}); err != nil {
					return err
				}
			}
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
