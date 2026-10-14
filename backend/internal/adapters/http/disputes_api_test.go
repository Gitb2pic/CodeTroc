package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisputesAPI(t *testing.T) {
	d := newDeliveryEnv(t)
	arb := d.login("arbitre")
	d.setRole(arb, "ARBITRE")
	d.gh.mu.Lock()
	pr := d.gh.pulls[42]
	pr.Merged = true
	d.gh.pulls[42] = pr
	d.gh.mu.Unlock()
	ms := "/api/v1/jalons/" + d.x.Milestones[0].ID.String()
	require.Equal(t, http.StatusCreated, d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.lea.Token,
		body: map[string]any{"pr_url": "https://github.com/acme/shop-api/pull/42"}}).Code)

	w := d.do(call{method: http.MethodPost, path: ms + "/litiges", token: d.bob.Token, body: map[string]any{"motif": "Je ne suis pas concerné mais je conteste."}})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = d.do(call{method: http.MethodPost, path: ms + "/litiges", token: d.sami.Token, body: map[string]any{"motif": "Le critère 4 n'est pas rempli : la rotation échoue."}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	dp := decode[disputeJSON](t, w)
	require.NotNil(t, dp.ArbiterID)
	assert.Equal(t, arb.ID, dp.ArbiterID.String())
	path := "/api/v1/litiges/" + dp.ID.String()

	// L'arbitre lit l'échange (RG22), un tiers non.
	assert.Equal(t, http.StatusOK, d.do(call{method: http.MethodGet, path: "/api/v1/echanges/" + d.x.ID.String(), token: arb.Token}).Code)
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodGet, path: path, token: d.bob.Token}).Code)

	w = d.do(call{method: http.MethodPost, path: path + "/reponse", token: d.lea.Token, body: map[string]any{"reponse": "Le test dépend d'une horloge non mockée."}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	dec := map[string]any{"decision": "PARTAGER", "part_prestataire": 60, "justification": "Le travail livré reste exploitable, un critère explicite manque."}
	w = d.do(call{method: http.MethodPost, path: path + "/decision", token: d.sami.Token, body: dec, headers: idemKey()})
	assert.Equal(t, http.StatusNotFound, w.Code, "une partie ne tranche pas son propre litige")
	w = d.do(call{method: http.MethodPost, path: path + "/decision", token: arb.Token, body: dec, headers: idemKey()})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "CLOS", decode[disputeJSON](t, w).Status)

	w = d.do(call{method: http.MethodGet, path: "/api/v1/me/compte", token: d.lea.Token})
	assert.JSONEq(t, `{"disponible":86,"sequestre_sortant":0,"sequestre_entrant":60}`, w.Body.String(), "60 % de 60 cr = 36 cr")

	w = d.do(call{method: http.MethodGet, path: "/api/v1/litiges", token: arb.Token})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), dp.ID.String())

	// Élévation : un membre ne peut pas s'affecter un litige.
	w = d.do(call{method: http.MethodPost, path: path + "/affecter", token: d.bob.Token, body: map[string]any{"id_arbitre": d.bob.ID}})
	assert.Equal(t, http.StatusForbidden, w.Code)
}
