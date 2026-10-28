// Package dbx opens the PostgreSQL database and runs units of work (ADR-0007).
//
// Every database lives in one schema of the POSTGRE_URL database: the live demo in "app", a demo
// reset's build in "app_next", each test in its own throwaway schema. Writes go through a
// single-connection pool, so units of work run one at a time exactly as the business rules were
// written for (first-writer-wins Claims, gapless Bill numbers, one open Bill per Table). Reads use
// a separate pool whose transactions are read-only.
package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"sushilatte/api/internal/httpx"
	"sushilatte/api/internal/realtime"
	"sushilatte/api/internal/store"
)

// LiveSchema holds the running demo's data.
const LiveSchema = "app"

type DB struct {
	url    string
	schema string
	build  bool // a throwaway build (seed): commits skip the WAL flush
	bus    *realtime.Bus
	// mu guards the write pool: every unit of work holds it shared; Replace takes it alone to
	// swap the schema underneath (demo reset, DEMO-AND-SEED §9). The read pool is swapped
	// atomically instead, so a read inside a unit of work never waits on Replace.
	mu        sync.RWMutex
	write     *sql.DB
	read      atomic.Pointer[sql.DB]
	replacing atomic.Bool
}

// ErrReplacing is returned to writes while the database is being swapped.
var ErrReplacing = httpx.NewProblem(http.StatusServiceUnavailable, "demo_resetting", "Demo sedang direset, coba lagi sebentar")

// Open opens (creating if needed) the schema in the database at url and applies all migrations.
func Open(ctx context.Context, url, schema string, bus *realtime.Bus) (*DB, error) {
	if url == "" {
		return nil, errors.New("POSTGRE_URL is not set: point it at a PostgreSQL database, e.g. postgres://user:pass@localhost:5432/sushi_latte?sslmode=disable")
	}
	d := &DB{url: url, schema: schema, bus: bus}
	if err := d.open(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

// OpenBuild opens a schema that is being built from nothing, such as the demo seed: statements are
// prepared once per connection and commits do not wait for the WAL flush, since nothing swaps the
// schema under a build and a crash only means building it again.
func OpenBuild(ctx context.Context, url, schema string) (*DB, error) {
	if url == "" {
		return nil, errors.New("POSTGRE_URL is not set")
	}
	d := &DB{url: url, schema: schema, build: true}
	if err := d.open(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) open(ctx context.Context) error {
	if err := EnsureSchema(ctx, d.url, d.schema); err != nil {
		return err
	}
	write, err := connect(d.url, d.schema, false, d.build)
	if err != nil {
		return err
	}
	write.SetMaxOpenConns(1)
	if err := migrate(ctx, write); err != nil {
		write.Close()
		return fmt.Errorf("migrate: %w", err)
	}
	read, err := connect(d.url, d.schema, true, d.build)
	if err != nil {
		write.Close()
		return err
	}
	read.SetMaxOpenConns(8)
	d.write = write
	d.read.Store(read)
	return nil
}

// connect opens a pool bound to one schema. Live pools do not cache statements server-side, so
// table names resolve afresh on every statement and a schema swap is picked up at once.
func connect(url, schema string, readOnly, build bool) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("POSTGRE_URL: %w", err)
	}
	cfg.RuntimeParams["search_path"] = schema
	if readOnly {
		cfg.RuntimeParams["default_transaction_read_only"] = "on"
	}
	if build {
		cfg.RuntimeParams["synchronous_commit"] = "off" // pgx's default mode caches prepared statements
	} else {
		cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	return stdlib.OpenDB(*cfg), nil
}

// Replace swaps the database: new writes are refused with ErrReplacing, in-flight units of work
// finish, the pools close, install moves the new data into place (SwapSchemas) and the pools
// reopen on it. Used by the demo reset; reads in flight at that moment fail once and are retried
// by their clients.
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
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, mustSub())
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

// QueryRow reads one row inside the unit of work (rare; prefer generated queries).
func (t *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return t.sqlTx.QueryRowContext(ctx, query, args...)
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

// IsUniqueViolation reports a unique constraint failure (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// ---------------------------------------------------------------- schemas

// admin runs fn on a short-lived connection to the database itself, outside any schema.
func admin(ctx context.Context, url string, fn func(*sql.Conn) error) error {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("POSTGRE_URL: %w", err)
	}
	pool := stdlib.OpenDB(*cfg)
	defer pool.Close()
	conn, err := pool.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return fn(conn)
}

func ident(name string) string { return pgx.Identifier{name}.Sanitize() }

// EnsureSchema creates the schema if it does not exist yet.
func EnsureSchema(ctx context.Context, url, schema string) error {
	return admin(ctx, url, func(c *sql.Conn) error {
		_, err := c.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+ident(schema))
		return err
	})
}

// DropSchema removes a schema and everything in it.
func DropSchema(ctx context.Context, url, schema string) error {
	return admin(ctx, url, func(c *sql.Conn) error {
		_, err := c.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+ident(schema)+" CASCADE")
		return err
	})
}

// SchemaHasData reports whether the schema exists with at least one Crew member, i.e. it was seeded.
func SchemaHasData(ctx context.Context, url, schema string) (bool, error) {
	var ok bool
	err := admin(ctx, url, func(c *sql.Conn) error {
		var exists bool
		if err := c.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", schema+".crew").Scan(&exists); err != nil || !exists {
			return err
		}
		return c.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM "+ident(schema)+".crew)").Scan(&ok)
	})
	return ok, err
}

// SwapSchemas puts next in place of live in one transaction and drops the old live data.
// Nothing may hold the live schema's tables open (Replace closes the pools first).
func SwapSchemas(ctx context.Context, url, next, live string) error {
	old := live + "_old"
	return admin(ctx, url, func(c *sql.Conn) error {
		tx, err := c.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		stmts := []string{"DROP SCHEMA IF EXISTS " + ident(old) + " CASCADE"}
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", live).Scan(&exists); err != nil {
			return errors.Join(err, rollback(tx))
		}
		if exists {
			stmts = append(stmts, "ALTER SCHEMA "+ident(live)+" RENAME TO "+ident(old))
		}
		stmts = append(stmts, "ALTER SCHEMA "+ident(next)+" RENAME TO "+ident(live), "DROP SCHEMA IF EXISTS "+ident(old)+" CASCADE")
		for _, s := range stmts {
			if _, err := tx.ExecContext(ctx, s); err != nil {
				return errors.Join(fmt.Errorf("%s: %w", s, err), rollback(tx))
			}
		}
		return tx.Commit()
	})
}
