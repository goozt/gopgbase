package adaptors

import (
	"fmt"
	"net/url"

	gopgbase "github.com/goozt/gopgbase"
)

// NeonConfig configures a connection to a Neon serverless PostgreSQL database.
type NeonConfig struct {
	ConnectionURL string `json:"connection_url,omitempty"`
	ProjectID     string `json:"project_id,omitempty"`
	BranchID      string `json:"branch_id,omitempty"`
	EndpointID    string `json:"endpoint_id,omitempty"`
	BaseConfig
	UsePooler bool `json:"use_pooler,omitempty"`
}

// NewNeonAdaptor creates a DataStore connected to a Neon serverless PostgreSQL instance.
//
// Neon uses SNI-based routing to direct connections to the correct compute
// endpoint. This adaptor ensures the connection string includes the
// required endpoint information for proper routing.
//
// Security:
//   - Insecure=false (default): sslmode=require. Neon always requires SSL
//     and uses SNI for routing; verify-full is not typically needed since
//     Neon manages certificates, but require ensures encrypted connections.
//   - Insecure=true: sslmode=disable — intended ONLY for development with
//     a local Neon-compatible setup (not supported by Neon Cloud).
func NewNeonAdaptor(cfg NeonConfig) (gopgbase.DataStore, error) {
	dsn, err := buildNeonDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("neon adaptor: %w", err)
	}

	db, err := openPgx(dsn)
	if err != nil {
		return nil, fmt.Errorf("neon adaptor: %w", err)
	}

	return &pgxDataStore{db: db}, nil
}

func buildNeonDSN(cfg NeonConfig) (string, error) {
	if cfg.ConnectionURL != "" {
		return applyNeonParams(cfg.ConnectionURL, cfg)
	}

	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	host := cfg.Host
	if host == "" && cfg.EndpointID != "" {
		if cfg.UsePooler {
			host = fmt.Sprintf("%s-pooler.neon.tech", cfg.EndpointID)
		} else {
			host = fmt.Sprintf("%s.neon.tech", cfg.EndpointID)
		}
	}

	// Neon requires SSL; sslmode=require is the recommended mode.
	// verify-full is not needed since Neon manages its own certificates.
	sslMode := "require"
	if cfg.Insecure {
		sslMode = "disable"
	}

	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   cfg.DBName,
	}

	q := u.Query()
	q.Set("sslmode", sslMode)

	// Neon uses the endpoint option for SNI-based routing when needed.
	if cfg.EndpointID != "" {
		q.Set("options", fmt.Sprintf("project=%s", cfg.EndpointID))
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func applyNeonParams(connURL string, cfg NeonConfig) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", fmt.Errorf("parse connection URL: %w", err)
	}

	q := u.Query()
	if cfg.Insecure {
		q.Set("sslmode", "disable")
	} else if q.Get("sslmode") == "" {
		q.Set("sslmode", "require")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}
