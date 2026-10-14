package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminAPI(t *testing.T) {
	e := newAPI(t)
	admin, bob := e.login("admin"), e.login("bob")
	e.setRole(admin, "ADMIN")
	path := "/api/v1/admin/utilisateurs/" + bob.ID

	// Élévation : un membre ne s'attribue pas de rôle et ne voit pas l'administration.
	assert.Equal(t, http.StatusForbidden, e.do(call{method: http.MethodPatch, path: path, token: bob.Token, body: map[string]any{"role": "ADMIN"}}).Code)
	assert.Equal(t, http.StatusForbidden, e.do(call{method: http.MethodGet, path: "/api/v1/admin/utilisateurs", token: bob.Token}).Code)
	assert.Equal(t, http.StatusForbidden, e.do(call{method: http.MethodGet, path: "/api/v1/admin/litiges", token: bob.Token}).Code)
	assert.Equal(t, http.StatusBadRequest, e.do(call{method: http.MethodPatch, path: "/api/v1/me", token: bob.Token, body: map[string]any{"role": "ADMIN"}}).Code,
		"mass assignment sur le profil")

	w := e.do(call{method: http.MethodGet, path: "/api/v1/admin/utilisateurs?q=bo", token: admin.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"pseudo":"bob"`)

	w = e.do(call{method: http.MethodPatch, path: path, token: admin.Token, body: map[string]any{"statut": "SUSPENDU"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"statut":"SUSPENDU"`)
	assert.Equal(t, http.StatusUnauthorized, e.do(call{method: http.MethodGet, path: "/api/v1/me", token: bob.Token}).Code,
		"suspension effective immédiatement")

	w = e.do(call{method: http.MethodPatch, path: "/api/v1/admin/utilisateurs/" + admin.ID, token: admin.Token, body: map[string]any{"role": "MEMBRE"}})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	w = e.do(call{method: http.MethodPatch, path: path, token: admin.Token, body: map[string]any{"statut": "SUPPRIME"}})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = e.do(call{method: http.MethodGet, path: "/api/v1/admin/litiges", token: admin.Token})
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}

func TestDeleteMeAPI(t *testing.T) {
	e := newAPI(t)
	zoe := e.login("zoe")
	w := e.do(call{method: http.MethodDelete, path: "/api/v1/me", token: zoe.Token})
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Set-Cookie"), "Max-Age=0")
	assert.Equal(t, http.StatusUnauthorized, e.do(call{method: http.MethodGet, path: "/api/v1/me", token: zoe.Token}).Code)
	assert.Equal(t, http.StatusNotFound, e.do(call{method: http.MethodGet, path: "/api/v1/utilisateurs/" + zoe.ID}).Code)
}
