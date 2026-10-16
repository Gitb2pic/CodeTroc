package httpapi

import (
	"context"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type memMailer struct {
	mu   sync.Mutex
	sent []app.Email
}

func (m *memMailer) Send(_ context.Context, e app.Email) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *memMailer) lastToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	require.NotEmpty(t, m.sent)
	match := regexp.MustCompile(`jeton=([A-Za-z0-9_-]+)`).FindStringSubmatch(m.sent[len(m.sent)-1].Text)
	require.NotNil(t, match)
	return match[1]
}

func TestEmailAuthAPI_RegisterConfirmLogin(t *testing.T) {
	mailer := &memMailer{}
	e := newAPI(t, func(d *app.Deps, _ *Options) { d.Mailer = mailer })

	w := e.do(call{method: http.MethodGet, path: "/api/v1/auth/methodes"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"github":true,"email":true}`, w.Body.String())

	body := map[string]string{"email": "lea@example.com", "mot_de_passe": "un mot de passe solide"}
	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/inscription", body: body})
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/connexion", body: body})
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "/problems/email-non-confirme", problemType(t, w))

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/email/confirmer", body: map[string]string{"jeton": mailer.lastToken(t)}})
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/connexion", body: body})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	refresh := cookieFrom(w, refreshCookie)
	require.NotNil(t, refresh)
	assert.True(t, refresh.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, refresh.SameSite)
	tok := decode[struct {
		AccessToken string   `json:"access_token"`
		User        userJSON `json:"user"`
	}](t, w)
	assert.False(t, tok.User.GitHubLinked)
	assert.True(t, tok.User.EmailVerified)

	// Sans GitHub : pas d'offre ; la liaison renvoie l'URL GitHub et pose le state.
	w = e.do(call{method: http.MethodPost, path: "/api/v1/offres", token: tok.AccessToken, body: map[string]any{
		"id_pack": e.pipelinePackID(), "titre": "x", "description": "x", "prix_credits": 50, "delai_jours": 5,
		"jalons": []map[string]any{{"titre": "Tout", "montant": 50}}}})
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "/problems/github-requis", problemType(t, w))

	w = e.do(call{method: http.MethodPost, path: "/api/v1/me/github/lier", token: tok.AccessToken})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, cookieFrom(w, stateCookie))

	bad := map[string]string{"email": "lea@example.com", "mot_de_passe": "mauvais mot de passe"}
	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/connexion", body: bad})
	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/problems/identifiants-invalides", problemType(t, w))
}

func TestEmailAuthAPI_PasswordReset(t *testing.T) {
	mailer := &memMailer{}
	e := newAPI(t, func(d *app.Deps, _ *Options) { d.Mailer = mailer })
	body := map[string]string{"email": "noe@example.com", "mot_de_passe": "un mot de passe solide"}
	require.Equal(t, http.StatusAccepted, e.do(call{method: http.MethodPost, path: "/api/v1/auth/inscription", body: body}).Code)

	w := e.do(call{method: http.MethodPost, path: "/api/v1/auth/mot-de-passe/oubli", body: map[string]string{"email": "inconnu@example.com"}})
	require.Equal(t, http.StatusAccepted, w.Code)
	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/mot-de-passe/oubli", body: map[string]string{"email": "noe@example.com"}})
	require.Equal(t, http.StatusAccepted, w.Code)

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/mot-de-passe/reinitialiser",
		body: map[string]string{"jeton": mailer.lastToken(t), "mot_de_passe": "nouveau mot de passe"}})
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/connexion", body: map[string]string{"email": "noe@example.com", "mot_de_passe": "nouveau mot de passe"}})
	require.Equal(t, http.StatusOK, w.Code, "le lien a aussi confirmé l'adresse")

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/email/confirmer", body: map[string]string{"jeton": "faux"}})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "/problems/lien-invalide", problemType(t, w))
}

func TestEmailAuthAPI_ClosedWithoutMailer(t *testing.T) {
	e := newAPI(t)
	w := e.do(call{method: http.MethodGet, path: "/api/v1/auth/methodes"})
	assert.JSONEq(t, `{"github":true,"email":false}`, w.Body.String())
	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/inscription", body: map[string]string{"email": "a@b.fr", "mot_de_passe": "un mot de passe solide"}})
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "/problems/inscription-email-indisponible", problemType(t, w))
}
