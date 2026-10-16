package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

var (
	errNoProject     = newErr(KindNotFound, "projet-scrum-absent", "Projet Scrum absent", "Cet échange n'a pas de projet Scrum (il n'est pas encore accepté, ou il est antérieur à F16).")
	errPOOnly        = forbidden("reserve-product-owner", "Action réservée au demandeur (Product Owner).")
	errDevField      = forbidden("champ-reserve", "Ce champ est réservé à l'autre rôle du projet (R-S6).")
	errSprintClosed  = conflict("sprint-clos", "Ce sprint est terminé ou annulé.")
	errSprintNotDone = conflict("sprint-non-termine", "La rétrospective se remplit une fois le sprint terminé.")
	errStoryDone     = conflict("story-terminee", "Une story terminée ne change plus de sprint.")
	errExchangeOver  = conflict("echange-clos", "Le projet d'un échange clos est en lecture seule.")
	errNoSync        = conflict("synchro-indisponible", "La synchronisation GitHub n'est pas configurée sur ce serveur.")
)

// ScrumRole est le rôle de l'appelant dans le projet.
type ScrumRole string

const (
	ScrumPO      ScrumRole = "PRODUCT_OWNER"
	ScrumDev     ScrumRole = "EQUIPE_DEV"
	ScrumArbiter ScrumRole = "ARBITRE"
)

// ProjectView est le projet Scrum d'un échange.
type ProjectView struct {
	ExchangeID        uuid.UUID
	Template          string
	Objective         string
	DefinitionOfDone  []string
	DefinitionOfReady []string
	Columns           []scrum.StoryStatus
	Points            []int
	StoryPattern      string
	SprintDays        int
	SyncStatus        string
	SyncDetail        *string
	PendingCalls      int
	Role              ScrumRole
	Amendment         *AmendmentView
	CreatedAt         time.Time
}

// scrumAccess charge l'échange et vérifie l'accès (parties, arbitre en lecture).
func (s *Service) scrumAccess(ctx context.Context, q querier, a Actor, exID uuid.UUID, write bool) (exRow, ScrumRole, error) {
	r, err := loadExchange(ctx, q, exID, false)
	if err != nil {
		return r, "", err
	}
	role := ScrumArbiter
	switch a.ID {
	case r.Requester:
		role = ScrumPO
	case r.Provider:
		role = ScrumDev
	default:
		ok, err := canSee(ctx, q, a, r)
		if err != nil {
			return r, "", err
		}
		if !ok || write {
			return r, "", ErrNotFound
		}
	}
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projet_scrum WHERE id_echange = $1)`, exID).Scan(&exists); err != nil {
		return r, "", err
	}
	if !exists {
		return r, "", errNoProject
	}
	if write && r.Status != exchange.InProgress && r.Status != exchange.InDispute {
		return r, "", errExchangeOver
	}
	return r, role, nil
}

// GetScrum renvoie le projet Scrum d'un échange (parties, arbitre F9).
func (s *Service) GetScrum(ctx context.Context, a Actor, exID uuid.UUID) (ProjectView, error) {
	_, role, err := s.scrumAccess(ctx, s.db, a, exID, false)
	if err != nil {
		return ProjectView{}, err
	}
	v := ProjectView{ExchangeID: exID, Role: role, Columns: scrum.Standard.Columns, Points: scrum.Standard.Points,
		StoryPattern: scrum.Standard.StoryPattern, DefinitionOfReady: scrum.Standard.DefinitionOfReady}
	var dod []byte
	if err := s.db.QueryRow(ctx, `
		SELECT p.template, p.objectif_produit, p.definition_of_done, p.statut_synchro, p.detail_synchro, p.date_creation, e.duree_sprint_jours,
		       (SELECT count(*) FROM outbox_github o WHERE o.id_echange = p.id_echange AND o.traite_le IS NULL)::int
		FROM projet_scrum p JOIN echange e USING (id_echange) WHERE p.id_echange = $1`, exID).
		Scan(&v.Template, &v.Objective, &dod, &v.SyncStatus, &v.SyncDetail, &v.CreatedAt, &v.SprintDays, &v.PendingCalls); err != nil {
		return ProjectView{}, err
	}
	if err := json.Unmarshal(dod, &v.DefinitionOfDone); err != nil {
		return ProjectView{}, err
	}
	am, err := s.openAmendment(ctx, s.db, exID)
	if err != nil {
		return ProjectView{}, err
	}
	v.Amendment = am
	return v, nil
}

// ProjectPatch modifie l'objectif produit ou la Definition of Done (PO).
type ProjectPatch struct {
	Objective        *string
	DefinitionOfDone []string
}

// PatchScrum modifie le projet (demandeur seulement).
func (s *Service) PatchScrum(ctx context.Context, a Actor, exID uuid.UUID, p ProjectPatch) (ProjectView, error) {
	_, role, err := s.scrumAccess(ctx, s.db, a, exID, true)
	if err != nil {
		return ProjectView{}, err
	}
	if role != ScrumPO {
		return ProjectView{}, errPOOnly
	}
	if p.Objective != nil && utf8.RuneCountInString(*p.Objective) > 2000 {
		return ProjectView{}, invalid("donnees-invalides", "objectif_produit : 2000 caractères maximum")
	}
	var dod []byte
	if p.DefinitionOfDone != nil {
		if len(p.DefinitionOfDone) > 20 {
			return ProjectView{}, invalid("donnees-invalides", "definition_of_done : 20 éléments maximum")
		}
		for _, c := range p.DefinitionOfDone {
			if n := utf8.RuneCountInString(strings.TrimSpace(c)); n == 0 || n > 200 {
				return ProjectView{}, invalid("donnees-invalides", "definition_of_done : éléments de 1 à 200 caractères")
			}
		}
		dod, _ = json.Marshal(p.DefinitionOfDone)
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE projet_scrum SET objectif_produit = COALESCE($2, objectif_produit),
		       definition_of_done = COALESCE($3::jsonb, definition_of_done)
		WHERE id_echange = $1`, exID, p.Objective, dod); err != nil {
		return ProjectView{}, err
	}
	return s.GetScrum(ctx, a, exID)
}

