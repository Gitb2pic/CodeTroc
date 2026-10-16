// Commande worker : traitements différés de CodeTroc (validation tacite,
// réconciliation des preuves GitHub, expiration des secrets, ménage,
// contrôle du grand livre). Plusieurs instances peuvent tourner : une seule
// travaille à la fois grâce à un verrou consultatif PostgreSQL.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/platform/config"
	"github.com/gitb2pic/codetroc/backend/internal/platform/logging"
	"github.com/gitb2pic/codetroc/backend/internal/wiring"
)

const interval = time.Minute

func main() {
	cfg, err := config.FromOS()
	if err != nil {
		slog.Error("configuration invalide", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel, "worker")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("base", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	svc, err := wiring.Service(cfg, pool, logger)
	if err != nil {
		logger.Error("assemblage", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("démarrage", slog.Duration("intervalle", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		r, err := svc.RunScheduledJobs(ctx)
		switch {
		case err != nil:
			logger.Error("passage en échec", slog.String("error", err.Error()))
		case !r.Ran:
			logger.Debug("autre instance active")
		default:
			level := slog.LevelInfo
			if r.LedgerMismatches > 0 || r.LedgerTotal != 0 {
				level = slog.LevelError // alerte : le grand livre est incohérent
			}
			logger.Log(ctx, level, "passage", slog.Int("liberations_tacites", r.TacitReleases),
				slog.Int("preuves_reverifiees", r.ProofsRechecked), slog.Int("secrets_expires", r.SecretsExpired), slog.Int("annonces_expirees", r.AnnouncementsExp),
				slog.Int("incoherences_grand_livre", r.LedgerMismatches), slog.Int64("somme_soldes", r.LedgerTotal))
		}
		select {
		case <-ctx.Done():
			logger.Info("arrêt")
			return
		case <-ticker.C:
		}
	}
}
