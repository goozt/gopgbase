// Package gopgbase provides a unified PostgreSQL client with constructor injection.
//
// gopgbase abstracts multiple PostgreSQL-compatible databases (PostgreSQL,
// AWS RDS, Supabase, CockroachDB, Neon, Redshift, TimescaleDB, Railway,
// Render, and others) behind the DataStore interface.
//
// All database access flows through the DataStore interface — never
// directly through *sql.DB or *sql.Tx. Users inject a DataStore into
// NewClient and interact exclusively via Client methods.
//
// Custom DataStore implementations (for mocking, alternative drivers,
// or custom connection pools) are encouraged and require no internal types.
package gopgbase

import (
	"context"
	"database/sql"
)

// DataStore defines the minimal database operations required by Client.
//
// It mirrors key parts of *sql.DB but is abstract enough for custom
// implementations (mocks, alternative pools, or wrapped drivers).
//
// All concrete adaptors in the adaptors package implement this interface.
// Users may also provide their own implementation for testing or custom setups.
//
// Concrete adaptor types additionally expose Unwrap() *sql.DB for interop
// with tools (like goose) that require a raw *sql.DB. Unwrap is intentionally
// NOT part of this interface to keep it driver-agnostic.
type DataStore interface {
	// QueryRowContext executes a query expected to return at most one row.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row

	// QueryContext executes a query that returns rows, typically a SELECT.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// ExecContext executes a query without returning rows (INSERT, UPDATE, DELETE, etc.).
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)

	// BeginTx starts a transaction with the given options.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// PingContext verifies the connection to the database is alive.
	PingContext(ctx context.Context) error

	// Close closes the underlying connection pool and releases resources.
	Close() error
}

// Unwrapper is an optional interface that concrete DataStore implementations
// may satisfy to expose the underlying *sql.DB. This is useful for interop
// with libraries (like goose) that require a raw *sql.DB.
//
// Example:
//
//	if u, ok := ds.(Unwrapper); ok {
//	    rawDB := u.Unwrap()
//	}
type Unwrapper interface {
	Unwrap() *sql.DB
}