// ---------- Sprints ----------

// SprintView est un sprint tel qu'affiché.
type SprintView struct {
	ID             uuid.UUID
	Number         int
	MilestoneID    uuid.UUID
	MilestoneOrder int
	Start, End     time.Time
	StartedAt      *time.Time
	EndedAt        *time.Time
	Status         scrum.SprintStatus
	Objective      string
	Issue          *int
	PointsPlanned  int
	PointsDone     int
	RetroGood      *string
	RetroImprove   *string
	RetroActions   *string
}

const sprintSelect = `
	SELECT sp.id_sprint, sp.numero, sp.id_jalon, j.ordre, sp.debut_prevu, sp.fin_prevue, sp.debut_reel, sp.fin_reelle,
	       sp.statut_sprint, sp.objectif_sprint, sp.github_issue_no,
	       COALESCE(sp.points_prevus, (SELECT COALESCE(SUM(points), 0) FROM story WHERE id_sprint = sp.id_sprint))::int,
	       COALESCE(sp.points_faits, (SELECT COALESCE(SUM(points), 0) FROM story WHERE id_sprint = sp.id_sprint AND statut_story = 'TERMINE'))::int,
	       sp.retro_bien, sp.retro_ameliorer, sp.retro_actions
	FROM sprint sp JOIN jalon j USING (id_jalon)`

func (s *Service) sprintViews(ctx context.Context, q querier, exID uuid.UUID) ([]SprintView, error) {
	rows, err := q.Query(ctx, sprintSelect+` WHERE sp.id_echange = $1 ORDER BY sp.numero`, exID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[SprintView])
}

// ListSprints renvoie les sprints du projet.
func (s *Service) ListSprints(ctx context.Context, a Actor, exID uuid.UUID) ([]SprintView, error) {
	if _, _, err := s.scrumAccess(ctx, s.db, a, exID, false); err != nil {
		return nil, err
	}
	return s.sprintViews(ctx, s.db, exID)
}

func (s *Service) loadSprintByNumber(ctx context.Context, tx pgx.Tx, exID uuid.UUID, n int) (SprintView, error) {
	rows, err := tx.Query(ctx, sprintSelect+` WHERE sp.id_echange = $1 AND sp.numero = $2 FOR UPDATE OF sp`, exID, n)
	if err != nil {
		return SprintView{}, err
	}
	v, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[SprintView])
	return v, notFoundIfNoRows(err)
}

