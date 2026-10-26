// Package db applies the SQLite migrations for the Sushi Latte API.
// Migration files are embedded so the server binary is self-contained:
// one Go binary hosts the API (TR-ARC-01) and one SQLite file is the
// single source of truth (TR-ARC-02).
package db

import (
	"database/sql"
	"embed"
	"errors"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Apply runs the migrations in order (Up part only). It is idempotent:
// the migrations use CREATE TABLE IF NOT EXISTS, INSERT OR IGNORE and
// IF NOT EXISTS triggers.
func Apply(db *sql.DB) error {
	for _, name := range []string{"migrations/0001_catalog.sql", "migrations/0002_pricing.sql", "migrations/0003_tables.sql", "migrations/0004_crew.sql", "migrations/0005_display.sql"} {
		data, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		up := strings.Split(string(data), "-- +goose Down")[0]
		up = strings.Replace(up, "-- +goose Up", "", 1)
		for _, stmt := range splitStatements(up) {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

// splitStatements splits a migration script into statements, keeping
// CREATE TRIGGER ... BEGIN ... END; blocks together even though their
// bodies contain semicolons.
func splitStatements(script string) []string {
	var out []string
	var cur strings.Builder
	inTrigger := false
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "CREATE TRIGGER") {
			inTrigger = true
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if inTrigger {
			if trimmed == "END;" {
				out = append(out, cur.String())
				cur.Reset()
				inTrigger = false
			}
			continue
		}
		if strings.HasSuffix(trimmed, ";") {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	return out
}

var errNoDB = errors.New("database is nil")

// Ensure is a small guard used by callers that must not proceed without a DB.
func Ensure(db *sql.DB) error {
	if db == nil {
		return errNoDB
	}
	return nil
}
