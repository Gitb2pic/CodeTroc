// Package postgres regroupe l'accès à PostgreSQL : pool de connexions et migrations.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pilote pgx5://
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gitb2pic/codetroc/backend/migrations"
)

// Open crée le pool de connexions. La connexion est établie à la demande :
// le service démarre même si la base n'est pas encore prête (/ready le signale).
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("postgres : DATABASE_URL invalide") // l'URL contient le mot de passe : ne pas la citer
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// MigrateUp applique toutes les migrations en attente.
func MigrateUp(databaseURL string) error {
	return withMigrator(databaseURL, func(m *migrate.Migrate) error { return m.Up() })
}

// MigrateDown annule toutes les migrations (tests et environnements jetables uniquement).
func MigrateDown(databaseURL string) error {
	return withMigrator(databaseURL, func(m *migrate.Migrate) error { return m.Down() })
}

func withMigrator(databaseURL string, run func(*migrate.Migrate) error) (err error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migrations : %w", err)
	}
	target, err := pgx5URL(databaseURL)
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, target)
	if err != nil {
		return fmt.Errorf("migrations : connexion impossible : %w", redact(err, databaseURL, target))
	}
	defer func() {
		srcErr, dbErr := m.Close()
		err = errors.Join(err, srcErr, dbErr)
	}()
	if err := run(m); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations : %w", err)
	}
	return nil
}

// pgx5URL convertit une URL postgres:// vers le schéma du pilote golang-migrate.
func pgx5URL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", errors.New("migrations : DATABASE_URL doit commencer par postgres://")
	}
	u.Scheme = "pgx5"
	return u.String(), nil
}

// redact retire les URL (et donc le mot de passe) d'un message d'erreur.
func redact(err error, urls ...string) error {
	msg := err.Error()
	for _, u := range urls {
		if u != "" {
			msg = strings.ReplaceAll(msg, u, "<DATABASE_URL>")
		}
	}
	return errors.New(msg)
}

// MigrateUpDB applique les migrations sur une connexion déjà ouverte (bac à
// sable navigateur, où la base n'est pas joignable par URL).
func MigrateUpDB(db *sql.DB) (err error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migrations : %w", err)
	}
	drv, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("migrations : %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", drv)
	if err != nil {
		return fmt.Errorf("migrations : %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations : %w", err)
	}
	return nil
}
