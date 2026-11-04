package db

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(ctx context.Context, database *sql.DB) error {
	goose.SetBaseFS(migrationFiles)
	return goose.UpContext(ctx, database, "migrations")
}
