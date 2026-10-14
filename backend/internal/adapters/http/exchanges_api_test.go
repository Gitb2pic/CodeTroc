package httpapi

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fund crédite un membre depuis ÉMISSION, en SQL (préparation de test).
func (e *apiEnv) fund(m member, amount int64) {
	e.t.Helper()
	ctx := context.Background()
	_, err := e.db.Exec(ctx, `
		WITH em AS (UPDATE compte SET solde = solde - $2 WHERE type_compte = 'EMISSION' RETURNING id_compte),
		     u  AS (UPDATE compte SET solde = solde + $2 WHERE id_utilisateur = $1 RETURNING id_compte)
		INSERT INTO mouvement (type_mouvement, montant, cle_idempotence, id_compte_debit, id_compte_credit)
		SELECT 'BONUS', $2, 'test:' || gen_random_uuid(), em.id_compte, u.id_compte FROM em, u`, m.ID, amount)
	require.NoError(e.t, err)
}

func (e *apiEnv) requestExchange(requester member, offerID string) exchangeJSON {
	e.t.Helper()
	w := e.do(call{method: http.MethodPost, path: "/api/v1/echanges", token: requester.Token, body: map[string]any{
		"id_offre": offerID, "depot_url": "https://github.com/acme/shop-api", "besoin": "Pipeline pour notre API"}})
	require.Equal(e.t, http.StatusCreated, w.Code, w.Body.String())
	return decode[exchangeJSON](e.t, w)
}

func idemKey() map[string]string { return map[string]string{idempotencyHeader: "k-" + uuid.NewString()} }

func TestExchangesAPI_Lifecycle(t *testing.T) {
	e := newAPI(t)
	lea, sami, tiers := e.login("lea"), e.login("sami"), e.login("tiers")
	e.fund(sami, 310)
	offer := e.publishOffer(lea, 120)
	x := e.requestExchange(sami, offer.ID.String())
	assert.Equal(t, "PRESTATAIRE", string(x.NextActor))
	path := "/api/v1/echanges/" + x.ID.String()

	w := e.do(call{method: http.MethodGet, path: path, token: tiers.Token})
	assert.Equal(t, http.StatusNotFound, w.Code, "un tiers ne voit pas l'échange (D11)")

	w = e.do(call{method: http.MethodPost, path: path + "/accepter", token: lea.Token})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "/problems/cle-idempotence-requise", problemType(t, w))

	w = e.do(call{method: http.MethodPost, path: path + "/accepter", token: sami.Token, headers: idemKey()})
	assert.Equal(t, http.StatusForbidden, w.Code, "le demandeur ne peut pas accepter")

	key := idemKey()
	w = e.do(call{method: http.MethodPost, path: path + "/accepter", token: lea.Token, headers: key})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	accepted := decode[exchangeJSON](t, w)
	assert.Equal(t, "EN_COURS", string(accepted.Status))
	assert.Equal(t, int64(120), accepted.Escrow.Locked)

	// Rejeu de la même requête (double clic, réseau) : même réponse, aucun second blocage.
	w2 := e.do(call{method: http.MethodPost, path: path + "/accepter", token: lea.Token, headers: key})
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "true", w2.Header().Get("Idempotent-Replayed"))
	assert.JSONEq(t, w.Body.String(), w2.Body.String())

	// Même clé pour une autre action : refusée.
	w = e.do(call{method: http.MethodPost, path: path + "/annuler", token: lea.Token, headers: key})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "/problems/cle-idempotence-reutilisee", problemType(t, w))

	// Une clé d'un autre utilisateur n'est pas partagée.
	w = e.do(call{method: http.MethodPost, path: path + "/accepter", token: sami.Token, headers: key})
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = e.do(call{method: http.MethodGet, path: "/api/v1/me/compte", token: sami.Token})
	assert.JSONEq(t, `{"disponible":240,"sequestre_sortant":120,"sequestre_entrant":0}`, w.Body.String())

	w = e.do(call{method: http.MethodGet, path: "/api/v1/echanges?role=prestataire", token: lea.Token})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), x.ID.String())
	w = e.do(call{method: http.MethodGet, path: "/api/v1/echanges?role=admin", token: lea.Token})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExchangesAPI_RejectsHostileInput(t *testing.T) {
	e := newAPI(t)
	lea, sami := e.login("lea"), e.login("sami")
	e.fund(sami, 310)
	offer := e.publishOffer(lea, 120)

	for _, repo := range []string{"http://localhost:8080/admin", "https://github.com/acme/api?ref=main", "file:///etc/passwd"} {
		w := e.do(call{method: http.MethodPost, path: "/api/v1/echanges", token: sami.Token, body: map[string]any{"id_offre": offer.ID, "depot_url": repo}})
		assert.Equal(t, http.StatusBadRequest, w.Code, repo)
		assert.Equal(t, "/problems/depot-invalide", problemType(t, w))
	}
	// Le client ne fixe ni le montant ni le statut.
	w := e.do(call{method: http.MethodPost, path: "/api/v1/echanges", token: sami.Token, body: map[string]any{
		"id_offre": offer.ID, "depot_url": "https://github.com/acme/api", "montant_total": 1}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExchangesAPI_ConcurrentAcceptWithDifferentKeys(t *testing.T) {
	e := newAPI(t)
	lea, sami := e.login("lea"), e.login("sami")
	e.fund(sami, 310)
	offer := e.publishOffer(lea, 120)
	x := e.requestExchange(sami, offer.ID.String())

	codes := make([]int, 8)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = e.do(call{method: http.MethodPost, path: "/api/v1/echanges/" + x.ID.String() + "/accepter", token: lea.Token, headers: idemKey()}).Code
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		} else {
			assert.Equal(t, http.StatusConflict, c)
		}
	}
	assert.Equal(t, 1, ok)
	w := e.do(call{method: http.MethodGet, path: "/api/v1/me/compte", token: sami.Token})
	assert.JSONEq(t, `{"disponible":240,"sequestre_sortant":120,"sequestre_entrant":0}`, w.Body.String())
}
