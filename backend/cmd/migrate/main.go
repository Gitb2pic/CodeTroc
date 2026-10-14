// Commande migrate : applique les migrations du schéma avant le démarrage de l'API.
//
//	migrate up    applique les migrations en attente (par défaut)
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/platform/config"
	"github.com/gitb2pic/codetroc/backend/internal/platform/logging"
)

func main() {
	logger := logging.New(os.Stdout, slog.LevelInfo, "migrate")
	dbURL, err := config.DatabaseURL(os.LookupEnv)
	if err != nil {
		logger.Error("configuration invalide", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	// La descente n'est volontairement pas exposée : en production, on corrige
	// par une nouvelle migration, jamais en annulant.
	if cmd != "up" {
		logger.Error("commande inconnue", slog.String("commande", cmd))
		fmt.Fprintln(os.Stderr, "usage : migrate [up]")
		os.Exit(2)
	}
	if err := postgres.MigrateUp(dbURL); err != nil {
		logger.Error("échec des migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("migrations à jour")
}
