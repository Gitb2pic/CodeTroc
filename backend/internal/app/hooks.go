package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// onUserCreated s'exécute dans la transaction qui crée un utilisateur.
func (s *Service) onUserCreated(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	return s.createUserAccount(ctx, tx, userID)
}
