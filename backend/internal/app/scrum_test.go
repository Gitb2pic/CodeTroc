package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

const scrumRepoID = int64(4242)

// scrumScenario : Léa propose 120 cr en 30 jours, 2 jalons, sprints de 7 jours ;
// Sami accepte. Sprints attendus : J0→J7, J7→J15 (jalon 1), J15→J22, J22→J30 (jalon 2).
type scrumScenario struct {
	sami, lea, tiers Actor
	x                ExchangeView
	gh               *fakeIssues
}

func newScrumEnv(t *testing.T) (*env, *fakeIssues) {
	gh := newFakeIssues()
	e := newEnv(t, func(d *Deps) { d.Issues = gh; d.AppURL = "https://codetroc.test/" })
	e.gh.addRepo("acme", "shop-api", scrumRepoID)
	return e, gh
}

func newScrumScenario(t *testing.T, e *env, gh *fakeIssues) scrumScenario {
	t.Helper()
	ctx := context.Background()
	sc := scrumScenario{sami: e.actor(t, e.login(t, "sami")), lea: e.actor(t, e.login(t, "lea")), tiers: e.actor(t, e.login(t, "tiers")), gh: gh}
	credit(t, e, sc.sami.ID, 310)
	in := offerInput(t, e)
	in.Days = 30
	in.Milestones = []catalog.Milestone{{Title: "Maquette", Amount: 60}, {Title: "API", Amount: 60}}
	o, err := e.svc.CreateOffer(ctx, sc.lea, in)
	require.NoError(t, err)
	x, err := e.svc.RequestExchange(ctx, sc.sami, ExchangeRequest{OfferID: o.ID, RepoURL: "https://github.com/acme/shop-api"})
	require.NoError(t, err)
	sc.x, err = e.svc.AcceptExchange(ctx, sc.lea, x.ID)
	require.NoError(t, err)
	return sc
}

func (sc scrumScenario) sprints(t *testing.T, e *env) []SprintView {
	t.Helper()
	ss, err := e.svc.ListSprints(context.Background(), sc.sami, sc.x.ID)
	require.NoError(t, err)
	return ss
}

func statuses(ss []SprintView) []scrum.SprintStatus {
	out := make([]scrum.SprintStatus, len(ss))
	for i, s := range ss {
		out[i] = s.Status
	}
	return out
}

func (sc scrumScenario) story(t *testing.T, e *env, title string, sprint *int) StoryView {
	t.Helper()
	v, err := e.svc.CreateStory(context.Background(), sc.sami, sc.x.ID, StoryInput{Title: title, Criteria: []string{"Given/When/Then"}, SprintNumber: sprint})
	require.NoError(t, err)
	return v
}

func (sc scrumScenario) points(t *testing.T, e *env, id uuid.UUID, p int) {
	t.Helper()
	_, err := e.svc.PatchStory(context.Background(), sc.lea, sc.x.ID, id, StoryPatch{Points: &p})
	require.NoError(t, err)
}

func (sc scrumScenario) move(t *testing.T, e *env, by Actor, id uuid.UUID, to scrum.StoryStatus) {
	t.Helper()
	_, err := e.svc.PatchStory(context.Background(), by, sc.x.ID, id, StoryPatch{Status: &to})
	require.NoError(t, err)
}

