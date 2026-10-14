package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

var (
	errTransition    = conflict("transition-interdite", "Cette action n'est pas possible dans l'état actuel de l'échange.")
	errOfferUnavail  = unprocessable("offre-indisponible", "Cette offre n'existe pas ou n'est plus publiée.")
	errOwnOffer      = unprocessable("propre-offre", "Tu ne peux pas demander ta propre offre (RG09).")
	errRepoURL       = invalid("depot-invalide", exchange.ErrRepoURL.Error())
	errMilestonePend = conflict("jalon-a-trancher", "Un jalon livré ou contesté doit d'abord être validé ou arbitré.")
	errNeedTooLong   = invalid("donnees-invalides", "besoin : 4000 caractères maximum")
)

// querier est commun au pool et aux transactions.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// MemberSummary résume un membre dans un échange ou une offre.
type MemberSummary = OfferProvider

// ExchangeRequest est une demande d'échange (UC06).
type ExchangeRequest struct {
	OfferID    uuid.UUID
	RepoURL    string
	Need       string
	Milestones []catalog.Milestone // facultatif : jalons de l'offre par défaut
}

// MilestoneView est un jalon tel qu'affiché.
type MilestoneView struct {
	ID          uuid.UUID
	Order       int
	Title       string
	Amount      int64
	Status      exchange.MilestoneStatus
	Deadline    *time.Time
	DeliveredAt *time.Time
	ClosedAt    *time.Time
	Proofs      []ProofView
}

// EscrowSummary résume le séquestre d'un échange.
type EscrowSummary struct {
	Locked   int64
	Released int64
	Refunded int64
}

// ExchangeOffer résume l'offre d'un échange.
type ExchangeOffer struct {
	ID        uuid.UUID
	Title     string
	PackTitle string
	ProofType catalog.ProofType
}

// ExchangeView est un échange tel qu'affiché dans l'espace d'échange.
type ExchangeView struct {
	ID                uuid.UUID
	Status            exchange.Status
	Total             int64
	Days              int
	Repo              exchange.Repo
	Need              string
	CreatedAt         time.Time
	AcceptedAt        *time.Time
	ClosedAt          *time.Time
	CancelRequestedBy *uuid.UUID
	Requester         MemberSummary
	Provider          MemberSummary
	Offer             ExchangeOffer
	Milestones        []MilestoneView
	Escrow            EscrowSummary
	NextActor         exchange.Actor
}

// exRow est l'état interne d'un échange chargé pour une décision.
type exRow struct {
	ID, Requester, Provider uuid.UUID
	Status                  exchange.Status
	Total                   int64
	Days                    int
	Repo                    exchange.Repo
	RepoID                  *int64
	AcceptedAt              *time.Time
	CancelRequestedBy       *uuid.UUID
	ProofType               catalog.ProofType
}

func (r exRow) isParty(id uuid.UUID) bool { return id == r.Requester || id == r.Provider }

func loadExchange(ctx context.Context, q querier, id uuid.UUID, forUpdate bool) (exRow, error) {
	sql := `SELECT e.id_echange, e.id_demandeur, e.id_prestataire, e.statut_echange, e.montant_total, e.delai_jours,
	               e.depot_owner, e.depot_nom, e.github_repo_id, e.date_acceptation, e.annulation_demandee_par, p.type_preuve
	        FROM echange e JOIN offre o USING (id_offre) JOIN pack p USING (id_pack)
	        WHERE e.id_echange = $1`
	if forUpdate {
		sql += ` FOR UPDATE OF e`
	}
	var r exRow
	err := q.QueryRow(ctx, sql, id).Scan(&r.ID, &r.Requester, &r.Provider, &r.Status, &r.Total, &r.Days,
		&r.Repo.Owner, &r.Repo.Name, &r.RepoID, &r.AcceptedAt, &r.CancelRequestedBy, &r.ProofType)
	return r, notFoundIfNoRows(err)
}

// canSee : parties de l'échange, et arbitre d'un litige qui le concerne (F9).
func canSee(ctx context.Context, q querier, a Actor, r exRow) (bool, error) {
	if r.isParty(a.ID) {
		return true, nil
	}
	var arbiter bool
	err := q.QueryRow(ctx, `SELECT `+isArbiterOfExchangeSQL, r.ID, a.ID).Scan(&arbiter)
	return arbiter, err
}

