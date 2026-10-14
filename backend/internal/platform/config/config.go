// Package config charge la configuration du service depuis l'environnement.
//
// Toute valeur sensible peut aussi être lue depuis un fichier en définissant
// NOM_FILE (convention des secrets Docker) : DATABASE_URL_FILE=/run/secrets/db_url.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env est l'environnement d'exécution.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvTest        Env = "test"
	EnvProduction  Env = "production"
)

// Getenv lit une variable d'environnement ; os.LookupEnv en production.
type Getenv func(key string) (string, bool)

// Config regroupe les réglages du service.
type Config struct {
	Env            Env
	HTTPAddr       string
	DatabaseURL    string
	MaxBodyBytes   int64
	TrustedProxies []string
	LogLevel       slog.Level

	// Authentification
	JWTSigningKey       []byte
	CookieSecure        bool
	PublicURL           string // URL publique de l'API (sans /api/v1)
	FrontendURL         string // où renvoyer le navigateur après la connexion
	MinGitHubAccountAge time.Duration

	// GitHub : identifiants OAuth (connexion) et GitHub App (preuves).
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubAppID         int64
	GitHubAppPrivateKey []byte
	GitHubWebhookSecret []byte

	// Coffre : clé maître (KEK) de 32 octets.
	VaultKEK []byte
}

// GitHubOAuthEnabled indique si la connexion GitHub est configurée.
func (c Config) GitHubOAuthEnabled() bool { return c.GitHubClientID != "" }

// GitHubAppEnabled indique si la GitHub App (preuves et webhooks) est configurée.
func (c Config) GitHubAppEnabled() bool { return c.GitHubAppID != 0 }

// OAuthRedirectURL est l'URL de retour déclarée dans l'application GitHub.
func (c Config) OAuthRedirectURL() string {
	return strings.TrimRight(c.PublicURL, "/") + "/api/v1/auth/github/callback"
}

// IsProduction indique si les protections de production s'appliquent.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// FromOS charge la configuration depuis l'environnement du processus.
func FromOS() (Config, error) { return Load(os.LookupEnv) }

// Load valide et charge la configuration. Toutes les erreurs sont renvoyées
// ensemble pour corriger la configuration en une fois.
func Load(getenv Getenv) (Config, error) {
	cfg := Config{
		Env:          EnvDevelopment,
		HTTPAddr:     ":8080",
		MaxBodyBytes: 1 << 20,
		LogLevel:     slog.LevelInfo,
	}
	var errs []error

	if v, ok := getenv("APP_ENV"); ok {
		switch Env(v) {
		case EnvDevelopment, EnvTest, EnvProduction:
			cfg.Env = Env(v)
		default:
			errs = append(errs, fmt.Errorf("APP_ENV : valeur %q inconnue (development, test, production)", v))
		}
	}

	if v, ok := getenv("HTTP_ADDR"); ok && v != "" {
		cfg.HTTPAddr = v
	}

	if v, ok := getenv("MAX_BODY_BYTES"); ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			errs = append(errs, fmt.Errorf("MAX_BODY_BYTES : entier strictement positif attendu, reçu %q", v))
		} else {
			cfg.MaxBodyBytes = n
		}
	}

	if v, ok := getenv("TRUSTED_PROXIES"); ok && strings.TrimSpace(v) != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if !validProxy(p) {
				errs = append(errs, fmt.Errorf("TRUSTED_PROXIES : %q n'est ni une IP ni un CIDR", p))
				continue
			}
			cfg.TrustedProxies = append(cfg.TrustedProxies, p)
		}
	}

	if v, ok := getenv("LOG_LEVEL"); ok {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			errs = append(errs, fmt.Errorf("LOG_LEVEL : valeur %q inconnue (debug, info, warn, error)", v))
		}
	}

	dbURL, err := secret(getenv, "DATABASE_URL")
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DatabaseURL = dbURL

	errs = append(errs, loadAuth(getenv, &cfg)...)
	errs = append(errs, loadGitHub(getenv, &cfg)...)
	errs = append(errs, loadVault(getenv, &cfg)...)
	return cfg, errors.Join(errs...)
}

