package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

// F17 — annonces : une personne qui a besoin d'aide publie sa demande, des
// prestataires candidatent, l'auteur en retient un, ce qui crée l'échange.

const (
	// AnnouncementTTL est la durée de publication d'une annonce avant expiration.
	AnnouncementTTL = 30 * 24 * time.Hour
	// maxApplicationsPerDay limite les candidatures d'un prestataire (anti-spam).
	maxApplicationsPerDay = 20
)

// Statuts d'une annonce et d'une candidature.
const (
	AnnOpen      = "OUVERTE"
	AnnFilled    = "POURVUE"
	AnnWithdrawn = "RETIREE"
	AnnExpired   = "EXPIREE"

	AppPending   = "EN_ATTENTE"
	AppAccepted  = "ACCEPTEE"
	AppRefused   = "REFUSEE"
	AppWithdrawn = "RETIREE"
)

// Notifications F17.
const (
	NotifApplicationReceived = "CANDIDATURE_RECUE"
	NotifApplicationAccepted = "CANDIDATURE_ACCEPTEE"
	NotifApplicationRefused  = "CANDIDATURE_REFUSEE"
)

var (
	errOwnAnnouncement = unprocessable("propre-annonce", "Tu ne peux pas candidater à ta propre annonce (RG09).")
	errAnnClosed       = conflict("annonce-fermee", "Cette annonce n'accepte plus de candidatures.")
	errAppNotPending   = conflict("candidature-traitee", "Cette candidature a déjà été acceptée, refusée ou retirée.")
	errAppExists       = conflict("candidature-existante", "Tu as déjà une candidature en attente sur cette annonce.")
	errAppLimit        = unprocessable("limite-candidatures", "Trop de candidatures envoyées en 24 h : réessaie plus tard.")
)

// AnnouncementInput décrit une annonce à publier.
type AnnouncementInput struct {
	PackID     uuid.UUID
	Title      string
	Need       string
	RepoURL    string
	Budget     int64
	Days       int
	SprintDays int // 7 par défaut
	Milestones []catalog.Milestone
}

// AnnouncementView est une annonce telle qu'affichée.
type AnnouncementView struct {
	ID           uuid.UUID
	Title        string
	Need         string
	Repo         exchange.Repo
	Budget       int64
	Days         int
	SprintDays   int
	Milestones   []catalog.Milestone
	Status       string
	PublishedAt  time.Time
	ExpiresAt    time.Time
	Pack         OfferPack
	Author       MemberSummary
	Applications int        // candidatures en attente
	MyApp        *uuid.UUID // candidature en attente de l'appelant
	ExchangeID   *uuid.UUID // échange créé (annonce pourvue), visible des parties
}

// ApplicationInput décrit une candidature.
type ApplicationInput struct {
	Price      int64
	Days       int
	Message    string
	Milestones []catalog.Milestone
}

// ApplicationView est une candidature telle qu'affichée.
type ApplicationView struct {
	ID             uuid.UUID
	AnnouncementID uuid.UUID
	Price          int64
	Days           int
	Message        string
	Milestones     []catalog.Milestone
	Status         string
	CreatedAt      time.Time
	DecidedAt      *time.Time
	Provider       MemberSummary
}

func (s *Service) packRules(ctx context.Context, id uuid.UUID) (PackView, error) {
	p, err := s.GetPack(ctx, nil, id)
	if errors.Is(err, ErrNotFound) {
		return p, errPackUnavail
	}
	return p, err
}

