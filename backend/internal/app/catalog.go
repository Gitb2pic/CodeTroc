package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
)

var (
	errAdminOnly   = forbidden("reserve-admin", "Action réservée aux administrateurs.")
	errAccessDenie = forbidden("acces-refuse", "Tu n'as pas le droit d'effectuer cette action sur cette ressource.")
	errPackUnavail = unprocessable("pack-indisponible", "Ce pack n'existe pas ou n'est plus proposé au catalogue.")
)

// ---------- Packs (F4) ----------

// PackInput décrit un pack à créer.
type PackInput struct {
	Title, Description, Deliverable string
	ProofType                       catalog.ProofType
	Criteria                        []catalog.Criterion
	MinPrice, MaxPrice              int64
	SkillIDs                        []uuid.UUID
}

// PackPatch modifie un pack ; un champ nil reste inchangé.
type PackPatch struct {
	Title, Description, Deliverable *string
	ProofType                       *catalog.ProofType
	Criteria                        []catalog.Criterion
	MinPrice, MaxPrice              *int64
	SkillIDs                        []uuid.UUID
	Active                          *bool
}

// PackView est un pack tel qu'affiché.
type PackView struct {
	ID          uuid.UUID
	Title       string
	Description string
	Deliverable string
	ProofType   catalog.ProofType
	Criteria    []catalog.Criterion
	MinPrice    int64
	MaxPrice    int64
	Active      bool
	Skills      []Skill
	CreatedAt   time.Time
}

func (in PackInput) domain() catalog.Pack {
	return catalog.Pack{Title: in.Title, Description: in.Description, Deliverable: in.Deliverable, ProofType: in.ProofType,
		Criteria: in.Criteria, MinPrice: in.MinPrice, MaxPrice: in.MaxPrice, SkillIDs: len(in.SkillIDs)}
}

// CreatePack ajoute un pack au catalogue (UC16, admin).
func (s *Service) CreatePack(ctx context.Context, a Actor, in PackInput) (PackView, error) {
	if a.Role != RoleAdmin {
		return PackView{}, errAdminOnly
	}
	if err := in.domain().Validate(); err != nil {
		return PackView{}, fromValidation(err)
	}
	var id uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		crit, _ := json.Marshal(in.Criteria)
		if err := tx.QueryRow(ctx, `
			INSERT INTO pack (titre_pack, description_pack, livrable, criteres_acceptation, type_preuve, prix_min, prix_max, date_creation)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id_pack`,
			in.Title, in.Description, in.Deliverable, crit, in.ProofType, in.MinPrice, in.MaxPrice, s.now()).Scan(&id); err != nil {
			return err
		}
		return setPackSkills(ctx, tx, id, in.SkillIDs)
	})
	if err != nil {
		return PackView{}, err
	}
	return s.GetPack(ctx, &a, id)
}

