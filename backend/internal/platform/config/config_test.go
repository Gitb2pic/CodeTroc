package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// env construit une source de variables d'environnement pour les tests.
func env(vars map[string]string) Getenv {
	return func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://u:p@db/codetroc"}))
	require.NoError(t, err)

	assert.Equal(t, EnvDevelopment, cfg.Env)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, int64(1<<20), cfg.MaxBodyBytes)
	assert.Equal(t, "postgres://u:p@db/codetroc", cfg.DatabaseURL)
	assert.Empty(t, cfg.TrustedProxies)
}

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	_, err := Load(env(map[string]string{}))
	require.ErrorContains(t, err, "DATABASE_URL")
}

func TestLoad_ReadsSecretFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db_url")
	require.NoError(t, os.WriteFile(path, []byte("postgres://u:p@db/codetroc\n"), 0o600))

	cfg, err := Load(env(map[string]string{"DATABASE_URL_FILE": path}))
	require.NoError(t, err)
	assert.Equal(t, "postgres://u:p@db/codetroc", cfg.DatabaseURL)
}

func TestLoad_RejectsValueAndFileTogether(t *testing.T) {
	_, err := Load(env(map[string]string{"DATABASE_URL": "x", "DATABASE_URL_FILE": "/run/secrets/db"}))
	require.ErrorContains(t, err, "DATABASE_URL")
}

func TestLoad_RejectsUnreadableSecretFile(t *testing.T) {
	_, err := Load(env(map[string]string{"DATABASE_URL_FILE": filepath.Join(t.TempDir(), "absent")}))
	require.ErrorContains(t, err, "DATABASE_URL_FILE")
}

func TestLoad_RejectsEmptySecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db_url")
	require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))

	_, err := Load(env(map[string]string{"DATABASE_URL_FILE": path}))
	require.ErrorContains(t, err, "DATABASE_URL")
}

func TestLoad_ParsesOptionalSettings(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"DATABASE_URL":    "postgres://db",
		"APP_ENV":         "production",
		"HTTP_ADDR":       ":9000",
		"MAX_BODY_BYTES":  "2048",
		"TRUSTED_PROXIES": "10.0.0.0/8, 172.16.0.1",
		"LOG_LEVEL":       "debug",
	}))
	require.NoError(t, err)

	assert.Equal(t, EnvProduction, cfg.Env)
	assert.True(t, cfg.IsProduction())
	assert.Equal(t, ":9000", cfg.HTTPAddr)
	assert.Equal(t, int64(2048), cfg.MaxBodyBytes)
	assert.Equal(t, []string{"10.0.0.0/8", "172.16.0.1"}, cfg.TrustedProxies)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	cases := map[string]map[string]string{
		"environnement inconnu": {"APP_ENV": "staging-2"},
		"taille non numérique":  {"MAX_BODY_BYTES": "beaucoup"},
		"taille nulle":          {"MAX_BODY_BYTES": "0"},
		"niveau de log inconnu": {"LOG_LEVEL": "bavard"},
		"proxy invalide":        {"TRUSTED_PROXIES": "pas-une-ip"},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			vars["DATABASE_URL"] = "postgres://db"
			_, err := Load(env(vars))
			require.Error(t, err)
		})
	}
}