// PublishAnnouncement publie une annonce (F17). Le budget doit être couvert
// par le solde disponible ; il n'est bloqué qu'à l'acceptation d'une candidature.
func (s *Service) PublishAnnouncement(ctx context.Context, a Actor, in AnnouncementInput) (AnnouncementView, error) {
	repo, err := exchange.ParseRepoURL(in.RepoURL)
	if err != nil {
		return AnnouncementView{}, errRepoURL
	}
	pack, err := s.packRules(ctx, in.PackID)
	if err != nil {
		return AnnouncementView{}, err
	}
	if in.SprintDays == 0 {
		in.SprintDays = catalog.SprintWeek
	}
	ann := catalog.Announcement{Title: in.Title, Need: in.Need, Budget: in.Budget, Days: in.Days, SprintDays: in.SprintDays, Milestones: in.Milestones}
	if err := ann.Validate(catalog.Pack{MinPrice: pack.MinPrice, MaxPrice: pack.MaxPrice}); err != nil {
		return AnnouncementView{}, fromValidation(err)
	}
	bal, err := s.Balance(ctx, a)
	if err != nil {
		return AnnouncementView{}, err
	}
	if bal.Available < in.Budget {
		return AnnouncementView{}, errInsufficient
	}
	ms, _ := json.Marshal(in.Milestones)
	var id uuid.UUID
	now := s.now()
	if err := s.db.QueryRow(ctx, `
		INSERT INTO annonce (titre_annonce, besoin, depot_owner, depot_nom, budget_credits, delai_jours, jalons_souhaites,
		                     duree_sprint_jours, date_publication, date_expiration, id_auteur, id_pack)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id_annonce`,
		in.Title, in.Need, repo.Owner, repo.Name, in.Budget, in.Days, ms, in.SprintDays, now, now.Add(AnnouncementTTL), a.ID, in.PackID).Scan(&id); err != nil {
		return AnnouncementView{}, err
	}
	return s.GetAnnouncement(ctx, &a, id)
}

// annSelect lit une annonce ; $1 est l'appelant (NULL si anonyme).
const annSelect = `
	SELECT a.id_annonce, a.titre_annonce, a.besoin, a.depot_owner, a.depot_nom, a.budget_credits, a.delai_jours,
	       a.duree_sprint_jours, a.jalons_souhaites, a.statut_annonce, a.date_publication, a.date_expiration,
	       p.id_pack, p.titre_pack, p.type_preuve,
	       COALESCE((SELECT array_agg(c.libelle_competence ORDER BY c.libelle_competence)
	                 FROM requerir r JOIN competence c USING (id_competence) WHERE r.id_pack = p.id_pack), '{}'),
	       u.id_utilisateur, u.pseudo, COALESCE(u.github_login, ''), u.score_reputation::float8,
	       (` + verifiedDeliveriesSQL + `),
	       (SELECT count(*) FROM candidature c WHERE c.id_annonce = a.id_annonce AND c.statut_candidature = 'EN_ATTENTE')::int,
	       (SELECT c.id_candidature FROM candidature c WHERE c.id_annonce = a.id_annonce
	          AND c.id_prestataire = $1::uuid AND c.statut_candidature = 'EN_ATTENTE'),
	       (SELECT e.id_echange FROM echange e WHERE e.id_annonce = a.id_annonce
	          AND $1::uuid IN (e.id_demandeur, e.id_prestataire))
	FROM annonce a
	JOIN pack p USING (id_pack)
	JOIN utilisateur u ON u.id_utilisateur = a.id_auteur`

func scanAnnouncement(row pgx.Row) (AnnouncementView, error) {
	var v AnnouncementView
	var ms []byte
	err := row.Scan(&v.ID, &v.Title, &v.Need, &v.Repo.Owner, &v.Repo.Name, &v.Budget, &v.Days,
		&v.SprintDays, &ms, &v.Status, &v.PublishedAt, &v.ExpiresAt,
		&v.Pack.ID, &v.Pack.Title, &v.Pack.ProofType, &v.Pack.Skills,
		&v.Author.ID, &v.Author.Pseudo, &v.Author.GitHubLogin, &v.Author.Reputation, &v.Author.Deliveries,
		&v.Applications, &v.MyApp, &v.ExchangeID)
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal(ms, &v.Milestones)
}

func actorID(a *Actor) *uuid.UUID {
	if a == nil {
		return nil
	}
	return &a.ID
}

