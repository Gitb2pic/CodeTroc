package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// onUserCreated s'exécute dans la transaction qui crée un utilisateur.
func (s *Service) onUserCreated(context.Context, pgx.Tx, uuid.UUID) error { return nil }
