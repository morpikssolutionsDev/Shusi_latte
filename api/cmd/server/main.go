// Package main is the Phase 1 scaffold for the Sushi Latte API.
//
// This is not yet the full Admin Panel. It provides a single health
// endpoint so the Phase 1 build can be verified before catalog,
// configuration and reporting endpoints are added (BR-ADM-01 to BR-ADM-13,
// TR-ARC-01: one Go binary hosts the API).
package main

import (
	"encoding/json"
	"log"
	"net/http"
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
		"phase":   "phase-1-admin-panel-scaffold",
	})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/api/v1/health", healthHandler)
	log.Println("sushi-latte-api listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