// requireParty charge l'échange et vérifie que l'acteur en est partie (404 sinon).
func requireParty(ctx context.Context, q querier, a Actor, id uuid.UUID, forUpdate bool) (exRow, error) {
	r, err := loadExchange(ctx, q, id, forUpdate)
	if err != nil {
		return r, err
	}
	if !r.isParty(a.ID) {
		return r, ErrNotFound
	}
	return r, nil
}

func recordTransition(ctx context.Context, q querier, entity string, id uuid.UUID, from, to string, by *uuid.UUID, reason string, at time.Time) error {
	var fromPtr *string
	if from != "" {
		fromPtr = &from
	}
	_, err := q.Exec(ctx, `INSERT INTO historique_transition (entite, id_entite, de, vers, par, motif, date) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entity, id, fromPtr, to, by, nilIfEmpty(reason), at)
	return err
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) setExchangeStatus(ctx context.Context, tx pgx.Tx, r *exRow, to exchange.Status, by *uuid.UUID, reason string) error {
	if err := exchange.CheckExchange(r.Status, to); err != nil {
		return errTransition
	}
	closing := to == exchange.Done || to == exchange.Cancelled || to == exchange.Refused
	if _, err := tx.Exec(ctx, `
		UPDATE echange SET statut_echange = $2,
		  date_cloture = CASE WHEN $3 THEN $4::timestamptz ELSE date_cloture END
		WHERE id_echange = $1`, r.ID, to, closing, s.now()); err != nil {
		return err
	}
	if err := recordTransition(ctx, tx, "ECHANGE", r.ID, string(r.Status), string(to), by, reason, s.now()); err != nil {
		return err
	}
	r.Status = to
	if to == exchange.Done || to == exchange.Cancelled {
		return s.onExchangeClosed(ctx, tx, r.ID)
	}
	return nil
}

func (s *Service) setMilestoneStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, from, to exchange.MilestoneStatus, by *uuid.UUID, reason string) error {
	if err := exchange.CheckMilestone(from, to); err != nil {
		return errTransition
	}
	if _, err := tx.Exec(ctx, `
		UPDATE jalon SET statut_jalon = $2,
		  date_livraison = CASE WHEN $3 THEN $4::timestamptz ELSE date_livraison END,
		  date_cloture   = CASE WHEN $5 THEN $4::timestamptz ELSE date_cloture END
		WHERE id_jalon = $1`, id, to, to == exchange.Delivered, s.now(), to.Final()); err != nil {
		return err
	}
	return recordTransition(ctx, tx, "JALON", id, string(from), string(to), by, reason, s.now())
}

// RequestExchange crée une demande d'échange au statut PROPOSE (UC06).
// Aucun crédit n'est bloqué avant l'acceptation.
func (s *Service) RequestExchange(ctx context.Context, a Actor, req ExchangeRequest) (ExchangeView, error) {
	repo, err := exchange.ParseRepoURL(req.RepoURL)
	if err != nil {
		return ExchangeView{}, errRepoURL
	}
	if len([]rune(req.Need)) > 4000 {
		return ExchangeView{}, errNeedTooLong
	}
	offer, err := s.GetOffer(ctx, &a, req.OfferID)
	if errors.Is(err, ErrNotFound) || (err == nil && offer.Status != "PUBLIEE") {
		return ExchangeView{}, errOfferUnavail
	}
	if err != nil {
		return ExchangeView{}, err
	}
	if offer.Provider.ID == a.ID {
		return ExchangeView{}, errOwnOffer
	}
	milestones := req.Milestones
	if milestones == nil {
		milestones = offer.Milestones
	}
	if err := catalog.ValidateMilestones(offer.Price, milestones); err != nil {
		return ExchangeView{}, fromValidation(err)
	}
	bal, err := s.Balance(ctx, a)
	if err != nil {
		return ExchangeView{}, err
	}
	if bal.Available < offer.Price {
		return ExchangeView{}, errInsufficient
	}

	var id uuid.UUID
	err = s.tx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO echange (montant_total, depot_owner, depot_nom, besoin, delai_jours, date_creation, id_demandeur, id_prestataire, id_offre)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id_echange`,
			offer.Price, repo.Owner, repo.Name, req.Need, offer.Days, s.now(), a.ID, offer.Provider.ID, offer.ID).Scan(&id); err != nil {
			return err
		}
		for i, m := range milestones {
			if _, err := tx.Exec(ctx, `INSERT INTO jalon (id_echange, ordre, titre_jalon, montant_jalon) VALUES ($1, $2, $3, $4)`,
				id, i+1, m.Title, m.Amount); err != nil {
				return err
			}
		}
		return recordTransition(ctx, tx, "ECHANGE", id, "", string(exchange.Proposed), &a.ID, "", s.now())
	})
	if err != nil {
		return ExchangeView{}, err
	}
	return s.GetExchange(ctx, a, id)
}

