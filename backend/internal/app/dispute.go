package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

var (
	errNotDelivered    = conflict("jalon-non-livre", "Seul un jalon livré et non soldé peut être contesté (RG15).")
	errDisputeClosed   = conflict("litige-clos", "Ce litige est déjà tranché.")
	errAlreadyAnswered = conflict("reponse-deja-donnee", "La réponse à ce litige a déjà été donnée.")
	errBadArbiter      = unprocessable("arbitre-invalide", "L'arbitre doit avoir le rôle ARBITRE ou ADMIN, être actif et ne pas être partie à l'échange (RG16).")
)

// DisputeView est un litige tel qu'affiché.
type DisputeView struct {
	ID            uuid.UUID
	ExchangeID    uuid.UUID
	MilestoneID   uuid.UUID
	MilestoneName string
	Amount        int64
	OpenedBy      uuid.UUID
	Reason        string
	Response      *string
	ArbiterID     *uuid.UUID
	Status        string
	Decision      *string
	ProviderShare *int
	Justification *string
	OpenedAt      time.Time
	ClosedAt      *time.Time
}

// Decision est la décision motivée d'un arbitre (RG17).
type Decision struct {
	Kind          string // LIBERER, REMBOURSER ou PARTAGER
	ProviderShare int    // 1 à 99, seulement pour PARTAGER
	Justification string
}

