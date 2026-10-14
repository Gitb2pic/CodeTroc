// Commande api : serveur HTTP de CodeTroc.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gitb2pic/codetroc/backend/api"
	httpapi "github.com/gitb2pic/codetroc/backend/internal/adapters/http"
	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/platform/config"
	"github.com/gitb2pic/codetroc/backend/internal/platform/logging"
)

const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("arrêt sur erreur", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromOS()
	if err != nil {
		return err
	}
	logger := logging.New(os.Stdout, cfg.LogLevel, "api")
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	svc, err := buildService(cfg, pool, logger)
	if err != nil {
		return err
	}
	router, err := httpapi.NewRouter(httpapi.Options{
		Logger:         logger,
		DB:             pool,
		MaxBodyBytes:   cfg.MaxBodyBytes,
		TrustedProxies: cfg.TrustedProxies,
		ServeSpec:      !cfg.IsProduction(),
		Spec:           api.Spec,
		App:            svc,
		FrontendURL:    cfg.FrontendURL,
		CookieSecure:   cfg.CookieSecure,
		Webhooks:       webhookConfig(cfg),
	})
	if err != nil {
		return err
	}

	// Délais explicites : protègent contre les connexions lentes (Slowloris).
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("démarrage", slog.String("addr", cfg.HTTPAddr), slog.String("env", string(cfg.Env)))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("arrêt demandé, fin des requêtes en cours")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}
