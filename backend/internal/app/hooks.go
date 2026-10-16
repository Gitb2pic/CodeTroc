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

// onExchangeClosed s'exécute dans la transaction qui termine ou annule un
// échange : les secrets du coffre sont révoqués (RG19).
func (s *Service) onExchangeClosed(ctx context.Context, tx pgx.Tx, exchangeID uuid.UUID) error {
	return s.revokeExchangeSecrets(ctx, tx, exchangeID)
}
