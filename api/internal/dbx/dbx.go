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
	"net/url"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

type DB struct {
	write *sql.DB
	read  *sql.DB
	bus   *realtime.Bus
}

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
	write, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)
	if err := migrate(ctx, write); err != nil {
		write.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	read, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(8)
	return &DB{write: write, read: read, bus: bus}, nil
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
	return errors.Join(d.read.Close(), d.write.Close())
}

// Read returns queries bound to the read-only pool.
func (d *DB) Read() *store.Queries { return store.New(d.read) }

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
	_, err := d.write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}
