package db

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct {
	pool *pgxpool.Pool
}

func New(databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) Migrate() error {
	ctx := context.Background()

	// Run migrations in order
	migrationFiles := []string{
		"migrations/001_schema.sql",
		"migrations/002_missing_ranges.sql",
	}

	for _, file := range migrationFiles {
		schema, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}

		_, err = d.pool.Exec(ctx, string(schema))
		if err != nil {
			return err
		}
	}

	return nil
}
