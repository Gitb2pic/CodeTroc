package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeGitHubServer(t *testing.T, emails string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "client", r.PostForm.Get("client_id"))
		assert.Equal(t, "secret", r.PostForm.Get("client_secret"))
		w.Header().Set("Content-Type", "application/json")
		if r.PostForm.Get("code") != "bon-code" {
			_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"gho_abc","token_type":"bearer"}`))
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer gho_abc", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"id":4242,"login":"leamartin","created_at":"2019-03-01T10:00:00Z"}`))
	})
	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(emails))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newOAuth(srv *httptest.Server) *OAuth {
	return &OAuth{ClientID: "client", ClientSecret: "secret", RedirectURL: "https://codetroc.test/api/v1/auth/github/callback",
		WebBase: srv.URL, APIBase: srv.URL, HTTP: srv.Client()}
}

func TestAuthorizeURL(t *testing.T) {
	o := &OAuth{ClientID: "client", RedirectURL: "https://codetroc.test/cb", WebBase: "https://github.com"}
	u, err := url.Parse(o.AuthorizeURL("etat-123"))
	require.NoError(t, err)
	assert.Equal(t, "github.com", u.Host)
	assert.Equal(t, "/login/oauth/authorize", u.Path)
	q := u.Query()
	assert.Equal(t, "client", q.Get("client_id"))
	assert.Equal(t, "etat-123", q.Get("state"))
	assert.Equal(t, "https://codetroc.test/cb", q.Get("redirect_uri"))
	assert.Equal(t, "read:user user:email", q.Get("scope"))
}

func TestExchange(t *testing.T) {
	srv := fakeGitHubServer(t, `[{"email":"autre@example.com","primary":false,"verified":true},
		{"email":"lea@example.com","primary":true,"verified":true}]`)
	id, err := newOAuth(srv).Exchange(context.Background(), "bon-code")
	require.NoError(t, err)
	assert.Equal(t, int64(4242), id.ID)
	assert.Equal(t, "leamartin", id.Login)
	assert.Equal(t, "lea@example.com", id.Email)
	assert.Equal(t, time.Date(2019, 3, 1, 10, 0, 0, 0, time.UTC), id.CreatedAt)
}

func TestExchange_IgnoresUnverifiedEmail(t *testing.T) {
	srv := fakeGitHubServer(t, `[{"email":"lea@example.com","primary":true,"verified":false}]`)
	id, err := newOAuth(srv).Exchange(context.Background(), "bon-code")
	require.NoError(t, err)
	assert.Empty(t, id.Email)
}

func TestExchange_EmailEndpointFailureIsNotFatal(t *testing.T) {
	srv := fakeGitHubServer(t, `pas du json`)
	id, err := newOAuth(srv).Exchange(context.Background(), "bon-code")
	require.NoError(t, err)
	assert.Empty(t, id.Email)
}

func TestExchange_BadCode(t *testing.T) {
	srv := fakeGitHubServer(t, `[]`)
	_, err := newOAuth(srv).Exchange(context.Background(), "mauvais")
	require.ErrorContains(t, err, "bad_verification_code")
}
