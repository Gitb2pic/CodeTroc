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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/proof"
)

var (
	errPRURL           = invalid("pr-invalide", proof.ErrPRURL.Error())
	errPROutsideRepo   = unprocessable("pr-hors-depot", "La PR doit appartenir au dépôt de l'échange.")
	errNotActive       = conflict("jalon-non-en-cours", "Seul le jalon en cours peut être livré (RG12).")
	errNoteRequired    = invalid("note-requise", "Une note de livraison (1 à 2000 caractères) est requise pour ce pack.")
	detailNoGitHub     = "Vérification GitHub non configurée sur ce serveur."
	detailNotInstalled = "PR introuvable, ou GitHub App CodeTroc non installée sur le dépôt."
	detailUnavailable  = "GitHub indisponible : nouvel essai automatique."
	detailManual       = "Livraison déclarée : à valider par le demandeur."
)

// ProofView est une preuve de livraison.
type ProofView struct {
	ID        uuid.UUID
	Kind      string
	PRURL     *string
	PRNumber  *int
	CommitSHA *string
	Status    string
	Detail    *string
	Note      *string
	CheckedAt *time.Time
	CreatedAt time.Time
}

// ProofInput est une livraison : URL de PR (pack PR_CI) ou note (pack MANUELLE).
type ProofInput struct {
	PRURL string
	Note  string
}

// msRow est un jalon chargé avec l'identifiant de son échange.
type msRow struct {
	ID         uuid.UUID
	ExchangeID uuid.UUID
	Order      int
	Amount     int64
	Status     exchange.MilestoneStatus
}

// lockMilestone verrouille l'échange puis le jalon (ordre fixe : échange, jalon, preuve).
func lockMilestone(ctx context.Context, tx pgx.Tx, a *Actor, milestoneID uuid.UUID) (exRow, msRow, error) {
	var exID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id_echange FROM jalon WHERE id_jalon = $1`, milestoneID).Scan(&exID); err != nil {
		return exRow{}, msRow{}, notFoundIfNoRows(err)
	}
	r, err := loadExchange(ctx, tx, exID, true)
	if err != nil {
		return r, msRow{}, err
	}
	if a != nil && !r.isParty(a.ID) {
		return r, msRow{}, ErrNotFound
	}
	var m msRow
	err = tx.QueryRow(ctx, `SELECT id_jalon, id_echange, ordre, montant_jalon, statut_jalon FROM jalon WHERE id_jalon = $1 FOR UPDATE`, milestoneID).
		Scan(&m.ID, &m.ExchangeID, &m.Order, &m.Amount, &m.Status)
	return r, m, notFoundIfNoRows(err)
}

// SubmitProof enregistre la livraison du jalon en cours (UC09). Une PR est
// vérifiée aussitôt auprès de GitHub ; une livraison manuelle passe le jalon
// à LIVRE directement.
func (s *Service) SubmitProof(ctx context.Context, a Actor, milestoneID uuid.UUID, in ProofInput) (ExchangeView, error) {
	var proofID uuid.UUID
	var exID uuid.UUID
	var needsCheck bool
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, m, err := lockMilestone(ctx, tx, &a, milestoneID)
		if err != nil {
			return err
		}
		exID = r.ID
		if a.ID != r.Provider {
			return errAccessDenie
		}
		if r.Status != exchange.InProgress && r.Status != exchange.InDispute {
			return errTransition
		}
		if m.Status != exchange.Active {
			return errNotActive
		}
		if r.ProofType == catalog.ProofManual {
			note := strings.TrimSpace(in.Note)
			if note == "" || utf8.RuneCountInString(note) > 2000 {
				return errNoteRequired
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO preuve (id_jalon, type_preuve, note, statut_preuve, detail_verif, date_verif, date_creation, soumis_par)
				VALUES ($1, 'MANUELLE', $2, 'VALIDE', $3, $4, $4, $5)`, m.ID, note, detailManual, s.now(), a.ID); err != nil {
				return err
			}
			return s.deliverMilestone(ctx, tx, r, m, &a.ID)
		}
		ref, err := proof.ParsePRURL(in.PRURL)
		if err != nil {
			return errPRURL
		}
		if !strings.EqualFold(ref.Owner, r.Repo.Owner) || !strings.EqualFold(ref.Repo, r.Repo.Name) {
			return errPROutsideRepo
		}
		needsCheck = true
		return tx.QueryRow(ctx, `
			INSERT INTO preuve (id_jalon, type_preuve, pr_owner, pr_repo, pr_numero, date_creation, soumis_par)
			VALUES ($1, 'PR_CI', $2, $3, $4, $5, $6) RETURNING id_preuve`,
			m.ID, ref.Owner, ref.Repo, ref.Number, s.now(), a.ID).Scan(&proofID)
	})
	if err != nil {
		return ExchangeView{}, err
	}
	if needsCheck {
		if err := s.verifyProof(ctx, proofID); err != nil {
			return ExchangeView{}, err
		}
	}
	return s.GetExchange(ctx, a, exID)
}

