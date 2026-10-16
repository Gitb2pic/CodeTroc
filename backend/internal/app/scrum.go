package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

// F16 — cadre Scrum d'un échange. Le module ne touche jamais au grand livre
// (R-S9) : ni les points, ni les fins de sprint, ni GitHub ne libèrent de crédits.

// Notifications F16.
const (
	NotifSprintEnded      = "SPRINT_TERMINE"
	NotifMilestoneAtRisk  = "JALON_A_RISQUE"
	NotifMilestoneOverdue = "JALON_EN_RETARD"
	NotifAmendmentAsked   = "AVENANT_PROPOSE"
	NotifAmendmentDecided = "AVENANT_DECIDE"
)

// Opérations de l'outbox GitHub (liste fermée, D19).
const (
	opLabels      = "LABELS"
	opMilestone   = "MILESTONE"
	opIssueSprint = "ISSUE_SPRINT"
	opIssueStory  = "ISSUE_STORY"
	opCloseIssue  = "FERMER_ISSUE"
	opUpdateIssue = "MAJ_ISSUE"
)

// enqueue ajoute un appel GitHub à l'outbox, dans la transaction métier. Une
// clé déjà présente est ignorée : l'appel n'est jamais mis deux fois en file.
func (s *Service) enqueue(ctx context.Context, q querier, exID uuid.UUID, key, op string, payload map[string]any) error {
	if s.issues == nil {
		return nil
	}
	b, _ := json.Marshal(payload)
	_, err := q.Exec(ctx, `
		INSERT INTO outbox_github (cle, id_echange, operation, charge, prochain_essai, cree_le)
		VALUES ($1, $2, $3, $4, $5, $5) ON CONFLICT (cle) DO NOTHING`, key, exID, op, b, s.now())
	return err
}

func uniqueKey(prefix string) string {
	id, _ := uuid.NewV7()
	return prefix + ":" + id.String()
}

type sprintRow struct {
	ID          uuid.UUID
	Number      int
	MilestoneID uuid.UUID
	Start, End  time.Time
	Status      scrum.SprintStatus
	Issue       *int
}

func loadSprints(ctx context.Context, q querier, exID uuid.UUID) ([]sprintRow, error) {
	rows, err := q.Query(ctx, `
		SELECT id_sprint, numero, id_jalon, debut_prevu, fin_prevue, statut_sprint, github_issue_no
		FROM sprint WHERE id_echange = $1 ORDER BY numero`, exID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[sprintRow])
}