func loadAuth(getenv Getenv, cfg *Config) []error {
	var errs []error
	key, err := secret(getenv, "JWT_SIGNING_KEY")
	if err != nil {
		errs = append(errs, err)
	} else if len(key) < 32 {
		errs = append(errs, errors.New("JWT_SIGNING_KEY : 32 octets minimum"))
	}
	cfg.JWTSigningKey = []byte(key)

	cfg.CookieSecure = true
	if v, ok := getenv("COOKIE_SECURE"); ok {
		b, err := strconv.ParseBool(v)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("COOKIE_SECURE : booléen attendu, reçu %q", v))
		case !b && cfg.IsProduction():
			errs = append(errs, errors.New("COOKIE_SECURE=false est interdit en production"))
		default:
			cfg.CookieSecure = b
		}
	}

	cfg.PublicURL = "http://localhost:8000"
	if v, ok := getenv("PUBLIC_URL"); ok && v != "" {
		cfg.PublicURL = v
	}
	if cfg.IsProduction() && !strings.HasPrefix(cfg.PublicURL, "https://") {
		errs = append(errs, errors.New("PUBLIC_URL doit être en https:// en production"))
	}
	cfg.FrontendURL = "/"
	if v, ok := getenv("FRONTEND_URL"); ok && v != "" {
		cfg.FrontendURL = v
	}

	if v, ok := getenv("GITHUB_MIN_ACCOUNT_AGE_DAYS"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("GITHUB_MIN_ACCOUNT_AGE_DAYS : entier positif attendu, reçu %q", v))
		} else {
			cfg.MinGitHubAccountAge = time.Duration(n) * 24 * time.Hour
		}
	}
	return errs
}

func loadGitHub(getenv Getenv, cfg *Config) []error {
	var errs []error
	cfg.GitHubClientID, _ = getenv("GITHUB_CLIENT_ID")
	clientSecret, err := optionalSecret(getenv, "GITHUB_CLIENT_SECRET")
	if err != nil {
		errs = append(errs, err)
	}
	cfg.GitHubClientSecret = clientSecret
	if (cfg.GitHubClientID == "") != (cfg.GitHubClientSecret == "") {
		errs = append(errs, errors.New("GITHUB_CLIENT_ID et GITHUB_CLIENT_SECRET vont ensemble"))
	}

	appID, _ := getenv("GITHUB_APP_ID")
	key, err := optionalSecret(getenv, "GITHUB_APP_PRIVATE_KEY")
	if err != nil {
		errs = append(errs, err)
	}
	webhook, err := optionalSecret(getenv, "GITHUB_WEBHOOK_SECRET")
	if err != nil {
		errs = append(errs, err)
	}
	if appID != "" || key != "" {
		id, err := strconv.ParseInt(appID, 10, 64)
		switch {
		case err != nil || id <= 0:
			errs = append(errs, fmt.Errorf("GITHUB_APP_ID : entier attendu, reçu %q", appID))
		case key == "":
			errs = append(errs, errors.New("GITHUB_APP_PRIVATE_KEY est requis avec GITHUB_APP_ID"))
		case webhook == "":
			errs = append(errs, errors.New("GITHUB_WEBHOOK_SECRET est requis avec la GitHub App"))
		default:
			cfg.GitHubAppID = id
			cfg.GitHubAppPrivateKey = []byte(key)
			cfg.GitHubWebhookSecret = []byte(webhook)
		}
	}

	if cfg.IsProduction() {
		if !cfg.GitHubOAuthEnabled() {
			errs = append(errs, errors.New("production : GITHUB_CLIENT_ID et GITHUB_CLIENT_SECRET sont obligatoires"))
		}
		if appID == "" {
			errs = append(errs, errors.New("production : la GitHub App (GITHUB_APP_ID, clé, secret de webhook) est obligatoire"))
		}
	}
	return errs
}

func loadVault(getenv Getenv, cfg *Config) []error {
	v, err := optionalSecret(getenv, "VAULT_KEK")
	if err != nil {
		return []error{err}
	}
	if v == "" {
		if cfg.IsProduction() {
			return []error{errors.New("production : VAULT_KEK est obligatoire")}
		}
		return nil
	}
	kek, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(kek) != 32 {
		return []error{errors.New("VAULT_KEK : 32 octets encodés en base64 attendus")}
	}
	cfg.VaultKEK = kek
	return nil
}

// optionalSecret lit un secret facultatif (vide s'il n'est pas défini).
func optionalSecret(getenv Getenv, name string) (string, error) {
	_, hasValue := getenv(name)
	_, hasFile := getenv(name + "_FILE")
	if !hasValue && !hasFile {
		return "", nil
	}
	v, err := secret(getenv, name)
	if err != nil && !hasFile {
		return "", nil // défini mais vide : considéré comme absent
	}
	return v, err
}

// secret lit une valeur obligatoire, directement ou depuis NOM_FILE.
func secret(getenv Getenv, name string) (string, error) {
	value, hasValue := getenv(name)
	path, hasFile := getenv(name + "_FILE")
	switch {
	case hasValue && hasFile:
		return "", fmt.Errorf("%s et %s_FILE sont définis : n'en garder qu'un", name, name)
	case hasFile:
		b, err := os.ReadFile(path) // #nosec G304 -- chemin fourni par l'opérateur, pas par un client
		if err != nil {
			return "", fmt.Errorf("%s_FILE : lecture impossible : %w", name, err)
		}
		value = string(b)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s est obligatoire (ou %s_FILE)", name, name)
	}
	return value, nil
}

func validProxy(s string) bool {
	if _, err := netip.ParseAddr(s); err == nil {
		return true
	}
	_, err := netip.ParsePrefix(s)
	return err == nil
}
