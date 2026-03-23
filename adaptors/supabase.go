package adaptors

import (
	"fmt"
	"net/url"

	gopgbase "github.com/goozt/gopgbase"
)

// SupabaseConfig configures a connection to a Supabase PostgreSQL database.
type SupabaseConfig struct {
	ConnectionURL  string `json:"connection_url,omitempty"`
	ProjectRef     string `json:"project_ref,omitempty"`
	APIKey         string `json:"api_key,omitempty"`
	ServiceRoleKey string `json:"service_role_key,omitempty"`
	BaseConfig
	UsePooler bool `json:"use_pooler,omitempty"`
}

// NewSupabaseAdaptor creates a DataStore connected to a Supabase PostgreSQL instance.
//
// Supabase provides managed PostgreSQL with additional features like
// Row Level Security, Auth, and Edge Functions. This adaptor handles
// the database connection; use the supabase companion library for
// RLS, auth, and other Supabase-specific features.
//
// Security:
//   - Insecure=false (default): sslmode=verify-full. Supabase always
//     requires SSL; this ensures certificate verification.
//   - Insecure=true: sslmode=disable — intended ONLY for local Supabase
//     instances (supabase start).
func NewSupabaseAdaptor(cfg SupabaseConfig) (gopgbase.DataStore, error) {
	dsn, err := buildSupabaseDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("supabase adaptor: %w", err)
	}

	db, err := openPgx(dsn)
	if err != nil {
		return nil, fmt.Errorf("supabase adaptor: %w", err)
	}

	return &pgxDataStore{db: db}, nil
}

func buildSupabaseDSN(cfg SupabaseConfig) (string, error) {
	if cfg.ConnectionURL != "" {
		return applySSLMode(cfg.ConnectionURL, cfg.Insecure)
	}

	port := cfg.Port
	if port == 0 {
		if cfg.UsePooler {
			port = 6543
		} else {
			port = 5432
		}
	}

	host := cfg.Host
	if host == "" && cfg.ProjectRef != "" {
		if cfg.UsePooler {
			host = fmt.Sprintf("aws-0-%s.pooler.supabase.com", cfg.ProjectRef)
		} else {
			host = fmt.Sprintf("db.%s.supabase.co", cfg.ProjectRef)
		}
	}

	dbname := cfg.DBName
	if dbname == "" {
		dbname = "postgres"
	}

	user := cfg.User
	if user == "" {
		user = "postgres"
	}

	sslMode := "verify-full"
	if cfg.Insecure {
		sslMode = "disable"
	}

	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(user, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   dbname,
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()

	return u.String(), nil
}
