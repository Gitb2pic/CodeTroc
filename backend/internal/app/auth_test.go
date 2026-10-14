package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogin_CreatesThenUpdatesUser(t *testing.T) {
	e := newEnv(t)
	id := GitHubIdentity{ID: 42, Login: "leamartin", Email: "lea@example.com", CreatedAt: e.clock.Now().AddDate(-3, 0, 0)}

	first := e.loginAs(t, id)
	assert.Equal(t, "leamartin", first.User.Pseudo)
	assert.Equal(t, "lea@example.com", *first.User.Email)
	assert.Equal(t, RoleMember, first.User.Role)
	assert.NotEmpty(t, first.AccessToken)
	assert.NotEmpty(t, first.RefreshToken)

	id.Login = "lea-renamed"
	second := e.loginAs(t, id)
	assert.Equal(t, first.User.ID, second.User.ID, "même compte GitHub = même utilisateur")
	assert.Equal(t, "lea-renamed", second.User.GitHubLogin)
	assert.Equal(t, "leamartin", second.User.Pseudo, "le pseudo choisi n'est pas écrasé")
}

func TestLogin_WithoutEmail(t *testing.T) {
	e := newEnv(t)
	s := e.loginAs(t, GitHubIdentity{ID: 7, Login: "discret", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)})
	assert.Nil(t, s.User.Email)
}

func TestLogin_PseudoCollisionGetsSuffix(t *testing.T) {
	e := newEnv(t)
	a := e.loginAs(t, GitHubIdentity{ID: 1, Login: "sami", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)})
	_, err := e.db.Exec(context.Background(), "UPDATE utilisateur SET github_login = 'ancien' WHERE id_utilisateur = $1", a.User.ID)
	require.NoError(t, err)

	b := e.loginAs(t, GitHubIdentity{ID: 2, Login: "sami", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)})
	assert.NotEqual(t, a.User.Pseudo, b.User.Pseudo)
	assert.Regexp(t, `^sami-[0-9a-f]{4}$`, b.User.Pseudo)
}

func TestLogin_StateChecks(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.oauth.add("code", GitHubIdentity{ID: 9, Login: "x", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)})

	_, state, err := e.svc.StartLogin(ctx)
	require.NoError(t, err)

	_, err = e.svc.CompleteLogin(ctx, state, "", "code")
	requireKind(t, err, KindUnauthenticated, "etat-oauth-invalide")
	_, err = e.svc.CompleteLogin(ctx, state, state+"x", "code")
	requireKind(t, err, KindUnauthenticated, "etat-oauth-invalide")
	_, err = e.svc.CompleteLogin(ctx, "inconnu", "inconnu", "code")
	requireKind(t, err, KindUnauthenticated, "etat-oauth-invalide")

	_, err = e.svc.CompleteLogin(ctx, state, state, "code")
	require.NoError(t, err)
	_, err = e.svc.CompleteLogin(ctx, state, state, "code")
	requireKind(t, err, KindUnauthenticated, "etat-oauth-invalide") // rejoué

	_, expired, err := e.svc.StartLogin(ctx)
	require.NoError(t, err)
	e.clock.Advance(11 * time.Minute)
	_, err = e.svc.CompleteLogin(ctx, expired, expired, "code")
	requireKind(t, err, KindUnauthenticated, "etat-oauth-invalide")
}

func TestLogin_BadCode(t *testing.T) {
	e := newEnv(t)
	_, state, err := e.svc.StartLogin(context.Background())
	require.NoError(t, err)
	_, err = e.svc.CompleteLogin(context.Background(), state, state, "faux")
	requireKind(t, err, KindUnauthenticated, "github-refuse")
}

func TestLogin_TooRecentGitHubAccount(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.Settings.MinGitHubAccountAge = 30 * 24 * time.Hour })
	ctx := context.Background()
	e.oauth.add("code", GitHubIdentity{ID: 11, Login: "jetable", CreatedAt: e.clock.Now().AddDate(0, 0, -3)})
	_, state, err := e.svc.StartLogin(ctx)
	require.NoError(t, err)
	_, err = e.svc.CompleteLogin(ctx, state, state, "code")
	requireKind(t, err, KindForbidden, "compte-github-trop-recent")
}