// GetAnnouncement renvoie une annonce. Une annonce fermée n'est visible que
// de son auteur et des prestataires qui y ont candidaté.
func (s *Service) GetAnnouncement(ctx context.Context, a *Actor, id uuid.UUID) (AnnouncementView, error) {
	v, err := scanAnnouncement(s.db.QueryRow(ctx, annSelect+` WHERE a.id_annonce = $2`, actorID(a), id))
	if err != nil {
		return v, notFoundIfNoRows(err)
	}
	if v.Status == AnnOpen || (a != nil && a.ID == v.Author.ID) {
		return v, nil
	}
	if a != nil {
		var applied bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM candidature WHERE id_annonce = $1 AND id_prestataire = $2)`, id, a.ID).Scan(&applied); err != nil {
			return v, err
		}
		if applied {
			return v, nil
		}
	}
	return AnnouncementView{}, ErrNotFound
}

// AnnouncementFilter restreint la liste des annonces.
type AnnouncementFilter struct {
	PackID *uuid.UUID
	Q      string
	Mine   bool // annonces de l'appelant, tous statuts
}

// ListAnnouncements renvoie les annonces ouvertes (ou celles de l'appelant).
func (s *Service) ListAnnouncements(ctx context.Context, a *Actor, f AnnouncementFilter, limit int, after string) (Page[AnnouncementView], error) {
	if f.Mine && a == nil {
		return Page[AnnouncementView]{}, ErrUnauthenticated
	}
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[AnnouncementView]{}, err
	}
	limit = clampLimit(limit)
	at, cid := cursorArgs(cur)
	rows, err := s.db.Query(ctx, annSelect+`
		WHERE (($2 AND a.id_auteur = $1::uuid)
		    OR (NOT $2 AND a.statut_annonce = 'OUVERTE' AND p.actif AND u.statut_utilisateur = 'ACTIF'))
		  AND ($3::uuid IS NULL OR a.id_pack = $3)
		  AND ($4 = '' OR position(lower($4) IN lower(a.titre_annonce || ' ' || a.besoin || ' ' || p.titre_pack)) > 0)
		  AND ($5::timestamptz IS NULL OR (a.date_publication, a.id_annonce) < ($5, $6))
		ORDER BY a.date_publication DESC, a.id_annonce DESC
		LIMIT $7`, actorID(a), f.Mine, f.PackID, f.Q, at, cid, limit+1)
	if err != nil {
		return Page[AnnouncementView]{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (AnnouncementView, error) { return scanAnnouncement(r) })
	if err != nil {
		return Page[AnnouncementView]{}, err
	}
	return paginate(items, limit, func(v AnnouncementView) cursor { return cursor{At: v.PublishedAt, ID: v.ID} }), nil
}

type annRow struct {
	ID, Author, PackID uuid.UUID
	Title, Status      string
	Need               string
	Repo               exchange.Repo
}

func loadAnnouncement(ctx context.Context, q querier, id uuid.UUID, forUpdate bool) (annRow, error) {
	sql := `SELECT id_annonce, id_auteur, id_pack, titre_annonce, statut_annonce, besoin, depot_owner, depot_nom FROM annonce WHERE id_annonce = $1`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	var r annRow
	err := q.QueryRow(ctx, sql, id).Scan(&r.ID, &r.Author, &r.PackID, &r.Title, &r.Status, &r.Need, &r.Repo.Owner, &r.Repo.Name)
	return r, notFoundIfNoRows(err)
}

// closePending refuse les candidatures en attente d'une annonce et prévient leurs auteurs.
func (s *Service) closePending(ctx context.Context, tx pgx.Tx, ann annRow, by *uuid.UUID) error {
	rows, err := tx.Query(ctx, `
		UPDATE candidature SET statut_candidature = 'REFUSEE', date_decision = $2
		WHERE id_annonce = $1 AND statut_candidature = 'EN_ATTENTE' RETURNING id_candidature, id_prestataire`, ann.ID, s.now())
	if err != nil {
		return err
	}
	type pair struct{ ID, Provider uuid.UUID }
	refused, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pair])
	if err != nil {
		return err
	}
	for _, p := range refused {
		if err := recordTransition(ctx, tx, "CANDIDATURE", p.ID, AppPending, AppRefused, by, "annonce fermée", s.now()); err != nil {
			return err
		}
		if err := s.notify(ctx, tx, NotifApplicationRefused, by, nil,
			map[string]any{"id_annonce": ann.ID, "annonce": ann.Title, "id_candidature": p.ID}, p.Provider); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) setAnnouncementStatus(ctx context.Context, tx pgx.Tx, ann annRow, to string, by *uuid.UUID, reason string) error {
	if _, err := tx.Exec(ctx, `UPDATE annonce SET statut_annonce = $2 WHERE id_annonce = $1`, ann.ID, to); err != nil {
		return err
	}
	return recordTransition(ctx, tx, "ANNONCE", ann.ID, ann.Status, to, by, reason, s.now())
}

// WithdrawAnnouncement retire une annonce ouverte de son auteur ; les
// candidatures en attente sont refusées.
func (s *Service) WithdrawAnnouncement(ctx context.Context, a Actor, id uuid.UUID) (AnnouncementView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		ann, err := loadAnnouncement(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if ann.Author != a.ID {
			if ann.Status != AnnOpen {
				return ErrNotFound
			}
			return errAccessDenie
		}
		if ann.Status != AnnOpen {
			return errAnnClosed
		}
		if err := s.closePending(ctx, tx, ann, &a.ID); err != nil {
			return err
		}
		return s.setAnnouncementStatus(ctx, tx, ann, AnnWithdrawn, &a.ID, "retrait de l'auteur")
	})
	if err != nil {
		return AnnouncementView{}, err
	}
	return s.GetAnnouncement(ctx, &a, id)
}

// Apply dépose la candidature d'un prestataire sur une annonce ouverte.
func (s *Service) Apply(ctx context.Context, a Actor, annID uuid.UUID, in ApplicationInput) (ApplicationView, error) {
	if err := s.requireGitHub(ctx, a); err != nil {
		return ApplicationView{}, err
	}
	ann, err := loadAnnouncement(ctx, s.db, annID, false)
	if err != nil {
		return ApplicationView{}, err
	}
	if ann.Status != AnnOpen {
		return ApplicationView{}, ErrNotFound
	}
	if ann.Author == a.ID {
		return ApplicationView{}, errOwnAnnouncement
	}
	pack, err := s.packRules(ctx, ann.PackID)
	if err != nil {
		return ApplicationView{}, err
	}
	app := catalog.Application{Price: in.Price, Days: in.Days, Message: in.Message, Milestones: in.Milestones}
	if err := app.Validate(catalog.Pack{MinPrice: pack.MinPrice, MaxPrice: pack.MaxPrice}); err != nil {
		return ApplicationView{}, fromValidation(err)
	}
	var id uuid.UUID
	err = s.tx(ctx, func(tx pgx.Tx) error {
		// Verrou partagé : une acceptation concurrente attend la fin de cette candidature.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM annonce WHERE id_annonce = $1 FOR SHARE`, annID); err != nil {
			return err
		}
		cur, err := loadAnnouncement(ctx, tx, annID, false)
		if err != nil {
			return err
		}
		if cur.Status != AnnOpen {
			return errAnnClosed
		}
		var recent int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM candidature WHERE id_prestataire = $1 AND date_candidature > $2`,
			a.ID, s.now().Add(-24*time.Hour)).Scan(&recent); err != nil {
			return err
		}
		if recent >= maxApplicationsPerDay {
			return errAppLimit
		}
		ms, _ := json.Marshal(in.Milestones)
		err = tx.QueryRow(ctx, `
			INSERT INTO candidature (id_annonce, id_prestataire, prix_credits, delai_jours, jalons_proposes, message, date_candidature)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id_candidature`,
			annID, a.ID, in.Price, in.Days, ms, in.Message, s.now()).Scan(&id)
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errAppExists
		}
		if err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "CANDIDATURE", id, "", AppPending, &a.ID, "", s.now()); err != nil {
			return err
		}
		return s.notify(ctx, tx, NotifApplicationReceived, &a.ID, nil,
			map[string]any{"id_annonce": annID, "annonce": cur.Title, "id_candidature": id, "prix": in.Price}, cur.Author)
	})
	if err != nil {
		return ApplicationView{}, err
	}
	return s.getApplication(ctx, s.db, id)
}

const appSelect = `
	SELECT c.id_candidature, c.id_annonce, c.prix_credits, c.delai_jours, c.message, c.jalons_proposes,
	       c.statut_candidature, c.date_candidature, c.date_decision,
	       u.id_utilisateur, u.pseudo, COALESCE(u.github_login, ''), u.score_reputation::float8,
	       (` + verifiedDeliveriesSQL + `)
	FROM candidature c
	JOIN utilisateur u ON u.id_utilisateur = c.id_prestataire`

func scanApplication(row pgx.Row) (ApplicationView, error) {
	var v ApplicationView
	var ms []byte
	err := row.Scan(&v.ID, &v.AnnouncementID, &v.Price, &v.Days, &v.Message, &ms, &v.Status, &v.CreatedAt, &v.DecidedAt,
		&v.Provider.ID, &v.Provider.Pseudo, &v.Provider.GitHubLogin, &v.Provider.Reputation, &v.Provider.Deliveries)
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal(ms, &v.Milestones)
}

func (s *Service) getApplication(ctx context.Context, q querier, id uuid.UUID) (ApplicationView, error) {
	v, err := scanApplication(q.QueryRow(ctx, appSelect+` WHERE c.id_candidature = $1`, id))
	return v, notFoundIfNoRows(err)
}

// ListApplications renvoie les candidatures d'une annonce : toutes à son
// auteur, la sienne seulement à un prestataire.
func (s *Service) ListApplications(ctx context.Context, a Actor, annID uuid.UUID) ([]ApplicationView, error) {
	ann, err := s.GetAnnouncement(ctx, &a, annID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, appSelect+`
		WHERE c.id_annonce = $1 AND ($2 OR c.id_prestataire = $3)
		ORDER BY c.date_candidature, c.id_candidature`, annID, ann.Author.ID == a.ID, a.ID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ApplicationView, error) { return scanApplication(r) })
}

type appRow struct {
	ID, AnnouncementID, Provider uuid.UUID
	Price                        int64
	Days                         int
	Milestones                   []catalog.Milestone
	Status                       string
}

func loadApplication(ctx context.Context, tx pgx.Tx, annID, id uuid.UUID) (appRow, error) {
	var r appRow
	var ms []byte
	err := tx.QueryRow(ctx, `
		SELECT id_candidature, id_annonce, id_prestataire, prix_credits, delai_jours, jalons_proposes, statut_candidature
		FROM candidature WHERE id_candidature = $1 AND id_annonce = $2 FOR UPDATE`, id, annID).
		Scan(&r.ID, &r.AnnouncementID, &r.Provider, &r.Price, &r.Days, &ms, &r.Status)
	if err != nil {
		return r, notFoundIfNoRows(err)
	}
	return r, json.Unmarshal(ms, &r.Milestones)
}

// decideApplication charge l'annonce (verrouillée) et la candidature pour une
// décision de son auteur ; un tiers reçoit 404 (D11).
func (s *Service) decideApplication(ctx context.Context, tx pgx.Tx, a Actor, annID, appID uuid.UUID) (annRow, appRow, error) {
	ann, err := loadAnnouncement(ctx, tx, annID, true)
	if err != nil {
		return ann, appRow{}, err
	}
	cand, err := loadApplication(ctx, tx, annID, appID)
	if err != nil {
		return ann, cand, err
	}
	if ann.Author != a.ID {
		if cand.Provider == a.ID {
			return ann, cand, errAccessDenie
		}
		return ann, cand, ErrNotFound
	}
	if ann.Status != AnnOpen {
		return ann, cand, errAnnClosed
	}
	if cand.Status != AppPending {
		return ann, cand, errAppNotPending
	}
	return ann, cand, nil
}

// AcceptApplication retient une candidature : dans une seule transaction,
// l'échange est créé directement EN_COURS (séquestre bloqué), les autres
// candidatures sont refusées et l'annonce est pourvue. Si le solde ne couvre
// plus le prix, rien ne change (422) et l'annonce reste ouverte.
func (s *Service) AcceptApplication(ctx context.Context, a Actor, annID, appID uuid.UUID) (ExchangeView, error) {
	var exID uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		ann, cand, err := s.decideApplication(ctx, tx, a, annID, appID)
		if err != nil {
			return err
		}
		var sprintDays int
		if err := tx.QueryRow(ctx, `SELECT duree_sprint_jours FROM annonce WHERE id_annonce = $1`, ann.ID).Scan(&sprintDays); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO echange (montant_total, depot_owner, depot_nom, besoin, delai_jours, date_creation,
			                     id_demandeur, id_prestataire, id_annonce, id_pack, duree_sprint_jours)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id_echange`,
			cand.Price, ann.Repo.Owner, ann.Repo.Name, ann.Need, cand.Days, s.now(),
			ann.Author, cand.Provider, ann.ID, ann.PackID, sprintDays).Scan(&exID); err != nil {
			return err
		}
		for i, m := range cand.Milestones {
			if _, err := tx.Exec(ctx, `INSERT INTO jalon (id_echange, ordre, titre_jalon, montant_jalon) VALUES ($1, $2, $3, $4)`,
				exID, i+1, m.Title, m.Amount); err != nil {
				return err
			}
		}
		if err := recordTransition(ctx, tx, "ECHANGE", exID, "", string(exchange.Proposed), &a.ID, "candidature acceptée", s.now()); err != nil {
			return err
		}
		r, err := loadExchange(ctx, tx, exID, true)
		if err != nil {
			return err
		}
		if err := s.startExchange(ctx, tx, &r, a.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE candidature SET statut_candidature = 'ACCEPTEE', date_decision = $2 WHERE id_candidature = $1`, cand.ID, s.now()); err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "CANDIDATURE", cand.ID, AppPending, AppAccepted, &a.ID, "", s.now()); err != nil {
			return err
		}
		if err := s.notify(ctx, tx, NotifApplicationAccepted, &a.ID, &exID,
			map[string]any{"id_annonce": ann.ID, "annonce": ann.Title, "id_candidature": cand.ID}, cand.Provider); err != nil {
			return err
		}
		if err := s.closePending(ctx, tx, ann, &a.ID); err != nil {
			return err
		}
		return s.setAnnouncementStatus(ctx, tx, ann, AnnFilled, &a.ID, "")
	})
	if err != nil {
		return ExchangeView{}, err
	}
	return s.GetExchange(ctx, a, exID)
}

// RefuseApplication refuse une candidature en attente (auteur de l'annonce).
func (s *Service) RefuseApplication(ctx context.Context, a Actor, annID, appID uuid.UUID) (ApplicationView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		ann, cand, err := s.decideApplication(ctx, tx, a, annID, appID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE candidature SET statut_candidature = 'REFUSEE', date_decision = $2 WHERE id_candidature = $1`, cand.ID, s.now()); err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "CANDIDATURE", cand.ID, AppPending, AppRefused, &a.ID, "", s.now()); err != nil {
			return err
		}
		return s.notify(ctx, tx, NotifApplicationRefused, &a.ID, nil,
			map[string]any{"id_annonce": ann.ID, "annonce": ann.Title, "id_candidature": cand.ID}, cand.Provider)
	})
	if err != nil {
		return ApplicationView{}, err
	}
	return s.getApplication(ctx, s.db, appID)
}

