package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

// Synchronisation GitHub du module Scrum (D18, D19) : les appels sont mis en
// file dans la transaction métier et exécutés ici, un à la fois, hors de
// toute transaction. Un échec GitHub ne bloque jamais une acceptation ni une
// libération : il reste en file avec un délai croissant.

const maxBackoff = time.Hour

type outboxRow struct {
	ID         int64
	ExchangeID uuid.UUID
	Op         string
	Payload    []byte
	Attempts   int
}

type outboxPayload struct {
	MilestoneID uuid.UUID `json:"id_jalon"`
	SprintID    uuid.UUID `json:"id_sprint"`
	StoryID     uuid.UUID `json:"id_story"`
	Kind        string    `json:"type"` // sprint ou story
	ID          uuid.UUID `json:"id"`
	Comment     string    `json:"commentaire"`
	State       string    `json:"etat"`
	Recreated   bool      `json:"recree"`
}

// ProcessGitHubOutbox exécute au plus limit appels GitHub en attente, dans
// l'ordre de mise en file. Après un échec, les appels suivants du même
// échange attendent le prochain passage (l'issue d'un sprint suppose son milestone).
func (s *Service) ProcessGitHubOutbox(ctx context.Context, limit int) (int, error) {
	if s.issues == nil {
		return 0, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT o.id, o.id_echange, o.operation, o.charge, o.tentatives FROM outbox_github o
		WHERE o.traite_le IS NULL AND o.prochain_essai <= $1
		  -- l'ordre par échange est préservé : un appel en attente de reprise bloque les suivants
		  AND NOT EXISTS (SELECT 1 FROM outbox_github b WHERE b.id_echange = o.id_echange AND b.traite_le IS NULL
		                  AND b.id < o.id AND b.prochain_essai > $1)
		ORDER BY o.id LIMIT $2`, s.now(), limit)
	if err != nil {
		return 0, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[outboxRow])
	if err != nil {
		return 0, err
	}
	failed := map[uuid.UUID]bool{}
	done := 0
	for _, o := range list {
		if failed[o.ExchangeID] {
			continue
		}
		opErr := s.runOutbox(ctx, o)
		if opErr == nil {
			if _, err := s.db.Exec(ctx, `UPDATE outbox_github SET traite_le = $2, erreur = NULL WHERE id = $1`, o.ID, s.now()); err != nil {
				return done, err
			}
			if _, err := s.db.Exec(ctx, `
				UPDATE projet_scrum SET statut_synchro = 'ACTIVE', detail_synchro = NULL
				WHERE id_echange = $1 AND statut_synchro <> 'ACTIVE'`, o.ExchangeID); err != nil {
				return done, err
			}
			done++
			continue
		}
		failed[o.ExchangeID] = true
		if err := s.outboxFailed(ctx, o, opErr); err != nil {
			return done, err
		}
	}
	return done, nil
}

// outboxFailed reporte l'appel (backoff exponentiel plafonné, ou Retry-After)
// et reflète l'état dans le bandeau de synchronisation du projet.
func (s *Service) outboxFailed(ctx context.Context, o outboxRow, opErr error) error {
	status, detail := "ERREUR", "GitHub a refusé l'opération : nouvel essai automatique."
	delay := min(time.Duration(1<<min(o.Attempts, 6))*time.Minute, maxBackoff)
	var rl *RateLimitError
	switch {
	case errors.Is(opErr, ErrGitHubPermission):
		status, detail, delay = "PERMISSION_MANQUANTE", "La GitHub App n'a pas la permission issues:write sur ce dépôt.", maxBackoff
	case errors.As(opErr, &rl):
		detail, delay = "Quota GitHub atteint : reprise automatique.", max(rl.RetryAfter, time.Minute)
	case errors.Is(opErr, ErrGitHubNotFound):
		detail = "GitHub App non installée sur le dépôt, ou dépôt introuvable."
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE outbox_github SET tentatives = tentatives + 1, prochain_essai = $2, erreur = $3 WHERE id = $1`,
		o.ID, s.now().Add(delay), truncate(opErr.Error(), 500)); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE projet_scrum SET statut_synchro = $2, detail_synchro = $3 WHERE id_echange = $1`,
		o.ExchangeID, status, detail)
	return err
}

// repoOf renvoie le dépôt de l'échange par son identifiant GitHub (R-S2),
// résolu une fois puis figé dans l'échange (D6).
func (s *Service) repoOf(ctx context.Context, exID uuid.UUID) (RepoRef, error) {
	var ref RepoRef
	var id *int64
	if err := s.db.QueryRow(ctx, `SELECT depot_owner, depot_nom, github_repo_id FROM echange WHERE id_echange = $1`, exID).
		Scan(&ref.Owner, &ref.Name, &id); err != nil {
		return ref, err
	}
	if id != nil {
		ref.ID = *id
		return ref, nil
	}
	if s.github == nil {
		return ref, ErrGitHubNotFound
	}
	repo, err := s.github.Repository(ctx, ref.Owner, ref.Name)
	if err != nil {
		return ref, err
	}
	if _, err := s.db.Exec(ctx, `UPDATE echange SET github_repo_id = $2 WHERE id_echange = $1 AND github_repo_id IS NULL`, exID, repo.ID); err != nil {
		return ref, err
	}
	ref.ID = repo.ID
	return ref, nil
}

func (s *Service) runOutbox(ctx context.Context, o outboxRow) error {
	var p outboxPayload
	if err := json.Unmarshal(o.Payload, &p); err != nil {
		return fmt.Errorf("charge illisible : %w", err)
	}
	repo, err := s.repoOf(ctx, o.ExchangeID)
	if err != nil {
		return err
	}
	switch o.Op {
	case opLabels:
		var milestones, sprints int
		if err := s.db.QueryRow(ctx, `
			SELECT (SELECT count(*) FROM jalon WHERE id_echange = $1)::int,
			       (SELECT COALESCE(max(numero), 0) FROM sprint WHERE id_echange = $1)::int`, o.ExchangeID).Scan(&milestones, &sprints); err != nil {
			return err
		}
		return s.issues.EnsureLabels(ctx, repo, scrum.ProjectLabels(milestones, sprints))
	case opMilestone:
		var order int
		var title string
		var due time.Time
		if err := s.db.QueryRow(ctx, `SELECT ordre, titre_jalon, echeance FROM jalon WHERE id_jalon = $1`, p.MilestoneID).Scan(&order, &title, &due); err != nil {
			return err
		}
		n, err := s.issues.EnsureMilestone(ctx, repo, fmt.Sprintf("Jalon %d — %s", order, title), due)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(ctx, `UPDATE jalon SET github_milestone_no = $2 WHERE id_jalon = $1`, p.MilestoneID, n)
		return err
	case opIssueSprint:
		return s.createSprintIssue(ctx, o.ExchangeID, repo, p.SprintID, p.Recreated)
	case opIssueStory:
		return s.createStoryIssue(ctx, o.ExchangeID, repo, p.StoryID, p.Recreated)
	case opUpdateIssue:
		return s.updateIssue(ctx, o.ExchangeID, repo, p)
	case opCloseIssue:
		n, err := s.issueNumber(ctx, p.Kind, p.ID)
		if err != nil || n == 0 {
			return err
		}
		if p.Comment != "" {
			if err := s.issues.Comment(ctx, repo, n, p.Comment); err != nil {
				return ignoreGone(err)
			}
		}
		return ignoreGone(s.issues.UpdateIssue(ctx, repo, n, IssueUpdate{State: "closed"}))
	}
	return fmt.Errorf("opération inconnue %q", o.Op)
}

// ignoreGone : fermer une issue supprimée n'a plus d'objet.
func ignoreGone(err error) error {
	if errors.Is(err, ErrGitHubNotFound) {
		return nil
	}
	return err
}

func (s *Service) issueNumber(ctx context.Context, kind string, id uuid.UUID) (int, error) {
	table, col := "story", "id_story"
	if kind == "sprint" {
		table, col = "sprint", "id_sprint"
	}
	var n *int
	err := s.db.QueryRow(ctx, `SELECT github_issue_no FROM `+table+` WHERE `+col+` = $1`, id).Scan(&n)
	if err != nil || n == nil {
		return 0, notFoundIgnored(err)
	}
	return *n, nil
}

func notFoundIgnored(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

const recreatedNote = "> Issue recréée par CodeTroc : la précédente a été supprimée.\n\n"

func (s *Service) link(exID uuid.UUID) string {
	if s.appURL == "" {
		return ""
	}
	return s.appURL + "/echanges/" + exID.String()
}

// sprintIssue rend l'issue d'un sprint à partir de l'état courant.
func (s *Service) sprintIssue(ctx context.Context, exID, sprintID uuid.UUID) (scrum.Issue, *int, *int, string, error) {
	var d scrum.SprintIssueData
	var issueNo, milestoneNo *int
	var dod []byte
	var assignee string
	err := s.db.QueryRow(ctx, `
		SELECT sp.numero, j.ordre, j.titre_jalon, sp.objectif_sprint, sp.debut_prevu, sp.fin_prevue, j.echeance, j.montant_jalon,
		       (SELECT count(*) FROM sprint s2 WHERE s2.id_jalon = sp.id_jalon AND s2.numero <= sp.numero AND s2.statut_sprint <> 'ANNULE')::int,
		       (SELECT count(*) FROM sprint s2 WHERE s2.id_jalon = sp.id_jalon AND s2.statut_sprint <> 'ANNULE')::int,
		       p.definition_of_done, sp.github_issue_no, j.github_milestone_no, COALESCE(u.github_login, '')
		FROM sprint sp JOIN jalon j USING (id_jalon) JOIN projet_scrum p ON p.id_echange = sp.id_echange
		JOIN echange e ON e.id_echange = sp.id_echange JOIN utilisateur u ON u.id_utilisateur = e.id_prestataire
		WHERE sp.id_sprint = $1`, sprintID).
		Scan(&d.Number, &d.MilestoneOrder, &d.MilestoneTitle, &d.Objective, &d.Start, &d.End, &d.Deadline, &d.Amount,
			&d.Index, &d.Count, &dod, &issueNo, &milestoneNo, &assignee)
	if err != nil {
		return scrum.Issue{}, nil, nil, "", err
	}
	_ = json.Unmarshal(dod, &d.DoD)
	rows, err := s.db.Query(ctx, `SELECT COALESCE(github_issue_no, 0), titre, points FROM story WHERE id_sprint = $1 ORDER BY priorite`, sprintID)
	if err != nil {
		return scrum.Issue{}, nil, nil, "", err
	}
	d.Stories, err = pgx.CollectRows(rows, pgx.RowToStructByPos[scrum.IssueStory])
	if err != nil {
		return scrum.Issue{}, nil, nil, "", err
	}
	d.Link = s.link(exID)
	return scrum.SprintIssue(d), issueNo, milestoneNo, assignee, nil
}

func (s *Service) createSprintIssue(ctx context.Context, exID uuid.UUID, repo RepoRef, sprintID uuid.UUID, recreated bool) error {
	is, issueNo, milestoneNo, assignee, err := s.sprintIssue(ctx, exID, sprintID)
	if err != nil || issueNo != nil {
		return notFoundIgnored(err)
	}
	if recreated {
		is.Body = recreatedNote + is.Body
	}
	in := IssueInput{Title: is.Title, Body: is.Body, Labels: is.Labels, Assignee: assignee}
	if milestoneNo != nil {
		in.Milestone = *milestoneNo
	}
	n, err := s.issues.CreateIssue(ctx, repo, in)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE sprint SET github_issue_no = $2 WHERE id_sprint = $1 AND github_issue_no IS NULL`, sprintID, n)
	return err
}

