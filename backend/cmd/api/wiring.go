package main

import (
	httpapi "github.com/gitb2pic/codetroc/backend/internal/adapters/http"
	"github.com/gitb2pic/codetroc/backend/internal/platform/config"
)

func webhookConfig(cfg config.Config) *httpapi.WebhookConfig {
	if !cfg.GitHubAppEnabled() {
		return nil
	}
	return &httpapi.WebhookConfig{Secret: cfg.GitHubWebhookSecret}
}
