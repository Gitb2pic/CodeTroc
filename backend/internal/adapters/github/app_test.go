package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

func testKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return k, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
}

func fakeAppServer(t *testing.T, key *rsa.PrivateKey, tokenCalls *atomic.Int32) *httptest.Server {
	t.Helper()
	appJWT := func(r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims := jwt.RegisteredClaims{}
		_, err := jwt.ParseWithClaims(tok, &claims, func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
			jwt.WithValidMethods([]string{"RS256"}))
		require.NoError(t, err, "JWT d'application invalide")
		assert.Equal(t, "123", claims.Issuer)
	}
	inst := func(r *http.Request) {
		assert.Equal(t, "token ghs_install", r.Header.Get("Authorization"))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/acme/shop-api/installation", func(w http.ResponseWriter, r *http.Request) {
		appJWT(r)
		_, _ = w.Write([]byte(`{"id":77}`))
	})
	mux.HandleFunc("GET /repos/autre/depot/installation", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("POST /app/installations/77/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		appJWT(r)
		tokenCalls.Add(1)
		_, _ = w.Write([]byte(`{"token":"ghs_install","expires_at":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `"}`))
	})
	mux.HandleFunc("GET /repos/acme/shop-api", func(w http.ResponseWriter, r *http.Request) {
		inst(r)
		_, _ = w.Write([]byte(`{"id":100,"full_name":"acme/shop-api"}`))
	})
	mux.HandleFunc("GET /repos/acme/shop-api/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		inst(r)
		_, _ = w.Write([]byte(`{"id":9001,"number":42,"merged":true,"created_at":"2026-10-14T10:00:00Z",
			"user":{"id":7},"head":{"sha":"a3f9c1e"},"base":{"repo":{"id":100}},"merge_commit_sha":"m1"}`))
	})
	mux.HandleFunc("GET /repos/acme/shop-api/commits/a3f9c1e/check-runs", func(w http.ResponseWriter, r *http.Request) {
		inst(r)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		_, _ = w.Write([]byte(`{"total_count":2,"check_runs":[{"name":"lint","status":"completed","conclusion":"success"},
			{"name":"test","status":"in_progress","conclusion":null}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestApp(t *testing.T) (*App, *atomic.Int32) {
	t.Helper()
	key, pemKey := testKey(t)
	var calls atomic.Int32
	srv := fakeAppServer(t, key, &calls)
	a, err := NewApp(123, pemKey)
	require.NoError(t, err)
	a.APIBase, a.HTTP = srv.URL, srv.Client()
	return a, &calls
}

func TestApp_ReadsRepositoryPullAndChecks(t *testing.T) {
	a, calls := newTestApp(t)
	ctx := context.Background()

	repo, err := a.Repository(ctx, "acme", "shop-api")
	require.NoError(t, err)
	assert.Equal(t, app.Repository{ID: 100, FullName: "acme/shop-api"}, repo)

	pr, err := a.PullRequest(ctx, "acme", "shop-api", 42)
	require.NoError(t, err)
	assert.Equal(t, app.PullRequest{ID: 9001, Number: 42, BaseRepoID: 100, AuthorID: 7, Merged: true, HeadSHA: "a3f9c1e", MergeSHA: "m1",
		CreatedAt: time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)}, pr)

	checks, err := a.CheckRuns(ctx, "acme", "shop-api", "a3f9c1e")
	require.NoError(t, err)
	assert.Equal(t, []app.CheckRun{{Name: "lint", Status: "completed", Conclusion: "success"}, {Name: "test", Status: "in_progress"}}, checks)

	assert.Equal(t, int32(1), calls.Load(), "le jeton d'installation est mis en cache")
}

func TestApp_NotInstalledIsNotFound(t *testing.T) {
	a, _ := newTestApp(t)
	_, err := a.Repository(context.Background(), "autre", "depot")
	require.ErrorIs(t, err, app.ErrGitHubNotFound)
}

func TestNewApp_RejectsBadKey(t *testing.T) {
	_, err := NewApp(1, []byte("pas une clé"))
	require.Error(t, err)
}

func TestVerifySignature(t *testing.T) {
	secret := []byte("whsec")
	body := []byte(`{"action":"closed"}`)
	good := Sign(secret, body)
	assert.True(t, VerifySignature(secret, body, good))
	for _, bad := range []string{"", "sha256=", "sha1=" + good[7:], good[:len(good)-1] + "0", "sha256=zz", strings.ToUpper(good)} {
		assert.False(t, VerifySignature(secret, body, bad), bad)
	}
	assert.False(t, VerifySignature(secret, []byte(`{"action":"opened"}`), good), "corps modifié")
	assert.False(t, VerifySignature(nil, body, Sign(nil, body)), "secret vide refusé")
}

func FuzzVerifySignature(f *testing.F) {
	f.Add([]byte("whsec"), []byte("{}"), "sha256=00")
	f.Fuzz(func(t *testing.T, secret, body []byte, sig string) {
		if VerifySignature(secret, body, sig) && sig != Sign(secret, body) {
			t.Fatalf("signature %q acceptée alors qu'elle diffère de la signature attendue", sig)
		}
	})
}