func (s *Service) storyIssue(ctx context.Context, exID, storyID uuid.UUID) (scrum.Issue, *int, *int, *uuid.UUID, string, error) {
	var d scrum.StoryIssueData
	var crit []byte
	var issueNo, milestoneNo *int
	var sprintID *uuid.UUID
	var status string
	err := s.db.QueryRow(ctx, `
		SELECT st.titre, st.description, st.criteres, sp.numero, j.ordre, st.points, st.github_issue_no, j.github_milestone_no,
		       st.id_sprint, st.statut_story
		FROM story st JOIN sprint sp USING (id_sprint) JOIN jalon j ON j.id_jalon = sp.id_jalon
		WHERE st.id_story = $1`, storyID).
		Scan(&d.Title, &d.Description, &crit, &d.Sprint, &d.MilestoneOrder, &d.Points, &issueNo, &milestoneNo, &sprintID, &status)
	if err != nil {
		return scrum.Issue{}, nil, nil, nil, "", err
	}
	_ = json.Unmarshal(crit, &d.Criteria)
	d.Link = s.link(exID)
	return scrum.StoryIssue(d), issueNo, milestoneNo, sprintID, status, nil
}

func (s *Service) createStoryIssue(ctx context.Context, exID uuid.UUID, repo RepoRef, storyID uuid.UUID, recreated bool) error {
	is, issueNo, milestoneNo, sprintID, _, err := s.storyIssue(ctx, exID, storyID)
	if err != nil || issueNo != nil {
		return notFoundIgnored(err) // story revenue au backlog : plus d'issue à créer
	}
	if recreated {
		is.Body = recreatedNote + is.Body
	}
	in := IssueInput{Title: is.Title, Body: is.Body, Labels: is.Labels}
	if milestoneNo != nil {
		in.Milestone = *milestoneNo
	}
	n, err := s.issues.CreateIssue(ctx, repo, in)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `UPDATE story SET github_issue_no = $2 WHERE id_story = $1 AND github_issue_no IS NULL`, storyID, n); err != nil {
		return err
	}
	// La checklist de l'issue du sprint référence désormais #n.
	return s.enqueue(ctx, s.db, exID, uniqueKey("maj-sprint:"+sprintID.String()), opUpdateIssue, map[string]any{"type": "sprint", "id": *sprintID})
}