// deliverMilestone passe le jalon à LIVRE et démarre le suivant (RG12).
func (s *Service) deliverMilestone(ctx context.Context, tx pgx.Tx, r exRow, m msRow, by *uuid.UUID) error {
	if err := s.setMilestoneStatus(ctx, tx, m.ID, m.Status, exchange.Delivered, by, ""); err != nil {
		return err
	}
	var next uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id_jalon FROM jalon WHERE id_echange = $1 AND ordre = $2 AND statut_jalon = 'EN_ATTENTE' FOR UPDATE`, r.ID, m.Order+1).Scan(&next)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.setMilestoneStatus(ctx, tx, next, exchange.Pending, exchange.Active, by, "jalon précédent livré")
}

// verifyProof interroge GitHub puis applique le verdict (RG13, D6). Les
// erreurs GitHub laissent la preuve EN_ATTENTE : le worker la revérifie.
func (s *Service) verifyProof(ctx context.Context, proofID uuid.UUID) error {
	var (
		exID             uuid.UUID
		status           string
		owner, repo      string
		number           int
		repoID           *int64
		acceptedAt       *time.Time
		providerGitHubID int64
		exOwner, exName  string
	)
	err := s.db.QueryRow(ctx, `
		SELECT e.id_echange, p.statut_preuve, p.pr_owner, p.pr_repo, p.pr_numero, e.github_repo_id, e.date_acceptation,
		       u.github_id, e.depot_owner, e.depot_nom
		FROM preuve p JOIN jalon j USING (id_jalon) JOIN echange e USING (id_echange)
		JOIN utilisateur u ON u.id_utilisateur = e.id_prestataire
		WHERE p.id_preuve = $1 AND p.type_preuve = 'PR_CI'`, proofID).
		Scan(&exID, &status, &owner, &repo, &number, &repoID, &acceptedAt, &providerGitHubID, &exOwner, &exName)
	if err != nil {
		return notFoundIfNoRows(err)
	}
	if status == string(proof.Valid) || acceptedAt == nil {
		return nil
	}
	if s.github == nil {
		return s.recordVerdict(ctx, proofID, proof.Verdict{Status: proof.Pending, Detail: detailNoGitHub}, nil)
	}

	pending := func(err error) error {
		d := detailUnavailable
		if errors.Is(err, ErrGitHubNotFound) {
			d = detailNotInstalled
		}
		return s.recordVerdict(ctx, proofID, proof.Verdict{Status: proof.Pending, Detail: d}, nil)
	}
	if repoID == nil {
		rp, err := s.github.Repository(ctx, exOwner, exName)
		if err != nil {
			return pending(err)
		}
		if _, err := s.db.Exec(ctx, `UPDATE echange SET github_repo_id = $2 WHERE id_echange = $1 AND github_repo_id IS NULL`, exID, rp.ID); err != nil {
			return err
		}
		repoID = &rp.ID
	}
	pr, err := s.github.PullRequest(ctx, owner, repo, number)
	if err != nil {
		return pending(err)
	}
	var checks []CheckRun
	if pr.Merged {
		if checks, err = s.github.CheckRuns(ctx, owner, repo, pr.HeadSHA); err != nil {
			return pending(err)
		}
	}
	var used bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM preuve WHERE github_pr_id = $1 AND statut_preuve = 'VALIDE' AND id_preuve <> $2)`, pr.ID, proofID).Scan(&used); err != nil {
		return err
	}
	facts := proof.Facts{RepoID: *repoID, ProviderGitHubID: providerGitHubID, AcceptedAt: *acceptedAt, AlreadyUsed: used,
		PR: proof.PR{ID: pr.ID, BaseRepoID: pr.BaseRepoID, AuthorID: pr.AuthorID, CreatedAt: pr.CreatedAt, Merged: pr.Merged, HeadSHA: pr.HeadSHA}}
	for _, c := range checks {
		facts.Checks = append(facts.Checks, proof.Check{Name: c.Name, Status: c.Status, Conclusion: c.Conclusion})
	}
	err = s.recordVerdict(ctx, proofID, proof.Evaluate(facts), &pr)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // même PR validée en parallèle pour un autre jalon
		return s.recordVerdict(ctx, proofID, proof.Verdict{Status: proof.Rejected, Detail: "Cette PR a déjà servi de preuve pour un autre jalon."}, &pr)
	}
	return err
}

