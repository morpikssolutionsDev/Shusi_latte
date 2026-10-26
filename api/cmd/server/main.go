// Package main is the Sushi Latte API server.
//
// Phase 1a: health endpoint plus the catalog configuration endpoints
// (BR-ADM-01, BR-PC-01 to BR-PC-07, TR-PC-05). The catalog Service here
// is in-memory and temporary: migration 0001_catalog.sql defines the
// SQLite schema that will replace it in Phase 1b. Auth and role checks
// arrive with the auth phase.
package main

import (
	"encoding/json"
	"log"
	"net/http"

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
		"phase":   "phase-1-catalog",
	})
}

func main() {
	svc := catalog.NewService()
	svc.SeedBaseline()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/api/v1/health", healthHandler)
	(&catalog.Handler{Service: svc}).Register(mux)

	log.Println("sushi-latte-api listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
