package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

// Avenant de planning (D21, R-S10) : une partie propose de nouvelles
// échéances, l'autre accepte ; les sprints à venir sont alors redécoupés.
// Les montants ne changent jamais par cette voie.

var (
	errAmendmentOpen   = conflict("avenant-en-cours", "Un avenant attend déjà la réponse de l'autre partie.")
	errAmendmentClosed = conflict("avenant-traite", "Cet avenant a déjà été accepté, refusé ou retiré.")
	errAmendmentSelf   = forbidden("avenant-propre", "L'avenant doit être accepté par l'autre partie.")
)

// AmendmentView est un avenant de planning.
type AmendmentView struct {
	ID         uuid.UUID
	ProposedBy uuid.UUID
	Deadlines  []time.Time
	Reason     string
	Status     string
	ProposedAt time.Time
	DecidedAt  *time.Time
	DecidedBy  *uuid.UUID
}

const amendmentSelect = `SELECT id_avenant, propose_par, echeances, motif, statut_avenant, date_proposition, date_decision, decide_par FROM avenant`

func scanAmendment(row pgx.Row) (AmendmentView, error) {
	var v AmendmentView
	var ds []byte
	if err := row.Scan(&v.ID, &v.ProposedBy, &ds, &v.Reason, &v.Status, &v.ProposedAt, &v.DecidedAt, &v.DecidedBy); err != nil {
		return v, err
	}
	return v, json.Unmarshal(ds, &v.Deadlines)
}