// UpdateSprintObjective fixe l'objectif d'un sprint à venir ou en cours (parties).
func (s *Service) UpdateSprintObjective(ctx context.Context, a Actor, exID uuid.UUID, number int, objective string) (SprintView, error) {
	if utf8.RuneCountInString(objective) > 1000 {
		return SprintView{}, invalid("donnees-invalides", "objectif : 1000 caractères maximum")
	}
	var out SprintView
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, _, err := s.scrumAccess(ctx, tx, a, exID, true); err != nil {
			return err
		}
		sp, err := s.loadSprintByNumber(ctx, tx, exID, number)
		if err != nil {
			return err
		}
		if sp.Status != scrum.SprintPlanned && sp.Status != scrum.SprintActive {
			return errSprintClosed
		}
		if _, err := tx.Exec(ctx, `UPDATE sprint SET objectif_sprint = $2 WHERE id_sprint = $1`, sp.ID, objective); err != nil {
			return err
		}
		if sp.Status == scrum.SprintActive {
			if err := s.enqueue(ctx, tx, exID, uniqueKey("maj-sprint:"+sp.ID.String()), opUpdateIssue, map[string]any{"type": "sprint", "id": sp.ID}); err != nil {
				return err
			}
		}
		out, err = s.loadSprintByNumber(ctx, tx, exID, number)
		return err
	})
	return out, err
}

// Retro est la rétrospective d'un sprint terminé.
type Retro struct {
	Good, Improve, Actions string
}

// SaveRetro enregistre la rétrospective d'un sprint terminé (parties).
func (s *Service) SaveRetro(ctx context.Context, a Actor, exID uuid.UUID, number int, r Retro) (SprintView, error) {
	for _, f := range []string{r.Good, r.Improve, r.Actions} {
		if utf8.RuneCountInString(f) > 2000 {
			return SprintView{}, invalid("donnees-invalides", "rétrospective : 2000 caractères maximum par champ")
		}
	}
	var out SprintView
	err := s.tx(ctx, func(tx pgx.Tx) error {
		_, role, err := s.scrumAccess(ctx, tx, a, exID, false)
		if err != nil {
			return err
		}
		if role == ScrumArbiter {
			return ErrNotFound
		}
		sp, err := s.loadSprintByNumber(ctx, tx, exID, number)
		if err != nil {
			return err
		}
		if sp.Status != scrum.SprintDone {
			return errSprintNotDone
		}
		if _, err := tx.Exec(ctx, `UPDATE sprint SET retro_bien = $2, retro_ameliorer = $3, retro_actions = $4 WHERE id_sprint = $1`,
			sp.ID, r.Good, r.Improve, r.Actions); err != nil {
			return err
		}
		out, err = s.loadSprintByNumber(ctx, tx, exID, number)
		return err
	})
	return out, err
}

// ---------- Stories ----------

// StoryView est une story telle qu'affichée.
type StoryView struct {
	ID             uuid.UUID
	MilestoneOrder *int
	SprintNumber   *int
	Title          string
	Description    string
	Criteria       []string
	Points         *int
	Priority       int
	Status         scrum.StoryStatus
	Issue          *int
	CreatedAt      time.Time
	DoneAt         *time.Time
	CreatedBy      uuid.UUID
}

const storySelect = `
	SELECT st.id_story, COALESCE(js.ordre, jt.ordre), sp.numero, st.titre, st.description, st.criteres, st.points::int,
	       st.priorite, st.statut_story, st.github_issue_no, st.date_creation, st.date_terminee, st.cree_par
	FROM story st
	LEFT JOIN sprint sp ON sp.id_sprint = st.id_sprint
	LEFT JOIN jalon js ON js.id_jalon = sp.id_jalon
	LEFT JOIN jalon jt ON jt.id_jalon = st.id_jalon`

func scanStory(row pgx.CollectableRow) (StoryView, error) {
	var v StoryView
	var crit []byte
	err := row.Scan(&v.ID, &v.MilestoneOrder, &v.SprintNumber, &v.Title, &v.Description, &crit, &v.Points,
		&v.Priority, &v.Status, &v.Issue, &v.CreatedAt, &v.DoneAt, &v.CreatedBy)
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal(crit, &v.Criteria)
}

// StoryFilter restreint la liste des stories.
type StoryFilter struct {
	Sprint    *int
	Milestone *int
	Status    scrum.StoryStatus
	Backlog   bool // stories sans sprint seulement
}

