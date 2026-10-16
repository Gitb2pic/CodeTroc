package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

// annScenario : Sami (360 cr) publie une annonce à 120 cr ; Léa et Noé candidatent.
type annScenario struct {
	sami, lea, noe, tiers Actor
	ann                   AnnouncementView
}

func annInput(t *testing.T, e *env) AnnouncementInput {
	return AnnouncementInput{
		PackID: packID(t, e, "Pipeline"), Title: "Pipeline CI pour notre API", Need: "Lint, tests et image Docker sur chaque PR.",
		RepoURL: "https://github.com/acme/shop-api", Budget: 120, Days: 20,
		Milestones: []catalog.Milestone{{Title: "CI de base", Amount: 60}, {Title: "Image Docker", Amount: 60}},
	}
}

func appInput(price int64) ApplicationInput {
	return ApplicationInput{Price: price, Days: 15, Message: "Déjà fait pour deux API Node.",
		Milestones: []catalog.Milestone{{Title: "CI", Amount: price / 2}, {Title: "Docker", Amount: price - price/2}}}
}

func newAnnScenario(t *testing.T, e *env) annScenario {
	t.Helper()
	sc := annScenario{
		sami:  e.actor(t, e.login(t, "sami")),
		lea:   e.actor(t, e.login(t, "lea")),
		noe:   e.actor(t, e.login(t, "noe")),
		tiers: e.actor(t, e.login(t, "tiers")),
	}
	credit(t, e, sc.sami.ID, 310)
	ann, err := e.svc.PublishAnnouncement(context.Background(), sc.sami, annInput(t, e))
	require.NoError(t, err)
	sc.ann = ann
	return sc
}

func (sc annScenario) apply(t *testing.T, e *env, a Actor, price int64) ApplicationView {
	t.Helper()
	v, err := e.svc.Apply(context.Background(), a, sc.ann.ID, appInput(price))
	require.NoError(t, err)
	return v
}

func TestAnnouncement_PublishAndList(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)

	assert.Equal(t, AnnOpen, sc.ann.Status)
	assert.Equal(t, sc.ann.PublishedAt.Add(AnnouncementTTL), sc.ann.ExpiresAt)
	assert.Equal(t, "acme", sc.ann.Repo.Owner)

	p, err := e.svc.ListAnnouncements(ctx, nil, AnnouncementFilter{}, 20, "")
	require.NoError(t, err)
	require.Len(t, p.Items, 1)

	p, err = e.svc.ListAnnouncements(ctx, &sc.lea, AnnouncementFilter{Q: "docker"}, 20, "")
	require.NoError(t, err)
	assert.Len(t, p.Items, 1)

	// Le budget doit être couvert par le solde disponible.
	poor := e.actor(t, e.login(t, "poor"))
	_, err = e.svc.PublishAnnouncement(ctx, poor, annInput(t, e))
	requireKind(t, err, KindUnprocessable, "solde-insuffisant")

	bad := annInput(t, e)
	bad.RepoURL = "http://169.254.169.254/latest"
	_, err = e.svc.PublishAnnouncement(ctx, sc.sami, bad)
	requireKind(t, err, KindInvalid, "depot-invalide")
}

