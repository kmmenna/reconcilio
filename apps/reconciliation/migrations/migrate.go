package migrations

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed *.sql
var files embed.FS

// Apply runs application and River migrations against the same database.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	if err := applyTern(ctx, pool); err != nil {
		return err
	}
	queue, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create River migrator: %w", err)
	}
	if _, err := queue.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("migrate River schema: %w", err)
	}
	return nil
}

func applyTern(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("connect for migration: %w", err)
	}
	defer conn.Release()

	migrator, err := migrate.NewMigrator(ctx, conn.Conn(), "public.reconcilio_schema_version")
	if err != nil {
		return fmt.Errorf("create application migrator: %w", err)
	}
	if err := migrator.LoadMigrations(files); err != nil {
		return fmt.Errorf("load application migrations: %w", err)
	}
	return migrator.Migrate(ctx)
}