// recordVerdict enregistre le résultat et livre le jalon si la preuve est valide.
func (s *Service) recordVerdict(ctx context.Context, proofID uuid.UUID, v proof.Verdict, pr *PullRequest) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		var msID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id_jalon FROM preuve WHERE id_preuve = $1`, proofID).Scan(&msID); err != nil {
			return err
		}
		r, m, err := lockMilestone(ctx, tx, nil, msID)
		if err != nil {
			return err
		}
		var current string
		if err := tx.QueryRow(ctx, `SELECT statut_preuve FROM preuve WHERE id_preuve = $1 FOR UPDATE`, proofID).Scan(&current); err != nil {
			return err
		}
		if current == string(proof.Valid) {
			return nil
		}
		if v.Status == proof.Valid && m.Status != exchange.Active {
			v = proof.Verdict{Status: proof.Rejected, Detail: "Le jalon n'est plus en cours."}
		}
		var prID *int64
		var sha *string
		if pr != nil {
			prID, sha = &pr.ID, &pr.HeadSHA
		}
		if _, err := tx.Exec(ctx, `
			UPDATE preuve SET statut_preuve = $2, detail_verif = $3, date_verif = $4,
			  github_pr_id = COALESCE($5, github_pr_id), sha_commit = COALESCE($6, sha_commit)
			WHERE id_preuve = $1`, proofID, v.Status, v.Detail, s.now(), prID, sha); err != nil {
			return err
		}
		if v.Status == proof.Valid {
			return s.deliverMilestone(ctx, tx, r, m, nil)
		}
		return nil
	})
}

// ReverifyMilestone relance la vérification des preuves non validées du jalon
// (bouton « Revérifier », UC09 4c).
func (s *Service) ReverifyMilestone(ctx context.Context, a Actor, milestoneID uuid.UUID) (ExchangeView, error) {
	var exID uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT id_echange FROM jalon WHERE id_jalon = $1`, milestoneID).Scan(&exID); err != nil {
		return ExchangeView{}, notFoundIfNoRows(err)
	}
	if _, err := requireParty(ctx, s.db, a, exID, false); err != nil {
		return ExchangeView{}, err
	}
	rows, err := s.db.Query(ctx, `SELECT id_preuve FROM preuve WHERE id_jalon = $1 AND type_preuve = 'PR_CI' AND statut_preuve <> 'VALIDE'`, milestoneID)
	if err != nil {
		return ExchangeView{}, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return ExchangeView{}, err
	}
	for _, id := range ids {
		if err := s.verifyProof(ctx, id); err != nil {
			return ExchangeView{}, err
		}
	}
	return s.GetExchange(ctx, a, exID)
}

