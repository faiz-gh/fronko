package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/fronko")
	t.Setenv("JWT_SECRET", "0123456789abcdef")
}

func TestDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("FRONKO_ENV", "")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, EnvProduction, cfg.Env)
	assert.Equal(t, "", cfg.PublicURL)
	assert.Equal(t, 2, cfg.JobWorkers)
}

func TestEnvironment(t *testing.T) {
	for raw, want := range map[string]string{"": EnvProduction, "production": EnvProduction, "development": EnvDevelopment} {
		t.Setenv("FRONKO_ENV", raw)
		env, err := Environment()
		require.NoError(t, err)
		assert.Equal(t, want, env)
	}
	t.Setenv("FRONKO_ENV", "staging")
	_, err := Environment()
	assert.ErrorContains(t, err, "FRONKO_ENV")
}

func TestPublicURL(t *testing.T) {
	setRequired(t)
	for raw, want := range map[string]string{
		"https://cards.example.com":    "https://cards.example.com",
		" https://cards.example.com/ ": "https://cards.example.com",
		"http://localhost:3000":        "http://localhost:3000",
		"https://example.com/fronko/":  "https://example.com/fronko",
	} {
		t.Setenv("PUBLIC_URL", raw)
		cfg, err := Load()
		require.NoError(t, err, raw)
		assert.Equal(t, want, cfg.PublicURL, raw)
	}
	for _, bad := range []string{"cards.example.com", "ftp://example.com", "https://", "https://u:p@example.com", "https://example.com/?a=1", "https://example.com/#x"} {
		t.Setenv("PUBLIC_URL", bad)
		_, err := Load()
		assert.ErrorContains(t, err, "PUBLIC_URL", bad)
	}
}

func TestJobWorkers(t *testing.T) {
	setRequired(t)
	t.Setenv("JOB_WORKERS", "0")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.JobWorkers)
	for _, bad := range []string{"-1", "33", "two"} {
		t.Setenv("JOB_WORKERS", bad)
		_, err := Load()
		assert.ErrorContains(t, err, "JOB_WORKERS", bad)
	}
}
