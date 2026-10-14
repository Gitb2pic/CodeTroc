package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationsAPI(t *testing.T) {
	d := newDeliveryEnv(t)
	msgs := "/api/v1/echanges/" + d.x.ID.String() + "/messages"
	require.Equal(t, http.StatusCreated, d.do(call{method: http.MethodPost, path: msgs, token: d.sami.Token, body: map[string]any{"contenu": "Hello"}}).Code)

	w := d.do(call{method: http.MethodGet, path: "/api/v1/notifications/compteur", token: d.lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"non_lues":2}`, w.Body.String(), "demande d'échange + message")

	w = d.do(call{method: http.MethodGet, path: "/api/v1/notifications?non_lues=true", token: d.lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	page := decode[struct{ Items []notificationJSON }](t, w)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "MESSAGE_RECU", page.Items[0].Kind)

	// Un autre membre ne peut pas marquer les notifications de lea.
	w = d.do(call{method: http.MethodPost, path: "/api/v1/notifications/lire", token: d.bob.Token, body: map[string]any{"ids": []string{page.Items[0].ID.String()}}})
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"marquees":0}`, w.Body.String())

	w = d.do(call{method: http.MethodPost, path: "/api/v1/notifications/lire", token: d.lea.Token, body: map[string]any{}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"marquees":2}`, w.Body.String())
	assert.Equal(t, http.StatusBadRequest, d.do(call{method: http.MethodPost, path: "/api/v1/notifications/lire", token: d.lea.Token, body: map[string]any{"ids": []string{"pas-un-uuid"}}}).Code)
	assert.Equal(t, http.StatusUnauthorized, d.do(call{method: http.MethodGet, path: "/api/v1/notifications"}).Code)
}