func outboxOps(t *testing.T, e *env, exID uuid.UUID) []string {
	t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT operation FROM outbox_github WHERE id_echange = $1 ORDER BY id`, exID)
	require.NoError(t, err)
	var ops []string
	for rows.Next() {
		var op string
		require.NoError(t, rows.Scan(&op))
		ops = append(ops, op)
	}
	return ops
}

func runJobs(t *testing.T, e *env) JobReport {
	t.Helper()
	r, err := e.svc.RunScheduledJobs(context.Background())
	require.NoError(t, err)
	require.True(t, r.Ran)
	return r
}

func intp(i int) *int { return &i }

func TestScrum_ProjectCreatedAtAcceptance(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)

	p, err := e.svc.GetScrum(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	assert.Equal(t, ScrumPO, p.Role)
	assert.Equal(t, "scrum-standard-v1", p.Template)
	assert.Equal(t, scrum.Standard.DefinitionOfDone, p.DefinitionOfDone)
	assert.Equal(t, 7, p.SprintDays)
	assert.Equal(t, "ACTIVE", p.SyncStatus)

	ss := sc.sprints(t, e)
	require.Len(t, ss, 4)
	assert.Equal(t, []scrum.SprintStatus{scrum.SprintActive, scrum.SprintPlanned, scrum.SprintPlanned, scrum.SprintPlanned}, statuses(ss))
	acc := *sc.x.AcceptedAt
	assert.Equal(t, acc, ss[0].Start)
	assert.Equal(t, acc.AddDate(0, 0, 15), ss[1].End, "dernier sprint du jalon 1 = échéance du jalon")
	assert.Equal(t, acc.AddDate(0, 0, 30), ss[3].End)
	assert.Equal(t, []int{1, 1, 2, 2}, []int{ss[0].MilestoneOrder, ss[1].MilestoneOrder, ss[2].MilestoneOrder, ss[3].MilestoneOrder})

	// Outbox de la transaction d'acceptation : labels, un milestone par jalon, issue du sprint 1.
	assert.Equal(t, []string{opLabels, opMilestone, opMilestone, opIssueSprint}, outboxOps(t, e, sc.x.ID))

	// Un échange refusé n'a pas de projet.
	o, err := e.svc.CreateOffer(ctx, sc.lea, offerInput(t, e))
	require.NoError(t, err)
	x2, err := e.svc.RequestExchange(ctx, sc.sami, ExchangeRequest{OfferID: o.ID, RepoURL: "https://github.com/acme/b"})
	require.NoError(t, err)
	_, err = e.svc.RefuseExchange(ctx, sc.lea, x2.ID)
	require.NoError(t, err)
	_, err = e.svc.GetScrum(ctx, sc.sami, x2.ID)
	requireKind(t, err, KindNotFound, "projet-scrum-absent")
	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM sprint WHERE id_echange = $1`, x2.ID).Scan(&n))
	assert.Zero(t, n)
}

