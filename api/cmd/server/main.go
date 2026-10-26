// Package main is the Sushi Latte API server.
//
// Phase 2b: Phase 2 Client Menu is complete on the API side:
// table scan, join and the single open Bill, plus customer
// Orders with Variants, Modifiers, dietary warnings, edit and
// cancel before Claim, and the Customer bill view (BR-CM-01 to
// BR-CM-15, TR-DM-01, TR-DM-02, TR-DM-05, TR-AUTH-07). The database
// path comes from SUSHILATTE_DB, defaulting to data/sushi-latte.db,
// which is git-ignored. On startup the server applies migrations
// 0001 to 0008 and seeds Kitchen, Bar, the BR-PC-06 Dietary Tags,
// tax (PB1, 10%), the example refund cap and the ops timer
// defaults if they are not already present. QR payloads use
// SUSHILATTE_PUBLIC_URL when set. Customer personal data is
// encrypted with per-day AES-256-GCM keys (TR-RET-04); the key
// file defaults to data/keys.json, wrapped by SUSHILATTE_MASTER_KEY
// or a locally generated master key in data/master.key, which is
// demo-only and must be replaced from the environment in a real
// deployment. Auth middleware wraps the whole mux and enforces
// the BRD §5.1 permission matrix server-side; all /public/
// Client Menu endpoints stay public, since scanning a QR is the
// Customer's authentication and guest tokens authenticate
// subsequent Customer requests. The Client Menu web app itself
// has not been built; this phase is the API it will use.
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/morpikssolutionsDev/Shusi_latte/api/db"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/auth"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/catalog"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crypto"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/display"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/ops"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/ordering"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/reports"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/tables"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "sushi-latte-api",
		"phase":   "phase-2-complete",
	})
}

func main() {
	dbPath := os.Getenv("SUSHILATTE_DB")
	if dbPath == "" {
		dbPath = filepath.Join("data", "sushi-latte.db")
	}
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		log.Fatal(err)
	}
	if err := db.Apply(sqlDB); err != nil {
		log.Fatal(err)
	}

	svc := catalog.NewSQLiteService(sqlDB)
	svc.SeedBaseline()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/api/v1/health", healthHandler)
	(&catalog.Handler{Service: svc}).Register(mux)
	(&pricing.Handler{Store: pricing.NewStore(sqlDB)}).Register(mux)
	(&tables.Handler{Store: tables.NewStore(sqlDB), PublicBase: os.Getenv("SUSHILATTE_PUBLIC_URL")}).Register(mux)
	(&crew.Handler{Store: crew.NewStore(sqlDB)}).Register(mux)
	(&display.Handler{Store: display.NewStore(sqlDB)}).Register(mux)
	(&reports.Handler{Store: reports.NewStore(sqlDB)}).Register(mux)
	(&ops.Handler{Store: ops.NewStore(sqlDB)}).Register(mux)
	keyFile := os.Getenv("SUSHILATTE_KEYS")
	if keyFile == "" {
		keyFile = filepath.Join(filepath.Dir(dbPath), "keys.json")
	}
	keyManager, err := crypto.NewKeyManager(keyFile, os.Getenv("SUSHILATTE_MASTER_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	(&ordering.Handler{Store: ordering.NewStore(sqlDB, keyManager), Catalog: svc, Pricing: pricing.NewStore(sqlDB)}).Register(mux)
	crewStore := crew.NewStore(sqlDB)
	authStore := auth.NewStore(sqlDB, crewStore)
	(&auth.Handler{Store: authStore, Limiter: auth.NewIPLimiter()}).Register(mux)

	log.Println("sushi-latte-api listening on :8080, db:", dbPath)
	if err := http.ListenAndServe(":8080", authStore.Middleware(mux)); err != nil {
		log.Fatal(err)
	}
}
