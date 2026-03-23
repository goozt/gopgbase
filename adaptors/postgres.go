package adaptors

import (
	"fmt"
	"net/url"

	gopgbase "github.com/goozt/gopgbase"
)

// PostgresConfig configures a connection to PostgreSQL (self-hosted),
// AWS RDS for PostgreSQL, Railway, Render, or any standard Postgres instance.
type PostgresConfig struct {
	ConnectionURL   string `json:"connection_url,omitempty"`
	ApplicationName string `json:"application_name,omitempty"`
	SSLRootCert     string `json:"ssl_root_cert,omitempty"`
	BaseConfig
}

// NewPostgresAdaptor creates a DataStore connected to a PostgreSQL-compatible
// database using the pgx driver.
//
// It supports self-hosted PostgreSQL, AWS RDS, Railway, Render, and any
// standard Postgres-protocol database.
//
// When ConnectionURL is provided (common for Railway/Render), it is used
// directly with the Insecure flag applied as an override. Otherwise, a DSN
// is built from individual config fields.
//
// Security:
//   - Insecure=false (default): sslmode=verify-full — server certificate is
//     verified against system CAs (or SSLRootCert if provided).
//   - Insecure=true: sslmode=disable — intended ONLY for local development.
func NewPostgresAdaptor(cfg PostgresConfig) (gopgbase.DataStore, error) {
	dsn, err := buildPostgresDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres adaptor: %w", err)
	}

	db, err := openPgx(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres adaptor: %w", err)
	}

	return &pgxDataStore{db: db}, nil
}

// buildPostgresDSN constructs a pgx-compatible connection string.
func buildPostgresDSN(cfg PostgresConfig) (string, error) {
	if cfg.ConnectionURL != "" {
		return applySSLMode(cfg.ConnectionURL, cfg.Insecure)
	}

	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	sslMode := "verify-full"
	if cfg.Insecure {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, port, cfg.User, cfg.Password, cfg.DBName, sslMode,
	)

	if cfg.ApplicationName != "" {
		dsn += fmt.Sprintf(" application_name=%s", cfg.ApplicationName)
	}

	if cfg.SSLRootCert != "" && !cfg.Insecure {
		dsn += fmt.Sprintf(" sslrootcert=%s", cfg.SSLRootCert)
	}

	return dsn, nil
}

// applySSLMode modifies a connection URL to set the appropriate sslmode.
func applySSLMode(connURL string, insecure bool) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", fmt.Errorf("parse connection URL: %w", err)
	}

	q := u.Query()
	if insecure {
		q.Set("sslmode", "disable")
	} else if q.Get("sslmode") == "" {
		q.Set("sslmode", "verify-full")
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