func TestScrum_GitHubSync(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	st := sc.story(t, e, "Connexion @octocat GitHub", intp(1))
	sc.points(t, e, st.ID, 5)

	runJobs(t, e)
	assert.True(t, gh.onlyRepo(scrumRepoID), "jamais un autre dépôt")
	for _, l := range []string{"codetroc", "type:sprint", "jalon:2", "sprint:4", "points:21", "statut:en-revue"} {
		assert.True(t, gh.hasLabel(l), l)
	}
	ms, err := e.svc.GetPlanning(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	require.NotNil(t, ms.Milestones[0].GitHubNo)
	assert.Equal(t, sc.x.Milestones[0].Deadline.UTC(), gh.due[*ms.Milestones[0].GitHubNo].UTC())

	ss := sc.sprints(t, e)
	require.NotNil(t, ss[0].Issue)
	sprintIssue := gh.issue(*ss[0].Issue)
	assert.Equal(t, "[CodeTroc] Sprint 1 — Jalon 1 : Maquette", sprintIssue.Title)
	assert.Equal(t, "lea", sprintIssue.Assignee)
	assert.ElementsMatch(t, []string{"codetroc", "type:sprint", "sprint:1", "jalon:1"}, sprintIssue.Labels)
	assert.Equal(t, *ms.Milestones[0].GitHubNo, sprintIssue.Milestone)
	assert.Contains(t, sprintIssue.Body, "https://codetroc.test/echanges/"+sc.x.ID.String())

	stories, err := e.svc.ListStories(ctx, sc.lea, sc.x.ID, StoryFilter{})
	require.NoError(t, err)
	require.NotNil(t, stories[0].Issue)
	storyIssue := gh.issue(*stories[0].Issue)
	assert.Equal(t, "Connexion @\u200boctocat GitHub", storyIssue.Title, "mention neutralisée")
	assert.Contains(t, storyIssue.Labels, "points:5")
	// L'issue du sprint référence la story une fois son issue créée.
	runJobs(t, e)
	assert.Contains(t, gh.issue(*ss[0].Issue).Body, "#"+itoa(*stories[0].Issue)+" Connexion")

	var pending int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM outbox_github WHERE traite_le IS NULL`).Scan(&pending))
	assert.Zero(t, pending)
}

func TestScrum_WorkerStartsAndEndsSprints(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	done := sc.story(t, e, "Écran d'accueil", intp(1))
	left := sc.story(t, e, "Écran de profil", intp(1))
	sc.points(t, e, done.ID, 3)
	sc.points(t, e, left.ID, 5)
	sc.move(t, e, sc.lea, done.ID, scrum.StoryDoing)
	sc.move(t, e, sc.lea, done.ID, scrum.StoryReview)
	sc.move(t, e, sc.sami, done.ID, scrum.StoryDone)
	runJobs(t, e)

	e.clock.Advance(7 * 24 * time.Hour)
	r := runJobs(t, e)
	assert.Equal(t, 1, r.ScrumProjects)
	ss := sc.sprints(t, e)
	assert.Equal(t, []scrum.SprintStatus{scrum.SprintDone, scrum.SprintActive, scrum.SprintPlanned, scrum.SprintPlanned}, statuses(ss))
	assert.Equal(t, 8, ss[0].PointsPlanned)
	assert.Equal(t, 3, ss[0].PointsDone)

	// R-S8 : la story non terminée retourne au backlog.
	backlog, err := e.svc.ListStories(ctx, sc.sami, sc.x.ID, StoryFilter{Backlog: true})
	require.NoError(t, err)
	require.Len(t, backlog, 1)
	assert.Equal(t, left.ID, backlog[0].ID)
	assert.Equal(t, scrum.StoryBacklog, backlog[0].Status)

	// Bilan sur l'issue du sprint 1, fermée ; issue du sprint 2 créée.
	is := gh.issue(*ss[0].Issue)
	assert.Equal(t, "closed", is.State)
	require.Len(t, is.Comments, 1)
	assert.Contains(t, is.Comments[0], "3 points faits sur 8 prévus")
	require.NotNil(t, ss[1].Issue)

	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM notification WHERE type = 'SPRINT_TERMINE' AND id_echange = $1`, sc.x.ID).Scan(&n))
	assert.Equal(t, 2, n, "rétro demandée aux deux parties")

	// Rétro : sprint terminé seulement.
	_, err = e.svc.SaveRetro(ctx, sc.lea, sc.x.ID, 2, Retro{Good: "x"})
	requireKind(t, err, KindConflict, "sprint-non-termine")
	v, err := e.svc.SaveRetro(ctx, sc.lea, sc.x.ID, 1, Retro{Good: "Rythme", Improve: "Estimations", Actions: "Découper"})
	require.NoError(t, err)
	assert.Equal(t, "Rythme", *v.RetroGood)

	// Jalon 1 en retard : à la fin du sprint 2, le sprint 3 (jalon 2) ne démarre pas.
	e.clock.Advance(8*24*time.Hour + time.Hour)
	runJobs(t, e)
	assert.Equal(t, []scrum.SprintStatus{scrum.SprintDone, scrum.SprintDone, scrum.SprintPlanned, scrum.SprintPlanned}, statuses(sc.sprints(t, e)))
	p, err := e.svc.GetPlanning(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	assert.True(t, p.Milestones[0].Late)
	assert.False(t, p.Milestones[1].Late)
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM notification WHERE type = 'JALON_EN_RETARD'`).Scan(&n))
	assert.Equal(t, 2, n)
	runJobs(t, e)
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM notification WHERE type = 'JALON_EN_RETARD'`).Scan(&n))
	assert.Equal(t, 2, n, "une seule alerte par jalon")
}