func setPackSkills(ctx context.Context, tx pgx.Tx, packID uuid.UUID, skills []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM requerir WHERE id_pack = $1`, packID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO requerir (id_pack, id_competence)
		SELECT $1, id_competence FROM competence WHERE id_competence = ANY($2)`, packID, skills)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(uniqueIDs(skills)) {
		return errUnknownSk
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) map[uuid.UUID]bool {
	m := map[uuid.UUID]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// UpdatePack modifie un pack (UC16, admin). La validation porte sur le pack complet.
func (s *Service) UpdatePack(ctx context.Context, a Actor, id uuid.UUID, p PackPatch) (PackView, error) {
	if a.Role != RoleAdmin {
		return PackView{}, errAdminOnly
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		cur, err := scanPack(tx.QueryRow(ctx, packSelect+` WHERE p.id_pack = $1 FOR UPDATE OF p`, id))
		if err != nil {
			return notFoundIfNoRows(err)
		}
		in := PackInput{Title: cur.Title, Description: cur.Description, Deliverable: cur.Deliverable, ProofType: cur.ProofType,
			Criteria: cur.Criteria, MinPrice: cur.MinPrice, MaxPrice: cur.MaxPrice}
		for _, sk := range cur.Skills {
			in.SkillIDs = append(in.SkillIDs, sk.ID)
		}
		assign(&in.Title, p.Title)
		assign(&in.Description, p.Description)
		assign(&in.Deliverable, p.Deliverable)
		assign(&in.ProofType, p.ProofType)
		assign(&in.MinPrice, p.MinPrice)
		assign(&in.MaxPrice, p.MaxPrice)
		if p.Criteria != nil {
			in.Criteria = p.Criteria
		}
		if p.SkillIDs != nil {
			in.SkillIDs = p.SkillIDs
		}
		if err := in.domain().Validate(); err != nil {
			return fromValidation(err)
		}
		active := cur.Active
		assign(&active, p.Active)
		crit, _ := json.Marshal(in.Criteria)
		if _, err := tx.Exec(ctx, `
			UPDATE pack SET titre_pack = $2, description_pack = $3, livrable = $4, criteres_acceptation = $5,
			  type_preuve = $6, prix_min = $7, prix_max = $8, actif = $9
			WHERE id_pack = $1`, id, in.Title, in.Description, in.Deliverable, crit, in.ProofType, in.MinPrice, in.MaxPrice, active); err != nil {
			return err
		}
		if p.SkillIDs != nil {
			return setPackSkills(ctx, tx, id, in.SkillIDs)
		}
		return nil
	})
	if err != nil {
		return PackView{}, err
	}
	return s.GetPack(ctx, &a, id)
}

