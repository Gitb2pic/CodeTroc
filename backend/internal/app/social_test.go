package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completed renvoie un échange TERMINE (trois jalons validés).
func completed(t *testing.T, e *env) deliverable {
	t.Helper()
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
	return d
}

// ---------- F10 : avis ----------

func TestReviews(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := completed(t, e)

	r, err := e.svc.LeaveReview(ctx, d.sami, d.x.ID, 5, "Livré en avance, PR découpée proprement.")
	require.NoError(t, err)
	assert.Equal(t, d.lea.ID, r.RecipientID, "le destinataire est l'autre partie (RG20)")

	_, err = e.svc.LeaveReview(ctx, d.sami, d.x.ID, 4, "Second avis")
	requireKind(t, err, KindConflict, "avis-deja-laisse")
	_, err = e.svc.LeaveReview(ctx, d.tiers, d.x.ID, 1, "Je n'étais pas là")
	requireKind(t, err, KindNotFound, "")
	for _, bad := range []int{0, 6} {
		_, err = e.svc.LeaveReview(ctx, d.lea, d.x.ID, bad, "")
		requireKind(t, err, KindInvalid, "donnees-invalides")
	}
	_, err = e.svc.LeaveReview(ctx, d.lea, d.x.ID, 4, strings.Repeat("x", 2001))
	requireKind(t, err, KindInvalid, "donnees-invalides")
	_, err = e.svc.LeaveReview(ctx, d.lea, d.x.ID, 4, "")
	require.NoError(t, err, "commentaire facultatif")

	prof, err := e.svc.PublicProfile(ctx, d.lea.ID)
	require.NoError(t, err)
	assert.Equal(t, 5.0, prof.Reputation, "RG21 : réputation recalculée")
	assert.Equal(t, 1, prof.ReviewsCount)
	assert.Equal(t, 3, prof.VerifiedDeliveries)

	list, err := e.svc.ListReviews(ctx, d.lea.ID, 20, "")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "sami", list.Items[0].AuthorPseudo)
}

func TestReviews_OnlyOnCompletedExchange(t *testing.T) {
	e := newEnv(t)
	d := newDeliverable(t, e)
	_, err := e.svc.LeaveReview(context.Background(), d.sami, d.x.ID, 5, "Trop tôt")
	requireKind(t, err, KindConflict, "echange-non-termine")
}

func TestReputationIsAverageOfReviews(t *testing.T) {
	assert.Equal(t, 0.0, reputation(nil))
	assert.Equal(t, 4.67, reputation([]int{5, 5, 4}))
	assert.Equal(t, 3.0, reputation([]int{1, 5}))
}

// ---------- F11 : messages ----------

func TestMessages(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := delivered(t, e)

	_, err := e.svc.PostMessage(ctx, d.sami, d.x.ID, "Les tests sont dans /test, `npm test` suffit.")
	require.NoError(t, err)
	e.clock.Advance(time.Minute)
	_, err = e.svc.PostMessage(ctx, d.lea, d.x.ID, "Bien reçu, PR prévue demain.")
	require.NoError(t, err)

	_, err = e.svc.PostMessage(ctx, d.tiers, d.x.ID, "Je m'invite")
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.PostMessage(ctx, d.sami, d.x.ID, "   ")
	requireKind(t, err, KindInvalid, "donnees-invalides")
	_, err = e.svc.PostMessage(ctx, d.sami, d.x.ID, strings.Repeat("x", 5001))
	requireKind(t, err, KindInvalid, "donnees-invalides")

	page, err := e.svc.ListMessages(ctx, d.lea, d.x.ID, 20, "")
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "lea", page.Items[0].AuthorPseudo, "du plus récent au plus ancien")

	_, err = e.svc.ListMessages(ctx, d.tiers, d.x.ID, 20, "")
	requireKind(t, err, KindNotFound, "")

	// RG22 : l'arbitre d'un litige lit les messages mais n'écrit pas.
	arb := e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
	_, err = e.svc.ListMessages(ctx, arb, d.x.ID, 20, "")
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)
	page, err = e.svc.ListMessages(ctx, arb, d.x.ID, 20, "")
	require.NoError(t, err)
	assert.Len(t, page.Items, 2)
	_, err = e.svc.PostMessage(ctx, arb, d.x.ID, "Je tranche.")
	requireKind(t, err, KindForbidden, "acces-refuse")
}
