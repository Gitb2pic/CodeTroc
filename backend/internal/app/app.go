// Package app contient les cas d'utilisation de CodeTroc. Chaque cas
// d'utilisation s'exécute dans une transaction PostgreSQL (voir ADR-001).
package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

// Role d'un utilisateur. ADMIN dispose aussi des droits d'arbitre.
type Role string

const (
	RoleMember  Role = "MEMBRE"
	RoleArbiter Role = "ARBITRE"
	RoleAdmin   Role = "ADMIN"
)

// Actor est l'utilisateur authentifié qui appelle un cas d'utilisation.
type Actor struct {
	ID   uuid.UUID
	Role Role
}

// CanArbitrate indique si l'acteur peut trancher des litiges.
func (a Actor) CanArbitrate() bool { return a.Role == RoleArbiter || a.Role == RoleAdmin }

// Settings regroupe les paramètres métier.
type Settings struct {
	RefreshTTL          time.Duration
	MinGitHubAccountAge time.Duration
	WelcomeBonus        int64
	TacitValidation     time.Duration
	AbandonGrace        time.Duration
	MaxSecretLifetime   time.Duration
}

// DefaultSettings renvoie les valeurs de la documentation technique.
func DefaultSettings() Settings {
	return Settings{
		RefreshTTL:        30 * 24 * time.Hour,
		WelcomeBonus:      50,
		TacitValidation:   72 * time.Hour,
		AbandonGrace:      48 * time.Hour,
		MaxSecretLifetime: 30 * 24 * time.Hour,
	}
}

// Deps regroupe les dépendances du service.
type Deps struct {
	DB       *pgxpool.Pool
	Now      func() time.Time
	Tokens   *token.Issuer
	OAuth    GitHubOAuth // nil si GitHub n'est pas configuré
	GitHub   GitHubAPI   // nil si la GitHub App n'est pas configurée
	Vault    Sealer      // nil si le coffre n'est pas configuré
	AppURL   string      // URL publique du site, pour les liens des emails
	Mailer   Mailer      // nil : inscription et connexion par email fermées (F18)
	Settings Settings
}

// Service expose les cas d'utilisation.
type Service struct {
	db       *pgxpool.Pool
	now      func() time.Time
	tokens   *token.Issuer
	oauth    GitHubOAuth
	github   GitHubAPI
	vault    Sealer
	appURL   string
	settings Settings
	mailer   Mailer
}

// New construit le service.
func New(d Deps) *Service {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &Service{db: d.DB, now: func() time.Time { return now().UTC() }, tokens: d.Tokens, oauth: d.OAuth, github: d.GitHub, vault: d.Vault,
		appURL: strings.TrimRight(d.AppURL, "/"), settings: d.Settings, mailer: d.Mailer}
}

// tx exécute fn dans une transaction READ COMMITTED : validée si fn réussit,
// annulée sinon.
func (s *Service) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, s.db, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

// notFoundIfNoRows traduit l'absence de ligne en 404.
func notFoundIfNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
