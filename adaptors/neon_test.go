package adaptors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildNeonDSN_FieldBased(t *testing.T) {
	tests := []struct {
		name   string
		cfg    NeonConfig
		wantIn []string
	}{
		{
			name: "with endpoint ID",
			cfg: NeonConfig{
				BaseConfig: BaseConfig{
					User:     "user",
					Password: "pass",
					DBName:   "neondb",
				},
				EndpointID: "ep-cool-name-123",
			},
			wantIn: []string{
				"ep-cool-name-123.neon.tech",
				"sslmode=require",
			},
		},
		{
			name: "with pooler",
			cfg: NeonConfig{
				BaseConfig: BaseConfig{
					User:     "user",
					Password: "pass",
					DBName:   "neondb",
				},
				EndpointID: "ep-cool-name-123",
				UsePooler:  true,
			},
			wantIn: []string{
				"ep-cool-name-123-pooler.neon.tech",
			},
		},
		{
			name: "insecure",
			cfg: NeonConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					User:     "user",
					Password: "pass",
					DBName:   "test",
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
			got, err := buildNeonDSN(tt.cfg)
			require.NoError(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, got, want)
			}
		})
	}
}

func TestBuildNeonDSN_ConnectionURL(t *testing.T) {
	cfg := NeonConfig{
		ConnectionURL: "postgresql://user:pass@ep-cool.neon.tech/neondb",
	}

	got, err := buildNeonDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=require")
}