// AcceptExchange bloque le montant total en séquestre et démarre le premier
// jalon (UC07, RG11).
func (s *Service) AcceptExchange(ctx context.Context, a Actor, id uuid.UUID) (ExchangeView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := requireParty(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if a.ID != r.Provider {
			return errAccessDenie
		}
		if err := exchange.CheckExchange(r.Status, exchange.InProgress); err != nil {
			return errTransition
		}
		var escrow, requesterAcc uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO compte (type_compte, id_echange) VALUES ('SEQUESTRE', $1) RETURNING id_compte`, r.ID).Scan(&escrow); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT id_compte FROM compte WHERE id_utilisateur = $1`, r.Requester).Scan(&requesterAcc); err != nil {
			return err
		}
		if _, err := s.transfer(ctx, tx, transferReq{From: requesterAcc, To: escrow, Amount: r.Total, Kind: ledger.Block,
			Key: "blocage:" + r.ID.String(), ExchangeID: &r.ID}); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE echange SET date_acceptation = $2 WHERE id_echange = $1`, r.ID, s.now()); err != nil {
			return err
		}
		if err := s.setExchangeStatus(ctx, tx, &r, exchange.InProgress, &a.ID, ""); err != nil {
			return err
		}
		ms, err := milestoneIDs(ctx, tx, r.ID)
		if err != nil {
			return err
		}
		for i, d := range exchange.Deadlines(s.now(), r.Days, len(ms)) {
			if _, err := tx.Exec(ctx, `UPDATE jalon SET echeance = $2 WHERE id_jalon = $1`, ms[i], d); err != nil {
				return err
			}
		}
		return s.setMilestoneStatus(ctx, tx, ms[0], exchange.Pending, exchange.Active, &a.ID, "")
	})
	if err != nil {
		return ExchangeView{}, err
	}
	return s.GetExchange(ctx, a, id)
}

func milestoneIDs(ctx context.Context, q querier, exchangeID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT id_jalon FROM jalon WHERE id_echange = $1 ORDER BY ordre`, exchangeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// RefuseExchange refuse une demande (UC07 1a). Aucun crédit n'a bougé.
func (s *Service) RefuseExchange(ctx context.Context, a Actor, id uuid.UUID) (ExchangeView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := requireParty(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if a.ID != r.Provider {
			return errAccessDenie
		}
		return s.setExchangeStatus(ctx, tx, &r, exchange.Refused, &a.ID, "")
	})
	if err != nil {
		return ExchangeView{}, err
	}
	return s.GetExchange(ctx, a, id)
}

type msState struct {
	ID       uuid.UUID
	Order    int
	Amount   int64
	Status   exchange.MilestoneStatus
	Deadline *time.Time
}

func loadMilestones(ctx context.Context, q querier, exchangeID uuid.UUID, forUpdate bool) ([]msState, error) {
	sql := `SELECT id_jalon, ordre, montant_jalon, statut_jalon, echeance FROM jalon WHERE id_echange = $1 ORDER BY ordre`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	rows, err := q.Query(ctx, sql, exchangeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[msState])
}

// CancelExchange annule un échange (RG18, D8) :
//   - proposé : par le demandeur ;
//   - en cours, sans jalon livré en attente : avec l'accord des deux parties
//     (la première demande est enregistrée), ou par le demandeur seul quand
//     le jalon en cours a dépassé son échéance et le délai de grâce.
//
// Les jalons non soldés sont remboursés au demandeur.
func (s *Service) CancelExchange(ctx context.Context, a Actor, id uuid.UUID) (ExchangeView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := requireParty(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		switch r.Status {
		case exchange.Proposed:
			if a.ID != r.Requester {
				return errAccessDenie
			}
			return s.setExchangeStatus(ctx, tx, &r, exchange.Cancelled, &a.ID, "retrait du demandeur")
		case exchange.InProgress:
			ms, err := loadMilestones(ctx, tx, r.ID, true)
			if err != nil {
				return err
			}
			abandoned := false
			for _, m := range ms {
				if m.Status == exchange.Delivered || m.Status == exchange.Disputed {
					return errMilestonePend
				}
				if m.Status == exchange.Active && m.Deadline != nil && !s.now().Before(m.Deadline.Add(s.settings.AbandonGrace)) {
					abandoned = true
				}
			}
			switch {
			case abandoned && a.ID == r.Requester:
				return s.cancelInProgress(ctx, tx, &r, ms, a, "abandon : échéance dépassée")
			case r.CancelRequestedBy != nil && *r.CancelRequestedBy != a.ID:
				return s.cancelInProgress(ctx, tx, &r, ms, a, "annulation d'un commun accord")
			case r.CancelRequestedBy == nil:
				_, err := tx.Exec(ctx, `UPDATE echange SET annulation_demandee_par = $2 WHERE id_echange = $1`, r.ID, a.ID)
				return err
			}
			return nil
		default:
			return errTransition
		}
	})
	if err != nil {
		return ExchangeView{}, err
	}
	return s.GetExchange(ctx, a, id)
}

