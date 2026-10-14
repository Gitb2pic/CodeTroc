package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

// ValidateMilestone libère les crédits d'un jalon livré au prestataire
// (UC11, RG14). Réservé au demandeur.
func (s *Service) ValidateMilestone(ctx context.Context, a Actor, milestoneID uuid.UUID) (ExchangeView, error) {
	var exID uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, m, err := lockMilestone(ctx, tx, &a, milestoneID)
		if err != nil {
			return err
		}
		exID = r.ID
		if a.ID != r.Requester {
			return errAccessDenie
		}
		if m.Status != exchange.Delivered {
			return errTransition
		}
		return s.release(ctx, tx, &r, m, &a.ID, "validation du demandeur")
	})
	if err != nil {
		return ExchangeView{}, err
	}
	return s.GetExchange(ctx, a, exID)
}

// release transfère le montant du jalon du séquestre au prestataire et
// termine l'échange si tous les jalons sont soldés.
func (s *Service) release(ctx context.Context, tx pgx.Tx, r *exRow, m msRow, by *uuid.UUID, reason string) error {
	escrow, provider, err := escrowAndAccount(ctx, tx, r.ID, r.Provider)
	if err != nil {
		return err
	}
	if _, err := s.transfer(ctx, tx, transferReq{From: escrow, To: provider, Amount: m.Amount, Kind: ledger.Release,
		Key: "liberation:" + m.ID.String(), ExchangeID: &r.ID, MilestoneID: &m.ID}); err != nil {
		return err
	}
	if err := s.setMilestoneStatus(ctx, tx, m.ID, m.Status, exchange.Released, by, reason); err != nil {
		return err
	}
	return s.completeIfSettled(ctx, tx, r, by)
}

func escrowAndAccount(ctx context.Context, tx pgx.Tx, exchangeID, userID uuid.UUID) (escrow, account uuid.UUID, err error) {
	if err = tx.QueryRow(ctx, `SELECT id_compte FROM compte WHERE id_echange = $1`, exchangeID).Scan(&escrow); err != nil {
		return
	}
	err = tx.QueryRow(ctx, `SELECT id_compte FROM compte WHERE id_utilisateur = $1`, userID).Scan(&account)
	return
}

// completeIfSettled passe l'échange à TERMINE quand tous ses jalons sont
// soldés, ou de EN_LITIGE à EN_COURS quand plus aucun jalon n'est contesté.
func (s *Service) completeIfSettled(ctx context.Context, tx pgx.Tx, r *exRow, by *uuid.UUID) error {
	ms, err := loadMilestones(ctx, tx, r.ID, false)
	if err != nil {
		return err
	}
	settled, disputed := true, false
	for _, m := range ms {
		settled = settled && m.Status.Final()
		disputed = disputed || m.Status == exchange.Disputed
	}
	switch {
	case settled:
		return s.setExchangeStatus(ctx, tx, r, exchange.Done, by, "tous les jalons sont soldés")
	case r.Status == exchange.InDispute && !disputed:
		return s.setExchangeStatus(ctx, tx, r, exchange.InProgress, by, "plus aucun jalon contesté")
	}
	return nil
}

// RunTacitValidation libère les jalons livrés depuis plus du délai de
// validation tacite, sans contestation (RG14). Exécuté par le worker.
func (s *Service) RunTacitValidation(ctx context.Context) (int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id_jalon FROM jalon WHERE statut_jalon = 'LIVRE' AND date_livraison <= $1
		ORDER BY date_livraison LIMIT 100`, s.now().Add(-s.settings.TacitValidation))
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, err
	}
	released := 0
	for _, id := range ids {
		err := s.tx(ctx, func(tx pgx.Tx) error {
			r, m, err := lockMilestone(ctx, tx, nil, id)
			if err != nil {
				return err
			}
			if m.Status != exchange.Delivered { // validé ou contesté entre-temps
				return errSkip
			}
			return s.release(ctx, tx, &r, m, nil, "validation tacite")
		})
		switch {
		case errors.Is(err, errSkip):
		case err != nil:
			return released, err
		default:
			released++
		}
	}
	return released, nil
}

var errSkip = errors.New("rien à faire")
