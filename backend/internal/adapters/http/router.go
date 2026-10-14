// Package httpapi expose l'API REST de CodeTroc avec Gin.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// Pinger vérifie que la base répond (implémenté par *pgxpool.Pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Options configure le routeur.
type Options struct {
	Logger         *slog.Logger
	DB             Pinger
	MaxBodyBytes   int64
	TrustedProxies []string
	// ServeSpec expose openapi.yaml ; désactivé en production.
	ServeSpec bool
	Spec      []byte

	// App porte les cas d'utilisation ; sans lui, seules les routes
	// d'exploitation sont montées.
	App          *app.Service
	FrontendURL  string
	CookieSecure bool
	Webhooks     *WebhookConfig
	// HSTS ajoute Strict-Transport-Security (production, servie en HTTPS).
	HSTS bool
	// RateLimits : nil = DefaultRateLimits().
	RateLimits *RateLimits

	authLimiter, writeLimiter *limiter
}

const readyTimeout = 2 * time.Second

var ginMode sync.Once

// NewRouter construit le routeur HTTP et ses middlewares.
func NewRouter(o Options) (*gin.Engine, error) {
	if o.Logger == nil || o.DB == nil {
		return nil, errors.New("httpapi : Logger et DB sont obligatoires")
	}
	if o.MaxBodyBytes <= 0 {
		return nil, errors.New("httpapi : MaxBodyBytes doit être strictement positif")
	}
	ginMode.Do(func() { gin.SetMode(gin.ReleaseMode) })

	r := gin.New()
	r.HandleMethodNotAllowed = true
	if err := r.SetTrustedProxies(o.TrustedProxies); err != nil {
		return nil, fmt.Errorf("httpapi : proxys de confiance : %w", err)
	}

	r.Use(
		requestIDMiddleware(),
		accessLog(o.Logger),
		recovery(o.Logger),
		securityHeaders(o.HSTS),
		bodyLimit(o.MaxBodyBytes),
	)
	limits := DefaultRateLimits()
	if o.RateLimits != nil {
		limits = *o.RateLimits
	}
	// Par adresse IP (derrière les proxys de confiance), sauf les webhooks GitHub, authentifiés par signature.
	r.Use(rateLimit(newLimiter(limits.IP, time.Now), func(c *gin.Context) string {
		if c.Request.URL.Path == "/api/v1/webhooks/github" {
			return ""
		}
		return c.ClientIP()
	}))
	o.writeLimiter = newLimiter(limits.Write, time.Now)
	o.authLimiter = newLimiter(limits.Auth, time.Now)
	r.NoRoute(func(c *gin.Context) { WriteProblem(c, ErrNotFound) })
	r.NoMethod(func(c *gin.Context) { WriteProblem(c, ErrMethodNotAllowed) })

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	v1.GET("/ready", ready(o.DB))
	if o.App != nil {
		mountRoutes(v1, o)
	}
	if o.ServeSpec {
		v1.GET("/openapi.yaml", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/yaml", o.Spec)
		})
	}
	return r, nil
}

func ready(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			_ = c.Error(fmt.Errorf("base injoignable : %w", err))
			WriteProblem(c, ErrUnavailable)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}
