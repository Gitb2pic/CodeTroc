// Package config charge la configuration du service depuis l'environnement.
//
// Toute valeur sensible peut aussi être lue depuis un fichier en définissant
// NOM_FILE (convention des secrets Docker) : DATABASE_URL_FILE=/run/secrets/db_url.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
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

	return cfg, errors.Join(errs...)
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
