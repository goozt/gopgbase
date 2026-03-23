package adaptors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTimescaleDSN_FieldBased(t *testing.T) {
	cfg := TimescaleConfig{
		BaseConfig: BaseConfig{
			Host:     "ts.example.com",
			Port:     5432,
			User:     "tsdbadmin",
			Password: "pass",
			DBName:   "tsdb",
		},
	}

	got, err := buildTimescaleDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "ts.example.com")
	assert.Contains(t, got, "sslmode=verify-full")
}

func TestBuildTimescaleDSN_Insecure(t *testing.T) {
	cfg := TimescaleConfig{
		BaseConfig: BaseConfig{
			Host:     "localhost",
			User:     "postgres",
			Password: "pass",
			DBName:   "tsdb",
			Insecure: true,
		},
	}

	got, err := buildTimescaleDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=disable")
}

func TestBuildTimescaleDSN_ConnectionURL(t *testing.T) {
	cfg := TimescaleConfig{
		ConnectionURL: "postgresql://user:pass@host/db",
	}

	got, err := buildTimescaleDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, got, "sslmode=verify-full")
}
