// Package main is the Sushi Latte API server.
//
// Phase 1b: the catalog is stored in SQLite (TR-ARC-02, TRD §4.2).
// The database path comes from SUSHILATTE_DB, defaulting to
// data/sushi-latte.db, which is git-ignored. On startup the server
// applies migration 0001_catalog.sql and seeds Kitchen, Bar and the
// BR-PC-06 Dietary Tags if they are not already present. Auth and role
// checks arrive with the auth phase; catalog handlers are staff
// endpoints until then.
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
		"phase":   "phase-1b-sqlite-catalog",
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

	log.Println("sushi-latte-api listening on :8080, db:", dbPath)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