// ListStories renvoie les stories du projet, par priorité.
func (s *Service) ListStories(ctx context.Context, a Actor, exID uuid.UUID, f StoryFilter) ([]StoryView, error) {
	if _, _, err := s.scrumAccess(ctx, s.db, a, exID, false); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, storySelect+`
		WHERE st.id_echange = $1
		  AND ($2::int IS NULL OR sp.numero = $2)
		  AND ($3::int IS NULL OR COALESCE(js.ordre, jt.ordre) = $3)
		  AND ($4 = '' OR st.statut_story::text = $4)
		  AND (NOT $5 OR st.id_sprint IS NULL)
		ORDER BY st.priorite, st.date_creation`, exID, f.Sprint, f.Milestone, string(f.Status), f.Backlog)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanStory)
}

func (s *Service) getStory(ctx context.Context, q querier, id uuid.UUID) (StoryView, error) {
	rows, err := q.Query(ctx, storySelect+` WHERE st.id_story = $1`, id)
	if err != nil {
		return StoryView{}, err
	}
	v, err := pgx.CollectExactlyOneRow(rows, scanStory)
	return v, notFoundIfNoRows(err)
}

// StoryInput décrit une story créée par le PO.
type StoryInput struct {
	Title          string
	Description    string
	Criteria       []string
	MilestoneOrder *int
	SprintNumber   *int
}

func validateStoryText(title, desc *string, crit []string) error {
	if title != nil {
		if n := utf8.RuneCountInString(strings.TrimSpace(*title)); n < 3 || utf8.RuneCountInString(*title) > 160 {
			return invalid("donnees-invalides", "titre : entre 3 et 160 caractères")
		}
	}
	if desc != nil && utf8.RuneCountInString(*desc) > 4000 {
		return invalid("donnees-invalides", "description : 4000 caractères maximum")
	}
	if len(crit) > 20 {
		return invalid("donnees-invalides", "criteres : 20 critères maximum")
	}
	for _, c := range crit {
		if n := utf8.RuneCountInString(strings.TrimSpace(c)); n == 0 || n > 300 {
			return invalid("donnees-invalides", "criteres : de 1 à 300 caractères")
		}
	}
	return nil
}

