package adaptors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPostgresDSN_FieldBased(t *testing.T) {
	tests := []struct {
		name string
		want string
		cfg  PostgresConfig
	}{
		{
			name: "basic secure",
			cfg: PostgresConfig{
				BaseConfig: BaseConfig{
					Host:     "db.example.com",
					Port:     5432,
					User:     "postgres",
					Password: "secret",
					DBName:   "mydb",
				},
			},
			want: "host=db.example.com port=5432 user=postgres password=secret dbname=mydb sslmode=verify-full",
		},
		{
			name: "insecure",
			cfg: PostgresConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					Port:     5432,
					User:     "postgres",
					Password: "secret",
					DBName:   "mydb",
					Insecure: true,
				},
			},
			want: "host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable",
		},
		{
			name: "default port",
			cfg: PostgresConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					User:     "postgres",
					Password: "pass",
					DBName:   "test",
				},
			},
			want: "host=localhost port=5432 user=postgres password=pass dbname=test sslmode=verify-full",
		},
		{
			name: "with application name",
			cfg: PostgresConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					User:     "postgres",
					Password: "pass",
					DBName:   "test",
					Insecure: true,
				},
				ApplicationName: "myapp",
			},
			want: "host=localhost port=5432 user=postgres password=pass dbname=test sslmode=disable application_name=myapp",
		},
		{
			name: "with ssl root cert",
			cfg: PostgresConfig{
				BaseConfig: BaseConfig{
					Host:     "rds.amazonaws.com",
					User:     "postgres",
					Password: "pass",
					DBName:   "test",
				},
				SSLRootCert: "/path/to/rds-ca.pem",
			},
			want: "host=rds.amazonaws.com port=5432 user=postgres password=pass dbname=test sslmode=verify-full sslrootcert=/path/to/rds-ca.pem",
		},
		{
			name: "ssl root cert ignored when insecure",
			cfg: PostgresConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					User:     "postgres",
					Password: "pass",
					DBName:   "test",
					Insecure: true,
				},
				SSLRootCert: "/path/to/cert.pem",
			},
			want: "host=localhost port=5432 user=postgres password=pass dbname=test sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPostgresDSN(tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildPostgresDSN_ConnectionURL(t *testing.T) {
	tests := []struct {
		name    string
		wantSSL string
		cfg     PostgresConfig
		wantErr bool
	}{
		{
			name: "URL with secure default",
			cfg: PostgresConfig{
				ConnectionURL: "postgresql://user:pass@host:5432/db",
			},
			wantSSL: "verify-full",
		},
		{
			name: "URL with insecure",
			cfg: PostgresConfig{
				ConnectionURL: "postgresql://user:pass@host:5432/db",
				BaseConfig:    BaseConfig{Insecure: true},
			},
			wantSSL: "disable",
		},
		{
			name: "URL with existing sslmode preserved",
			cfg: PostgresConfig{
				ConnectionURL: "postgresql://user:pass@host:5432/db?sslmode=require",
			},
			wantSSL: "require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPostgresDSN(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, got, "sslmode="+tt.wantSSL)
		})
	}
}

func TestApplySSLMode(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantSSL  string
		insecure bool
	}{
		{
			name:     "add verify-full",
			url:      "postgresql://user:pass@host/db",
			insecure: false,
			wantSSL:  "verify-full",
		},
		{
			name:     "set disable",
			url:      "postgresql://user:pass@host/db",
			insecure: true,
			wantSSL:  "disable",
		},
		{
			name:     "override existing with insecure",
			url:      "postgresql://user:pass@host/db?sslmode=verify-full",
			insecure: true,
			wantSSL:  "disable",
		},
		{
			name:     "preserve existing when secure",
			url:      "postgresql://user:pass@host/db?sslmode=require",
			insecure: false,
			wantSSL:  "require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applySSLMode(tt.url, tt.insecure)
			require.NoError(t, err)
			assert.Contains(t, got, "sslmode="+tt.wantSSL)
		})
	}
}