func TestAnnouncement_ApplyRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)

	_, err := e.svc.Apply(ctx, sc.sami, sc.ann.ID, appInput(100))
	requireKind(t, err, KindUnprocessable, "propre-annonce")

	sc.apply(t, e, sc.lea, 100)
	_, err = e.svc.Apply(ctx, sc.lea, sc.ann.ID, appInput(110))
	requireKind(t, err, KindConflict, "candidature-existante")

	v, err := e.svc.GetAnnouncement(ctx, &sc.lea, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, v.Applications)
	require.NotNil(t, v.MyApp)

	// L'auteur voit toutes les candidatures, un prestataire la sienne seulement.
	sc.apply(t, e, sc.noe, 120)
	all, err := e.svc.ListApplications(ctx, sc.sami, sc.ann.ID)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	mine, err := e.svc.ListApplications(ctx, sc.noe, sc.ann.ID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, sc.noe.ID, mine[0].Provider.ID)

	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM notification WHERE id_utilisateur = $1 AND type = 'CANDIDATURE_RECUE'`, sc.sami.ID).Scan(&n))
	assert.Equal(t, 2, n)
}

func TestAnnouncement_AcceptCreatesRunningExchange(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	lea := sc.apply(t, e, sc.lea, 100)
	noe := sc.apply(t, e, sc.noe, 120)

	// Seul l'auteur décide ; un candidat reçoit 403, un tiers 404.
	_, err := e.svc.AcceptApplication(ctx, sc.lea, sc.ann.ID, lea.ID)
	requireKind(t, err, KindForbidden, "")
	_, err = e.svc.AcceptApplication(ctx, sc.tiers, sc.ann.ID, lea.ID)
	requireKind(t, err, KindNotFound, "")

	x, err := e.svc.AcceptApplication(ctx, sc.sami, sc.ann.ID, lea.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.InProgress, x.Status)
	assert.Equal(t, int64(100), x.Total)
	assert.Equal(t, 15, x.Days)
	assert.Nil(t, x.Offer.ID)
	require.NotNil(t, x.Offer.AnnouncementID)
	assert.Equal(t, sc.ann.ID, *x.Offer.AnnouncementID)
	assert.Equal(t, "Pipeline CI pour notre API", x.Offer.Title)
	assert.Equal(t, catalog.ProofPRCI, x.Offer.ProofType)
	assert.Equal(t, sc.sami.ID, x.Requester.ID)
	assert.Equal(t, sc.lea.ID, x.Provider.ID)
	assert.Equal(t, int64(100), x.Escrow.Locked)
	require.Len(t, x.Milestones, 2)
	assert.Equal(t, exchange.Active, x.Milestones[0].Status)
	assert.NotNil(t, x.Milestones[1].Deadline)
	assert.Equal(t, int64(260), balanceOf(t, e, sc.sami).Available)

	ann, err := e.svc.GetAnnouncement(ctx, &sc.sami, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, AnnFilled, ann.Status)
	require.NotNil(t, ann.ExchangeID)
	assert.Equal(t, x.ID, *ann.ExchangeID)

	// L'autre candidature est refusée et son auteur prévenu ; elle reste visible de lui.
	others, err := e.svc.ListApplications(ctx, sc.noe, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, AppRefused, others[0].Status)
	assert.Equal(t, noe.ID, others[0].ID)
	var kinds []string
	rows, err := e.db.Query(ctx, `SELECT type FROM notification WHERE id_utilisateur IN ($1, $2) ORDER BY type`, sc.lea.ID, sc.noe.ID)
	require.NoError(t, err)
	for rows.Next() {
		var k string
		require.NoError(t, rows.Scan(&k))
		kinds = append(kinds, k)
	}
	assert.Equal(t, []string{NotifApplicationAccepted, NotifApplicationRefused}, kinds)

	// Annonce pourvue : plus de candidature, plus d'acceptation ; invisible des tiers.
	_, err = e.svc.Apply(ctx, sc.tiers, sc.ann.ID, appInput(100))
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.AcceptApplication(ctx, sc.sami, sc.ann.ID, noe.ID)
	requireKind(t, err, KindConflict, "annonce-fermee")
	_, err = e.svc.GetAnnouncement(ctx, &sc.tiers, sc.ann.ID)
	requireKind(t, err, KindNotFound, "")

	// L'échange né de l'annonce suit le cycle normal : historique complet.
	var hist int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM historique_transition WHERE id_entite = $1`, x.ID).Scan(&hist))
	assert.Equal(t, 2, hist, "PROPOSE puis EN_COURS")
}

func TestAnnouncement_InsufficientBalanceKeepsAnnouncementOpen(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	lea := sc.apply(t, e, sc.lea, 150)

	// Le solde de Sami est descendu sous le prix entre-temps.
	other := e.actor(t, e.login(t, "autre"))
	o, err := e.svc.CreateOffer(ctx, other, offerInput(t, e))
	require.NoError(t, err)
	for range 2 {
		x, err := e.svc.RequestExchange(ctx, sc.sami, ExchangeRequest{OfferID: o.ID, RepoURL: "https://github.com/acme/a"})
		require.NoError(t, err)
		_, err = e.svc.AcceptExchange(ctx, other, x.ID)
		require.NoError(t, err)
	}
	require.Equal(t, int64(120), balanceOf(t, e, sc.sami).Available)

	_, err = e.svc.AcceptApplication(ctx, sc.sami, sc.ann.ID, lea.ID)
	requireKind(t, err, KindUnprocessable, "solde-insuffisant")

	ann, err := e.svc.GetAnnouncement(ctx, &sc.sami, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, AnnOpen, ann.Status)
	assert.Equal(t, 1, ann.Applications)
	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM echange WHERE id_annonce = $1`, sc.ann.ID).Scan(&n))
	assert.Zero(t, n)
}

func TestAnnouncement_ConcurrentAcceptsCreateOneExchange(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	apps := []ApplicationView{sc.apply(t, e, sc.lea, 100), sc.apply(t, e, sc.noe, 110)}

	var wg sync.WaitGroup
	errs := make([]error, len(apps))
	for i, a := range apps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.svc.AcceptApplication(ctx, sc.sami, sc.ann.ID, a.ID)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			requireKind(t, err, KindConflict, "")
		}
	}
	assert.Equal(t, 1, ok)
	var exchanges, blocks int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM echange WHERE id_annonce = $1`, sc.ann.ID).Scan(&exchanges))
	require.NoError(t, e.db.QueryRow(ctx, `
		SELECT count(*) FROM mouvement m JOIN echange x USING (id_echange)
		WHERE x.id_annonce = $1 AND m.type_mouvement = 'BLOCAGE'`, sc.ann.ID).Scan(&blocks))
	assert.Equal(t, 1, exchanges)
	assert.Equal(t, 1, blocks)
}

