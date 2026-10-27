// Package testutil provides a temporary database and a controllable clock for tests
// (TR-TST-02: real SQLite files, same migrations).
package testutil

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/vault"
)

// NewDB opens a fresh migrated database in a temp dir with its own bus.
func NewDB(t *testing.T) (*dbx.DB, *realtime.Bus) {
	t.Helper()
	bus := realtime.NewBus()
	db, err := dbx.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), bus)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, bus
}

// Clock is a manually advanced clock.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func NewClock(t time.Time) *Clock { return &Clock{now: t} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// NewVault opens a key file in a temp dir with a random master key (TR-RET-04).
func NewVault(t *testing.T) *vault.Vault {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(filepath.Join(t.TempDir(), "keys.json"), key)
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	return v
}