func checkText(field, v string, min, max int) error {
	n := utf8.RuneCountInString(strings.TrimSpace(v))
	if n < min || utf8.RuneCountInString(v) > max {
		return fieldInvalid(field, "longueur attendue entre "+itoa(min)+" et "+itoa(max)+" caractères")
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// OpenDispute conteste un jalon livré (UC12, RG15) : la libération est gelée
// et un arbitre disponible, non partie à l'échange, est affecté (RG16).
func (s *Service) OpenDispute(ctx context.Context, a Actor, milestoneID uuid.UUID, reason string) (DisputeView, error) {
	if err := checkText("motif", reason, 10, 4000); err != nil {
		return DisputeView{}, err
	}
	var id uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, m, err := lockMilestone(ctx, tx, &a, milestoneID)
		if err != nil {
			return err
		}
		if m.Status != exchange.Delivered {
			return errNotDelivered
		}
		if err := s.setMilestoneStatus(ctx, tx, m.ID, m.Status, exchange.Disputed, &a.ID, "litige ouvert"); err != nil {
			return err
		}
		if r.Status == exchange.InProgress {
			if err := s.setExchangeStatus(ctx, tx, &r, exchange.InDispute, &a.ID, "litige ouvert"); err != nil {
				return err
			}
		}
		var arbiter *uuid.UUID
		err = tx.QueryRow(ctx, `
			SELECT u.id_utilisateur FROM utilisateur u
			WHERE u.role IN ('ARBITRE', 'ADMIN') AND u.statut_utilisateur = 'ACTIF' AND u.id_utilisateur NOT IN ($1, $2)
			ORDER BY (SELECT count(*) FROM litige l WHERE l.id_arbitre = u.id_utilisateur AND l.statut_litige = 'OUVERT'),
			         (u.role = 'ADMIN'), random()
			LIMIT 1`, r.Requester, r.Provider).Scan(&arbiter)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var assignedAt *time.Time
		if arbiter != nil {
			now := s.now()
			assignedAt = &now
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO litige (id_jalon, id_ouvreur, motif, id_arbitre, date_ouverture, date_affectation)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id_litige`,
			m.ID, a.ID, strings.TrimSpace(reason), arbiter, s.now(), assignedAt).Scan(&id); err != nil {
			return err
		}
		return s.notifyDispute(ctx, tx, id, NotifDisputeOpened, &a.ID, true, true)
	})
	if err != nil {
		return DisputeView{}, err
	}
	return s.GetDispute(ctx, a, id)
}

const disputeSelect = `
	SELECT l.id_litige, j.id_echange, l.id_jalon, j.titre_jalon, j.montant_jalon, l.id_ouvreur, l.motif, l.reponse,
	       l.id_arbitre, l.statut_litige, l.decision, l.part_prestataire, l.justification, l.date_ouverture, l.date_cloture
	FROM litige l JOIN jalon j USING (id_jalon) JOIN echange e USING (id_echange)`

func scanDispute(row pgx.Row) (DisputeView, error) {
	var d DisputeView
	err := row.Scan(&d.ID, &d.ExchangeID, &d.MilestoneID, &d.MilestoneName, &d.Amount, &d.OpenedBy, &d.Reason, &d.Response,
		&d.ArbiterID, &d.Status, &d.Decision, &d.ProviderShare, &d.Justification, &d.OpenedAt, &d.ClosedAt)
	return d, err
}

// disputeVisibleSQL : parties, arbitre affecté, ou administrateur pour un litige sans arbitre.
const disputeVisibleSQL = ` AND ($2 IN (e.id_demandeur, e.id_prestataire) OR l.id_arbitre = $2 OR ($3 AND l.id_arbitre IS NULL))`

// GetDispute renvoie un litige à ses parties, à son arbitre, ou à un admin
// s'il attend un arbitre.
func (s *Service) GetDispute(ctx context.Context, a Actor, id uuid.UUID) (DisputeView, error) {
	d, err := scanDispute(s.db.QueryRow(ctx, disputeSelect+` WHERE l.id_litige = $1`+disputeVisibleSQL, id, a.ID, a.Role == RoleAdmin))
	return d, notFoundIfNoRows(err)
}

// ListDisputes renvoie les litiges qui concernent l'acteur (ouverts d'abord),
// et pour un admin la file des litiges sans arbitre.
func (s *Service) ListDisputes(ctx context.Context, a Actor) ([]DisputeView, error) {
	rows, err := s.db.Query(ctx, disputeSelect+` WHERE $1::uuid IS NOT NULL`+disputeVisibleSQL+`
		ORDER BY (l.statut_litige = 'OUVERT') DESC, l.date_ouverture DESC LIMIT 100`, uuid.Nil, a.ID, a.Role == RoleAdmin)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DisputeView, error) { return scanDispute(r) })
}

// RespondToDispute enregistre la réponse de la partie qui n'a pas ouvert le litige.
func (s *Service) RespondToDispute(ctx context.Context, a Actor, id uuid.UUID, text string) (DisputeView, error) {
	if err := checkText("reponse", text, 1, 4000); err != nil {
		return DisputeView{}, err
	}
	d, err := s.GetDispute(ctx, a, id)
	if err != nil {
		return DisputeView{}, err
	}
	r, err := loadExchange(ctx, s.db, d.ExchangeID, false)
	if err != nil {
		return DisputeView{}, err
	}
	switch {
	case !r.isParty(a.ID) || a.ID == d.OpenedBy:
		return DisputeView{}, errAccessDenie
	case d.Status != "OUVERT":
		return DisputeView{}, errDisputeClosed
	}
	tag, err := s.db.Exec(ctx, `UPDATE litige SET reponse = $2 WHERE id_litige = $1 AND reponse IS NULL AND statut_litige = 'OUVERT'`, id, strings.TrimSpace(text))
	if err != nil {
		return DisputeView{}, err
	}
	if tag.RowsAffected() == 0 {
		return DisputeView{}, errAlreadyAnswered
	}
	if err := s.notifyDispute(ctx, s.db, id, NotifDisputeAnswered, &a.ID, false, true); err != nil {
		return DisputeView{}, err
	}
	return s.GetDispute(ctx, a, id)
}

// AssignDispute affecte un arbitre à un litige (administrateur, UC12 3a).
func (s *Service) AssignDispute(ctx context.Context, a Actor, id, arbiterID uuid.UUID) (DisputeView, error) {
	if a.Role != RoleAdmin {
		return DisputeView{}, errAdminOnly
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var exID uuid.UUID
		var status string
		if err := tx.QueryRow(ctx, `SELECT j.id_echange, l.statut_litige FROM litige l JOIN jalon j USING (id_jalon) WHERE l.id_litige = $1 FOR UPDATE OF l`, id).Scan(&exID, &status); err != nil {
			return notFoundIfNoRows(err)
		}
		if status != "OUVERT" {
			return errDisputeClosed
		}
		var ok bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM utilisateur u, echange e
			  WHERE u.id_utilisateur = $1 AND e.id_echange = $2 AND u.role IN ('ARBITRE', 'ADMIN')
			    AND u.statut_utilisateur = 'ACTIF' AND u.id_utilisateur NOT IN (e.id_demandeur, e.id_prestataire))`, arbiterID, exID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return errBadArbiter
		}
		if _, err := tx.Exec(ctx, `UPDATE litige SET id_arbitre = $2, date_affectation = $3 WHERE id_litige = $1`, id, arbiterID, s.now()); err != nil {
			return err
		}
		return s.notifyDispute(ctx, tx, id, NotifDisputeAssigned, &a.ID, false, true)
	})
	if err != nil {
		return DisputeView{}, err
	}
	return s.GetDispute(ctx, a, id)
}