func (s *Service) openAmendment(ctx context.Context, q querier, exID uuid.UUID) (*AmendmentView, error) {
	v, err := scanAmendment(q.QueryRow(ctx, amendmentSelect+` WHERE id_echange = $1 AND statut_avenant = 'PROPOSE'`, exID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}

// checkAmendment vérifie de nouvelles échéances contre l'état courant :
// une par jalon, croissantes, durée totale ≤ 90 jours, jalons soldés ou livrés
// inchangés, échéances futures, et le sprint actif garde ses dates.
func (s *Service) checkAmendment(ctx context.Context, q querier, exID uuid.UUID, deadlines []time.Time) error {
	ms, err := loadMilestones(ctx, q, exID, false)
	if err != nil {
		return err
	}
	if len(deadlines) != len(ms) {
		return invalid("donnees-invalides", fmt.Sprintf("echeances : une échéance par jalon (%d)", len(ms)))
	}
	var accepted time.Time
	if err := q.QueryRow(ctx, `SELECT date_acceptation FROM echange WHERE id_echange = $1`, exID).Scan(&accepted); err != nil {
		return err
	}
	var activeEnd *time.Time
	var activeMilestone uuid.UUID
	err = q.QueryRow(ctx, `SELECT fin_prevue, id_jalon FROM sprint WHERE id_echange = $1 AND statut_sprint = 'ACTIF'`, exID).Scan(&activeEnd, &activeMilestone)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	changed := false
	prev := accepted
	for i, m := range ms {
		d := deadlines[i]
		if !d.After(prev) {
			return unprocessable("echeances-invalides", "Les échéances doivent être strictement croissantes, après l'acceptation.")
		}
		prev = d
		same := m.Deadline != nil && m.Deadline.Equal(d)
		if !same {
			changed = true
			if m.Status.Final() || m.Status == "LIVRE" || m.Status == "CONTESTE" {
				return unprocessable("echeances-invalides", fmt.Sprintf("Le jalon %d est livré ou soldé : son échéance ne bouge plus.", m.Order))
			}
			if !d.After(s.now()) {
				return unprocessable("echeances-invalides", "Une nouvelle échéance doit être dans le futur.")
			}
		}
		if activeEnd != nil && m.ID == activeMilestone && d.Before(*activeEnd) {
			return unprocessable("echeances-invalides", fmt.Sprintf("Le jalon %d ne peut pas finir avant la fin du sprint en cours.", m.Order))
		}
	}
	if !changed {
		return unprocessable("echeances-invalides", "L'avenant ne change aucune échéance.")
	}
	if prev.Sub(accepted) > 90*24*time.Hour {
		return unprocessable("echeances-invalides", "La durée totale ne peut pas dépasser 90 jours.")
	}
	return nil
}

// ProposeAmendment propose de nouvelles échéances (une partie).
func (s *Service) ProposeAmendment(ctx context.Context, a Actor, exID uuid.UUID, deadlines []time.Time, reason string) (AmendmentView, error) {
	if utf8.RuneCountInString(reason) > 1000 {
		return AmendmentView{}, invalid("donnees-invalides", "motif : 1000 caractères maximum")
	}
	for i := range deadlines {
		deadlines[i] = deadlines[i].UTC()
	}
	var id uuid.UUID
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, role, err := s.scrumAccess(ctx, tx, a, exID, true)
		if err != nil {
			return err
		}
		if role == ScrumArbiter {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM projet_scrum WHERE id_echange = $1 FOR UPDATE`, exID); err != nil {
			return err
		}
		if err := s.checkAmendment(ctx, tx, exID, deadlines); err != nil {
			return err
		}
		ds, _ := json.Marshal(deadlines)
		err = tx.QueryRow(ctx, `
			INSERT INTO avenant (id_echange, propose_par, echeances, motif, date_proposition) VALUES ($1, $2, $3, $4, $5) RETURNING id_avenant`,
			exID, a.ID, ds, reason, s.now()).Scan(&id)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errAmendmentOpen
		}
		if err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "AVENANT", id, "", "PROPOSE", &a.ID, reason, s.now()); err != nil {
			return err
		}
		return s.notify(ctx, tx, NotifAmendmentAsked, &a.ID, &exID, map[string]any{"id_avenant": id}, r.Requester, r.Provider)
	})
	if err != nil {
		return AmendmentView{}, err
	}
	return scanAmendment(s.db.QueryRow(ctx, amendmentSelect+` WHERE id_avenant = $1`, id))
}

// DecideAmendment accepte (l'autre partie) ou écarte un avenant : refusé par
// l'autre partie, retiré par son auteur. À l'acceptation, les échéances
// changent et les sprints à venir sont redécoupés ; le sprint actif garde ses
// dates, les stories des sprints redécoupés retournent au backlog.
func (s *Service) DecideAmendment(ctx context.Context, a Actor, exID, id uuid.UUID, accept bool) (AmendmentView, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, role, err := s.scrumAccess(ctx, tx, a, exID, true)
		if err != nil {
			return err
		}
		if role == ScrumArbiter {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM projet_scrum WHERE id_echange = $1 FOR UPDATE`, exID); err != nil {
			return err
		}
		am, err := scanAmendment(tx.QueryRow(ctx, amendmentSelect+` WHERE id_avenant = $1 AND id_echange = $2 FOR UPDATE`, id, exID))
		if err != nil {
			return notFoundIfNoRows(err)
		}
		if am.Status != "PROPOSE" {
			return errAmendmentClosed
		}
		status := "REFUSE"
		switch {
		case accept && am.ProposedBy == a.ID:
			return errAmendmentSelf
		case accept:
			status = "ACCEPTE"
		case am.ProposedBy == a.ID:
			status = "RETIRE"
		}
		if accept {
			if err := s.checkAmendment(ctx, tx, exID, am.Deadlines); err != nil {
				return err
			}
			if err := s.applyAmendment(ctx, tx, exID, am.Deadlines); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE avenant SET statut_avenant = $2, date_decision = $3, decide_par = $4 WHERE id_avenant = $1`,
			id, status, s.now(), a.ID); err != nil {
			return err
		}
		if err := recordTransition(ctx, tx, "AVENANT", id, "PROPOSE", status, &a.ID, "", s.now()); err != nil {
			return err
		}
		if err := s.notify(ctx, tx, NotifAmendmentDecided, &a.ID, &exID, map[string]any{"id_avenant": id, "statut": status}, r.Requester, r.Provider); err != nil {
			return err
		}
		if accept {
			return s.advanceSprints(ctx, tx, exID)
		}
		return nil
	})
	if err != nil {
		return AmendmentView{}, err
	}
	return scanAmendment(s.db.QueryRow(ctx, amendmentSelect+` WHERE id_avenant = $1`, id))
}

func (s *Service) applyAmendment(ctx context.Context, tx pgx.Tx, exID uuid.UUID, deadlines []time.Time) error {
	ms, err := loadMilestones(ctx, tx, exID, true)
	if err != nil {
		return err
	}
	for i, m := range ms {
		if m.Deadline != nil && m.Deadline.Equal(deadlines[i]) {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE jalon SET echeance = $2 WHERE id_jalon = $1`, m.ID, deadlines[i]); err != nil {
			return err
		}
		if err := s.enqueue(ctx, tx, exID, fmt.Sprintf("milestone:%s:%d", m.ID, deadlines[i].Unix()), opMilestone, map[string]any{"id_jalon": m.ID}); err != nil {
			return err
		}
		ms[i].Deadline = &deadlines[i]
	}
	// Stories des sprints planifiés → backlog, puis redécoupage (R-S10).
	if _, err := tx.Exec(ctx, `
		UPDATE story SET id_sprint = NULL, statut_story = 'BACKLOG'
		WHERE id_sprint IN (SELECT id_sprint FROM sprint WHERE id_echange = $1 AND statut_sprint = 'PLANIFIE')`, exID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sprint WHERE id_echange = $1 AND statut_sprint = 'PLANIFIE'`, exID); err != nil {
		return err
	}
	kept, err := loadSprints(ctx, tx, exID)
	if err != nil {
		return err
	}
	index := map[uuid.UUID]int{}
	closed := make([]bool, len(ms))
	deadlinesNow := make([]time.Time, len(ms))
	for i, m := range ms {
		index[m.ID] = i
		closed[i] = m.Status.Final()
		deadlinesNow[i] = *m.Deadline
	}
	keptPlan := make([]scrum.Sprint, len(kept))
	for i, sp := range kept {
		keptPlan[i] = scrum.Sprint{Number: sp.Number, Milestone: index[sp.MilestoneID], Start: sp.Start, End: sp.End}
	}
	var sprintDays int
	var accepted time.Time
	if err := tx.QueryRow(ctx, `SELECT duree_sprint_jours, date_acceptation FROM echange WHERE id_echange = $1`, exID).Scan(&sprintDays, &accepted); err != nil {
		return err
	}
	plan := scrum.Replan(scrum.MilestoneWindows(accepted, deadlinesNow), keptPlan, closed, sprintDays)
	if err := insertSprints(ctx, tx, exID, ms, plan); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*)::int FROM sprint WHERE id_echange = $1`, exID).Scan(&count); err != nil {
		return err
	}
	return s.enqueue(ctx, tx, exID, fmt.Sprintf("labels:%s:%d", exID, count), opLabels, map[string]any{})
}
