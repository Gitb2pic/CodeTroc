package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmin_OnlyAdmins(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	member := e.actor(t, e.login(t, "membre"))
	arb := e.setRole(t, e.actor(t, e.login(t, "arbitre")), RoleArbiter)
	for _, a := range []Actor{member, arb} {
		_, err := e.svc.AdminListUsers(ctx, a, "", 20, "")
		requireKind(t, err, KindForbidden, "reserve-admin")
		role := RoleAdmin
		_, err = e.svc.AdminUpdateUser(ctx, a, a.ID, AdminPatch{Role: &role})
		requireKind(t, err, KindForbidden, "reserve-admin")
	}
}

func TestAdmin_SuspendRevokesSessionsAndUnassignsDisputes(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	admin := e.setRole(t, e.actor(t, e.login(t, "admin")), RoleAdmin)
	d := delivered(t, e)
	arbSess := e.login(t, "arbitre")
	arb := e.setRole(t, e.actor(t, arbSess), RoleArbiter)
	dp, err := e.svc.OpenDispute(ctx, d.sami, d.x.Milestones[0].ID, reason)
	require.NoError(t, err)
	require.NotNil(t, dp.ArbiterID)
	require.Equal(t, arb.ID, *dp.ArbiterID, "un arbitre est préféré à un administrateur")

	suspended := "SUSPENDU"
	u, err := e.svc.AdminUpdateUser(ctx, admin, arb.ID, AdminPatch{Status: &suspended})
	require.NoError(t, err)
	assert.Equal(t, "SUSPENDU", u.Status)

	_, err = e.svc.Refresh(ctx, arbSess.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "")
	_, err = e.svc.Authenticate(ctx, arbSess.AccessToken)
	require.ErrorIs(t, err, ErrUnauthenticated, "le jeton d'accès cesse de fonctionner immédiatement")

	got, err := e.svc.GetDispute(ctx, admin, dp.ID)
	require.NoError(t, err)
	assert.Nil(t, got.ArbiterID, "le litige retourne dans la file")
	queue, err := e.svc.AdminDisputeQueue(ctx, admin)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, dp.ID, queue[0].ID)

	active := "ACTIF"
	_, err = e.svc.AdminUpdateUser(ctx, admin, arb.ID, AdminPatch{Status: &active})
	require.NoError(t, err)
}

func TestAdmin_Validation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	admin := e.setRole(t, e.actor(t, e.login(t, "admin")), RoleAdmin)
	m := e.actor(t, e.login(t, "membre"))

	deleted := "SUPPRIME"
	_, err := e.svc.AdminUpdateUser(ctx, admin, m.ID, AdminPatch{Status: &deleted})
	requireKind(t, err, KindInvalid, "donnees-invalides")
	bad := Role("ROOT")
	_, err = e.svc.AdminUpdateUser(ctx, admin, m.ID, AdminPatch{Role: &bad})
	requireKind(t, err, KindInvalid, "donnees-invalides")
	member := RoleMember
	_, err = e.svc.AdminUpdateUser(ctx, admin, admin.ID, AdminPatch{Role: &member})
	requireKind(t, err, KindUnprocessable, "auto-modification")
	_, err = e.svc.AdminUpdateUser(ctx, admin, uuid.New(), AdminPatch{Role: &member})
	requireKind(t, err, KindNotFound, "")

	arbiter := RoleArbiter
	u, err := e.svc.AdminUpdateUser(ctx, admin, m.ID, AdminPatch{Role: &arbiter})
	require.NoError(t, err)
	assert.Equal(t, RoleArbiter, u.Role)

	list, err := e.svc.AdminListUsers(ctx, admin, "memb", 20, "")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, m.ID, list.Items[0].ID)
}

func TestDeleteAccount(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d := completed(t, e)
	_, err := e.svc.LeaveReview(ctx, d.sami, d.x.ID, 5, "Top")
	require.NoError(t, err)

	// Échange en cours : suppression refusée.
	busy := newDeliverable(t, e)
	err = e.svc.DeleteAccount(ctx, busy.lea)
	requireKind(t, err, KindConflict, "echanges-en-cours")

	samiIdentity := GitHubIdentity{ID: githubID(t, e, d.sami), Login: "sami", CreatedAt: e.clock.Now().AddDate(-2, 0, 0)}
	samiSession := e.loginAs(t, samiIdentity) // une session ouverte, révoquée par la suppression
	before := balanceOf(t, e, d.sami)
	err = e.svc.DeleteAccount(ctx, d.sami)
	require.NoError(t, err)

	var pseudo, login string
	var email, bio *string
	var status string
	require.NoError(t, e.db.QueryRow(ctx, `SELECT pseudo, email, bio, github_login, statut_utilisateur FROM utilisateur WHERE id_utilisateur = $1`, d.sami.ID).
		Scan(&pseudo, &email, &bio, &login, &status))
	assert.Equal(t, "SUPPRIME", status)
	assert.NotEqual(t, "sami", pseudo)
	assert.Nil(t, email)
	assert.Nil(t, bio)
	assert.NotEqual(t, "sami", login)
	_, err = e.svc.PublicProfile(ctx, d.sami.ID)
	requireKind(t, err, KindNotFound, "")
	_, err = e.svc.Refresh(ctx, samiSession.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "")

	// Le grand livre reste intact : les écritures ne bougent pas.
	assert.Equal(t, before, balanceOf(t, e, d.sami))
	requireLedgerConsistent(t, e)

	// Les avis laissés restent, signés du pseudo anonymisé.
	reviews, err := e.svc.ListReviews(ctx, d.lea.ID, 20, "")
	require.NoError(t, err)
	assert.Equal(t, pseudo, reviews.Items[0].AuthorPseudo)

	// Réinscription avec le même compte GitHub : compte neuf, sans second bonus.
	again := e.loginAs(t, samiIdentity)
	assert.NotEqual(t, d.sami.ID, again.User.ID)
	assert.Equal(t, int64(0), balanceOf(t, e, e.actor(t, again)).Available)
}
