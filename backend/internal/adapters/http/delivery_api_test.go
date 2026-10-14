package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ghadapter "github.com/gitb2pic/codetroc/backend/internal/adapters/github"
	"github.com/gitb2pic/codetroc/backend/internal/app"
)

type stubGitHub struct {
	mu     sync.Mutex
	pulls  map[int]app.PullRequest
	checks map[string][]app.CheckRun
}

func (s *stubGitHub) Repository(_ context.Context, owner, name string) (app.Repository, error) {
	if owner != "acme" || name != "shop-api" {
		return app.Repository{}, app.ErrGitHubNotFound
	}
	return app.Repository{ID: 100, FullName: "acme/shop-api"}, nil
}

func (s *stubGitHub) PullRequest(_ context.Context, _, _ string, n int) (app.PullRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pulls[n]
	if !ok {
		return app.PullRequest{}, app.ErrGitHubNotFound
	}
	return p, nil
}

func (s *stubGitHub) CheckRuns(_ context.Context, _, _, sha string) ([]app.CheckRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checks[sha], nil
}

const webhookSecret = "whsec-test"

// deliveryEnv : échange accepté entre Sami (demandeur) et Léa (prestataire).
type deliveryEnv struct {
	*apiEnv
	gh             *stubGitHub
	lea, sami, bob member
	x              exchangeJSON
}

func newDeliveryEnv(t *testing.T) deliveryEnv {
	t.Helper()
	return newDeliveryEnvWith(t)
}

