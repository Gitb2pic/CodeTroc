package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/catalog"
	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
	"github.com/gitb2pic/codetroc/backend/internal/domain/ledger"
)

// credit ajoute des crédits à un membre depuis ÉMISSION (préparation de test).
func credit(t *testing.T, e *env, userID uuid.UUID, amount int64) {
	t.Helper()
	ctx := context.Background()
	var issuance uuid.UUID
	require.NoError(t, e.db.QueryRow(ctx, `SELECT id_compte FROM compte WHERE type_compte = 'EMISSION'`).Scan(&issuance))
	require.NoError(t, e.svc.tx(ctx, func(tx pgx.Tx) error {
		_, err := e.svc.transfer(ctx, tx, transferReq{From: issuance, To: userAccount(t, e, userID), Amount: amount, Kind: ledger.Bonus, Key: "test:" + uuid.NewString()})
		return err
	}))
}

// scenario prépare un demandeur (Sami, 360 cr), une prestataire (Léa) et son offre à 120 cr.
type scenario struct {
	sami, lea, tiers Actor
	offer            OfferView
}

func newScenario(t *testing.T, e *env) scenario {
	t.Helper()
	sc := scenario{
		sami:  e.actor(t, e.login(t, "sami")),
		lea:   e.actor(t, e.login(t, "lea")),
		tiers: e.actor(t, e.login(t, "tiers")),
	}
	credit(t, e, sc.sami.ID, 310)
	o, err := e.svc.CreateOffer(context.Background(), sc.lea, offerInput(t, e))
	require.NoError(t, err)
	sc.offer = o
	return sc
}

func (sc scenario) request(t *testing.T, e *env) ExchangeView {
	t.Helper()
	x, err := e.svc.RequestExchange(context.Background(), sc.sami, ExchangeRequest{OfferID: sc.offer.ID, RepoURL: "https://github.com/acme/shop-api", Need: "Pipeline pour notre API"})
	require.NoError(t, err)
	return x
}

func (sc scenario) accepted(t *testing.T, e *env) ExchangeView {
	t.Helper()
	x := sc.request(t, e)
	x, err := e.svc.AcceptExchange(context.Background(), sc.lea, x.ID)
	require.NoError(t, err)
	return x
}

func balanceOf(t *testing.T, e *env, a Actor) Balance {
	t.Helper()
	b, err := e.svc.Balance(context.Background(), a)
	require.NoError(t, err)
	return b
}

func TestRequestExchange(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newScenario(t, e)

	x := sc.request(t, e)
	assert.Equal(t, exchange.Proposed, x.Status)
	assert.Equal(t, int64(120), x.Total)
	assert.Equal(t, "https://github.com/acme/shop-api", x.Repo.URL())
	require.Len(t, x.Milestones, 3)
	assert.Equal(t, "Workflow CI de base", x.Milestones[0].Title, "jalons de l'offre par défaut")
	assert.Equal(t, exchange.ActorProvider, x.NextActor)
	assert.Equal(t, int64(360), balanceOf(t, e, sc.sami).Available, "aucun crédit bloqué avant l'acceptation (UC06)")

	custom := ExchangeRequest{OfferID: sc.offer.ID, RepoURL: "https://github.com/acme/shop-api",
		Milestones: []catalog.Milestone{{Title: "Tout d'un coup", Amount: 120}}}
	x2, err := e.svc.RequestExchange(ctx, sc.sami, custom)
	require.NoError(t, err)
	assert.Len(t, x2.Milestones, 1)

	custom.Milestones = []catalog.Milestone{{Title: "Trop peu", Amount: 100}}
	_, err = e.svc.RequestExchange(ctx, sc.sami, custom)
	requireKind(t, err, KindInvalid, "jalons-incoherents")

	_, err = e.svc.RequestExchange(ctx, sc.sami, ExchangeRequest{OfferID: sc.offer.ID, RepoURL: "https://169.254.169.254/latest"})
	requireKind(t, err, KindInvalid, "depot-invalide")

	_, err = e.svc.RequestExchange(ctx, sc.lea, ExchangeRequest{OfferID: sc.offer.ID, RepoURL: "https://github.com/a/b"})
	requireKind(t, err, KindUnprocessable, "propre-offre")

	_, err = e.svc.RequestExchange(ctx, sc.tiers, ExchangeRequest{OfferID: sc.offer.ID, RepoURL: "https://github.com/a/b"})
	requireKind(t, err, KindUnprocessable, "solde-insuffisant")

	require.NoError(t, e.svc.WithdrawOffer(ctx, sc.lea, sc.offer.ID))
	_, err = e.svc.RequestExchange(ctx, sc.sami, ExchangeRequest{OfferID: sc.offer.ID, RepoURL: "https://github.com/a/b"})
	requireKind(t, err, KindUnprocessable, "offre-indisponible")
}

