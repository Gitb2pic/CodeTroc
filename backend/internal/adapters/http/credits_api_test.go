package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreditsAPI(t *testing.T) {
	e := newAPI(t)
	m := e.login("lea")

	w := e.do(call{method: http.MethodGet, path: "/api/v1/me/compte", token: m.Token})
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"disponible":50,"sequestre_sortant":0,"sequestre_entrant":0}`, w.Body.String())

	w = e.do(call{method: http.MethodGet, path: "/api/v1/me/compte/mouvements?limit=10", token: m.Token})
	require.Equal(t, http.StatusOK, w.Code)
	page := decode[struct {
		Items []movementJSON `json:"items"`
		Next  *string        `json:"next_cursor"`
	}](t, w)
	require.Len(t, page.Items, 1)
	assert.Equal(t, int64(50), page.Items[0].Amount)
	assert.Nil(t, page.Next)

	w = e.do(call{method: http.MethodGet, path: "/api/v1/me/compte/mouvements?limit=abc", token: m.Token})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = e.do(call{method: http.MethodGet, path: "/api/v1/me/compte/mouvements?cursor=%25%25", token: m.Token})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "/problems/curseur-invalide", problemType(t, w))

	w = e.do(call{method: http.MethodGet, path: "/api/v1/me/compte"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
