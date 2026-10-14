package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/api"
	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
	"github.com/gitb2pic/codetroc/backend/internal/testutil"
)

type stubOAuth struct {
	mu  sync.Mutex
	ids map[string]app.GitHubIdentity
}

func (s *stubOAuth) AuthorizeURL(state string) string {
	return "https://github.com/login/oauth/authorize?state=" + url.QueryEscape(state)
}

func (s *stubOAuth) Exchange(_ context.Context, code string) (app.GitHubIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.ids[code]
	if !ok {
		return app.GitHubIdentity{}, errors.New("bad code")
	}
	return id, nil
}

type apiEnv struct {
	t      *testing.T
	router *gin.Engine
	svc    *app.Service
	db     *pgxpool.Pool
	oauth  *stubOAuth
	spec   routers.Router
	now    time.Time
}

var apiGitHubIDs atomic.Int64

func newAPI(t *testing.T, mutate ...func(*app.Deps, *Options)) *apiEnv {
	t.Helper()
	pool, _ := testutil.NewDB(t, postgres.MigrateUp)
	now := time.Now().UTC()
	iss, err := token.NewIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute, func() time.Time { return now })
	require.NoError(t, err)
	oauth := &stubOAuth{ids: map[string]app.GitHubIdentity{}}
	deps := app.Deps{DB: pool, Now: func() time.Time { return now }, Tokens: iss, OAuth: oauth, Settings: app.DefaultSettings()}
	opts := Options{
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), DB: pool, MaxBodyBytes: 1 << 20,
		ServeSpec: true, Spec: api.Spec, FrontendURL: "https://codetroc.test/", CookieSecure: true,
	}
	for _, m := range mutate {
		m(&deps, &opts)
	}
	opts.App = app.New(deps)
	r, err := NewRouter(opts)
	require.NoError(t, err)

	doc := loadSpec(t)
	specRouter, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)
	return &apiEnv{t: t, router: r, svc: opts.App, db: pool, oauth: oauth, spec: specRouter, now: now}
}

type call struct {
	method, path string
	body         any
	token        string
	headers      map[string]string
	cookies      []*http.Cookie
}

// do exécute la requête et vérifie la réponse contre openapi.yaml quand la
// route y est documentée.
func (e *apiEnv) do(c call) *httptest.ResponseRecorder {
	e.t.Helper()
	var body io.Reader
	if c.body != nil {
		if s, ok := c.body.(string); ok {
			body = strings.NewReader(s)
		} else {
			body = jsonBody(c.body)
		}
	}
	req := httptest.NewRequest(c.method, c.path, body)
	if c.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	e.checkContract(req, w)
	return w
}

func (e *apiEnv) checkContract(req *http.Request, w *httptest.ResponseRecorder) {
	e.t.Helper()
	route, params, err := e.spec.FindRoute(req)
	if err != nil {
		return // route non documentée (404 générique, route de test)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}},
		Status:  w.Code,
		Header:  w.Header(),
		Body:    io.NopCloser(strings.NewReader(w.Body.String())),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		e.t.Errorf("réponse hors contrat pour %s %s (%d) : %v\n%s", req.Method, req.URL.Path, w.Code, err, w.Body.String())
	}
}

func cookieFrom(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// member est un utilisateur connecté de bout en bout via l'API.
type member struct {
	ID      string
	Token   string
	Refresh *http.Cookie
	Login   string
}

// login réalise le parcours complet : /login, retour GitHub, /refresh.
func (e *apiEnv) login(ghLogin string) member {
	e.t.Helper()
	id := app.GitHubIdentity{ID: 5000 + apiGitHubIDs.Add(1), Login: ghLogin, CreatedAt: e.now.AddDate(-2, 0, 0)}
	code := fmt.Sprintf("code-%d", id.ID)
	e.oauth.mu.Lock()
	e.oauth.ids[code] = id
	e.oauth.mu.Unlock()

	w := e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/login"})
	require.Equal(e.t, http.StatusFound, w.Code)
	state := cookieFrom(w, stateCookie)
	require.NotNil(e.t, state)

	w = e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/callback?code=" + code + "&state=" + url.QueryEscape(state.Value), cookies: []*http.Cookie{state}})
	require.Equal(e.t, http.StatusFound, w.Code)
	require.Equal(e.t, "https://codetroc.test/?connexion=ok", w.Header().Get("Location"))
	refresh := cookieFrom(w, refreshCookie)
	require.NotNil(e.t, refresh)

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/refresh", cookies: []*http.Cookie{refresh}})
	require.Equal(e.t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	require.NoError(e.t, json.Unmarshal(w.Body.Bytes(), &out))
	return member{ID: out.User.ID, Token: out.AccessToken, Refresh: cookieFrom(w, refreshCookie), Login: ghLogin}
}

func (e *apiEnv) setRole(m member, role string) {
	e.t.Helper()
	_, err := e.db.Exec(context.Background(), "UPDATE utilisateur SET role = $1 WHERE id_utilisateur = $2", role, m.ID)
	require.NoError(e.t, err)
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v), w.Body.String())
	return v
}

func problemType(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	return decode[Problem](t, w).Type
}
