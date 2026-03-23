package adaptors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRedshiftDSN_FieldBased(t *testing.T) {
	tests := []struct {
		name   string
		wantIn []string
		cfg    RedshiftConfig
	}{
		{
			name: "basic secure",
			cfg: RedshiftConfig{
				BaseConfig: BaseConfig{
					Host:     "my-cluster.us-east-1.redshift.amazonaws.com",
					User:     "admin",
					Password: "pass",
					DBName:   "dev",
				},
			},
			wantIn: []string{
				"5439",
				"sslmode=verify-full",
			},
		},
		{
			name: "with statement timeout",
			cfg: RedshiftConfig{
				BaseConfig: BaseConfig{
					Host:     "host",
					User:     "admin",
					Password: "pass",
					DBName:   "dev",
				},
				StatementTimeout: 300000,
			},
			wantIn: []string{
				"statement_timeout=300000",
			},
		},
		{
			name: "insecure",
			cfg: RedshiftConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					User:     "admin",
					Password: "pass",
					DBName:   "dev",
					Insecure: true,
				},
			},
			wantIn: []string{
				"sslmode=disable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildRedshiftDSN(tt.cfg)
			require.NoError(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, got, want)
			}
		})
	}
}

func TestBuildRedshiftDSN_ConnectionURL(t *testing.T) {
	cfg := RedshiftConfig{
		ConnectionURL:    "postgresql://admin:pass@cluster:5439/dev",
		StatementTimeout: 60000,
	}

	got, err := buildRedshiftDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=verify-full")
	assert.Contains(t, got, "statement_timeout=60000")
}