// createScrumProject crée le projet Scrum d'un échange accepté (R-S1) : projet
// depuis le template, découpage de tous les jalons en sprints (R-S3), labels
// et milestones GitHub mis en file. Le sprint 1 démarre quand le premier
// jalon passe EN_COURS (advanceSprints), dans la même transaction.
func (s *Service) createScrumProject(ctx context.Context, tx pgx.Tx, r exRow) error {
	tpl := scrum.Standard
	dod, _ := json.Marshal(tpl.DefinitionOfDone)
	sync := "INACTIVE"
	if s.issues != nil {
		sync = "ACTIVE"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO projet_scrum (id_echange, template, definition_of_done, statut_synchro, date_creation)
		VALUES ($1, $2, $3, $4, $5)`, r.ID, tpl.Name, dod, sync, s.now()); err != nil {
		return err
	}
	ms, err := loadMilestones(ctx, tx, r.ID, false)
	if err != nil {
		return err
	}
	var sprintDays int
	var accepted time.Time
	if err := tx.QueryRow(ctx, `SELECT duree_sprint_jours, date_acceptation FROM echange WHERE id_echange = $1`, r.ID).Scan(&sprintDays, &accepted); err != nil {
		return err
	}
	deadlines := make([]time.Time, len(ms))
	for i, m := range ms {
		deadlines[i] = *m.Deadline
	}
	plan := scrum.Split(scrum.MilestoneWindows(accepted, deadlines), sprintDays, 1)
	if err := insertSprints(ctx, tx, r.ID, ms, plan); err != nil {
		return err
	}
	if err := s.enqueue(ctx, tx, r.ID, fmt.Sprintf("labels:%s:%d", r.ID, len(plan)), opLabels, map[string]any{}); err != nil {
		return err
	}
	for _, m := range ms {
		if err := s.enqueue(ctx, tx, r.ID, fmt.Sprintf("milestone:%s:%d", m.ID, m.Deadline.Unix()), opMilestone,
			map[string]any{"id_jalon": m.ID}); err != nil {
			return err
		}
	}
	return nil
}

func insertSprints(ctx context.Context, tx pgx.Tx, exID uuid.UUID, ms []msState, plan []scrum.Sprint) error {
	for _, sp := range plan {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sprint (id_echange, id_jalon, numero, debut_prevu, fin_prevue) VALUES ($1, $2, $3, $4, $5)`,
			exID, ms[sp.Milestone].ID, sp.Number, sp.Start, sp.End); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) setSprintStatus(ctx context.Context, tx pgx.Tx, sp sprintRow, to scrum.SprintStatus, reason string) error {
	if err := scrum.CheckSprint(sp.Status, to); err != nil {
		return errTransition
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sprint SET statut_sprint = $2,
		  debut_reel = CASE WHEN $3 THEN $4::timestamptz ELSE debut_reel END,
		  fin_reelle = CASE WHEN $5 THEN $4::timestamptz ELSE fin_reelle END
		WHERE id_sprint = $1`, sp.ID, to, to == scrum.SprintActive, s.now(), sp.Status == scrum.SprintActive); err != nil {
		return err
	}
	return recordTransition(ctx, tx, "SPRINT", sp.ID, string(sp.Status), string(to), nil, reason, s.now())
}

// endSprint clôt le sprint actif : bilan figé, stories non terminées renvoyées
// au backlog (R-S8), issue fermée avec le bilan, rétro demandée (F13).
func (s *Service) endSprint(ctx context.Context, tx pgx.Tx, r exRow, sp sprintRow, to scrum.SprintStatus, reason string) error {
	var planned, done, unfinished int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(points), 0)::int, COALESCE(SUM(points) FILTER (WHERE statut_story = 'TERMINE'), 0)::int,
		       count(*) FILTER (WHERE statut_story <> 'TERMINE')::int
		FROM story WHERE id_sprint = $1`, sp.ID).Scan(&planned, &done, &unfinished); err != nil {
		return err
	}
	if err := s.setSprintStatus(ctx, tx, sp, to, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sprint SET points_prevus = $2, points_faits = $3 WHERE id_sprint = $1`, sp.ID, planned, done); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		UPDATE story SET id_sprint = NULL, statut_story = 'BACKLOG'
		WHERE id_sprint = $1 AND statut_story <> 'TERMINE' RETURNING id_story`, sp.ID)
	if err != nil {
		return err
	}
	back, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, id := range back {
		if err := recordTransition(ctx, tx, "STORY", id, "", string(scrum.StoryBacklog), nil, fmt.Sprintf("fin du sprint %d", sp.Number), s.now()); err != nil {
			return err
		}
	}
	if err := s.enqueue(ctx, tx, r.ID, "fermer-sprint:"+sp.ID.String(), opCloseIssue,
		map[string]any{"type": "sprint", "id": sp.ID, "commentaire": scrum.SprintReview(sp.Number, done, planned, unfinished)}); err != nil {
		return err
	}
	// Seule une fin à la date prévue appelle rétro et planning : une fin anticipée
	// suit la libération du jalon, déjà notifiée (JALON_LIBERE).
	if to != scrum.SprintDone || reason != "" {
		return nil
	}
	return s.notify(ctx, tx, NotifSprintEnded, nil, &r.ID,
		map[string]any{"sprint": sp.Number, "points_faits": done, "points_prevus": planned}, r.Requester, r.Provider)
}

func (s *Service) startSprint(ctx context.Context, tx pgx.Tx, r exRow, sp sprintRow) error {
	if err := s.setSprintStatus(ctx, tx, sp, scrum.SprintActive, ""); err != nil {
		return err
	}
	return s.enqueue(ctx, tx, r.ID, "issue-sprint:"+sp.ID.String(), opIssueSprint, map[string]any{"id_sprint": sp.ID})
}

