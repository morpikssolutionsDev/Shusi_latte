package dbx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sushilatte/api/internal/audit"
	"sushilatte/api/internal/dbx"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/testutil"
)

// TR-ARC-03: events reach subscribers only after COMMIT; a rollback publishes nothing.
func TestEventsPublishedOnlyAfterCommit(t *testing.T) {
	db, bus := testutil.NewDB(t)
	ctx := context.Background()

	var got []realtime.Event
	bus.Subscribe(func(e realtime.Event) { got = append(got, e) })

	err := db.Write(ctx, func(tx *dbx.Tx) error {
		tx.Publish(realtime.Event{Topic: "menu", Type: "test.committed"})
		if len(got) != 0 {
			t.Fatal("event delivered before commit")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Type != "test.committed" {
		t.Fatalf("want 1 committed event, got %+v", got)
	}

	boom := errors.New("boom")
	err = db.Write(ctx, func(tx *dbx.Tx) error {
		tx.Publish(realtime.Event{Topic: "menu", Type: "test.rolledback"})
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("rolled-back event was published: %+v", got)
	}
}

// ADR-0005: concurrent writers serialise on BEGIN IMMEDIATE instead of failing with SQLITE_BUSY.
func TestConcurrentWritersSerialise(t *testing.T) {
	db, _ := testutil.NewDB(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- db.Write(ctx, func(tx *dbx.Tx) error {
				return audit.Write(ctx, tx, time.Now(), audit.Entry{Action: "test.concurrent", Entity: "test"})
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Read().ListAudit(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 {
		t.Fatalf("want 20 audit rows, got %d", len(rows))
	}
}

// TR-DM-04: the audit log rejects updates and deletes.
func TestAuditLogIsAppendOnly(t *testing.T) {
	db, _ := testutil.NewDB(t)
	ctx := context.Background()
	if err := db.Write(ctx, func(tx *dbx.Tx) error {
		return audit.Write(ctx, tx, time.Now(), audit.Entry{Action: "test", Entity: "test"})
	}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{"UPDATE audit_log SET reason = 'x'", "DELETE FROM audit_log"} {
		err := db.Write(ctx, func(tx *dbx.Tx) error { return tx.Exec(ctx, stmt) })
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("%s: want append-only error, got %v", stmt, err)
		}
	}
}

// DEMO-AND-SEED §9: the demo reset swaps the database file under a running API; writes during
// the swap are refused with demo_resetting, and afterwards reads and writes use the new file.
func TestReplaceSwapsTheFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	live, next := filepath.Join(dir, "live.db"), filepath.Join(dir, "next.db")
	mark := func(db *dbx.DB, label string) {
		if err := db.Write(ctx, func(tx *dbx.Tx) error {
			return tx.Exec(ctx, "INSERT INTO cash_drawer (id, label) VALUES (?, ?)", label, label)
		}); err != nil {
			t.Fatal(err)
		}
	}
	has := func(db *dbx.DB, label string) bool {
		if _, err := db.Read().ListAllCrew(ctx); err != nil { // the read pool reopened too
			t.Fatal(err)
		}
		var n int
		err := db.Write(ctx, func(tx *dbx.Tx) error {
			return tx.QueryRow(ctx, "SELECT COUNT(*) FROM cash_drawer WHERE id = ?", label).Scan(&n)
		})
		if err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	db, err := dbx.Open(ctx, live, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mark(db, "old")
	built, err := dbx.Open(ctx, next, nil)
	if err != nil {
		t.Fatal(err)
	}
	mark(built, "new")
	if err := errors.Join(built.Checkpoint(ctx), built.Close()); err != nil {
		t.Fatal(err)
	}

	err = db.Replace(ctx, func() error {
		if err := db.Write(ctx, func(*dbx.Tx) error { return nil }); !errors.Is(err, dbx.ErrReplacing) {
			t.Errorf("write during the swap: %v", err)
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(live + suffix)
		}
		return os.Rename(next, live)
	})
	if err != nil {
		t.Fatal(err)
	}
	if has(db, "old") || !has(db, "new") {
		t.Fatal("the swapped database is not the new file")
	}
	mark(db, "after")
}