// DecideDispute exécute la décision motivée de l'arbitre affecté (UC13, RG17).
func (s *Service) DecideDispute(ctx context.Context, a Actor, id uuid.UUID, dec Decision) (DisputeView, error) {
	switch {
	case dec.Kind != "LIBERER" && dec.Kind != "REMBOURSER" && dec.Kind != "PARTAGER":
		return DisputeView{}, fieldInvalid("decision", "LIBERER, REMBOURSER ou PARTAGER")
	case dec.Kind == "PARTAGER" && (dec.ProviderShare < 1 || dec.ProviderShare > 99):
		return DisputeView{}, fieldInvalid("part_prestataire", "entre 1 et 99 pour un partage")
	case dec.Kind != "PARTAGER" && dec.ProviderShare != 0:
		return DisputeView{}, fieldInvalid("part_prestataire", "réservée à la décision PARTAGER")
	}
	if err := checkText("justification", dec.Justification, 10, 4000); err != nil {
		return DisputeView{}, err
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var msID uuid.UUID
		var arbiter *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id_jalon, id_arbitre FROM litige WHERE id_litige = $1`, id).Scan(&msID, &arbiter); err != nil {
			return notFoundIfNoRows(err)
		}
		if arbiter == nil || *arbiter != a.ID || !a.CanArbitrate() {
			return ErrNotFound
		}
		r, m, err := lockMilestone(ctx, tx, nil, msID)
		if err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT statut_litige FROM litige WHERE id_litige = $1 FOR UPDATE`, id).Scan(&status); err != nil {
			return err
		}
		if status != "OUVERT" {
			return errDisputeClosed
		}
		toProvider, toRequester := m.Amount, int64(0)
		target := exchange.Released
		switch dec.Kind {
		case "REMBOURSER":
			toProvider, toRequester, target = 0, m.Amount, exchange.Refunded
		case "PARTAGER":
			if toProvider, toRequester, err = ledger.Split(m.Amount, int64(dec.ProviderShare)); err != nil {
				return err
			}
			target = exchange.Split
		}
		escrow, provider, err := escrowAndAccount(ctx, tx, r.ID, r.Provider)
		if err != nil {
			return err
		}
		_, requester, err := escrowAndAccount(ctx, tx, r.ID, r.Requester)
		if err != nil {
			return err
		}
		if toProvider > 0 {
			if _, err := s.transfer(ctx, tx, transferReq{From: escrow, To: provider, Amount: toProvider, Kind: ledger.Release,
				Key: "liberation:" + m.ID.String(), ExchangeID: &r.ID, MilestoneID: &m.ID}); err != nil {
				return err
			}
		}
		if toRequester > 0 {
			if _, err := s.transfer(ctx, tx, transferReq{From: escrow, To: requester, Amount: toRequester, Kind: ledger.Refund,
				Key: "remboursement:" + m.ID.String(), ExchangeID: &r.ID, MilestoneID: &m.ID}); err != nil {
				return err
			}
		}
		var share *int
		if dec.Kind == "PARTAGER" {
			share = &dec.ProviderShare
		}
		if _, err := tx.Exec(ctx, `
			UPDATE litige SET statut_litige = 'CLOS', decision = $2, part_prestataire = $3, justification = $4, date_cloture = $5
			WHERE id_litige = $1`, id, dec.Kind, share, strings.TrimSpace(dec.Justification), s.now()); err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "LITIGE", id, "OUVERT", "CLOS", &a.ID, dec.Kind, s.now()); err != nil {
			return err
		}
		if err := s.notifyDispute(ctx, tx, id, NotifDisputeDecided, &a.ID, true, false); err != nil {
			return err
		}
		if err := s.setMilestoneStatus(ctx, tx, m.ID, m.Status, target, &a.ID, "décision d'arbitrage "+dec.Kind); err != nil {
			return err
		}
		return s.completeIfSettled(ctx, tx, &r, &a.ID)
	})
	if err != nil {
		return DisputeView{}, err
	}
	return s.GetDispute(ctx, a, id)
}
