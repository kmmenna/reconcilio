package migrations

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed *.sql
var files embed.FS

// Apply runs the application schema migrations against the supplied database.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
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
