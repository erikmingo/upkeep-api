package db

//go:generate go tool sqlc generate -f ../../sqlc.yaml

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

func Migrator(pool *pgxpool.Pool) (*goose.Provider, error) {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(database.DialectPostgres, stdlib.OpenDBFromPool(pool), sub)
}

// Migrate applies every pending migration; run at startup so dev never drifts.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	p, err := Migrator(pool)
	if err != nil {
		return err
	}
	_, err = p.Up(ctx)
	return err
}