func newDeliveryEnvWith(t *testing.T, mutate ...func(*app.Deps, *Options)) deliveryEnv {
	t.Helper()
	gh := &stubGitHub{pulls: map[int]app.PullRequest{}, checks: map[string][]app.CheckRun{}}
	e := newAPI(t, append([]func(*app.Deps, *Options){func(d *app.Deps, o *Options) {
		d.GitHub = gh
		o.Webhooks = &WebhookConfig{Secret: []byte(webhookSecret)}
	}}, mutate...)...)
	lea, sami, bob := e.login("lea"), e.login("sami"), e.login("bob")
	e.fund(sami, 310)
	offer := e.publishOffer(lea, 120)
	x := e.requestExchange(sami, offer.ID.String())
	w := e.do(call{method: http.MethodPost, path: "/api/v1/echanges/" + x.ID.String() + "/accepter", token: lea.Token, headers: idemKey()})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	x = decode[exchangeJSON](t, w)

	var leaGitHubID int64
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT github_id FROM utilisateur WHERE id_utilisateur = $1`, lea.ID).Scan(&leaGitHubID))
	gh.pulls[42] = app.PullRequest{ID: 9001, Number: 42, BaseRepoID: 100, AuthorID: leaGitHubID, CreatedAt: e.now.Add(time.Hour), Merged: false, HeadSHA: "a3f9c1e"}
	gh.checks["a3f9c1e"] = []app.CheckRun{{Name: "lint", Status: "completed", Conclusion: "success"}}
	return deliveryEnv{apiEnv: e, gh: gh, lea: lea, sami: sami, bob: bob, x: x}
}

func (d deliveryEnv) webhook(delivery, event, body string, sign bool) int {
	d.t.Helper()
	h := map[string]string{"X-GitHub-Delivery": delivery, "X-GitHub-Event": event}
	if sign {
		h["X-Hub-Signature-256"] = ghadapter.Sign([]byte(webhookSecret), []byte(body))
	}
	return d.do(call{method: http.MethodPost, path: "/api/v1/webhooks/github", body: body, headers: h}).Code
}

func TestDeliveryAPI_FullFlowWithWebhook(t *testing.T) {
	d := newDeliveryEnv(t)
	ms := "/api/v1/jalons/" + d.x.Milestones[0].ID.String()

	w := d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.lea.Token, body: map[string]any{"pr_url": "https://github.com/acme/shop-api/pull/42"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	x := decode[exchangeJSON](t, w)
	assert.Equal(t, "EN_ATTENTE", x.Milestones[0].Proofs[0].Status, "PR pas encore mergée")

	// La PR est mergée : GitHub prévient CodeTroc.
	d.gh.mu.Lock()
	pr := d.gh.pulls[42]
	pr.Merged = true
	d.gh.pulls[42] = pr
	d.gh.mu.Unlock()
	body := `{"action":"closed","pull_request":{"number":42,"merged":true},"repository":{"id":100,"full_name":"acme/shop-api"}}`
	assert.Equal(t, http.StatusAccepted, d.webhook("d-1", "pull_request", body, true))

	w = d.do(call{method: http.MethodGet, path: "/api/v1/echanges/" + d.x.ID.String(), token: d.sami.Token})
	x = decode[exchangeJSON](t, w)
	assert.Equal(t, "LIVRE", string(x.Milestones[0].Status))
	assert.Equal(t, "DEMANDEUR", string(x.NextActor))

	w = d.do(call{method: http.MethodPost, path: ms + "/valider", token: d.lea.Token, headers: idemKey()})
	assert.Equal(t, http.StatusForbidden, w.Code, "le prestataire ne valide pas lui-même")
	w = d.do(call{method: http.MethodPost, path: ms + "/valider", token: d.sami.Token, headers: idemKey()})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	x = decode[exchangeJSON](t, w)
	assert.Equal(t, "LIBERE", string(x.Milestones[0].Status))
	assert.Equal(t, int64(60), x.Escrow.Released, "offre en deux jalons de 60 cr")

	w = d.do(call{method: http.MethodGet, path: "/api/v1/me/compte", token: d.lea.Token})
	assert.JSONEq(t, `{"disponible":110,"sequestre_sortant":0,"sequestre_entrant":60}`, w.Body.String())
}

func TestDeliveryAPI_WebhookSecurity(t *testing.T) {
	d := newDeliveryEnv(t)
	body := `{"action":"closed","pull_request":{"number":42},"repository":{"id":100,"full_name":"acme/shop-api"}}`

	assert.Equal(t, http.StatusUnauthorized, d.webhook("d-1", "pull_request", body, false), "sans signature")
	w := d.do(call{method: http.MethodPost, path: "/api/v1/webhooks/github", body: body, headers: map[string]string{
		"X-GitHub-Delivery": "d-2", "X-GitHub-Event": "pull_request",
		"X-Hub-Signature-256": ghadapter.Sign([]byte("autre-secret"), []byte(body))}})
	assert.Equal(t, http.StatusUnauthorized, w.Code, "signée avec un autre secret")
	assert.Equal(t, http.StatusBadRequest, d.webhook("../../etc", "pull_request", body, true), "identifiant de livraison hostile")

	assert.Equal(t, http.StatusAccepted, d.webhook("d-3", "pull_request", body, true))
	w = d.do(call{method: http.MethodPost, path: "/api/v1/webhooks/github", body: body, headers: map[string]string{
		"X-GitHub-Delivery": "d-3", "X-GitHub-Event": "pull_request", "X-Hub-Signature-256": ghadapter.Sign([]byte(webhookSecret), []byte(body))}})
	assert.JSONEq(t, `{"doublon":true,"preuves_verifiees":0}`, w.Body.String(), "rejeu ignoré")
}

func TestDeliveryAPI_AccessControl(t *testing.T) {
	d := newDeliveryEnv(t)
	ms := "/api/v1/jalons/" + d.x.Milestones[0].ID.String()
	pr := map[string]any{"pr_url": "https://github.com/acme/shop-api/pull/42"}

	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.bob.Token, body: pr}).Code)
	assert.Equal(t, http.StatusForbidden, d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.sami.Token, body: pr}).Code)
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodPost, path: ms + "/reverifier", token: d.bob.Token}).Code)
	assert.Equal(t, http.StatusNotFound, d.do(call{method: http.MethodPost, path: ms + "/valider", token: d.bob.Token, headers: idemKey()}).Code)

	w := d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.lea.Token, body: map[string]any{"pr_url": "https://github.com/evil/repo/pull/1"}})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	w = d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.lea.Token, body: map[string]any{"pr_url": "https://github.com/acme/shop-api/pull/42", "statut_preuve": "VALIDE"}})
	assert.Equal(t, http.StatusBadRequest, w.Code, "un client ne peut pas déclarer sa preuve valide")
}

func TestDeliveryAPI_ConcurrentValidationReleasesOnce(t *testing.T) {
	d := newDeliveryEnv(t)
	d.gh.mu.Lock()
	pr := d.gh.pulls[42]
	pr.Merged = true
	d.gh.pulls[42] = pr
	d.gh.mu.Unlock()
	ms := "/api/v1/jalons/" + d.x.Milestones[0].ID.String()
	require.Equal(t, http.StatusCreated, d.do(call{method: http.MethodPost, path: ms + "/preuves", token: d.lea.Token,
		body: map[string]any{"pr_url": "https://github.com/acme/shop-api/pull/42"}}).Code)

	var wg sync.WaitGroup
	codes := make([]int, 6)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = d.do(call{method: http.MethodPost, path: ms + "/valider", token: d.sami.Token, headers: idemKey()}).Code
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		}
	}
	assert.Equal(t, 1, ok, fmt.Sprint(codes))
	w := d.do(call{method: http.MethodGet, path: "/api/v1/me/compte", token: d.lea.Token})
	assert.JSONEq(t, `{"disponible":110,"sequestre_sortant":0,"sequestre_entrant":60}`, w.Body.String())
}