// milestoneByOrder renvoie l'identifiant du jalon d'ordre n de l'échange.
func milestoneByOrder(ctx context.Context, q querier, exID uuid.UUID, n int) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id_jalon FROM jalon WHERE id_echange = $1 AND ordre = $2`, exID, n).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, invalid("donnees-invalides", "jalon inconnu")
	}
	return id, err
}

// openSprint renvoie un sprint planifié ou actif pour y affecter une story.
func openSprint(ctx context.Context, q querier, exID uuid.UUID, n int) (uuid.UUID, scrum.SprintStatus, error) {
	var id uuid.UUID
	var st scrum.SprintStatus
	err := q.QueryRow(ctx, `SELECT id_sprint, statut_sprint FROM sprint WHERE id_echange = $1 AND numero = $2`, exID, n).Scan(&id, &st)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, st, invalid("donnees-invalides", "sprint inconnu")
	}
	if err == nil && st != scrum.SprintPlanned && st != scrum.SprintActive {
		return id, st, errSprintClosed
	}
	return id, st, err
}

// CreateStory ajoute une story au backlog, ou directement à un sprint (PO).
func (s *Service) CreateStory(ctx context.Context, a Actor, exID uuid.UUID, in StoryInput) (StoryView, error) {
	if err := validateStoryText(&in.Title, &in.Description, in.Criteria); err != nil {
		return StoryView{}, err
	}
	if in.Criteria == nil {
		in.Criteria = []string{}
	}
	var id uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		_, role, err := s.scrumAccess(ctx, tx, a, exID, true)
		if err != nil {
			return err
		}
		if role != ScrumPO {
			return errPOOnly
		}
		var milestone, sprint *uuid.UUID
		if in.MilestoneOrder != nil {
			m, err := milestoneByOrder(ctx, tx, exID, *in.MilestoneOrder)
			if err != nil {
				return err
			}
			milestone = &m
		}
		status := scrum.StoryBacklog
		if in.SprintNumber != nil {
			sp, _, err := openSprint(ctx, tx, exID, *in.SprintNumber)
			if err != nil {
				return err
			}
			sprint, status = &sp, scrum.StoryTodo
		}
		crit, _ := json.Marshal(in.Criteria)
		if err := tx.QueryRow(ctx, `
			INSERT INTO story (id_echange, id_jalon, id_sprint, titre, description, criteres, priorite, statut_story, cree_par, date_creation)
			VALUES ($1, $2, $3, $4, $5, $6, (SELECT COALESCE(max(priorite), 0) + 1 FROM story WHERE id_echange = $1), $7, $8, $9)
			RETURNING id_story`, exID, milestone, sprint, in.Title, in.Description, crit, status, a.ID, s.now()).Scan(&id); err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "STORY", id, "", string(status), &a.ID, "", s.now()); err != nil {
			return err
		}
		if sprint != nil {
			return s.enqueue(ctx, tx, exID, "issue-story:"+id.String(), opIssueStory, map[string]any{"id_story": id})
		}
		return nil
	})
	if err != nil {
		return StoryView{}, err
	}
	return s.getStory(ctx, s.db, id)
}

// StoryPatch modifie une story ; un champ nil reste inchangé (R-S6) :
//   - demandeur (PO) : titre, description, critères, priorité, jalon, sprint
//     (ToBacklog pour la renvoyer au backlog), statut (accepter en « Terminé ») ;
//   - prestataire (équipe) : statut, points.
type StoryPatch struct {
	Title          *string
	Description    *string
	Criteria       []string
	Priority       *int
	MilestoneOrder *int
	SprintNumber   *int
	ToBacklog      bool
	Status         *scrum.StoryStatus
	Points         *int
}

func (p StoryPatch) poFields() bool {
	return p.Title != nil || p.Description != nil || p.Criteria != nil || p.Priority != nil ||
		p.MilestoneOrder != nil || p.SprintNumber != nil || p.ToBacklog
}

// PatchStory applique une modification autorisée au rôle de l'appelant.
func (s *Service) PatchStory(ctx context.Context, a Actor, exID, storyID uuid.UUID, p StoryPatch) (StoryView, error) {
	if err := validateStoryText(p.Title, p.Description, p.Criteria); err != nil {
		return StoryView{}, err
	}
	if p.Points != nil && !scrum.ValidPoints(*p.Points) {
		return StoryView{}, invalid("donnees-invalides", "points : 0, 1, 2, 3, 5, 8, 13 ou 21")
	}
	if p.Priority != nil && *p.Priority < 1 {
		return StoryView{}, invalid("donnees-invalides", "priorite : entier positif")
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		_, role, err := s.scrumAccess(ctx, tx, a, exID, true)
		if err != nil {
			return err
		}
		if (role == ScrumDev && p.poFields()) || (role == ScrumPO && p.Points != nil) {
			return errDevField
		}
		var status scrum.StoryStatus
		var sprintID *uuid.UUID
		var issue *int
		if err := tx.QueryRow(ctx, `SELECT statut_story, id_sprint, github_issue_no FROM story WHERE id_story = $1 AND id_echange = $2 FOR UPDATE`,
			storyID, exID).Scan(&status, &sprintID, &issue); err != nil {
			return notFoundIfNoRows(err)
		}
		by := scrum.PO
		if role == ScrumDev {
			by = scrum.Dev
		}
		textChanged := p.Title != nil || p.Description != nil || p.Criteria != nil || p.Points != nil
		if p.Criteria != nil {
			crit, _ := json.Marshal(p.Criteria)
			if _, err := tx.Exec(ctx, `UPDATE story SET criteres = $2 WHERE id_story = $1`, storyID, crit); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE story SET titre = COALESCE($2, titre), description = COALESCE($3, description),
			       priorite = COALESCE($4, priorite), points = COALESCE($5, points)
			WHERE id_story = $1`, storyID, p.Title, p.Description, p.Priority, p.Points); err != nil {
			return err
		}
		if p.MilestoneOrder != nil {
			m, err := milestoneByOrder(ctx, tx, exID, *p.MilestoneOrder)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE story SET id_jalon = $2 WHERE id_story = $1`, storyID, m); err != nil {
				return err
			}
		}
		// Affectation à un sprint, ou retour au backlog.
		if p.SprintNumber != nil || p.ToBacklog {
			if status == scrum.StoryDone {
				return errStoryDone
			}
			if p.ToBacklog {
				if sprintID != nil {
					if err := s.moveStory(ctx, tx, storyID, nil, status, scrum.StoryBacklog, &a.ID); err != nil {
						return err
					}
					status, sprintID = scrum.StoryBacklog, nil
				}
			} else {
				sp, _, err := openSprint(ctx, tx, exID, *p.SprintNumber)
				if err != nil {
					return err
				}
				to := status
				if status == scrum.StoryBacklog {
					to = scrum.StoryTodo
				}
				if err := s.moveStory(ctx, tx, storyID, &sp, status, to, &a.ID); err != nil {
					return err
				}
				if issue == nil {
					if err := s.enqueue(ctx, tx, exID, uniqueKey("issue-story:"+storyID.String()), opIssueStory, map[string]any{"id_story": storyID}); err != nil {
						return err
					}
				} else {
					textChanged = true // labels sprint:n à jour
				}
				status, sprintID = to, &sp
			}
		}
		if p.Status != nil && *p.Status != status {
			if *p.Status == scrum.StoryBacklog || status == scrum.StoryBacklog {
				return invalid("donnees-invalides", "statut : le backlog se gère par l'affectation au sprint")
			}
			if err := scrum.CheckStory(status, *p.Status, by); err != nil {
				return errDevField
			}
			if err := s.setStoryStatus(ctx, tx, storyID, status, *p.Status, &a.ID, ""); err != nil {
				return err
			}
			if issue != nil {
				state := ""
				switch {
				case *p.Status == scrum.StoryDone:
					state = "closed"
				case status == scrum.StoryDone:
					state = "open"
				}
				if state != "" {
					if err := s.enqueue(ctx, tx, exID, uniqueKey("etat-story:"+storyID.String()), opUpdateIssue,
						map[string]any{"type": "story", "id": storyID, "etat": state}); err != nil {
						return err
					}
					textChanged = false
				}
			}
		}
		if textChanged && issue != nil {
			if err := s.enqueue(ctx, tx, exID, uniqueKey("maj-story:"+storyID.String()), opUpdateIssue, map[string]any{"type": "story", "id": storyID}); err != nil {
				return err
			}
		}
		if sprintID != nil {
			var active bool
			if err := tx.QueryRow(ctx, `SELECT statut_sprint = 'ACTIF' AND github_issue_no IS NOT NULL FROM sprint WHERE id_sprint = $1`, *sprintID).Scan(&active); err != nil {
				return err
			}
			if active && textChanged {
				return s.enqueue(ctx, tx, exID, uniqueKey("maj-sprint:"+sprintID.String()), opUpdateIssue, map[string]any{"type": "sprint", "id": *sprintID})
			}
		}
		return nil
	})
	if err != nil {
		return StoryView{}, err
	}
	return s.getStory(ctx, s.db, storyID)
}

func (s *Service) moveStory(ctx context.Context, tx pgx.Tx, id uuid.UUID, sprint *uuid.UUID, from, to scrum.StoryStatus, by *uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE story SET id_sprint = $2, statut_story = $3 WHERE id_story = $1`, id, sprint, to); err != nil {
		return err
	}
	if from == to {
		return nil
	}
	return recordTransition(ctx, tx, "STORY", id, string(from), string(to), by, "", s.now())
}