// updateIssue réécrit une issue depuis l'état courant ; une issue supprimée
// sur GitHub est recréée au passage suivant, avec une note.
func (s *Service) updateIssue(ctx context.Context, exID uuid.UUID, repo RepoRef, p outboxPayload) error {
	var is scrum.Issue
	var issueNo *int
	var err error
	if p.Kind == "sprint" {
		is, issueNo, _, _, err = s.sprintIssue(ctx, exID, p.ID)
	} else {
		is, issueNo, _, _, _, err = s.storyIssue(ctx, exID, p.ID)
	}
	if err != nil || issueNo == nil {
		return notFoundIgnored(err)
	}
	err = s.issues.UpdateIssue(ctx, repo, *issueNo, IssueUpdate{Title: is.Title, Body: is.Body, Labels: is.Labels, State: p.State})
	if errors.Is(err, ErrGitHubNotFound) {
		return s.forgetIssue(ctx, s.db, exID, p.Kind, p.ID)
	}
	return err
}

// forgetIssue oublie le numéro d'une issue supprimée et met sa recréation en file.
func (s *Service) forgetIssue(ctx context.Context, q querier, exID uuid.UUID, kind string, id uuid.UUID) error {
	if kind == "sprint" {
		if _, err := q.Exec(ctx, `UPDATE sprint SET github_issue_no = NULL WHERE id_sprint = $1`, id); err != nil {
			return err
		}
		return s.enqueue(ctx, q, exID, uniqueKey("recree-sprint:"+id.String()), opIssueSprint, map[string]any{"id_sprint": id, "recree": true})
	}
	if _, err := q.Exec(ctx, `UPDATE story SET github_issue_no = NULL WHERE id_story = $1`, id); err != nil {
		return err
	}
	return s.enqueue(ctx, q, exID, uniqueKey("recree-story:"+id.String()), opIssueStory, map[string]any{"id_story": id, "recree": true})
}

