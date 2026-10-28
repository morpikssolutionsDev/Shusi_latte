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
	"sushilatte/api/internal/catalog"
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
	DatabaseURL string    // the PostgreSQL database (POSTGRE_URL)
	Schema      string    // schema to build into; it is dropped and recreated
	ImagesDir   string    // served images (data/images); processed variants are cached here
	AssetsDir   string    // demo photos (docs/images)
	T0          time.Time // anchor for every live timestamp
	// Vault encrypts seeded Customer data. Its key file is built next to the database and
	// installed with it, so a reset also rotates the data keys (DEMO-AND-SEED §9).
	Vault *vault.Vault
	// Story adds the scripted demo: busy Tables, Takeaways, today's and 14 days of Bills, the
	// Cash Drawer, HRIS records and the W005 lockout. Without it the demo starts clean: menu,
	// Crew, Tables, discounts, Product Display and settings only.
	Story bool
}

// Build creates a fresh demo database in opts.Schema, anchored at opts.T0.
func Build(ctx context.Context, opts Options) error {
	if err := dbx.DropSchema(ctx, opts.DatabaseURL, opts.Schema); err != nil {
		return err
	}
	db, err := dbx.OpenBuild(ctx, opts.DatabaseURL, opts.Schema)
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
	if opts.Story {
		if err := seedHRIS(ctx, db, opts, crewIDs); err != nil {
			return err
		}
		if err := seedCustomers(ctx, db, opts, ids, crewIDs); err != nil {
			return err
		}
		if err := lockOut(ctx, db, opts, crewIDs[lockedOutCrew]); err != nil {
			return err
		}
	} else if err := retire(ctx, db, opts, crewIDs["M001"], ids.products["ebi-tempura"]); err != nil {
		return err
	}
	return db.Write(ctx, func(tx *dbx.Tx) error {
		return tx.SetDemoSeed(ctx, store.SetDemoSeedParams{Story: boolInt(opts.Story), T0: platform.Millis(opts.T0)})
	})
}

// retire archives the Ebi Tempura Roll, which the story's history does a week back: it is a
// retired Product in every demo (DEMO-AND-SEED §7).
func retire(ctx context.Context, db *dbx.DB, opts Options, managementID string, p catalog.Product) error {
	p.Status = catalog.StatusArchived
	_, err := catalog.NewService(db, fixedClock{opts.T0}).SaveProduct(ctx, managementID, p)
	return err
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
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

// NextSchema is where a demo reset builds before it is swapped in.
const NextSchema = dbx.LiveSchema + "_next"

// Next is a freshly built demo database and key file waiting beside the live ones.
type Next struct{ Schema, Keys string }

// BuildNext builds the demo anchored at t0 into the app_next schema with its own key file
// (DEMO-AND-SEED §9 step 1); story adds the scripted demo. The live data is untouched until Install.
func BuildNext(ctx context.Context, cfg config.Config, t0 time.Time, story bool) (Next, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return Next{}, err
	}
	n := Next{Schema: NextSchema, Keys: filepath.Join(cfg.DataDir, "keys.next.json")}
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
	opts := Options{DatabaseURL: cfg.DatabaseURL, Schema: n.Schema, ImagesDir: cfg.ImagesDir(), AssetsDir: cfg.AssetsDir, T0: t0, Vault: keys, Story: story}
	return n, Build(ctx, opts)
}

// Install swaps the built schema in for the live one and the new key file for the old. A running
// API must reload its keys afterwards (the in-app reset does), so use Reset demo while it runs.
func (n Next) Install(cfg config.Config) error {
	if err := dbx.SwapSchemas(context.Background(), cfg.DatabaseURL, n.Schema, dbx.LiveSchema); err != nil {
		return err
	}
	if _, err := os.Stat(n.Keys); errors.Is(err, os.ErrNotExist) {
		if err := os.Remove(cfg.KeysPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil // nothing was encrypted; old keys are useless anyway
	}
	return os.Rename(n.Keys, cfg.KeysPath())
}
