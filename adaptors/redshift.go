package adaptors

import (
	"fmt"
	"net/url"

	gopgbase "github.com/goozt/gopgbase"
)

// RedshiftConfig configures a connection to Amazon Redshift.
type RedshiftConfig struct {
	BaseConfig

	// ConnectionURL is an optional full connection string.
	ConnectionURL string `json:"connection_url,omitempty"`

	// ClusterIdentifier is the Redshift cluster name.
	ClusterIdentifier string `json:"cluster_identifier,omitempty"`

	// Region is the AWS region (e.g., "us-east-1").
	Region string `json:"region,omitempty"`

	// SSLRootCert path to the Amazon Redshift CA bundle.
	SSLRootCert string `json:"ssl_root_cert,omitempty"`

	// StatementTimeout sets a default statement timeout in milliseconds
	// for long-running OLAP queries. 0 means no timeout.
	StatementTimeout int `json:"statement_timeout,omitempty"`
}

// NewRedshiftAdaptor creates a DataStore connected to an Amazon Redshift cluster.
//
// Redshift is optimized for OLAP workloads (large analytical queries).
// This adaptor adjusts connection parameters for columnar query patterns,
// including optional statement timeouts for long-running queries.
//
// Security:
//   - Insecure=false (default): sslmode=verify-full with the Amazon Redshift
//     CA bundle. Redshift requires SSL by default.
//   - Insecure=true: sslmode=disable — intended ONLY for local development
//     or testing against a Redshift-compatible proxy.
func NewRedshiftAdaptor(cfg RedshiftConfig) (gopgbase.DataStore, error) {
	dsn, err := buildRedshiftDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("redshift adaptor: %w", err)
	}

	db, err := openPgx(dsn)
	if err != nil {
		return nil, fmt.Errorf("redshift adaptor: %w", err)
	}

	// Redshift benefits from fewer, longer-lived connections for OLAP.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	return &pgxDataStore{db: db}, nil
}

func buildRedshiftDSN(cfg RedshiftConfig) (string, error) {
	if cfg.ConnectionURL != "" {
		return applyRedshiftParams(cfg.ConnectionURL, cfg)
	}

	port := cfg.Port
	if port == 0 {
		port = 5439 // Redshift default port
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

	if cfg.StatementTimeout > 0 {
		q.Set("statement_timeout", fmt.Sprintf("%d", cfg.StatementTimeout))
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func applyRedshiftParams(connURL string, cfg RedshiftConfig) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", fmt.Errorf("parse connection URL: %w", err)
	}

	q := u.Query()
	if cfg.Insecure {
		q.Set("sslmode", "disable")
	} else if q.Get("sslmode") == "" {
		q.Set("sslmode", "verify-full")
	}

	if cfg.StatementTimeout > 0 {
		q.Set("statement_timeout", fmt.Sprintf("%d", cfg.StatementTimeout))
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}