// advanceSprints fait avancer le cycle des sprints d'un échange (D17,
// R-S5) ; appelé par le worker et par les transitions de jalon et
// d'échange, dans leur transaction :
//   - échange clos : sprints à venir et actif annulés (terminé si l'échange est TERMINE) ;
//   - sprint actif dont la fin est passée ou dont le jalon est soldé : clos ;
//   - sprints planifiés d'un jalon soldé : annulés ;
//   - sans sprint actif : le premier sprint planifié d'un jalon EN_COURS
//     démarre à sa date, ou tout de suite si son jalon vient de démarrer en
//     avance. Les sprints dont la fenêtre est passée pendant l'attente d'un
//     jalon en retard sont annulés, sauf le dernier du jalon.
//
// Sans projet Scrum (échange antérieur à F16), rien ne se passe.
func (s *Service) advanceSprints(ctx context.Context, tx pgx.Tx, exID uuid.UUID) error {
	var locked uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id_echange FROM projet_scrum WHERE id_echange = $1 FOR UPDATE`, exID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	r, err := loadExchange(ctx, tx, exID, false)
	if err != nil {
		return err
	}
	ms, err := loadMilestones(ctx, tx, exID, false)
	if err != nil {
		return err
	}
	msByID := map[uuid.UUID]msState{}
	for _, m := range ms {
		msByID[m.ID] = m
	}
	sprints, err := loadSprints(ctx, tx, exID)
	if err != nil {
		return err
	}
	now := s.now()

	closed := r.Status == exchange.Done || r.Status == exchange.Cancelled || r.Status == exchange.Refused
	for i, sp := range sprints {
		m := msByID[sp.MilestoneID]
		switch {
		case sp.Status == scrum.SprintActive && closed:
			to := scrum.SprintCancelled
			if r.Status == exchange.Done {
				to = scrum.SprintDone
			}
			if err := s.endSprint(ctx, tx, r, sp, to, "échange clos"); err != nil {
				return err
			}
			sprints[i].Status = to
		case sp.Status == scrum.SprintActive && (m.Status.Final() || !now.Before(sp.End)):
			to, reason := scrum.SprintDone, ""
			switch {
			case m.Status == exchange.Refunded:
				to, reason = scrum.SprintCancelled, "jalon remboursé"
			case now.Before(sp.End):
				reason = "jalon soldé avant la fin du sprint"
			}
			if err := s.endSprint(ctx, tx, r, sp, to, reason); err != nil {
				return err
			}
			sprints[i].Status = to
		case sp.Status == scrum.SprintPlanned && (closed || m.Status.Final()):
			if err := s.setSprintStatus(ctx, tx, sp, scrum.SprintCancelled, "jalon soldé ou échange clos"); err != nil {
				return err
			}
			sprints[i].Status = scrum.SprintCancelled
		}
	}
	if closed {
		return s.closeOpenStoryIssues(ctx, tx, exID, nil, "Échange clos sur CodeTroc.")
	}
	for _, sp := range sprints {
		if sp.Status == scrum.SprintActive {
			return nil
		}
	}
	// Premier sprint planifié d'un jalon en cours.
	started := map[uuid.UUID]bool{} // jalon déjà entamé par un sprint
	for _, sp := range sprints {
		if sp.Status == scrum.SprintActive || sp.Status == scrum.SprintDone {
			started[sp.MilestoneID] = true
		}
	}
	for i, sp := range sprints {
		if sp.Status != scrum.SprintPlanned || msByID[sp.MilestoneID].Status != exchange.Active {
			continue
		}
		lastOfMilestone := true
		for _, next := range sprints[i+1:] {
			if next.MilestoneID == sp.MilestoneID && next.Status == scrum.SprintPlanned {
				lastOfMilestone = false
			}
		}
		if !now.Before(sp.End) {
			if lastOfMilestone {
				return nil // jalon en retard : aucun sprint à démarrer, le Gantt le montre
			}
			if err := s.setSprintStatus(ctx, tx, sp, scrum.SprintCancelled, "fenêtre dépassée"); err != nil {
				return err
			}
			continue
		}
		if !now.Before(sp.Start) || !started[sp.MilestoneID] {
			return s.startSprint(ctx, tx, r, sp)
		}
		return nil
	}
	return nil
}

// closeOpenStoryIssues ferme les issues encore ouvertes des stories d'un
// jalon (ou de tout le projet si milestoneID est nil), à la libération du
// jalon (R-S7) ou à la clôture de l'échange.
func (s *Service) closeOpenStoryIssues(ctx context.Context, tx pgx.Tx, exID uuid.UUID, milestoneID *uuid.UUID, comment string) error {
	rows, err := tx.Query(ctx, `
		SELECT st.id_story FROM story st LEFT JOIN sprint sp USING (id_sprint)
		WHERE st.id_echange = $1 AND st.github_issue_no IS NOT NULL AND st.statut_story <> 'TERMINE'
		  AND ($2::uuid IS NULL OR COALESCE(sp.id_jalon, st.id_jalon) = $2)`, exID, milestoneID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.enqueue(ctx, tx, exID, "fermer-story-fin:"+id.String(), opCloseIssue,
			map[string]any{"type": "story", "id": id, "commentaire": comment}); err != nil {
			return err
		}
	}
	return nil
}

// onMilestoneChanged suit une transition de jalon : un jalon soldé ferme ses
// issues de stories ; tout changement relance le cycle des sprints.
func (s *Service) onMilestoneChanged(ctx context.Context, tx pgx.Tx, milestoneID uuid.UUID, to exchange.MilestoneStatus) error {
	var exID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id_echange FROM jalon WHERE id_jalon = $1`, milestoneID).Scan(&exID); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projet_scrum WHERE id_echange = $1)`, exID).Scan(&exists); err != nil || !exists {
		return err
	}
	if to == exchange.Released || to == exchange.Split {
		if err := s.closeOpenStoryIssues(ctx, tx, exID, &milestoneID, "Jalon livré et validé sur CodeTroc."); err != nil {
			return err
		}
	}
	return s.advanceSprints(ctx, tx, exID)
}

// RunScrum fait avancer les sprints arrivés à échéance et signale les jalons
// à risque ou en retard (planificateur). Renvoie le nombre de projets traités.
func (s *Service) RunScrum(ctx context.Context) (int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT sp.id_echange FROM sprint sp JOIN jalon j USING (id_jalon)
		WHERE (sp.statut_sprint = 'ACTIF' AND sp.fin_prevue <= $1)
		   OR (sp.statut_sprint = 'PLANIFIE' AND sp.debut_prevu <= $1 AND j.statut_jalon = 'EN_COURS')`, s.now())
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.tx(ctx, func(tx pgx.Tx) error { return s.advanceSprints(ctx, tx, id) }); err != nil {
			return 0, err
		}
	}
	return len(ids), s.alertMilestones(ctx)
}

