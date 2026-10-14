package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/adapters/vault"
	"github.com/gitb2pic/codetroc/backend/internal/app"
)

const witness = "temoin-du-coffre-a-ne-jamais-journaliser"

// withTestVault branche un coffre réel avec une clé aléatoire.
func withTestVault(t *testing.T) func(*app.Deps, *Options) {
	kek := make([]byte, 32)
	_, err := rand.Read(kek)
	require.NoError(t, err)
	v, err := vault.New(map[int][]byte{1: kek}, 1)
	require.NoError(t, err)
	return func(d *app.Deps, _ *Options) { d.Vault = v }
}

func TestVaultAPI(t *testing.T) {
	var logs bytes.Buffer
	kek := make([]byte, 32)
	_, err := rand.Read(kek)
	require.NoError(t, err)
	v, err := vault.New(map[int][]byte{1: kek}, 1)
	require.NoError(t, err)
	d := newDeliveryEnvWith(t, func(deps *app.Deps, o *Options) {
		deps.Vault = v
		o.Logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	base := "/api/v1/echanges/" + d.x.ID.String() + "/secrets"
	in := map[string]any{"libelle": "Token CI", "type": "JETON", "portee": "repo acme/shop-api", "valeur": witness,
		"date_expiration": d.now.Add(72 * time.Hour).Format(time.RFC3339)}

	w := d.do(call{method: http.MethodPost, path: base, token: d.lea.Token, body: in})
	assert.Equal(t, http.StatusForbidden, w.Code, "le prestataire ne dépose pas")
	w = d.do(call{method: http.MethodPost, path: base, token: d.sami.Token, body: in})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	sec := decode[secretJSON](t, w)
	assert.NotContains(t, w.Body.String(), witness)

	w = d.do(call{method: http.MethodGet, path: base, token: d.lea.Token})
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), witness, "la liste ne contient que des métadonnées")
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodGet, path: base, token: d.bob.Token}).Code)

	path := "/api/v1/secrets/" + sec.ID.String()
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodPost, path: path + "/reveler", token: d.bob.Token}).Code)
	assert.Equal(t, http.StatusForbidden, d.do(call{method: http.MethodPost, path: path + "/reveler", token: d.sami.Token}).Code)
	w = d.do(call{method: http.MethodPost, path: path + "/reveler", token: d.lea.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"valeur":"`+witness+`"}`, w.Body.String())
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	w = d.do(call{method: http.MethodPost, path: path + "/revoquer", token: d.sami.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "REVOQUE", decode[secretJSON](t, w).Status)
	w = d.do(call{method: http.MethodPost, path: path + "/reveler", token: d.lea.Token})
	assert.Equal(t, http.StatusGone, w.Code)
	assert.Equal(t, "/problems/secret-indisponible", problemType(t, w))

	w = d.do(call{method: http.MethodGet, path: path + "/journal", token: d.sami.Token})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"REVELATION"`)

	// La valeur ne doit apparaître nulle part ailleurs que dans la réponse de révélation.
	assert.NotContains(t, logs.String(), witness, "jamais dans les journaux")
	var leaked bool
	require.NoError(t, d.db.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM secret WHERE position(convert_to($1, 'UTF8') IN coalesce(valeur_chiffree, ''::bytea)) > 0)
		    OR EXISTS (SELECT 1 FROM cle_idempotence WHERE position(convert_to($1, 'UTF8') IN coalesce(corps, ''::bytea)) > 0)`, witness).Scan(&leaked))
	assert.False(t, leaked, "jamais en clair en base")
}

func TestVaultAPI_NotConfigured(t *testing.T) {
	d := newDeliveryEnv(t)
	w := d.do(call{method: http.MethodPost, path: "/api/v1/echanges/" + d.x.ID.String() + "/secrets", token: d.sami.Token,
		body: map[string]any{"libelle": "x", "type": "AUTRE", "valeur": "v", "date_expiration": d.now.Add(time.Hour).Format(time.RFC3339)}})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
