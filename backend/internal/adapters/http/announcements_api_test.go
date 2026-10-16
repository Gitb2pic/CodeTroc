package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (e *apiEnv) publishAnnouncement(m member, budget int64) announcementJSON {
	e.t.Helper()
	half := budget / 2
	w := e.do(call{method: http.MethodPost, path: "/api/v1/annonces", token: m.Token, body: map[string]any{
		"id_pack": e.pipelinePackID(), "titre": "Pipeline CI pour notre API", "besoin": "Lint, tests et image Docker.",
		"depot_url": "https://github.com/acme/shop-api", "budget_credits": budget, "delai_jours": 20, "duree_sprint_jours": 14,
		"jalons": []map[string]any{{"titre": "CI", "montant": half}, {"titre": "Docker", "montant": budget - half}},
	}})
	require.Equal(e.t, http.StatusCreated, w.Code, w.Body.String())
	return decode[announcementJSON](e.t, w)
}

func (e *apiEnv) applyTo(m member, annID string, price int64) applicationJSON {
	e.t.Helper()
	w := e.do(call{method: http.MethodPost, path: "/api/v1/annonces/" + annID + "/candidatures", token: m.Token, body: map[string]any{
		"prix_credits": price, "delai_jours": 15, "message": "Dispo dès lundi.",
		"jalons": []map[string]any{{"titre": "Tout", "montant": price}},
	}})
	require.Equal(e.t, http.StatusCreated, w.Code, w.Body.String())
	return decode[applicationJSON](e.t, w)
}

func TestAnnouncementsAPI_Lifecycle(t *testing.T) {
	e := newAPI(t)
	sami, lea, noe, tiers := e.login("sami"), e.login("lea"), e.login("noe"), e.login("tiers")
	e.fund(sami, 310)
	ann := e.publishAnnouncement(sami, 120)
	assert.Equal(t, 14, ann.SprintDays)
	path := "/api/v1/annonces/" + ann.ID.String()

	// Liste publique, sans authentification.
	w := e.do(call{method: http.MethodGet, path: "/api/v1/annonces"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = e.do(call{method: http.MethodGet, path: "/api/v1/annonces?mes=true"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// RG09 et affectation de masse.
	w = e.do(call{method: http.MethodPost, path: path + "/candidatures", token: sami.Token, body: map[string]any{
		"prix_credits": 100, "delai_jours": 10, "jalons": []map[string]any{{"titre": "x", "montant": 100}}}})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	w = e.do(call{method: http.MethodPost, path: path + "/candidatures", token: lea.Token, body: map[string]any{
		"prix_credits": 100, "delai_jours": 10, "statut": "ACCEPTEE", "jalons": []map[string]any{{"titre": "x", "montant": 100}}}})
	assert.Equal(t, http.StatusBadRequest, w.Code, "champ inconnu refusé")

	la := e.applyTo(lea, ann.ID.String(), 100)
	e.applyTo(noe, ann.ID.String(), 110)

	// BOLA : un tiers ne décide pas, un candidat ne voit que sa candidature.
	accept := path + "/candidatures/" + la.ID.String() + "/accepter"
	w = e.do(call{method: http.MethodPost, path: accept, token: tiers.Token, headers: idemKey()})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = e.do(call{method: http.MethodPost, path: accept, token: sami.Token})
	assert.Equal(t, "/problems/cle-idempotence-requise", problemType(t, w))
	w = e.do(call{method: http.MethodGet, path: path + "/candidatures", token: noe.Token})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode[struct{ Items []applicationJSON }](t, w).Items, 1)

	w = e.do(call{method: http.MethodPost, path: accept, token: sami.Token, headers: idemKey()})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	x := decode[exchangeJSON](t, w)
	assert.Equal(t, "EN_COURS", string(x.Status))
	require.NotNil(t, x.AnnouncementID)
	assert.Nil(t, x.Offer.ID)

	w = e.do(call{method: http.MethodGet, path: path, token: sami.Token})
	require.Equal(t, http.StatusOK, w.Code)
	got := decode[announcementJSON](t, w)
	assert.Equal(t, "POURVUE", got.Status)
	require.NotNil(t, got.ExchangeID)
	assert.Equal(t, x.ID, *got.ExchangeID)
	w = e.do(call{method: http.MethodGet, path: path})
	assert.Equal(t, http.StatusNotFound, w.Code, "annonce pourvue invisible des anonymes")
}
