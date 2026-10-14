package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/domain/exchange"
)

const reason = "Le critère 4 (rotation des refresh tokens) n'est pas rempli."

// delivered renvoie un échange dont le jalon 1 (40 cr) est livré.
func delivered(t *testing.T, e *env) deliverable {
	t.Helper()
	d := newDeliverable(t, e)
	d.submit(t, e, 0, 42)
	return d
}

func TestOpenDispute(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	arb := e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
	d := delivered(t, e)
	ms := d.x.Milestones[0].ID

	_, err := e.svc.OpenDispute(ctx, d.tiers, ms, reason)
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.OpenDispute(ctx, d.sami, ms, "court")
	requireKind(t, err, KindInvalid, "donnees-invalides")
	_, err = e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[1].ID, reason)
	requireKind(t, err, KindConflict, "jalon-non-livre")

	dp, err := e.svc.OpenDispute(ctx, d.sami, ms, reason)
	require.NoError(t, err)
	assert.Equal(t, "OUVERT", dp.Status)
	require.NotNil(t, dp.ArbiterID)
	assert.Equal(t, arb.ID, *dp.ArbiterID, "seul arbitre disponible, non partie (RG16)")

	x, err := e.svc.GetExchange(ctx, d.sami, d.x.ID)
	require.NoError(t, err)
	assert.Equal(t, exchange.InDispute, x.Status)
	assert.Equal(t, exchange.Disputed, x.Milestones[0].Status)
	assert.Equal(t, exchange.ActorArbiter, x.NextActor)

	_, err = e.svc.OpenDispute(ctx, d.lea, ms, reason)
	requireKind(t, err, KindConflict, "jalon-non-livre") // déjà contesté (RG15)
	_, err = e.svc.ValidateMilestone(ctx, d.sami, ms)
	requireKind(t, err, KindConflict, "transition-interdite")

	// L'arbitre voit l'échange (RG22) ; un autre arbitre non.
	_, err = e.svc.GetExchange(ctx, arb, d.x.ID)
	require.NoError(t, err)
}

func TestDisputeSuspendsTacitValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
	d := delivered(t, e)
	_, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)
	e.clock.Advance(100 * 24 * 60 * 60 * 1e9)
	n, err := e.svc.RunTacitValidation(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestDispute_NoArbiterGoesToAdminQueue(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := delivered(t, e)
	// Seul arbitre possible : une partie — il ne peut pas être choisi.
	e.setRole(t, d.lea, RoleArbiter)
	dp, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)
	assert.Nil(t, dp.ArbiterID)

	admin := e.setRole(t, e.actor(t, e.login(t, "admin")), RoleAdmin)
	list, err := e.svc.ListDisputes(ctx, admin)
	require.NoError(t, err)
	require.Len(t, list, 1, "file de l'administrateur (UC12 3a)")

	_, err = e.svc.AssignDispute(ctx, admin, dp.ID, d.lea.ID)
	requireKind(t, err, KindUnprocessable, "arbitre-invalide")
	_, err = e.svc.AssignDispute(ctx, d.sami, dp.ID, admin.ID)
	requireKind(t, err, KindForbidden, "reserve-admin")
	dp, err = e.svc.AssignDispute(ctx, admin, dp.ID, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, admin.ID, *dp.ArbiterID)
}

func TestRespondToDispute(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
	d := delivered(t, e)
	dp, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)

	_, err = e.svc.RespondToDispute(ctx, d.sami, dp.ID, "Je complète ma propre demande.")
	requireKind(t, err, KindForbidden, "acces-refuse")
	dp, err = e.svc.RespondToDispute(ctx, d.lea, dp.ID, "La rotation fonctionne, le test dépend d'une horloge non mockée.")
	require.NoError(t, err)
	require.NotNil(t, dp.Response)
	_, err = e.svc.RespondToDispute(ctx, d.lea, dp.ID, "Une seconde réponse.")
	requireKind(t, err, KindConflict, "reponse-deja-donnee")
}

func TestDecideDispute(t *testing.T) {
	cases := []struct {
		kind               string
		share              int
		provider, refunded int64
		status             exchange.MilestoneStatus
	}{
		{"LIBERER", 0, 40, 0, exchange.Released},
		{"REMBOURSER", 0, 0, 40, exchange.Refunded},
		{"PARTAGER", 60, 24, 16, exchange.Split},
		{"PARTAGER", 1, 0, 40, exchange.Split}, // 1 % de 40 arrondi à 0 : tout au demandeur (D10)
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			arb := e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
			d := delivered(t, e)
			dp, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
			require.NoError(t, err)

			dec := Decision{Kind: c.kind, ProviderShare: c.share, Justification: "Jalons 1–2 prouvés par la CI, le jalon 3 échoue sur un critère explicite."}
			_, err = e.svc.DecideDispute(ctx, d.sami, dp.ID, dec)
			requireKind(t, err, KindNotFound, "")

			dp, err = e.svc.DecideDispute(ctx, arb, dp.ID, dec)
			require.NoError(t, err)
			assert.Equal(t, "CLOS", dp.Status)

			x, err := e.svc.GetExchange(ctx, d.sami, d.x.ID)
			require.NoError(t, err)
			assert.Equal(t, c.status, x.Milestones[0].Status)
			assert.Equal(t, exchange.InProgress, x.Status, "plus de jalon contesté : l'échange reprend")
			assert.Equal(t, c.provider, x.Escrow.Released)
			assert.Equal(t, c.refunded, x.Escrow.Refunded)
			assert.Equal(t, int64(80), x.Escrow.Locked)
			requireLedgerConsistent(t, e)

			_, err = e.svc.DecideDispute(ctx, arb, dp.ID, dec)
			requireKind(t, err, KindConflict, "litige-clos")
		})
	}
}

func TestDecideDispute_Validation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	arb := e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
	d := delivered(t, e)
	dp, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)
	just := "Décision motivée par les preuves jointes."
	for name, dec := range map[string]Decision{
		"décision inconnue":       {Kind: "TOUT_AU_PRESTATAIRE", Justification: just},
		"part hors bornes":        {Kind: "PARTAGER", ProviderShare: 100, Justification: just},
		"part sans partage":       {Kind: "LIBERER", ProviderShare: 50, Justification: just},
		"justification manquante": {Kind: "LIBERER"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := e.svc.DecideDispute(ctx, arb, dp.ID, dec)
			requireKind(t, err, KindInvalid, "donnees-invalides")
		})
	}
	other := e.setRole(t, e.actor(t, e.login(t, "autre-arbitre")), RoleArbiter)
	_, err = e.svc.DecideDispute(ctx, other, dp.ID, Decision{Kind: "LIBERER", Justification: just})
	requireKind(t, err, KindNotFound, "") // seul l'arbitre affecté décide
	_, err = e.svc.GetDispute(ctx, other, uuid.New())
	requireKind(t, err, KindNotFound, "")
}
