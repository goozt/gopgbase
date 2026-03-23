package adaptors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCockroachDSN_FieldBased(t *testing.T) {
	tests := []struct {
		name   string
		wantIn []string
		cfg    CockroachConfig
	}{
		{
			name: "basic secure",
			cfg: CockroachConfig{
				BaseConfig: BaseConfig{
					Host:     "free-tier.cockroachlabs.cloud",
					User:     "user",
					Password: "pass",
					DBName:   "defaultdb",
				},
			},
			wantIn: []string{
				"free-tier.cockroachlabs.cloud",
				"26257",
				"sslmode=verify-full",
			},
		},
		{
			name: "with cluster ID",
			cfg: CockroachConfig{
				BaseConfig: BaseConfig{
					Host:     "host",
					User:     "user",
					Password: "pass",
					DBName:   "db",
				},
				ClusterID: "my-cluster-123",
			},
			wantIn: []string{
				"options=--cluster",
				"my-cluster-123",
			},
		},
		{
			name: "with routing ID",
			cfg: CockroachConfig{
				BaseConfig: BaseConfig{
					Host:     "host",
					User:     "user",
					Password: "pass",
					DBName:   "db",
				},
				RoutingID: "routing-456",
			},
			wantIn: []string{
				"options=--cluster",
				"routing-456",
			},
		},
		{
			name: "insecure local",
			cfg: CockroachConfig{
				BaseConfig: BaseConfig{
					Host:     "localhost",
					User:     "root",
					DBName:   "defaultdb",
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
			got, err := buildCockroachDSN(tt.cfg)
			require.NoError(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, got, want)
			}
		})
	}
}

func TestBuildCockroachDSN_ConnectionURL(t *testing.T) {
	cfg := CockroachConfig{
		ConnectionURL: "postgresql://user:pass@host:26257/db",
		ClusterID:     "my-cluster",
	}

	got, err := buildCockroachDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=verify-full")
	assert.Contains(t, got, "my-cluster")
}
