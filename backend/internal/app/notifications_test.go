package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kinds(t *testing.T, e *env, a Actor) []string {
	t.Helper()
	p, err := e.svc.ListNotifications(context.Background(), a, false, 100, "")
	require.NoError(t, err)
	out := make([]string, len(p.Items))
	for i, n := range p.Items {
		out[len(p.Items)-1-i] = n.Kind // ordre chronologique
	}
	return out
}

func TestNotifications_ExchangeLifecycle(t *testing.T) {
	e := newEnv(t, withVault)
	ctx := context.Background()
	d := completed(t, e)
	_, err := e.svc.LeaveReview(ctx, d.sami, d.x.ID, 5, "")
	require.NoError(t, err)

	assert.Equal(t, []string{
		NotifExchangeRequested, NotifMilestonePaid, NotifMilestonePaid, NotifMilestonePaid, NotifExchangeDone, NotifReview,
	}, kinds(t, e, d.lea), "prestataire")
	// Le demandeur termine l'échange en validant le dernier jalon : jamais notifié de ses propres actions.
	assert.Equal(t, []string{
		NotifExchangeAccepted, NotifMilestoneDone, NotifMilestoneDone, NotifMilestoneDone,
	}, kinds(t, e, d.sami), "demandeur")
	assert.Empty(t, kinds(t, e, d.tiers))

	p, err := e.svc.ListNotifications(ctx, d.sami, false, 1, "")
	require.NoError(t, err)
	assert.Equal(t, "Pipeline CI/CD GitHub Actions", p.Items[0].Data["pack"], "titre du pack ajouté aux données")
}

func TestNotifications_DisputeMessagesSecrets(t *testing.T) {
	e := newEnv(t, withVault)
	ctx := context.Background()
	d := delivered(t, e)
	arb := e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)

	_, err := e.svc.PostMessage(ctx, d.sami, d.x.ID, "Bonjour")
	require.NoError(t, err)
	_, err = e.svc.ShareSecret(ctx, d.sami, d.x.ID, secretInput(e), "")
	require.NoError(t, err)
	dp, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)
	_, err = e.svc.RespondToDispute(ctx, d.lea, dp.ID, "Ma réponse détaillée.")
	require.NoError(t, err)
	_, err = e.svc.DecideDispute(ctx, arb, dp.ID, Decision{Kind: "LIBERER", Justification: "Les critères sont remplis."})
	require.NoError(t, err)

	lea := kinds(t, e, d.lea)
	for _, k := range []string{NotifMessage, NotifSecretShared, NotifDisputeOpened, NotifDisputeDecided} {
		assert.Contains(t, lea, k)
	}
	assert.NotContains(t, lea, NotifMilestonePaid, "l'issue d'un litige est annoncée par LITIGE_DECIDE")
	assert.Equal(t, []string{NotifDisputeOpened, NotifDisputeAnswered}, kinds(t, e, arb))
	assert.NotContains(t, kinds(t, e, d.sami), NotifDisputeOpened, "l'ouvreur n'est pas notifié")

	// Aucune valeur secrète dans les notifications.
	var leaked bool
	require.NoError(t, e.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notification WHERE donnees::text LIKE '%ghp_tresSecret%')`).Scan(&leaked))
	assert.False(t, leaked)
}

func TestNotifications_ReadState(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := delivered(t, e)
	for range 3 {
		_, err := e.svc.PostMessage(ctx, d.sami, d.x.ID, "Ping")
		require.NoError(t, err)
	}
	n, err := e.svc.UnreadCount(ctx, d.lea)
	require.NoError(t, err)
	require.Positive(t, n)

	p, err := e.svc.ListNotifications(ctx, d.lea, true, 1, "")
	require.NoError(t, err)
	changed, err := e.svc.MarkNotificationsRead(ctx, d.lea, []uuid.UUID{p.Items[0].ID})
	require.NoError(t, err)
	assert.Equal(t, 1, changed)

	// On ne marque pas les notifications d'un autre.
	changed, err = e.svc.MarkNotificationsRead(ctx, d.sami, []uuid.UUID{p.Items[0].ID})
	require.NoError(t, err)
	assert.Zero(t, changed)

	_, err = e.svc.MarkNotificationsRead(ctx, d.lea, nil)
	require.NoError(t, err)
	n, err = e.svc.UnreadCount(ctx, d.lea)
	require.NoError(t, err)
	assert.Zero(t, n)
	p, err = e.svc.ListNotifications(ctx, d.lea, true, 20, "")
	require.NoError(t, err)
	assert.Empty(t, p.Items)
}
