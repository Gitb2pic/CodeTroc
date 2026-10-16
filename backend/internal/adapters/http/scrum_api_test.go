package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrumAPI(t *testing.T) {
	e := newAPI(t)
	lea, sami, tiers := e.login("lea"), e.login("sami"), e.login("tiers")
	e.fund(sami, 310)
	x := e.requestExchange(sami, e.publishOffer(lea, 120).ID.String())
	w := e.do(call{method: http.MethodPost, path: "/api/v1/echanges/" + x.ID.String() + "/accepter", token: lea.Token, headers: idemKey()})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	base := "/api/v1/echanges/" + x.ID.String() + "/scrum"

	w = e.do(call{method: http.MethodGet, path: base, token: sami.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	p := decode[projectJSON](t, w)
	assert.Equal(t, "PRODUCT_OWNER", string(p.Role))
	assert.Equal(t, "INACTIVE", p.SyncStatus, "GitHub non configuré dans le banc d'essai")
	w = e.do(call{method: http.MethodGet, path: base, token: tiers.Token})
	assert.Equal(t, http.StatusNotFound, w.Code, "BOLA")

	w = e.do(call{method: http.MethodPatch, path: base, token: sami.Token, body: map[string]any{"objectif_produit": "Boutique"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = e.do(call{method: http.MethodGet, path: base + "/planning", token: lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = e.do(call{method: http.MethodGet, path: base + "/sprints", token: lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	sprints := decode[struct{ Items []sprintJSON }](t, w).Items
	require.NotEmpty(t, sprints)
	assert.Equal(t, "ACTIF", string(sprints[0].Status))

	w = e.do(call{method: http.MethodPost, path: base + "/stories", token: sami.Token, body: map[string]any{"titre": "Connexion GitHub"}})
	assert.Equal(t, http.StatusBadRequest, w.Code, "Idempotency-Key obligatoire")
	w = e.do(call{method: http.MethodPost, path: base + "/stories", token: sami.Token, headers: idemKey(),
		body: map[string]any{"titre": "Connexion GitHub", "criteres": []string{"Given/When/Then"}, "sprint": 1}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	st := decode[storyJSON](t, w)
	assert.Equal(t, "A_FAIRE", string(st.Status))
	story := base + "/stories/" + st.ID.String()

	// Affectation de masse et séparation des rôles.
	w = e.do(call{method: http.MethodPatch, path: story, token: sami.Token, body: map[string]any{"github_issue": 9}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = e.do(call{method: http.MethodPatch, path: story, token: sami.Token, body: map[string]any{"points": 3}})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = e.do(call{method: http.MethodPatch, path: story, token: lea.Token, body: map[string]any{"priorite": 2}})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = e.do(call{method: http.MethodPatch, path: story, token: lea.Token, body: map[string]any{"points": 3, "statut": "EN_COURS"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = e.do(call{method: http.MethodPatch, path: story, token: sami.Token, body: map[string]any{"sprint": nil}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := decode[storyJSON](t, w)
	assert.Equal(t, "BACKLOG", string(got.Status))
	assert.Nil(t, got.SprintNumber)
	w = e.do(call{method: http.MethodGet, path: base + "/stories?sprint=backlog", token: lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, decode[struct{ Items []storyJSON }](t, w).Items, 1)

	w = e.do(call{method: http.MethodPatch, path: base + "/sprints/1", token: lea.Token, body: map[string]any{"objectif": "Auth"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = e.do(call{method: http.MethodPut, path: base + "/sprints/1/retro", token: lea.Token, body: map[string]any{"bien": "x"}})
	assert.Equal(t, http.StatusConflict, w.Code)
	w = e.do(call{method: http.MethodPatch, path: base + "/sprints/abc", token: lea.Token, body: map[string]any{"objectif": "x"}})
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Avenant : proposé par Léa, refusé par Sami.
	deadlines := make([]time.Time, len(x.Milestones))
	start := time.Now().UTC().Add(24 * time.Hour)
	for i := range deadlines {
		deadlines[i] = start.AddDate(0, 0, 10*(i+1))
	}
	w = e.do(call{method: http.MethodPost, path: base + "/avenants", token: lea.Token, headers: idemKey(),
		body: map[string]any{"echeances": deadlines, "motif": "Retard"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	am := decode[amendmentJSON](t, w)
	w = e.do(call{method: http.MethodPost, path: base + "/avenants/" + am.ID.String() + "/accepter", token: lea.Token, headers: idemKey()})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = e.do(call{method: http.MethodPost, path: base + "/avenants/" + am.ID.String() + "/refuser", token: sami.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "REFUSE", decode[amendmentJSON](t, w).Status)

	w = e.do(call{method: http.MethodPost, path: base + "/synchro", token: lea.Token, headers: idemKey()})
	assert.Equal(t, http.StatusConflict, w.Code, "synchro indisponible sans GitHub App")
}
