package app

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

var (
	errNotDone      = conflict("echange-non-termine", "Un avis ne se laisse que sur un échange terminé (RG20).")
	errReviewExists = conflict("avis-deja-laisse", "Tu as déjà laissé un avis sur cet échange.")
)

// ---------- F10 : avis et réputation ----------

// Review est un avis reçu par un membre.
type Review struct {
	ID           uuid.UUID
	ExchangeID   uuid.UUID
	AuthorID     uuid.UUID
	AuthorPseudo string
	RecipientID  uuid.UUID
	Rating       int
	Comment      *string
	At           time.Time
}

// reputation est la moyenne des notes reçues, arrondie au centième (RG21).
func reputation(notes []int) float64 {
	if len(notes) == 0 {
		return 0
	}
	sum := 0
	for _, n := range notes {
		sum += n
	}
	return math.Round(float64(sum)/float64(len(notes))*100) / 100
}

// LeaveReview enregistre l'avis d'une partie sur l'autre (UC14, RG20) et
// recalcule la réputation du destinataire (RG21).
func (s *Service) LeaveReview(ctx context.Context, a Actor, exchangeID uuid.UUID, rating int, comment string) (Review, error) {
	if rating < 1 || rating > 5 {
		return Review{}, fieldInvalid("note", "entier de 1 à 5")
	}
	if utf8.RuneCountInString(comment) > 2000 {
		return Review{}, fieldInvalid("commentaire", "2000 caractères maximum")
	}
	var id uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := requireParty(ctx, tx, a, exchangeID, true)
		if err != nil {
			return err
		}
		if r.Status != exchange.Done {
			return errNotDone
		}
		recipient := r.Provider
		if a.ID == r.Provider {
			recipient = r.Requester
		}
		var c *string
		if t := strings.TrimSpace(comment); t != "" {
			c = &t
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO avis (id_echange, id_auteur, id_destinataire, note, commentaire, date_avis)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id_avis`, r.ID, a.ID, recipient, rating, c, s.now()).Scan(&id)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errReviewExists
		}
		if err != nil {
			return err
		}
		return s.refreshReputation(ctx, tx, recipient)
	})
	if err != nil {
		return Review{}, err
	}
	return scanReview(s.db.QueryRow(ctx, reviewSelect+` WHERE a.id_avis = $1`, id))
}

func (s *Service) refreshReputation(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT note::int FROM avis WHERE id_destinataire = $1`, userID)
	if err != nil {
		return err
	}
	notes, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE utilisateur SET score_reputation = $2 WHERE id_utilisateur = $1`, userID, reputation(notes))
	return err
}

const reviewSelect = `
	SELECT a.id_avis, a.id_echange, a.id_auteur, u.pseudo, a.id_destinataire, a.note, a.commentaire, a.date_avis
	FROM avis a JOIN utilisateur u ON u.id_utilisateur = a.id_auteur`

func scanReview(row pgx.Row) (Review, error) {
	var r Review
	err := row.Scan(&r.ID, &r.ExchangeID, &r.AuthorID, &r.AuthorPseudo, &r.RecipientID, &r.Rating, &r.Comment, &r.At)
	return r, err
}

// ListReviews renvoie les avis reçus par un membre (public).
func (s *Service) ListReviews(ctx context.Context, userID uuid.UUID, limit int, after string) (Page[Review], error) {
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[Review]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	rows, err := s.db.Query(ctx, reviewSelect+`
		WHERE a.id_destinataire = $1 AND ($2::timestamptz IS NULL OR (a.date_avis, a.id_avis) < ($2, $3))
		ORDER BY a.date_avis DESC, a.id_avis DESC LIMIT $4`, userID, at, id, limit+1)
	if err != nil {
		return Page[Review]{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Review, error) { return scanReview(r) })
	if err != nil {
		return Page[Review]{}, err
	}
	return paginate(items, limit, func(r Review) cursor { return cursor{At: r.At, ID: r.ID} }), nil
}

// ---------- F11 : messages ----------

// Message est un message d'un échange.
type Message struct {
	ID           uuid.UUID
	AuthorID     uuid.UUID
	AuthorPseudo string
	Text         string
	At           time.Time
}

// PostMessage publie un message ; seules les parties écrivent (RG22).
func (s *Service) PostMessage(ctx context.Context, a Actor, exchangeID uuid.UUID, text string) (Message, error) {
	if err := checkText("contenu", text, 1, 5000); err != nil {
		return Message{}, err
	}
	r, err := loadExchange(ctx, s.db, exchangeID, false)
	if err != nil {
		return Message{}, err
	}
	if !r.isParty(a.ID) {
		visible, err := canSee(ctx, s.db, a, r)
		if err != nil {
			return Message{}, err
		}
		if visible {
			return Message{}, errAccessDenie // arbitre : lecture seule
		}
		return Message{}, ErrNotFound
	}
	var m Message
	err = s.db.QueryRow(ctx, `
		WITH ins AS (INSERT INTO message (id_echange, id_auteur, contenu, date_envoi) VALUES ($1, $2, $3, $4)
		             RETURNING id_message, id_auteur, contenu, date_envoi)
		SELECT ins.id_message, ins.id_auteur, u.pseudo, ins.contenu, ins.date_envoi
		FROM ins JOIN utilisateur u ON u.id_utilisateur = ins.id_auteur`,
		r.ID, a.ID, strings.TrimSpace(text), s.now()).Scan(&m.ID, &m.AuthorID, &m.AuthorPseudo, &m.Text, &m.At)
	return m, err
}

// ListMessages renvoie les messages, du plus récent au plus ancien, aux parties
// et à l'arbitre d'un litige de l'échange (RG22).
func (s *Service) ListMessages(ctx context.Context, a Actor, exchangeID uuid.UUID, limit int, after string) (Page[Message], error) {
	r, err := loadExchange(ctx, s.db, exchangeID, false)
	if err != nil {
		return Page[Message]{}, err
	}
	visible, err := canSee(ctx, s.db, a, r)
	if err != nil {
		return Page[Message]{}, err
	}
	if !visible {
		return Page[Message]{}, ErrNotFound
	}
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[Message]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	rows, err := s.db.Query(ctx, `
		SELECT m.id_message, m.id_auteur, u.pseudo, m.contenu, m.date_envoi
		FROM message m JOIN utilisateur u ON u.id_utilisateur = m.id_auteur
		WHERE m.id_echange = $1 AND ($2::timestamptz IS NULL OR (m.date_envoi, m.id_message) < ($2, $3))
		ORDER BY m.date_envoi DESC, m.id_message DESC LIMIT $4`, exchangeID, at, id, limit+1)
	if err != nil {
		return Page[Message]{}, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Message])
	if err != nil {
		return Page[Message]{}, err
	}
	return paginate(items, limit, func(m Message) cursor { return cursor{At: m.At, ID: m.ID} }), nil
}