// deliverMilestone livre le jalon i par une PR mergée et CI verte, puis le fait valider.
func (sc scrumScenario) deliverMilestone(t *testing.T, e *env, i, pr int) {
	t.Helper()
	ctx := context.Background()
	sha := "sha" + itoa(pr)
	e.gh.setPull("acme", "shop-api", PullRequest{ID: int64(9000 + pr), Number: pr, BaseRepoID: scrumRepoID, AuthorID: githubID(t, e, sc.lea),
		CreatedAt: e.clock.Now().Add(time.Hour), Merged: true, HeadSHA: sha})
	e.gh.setChecks(sha, CheckRun{Name: "test", Status: "completed", Conclusion: "success"})
	_, err := e.svc.SubmitProof(ctx, sc.lea, sc.x.Milestones[i].ID, ProofInput{PRURL: "https://github.com/acme/shop-api/pull/" + itoa(pr)})
	require.NoError(t, err)
	_, err = e.svc.ValidateMilestone(ctx, sc.sami, sc.x.Milestones[i].ID)
	require.NoError(t, err)
}

func TestScrum_EarlyReleaseStartsNextMilestone(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	st := sc.story(t, e, "Maquette Figma", intp(1))
	runJobs(t, e)

	e.clock.Advance(3 * 24 * time.Hour)
	sc.deliverMilestone(t, e, 0, 7)

	ss := sc.sprints(t, e)
	assert.Equal(t, []scrum.SprintStatus{scrum.SprintDone, scrum.SprintCancelled, scrum.SprintActive, scrum.SprintPlanned}, statuses(ss))
	require.NotNil(t, ss[2].StartedAt)
	assert.Equal(t, e.clock.Now(), ss[2].StartedAt.UTC(), "démarré tout de suite")
	assert.True(t, ss[2].Start.After(e.clock.Now()), "dates prévues inchangées")

	x, err := e.svc.GetExchange(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	assert.Equal(t, sc.x.Milestones[1].Deadline, x.Milestones[1].Deadline, "échéances contractuelles inchangées")

	// R-S7 : à la libération, les issues ouvertes du jalon sont fermées.
	runJobs(t, e)
	v, err := e.svc.ListStories(ctx, sc.sami, sc.x.ID, StoryFilter{})
	require.NoError(t, err)
	assert.Equal(t, st.ID, v[0].ID)
	assert.Equal(t, "closed", gh.issue(*v[0].Issue).State)
	var notif int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM notification WHERE type = 'SPRINT_TERMINE'`).Scan(&notif))
	assert.Zero(t, notif, "fin anticipée : la libération est déjà notifiée")
}

func TestScrum_CancelClosesEverything(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	st := sc.story(t, e, "Login", intp(1))
	runJobs(t, e)

	_, err := e.svc.CancelExchange(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	_, err = e.svc.CancelExchange(ctx, sc.lea, sc.x.ID)
	require.NoError(t, err)
	ss := sc.sprints(t, e)
	for _, s := range ss {
		assert.Equal(t, scrum.SprintCancelled, s.Status)
	}
	runJobs(t, e)
	assert.Equal(t, "closed", gh.issue(*ss[0].Issue).State)
	v, err := e.svc.ListStories(ctx, sc.sami, sc.x.ID, StoryFilter{})
	require.NoError(t, err)
	require.Equal(t, st.ID, v[0].ID)
	_, err = e.svc.CreateStory(ctx, sc.sami, sc.x.ID, StoryInput{Title: "Encore une"})
	requireKind(t, err, KindConflict, "echange-clos")
	requireLedgerConsistent(t, e)
}

func TestScrum_ConcurrentWorkersStartOnce(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	e.clock.Advance(7 * 24 * time.Hour)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.svc.tx(ctx, func(tx pgx.Tx) error { return e.svc.advanceSprints(ctx, tx, sc.x.ID) })
		}()
	}
	wg.Wait()
	assert.Equal(t, []scrum.SprintStatus{scrum.SprintDone, scrum.SprintActive, scrum.SprintPlanned, scrum.SprintPlanned}, statuses(sc.sprints(t, e)))
	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM outbox_github WHERE operation = 'ISSUE_SPRINT' AND id_echange = $1`, sc.x.ID).Scan(&n))
	assert.Equal(t, 2, n, "une ligne d'outbox par sprint démarré")
}