func (s *Service) cancelInProgress(ctx context.Context, tx pgx.Tx, r *exRow, ms []msState, a Actor, reason string) error {
	for _, m := range ms {
		if !m.Status.Final() {
			if err := s.setMilestoneStatus(ctx, tx, m.ID, m.Status, exchange.Refunded, &a.ID, reason); err != nil {
				return err
			}
		}
	}
	if err := s.refundRemaining(ctx, tx, *r, "remboursement-annulation:"+r.ID.String()); err != nil {
		return err
	}
	return s.setExchangeStatus(ctx, tx, r, exchange.Cancelled, &a.ID, reason)
}

// refundRemaining rend au demandeur tout ce qui reste dans le séquestre.
func (s *Service) refundRemaining(ctx context.Context, tx pgx.Tx, r exRow, key string) error {
	var escrow, requesterAcc uuid.UUID
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT id_compte, solde FROM compte WHERE id_echange = $1`, r.ID).Scan(&escrow, &balance); err != nil {
		return err
	}
	if balance == 0 {
		return nil
	}
	if err := tx.QueryRow(ctx, `SELECT id_compte FROM compte WHERE id_utilisateur = $1`, r.Requester).Scan(&requesterAcc); err != nil {
		return err
	}
	_, err := s.transfer(ctx, tx, transferReq{From: escrow, To: requesterAcc, Amount: balance, Kind: ledger.Refund, Key: key, ExchangeID: &r.ID})
	return err
}

// GetExchange renvoie un échange à ses parties (et à l'arbitre d'un de ses litiges).
func (s *Service) GetExchange(ctx context.Context, a Actor, id uuid.UUID) (ExchangeView, error) {
	r, err := loadExchange(ctx, s.db, id, false)
	if err != nil {
		return ExchangeView{}, err
	}
	ok, err := canSee(ctx, s.db, a, r)
	if err != nil {
		return ExchangeView{}, err
	}
	if !ok {
		return ExchangeView{}, ErrNotFound
	}
	return s.exchangeView(ctx, id)
}

const exchangeSelect = `
	SELECT e.id_echange, e.statut_echange, e.montant_total, e.delai_jours, e.depot_owner, e.depot_nom, e.besoin,
	       e.date_creation, e.date_acceptation, e.date_cloture, e.annulation_demandee_par,
	       o.id_offre, o.titre_offre, p.titre_pack, p.type_preuve,
	       d.id_utilisateur, d.pseudo, d.github_login, d.score_reputation::float8,
	       (SELECT count(*) FROM jalon j2 JOIN echange e2 USING (id_echange)
	         WHERE e2.id_prestataire = d.id_utilisateur AND j2.statut_jalon = 'LIBERE')::int,
	       r.id_utilisateur, r.pseudo, r.github_login, r.score_reputation::float8,
	       (SELECT count(*) FROM jalon j2 JOIN echange e2 USING (id_echange)
	         WHERE e2.id_prestataire = r.id_utilisateur AND j2.statut_jalon = 'LIBERE')::int,
	       COALESCE((SELECT solde FROM compte WHERE id_echange = e.id_echange), 0),
	       COALESCE((SELECT SUM(montant) FROM mouvement WHERE id_echange = e.id_echange AND type_mouvement = 'LIBERATION'), 0)::bigint,
	       COALESCE((SELECT SUM(montant) FROM mouvement WHERE id_echange = e.id_echange AND type_mouvement = 'REMBOURSEMENT'), 0)::bigint
	FROM echange e
	JOIN offre o USING (id_offre)
	JOIN pack p USING (id_pack)
	JOIN utilisateur d ON d.id_utilisateur = e.id_demandeur
	JOIN utilisateur r ON r.id_utilisateur = e.id_prestataire`

func scanExchange(row pgx.Row) (ExchangeView, error) {
	var x ExchangeView
	err := row.Scan(&x.ID, &x.Status, &x.Total, &x.Days, &x.Repo.Owner, &x.Repo.Name, &x.Need,
		&x.CreatedAt, &x.AcceptedAt, &x.ClosedAt, &x.CancelRequestedBy,
		&x.Offer.ID, &x.Offer.Title, &x.Offer.PackTitle, &x.Offer.ProofType,
		&x.Requester.ID, &x.Requester.Pseudo, &x.Requester.GitHubLogin, &x.Requester.Reputation, &x.Requester.Deliveries,
		&x.Provider.ID, &x.Provider.Pseudo, &x.Provider.GitHubLogin, &x.Provider.Reputation, &x.Provider.Deliveries,
		&x.Escrow.Locked, &x.Escrow.Released, &x.Escrow.Refunded)
	return x, err
}

func (s *Service) exchangeView(ctx context.Context, id uuid.UUID) (ExchangeView, error) {
	x, err := scanExchange(s.db.QueryRow(ctx, exchangeSelect+` WHERE e.id_echange = $1`, id))
	if err != nil {
		return ExchangeView{}, notFoundIfNoRows(err)
	}
	rows, err := s.db.Query(ctx, `
		SELECT id_jalon, ordre, titre_jalon, montant_jalon, statut_jalon, echeance, date_livraison, date_cloture
		FROM jalon WHERE id_echange = $1 ORDER BY ordre`, id)
	if err != nil {
		return ExchangeView{}, err
	}
	x.Milestones, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (MilestoneView, error) {
		var m MilestoneView
		err := r.Scan(&m.ID, &m.Order, &m.Title, &m.Amount, &m.Status, &m.Deadline, &m.DeliveredAt, &m.ClosedAt)
		return m, err
	})
	if err != nil {
		return ExchangeView{}, err
	}
	if err := s.attachProofs(ctx, &x); err != nil {
		return ExchangeView{}, err
	}
	statuses := make([]exchange.MilestoneStatus, len(x.Milestones))
	for i, m := range x.Milestones {
		statuses[i] = m.Status
	}
	x.NextActor = exchange.NextActor(x.Status, statuses)
	return x, nil
}

// ExchangeFilter restreint la liste des échanges du membre.
type ExchangeFilter struct {
	Role   string // "demandeur", "prestataire" ou "" (les deux)
	Status exchange.Status
}

// ListExchanges renvoie les échanges du membre, du plus récent au plus ancien.
func (s *Service) ListExchanges(ctx context.Context, a Actor, f ExchangeFilter, limit int, after string) (Page[ExchangeView], error) {
	if f.Role != "" && f.Role != "demandeur" && f.Role != "prestataire" {
		return Page[ExchangeView]{}, invalid("parametre-invalide", "role : demandeur ou prestataire")
	}
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[ExchangeView]{}, err
	}
	limit = clampLimit(limit)
	at, cid := cursorArgs(cur)
	rows, err := s.db.Query(ctx, `
		SELECT id_echange, date_creation FROM echange
		WHERE (($2 = '' AND $1 IN (id_demandeur, id_prestataire))
		    OR ($2 = 'demandeur' AND id_demandeur = $1)
		    OR ($2 = 'prestataire' AND id_prestataire = $1))
		  AND ($3 = '' OR statut_echange::text = $3)
		  AND ($4::timestamptz IS NULL OR (date_creation, id_echange) < ($4, $5))
		ORDER BY date_creation DESC, id_echange DESC LIMIT $6`, a.ID, f.Role, string(f.Status), at, cid, limit+1)
	if err != nil {
		return Page[ExchangeView]{}, err
	}
	type idAt struct {
		ID uuid.UUID
		At time.Time
	}
	ids, err := pgx.CollectRows(rows, pgx.RowToStructByPos[idAt])
	if err != nil {
		return Page[ExchangeView]{}, err
	}
	page := paginate(ids, limit, func(x idAt) cursor { return cursor{At: x.At, ID: x.ID} })
	out := Page[ExchangeView]{NextCursor: page.NextCursor, Items: make([]ExchangeView, 0, len(page.Items))}
	for _, x := range page.Items {
		v, err := s.exchangeView(ctx, x.ID)
		if err != nil {
			return Page[ExchangeView]{}, err
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}
