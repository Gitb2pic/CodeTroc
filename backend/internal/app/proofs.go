package app

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProofView est une preuve de livraison (F7).
type ProofView struct {
	ID        uuid.UUID
	Kind      string
	PRURL     *string
	PRNumber  *int
	CommitSHA *string
	Status    string
	Detail    *string
	Note      *string
	CheckedAt *time.Time
	CreatedAt time.Time
}

// attachProofs ajoute les preuves aux jalons (complété par F7).
func (s *Service) attachProofs(context.Context, *ExchangeView) error { return nil }
