package main

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ghadapter "github.com/gitb2pic/codetroc/backend/internal/adapters/github"
	httpapi "github.com/gitb2pic/codetroc/backend/internal/adapters/http"
	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/platform/config"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

const accessTokenTTL = 15 * time.Minute

// buildService assemble les cas d'utilisation et leurs adaptateurs.
func buildService(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*app.Service, error) {
	tokens, err := token.NewIssuer(cfg.JWTSigningKey, accessTokenTTL, time.Now)
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
	return app.New(deps), nil
}

func webhookConfig(cfg config.Config) *httpapi.WebhookConfig {
	if !cfg.GitHubAppEnabled() {
		return nil
	}
	return &httpapi.WebhookConfig{Secret: cfg.GitHubWebhookSecret}
}