func TestAnnouncement_RefuseWithdrawAndExpire(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	lea := sc.apply(t, e, sc.lea, 100)
	noe := sc.apply(t, e, sc.noe, 110)

	v, err := e.svc.RefuseApplication(ctx, sc.sami, sc.ann.ID, lea.ID)
	require.NoError(t, err)
	assert.Equal(t, AppRefused, v.Status)
	_, err = e.svc.RefuseApplication(ctx, sc.sami, sc.ann.ID, lea.ID)
	requireKind(t, err, KindConflict, "candidature-traitee")

	// Seul le candidat retire sa candidature.
	_, err = e.svc.WithdrawApplication(ctx, sc.lea, sc.ann.ID, noe.ID)
	requireKind(t, err, KindNotFound, "")
	v, err = e.svc.WithdrawApplication(ctx, sc.noe, sc.ann.ID, noe.ID)
	require.NoError(t, err)
	assert.Equal(t, AppWithdrawn, v.Status)

	// Un nouveau candidat, puis expiration par le planificateur.
	tiers := sc.apply(t, e, sc.tiers, 120)
	e.clock.Advance(AnnouncementTTL)
	n, err := e.svc.ExpireAnnouncements(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	ann, err := e.svc.GetAnnouncement(ctx, &sc.sami, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, AnnExpired, ann.Status)
	apps, err := e.svc.ListApplications(ctx, sc.tiers, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, tiers.ID, apps[0].ID)
	assert.Equal(t, AppRefused, apps[0].Status)
	n, err = e.svc.ExpireAnnouncements(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "idempotent")
}

func TestAnnouncement_Withdraw(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	lea := sc.apply(t, e, sc.lea, 100)

	_, err := e.svc.WithdrawAnnouncement(ctx, sc.lea, sc.ann.ID)
	requireKind(t, err, KindForbidden, "")
	v, err := e.svc.WithdrawAnnouncement(ctx, sc.sami, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, AnnWithdrawn, v.Status)
	apps, err := e.svc.ListApplications(ctx, sc.lea, sc.ann.ID)
	require.NoError(t, err)
	assert.Equal(t, lea.ID, apps[0].ID)
	assert.Equal(t, AppRefused, apps[0].Status)

	p, err := e.svc.ListAnnouncements(ctx, nil, AnnouncementFilter{}, 20, "")
	require.NoError(t, err)
	assert.Empty(t, p.Items)
	p, err = e.svc.ListAnnouncements(ctx, &sc.sami, AnnouncementFilter{Mine: true}, 20, "")
	require.NoError(t, err)
	assert.Len(t, p.Items, 1, "l'auteur retrouve ses annonces fermées")
	_, err = e.svc.ListAnnouncements(ctx, nil, AnnouncementFilter{Mine: true}, 20, "")
	requireKind(t, err, KindUnauthenticated, "")
}

func TestAnnouncement_ApplicationRateLimit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newAnnScenario(t, e)
	for i := range maxApplicationsPerDay {
		_, err := e.db.Exec(ctx, `
			INSERT INTO candidature (id_annonce, id_prestataire, prix_credits, delai_jours, jalons_proposes, statut_candidature, date_candidature)
			VALUES ($1, $2, 100, 10, '[]', 'RETIREE', $3)`, sc.ann.ID, sc.lea.ID, e.clock.Now().Add(-time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	_, err := e.svc.Apply(ctx, sc.lea, sc.ann.ID, appInput(100))
	requireKind(t, err, KindUnprocessable, "limite-candidatures")
}
