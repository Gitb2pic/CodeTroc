package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

// Types de notifications (F13). Le front compose le texte à partir du type et
// des données ; aucune donnée sensible (secret, e-mail) n'y figure.
const (
	NotifExchangeRequested = "ECHANGE_DEMANDE"
	NotifExchangeAccepted  = "ECHANGE_ACCEPTE"
	NotifExchangeRefused   = "ECHANGE_REFUSE"
	NotifCancelRequested   = "ANNULATION_DEMANDEE"
	NotifExchangeCancelled = "ECHANGE_ANNULE"
	NotifExchangeDone      = "ECHANGE_TERMINE"
	NotifMilestoneDone     = "JALON_LIVRE"
	NotifMilestonePaid     = "JALON_LIBERE"
	NotifProofRejected     = "PREUVE_REJETEE"
	NotifDisputeOpened     = "LITIGE_OUVERT"
	NotifDisputeAssigned   = "LITIGE_AFFECTE"
	NotifDisputeAnswered   = "LITIGE_REPONSE"
	NotifDisputeDecided    = "LITIGE_DECIDE"
	NotifMessage           = "MESSAGE_RECU"
	NotifSecretShared      = "SECRET_PARTAGE"
	NotifSecretRevealed    = "SECRET_REVELE"
	NotifReview            = "AVIS_RECU"
)

// Notification est une notification d'un membre.
type Notification struct {
	ID         uuid.UUID
	Kind       string
	ExchangeID *uuid.UUID
	Data       map[string]any
	Read       bool
	At         time.Time
}

// notify écrit une notification pour chaque destinataire, sauf l'auteur de
// l'action. Le titre du pack de l'échange est ajouté aux données.
func (s *Service) notify(ctx context.Context, q querier, kind string, by *uuid.UUID, exchangeID *uuid.UUID, data map[string]any, to ...uuid.UUID) error {
	if data == nil {
		data = map[string]any{}
	}
	seen := map[uuid.UUID]bool{}
	for _, u := range to {
		if u == uuid.Nil || seen[u] || (by != nil && *by == u) {
			continue
		}
		seen[u] = true
		// UUIDv7 : ordonné dans le temps, il départage les notifications d'une même transaction.
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			INSERT INTO notification (id_notification, id_utilisateur, type, id_echange, donnees, date_creation)
			SELECT $6, $1, $2, $3, $4::jsonb || COALESCE((
			         SELECT jsonb_build_object('pack', p.titre_pack)
			         FROM echange e JOIN offre o USING (id_offre) JOIN pack p USING (id_pack)
			         WHERE e.id_echange = $3), '{}'::jsonb), $5`,
			u, kind, exchangeID, data, s.now(), id); err != nil {
			return err
		}
	}
	return nil
}

// notifyExchange traduit une transition d'échange en notifications.
func (s *Service) notifyExchange(ctx context.Context, tx pgx.Tx, r exRow, from, to exchange.Status, by *uuid.UUID) error {
	switch {
	case to == exchange.InProgress && from == exchange.Proposed:
		return s.notify(ctx, tx, NotifExchangeAccepted, by, &r.ID, nil, r.Requester)
	case to == exchange.Refused:
		return s.notify(ctx, tx, NotifExchangeRefused, by, &r.ID, nil, r.Requester)
	case to == exchange.Cancelled:
		return s.notify(ctx, tx, NotifExchangeCancelled, by, &r.ID, nil, r.Requester, r.Provider)
	case to == exchange.Done:
		return s.notify(ctx, tx, NotifExchangeDone, by, &r.ID, nil, r.Requester, r.Provider)
	}
	return nil
}

// notifyMilestone traduit une transition de jalon en notifications. Les issues
// d'un litige sont annoncées par LITIGE_DECIDE, celles d'une annulation par ECHANGE_ANNULE.
func (s *Service) notifyMilestone(ctx context.Context, tx pgx.Tx, id uuid.UUID, from, to exchange.MilestoneStatus, by *uuid.UUID) error {
	if to != exchange.Delivered && (to != exchange.Released || from == exchange.Disputed) {
		return nil
	}
	var exID, requester, provider uuid.UUID
	var title string
	var amount int64
	if err := tx.QueryRow(ctx, `
		SELECT e.id_echange, e.id_demandeur, e.id_prestataire, j.titre_jalon, j.montant_jalon
		FROM jalon j JOIN echange e USING (id_echange) WHERE j.id_jalon = $1`, id).
		Scan(&exID, &requester, &provider, &title, &amount); err != nil {
		return err
	}
	data := map[string]any{"id_jalon": id, "jalon": title, "montant": amount}
	if to == exchange.Delivered {
		return s.notify(ctx, tx, NotifMilestoneDone, by, &exID, data, requester)
	}
	return s.notify(ctx, tx, NotifMilestonePaid, by, &exID, data, provider)
}

// notifyDispute prévient les parties et/ou l'arbitre d'un litige.
func (s *Service) notifyDispute(ctx context.Context, q querier, id uuid.UUID, kind string, by *uuid.UUID, parties, arbiter bool) error {
	var exID, requester, provider uuid.UUID
	var arb *uuid.UUID
	var title string
	if err := q.QueryRow(ctx, `
		SELECT e.id_echange, e.id_demandeur, e.id_prestataire, l.id_arbitre, j.titre_jalon
		FROM litige l JOIN jalon j USING (id_jalon) JOIN echange e USING (id_echange) WHERE l.id_litige = $1`, id).
		Scan(&exID, &requester, &provider, &arb, &title); err != nil {
		return err
	}
	var to []uuid.UUID
	if parties {
		to = append(to, requester, provider)
	}
	if arbiter && arb != nil {
		to = append(to, *arb)
	}
	return s.notify(ctx, q, kind, by, &exID, map[string]any{"id_litige": id, "jalon": title}, to...)
}

// ListNotifications renvoie les notifications du membre, des plus récentes aux plus anciennes.
func (s *Service) ListNotifications(ctx context.Context, a Actor, unreadOnly bool, limit int, after string) (Page[Notification], error) {
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[Notification]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	rows, err := s.db.Query(ctx, `
		SELECT id_notification, type, id_echange, donnees, lue, date_creation
		FROM notification
		WHERE id_utilisateur = $1 AND (NOT $2 OR NOT lue)
		  AND ($3::timestamptz IS NULL OR (date_creation, id_notification) < ($3, $4))
		ORDER BY date_creation DESC, id_notification DESC LIMIT $5`, a.ID, unreadOnly, at, id, limit+1)
	if err != nil {
		return Page[Notification]{}, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Notification])
	if err != nil {
		return Page[Notification]{}, err
	}
	return paginate(items, limit, func(n Notification) cursor { return cursor{At: n.At, ID: n.ID} }), nil
}

// UnreadCount renvoie le nombre de notifications non lues (pastille de la cloche).
func (s *Service) UnreadCount(ctx context.Context, a Actor) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM notification WHERE id_utilisateur = $1 AND NOT lue`, a.ID).Scan(&n)
	return n, err
}

// MarkNotificationsRead marque comme lues les notifications désignées, ou
// toutes si ids est vide ; seules celles du membre sont touchées.
func (s *Service) MarkNotificationsRead(ctx context.Context, a Actor, ids []uuid.UUID) (int, error) {
	if len(ids) > 100 {
		return 0, fieldInvalid("ids", "100 identifiants maximum")
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE notification SET lue = true
		WHERE id_utilisateur = $1 AND NOT lue AND (COALESCE(cardinality($2::uuid[]), 0) = 0 OR id_notification = ANY($2))`, a.ID, ids)
	return int(tag.RowsAffected()), err
}
