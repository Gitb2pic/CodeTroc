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
		securityHeaders(),
		bodyLimit(o.MaxBodyBytes),
	)
	r.NoRoute(func(c *gin.Context) { WriteProblem(c, ErrNotFound) })
	r.NoMethod(func(c *gin.Context) { WriteProblem(c, ErrMethodNotAllowed) })

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	v1.GET("/ready", ready(o.DB))
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