// WithdrawApplication retire la candidature en attente de son auteur.
func (s *Service) WithdrawApplication(ctx context.Context, a Actor, annID, appID uuid.UUID) (ApplicationView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		cand, err := loadApplication(ctx, tx, annID, appID)
		if err != nil {
			return err
		}
		if cand.Provider != a.ID {
			return ErrNotFound
		}
		if cand.Status != AppPending {
			return errAppNotPending
		}
		if _, err := tx.Exec(ctx, `UPDATE candidature SET statut_candidature = 'RETIREE', date_decision = $2 WHERE id_candidature = $1`, cand.ID, s.now()); err != nil {
			return err
		}
		return recordTransition(ctx, tx, "CANDIDATURE", cand.ID, AppPending, AppWithdrawn, &a.ID, "", s.now())
	})
	if err != nil {
		return ApplicationView{}, err
	}
	return s.getApplication(ctx, s.db, appID)
}

// ExpireAnnouncements ferme les annonces ouvertes arrivées à expiration
// (planificateur) ; leurs candidatures en attente sont refusées.
func (s *Service) ExpireAnnouncements(ctx context.Context) (int, error) {
	rows, err := s.db.Query(ctx, `SELECT id_annonce FROM annonce WHERE statut_annonce = 'OUVERTE' AND date_expiration <= $1`, s.now())
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		expired := false
		err := s.tx(ctx, func(tx pgx.Tx) error {
			ann, err := loadAnnouncement(ctx, tx, id, true)
			if err != nil || ann.Status != AnnOpen {
				return err
			}
			if err := s.closePending(ctx, tx, ann, nil); err != nil {
				return err
			}
			expired = true
			return s.setAnnouncementStatus(ctx, tx, ann, AnnExpired, nil, "expiration")
		})
		if err != nil {
			return n, err
		}
		if expired {
			n++
		}
	}
	return n, nil
}
