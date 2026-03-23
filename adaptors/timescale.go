package adaptors

import (
	"fmt"
	"net/url"

	gopgbase "github.com/goozt/gopgbase"
)

// TimescaleConfig configures a connection to a TimescaleDB instance.
type TimescaleConfig struct {
	BaseConfig

	// ConnectionURL is an optional full connection string.
	// Timescale Cloud provides this in the dashboard.
	ConnectionURL string `json:"connection_url,omitempty"`

	// ServiceID is the Timescale Cloud service identifier.
	ServiceID string `json:"service_id,omitempty"`

	// SSLRootCert is the path to the Timescale Cloud CA certificate.
	SSLRootCert string `json:"ssl_root_cert,omitempty"`
}

// NewTimescaleAdaptor creates a DataStore connected to a TimescaleDB instance.
//
// TimescaleDB extends PostgreSQL with time-series capabilities (hypertables,
// continuous aggregates, compression). This adaptor uses a standard Postgres
// DSN since TimescaleDB is a PostgreSQL extension, not a separate protocol.
//
// Use the timescale companion library for hypertable creation, compression
// policies, and TimescaleDB-specific operations.
//
// Security:
//   - Insecure=false (default): sslmode=verify-full. Timescale Cloud
//     requires SSL; certificates are verified against system CAs.
//   - Insecure=true: sslmode=disable — intended ONLY for local TimescaleDB
//     instances (Docker containers, etc.).
func NewTimescaleAdaptor(cfg TimescaleConfig) (gopgbase.DataStore, error) {
	dsn, err := buildTimescaleDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("timescale adaptor: %w", err)
	}

	db, err := openPgx(dsn)
	if err != nil {
		return nil, fmt.Errorf("timescale adaptor: %w", err)
	}

	return &pgxDataStore{db: db}, nil
}

func buildTimescaleDSN(cfg TimescaleConfig) (string, error) {
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

	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, port),
		Path:   cfg.DBName,
	}

	q := u.Query()
	q.Set("sslmode", sslMode)

	if cfg.SSLRootCert != "" && !cfg.Insecure {
		q.Set("sslrootcert", cfg.SSLRootCert)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}
