// Package main is the Sushi Latte API server.
//
// Phase 1e: the catalog, pricing settings, Tables and crew records
// are stored in SQLite (TR-ARC-02, TRD §4.1 to §4.4). The database
// path comes from SUSHILATTE_DB, defaulting to data/sushi-latte.db,
// which is git-ignored. On startup the server applies migrations
// 0001 to 0004 and seeds Kitchen, Bar, the BR-PC-06 Dietary Tags,
// tax (PB1, 10%) and the example refund cap if they are not already
// present. QR payloads use SUSHILATTE_PUBLIC_URL when set.
// Auth and role checks arrive with the auth phase; catalog, pricing,
// table and crew handlers are staff endpoints until then.
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
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/catalog"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/crew"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/pricing"
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
		"phase":   "phase-1e-crew",
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

	log.Println("sushi-latte-api listening on :8080, db:", dbPath)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