func TestAcceptExchange_BlocksEscrow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newScenario(t, e)
	x := sc.request(t, e)

	_, err := e.svc.AcceptExchange(ctx, sc.sami, x.ID)
	requireKind(t, err, KindForbidden, "acces-refuse")
	_, err = e.svc.AcceptExchange(ctx, sc.tiers, x.ID)
	requireKind(t, err, KindNotFound, "")

	x, err = e.svc.AcceptExchange(ctx, sc.lea, x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.InProgress, x.Status)
	assert.Equal(t, exchange.Active, x.Milestones[0].Status)
	assert.Equal(t, exchange.Pending, x.Milestones[1].Status)
	require.NotNil(t, x.Milestones[2].Deadline)
	assert.True(t, e.clock.Now().AddDate(0, 0, 5).Equal(*x.Milestones[2].Deadline))
	assert.Equal(t, int64(120), x.Escrow.Locked)

	assert.Equal(t, Balance{Available: 240, EscrowOut: 120}, balanceOf(t, e, sc.sami))
	assert.Equal(t, Balance{Available: 50, EscrowIn: 120}, balanceOf(t, e, sc.lea))

	_, err = e.svc.AcceptExchange(ctx, sc.lea, x.ID)
	requireKind(t, err, KindConflict, "transition-interdite")
	requireLedgerConsistent(t, e)
}

