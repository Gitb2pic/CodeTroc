package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (e *apiEnv) idOf(query string, args ...any) string {
	e.t.Helper()
	var id string
	require.NoError(e.t, e.db.QueryRow(context.Background(), query, args...).Scan(&id))
	return id
}

func (e *apiEnv) pipelinePackID() string {
	return e.idOf(`SELECT id_pack::text FROM pack WHERE titre_pack LIKE 'Pipeline%'`)
}

func (e *apiEnv) publishOffer(m member, price int64) offerJSON {
	e.t.Helper()
	half := price / 2
	w := e.do(call{method: http.MethodPost, path: "/api/v1/offres", token: m.Token, body: map[string]any{
		"id_pack": e.pipelinePackID(), "titre": "Pipeline CI/CD GitHub Actions pour app Node",
		"description": "Sur ton dépôt.", "prix_credits": price, "delai_jours": 5,
		"jalons": []map[string]any{{"titre": "Workflow CI", "montant": half}, {"titre": "Déploiement", "montant": price - half}},
	}})
	require.Equal(e.t, http.StatusCreated, w.Code, w.Body.String())
	return decode[offerJSON](e.t, w)
}

func TestProfileAPI(t *testing.T) {
	e := newAPI(t)
	m := e.login("lea")

	w := e.do(call{method: http.MethodPatch, path: "/api/v1/me", token: m.Token, body: map[string]any{"pseudo": "lea-devops", "bio": "DevOps"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "lea-devops", decode[userJSON](t, w).Pseudo)

	// Affectation de masse : un membre ne peut pas se donner un rôle.
	w = e.do(call{method: http.MethodPatch, path: "/api/v1/me", token: m.Token, body: map[string]any{"role": "ADMIN"}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "/problems/corps-invalide", problemType(t, w))
	w = e.do(call{method: http.MethodGet, path: "/api/v1/me", token: m.Token})
	assert.Equal(t, "MEMBRE", string(decode[userJSON](t, w).Role))

	w = e.do(call{method: http.MethodPatch, path: "/api/v1/me", token: m.Token, body: `{"pseudo":"a"} {"pseudo":"b"}`})
	assert.Equal(t, http.StatusBadRequest, w.Code, "un seul objet JSON")

	docker := e.idOf(`SELECT id_competence::text FROM competence WHERE libelle_competence = 'Docker'`)
	w = e.do(call{method: http.MethodPut, path: "/api/v1/me/competences", token: m.Token, body: map[string]any{
		"competences": []map[string]any{{"id_competence": docker, "niveau": "EXPERT"}}}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = e.do(call{method: http.MethodGet, path: "/api/v1/utilisateurs/" + m.ID})
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "email")
	assert.Contains(t, w.Body.String(), `"niveau":"EXPERT"`)

	w = e.do(call{method: http.MethodGet, path: "/api/v1/utilisateurs/pas-un-uuid"})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = e.do(call{method: http.MethodGet, path: "/api/v1/competences"})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPacksAPI(t *testing.T) {
	e := newAPI(t)
	member := e.login("membre")
	admin := e.login("admin")
	e.setRole(admin, "ADMIN")
	postgres := e.idOf(`SELECT id_competence::text FROM competence WHERE libelle_competence = 'Postgres'`)
	body := map[string]any{
		"titre": "Migration PostgreSQL 16", "description": "Montée de version.", "livrable": "Scripts",
		"type_preuve": "PR_CI", "criteres": []map[string]any{{"texte": "Migrations vertes", "verification": "CI"}},
		"prix_min": 50, "prix_max": 150, "competences": []string{postgres},
	}

	w := e.do(call{method: http.MethodPost, path: "/api/v1/packs", token: member.Token, body: body})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = e.do(call{method: http.MethodPost, path: "/api/v1/packs", body: body})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = e.do(call{method: http.MethodPost, path: "/api/v1/packs", token: admin.Token, body: body})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	pack := decode[packJSON](t, w)

	w = e.do(call{method: http.MethodPatch, path: "/api/v1/packs/" + pack.ID.String(), token: admin.Token, body: map[string]any{"actif": false}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = e.do(call{method: http.MethodGet, path: "/api/v1/packs/" + pack.ID.String()})
	assert.Equal(t, http.StatusNotFound, w.Code, "pack inactif invisible du public")

	w = e.do(call{method: http.MethodGet, path: "/api/v1/packs?limit=3"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), `"next_cursor":null`)
	w = e.do(call{method: http.MethodGet, path: "/api/v1/packs", token: "jeton-invalide"})
	assert.Equal(t, http.StatusUnauthorized, w.Code, "un jeton présent mais invalide est refusé même sur une route publique")
}

func TestOffersAPI(t *testing.T) {
	e := newAPI(t)
	lea := e.login("lea")
	karim := e.login("karim")
	o := e.publishOffer(lea, 120)
	assert.Equal(t, "lea", o.Provider.Pseudo)

	w := e.do(call{method: http.MethodGet, path: "/api/v1/offres?competence=Docker&prix_max=150&q=node"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), o.ID.String())
	w = e.do(call{method: http.MethodGet, path: "/api/v1/offres?prix_max=cher"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Un client ne peut pas fixer lui-même le statut ou le prestataire.
	w = e.do(call{method: http.MethodPost, path: "/api/v1/offres", token: karim.Token, body: map[string]any{
		"id_pack": e.pipelinePackID(), "titre": "Offre usurpée", "prix_credits": 100, "delai_jours": 3,
		"jalons": []map[string]any{{"titre": "Tout", "montant": 100}}, "id_prestataire": lea.ID}})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = e.do(call{method: http.MethodPost, path: "/api/v1/offres/" + o.ID.String() + "/retirer", token: karim.Token})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = e.do(call{method: http.MethodPost, path: "/api/v1/offres/" + o.ID.String() + "/retirer", token: lea.Token})
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = e.do(call{method: http.MethodGet, path: "/api/v1/offres/" + o.ID.String()})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = e.do(call{method: http.MethodGet, path: "/api/v1/offres/" + o.ID.String(), token: lea.Token})
	assert.Equal(t, http.StatusOK, w.Code)
}