// ReconcilePendingProofs revérifie les preuves EN_ATTENTE depuis plus de
// olderThan (webhooks perdus, GitHub indisponible). Exécuté par le worker.
func (s *Service) ReconcilePendingProofs(ctx context.Context, olderThan time.Duration) (int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.id_preuve FROM preuve p JOIN jalon j USING (id_jalon)
		WHERE p.statut_preuve = 'EN_ATTENTE' AND p.type_preuve = 'PR_CI' AND j.statut_jalon = 'EN_COURS'
		  AND p.date_creation <= $1
		ORDER BY p.date_creation LIMIT 50`, s.now().Add(-olderThan))
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.verifyProof(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// WebhookResult résume le traitement d'un webhook.
type WebhookResult struct {
	Duplicate bool
	Verified  int
}

type webhookPayload struct {
	Number      int `json:"number"`
	PullRequest *struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	CheckSuite *checkPayload `json:"check_suite"`
	CheckRun   *checkPayload `json:"check_run"`
	Repository struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type checkPayload struct {
	HeadSHA      string `json:"head_sha"`
	PullRequests []struct {
		Number int `json:"number"`
	} `json:"pull_requests"`
}

// HandleGitHubWebhook traite un webhook déjà authentifié (signature HMAC).
// Le contenu ne sert qu'à repérer les preuves concernées : la décision vient
// toujours d'un appel authentifié à l'API GitHub.
func (s *Service) HandleGitHubWebhook(ctx context.Context, deliveryID, event string, payload []byte) (WebhookResult, error) {
	tag, err := s.db.Exec(ctx, `INSERT INTO webhook_livraison (id_livraison, evenement, recu_le) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		deliveryID, truncate(event, 50), s.now())
	if err != nil {
		return WebhookResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return WebhookResult{Duplicate: true}, nil
	}
	var p webhookPayload
	if err := json.Unmarshal(payload, &p); err != nil || (p.Repository.ID == 0 && p.Repository.FullName == "") {
		return WebhookResult{}, nil
	}
	var numbers []int
	var sha string
	switch {
	case p.PullRequest != nil:
		numbers = append(numbers, p.PullRequest.Number)
	case p.CheckSuite != nil:
		sha = p.CheckSuite.HeadSHA
		for _, pr := range p.CheckSuite.PullRequests {
			numbers = append(numbers, pr.Number)
		}
	case p.CheckRun != nil:
		sha = p.CheckRun.HeadSHA
		for _, pr := range p.CheckRun.PullRequests {
			numbers = append(numbers, pr.Number)
		}
	}
	if len(numbers) == 0 && sha == "" {
		return WebhookResult{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT p.id_preuve FROM preuve p JOIN jalon j USING (id_jalon) JOIN echange e USING (id_echange)
		WHERE p.statut_preuve <> 'VALIDE' AND p.type_preuve = 'PR_CI' AND j.statut_jalon = 'EN_COURS'
		  AND (e.github_repo_id = $1 OR lower(e.depot_owner || '/' || e.depot_nom) = lower($2))
		  AND (p.pr_numero = ANY($3) OR ($4 <> '' AND p.sha_commit = $4))`, p.Repository.ID, p.Repository.FullName, numbers, sha)
	if err != nil {
		return WebhookResult{}, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return WebhookResult{}, err
	}
	for _, id := range ids {
		if err := s.verifyProof(ctx, id); err != nil {
			return WebhookResult{}, err
		}
	}
	return WebhookResult{Verified: len(ids)}, nil
}

// attachProofs ajoute les preuves aux jalons d'un échange.
func (s *Service) attachProofs(ctx context.Context, x *ExchangeView) error {
	byMilestone := map[uuid.UUID][]ProofView{}
	rows, err := s.db.Query(ctx, `
		SELECT p.id_jalon, p.id_preuve, p.type_preuve::text,
		       CASE WHEN p.pr_numero IS NULL THEN NULL
		            ELSE 'https://github.com/' || p.pr_owner || '/' || p.pr_repo || '/pull/' || p.pr_numero END,
		       p.pr_numero, p.sha_commit, p.statut_preuve::text, p.detail_verif, p.note, p.date_verif, p.date_creation
		FROM preuve p JOIN jalon j USING (id_jalon)
		WHERE j.id_echange = $1 ORDER BY p.date_creation, p.id_preuve`, x.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ms uuid.UUID
		var p ProofView
		if err := rows.Scan(&ms, &p.ID, &p.Kind, &p.PRURL, &p.PRNumber, &p.CommitSHA, &p.Status, &p.Detail, &p.Note, &p.CheckedAt, &p.CreatedAt); err != nil {
			return err
		}
		byMilestone[ms] = append(byMilestone[ms], p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range x.Milestones {
		x.Milestones[i].Proofs = byMilestone[x.Milestones[i].ID]
		if x.Milestones[i].Proofs == nil {
			x.Milestones[i].Proofs = []ProofView{}
		}
	}
	return nil
}
