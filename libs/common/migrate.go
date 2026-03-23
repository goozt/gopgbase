package common

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	gopgbase "github.com/goozt/gopgbase"
	"github.com/pressly/goose/v3"
)

// MigrateLibrary provides database migration operations using goose.
//
// It wraps the goose library to provide a clean API that integrates with
// gopgbase's DataStore pattern. Migrations are read from an fs.FS
// (typically embed.FS) so SQL files are compiled into the binary.
//
// Users who prefer CLI-driven migrations (CI/CD, DBA workflows) should
// use the goose binary directly against DATABASE_URL and skip this library.
//
// Example:
//
//	//go:embed migrations/*.sql
//	var MigrationsFS embed.FS
//
//	migrator := common.NewMigrateLibrary(client)
//	if err := migrator.Up(ctx, MigrationsFS); err != nil {
//	    log.Fatal(err)
//	}
type MigrateLibrary struct {
	client *gopgbase.Client
}

// NewMigrateLibrary creates a new MigrateLibrary backed by the given Client.
//
// The Client's DataStore must implement Unwrap() *sql.DB (all built-in
// adaptors do) because goose requires a raw *sql.DB.
func NewMigrateLibrary(client *gopgbase.Client) *MigrateLibrary {
	return &MigrateLibrary{client: client}
}

// Up runs all pending migrations from the provided filesystem.
//
// The fs should contain SQL migration files in goose format (e.g.,
// 00001_create_users.sql). Typically created with embed.FS.
func (m *MigrateLibrary) Up(ctx context.Context, fsys fs.FS) error {
	db, err := m.rawDB()
	if err != nil {
		return err
	}

	goose.SetBaseFS(fsys)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}

	return nil
}

// Down rolls back the last N migrations.
func (m *MigrateLibrary) Down(ctx context.Context, fsys fs.FS, n int) error {
	db, err := m.rawDB()
	if err != nil {
		return err
	}

	goose.SetBaseFS(fsys)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}

	for range n {
		if err := goose.DownContext(ctx, db, "."); err != nil {
			return fmt.Errorf("migrate: down: %w", err)
		}
	}

	return nil
}

// Version returns the currently applied migration version.
func (m *MigrateLibrary) Version(_ context.Context) (int64, error) {
	db, err := m.rawDB()
	if err != nil {
		return 0, err
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return 0, fmt.Errorf("migrate: set dialect: %w", err)
	}

	version, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("migrate: version: %w", err)
	}

	return version, nil
}

// Status prints the migration status (applied/pending) to stdout.
func (m *MigrateLibrary) Status(ctx context.Context, fsys fs.FS) error {
	db, err := m.rawDB()
	if err != nil {
		return err
	}

	goose.SetBaseFS(fsys)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}

	if err := goose.StatusContext(ctx, db, "."); err != nil {
		return fmt.Errorf("migrate: status: %w", err)
	}

	return nil
}

// Create generates a new timestamped migration file in the given directory.
//
// migrationType is either "sql" or "go".
func (m *MigrateLibrary) Create(dir, name, migrationType string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}

	if err := goose.Create(nil, dir, name, migrationType); err != nil {
		return fmt.Errorf("migrate: create: %w", err)
	}

	return nil
}

// rawDB extracts the *sql.DB from the Client's DataStore.
func (m *MigrateLibrary) rawDB() (*sql.DB, error) {
	ds := m.client.DataStore()
	u, ok := ds.(gopgbase.Unwrapper)
	if !ok {
		return nil, fmt.Errorf("migrate: DataStore does not implement Unwrap() *sql.DB")
	}
	return u.Unwrap(), nil
}
