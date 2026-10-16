package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

type issuesServer struct {
	mu        sync.Mutex
	tokenReq  map[string]any
	labels    map[string]bool
	posted    []map[string]any
	patched   map[string]map[string]any
	permDeny  bool
	rateLimit bool
}

func newIssuesServer(t *testing.T) (*httptest.Server, *issuesServer) {
	t.Helper()
	st := &issuesServer{labels: map[string]bool{"codetroc": true}, patched: map[string]map[string]any{}}
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "token ghs_issues" {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		return true
	}
	decode := func(r *http.Request) map[string]any {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		return m
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/acme/shop-api/installation", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"id":77}`)) })
	mux.HandleFunc("POST /app/installations/77/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.tokenReq = decode(r)
		if st.permDeny {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		_, _ = w.Write([]byte(`{"token":"ghs_issues","expires_at":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `"}`))
	})
	mux.HandleFunc("POST /repositories/100/labels", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		st.mu.Lock()
		defer st.mu.Unlock()
		name := decode(r)["name"].(string)
		if st.labels[name] {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":[{"code":"already_exists"}]}`))
			return
		}
		st.labels[name] = true
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("GET /repositories/100/milestones", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			_, _ = w.Write([]byte(`[{"number":3,"title":"Jalon 1 — Maquette"}]`))
		}
	})
	mux.HandleFunc("PATCH /repositories/100/milestones/3", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			st.mu.Lock()
			st.patched["milestone"] = decode(r)
			st.mu.Unlock()
		}
	})
	mux.HandleFunc("POST /repositories/100/milestones", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			_, _ = w.Write([]byte(`{"number":4}`))
		}
	})
	mux.HandleFunc("POST /repositories/100/issues", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		st.mu.Lock()
		defer st.mu.Unlock()
		if st.rateLimit {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit"}`))
			return
		}
		st.posted = append(st.posted, decode(r))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":12}`))
	})
	mux.HandleFunc("PATCH /repositories/100/issues/12", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			st.mu.Lock()
			st.patched["issue"] = decode(r)
			st.mu.Unlock()
		}
	})
	mux.HandleFunc("PATCH /repositories/100/issues/13", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusGone) })
	mux.HandleFunc("POST /repositories/100/issues/12/comments", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			w.WriteHeader(http.StatusCreated)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, st
}

func TestIssues_WriteThroughScopedToken(t *testing.T) {
	_, pemKey := testKey(t)
	srv, st := newIssuesServer(t)
	a, err := NewApp(123, pemKey)
	require.NoError(t, err)
	a.APIBase = srv.URL
	ctx := context.Background()
	repo := app.RepoRef{ID: 100, Owner: "acme", Name: "shop-api"}

	require.NoError(t, a.EnsureLabels(ctx, repo, []scrum.Label{{Name: "codetroc", Color: "5a4bfe"}, {Name: "sprint:1", Color: "c5def5"}}))
	assert.True(t, st.labels["sprint:1"])
	// Jeton limité au dépôt et à issues:write.
	assert.Equal(t, []any{float64(100)}, st.tokenReq["repository_ids"])
	assert.Equal(t, map[string]any{"issues": "write", "metadata": "read"}, st.tokenReq["permissions"])

	due := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	n, err := a.EnsureMilestone(ctx, repo, "Jalon 1 — Maquette", due)
	require.NoError(t, err)
	assert.Equal(t, 3, n, "milestone existant réutilisé")
	assert.Equal(t, "2026-11-03T09:00:00Z", st.patched["milestone"]["due_on"])
	n, err = a.EnsureMilestone(ctx, repo, "Jalon 2 — API", due)
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	n, err = a.CreateIssue(ctx, repo, app.IssueInput{Title: "Sprint 1", Body: "b", Labels: []string{"codetroc"}, Milestone: 3, Assignee: "lea"})
	require.NoError(t, err)
	assert.Equal(t, 12, n)
	assert.Equal(t, []any{"lea"}, st.posted[0]["assignees"])
	assert.Equal(t, float64(3), st.posted[0]["milestone"])

	require.NoError(t, a.UpdateIssue(ctx, repo, 12, app.IssueUpdate{State: "closed"}))
	assert.Equal(t, map[string]any{"state": "closed"}, st.patched["issue"], "champs vides non envoyés")
	require.NoError(t, a.Comment(ctx, repo, 12, "Bilan"))
	assert.ErrorIs(t, a.UpdateIssue(ctx, repo, 13, app.IssueUpdate{State: "closed"}), app.ErrGitHubNotFound, "issue supprimée")

	st.mu.Lock()
	st.rateLimit = true
	st.mu.Unlock()
	_, err = a.CreateIssue(ctx, repo, app.IssueInput{Title: "x"})
	var rl *app.RateLimitError
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, 2*time.Minute, rl.RetryAfter)
}

func TestIssues_PermissionNotGranted(t *testing.T) {
	_, pemKey := testKey(t)
	srv, st := newIssuesServer(t)
	st.permDeny = true
	a, err := NewApp(123, pemKey)
	require.NoError(t, err)
	a.APIBase = srv.URL
	err = a.EnsureLabels(context.Background(), app.RepoRef{ID: 100, Owner: "acme", Name: "shop-api"}, []scrum.Label{{Name: "x", Color: "000000"}})
	assert.ErrorIs(t, err, app.ErrGitHubPermission)
}
