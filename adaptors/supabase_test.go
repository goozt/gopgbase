package adaptors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSupabaseDSN_ConnectionURL(t *testing.T) {
	cfg := SupabaseConfig{
		ConnectionURL: "postgresql://postgres.abc:pass@host:6543/postgres",
	}

	got, err := buildSupabaseDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=verify-full")
}

func TestBuildSupabaseDSN_ConnectionURL_Insecure(t *testing.T) {
	cfg := SupabaseConfig{
		ConnectionURL: "postgresql://postgres:pass@localhost:54322/postgres",
		BaseConfig:    BaseConfig{Insecure: true},
	}

	got, err := buildSupabaseDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=disable")
}

func TestBuildSupabaseDSN_FieldBased(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SupabaseConfig
		wantIn  []string
	}{
		{
			name: "with project ref",
			cfg: SupabaseConfig{
				BaseConfig: BaseConfig{
					Password: "pass",
				},
				ProjectRef: "abcdef",
			},
			wantIn: []string{
				"db.abcdef.supabase.co",
				"sslmode=verify-full",
				"postgres", // default dbname
			},
		},
		{
			name: "with pooler",
			cfg: SupabaseConfig{
				BaseConfig: BaseConfig{
					Password: "pass",
				},
				ProjectRef: "abcdef",
				UsePooler:  true,
			},
			wantIn: []string{
				"pooler.supabase.com",
				"6543",
			},
		},
		{
			name: "insecure local",
			cfg: SupabaseConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					Port:     54322,
					User:     "postgres",
					Password: "postgres",
					DBName:   "postgres",
					Insecure: true,
				},
			},
			wantIn: []string{
				"localhost",
				"54322",
				"sslmode=disable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSupabaseDSN(tt.cfg)
			require.NoError(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, got, want)
			}
		})
	}
}
