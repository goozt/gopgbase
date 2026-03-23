// Package adaptors provides DataStore implementations for various
// PostgreSQL-compatible databases and services.
//
// Each adaptor constructs a DataStore backed by a *sql.DB using the
// pgx stdlib driver. Adaptors manage their own connection pool and
// apply provider-appropriate security defaults.
//
// Security:
//   - When Insecure is false (default), connections use the most secure
//     TLS mode supported by the provider (typically sslmode=verify-full).
//   - When Insecure is true, TLS verification is disabled. This is
//     intended ONLY for local development environments.
package adaptors

import (
	"context"
	"database/sql"
	"fmt"

	gopgbase "github.com/goozt/gopgbase"

	// Register the pgx stdlib driver for database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// BaseConfig contains connection parameters shared by all adaptors.
type BaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`

	// Insecure disables TLS verification/SSL where applicable.
	// Intended for local development only — never use in production.
	Insecure bool `json:"insecure"`
}

// pgxDataStore wraps a *sql.DB as a DataStore. It is the concrete
// implementation returned by all adaptor constructors.
type pgxDataStore struct {
	db *sql.DB
}

// Ensure pgxDataStore implements DataStore and Unwrapper.
var (
	_ gopgbase.DataStore = (*pgxDataStore)(nil)
	_ gopgbase.Unwrapper = (*pgxDataStore)(nil)
)

func (s *pgxDataStore) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *pgxDataStore) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *pgxDataStore) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *pgxDataStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

func (s *pgxDataStore) PingContext(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *pgxDataStore) Close() error {
	return s.db.Close()
}

// Unwrap returns the underlying *sql.DB for interop with libraries
// (like goose) that require a raw database connection.
func (s *pgxDataStore) Unwrap() *sql.DB {
	return s.db
}

// openPgx opens a database/sql connection using the pgx stdlib driver.
func openPgx(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open pgx: %w", err)
	}
	return db, nil
}