func TestLogin_SuspendedAccount(t *testing.T) {
	e := newEnv(t)
	id := GitHubIdentity{ID: 12, Login: "suspendu", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)}
	s := e.loginAs(t, id)
	_, err := e.db.Exec(context.Background(), "UPDATE utilisateur SET statut_utilisateur = 'SUSPENDU' WHERE id_utilisateur = $1", s.User.ID)
	require.NoError(t, err)

	e.oauth.add("code2", id)
	_, state, _ := e.svc.StartLogin(context.Background())
	_, err = e.svc.CompleteLogin(context.Background(), state, state, "code2")
	requireKind(t, err, KindForbidden, "compte-suspendu")

	_, err = e.svc.Authenticate(context.Background(), s.AccessToken)
	requireKind(t, err, KindUnauthenticated, "")
	_, err = e.svc.Refresh(context.Background(), s.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "")
}

func TestLogin_ConcurrentFirstLoginCreatesOneUser(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := GitHubIdentity{ID: 77, Login: "rapide", CreatedAt: e.clock.Now().AddDate(-1, 0, 0)}

	const n = 8
	states := make([]string, n)
	for i := range states {
		e.oauth.add("c"+string(rune('a'+i)), id)
		_, st, err := e.svc.StartLogin(ctx)
		require.NoError(t, err)
		states[i] = st
	}
	var wg sync.WaitGroup
	ids := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := e.svc.CompleteLogin(ctx, states[i], states[i], "c"+string(rune('a'+i)))
			errs[i] = err
			ids[i] = s.User.ID.String()
		}(i)
	}
	wg.Wait()
	for i := range errs {
		require.NoError(t, errs[i])
		assert.Equal(t, ids[0], ids[i])
	}
	var count int
	require.NoError(t, e.db.QueryRow(ctx, "SELECT count(*) FROM utilisateur WHERE github_id = 77").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestRefresh_RotatesAndDetectsReuse(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.login(t, "rotation")

	s2, err := e.svc.Refresh(ctx, s.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, s.RefreshToken, s2.RefreshToken)
	assert.Equal(t, s.User.ID, s2.User.ID)

	// L'ancien jeton est rejoué : vol probable, toute la famille tombe.
	_, err = e.svc.Refresh(ctx, s.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "jeton-reutilise")
	_, err = e.svc.Refresh(ctx, s2.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "")
}

func TestRefresh_ExpiredAndUnknown(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.login(t, "expire")

	_, err := e.svc.Refresh(ctx, "inconnu")
	requireKind(t, err, KindUnauthenticated, "")

	e.clock.Advance(31 * 24 * time.Hour)
	_, err = e.svc.Refresh(ctx, s.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "")
}

func TestLogout_RevokesFamily(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.login(t, "depart")
	s2, err := e.svc.Refresh(ctx, s.RefreshToken)
	require.NoError(t, err)

	require.NoError(t, e.svc.Logout(ctx, s2.RefreshToken))
	_, err = e.svc.Refresh(ctx, s2.RefreshToken)
	requireKind(t, err, KindUnauthenticated, "")
	require.NoError(t, e.svc.Logout(ctx, "inconnu"), "déconnexion idempotente")
}

func TestAuthenticateAndMe(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.login(t, "moi")

	a, err := e.svc.Authenticate(ctx, s.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.User.ID, a.ID)
	assert.Equal(t, RoleMember, a.Role)

	me, err := e.svc.Me(ctx, a)
	require.NoError(t, err)
	assert.Equal(t, "moi", me.Pseudo)

	_, err = e.svc.Authenticate(ctx, "faux")
	requireKind(t, err, KindUnauthenticated, "non-authentifie")
	e.clock.Advance(16 * time.Minute)
	_, err = e.svc.Authenticate(ctx, s.AccessToken)
	requireKind(t, err, KindUnauthenticated, "non-authentifie")
}

func TestStartLogin_WithoutGitHub(t *testing.T) {
	e := newEnv(t, func(d *Deps) { d.OAuth = nil })
	_, _, err := e.svc.StartLogin(context.Background())
	requireKind(t, err, KindUnavailable, "github-non-configure")
}