func assign[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

const packSelect = `
	SELECT p.id_pack, p.titre_pack, p.description_pack, p.livrable, p.criteres_acceptation, p.type_preuve,
	       p.prix_min, p.prix_max, p.actif, p.date_creation,
	       COALESCE((SELECT json_agg(json_build_object('id', c.id_competence, 'label', c.libelle_competence, 'category', c.categorie)
	                         ORDER BY c.libelle_competence)
	                 FROM requerir r JOIN competence c USING (id_competence) WHERE r.id_pack = p.id_pack), '[]')
	FROM pack p`

func scanPack(row pgx.Row) (PackView, error) {
	var p PackView
	var crit, skills []byte
	if err := row.Scan(&p.ID, &p.Title, &p.Description, &p.Deliverable, &crit, &p.ProofType, &p.MinPrice, &p.MaxPrice, &p.Active, &p.CreatedAt, &skills); err != nil {
		return p, err
	}
	if err := json.Unmarshal(crit, &p.Criteria); err != nil {
		return p, err
	}
	var sk []struct {
		ID       uuid.UUID `json:"id"`
		Label    string    `json:"label"`
		Category string    `json:"category"`
	}
	if err := json.Unmarshal(skills, &sk); err != nil {
		return p, err
	}
	p.Skills = make([]Skill, len(sk))
	for i, x := range sk {
		p.Skills[i] = Skill(x)
	}
	return p, nil
}

// GetPack renvoie un pack ; un pack inactif n'est visible que des administrateurs.
func (s *Service) GetPack(ctx context.Context, a *Actor, id uuid.UUID) (PackView, error) {
	p, err := scanPack(s.db.QueryRow(ctx, packSelect+` WHERE p.id_pack = $1`, id))
	if err != nil {
		return PackView{}, notFoundIfNoRows(err)
	}
	if !p.Active && (a == nil || a.Role != RoleAdmin) {
		return PackView{}, ErrNotFound
	}
	return p, nil
}

// PackFilter restreint la liste des packs.
type PackFilter struct {
	SkillID *uuid.UUID
	Q       string
}

// ListPacks renvoie les packs actifs du catalogue (UC03).
func (s *Service) ListPacks(ctx context.Context, f PackFilter, limit int, after string) (Page[PackView], error) {
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[PackView]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	rows, err := s.db.Query(ctx, packSelect+`
		WHERE p.actif
		  AND ($1::uuid IS NULL OR EXISTS (SELECT 1 FROM requerir r WHERE r.id_pack = p.id_pack AND r.id_competence = $1))
		  AND ($2 = '' OR position(lower($2) IN lower(p.titre_pack || ' ' || p.description_pack)) > 0)
		  AND ($3::timestamptz IS NULL OR (p.date_creation, p.id_pack) < ($3, $4))
		ORDER BY p.date_creation DESC, p.id_pack DESC
		LIMIT $5`, f.SkillID, f.Q, at, id, limit+1)
	if err != nil {
		return Page[PackView]{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (PackView, error) { return scanPack(r) })
	if err != nil {
		return Page[PackView]{}, err
	}
	return paginate(items, limit, func(p PackView) cursor { return cursor{At: p.CreatedAt, ID: p.ID} }), nil
}

// ---------- Offres (F5) ----------

// OfferInput décrit une offre à publier.
type OfferInput struct {
	PackID      uuid.UUID
	Title       string
	Description string
	Price       int64
	Days        int
	Milestones  []catalog.Milestone
}

// OfferPack résume le pack d'une offre.
type OfferPack struct {
	ID        uuid.UUID
	Title     string
	ProofType catalog.ProofType
	Skills    []string
}

// OfferProvider résume le prestataire d'une offre.
type OfferProvider struct {
	ID          uuid.UUID
	Pseudo      string
	GitHubLogin string
	Reputation  float64
	Deliveries  int
}

// OfferView est une offre telle qu'affichée dans le catalogue.
type OfferView struct {
	ID          uuid.UUID
	Title       string
	Description string
	Price       int64
	Days        int
	Milestones  []catalog.Milestone
	Status      string
	PublishedAt time.Time
	Pack        OfferPack
	Provider    OfferProvider
}

// CreateOffer publie une offre sur un pack actif (UC04, RG08).
func (s *Service) CreateOffer(ctx context.Context, a Actor, in OfferInput) (OfferView, error) {
	pack, err := s.GetPack(ctx, nil, in.PackID)
	if errors.Is(err, ErrNotFound) {
		return OfferView{}, errPackUnavail
	}
	if err != nil {
		return OfferView{}, err
	}
	o := catalog.Offer{Title: in.Title, Description: in.Description, Price: in.Price, Days: in.Days, Milestones: in.Milestones}
	if err := o.Validate(catalog.Pack{MinPrice: pack.MinPrice, MaxPrice: pack.MaxPrice}); err != nil {
		return OfferView{}, fromValidation(err)
	}
	ms, _ := json.Marshal(in.Milestones)
	var id uuid.UUID
	if err := s.db.QueryRow(ctx, `
		INSERT INTO offre (titre_offre, description, prix_credits, delai_jours, jalons_proposes, date_publication, id_prestataire, id_pack)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id_offre`,
		in.Title, in.Description, in.Price, in.Days, ms, s.now(), a.ID, in.PackID).Scan(&id); err != nil {
		return OfferView{}, err
	}
	return s.GetOffer(ctx, &a, id)
}

// WithdrawOffer retire une offre de son auteur. Sans effet si déjà retirée.
func (s *Service) WithdrawOffer(ctx context.Context, a Actor, id uuid.UUID) error {
	var author uuid.UUID
	var status string
	err := s.db.QueryRow(ctx, `SELECT id_prestataire, statut_offre FROM offre WHERE id_offre = $1`, id).Scan(&author, &status)
	if err != nil {
		return notFoundIfNoRows(err)
	}
	if author != a.ID {
		if status != "PUBLIEE" {
			return ErrNotFound // une offre retirée n'est visible que de son auteur
		}
		return errAccessDenie
	}
	_, err = s.db.Exec(ctx, `UPDATE offre SET statut_offre = 'RETIREE' WHERE id_offre = $1`, id)
	return err
}

const offerSelect = `
	SELECT o.id_offre, o.titre_offre, o.description, o.prix_credits, o.delai_jours, o.jalons_proposes,
	       o.statut_offre, o.date_publication,
	       p.id_pack, p.titre_pack, p.type_preuve,
	       COALESCE((SELECT array_agg(c.libelle_competence ORDER BY c.libelle_competence)
	                 FROM requerir r JOIN competence c USING (id_competence) WHERE r.id_pack = p.id_pack), '{}'),
	       u.id_utilisateur, u.pseudo, u.github_login, u.score_reputation::float8,
	       (` + verifiedDeliveriesSQL + `)
	FROM offre o
	JOIN pack p USING (id_pack)
	JOIN utilisateur u ON u.id_utilisateur = o.id_prestataire`

func scanOffer(row pgx.Row) (OfferView, error) {
	var o OfferView
	var ms []byte
	err := row.Scan(&o.ID, &o.Title, &o.Description, &o.Price, &o.Days, &ms, &o.Status, &o.PublishedAt,
		&o.Pack.ID, &o.Pack.Title, &o.Pack.ProofType, &o.Pack.Skills,
		&o.Provider.ID, &o.Provider.Pseudo, &o.Provider.GitHubLogin, &o.Provider.Reputation, &o.Provider.Deliveries)
	if err != nil {
		return o, err
	}
	return o, json.Unmarshal(ms, &o.Milestones)
}

// GetOffer renvoie une offre ; une offre retirée n'est visible que de son auteur.
func (s *Service) GetOffer(ctx context.Context, a *Actor, id uuid.UUID) (OfferView, error) {
	o, err := scanOffer(s.db.QueryRow(ctx, offerSelect+` WHERE o.id_offre = $1`, id))
	if err != nil {
		return OfferView{}, notFoundIfNoRows(err)
	}
	if o.Status != "PUBLIEE" && (a == nil || a.ID != o.Provider.ID) {
		return OfferView{}, ErrNotFound
	}
	return o, nil
}

// OfferFilter restreint la liste des offres (filtres de l'écran Catalogue).
type OfferFilter struct {
	PackID     *uuid.UUID
	ProviderID *uuid.UUID
	Skill      string // libellé exact, ex. « Docker »
	Q          string
	MaxPrice   int64
	MaxDays    int
	MinRep     float64
}

// ListOffers renvoie les offres publiées (UC03, UC05).
func (s *Service) ListOffers(ctx context.Context, f OfferFilter, limit int, after string) (Page[OfferView], error) {
	cur, err := decodeCursor(after)
	if err != nil {
		return Page[OfferView]{}, err
	}
	limit = clampLimit(limit)
	at, id := cursorArgs(cur)
	rows, err := s.db.Query(ctx, offerSelect+`
		WHERE o.statut_offre = 'PUBLIEE' AND p.actif AND u.statut_utilisateur = 'ACTIF'
		  AND ($1::uuid IS NULL OR o.id_pack = $1)
		  AND ($2::uuid IS NULL OR o.id_prestataire = $2)
		  AND ($3 = '' OR EXISTS (SELECT 1 FROM requerir r JOIN competence c USING (id_competence)
		                          WHERE r.id_pack = p.id_pack AND c.libelle_competence = $3))
		  AND ($4 = '' OR position(lower($4) IN lower(o.titre_offre || ' ' || p.titre_pack || ' ' || u.pseudo)) > 0
		       OR EXISTS (SELECT 1 FROM requerir r JOIN competence c USING (id_competence)
		                  WHERE r.id_pack = p.id_pack AND position(lower($4) IN lower(c.libelle_competence)) > 0))
		  AND ($5 = 0 OR o.prix_credits <= $5)
		  AND ($6 = 0 OR o.delai_jours <= $6)
		  AND u.score_reputation >= $7
		  AND ($8::timestamptz IS NULL OR (o.date_publication, o.id_offre) < ($8, $9))
		ORDER BY o.date_publication DESC, o.id_offre DESC
		LIMIT $10`, f.PackID, f.ProviderID, f.Skill, f.Q, f.MaxPrice, f.MaxDays, f.MinRep, at, id, limit+1)
	if err != nil {
		return Page[OfferView]{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (OfferView, error) { return scanOffer(r) })
	if err != nil {
		return Page[OfferView]{}, err
	}
	return paginate(items, limit, func(o OfferView) cursor { return cursor{At: o.PublishedAt, ID: o.ID} }), nil
}
