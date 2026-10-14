package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
	"github.com/gitb2pic/codetroc/backend/internal/testutil"
)

// clock est une horloge de test réglable.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeOAuth simule GitHub : chaque code renvoie l'identité enregistrée.
type fakeOAuth struct {
	mu         sync.Mutex
	identities map[string]GitHubIdentity
}

func (f *fakeOAuth) AuthorizeURL(state string) string {
	return "https://github.com/login/oauth/authorize?state=" + state
}

func (f *fakeOAuth) Exchange(_ context.Context, code string) (GitHubIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.identities[code]
	if !ok {
		return GitHubIdentity{}, errors.New("bad_verification_code")
	}
	return id, nil
}

func (f *fakeOAuth) add(code string, id GitHubIdentity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.identities[code] = id
}

type env struct {
	svc   *Service
	db    *pgxpool.Pool
	clock *clock
	oauth *fakeOAuth
	gh    *fakeGitHub
}

var githubIDs atomic.Int64

func newEnv(t *testing.T, mutate ...func(*Deps)) *env {
	t.Helper()
	pool, _ := testutil.NewDB(t, postgres.MigrateUp)
	c := &clock{t: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	iss, err := token.NewIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute, c.Now)
	require.NoError(t, err)
	oauth := &fakeOAuth{identities: map[string]GitHubIdentity{}}
	gh := newFakeGitHub()
	d := Deps{DB: pool, Now: c.Now, Tokens: iss, OAuth: oauth, GitHub: gh, Settings: DefaultSettings()}
	for _, m := range mutate {
		m(&d)
	}
	return &env{svc: New(d), db: pool, clock: c, oauth: oauth, gh: gh}
}

// login connecte un nouvel utilisateur GitHub et renvoie sa session.
func (e *env) login(t *testing.T, ghLogin string) Session {
	t.Helper()
	id := GitHubIdentity{ID: 1000 + githubIDs.Add(1), Login: ghLogin, CreatedAt: e.clock.Now().AddDate(-2, 0, 0)}
	return e.loginAs(t, id)
}

func (e *env) loginAs(t *testing.T, id GitHubIdentity) Session {
	t.Helper()
	ctx := context.Background()
	code := fmt.Sprintf("code-%d-%d", id.ID, githubIDs.Add(1))
	e.oauth.add(code, id)
	_, state, err := e.svc.StartLogin(ctx)
	require.NoError(t, err)
	sess, err := e.svc.CompleteLogin(ctx, state, state, code)
	require.NoError(t, err)
	return sess
}

func (e *env) actor(t *testing.T, s Session) Actor {
	t.Helper()
	a, err := e.svc.Authenticate(context.Background(), s.AccessToken)
	require.NoError(t, err)
	return a
}

func (e *env) setRole(t *testing.T, a Actor, r Role) Actor {
	t.Helper()
	_, err := e.db.Exec(context.Background(), "UPDATE utilisateur SET role = $1 WHERE id_utilisateur = $2", r, a.ID)
	require.NoError(t, err)
	a.Role = r
	return a
}

// requireKind vérifie le type et le code d'une erreur métier.
func requireKind(t *testing.T, err error, k Kind, code string) {
	t.Helper()
	var ae *Error
	require.ErrorAs(t, err, &ae)
	require.Equal(t, k, ae.Kind, "kind de %v", err)
	if code != "" {
		require.Equal(t, code, ae.Code)
	}
}
