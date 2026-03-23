package adaptors

import (
	"fmt"
	"net/url"

	gopgbase "github.com/goozt/gopgbase"
)

// CockroachConfig configures a connection to a CockroachDB cluster.
type CockroachConfig struct {
	BaseConfig

	// ConnectionURL is an optional full connection string.
	ConnectionURL string `json:"connection_url,omitempty"`

	// ClusterID is the CockroachDB Serverless cluster identifier.
	// Used for routing in multi-tenant CockroachDB Serverless deployments.
	ClusterID string `json:"cluster_id,omitempty"`

	// RoutingID is an alias for ClusterID used in newer CockroachDB versions.
	RoutingID string `json:"routing_id,omitempty"`
}

// NewCockroachAdaptor creates a DataStore connected to a CockroachDB cluster.
//
// CockroachDB uses the PostgreSQL wire protocol but has specific requirements
// for cluster routing in serverless deployments. This adaptor handles
// cluster ID injection and CockroachDB's recommended TLS configuration.
//
// Security:
//   - Insecure=false (default): sslmode=verify-full. CockroachDB Cloud
//     always requires TLS; certificates are verified against system CAs.
//   - Insecure=true: sslmode=disable — intended ONLY for local CockroachDB
//     instances (cockroach start-single-node --insecure).
func NewCockroachAdaptor(cfg CockroachConfig) (gopgbase.DataStore, error) {
	dsn, err := buildCockroachDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("cockroach adaptor: %w", err)
	}

	db, err := openPgx(dsn)
	if err != nil {
		return nil, fmt.Errorf("cockroach adaptor: %w", err)
	}

	return &pgxDataStore{db: db}, nil
}

func buildCockroachDSN(cfg CockroachConfig) (string, error) {
	if cfg.ConnectionURL != "" {
		return applyCockroachParams(cfg.ConnectionURL, cfg)
	}

	port := cfg.Port
	if port == 0 {
		port = 26257
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

	// CockroachDB Serverless routing.
	routingID := cfg.RoutingID
	if routingID == "" {
		routingID = cfg.ClusterID
	}
	if routingID != "" {
		q.Set("options", fmt.Sprintf("--cluster=%s", routingID))
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func applyCockroachParams(connURL string, cfg CockroachConfig) (string, error) {
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

	routingID := cfg.RoutingID
	if routingID == "" {
		routingID = cfg.ClusterID
	}
	if routingID != "" && q.Get("options") == "" {
		q.Set("options", fmt.Sprintf("--cluster=%s", routingID))
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}
