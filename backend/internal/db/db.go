// Package db is block-explorer's own postgres access layer. It is now
// radically smaller than it used to be: chain data (blocks, transactions,
// logs, transfers, balances, etc.) lives in chain-indexer's postgres and
// is read over gRPC. Block-explorer's DB stores only what is genuinely
// block-explorer-specific — contract verification metadata.
//
// RD-855 Phase 6 deleted all chain-data tables and queries. Any remaining
// methods here are verification-related, or low-level helpers the api
// package keeps using.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"explorer/internal/log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB holds the pool handle. The HiddenTxTypes field was preserved so the
// api package's existing code compiles; it no longer affects any query
// since this DB doesn't serve chain data anymore.
type DB struct {
	pool          *pgxpool.Pool
	HiddenTxTypes []int
}

func New(databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	return &DB{pool: pool, HiddenTxTypes: []int{}}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) Migrate() error {
	ctx := context.Background()

	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	migrator, err := migrate.NewMigrator(ctx, conn.Conn(), "schema_version")
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	migrationsFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations sub-fs: %w", err)
	}

	if err := migrator.LoadMigrations(migrationsFS); err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	migrator.OnStart = func(seq int32, name string, direction string, sql string) {
		log.Info(fmt.Sprintf("running migration %d: %s %s", seq, name, direction))
	}

	log.Info("migrations loaded", "count", len(migrator.Migrations))

	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