func TestScrum_Roles(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	st := sc.story(t, e, "Liste des offres", nil)
	assert.Equal(t, scrum.StoryBacklog, st.Status)

	// BOLA : un tiers ne voit rien.
	_, err := e.svc.GetScrum(ctx, sc.tiers, sc.x.ID)
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.PatchStory(ctx, sc.tiers, sc.x.ID, st.ID, StoryPatch{Title: strp("pirate")})
	requireKind(t, err, KindNotFound, "")
	// Le prestataire ne crée pas de story, ne change pas les priorités.
	_, err = e.svc.CreateStory(ctx, sc.lea, sc.x.ID, StoryInput{Title: "Ma story"})
	requireKind(t, err, KindForbidden, "reserve-product-owner")
	_, err = e.svc.PatchStory(ctx, sc.lea, sc.x.ID, st.ID, StoryPatch{Priority: intp(9)})
	requireKind(t, err, KindForbidden, "champ-reserve")
	// Le PO ne change pas les points, n'avance pas une story à la place de l'équipe.
	_, err = e.svc.PatchStory(ctx, sc.sami, sc.x.ID, st.ID, StoryPatch{Points: intp(3)})
	requireKind(t, err, KindForbidden, "champ-reserve")
	_, err = e.svc.PatchStory(ctx, sc.lea, sc.x.ID, st.ID, StoryPatch{Points: intp(4)})
	requireKind(t, err, KindInvalid, "")

	v, err := e.svc.PatchStory(ctx, sc.sami, sc.x.ID, st.ID, StoryPatch{SprintNumber: intp(2)})
	require.NoError(t, err)
	assert.Equal(t, scrum.StoryTodo, v.Status)
	assert.Equal(t, 2, *v.SprintNumber)
	doing := scrum.StoryDoing
	_, err = e.svc.PatchStory(ctx, sc.sami, sc.x.ID, st.ID, StoryPatch{Status: &doing})
	requireKind(t, err, KindForbidden, "champ-reserve")
	sc.move(t, e, sc.lea, st.ID, scrum.StoryDoing)
	done := scrum.StoryDone
	_, err = e.svc.PatchStory(ctx, sc.lea, sc.x.ID, st.ID, StoryPatch{Status: &done})
	requireKind(t, err, KindForbidden, "champ-reserve")

	// Objectif produit : PO seulement.
	_, err = e.svc.PatchScrum(ctx, sc.lea, sc.x.ID, ProjectPatch{Objective: strp("x")})
	requireKind(t, err, KindForbidden, "reserve-product-owner")
	p, err := e.svc.PatchScrum(ctx, sc.sami, sc.x.ID, ProjectPatch{Objective: strp("Boutique en ligne"), DefinitionOfDone: []string{"Revu", "Testé"}})
	require.NoError(t, err)
	assert.Equal(t, "Boutique en ligne", p.Objective)
	assert.Equal(t, []string{"Revu", "Testé"}, p.DefinitionOfDone)

	// Historique : chaque changement de colonne est tracé.
	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM historique_transition WHERE entite = 'STORY' AND id_entite = $1`, st.ID).Scan(&n))
	assert.Equal(t, 3, n, "création, A_FAIRE, EN_COURS")
}

func strp(s string) *string { return &s }

func TestScrum_WebhookMovesStories(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	st := sc.story(t, e, "Paiement", intp(1))
	runJobs(t, e)
	stories, err := e.svc.ListStories(ctx, sc.sami, sc.x.ID, StoryFilter{})
	require.NoError(t, err)
	issue := *stories[0].Issue

	send := func(id, action string, repo int64, label string) WebhookResult {
		body := map[string]any{"action": action, "issue": map[string]any{"number": issue}, "repository": map[string]any{"id": repo}}
		if label != "" {
			body["label"] = map[string]any{"name": label}
		}
		b, _ := json.Marshal(body)
		r, err := e.svc.HandleGitHubWebhook(ctx, id, "issues", b)
		require.NoError(t, err)
		return r
	}
	status := func() scrum.StoryStatus {
		v, err := e.svc.getStory(ctx, e.db, st.ID)
		require.NoError(t, err)
		return v.Status
	}

	assert.Zero(t, send("d1", "closed", 999, "").Stories, "autre dépôt ignoré")
	assert.Equal(t, scrum.StoryTodo, status())
	send("d2", "closed", scrumRepoID, "")
	assert.Equal(t, scrum.StoryDone, status())
	assert.True(t, send("d2", "reopened", scrumRepoID, "").Duplicate, "livraison rejouée sans effet")
	assert.Equal(t, scrum.StoryDone, status())
	send("d3", "reopened", scrumRepoID, "")
	assert.Equal(t, scrum.StoryDoing, status())
	send("d4", "labeled", scrumRepoID, "statut:en-revue")
	assert.Equal(t, scrum.StoryReview, status())

	// Issue supprimée → recréée au passage suivant, avec une note.
	gh.delete(issue)
	send("d5", "deleted", scrumRepoID, "")
	runJobs(t, e)
	v, err := e.svc.getStory(ctx, e.db, st.ID)
	require.NoError(t, err)
	require.NotNil(t, v.Issue)
	assert.NotEqual(t, issue, *v.Issue)
	assert.Contains(t, gh.issue(*v.Issue).Body, "Issue recréée par CodeTroc")
}

func TestScrum_OutboxFailuresAndBackoff(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)

	gh.setFail(func(string) error { return ErrGitHubPermission })
	runJobs(t, e)
	p, err := e.svc.GetScrum(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	assert.Equal(t, "PERMISSION_MANQUANTE", p.SyncStatus)
	assert.Equal(t, 4, p.PendingCalls, "rien n'est perdu")
	calls := gh.calls
	runJobs(t, e)
	assert.Equal(t, calls, gh.calls, "backoff : pas de nouvel essai immédiat")

	// Quota épuisé : Retry-After respecté.
	gh.setFail(func(string) error { return &RateLimitError{RetryAfter: 10 * time.Minute} })
	_, err = e.svc.ResyncScrum(ctx, sc.lea, sc.x.ID)
	require.NoError(t, err)
	runJobs(t, e)
	var next time.Time
	require.NoError(t, e.db.QueryRow(ctx, `SELECT prochain_essai FROM outbox_github WHERE operation = 'LABELS' AND traite_le IS NULL ORDER BY id LIMIT 1`).Scan(&next))
	assert.Equal(t, e.clock.Now().Add(10*time.Minute), next.UTC())

	// Erreur serveur puis rétablissement : tout passe, dans l'ordre, sans doublon.
	gh.setFail(func(op string) error {
		if op == "labels" {
			return errors.New("github : statut 502")
		}
		return nil
	})
	e.clock.Advance(11 * time.Minute)
	runJobs(t, e)
	gh.setFail(nil)
	e.clock.Advance(time.Hour)
	runJobs(t, e)
	p, err = e.svc.GetScrum(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", p.SyncStatus)
	assert.Zero(t, p.PendingCalls)
	assert.Len(t, gh.milestones, 2, "milestones créés une seule fois")
	var issues int
	for range gh.issues {
		issues++
	}
	assert.Equal(t, 1, issues, "une seule issue de sprint")
}

func TestScrum_AmendmentReplansFutureSprints(t *testing.T) {
	e, gh := newScrumEnv(t)
	ctx := context.Background()
	sc := newScrumScenario(t, e, gh)
	planned := sc.story(t, e, "API produits", intp(3))
	acc := *sc.x.AcceptedAt

	bad := []time.Time{acc.AddDate(0, 0, 20), acc.AddDate(0, 0, 18)}
	_, err := e.svc.ProposeAmendment(ctx, sc.lea, sc.x.ID, bad, "")
	requireKind(t, err, KindUnprocessable, "echeances-invalides")
	tooLong := []time.Time{acc.AddDate(0, 0, 20), acc.AddDate(0, 0, 91)}
	_, err = e.svc.ProposeAmendment(ctx, sc.lea, sc.x.ID, tooLong, "")
	requireKind(t, err, KindUnprocessable, "echeances-invalides")
	early := []time.Time{acc.AddDate(0, 0, 5), acc.AddDate(0, 0, 30)}
	_, err = e.svc.ProposeAmendment(ctx, sc.lea, sc.x.ID, early, "")
	requireKind(t, err, KindUnprocessable, "echeances-invalides") // avant la fin du sprint actif

	nd := []time.Time{acc.AddDate(0, 0, 20), acc.AddDate(0, 0, 40)}
	am, err := e.svc.ProposeAmendment(ctx, sc.lea, sc.x.ID, nd, "Retard du design")
	require.NoError(t, err)
	_, err = e.svc.ProposeAmendment(ctx, sc.sami, sc.x.ID, nd, "")
	requireKind(t, err, KindConflict, "avenant-en-cours")
	_, err = e.svc.DecideAmendment(ctx, sc.lea, sc.x.ID, am.ID, true)
	requireKind(t, err, KindForbidden, "avenant-propre")

	_, err = e.svc.DecideAmendment(ctx, sc.sami, sc.x.ID, am.ID, true)
	require.NoError(t, err)
	x, err := e.svc.GetExchange(ctx, sc.sami, sc.x.ID)
	require.NoError(t, err)
	assert.Equal(t, nd[0], x.Milestones[0].Deadline.UTC())
	assert.Equal(t, int64(120), x.Total, "les montants ne bougent pas")

	ss := sc.sprints(t, e)
	require.Len(t, ss, 6) // sprint 1 actif conservé, puis J7→J14, J14→J20 | J20→J27, J27→J34, J34→J40
	assert.Equal(t, scrum.SprintActive, ss[0].Status)
	assert.Equal(t, acc.AddDate(0, 0, 7), ss[0].End, "le sprint actif garde ses dates")
	assert.Equal(t, acc.AddDate(0, 0, 20), ss[2].End)
	assert.Equal(t, 2, ss[3].MilestoneOrder)
	assert.Equal(t, acc.AddDate(0, 0, 40), ss[5].End)
	v, err := e.svc.getStory(ctx, e.db, planned.ID)
	require.NoError(t, err)
	assert.Equal(t, scrum.StoryBacklog, v.Status, "les stories des sprints redécoupés retournent au backlog")
}

func TestScrum_AnnouncementExchangeUsesItsSprintLength(t *testing.T) {
	e, _ := newScrumEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	in := annInput(t, e)
	in.SprintDays, in.Days, in.Title = 14, 28, "Deuxième annonce"
	ann, err := e.svc.PublishAnnouncement(ctx, sc.sami, in)
	require.NoError(t, err)
	app := appInput(120)
	app.Days = 28
	cand, err := e.svc.Apply(ctx, sc.lea, ann.ID, app)
	require.NoError(t, err)
	x, err := e.svc.AcceptApplication(ctx, sc.sami, ann.ID, cand.ID)
	require.NoError(t, err)
	p, err := e.svc.GetScrum(ctx, sc.sami, x.ID)
	require.NoError(t, err)
	assert.Equal(t, 14, p.SprintDays)
	ss, err := e.svc.ListSprints(ctx, sc.lea, x.ID)
	require.NoError(t, err)
	require.Len(t, ss, 2, "deux jalons de 14 jours, un sprint chacun")
	assert.Equal(t, exchange.Active, x.Milestones[0].Status)
}
