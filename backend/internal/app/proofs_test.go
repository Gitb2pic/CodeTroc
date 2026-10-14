package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

const repoID = int64(100)

func githubID(t *testing.T, e *env, a Actor) int64 {
	t.Helper()
	var id int64
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT github_id FROM utilisateur WHERE id_utilisateur = $1`, a.ID).Scan(&id))
	return id
}

// deliverable prépare un échange accepté et une PR mergée, CI verte, de la prestataire.
type deliverable struct {
	scenario
	x  ExchangeView
	pr PullRequest
}

func newDeliverable(t *testing.T, e *env) deliverable {
	t.Helper()
	sc := newScenario(t, e)
	e.gh.addRepo("acme", "shop-api", repoID)
	x := sc.accepted(t, e)
	pr := PullRequest{ID: 9001, Number: 42, BaseRepoID: repoID, AuthorID: githubID(t, e, sc.lea),
		CreatedAt: e.clock.Now().Add(time.Hour), Merged: true, HeadSHA: "a3f9c1e"}
	e.gh.setPull("acme", "shop-api", pr)
	e.gh.setChecks("a3f9c1e", CheckRun{Name: "lint", Status: "completed", Conclusion: "success"},
		CheckRun{Name: "test (node 18)", Status: "completed", Conclusion: "success"})
	return deliverable{scenario: sc, x: x, pr: pr}
}

func (d deliverable) submit(t *testing.T, e *env, milestone int, prNumber int) ExchangeView {
	t.Helper()
	x, err := e.svc.SubmitProof(context.Background(), d.lea, d.x.Milestones[milestone].ID,
		ProofInput{PRURL: fmt.Sprintf("https://github.com/acme/shop-api/pull/%d", prNumber)})
	require.NoError(t, err)
	return x
}

func lastProof(x ExchangeView, milestone int) ProofView {
	ps := x.Milestones[milestone].Proofs
	return ps[len(ps)-1]
}

// ---------- F7 : preuves ----------

func TestSubmitProof_ValidPRDeliversMilestone(t *testing.T) {
	e := newEnv(t)
	d := newDeliverable(t, e)

	x := d.submit(t, e, 0, 42)
	p := lastProof(x, 0)
	assert.Equal(t, "VALIDE", p.Status)
	assert.Equal(t, "a3f9c1e", *p.CommitSHA)
	assert.Equal(t, exchange.Delivered, x.Milestones[0].Status)
	assert.NotNil(t, x.Milestones[0].DeliveredAt)
	assert.Equal(t, exchange.Active, x.Milestones[1].Status, "RG12 : le jalon suivant démarre")
	assert.Equal(t, exchange.ActorRequester, x.NextActor)
}

func TestSubmitProof_Access(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)
	in := ProofInput{PRURL: "https://github.com/acme/shop-api/pull/42"}

	_, err := e.svc.SubmitProof(ctx, d.sami, d.x.Milestones[0].ID, in)
	requireKind(t, err, KindForbidden, "acces-refuse")
	_, err = e.svc.SubmitProof(ctx, d.tiers, d.x.Milestones[0].ID, in)
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.SubmitProof(ctx, d.lea, d.x.Milestones[1].ID, in)
	requireKind(t, err, KindConflict, "jalon-non-en-cours")
	_, err = e.svc.SubmitProof(ctx, d.lea, d.x.Milestones[0].ID, ProofInput{PRURL: "https://github.com/autre/depot/pull/1"})
	requireKind(t, err, KindUnprocessable, "pr-hors-depot")
	_, err = e.svc.SubmitProof(ctx, d.lea, d.x.Milestones[0].ID, ProofInput{PRURL: "https://github.com/acme/shop-api/issues/1"})
	requireKind(t, err, KindInvalid, "pr-invalide")
}

func TestSubmitProof_Rejections(t *testing.T) {
	cases := map[string]func(*deliverable, *env){
		"autre auteur": func(d *deliverable, e *env) { d.pr.AuthorID = 1; e.gh.setPull("acme", "shop-api", d.pr) },
		"PR antérieure à l'acceptation": func(d *deliverable, e *env) {
			d.pr.CreatedAt = e.clock.Now().Add(-time.Hour)
			e.gh.setPull("acme", "shop-api", d.pr)
		},
		"autre dépôt de base": func(d *deliverable, e *env) { d.pr.BaseRepoID = 7; e.gh.setPull("acme", "shop-api", d.pr) },
		"CI en échec": func(_ *deliverable, e *env) {
			e.gh.setChecks("a3f9c1e", CheckRun{Name: "test", Status: "completed", Conclusion: "failure"})
		},
		"aucun check": func(_ *deliverable, e *env) { e.gh.setChecks("a3f9c1e") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			d := newDeliverable(t, e)
			mutate(&d, e)
			x := d.submit(t, e, 0, 42)
			assert.Equal(t, "REJETEE", lastProof(x, 0).Status)
			assert.Equal(t, exchange.Active, x.Milestones[0].Status, "le jalon reste en cours")
		})
	}
}

func TestSubmitProof_PendingThenReverify(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)
	d.pr.Merged = false
	e.gh.setPull("acme", "shop-api", d.pr)

	x := d.submit(t, e, 0, 42)
	assert.Equal(t, "EN_ATTENTE", lastProof(x, 0).Status)

	d.pr.Merged = true
	e.gh.setPull("acme", "shop-api", d.pr)
	x, err := e.svc.ReverifyMilestone(ctx, d.lea, d.x.Milestones[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "VALIDE", lastProof(x, 0).Status)
	assert.Equal(t, exchange.Delivered, x.Milestones[0].Status)

	_, err = e.svc.ReverifyMilestone(ctx, d.tiers, d.x.Milestones[0].ID)
	requireKind(t, err, KindNotFound, "")
}

func TestSubmitProof_GitHubUnavailableKeepsPending(t *testing.T) {
	e := newEnv(t)
	d := newDeliverable(t, e)
	e.gh.err = errors.New("dial tcp: i/o timeout")
	x := d.submit(t, e, 0, 42)
	p := lastProof(x, 0)
	assert.Equal(t, "EN_ATTENTE", p.Status)
	assert.Contains(t, *p.Detail, "indisponible")

	e.gh.err = nil
	n, err := e.svc.ReconcilePendingProofs(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	x, err = e.svc.GetExchange(context.Background(), d.lea, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Delivered, x.Milestones[0].Status, "la réconciliation rattrape un webhook perdu")
}

func TestSubmitProof_SamePRCannotProveTwoMilestones(t *testing.T) {
	e := newEnv(t)
	d := newDeliverable(t, e)
	d.submit(t, e, 0, 42)
	x := d.submit(t, e, 1, 42)
	p := lastProof(x, 1)
	assert.Equal(t, "REJETEE", p.Status)
	assert.Contains(t, *p.Detail, "déjà")
}

func TestWebhook_TriggersVerificationAndIgnoresDuplicates(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)
	d.pr.Merged = false
	e.gh.setPull("acme", "shop-api", d.pr)
	d.submit(t, e, 0, 42)

	d.pr.Merged = true
	e.gh.setPull("acme", "shop-api", d.pr)
	payload := []byte(`{"action":"closed","number":42,"pull_request":{"number":42,"merged":true},
		"repository":{"id":100,"full_name":"acme/shop-api"}}`)
	res, err := e.svc.HandleGitHubWebhook(ctx, "delivery-1", "pull_request", payload)
	require.NoError(t, err)
	assert.False(t, res.Duplicate)
	assert.Equal(t, 1, res.Verified)

	res, err = e.svc.HandleGitHubWebhook(ctx, "delivery-1", "pull_request", payload)
	require.NoError(t, err)
	assert.True(t, res.Duplicate, "X-GitHub-Delivery déjà vu")

	x, err := e.svc.GetExchange(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Delivered, x.Milestones[0].Status)

	// Un webhook sur un autre dépôt ne touche à rien.
	res, err = e.svc.HandleGitHubWebhook(ctx, "delivery-2", "check_suite",
		[]byte(`{"action":"completed","check_suite":{"head_sha":"zzz","pull_requests":[{"number":42}]},"repository":{"id":7,"full_name":"autre/depot"}}`))
	require.NoError(t, err)
	assert.Zero(t, res.Verified)

	_, err = e.svc.HandleGitHubWebhook(ctx, "delivery-3", "pull_request", []byte(`pas du json`))
	require.NoError(t, err, "un payload illisible est ignoré, pas une erreur serveur")
}

func TestManualProofDeliversDirectly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sami, lea := e.actor(t, e.login(t, "sami")), e.actor(t, e.login(t, "lea"))
	credit(t, e, sami.ID, 100)
	o, err := e.svc.CreateOffer(ctx, lea, OfferInput{PackID: packID(t, e, "Revue de code"), Title: "Revue de sécurité d'une API",
		Price: 40, Days: 2, Milestones: []catalog.Milestone{{Title: "Rapport", Amount: 40}}})
	require.NoError(t, err)
	x, err := e.svc.RequestExchange(ctx, sami, ExchangeRequest{OfferID: o.ID, RepoURL: "https://github.com/acme/api"})
	require.NoError(t, err)
	x, err = e.svc.AcceptExchange(ctx, lea, x.ID)
	require.NoError(t, err)

	_, err = e.svc.SubmitProof(ctx, lea, x.Milestones[0].ID, ProofInput{})
	requireKind(t, err, KindInvalid, "note-requise")
	x, err = e.svc.SubmitProof(ctx, lea, x.Milestones[0].ID, ProofInput{Note: "Rapport joint au fil : 3 points bloquants."})
	require.NoError(t, err)
	assert.Equal(t, exchange.Delivered, x.Milestones[0].Status)
	assert.Equal(t, "MANUELLE", lastProof(x, 0).Kind)
}

// ---------- F8 : validation et libération ----------

func TestValidateMilestone_ReleasesCredits(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)

	_, err := e.svc.ValidateMilestone(ctx, d.sami, d.x.Milestones[0].ID)
	requireKind(t, err, KindConflict, "transition-interdite") // pas encore livré

	d.submit(t, e, 0, 42)
	_, err = e.svc.ValidateMilestone(ctx, d.lea, d.x.Milestones[0].ID)
	requireKind(t, err, KindForbidden, "acces-refuse")
	_, err = e.svc.ValidateMilestone(ctx, d.tiers, d.x.Milestones[0].ID)
	requireKind(t, err, KindNotFound, "")

	x, err := e.svc.ValidateMilestone(ctx, d.sami, d.x.Milestones[0].ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Released, x.Milestones[0].Status)
	assert.Equal(t, EscrowSummary{Locked: 80, Released: 40}, x.Escrow)
	assert.Equal(t, Balance{Available: 90, EscrowIn: 80}, balanceOf(t, e, d.lea))
	assert.Equal(t, Balance{Available: 240, EscrowOut: 80}, balanceOf(t, e, d.sami))

	prof, err := e.svc.PublicProfile(ctx, d.lea.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, prof.VerifiedDeliveries)
	requireLedgerConsistent(t, e)
}

func TestValidateMilestone_LastOneClosesExchange(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)
	for i, n := range []int{42, 43, 44} {
		pr := d.pr
		pr.ID, pr.Number = int64(9001+i), n
		e.gh.setPull("acme", "shop-api", pr)
		d.submit(t, e, i, n)
		_, err := e.svc.ValidateMilestone(ctx, d.sami, d.x.Milestones[i].ID)
		require.NoError(t, err)
	}
	x, err := e.svc.GetExchange(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Done, x.Status)
	assert.NotNil(t, x.ClosedAt)
	assert.Equal(t, EscrowSummary{Locked: 0, Released: 120}, x.Escrow)
	assert.Equal(t, int64(170), balanceOf(t, e, d.lea).Available)
	requireLedgerConsistent(t, e)
}

func TestValidateMilestone_ConcurrentReleaseOnce(t *testing.T) {
	e := newEnv(t)
	d := newDeliverable(t, e)
	d.submit(t, e, 0, 42)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = e.svc.ValidateMilestone(context.Background(), d.sami, d.x.Milestones[0].ID)
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(90), balanceOf(t, e, d.lea).Available)
	requireLedgerConsistent(t, e)
}

func TestTacitValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := newDeliverable(t, e)
	d.submit(t, e, 0, 42)

	e.clock.Advance(71 * time.Hour)
	n, err := e.svc.RunTacitValidation(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)

	e.clock.Advance(2 * time.Hour)
	n, err = e.svc.RunTacitValidation(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = e.svc.RunTacitValidation(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "une seule libération")

	x, err := e.svc.GetExchange(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Released, x.Milestones[0].Status)
	var by *uuid.UUID
	require.NoError(t, e.db.QueryRow(ctx, `SELECT par FROM historique_transition WHERE id_entite = $1 AND vers = 'LIBERE'`, x.Milestones[0].ID).Scan(&by))
	assert.Nil(t, by, "libération attribuée au système")
	requireLedgerConsistent(t, e)
}