func TestAcceptExchange_Concurrent(t *testing.T) {
	e := newEnv(t)
	sc := newScenario(t, e)
	x := sc.request(t, e)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = e.svc.AcceptExchange(context.Background(), sc.lea, x.ID)
		}()
	}
	wg.Wait()
	var n int
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT count(*) FROM mouvement WHERE type_mouvement = 'BLOCAGE'`).Scan(&n))
	assert.Equal(t, 1, n)
	assert.Equal(t, int64(240), balanceOf(t, e, sc.sami).Available)
	requireLedgerConsistent(t, e)
}

func TestAcceptExchange_BalanceDroppedMeanwhile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newScenario(t, e)
	x := sc.request(t, e)
	other := sc.request(t, e)
	other2 := sc.request(t, e)
	last := sc.request(t, e)
	_, err := e.svc.AcceptExchange(ctx, sc.lea, other.ID)
	require.NoError(t, err)
	_, err = e.svc.AcceptExchange(ctx, sc.lea, other2.ID)
	require.NoError(t, err)

	_, err = e.svc.AcceptExchange(ctx, sc.lea, x.ID) // 360 − 240 = 120 : encore possible
	require.NoError(t, err)
	_, err = e.svc.AcceptExchange(ctx, sc.lea, last.ID)
	requireKind(t, err, KindUnprocessable, "solde-insuffisant")
	got, err := e.svc.GetExchange(ctx, sc.sami, last.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Proposed, got.Status, "UC07 2a : l'échange reste proposé")
	requireLedgerConsistent(t, e)
}

func TestRefuseExchange(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newScenario(t, e)
	x := sc.request(t, e)
	_, err := e.svc.RefuseExchange(ctx, sc.sami, x.ID)
	requireKind(t, err, KindForbidden, "acces-refuse")
	x, err = e.svc.RefuseExchange(ctx, sc.lea, x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.Refused, x.Status)
	assert.Equal(t, exchange.ActorNone, x.NextActor)
}

func TestCancelExchange(t *testing.T) {
	ctx := context.Background()

	t.Run("demande proposée : annulée par le demandeur", func(t *testing.T) {
		e := newEnv(t)
		sc := newScenario(t, e)
		x := sc.request(t, e)
		_, err := e.svc.CancelExchange(ctx, sc.lea, x.ID)
		requireKind(t, err, KindForbidden, "acces-refuse")
		x, err = e.svc.CancelExchange(ctx, sc.sami, x.ID)
		require.NoError(t, err)
		assert.Equal(t, exchange.Cancelled, x.Status)
	})

	t.Run("en cours : il faut l'accord des deux parties", func(t *testing.T) {
		e := newEnv(t)
		sc := newScenario(t, e)
		x := sc.accepted(t, e)

		x, err := e.svc.CancelExchange(ctx, sc.sami, x.ID)
		require.NoError(t, err)
		assert.Equal(t, exchange.InProgress, x.Status)
		require.NotNil(t, x.CancelRequestedBy)
		assert.Equal(t, sc.sami.ID, *x.CancelRequestedBy)

		x, err = e.svc.CancelExchange(ctx, sc.sami, x.ID) // répéter ne suffit pas
		require.NoError(t, err)
		assert.Equal(t, exchange.InProgress, x.Status)

		x, err = e.svc.CancelExchange(ctx, sc.lea, x.ID)
		require.NoError(t, err)
		assert.Equal(t, exchange.Cancelled, x.Status)
		for _, m := range x.Milestones {
			assert.Equal(t, exchange.Refunded, m.Status)
		}
		assert.Equal(t, Balance{Available: 360}, balanceOf(t, e, sc.sami), "RG18 : tout est remboursé")
		requireLedgerConsistent(t, e)
	})

	t.Run("abandon : le demandeur annule seul après échéance et délai de grâce (D8)", func(t *testing.T) {
		e := newEnv(t)
		sc := newScenario(t, e)
		x := sc.accepted(t, e)

		e.clock.Advance(2*24*time.Hour + 47*time.Hour) // échéance J1 (2 j) + 47 h
		x, err := e.svc.CancelExchange(ctx, sc.sami, x.ID)
		require.NoError(t, err)
		assert.Equal(t, exchange.InProgress, x.Status, "délai de grâce pas encore écoulé")

		e.clock.Advance(2 * time.Hour)
		x, err = e.svc.CancelExchange(ctx, sc.sami, x.ID)
		require.NoError(t, err)
		assert.Equal(t, exchange.Cancelled, x.Status)
		assert.Equal(t, int64(360), balanceOf(t, e, sc.sami).Available)
		requireLedgerConsistent(t, e)
	})

	t.Run("refusée si un jalon livré attend une décision", func(t *testing.T) {
		e := newEnv(t)
		sc := newScenario(t, e)
		x := sc.accepted(t, e)
		_, err := e.db.Exec(ctx, `UPDATE jalon SET statut_jalon = 'LIVRE' WHERE id_jalon = $1`, x.Milestones[0].ID)
		require.NoError(t, err)
		_, err = e.svc.CancelExchange(ctx, sc.sami, x.ID)
		requireKind(t, err, KindConflict, "jalon-a-trancher")
	})

	t.Run("un tiers ne voit pas l'échange", func(t *testing.T) {
		e := newEnv(t)
		sc := newScenario(t, e)
		x := sc.request(t, e)
		_, err := e.svc.CancelExchange(ctx, sc.tiers, x.ID)
		requireKind(t, err, KindNotFound, "")
	})
}

func TestGetAndListExchanges(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sc := newScenario(t, e)
	x := sc.accepted(t, e)

	_, err := e.svc.GetExchange(ctx, sc.tiers, x.ID)
	requireKind(t, err, KindNotFound, "")
	admin := e.setRole(t, sc.tiers, RoleAdmin)
	_, err = e.svc.GetExchange(ctx, admin, x.ID)
	requireKind(t, err, KindNotFound, "") // même un admin ne lit pas un échange dont il n'est pas partie

	mine, err := e.svc.ListExchanges(ctx, sc.sami, ExchangeFilter{Role: "demandeur"}, 20, "")
	require.NoError(t, err)
	assert.Len(t, mine.Items, 1)
	mine, err = e.svc.ListExchanges(ctx, sc.sami, ExchangeFilter{Role: "prestataire"}, 20, "")
	require.NoError(t, err)
	assert.Empty(t, mine.Items)
	mine, err = e.svc.ListExchanges(ctx, sc.lea, ExchangeFilter{}, 20, "")
	require.NoError(t, err)
	assert.Len(t, mine.Items, 1)

	var n int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM historique_transition WHERE id_entite = $1`, x.ID).Scan(&n))
	assert.Equal(t, 2, n, "création puis acceptation sont tracées (ENF04)")
}
