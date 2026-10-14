package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://u:p@db/codetroc", "JWT_SIGNING_KEY": jwtKey}))
	require.NoError(t, err)

	assert.Equal(t, EnvDevelopment, cfg.Env)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, int64(1<<20), cfg.MaxBodyBytes)
	assert.Equal(t, "postgres://u:p@db/codetroc", cfg.DatabaseURL)
	assert.Empty(t, cfg.TrustedProxies)
}

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	_, err := Load(env(map[string]string{"JWT_SIGNING_KEY": jwtKey}))
	require.ErrorContains(t, err, "DATABASE_URL")
}

func TestLoad_ReadsSecretFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db_url")
	require.NoError(t, os.WriteFile(path, []byte("postgres://u:p@db/codetroc\n"), 0o600))

	cfg, err := Load(env(map[string]string{"DATABASE_URL_FILE": path, "JWT_SIGNING_KEY": jwtKey}))
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
	cfg, err := Load(env(prod(map[string]string{
		"HTTP_ADDR":       ":9000",
		"MAX_BODY_BYTES":  "2048",
		"TRUSTED_PROXIES": "10.0.0.0/8, 172.16.0.1",
		"LOG_LEVEL":       "debug",
	})))
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
			_, err := Load(env(base(vars)))
			require.Error(t, err)
		})
	}
}

const jwtKey = "0123456789abcdef0123456789abcdef" // gitleaks:allow (clé de test)

func base(extra map[string]string) map[string]string {
	m := map[string]string{"DATABASE_URL": "postgres://db", "JWT_SIGNING_KEY": jwtKey}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func prod(extra map[string]string) map[string]string {
	m := base(map[string]string{
		"APP_ENV":                "production",
		"GITHUB_CLIENT_ID":       "Iv1.abc",
		"GITHUB_CLIENT_SECRET":   "s3cret",
		"GITHUB_APP_ID":          "12345",
		"GITHUB_APP_PRIVATE_KEY": "-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----",
		"GITHUB_WEBHOOK_SECRET":  "whsec",
		"VAULT_KEK":              "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		"PUBLIC_URL":             "https://codetroc.dev",
		"FRONTEND_URL":           "https://codetroc.dev/",
	})
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestLoad_AuthAndGitHubSettings(t *testing.T) {
	cfg, err := Load(env(base(nil)))
	require.NoError(t, err)
	assert.Equal(t, []byte(jwtKey), cfg.JWTSigningKey)
	assert.False(t, cfg.GitHubOAuthEnabled(), "GitHub facultatif en développement")
	assert.False(t, cfg.GitHubAppEnabled())
	assert.Nil(t, cfg.VaultKEK)
	assert.True(t, cfg.CookieSecure)
	assert.Equal(t, "http://localhost:8000", cfg.PublicURL)

	cfg, err = Load(env(prod(map[string]string{"GITHUB_MIN_ACCOUNT_AGE_DAYS": "30"})))
	require.NoError(t, err)
	assert.True(t, cfg.GitHubOAuthEnabled())
	assert.True(t, cfg.GitHubAppEnabled())
	assert.Equal(t, int64(12345), cfg.GitHubAppID)
	assert.Len(t, cfg.VaultKEK, 32)
	assert.Equal(t, 30*24*time.Hour, cfg.MinGitHubAccountAge)
	assert.Equal(t, "https://codetroc.dev/api/v1/auth/github/callback", cfg.OAuthRedirectURL())
}

func TestLoad_AuthAndGitHubRejections(t *testing.T) {
	cases := map[string]map[string]string{
		"clé JWT absente":            {"JWT_SIGNING_KEY": ""},
		"clé JWT courte":             {"JWT_SIGNING_KEY": "court"},
		"client GitHub sans secret":  {"GITHUB_CLIENT_ID": "x"},
		"App sans clé privée":        {"GITHUB_APP_ID": "1"},
		"App sans secret de webhook": {"GITHUB_APP_ID": "1", "GITHUB_APP_PRIVATE_KEY": "k"},
		"App id non numérique":       {"GITHUB_APP_ID": "abc", "GITHUB_APP_PRIVATE_KEY": "k", "GITHUB_WEBHOOK_SECRET": "w"},
		"KEK pas en base64":          {"VAULT_KEK": "%%%"},
		"KEK de mauvaise taille":     {"VAULT_KEK": "YWJj"},
		"âge minimal négatif":        {"GITHUB_MIN_ACCOUNT_AGE_DAYS": "-1"},
		"COOKIE_SECURE illisible":    {"COOKIE_SECURE": "peut-être"},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(env(base(vars)))
			require.Error(t, err)
		})
	}

	prodCases := map[string]map[string]string{
		"production sans OAuth":        {"GITHUB_CLIENT_ID": "", "GITHUB_CLIENT_SECRET": ""},
		"production sans App":          {"GITHUB_APP_ID": "", "GITHUB_APP_PRIVATE_KEY": "", "GITHUB_WEBHOOK_SECRET": ""},
		"production sans coffre":       {"VAULT_KEK": ""},
		"production cookies en clair":  {"COOKIE_SECURE": "false"},
		"production URL publique http": {"PUBLIC_URL": "http://codetroc.dev"},
	}
	for name, vars := range prodCases {
		t.Run(name, func(t *testing.T) {
			m := prod(nil)
			for k, v := range vars {
				if v == "" {
					delete(m, k)
				} else {
					m[k] = v
				}
			}
			_, err := Load(env(m))
			require.Error(t, err)
		})
	}
}

func TestDatabaseURL_NeedsNothingElse(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://db/codetroc"}
	got, err := DatabaseURL(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	require.NoError(t, err)
	assert.Equal(t, "postgres://db/codetroc", got)
	_, err = DatabaseURL(func(string) (string, bool) { return "", false })
	assert.Error(t, err)
}