// setStoryStatus change la colonne d'une story (historique ENF04).
func (s *Service) setStoryStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, from, to scrum.StoryStatus, by *uuid.UUID, reason string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE story SET statut_story = $2::statut_story,
		       date_terminee = CASE WHEN $2::statut_story = 'TERMINE' THEN $3::timestamptz ELSE NULL END
		WHERE id_story = $1`, id, to, s.now()); err != nil {
		return err
	}
	return recordTransition(ctx, tx, "STORY", id, string(from), string(to), by, reason, s.now())
}

// ---------- Planning (Gantt) ----------

// PlanningSprint est un sprint dans le Gantt.
type PlanningSprint struct {
	Number        int
	Start, End    time.Time
	StartedAt     *time.Time
	EndedAt       *time.Time
	Status        scrum.SprintStatus
	PointsPlanned int
	PointsDone    int
	Issue         *int
}

// PlanningMilestone est une ligne de jalon dans le Gantt.
type PlanningMilestone struct {
	Order      int
	Title      string
	Start      time.Time
	Deadline   time.Time
	Status     exchange.MilestoneStatus
	Amount     int64
	Late       bool
	Risk       scrum.RiskLevel
	GitHubNo   *int
	PointsDone int
	PointsAll  int
	Sprints    []PlanningSprint
}

// PlanningMarker est un événement daté (preuve PR).
type PlanningMarker struct {
	Kind      string
	At        time.Time
	Milestone int
	PR        *int
}

// Planning est le Gantt prêt à dessiner.
type Planning struct {
	Start, End time.Time
	Today      time.Time
	Milestones []PlanningMilestone
	Markers    []PlanningMarker
}

// GetPlanning renvoie le Gantt : jalons, sprints imbriqués, retard et risque (R-S4).
func (s *Service) GetPlanning(ctx context.Context, a Actor, exID uuid.UUID) (Planning, error) {
	r, _, err := s.scrumAccess(ctx, s.db, a, exID, false)
	if err != nil {
		return Planning{}, err
	}
	var accepted time.Time
	if err := s.db.QueryRow(ctx, `SELECT date_acceptation FROM echange WHERE id_echange = $1`, exID).Scan(&accepted); err != nil {
		return Planning{}, err
	}
	rows, err := s.db.Query(ctx, `SELECT id_jalon, ordre, titre_jalon, statut_jalon, echeance, montant_jalon, github_milestone_no
		FROM jalon WHERE id_echange = $1 ORDER BY ordre`, exID)
	if err != nil {
		return Planning{}, err
	}
	type msRow struct {
		ID       uuid.UUID
		Order    int
		Title    string
		Status   exchange.MilestoneStatus
		Deadline time.Time
		Amount   int64
		GitHubNo *int
	}
	ms, err := pgx.CollectRows(rows, pgx.RowToStructByPos[msRow])
	if err != nil {
		return Planning{}, err
	}
	sprints, err := s.sprintViews(ctx, s.db, exID)
	if err != nil {
		return Planning{}, err
	}
	now := s.now()
	p := Planning{Start: accepted, Today: now}
	prev := accepted
	for _, m := range ms {
		done, total, err := milestonePoints(ctx, s.db, m.ID)
		if err != nil {
			return Planning{}, err
		}
		pm := PlanningMilestone{Order: m.Order, Title: m.Title, Start: prev, Deadline: m.Deadline, Status: m.Status, Amount: m.Amount,
			GitHubNo: m.GitHubNo, PointsDone: done, PointsAll: total, Risk: scrum.RiskNone, Sprints: []PlanningSprint{}}
		open := !m.Status.Final() && m.Status != exchange.Delivered
		pm.Late = open && now.After(m.Deadline) && r.Status != exchange.Cancelled
		if m.Status == exchange.Active {
			pm.Risk = scrum.Risk(now, prev, m.Deadline, done, total)
		}
		for _, sp := range sprints {
			if sp.MilestoneID == m.ID {
				pm.Sprints = append(pm.Sprints, PlanningSprint{Number: sp.Number, Start: sp.Start, End: sp.End, StartedAt: sp.StartedAt,
					EndedAt: sp.EndedAt, Status: sp.Status, PointsPlanned: sp.PointsPlanned, PointsDone: sp.PointsDone, Issue: sp.Issue})
			}
		}
		p.Milestones = append(p.Milestones, pm)
		prev = m.Deadline
	}
	p.End = prev
	mrows, err := s.db.Query(ctx, `
		SELECT 'PREUVE_PR', p.date_creation, j.ordre, p.pr_numero FROM preuve p JOIN jalon j USING (id_jalon)
		WHERE j.id_echange = $1 AND p.pr_numero IS NOT NULL ORDER BY p.date_creation`, exID)
	if err != nil {
		return Planning{}, err
	}
	p.Markers, err = pgx.CollectRows(mrows, pgx.RowToStructByPos[PlanningMarker])
	if err != nil {
		return Planning{}, err
	}
	if p.Markers == nil {
		p.Markers = []PlanningMarker{}
	}
	return p, nil
}

// ResyncScrum relance la synchronisation GitHub du projet (parties).
func (s *Service) ResyncScrum(ctx context.Context, a Actor, exID uuid.UUID) (ProjectView, error) {
	if s.issues == nil {
		return ProjectView{}, errNoSync
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, _, err := s.scrumAccess(ctx, tx, a, exID, true); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox_github SET prochain_essai = $2 WHERE id_echange = $1 AND traite_le IS NULL`, exID, s.now()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE projet_scrum SET statut_synchro = 'ACTIVE', detail_synchro = NULL WHERE id_echange = $1`, exID); err != nil {
			return err
		}
		return s.enqueueMissing(ctx, tx, exID)
	})
	if err != nil {
		return ProjectView{}, err
	}
	return s.GetScrum(ctx, a, exID)
}
