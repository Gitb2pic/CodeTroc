package httpapi

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

func TestAuthAPI_LoginFlowAndCookies(t *testing.T) {
	e := newAPI(t)

	w := e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/login"})
	require.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	state := cookieFrom(w, stateCookie)
	require.NotNil(t, state)
	assert.Equal(t, state.Value, loc.Query().Get("state"))
	assert.True(t, state.HttpOnly)
	assert.True(t, state.Secure)
	assert.Equal(t, http.SameSiteLaxMode, state.SameSite)

	m := e.login("leamartin")
	assert.True(t, m.Refresh.HttpOnly)
	assert.True(t, m.Refresh.Secure)
	assert.Equal(t, http.SameSiteStrictMode, m.Refresh.SameSite)
	assert.Equal(t, "/api/v1/auth", m.Refresh.Path)

	w = e.do(call{method: http.MethodGet, path: "/api/v1/me", token: m.Token})
	require.Equal(t, http.StatusOK, w.Code)
	me := decode[userJSON](t, w)
	assert.Equal(t, "leamartin", me.Pseudo)
	assert.Equal(t, app.RoleMember, me.Role)
}

func TestAuthAPI_CallbackRejectsForgedState(t *testing.T) {
	e := newAPI(t)
	w := e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/login"})
	state := cookieFrom(w, stateCookie)

	// L'attaquant envoie sa propre URL de retour à la victime : le cookie de la
	// victime ne correspond pas au state de l'attaquant.
	w = e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/callback?code=x&state=autre", cookies: []*http.Cookie{state}})
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://codetroc.test/?connexion=etat-oauth-invalide", w.Header().Get("Location"))
	assert.Nil(t, cookieFrom(w, refreshCookie))

	w = e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/callback?error=access_denied"})
	assert.Equal(t, "https://codetroc.test/?connexion=acces-refuse", w.Header().Get("Location"))
}

func TestAuthAPI_RefreshRotationAndReuse(t *testing.T) {
	e := newAPI(t)
	m := e.login("rotation")

	w := e.do(call{method: http.MethodPost, path: "/api/v1/auth/refresh", cookies: []*http.Cookie{m.Refresh}})
	require.Equal(t, http.StatusOK, w.Code)
	next := cookieFrom(w, refreshCookie)
	require.NotEqual(t, m.Refresh.Value, next.Value)

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/refresh", cookies: []*http.Cookie{m.Refresh}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/problems/jeton-reutilise", problemType(t, w))
	assert.Equal(t, -1, cookieFrom(w, refreshCookie).MaxAge, "le cookie est effacé")

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/refresh", cookies: []*http.Cookie{next}})
	assert.Equal(t, http.StatusUnauthorized, w.Code, "toute la famille est révoquée")

	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/refresh"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthAPI_Logout(t *testing.T) {
	e := newAPI(t)
	m := e.login("depart")
	w := e.do(call{method: http.MethodPost, path: "/api/v1/auth/logout", cookies: []*http.Cookie{m.Refresh}})
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = e.do(call{method: http.MethodPost, path: "/api/v1/auth/refresh", cookies: []*http.Cookie{m.Refresh}})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthAPI_MeRequiresValidToken(t *testing.T) {
	e := newAPI(t)
	m := e.login("moi")

	for name, h := range map[string]string{
		"sans en-tête":    "",
		"schéma Basic":    "Basic " + m.Token,
		"jeton illisible": "Bearer abc",
		"jeton vide":      "Bearer ",
	} {
		t.Run(name, func(t *testing.T) {
			headers := map[string]string{}
			if h != "" {
				headers["Authorization"] = h
			}
			w := e.do(call{method: http.MethodGet, path: "/api/v1/me", headers: headers})
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, `Bearer realm="codetroc"`, w.Header().Get("WWW-Authenticate"))
			assert.Equal(t, "/problems/non-authentifie", problemType(t, w))
		})
	}

	_, err := e.db.Exec(t.Context(), "UPDATE utilisateur SET statut_utilisateur = 'SUSPENDU' WHERE id_utilisateur = $1", m.ID)
	require.NoError(t, err)
	w := e.do(call{method: http.MethodGet, path: "/api/v1/me", token: m.Token})
	assert.Equal(t, http.StatusUnauthorized, w.Code, "suspension appliquée sans attendre l'expiration du jeton")
}

func TestAuthAPI_GitHubNotConfigured(t *testing.T) {
	e := newAPI(t, func(d *app.Deps, _ *Options) { d.OAuth = nil })
	w := e.do(call{method: http.MethodGet, path: "/api/v1/auth/github/login"})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "/problems/github-non-configure", problemType(t, w))
}
