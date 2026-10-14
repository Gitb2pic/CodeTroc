// Package wiring assemble les cas d'utilisation et leurs adaptateurs à partir
// de la configuration ; partagé par l'API et le worker.
package wiring

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ghadapter "github.com/gitb2pic/codetroc/backend/internal/adapters/github"
	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/platform/config"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

// AccessTokenTTL est la durée de vie d'un jeton d'accès.
const AccessTokenTTL = 15 * time.Minute

// Service construit le service applicatif.
func Service(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*app.Service, error) {
	tokens, err := token.NewIssuer(cfg.JWTSigningKey, AccessTokenTTL, time.Now)
	if err != nil {
		return nil, err
	}
	settings := app.DefaultSettings()
	settings.MinGitHubAccountAge = cfg.MinGitHubAccountAge

	deps := app.Deps{DB: pool, Tokens: tokens, Settings: settings}
	if cfg.GitHubOAuthEnabled() {
		deps.OAuth = &ghadapter.OAuth{ClientID: cfg.GitHubClientID, ClientSecret: cfg.GitHubClientSecret, RedirectURL: cfg.OAuthRedirectURL()}
	} else {
		logger.Warn("connexion GitHub non configurée (GITHUB_CLIENT_ID absent)")
	}
	if cfg.GitHubAppEnabled() {
		gh, err := ghadapter.NewApp(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
		if err != nil {
			return nil, err
		}
		deps.GitHub = gh
	} else {
		logger.Warn("GitHub App non configurée : les preuves PR resteront en attente")
	}
	return app.New(deps), nil
}