// alertMilestones prévient les parties, une seule fois par jalon, quand le
// risque passe ÉLEVÉ et quand l'échéance est dépassée (R-S4).
func (s *Service) alertMilestones(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT j.id_jalon, j.id_echange, j.ordre, j.titre_jalon, j.echeance,
		       COALESCE((SELECT j0.echeance FROM jalon j0 WHERE j0.id_echange = j.id_echange AND j0.ordre = j.ordre - 1), e.date_acceptation),
		       e.id_demandeur, e.id_prestataire
		FROM jalon j JOIN echange e USING (id_echange) JOIN projet_scrum USING (id_echange)
		WHERE j.statut_jalon = 'EN_COURS' AND e.statut_echange = 'EN_COURS'`)
	if err != nil {
		return err
	}
	type msAlert struct {
		ID, ExchangeID      uuid.UUID
		Order               int
		Title               string
		Deadline, Start     time.Time
		Requester, Provider uuid.UUID
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[msAlert])
	if err != nil {
		return err
	}
	now := s.now()
	for _, m := range list {
		done, total, err := milestonePoints(ctx, s.db, m.ID)
		if err != nil {
			return err
		}
		kind := ""
		switch {
		case now.After(m.Deadline):
			kind = NotifMilestoneOverdue
		case scrum.Risk(now, m.Start, m.Deadline, done, total) == scrum.RiskHigh:
			kind = NotifMilestoneAtRisk
		default:
			continue
		}
		err = s.tx(ctx, func(tx pgx.Tx) error {
			var sent bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notification WHERE type = $1 AND donnees->>'id_jalon' = $2)`,
				kind, m.ID.String()).Scan(&sent); err != nil || sent {
				return err
			}
			return s.notify(ctx, tx, kind, nil, &m.ExchangeID,
				map[string]any{"id_jalon": m.ID, "jalon": m.Title, "ordre": m.Order}, m.Requester, m.Provider)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// milestonePoints renvoie les points faits et prévus d'un jalon : stories de
// ses sprints, ou visant ce jalon depuis le backlog.
func milestonePoints(ctx context.Context, q querier, milestoneID uuid.UUID) (done, total int, err error) {
	err = q.QueryRow(ctx, `
		SELECT COALESCE(SUM(st.points) FILTER (WHERE st.statut_story = 'TERMINE'), 0)::int, COALESCE(SUM(st.points), 0)::int
		FROM story st LEFT JOIN sprint sp USING (id_sprint)
		WHERE COALESCE(sp.id_jalon, st.id_jalon) = $1`, milestoneID).Scan(&done, &total)
	return done, total, err
}