// enqueueMissing remet en file tout ce qui manque côté GitHub (bouton
// « Relancer la synchro ») : labels, milestones, issues des sprints actifs et
// des stories planifiées.
func (s *Service) enqueueMissing(ctx context.Context, tx pgx.Tx, exID uuid.UUID) error {
	if err := s.enqueue(ctx, tx, exID, uniqueKey("labels:"+exID.String()), opLabels, map[string]any{}); err != nil {
		return err
	}
	type item struct {
		Kind string
		ID   uuid.UUID
	}
	rows, err := tx.Query(ctx, `
		SELECT 'jalon', id_jalon FROM jalon WHERE id_echange = $1 AND github_milestone_no IS NULL
		UNION ALL SELECT 'sprint', id_sprint FROM sprint WHERE id_echange = $1 AND statut_sprint = 'ACTIF' AND github_issue_no IS NULL
		UNION ALL SELECT 'story', id_story FROM story WHERE id_echange = $1 AND id_sprint IS NOT NULL AND github_issue_no IS NULL`, exID)
	if err != nil {
		return err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[item])
	if err != nil {
		return err
	}
	for _, it := range items {
		op, field := opMilestone, "id_jalon"
		switch it.Kind {
		case "sprint":
			op, field = opIssueSprint, "id_sprint"
		case "story":
			op, field = opIssueStory, "id_story"
		}
		if err := s.enqueue(ctx, tx, exID, uniqueKey("relance-"+it.Kind+":"+it.ID.String()), op, map[string]any{field: it.ID}); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Webhooks (retour GitHub → CodeTroc) ----------

type issuesPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number int `json:"number"`
	} `json:"issue"`
	Label *struct {
		Name string `json:"name"`
	} `json:"label"`
	Repository struct {
		ID int64 `json:"id"`
	} `json:"repository"`
}

