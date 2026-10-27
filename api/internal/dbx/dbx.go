// Package dbx opens the SQLite database and runs units of work (ADR-0005).
//
// SQLite allows one writer at a time, so writes go through a single-connection pool whose
// transactions start with BEGIN IMMEDIATE (the write lock is taken up front, never upgraded
// mid-transaction). Reads use a separate read-only pool and run concurrently under WAL.
package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

type DB struct {
	path string
	bus  *realtime.Bus
	// mu guards the write pool: every unit of work holds it shared; Replace takes it alone to
	// swap the file underneath (demo reset, DEMO-AND-SEED §9). The read pool is swapped
	// atomically instead, so a read inside a unit of work never waits on Replace.
	mu        sync.RWMutex
	write     *sql.DB
	read      atomic.Pointer[sql.DB]
	replacing atomic.Bool
}

// ErrReplacing is returned to writes while the database file is being swapped.
var ErrReplacing = httpx.NewProblem(http.StatusServiceUnavailable, "demo_resetting", "Demo sedang direset, coba lagi sebentar")

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		q.Set("_txlock", "immediate")
	}
	return "file:" + path + "?" + q.Encode()
}

// Open opens (creating if needed) the database at path and applies all migrations.
func Open(ctx context.Context, path string, bus *realtime.Bus) (*DB, error) {
	d := &DB{path: path, bus: bus}
	if err := d.open(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) open(ctx context.Context) error {
	write, err := sql.Open("sqlite", dsn(d.path, false))
	if err != nil {
		return err
	}
	write.SetMaxOpenConns(1)
	if err := migrate(ctx, write); err != nil {
		write.Close()
		return fmt.Errorf("migrate: %w", err)
	}
	read, err := sql.Open("sqlite", dsn(d.path, true))
	if err != nil {
		write.Close()
		return err
	}
	read.SetMaxOpenConns(8)
	d.write = write
	d.read.Store(read)
	return nil
}

// Replace swaps the database file: new writes are refused with ErrReplacing, in-flight units
// of work finish, the pools close, install moves the new file into place and the pools reopen
// on it. Used by the demo reset; reads in flight at that moment fail once and are retried by
// their clients.
func (d *DB) Replace(ctx context.Context, install func() error) error {
	d.replacing.Store(true)
	defer d.replacing.Store(false)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := errors.Join(d.read.Load().Close(), d.write.Close()); err != nil {
		return err
	}
	installErr := install()
	// Reopen whatever is in place now, so the API keeps serving even if the install failed.
	return errors.Join(installErr, d.open(ctx))
}

func migrate(ctx context.Context, conn *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, mustSub())
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return errors.Join(d.read.Load().Close(), d.write.Close())
}

// Read returns queries bound to the read-only pool.
func (d *DB) Read() *store.Queries { return store.New(d.read.Load()) }

// Tx is one write unit of work. Events queued with Publish are delivered on the bus only
// after a successful commit, so no client ever sees a rolled-back change (TR-ARC-03).
type Tx struct {
	*store.Queries
	sqlTx  *sql.Tx
	events []realtime.Event
}

func (t *Tx) Publish(events ...realtime.Event) { t.events = append(t.events, events...) }

// Exec runs a statement that has no sqlc query (rare; prefer generated queries).
func (t *Tx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.sqlTx.ExecContext(ctx, query, args...)
	return err
}

// Write runs fn in a write transaction. If fn returns an error the transaction rolls back
// and its queued events are discarded.
func (d *DB) Write(ctx context.Context, fn func(tx *Tx) error) error {
	if d.replacing.Load() {
		return ErrReplacing
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	sqlTx, err := d.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	tx := &Tx{Queries: store.New(sqlTx), sqlTx: sqlTx}
	if err := fn(tx); err != nil {
		return errors.Join(err, rollback(sqlTx))
	}
	if err := sqlTx.Commit(); err != nil {
		return err
	}
	if d.bus != nil && len(tx.events) > 0 {
		d.bus.Publish(tx.events...)
	}
	return nil
}

func rollback(tx *sql.Tx) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

// Bus exposes the event bus for publishing events that are not tied to a write.
func (d *DB) Bus() *realtime.Bus { return d.bus }

// Checkpoint flushes the WAL into the main file (used before copying the database).
func (d *DB) Checkpoint(ctx context.Context) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, err := d.write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// QueryRow reads one row inside the unit of work (rare; prefer generated queries).
func (t *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return t.sqlTx.QueryRowContext(ctx, query, args...)
}
