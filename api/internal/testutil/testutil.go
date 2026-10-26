// Package testutil provides a temporary database and a controllable clock for tests
// (TR-TST-02: real SQLite files, same migrations).
package testutil

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/realtime"
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