// handleIssuesEvent applique un événement `issues` déjà authentifié (HMAC) et
// dédoublonné : seule une issue du dépôt d'un échange (même github_repo_id)
// est prise en compte. Renvoie le nombre de stories ou sprints touchés.
func (s *Service) handleIssuesEvent(ctx context.Context, payload []byte) (int, error) {
	var p issuesPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.Repository.ID == 0 || p.Issue.Number <= 0 {
		return 0, nil
	}
	n := 0
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var storyID, exID uuid.UUID
		var status string
		err := tx.QueryRow(ctx, `
			SELECT st.id_story, st.id_echange, st.statut_story FROM story st JOIN echange e USING (id_echange)
			WHERE e.github_repo_id = $1 AND st.github_issue_no = $2 AND e.statut_echange IN ('EN_COURS', 'EN_LITIGE')
			FOR UPDATE OF st`, p.Repository.ID, p.Issue.Number).Scan(&storyID, &exID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			if p.Action != "deleted" {
				return nil
			}
			var sprintID uuid.UUID
			err = tx.QueryRow(ctx, `
				SELECT sp.id_sprint, sp.id_echange FROM sprint sp JOIN echange e USING (id_echange)
				WHERE e.github_repo_id = $1 AND sp.github_issue_no = $2 AND sp.statut_sprint = 'ACTIF'`, p.Repository.ID, p.Issue.Number).
				Scan(&sprintID, &exID)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			n++
			return s.forgetIssue(ctx, tx, exID, "sprint", sprintID)
		}
		if err != nil {
			return err
		}
		var to scrum.StoryStatus
		switch {
		case p.Action == "closed":
			to = scrum.StoryDone
		case p.Action == "reopened":
			to = scrum.StoryDoing
		case p.Action == "labeled" && p.Label != nil && p.Label.Name == "statut:en-revue":
			to = scrum.StoryReview
		case p.Action == "deleted":
			n++
			return s.forgetIssue(ctx, tx, exID, "story", storyID)
		default:
			return nil
		}
		if scrum.CheckStory(scrum.StoryStatus(status), to, scrum.System) != nil {
			return nil
		}
		n++
		return s.setStoryStatus(ctx, tx, storyID, scrum.StoryStatus(status), to, nil, "GitHub : issue "+p.Action)
	})
	return n, err
}

// handleInstallationEvent : quand le propriétaire accepte la nouvelle
// permission issues:write, les projets bloqués reprennent tout de suite.
func (s *Service) handleInstallationEvent(ctx context.Context, payload []byte) error {
	var p struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(payload, &p); err != nil || p.Action != "new_permissions_accepted" {
		return nil
	}
	return s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE outbox_github o SET prochain_essai = $1 FROM projet_scrum p
			WHERE p.id_echange = o.id_echange AND p.statut_synchro = 'PERMISSION_MANQUANTE' AND o.traite_le IS NULL`, s.now()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE projet_scrum SET statut_synchro = 'ACTIVE', detail_synchro = NULL WHERE statut_synchro = 'PERMISSION_MANQUANTE'`)
		return err
	})
}
