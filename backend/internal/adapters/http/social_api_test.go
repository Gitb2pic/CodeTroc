package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completeExchange livre et valide les deux jalons de l'échange.
func (d deliveryEnv) completeExchange() {
	d.t.Helper()
	d.gh.mu.Lock()
	for n, id := range map[int]int64{42: 9001, 43: 9002} {
		pr := d.gh.pulls[42]
		pr.ID, pr.Number, pr.Merged = id, n, true
		d.gh.pulls[n] = pr
	}
	d.gh.mu.Unlock()
	for i, n := range []string{"42", "43"} {
		ms := "/api/v1/jalons/" + d.x.Milestones[i].ID.String()
		w := d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.lea.Token, body: map[string]any{"pr_url": "https://github.com/acme/shop-api/pull/" + n}})
		require.Equal(d.t, http.StatusCreated, w.Code, w.Body.String())
		w = d.do(call{method: http.MethodPost, path: ms + "/valider", token: d.sami.Token, headers: idemKey()})
		require.Equal(d.t, http.StatusOK, w.Code, w.Body.String())
	}
}

func TestReviewsAPI(t *testing.T) {
	d := newDeliveryEnv(t)
	path := "/api/v1/echanges/" + d.x.ID.String() + "/avis"
	review := map[string]any{"note": 5, "commentaire": "Livré en avance."}

	w := d.do(call{method: http.MethodPost, path: path, token: d.sami.Token, body: review})
	assert.Equal(t, http.StatusConflict, w.Code, "échange pas encore terminé")
	assert.Equal(t, "/problems/echange-non-termine", problemType(t, w))

	d.completeExchange()
	w = d.do(call{method: http.MethodPost, path: path, token: d.sami.Token, body: map[string]any{"note": 5, "id_destinataire": d.sami.ID}})
	assert.Equal(t, http.StatusBadRequest, w.Code, "le destinataire n'est pas choisi par le client")
	w = d.do(call{method: http.MethodPost, path: path, token: d.bob.Token, body: review})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = d.do(call{method: http.MethodPost, path: path, token: d.sami.Token, body: review})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, d.lea.ID, decode[reviewJSON](t, w).RecipientID.String())
	w = d.do(call{method: http.MethodPost, path: path, token: d.sami.Token, body: review})
	assert.Equal(t, http.StatusConflict, w.Code)

	w = d.do(call{method: http.MethodGet, path: "/api/v1/utilisateurs/" + d.lea.ID + "/avis"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Livré en avance.")
	w = d.do(call{method: http.MethodGet, path: "/api/v1/utilisateurs/" + d.lea.ID})
	assert.Contains(t, w.Body.String(), `"score_reputation":5`)
}

func TestMessagesAPI(t *testing.T) {
	d := newDeliveryEnv(t)
	path := "/api/v1/echanges/" + d.x.ID.String() + "/messages"

	w := d.do(call{method: http.MethodPost, path: path, token: d.sami.Token, body: map[string]any{"contenu": "<script>alert(1)</script>"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "<script>alert(1)</script>", decode[messageJSON](t, w).Text, "stocké tel quel, échappé à l'affichage par le front")
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodPost, path: path, token: d.bob.Token, body: map[string]any{"contenu": "Salut"}}).Code)
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodGet, path: path, token: d.bob.Token}).Code)
	assert.Equal(t, http.StatusUnauthorized, d.do(call{method: http.MethodGet, path: path}).Code)

	w = d.do(call{method: http.MethodGet, path: path, token: d.lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	page := decode[struct{ Items []messageJSON }](t, w)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "sami", page.Items[0].AuthorPseudo)
}
