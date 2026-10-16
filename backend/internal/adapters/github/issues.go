package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/domain/scrum"
)

// Écriture GitHub du module Scrum (F16, D18) : labels, milestones et issues.
// Chaque jeton d'installation est demandé pour UN dépôt (repository_ids) et
// la seule permission issues:write ; les appels désignent le dépôt par son
// identifiant (/repositories/{id}), jamais par son nom (R-S2).

var _ app.GitHubIssues = (*App)(nil)

// issuesToken renvoie un jeton limité au dépôt et à issues:write (mis en cache).
func (a *App) issuesToken(ctx context.Context, repo app.RepoRef) (string, error) {
	cacheKey := "issues:" + strconv.FormatInt(repo.ID, 10)
	a.mu.Lock()
	if t, ok := a.cache[cacheKey]; ok && time.Until(t.expires) > time.Minute {
		a.mu.Unlock()
		return t.value, nil
	}
	a.mu.Unlock()

	jwtTok, err := a.appJWT()
	if err != nil {
		return "", err
	}
	var inst struct {
		ID int64 `json:"id"`
	}
	status, _, err := a.send(ctx, http.MethodGet, a.api()+repoPath(repo.Owner, repo.Name)+"/installation", "Bearer "+jwtTok, nil, &inst)
	if err != nil {
		return "", classify(status, nil, err)
	}
	body := map[string]any{"repository_ids": []int64{repo.ID}, "permissions": map[string]string{"issues": "write", "metadata": "read"}}
	var tok struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	status, hdr, err := a.send(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", a.api(), inst.ID), "Bearer "+jwtTok, body, &tok)
	if err != nil {
		// 422 : permission non accordée à l'installation, ou dépôt hors installation.
		if status == http.StatusUnprocessableEntity || status == http.StatusForbidden {
			return "", app.ErrGitHubPermission
		}
		return "", classify(status, hdr, err)
	}
	a.mu.Lock()
	a.cache[cacheKey] = installToken{value: tok.Token, expires: tok.ExpiresAt}
	a.mu.Unlock()
	return tok.Token, nil
}

// send exécute une requête JSON et renvoie statut et en-têtes.
func (a *App) send(ctx context.Context, method, u, auth string, in, out any) (int, http.Header, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", auth)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("github : %s %s : %w", method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, resp.Header, err
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, resp.Header, &apiError{status: resp.StatusCode, body: string(raw), path: req.URL.Path}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, resp.Header, fmt.Errorf("github : réponse illisible : %w", err)
		}
	}
	return resp.StatusCode, resp.Header, nil
}

type apiError struct {
	status int
	body   string
	path   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("github : %s : statut %d", e.path, e.status)
}

// classify traduit une réponse d'erreur en erreur du port : quota (429, ou
// 403 avec quota épuisé), permission (403), ressource absente (404, 410).
func classify(status int, hdr http.Header, err error) error {
	if err == nil {
		return nil
	}
	body := ""
	var ae *apiError
	if errors.As(err, &ae) {
		body = strings.ToLower(ae.body)
	}
	switch {
	case status == http.StatusTooManyRequests,
		status == http.StatusForbidden && (hdr.Get("X-RateLimit-Remaining") == "0" || strings.Contains(body, "rate limit")):
		return &app.RateLimitError{RetryAfter: retryAfter(hdr)}
	case status == http.StatusForbidden:
		return app.ErrGitHubPermission
	case status == http.StatusNotFound || status == http.StatusGone:
		return app.ErrGitHubNotFound
	}
	return err
}

// retryAfter lit Retry-After (secondes) ou X-RateLimit-Reset (epoch) ; 1 min par défaut.
func retryAfter(hdr http.Header) time.Duration {
	if hdr == nil {
		return time.Minute
	}
	if s, err := strconv.Atoi(hdr.Get("Retry-After")); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	if r, err := strconv.ParseInt(hdr.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		if d := time.Until(time.Unix(r, 0)); d > 0 {
			return d
		}
	}
	return time.Minute
}

// call exécute un appel authentifié sur /repositories/{id}{path}.
func (a *App) call(ctx context.Context, repo app.RepoRef, method, path string, in, out any) (int, error) {
	tok, err := a.issuesToken(ctx, repo)
	if err != nil {
		return 0, err
	}
	status, hdr, err := a.send(ctx, method, fmt.Sprintf("%s/repositories/%d%s", a.api(), repo.ID, path), "token "+tok, in, out)
	return status, classify(status, hdr, err)
}

// EnsureLabels crée les labels manquants ; un label existant (422) est conservé.
func (a *App) EnsureLabels(ctx context.Context, repo app.RepoRef, labels []scrum.Label) error {
	for _, l := range labels {
		status, err := a.call(ctx, repo, http.MethodPost, "/labels", l, nil)
		if err != nil && status != http.StatusUnprocessableEntity {
			return err
		}
	}
	return nil
}

// EnsureMilestone crée le milestone, ou met à jour l'échéance de celui qui porte ce titre.
func (a *App) EnsureMilestone(ctx context.Context, repo app.RepoRef, title string, due time.Time) (int, error) {
	var list []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if _, err := a.call(ctx, repo, http.MethodGet, "/milestones?state=all&per_page=100", nil, &list); err != nil {
		return 0, err
	}
	body := map[string]any{"title": title, "due_on": due.UTC().Format(time.RFC3339)}
	for _, m := range list {
		if m.Title == title {
			_, err := a.call(ctx, repo, http.MethodPatch, "/milestones/"+strconv.Itoa(m.Number), body, nil)
			return m.Number, err
		}
	}
	var created struct {
		Number int `json:"number"`
	}
	_, err := a.call(ctx, repo, http.MethodPost, "/milestones", body, &created)
	return created.Number, err
}

// CreateIssue crée une issue et renvoie son numéro.
func (a *App) CreateIssue(ctx context.Context, repo app.RepoRef, in app.IssueInput) (int, error) {
	body := map[string]any{"title": in.Title, "body": in.Body, "labels": in.Labels}
	if in.Milestone > 0 {
		body["milestone"] = in.Milestone
	}
	if in.Assignee != "" {
		body["assignees"] = []string{in.Assignee}
	}
	var created struct {
		Number int `json:"number"`
	}
	_, err := a.call(ctx, repo, http.MethodPost, "/issues", body, &created)
	return created.Number, err
}

// UpdateIssue modifie une issue ; les champs vides ne sont pas envoyés.
func (a *App) UpdateIssue(ctx context.Context, repo app.RepoRef, number int, in app.IssueUpdate) error {
	body := map[string]any{}
	if in.Title != "" {
		body["title"] = in.Title
	}
	if in.Body != "" {
		body["body"] = in.Body
	}
	if in.State != "" {
		body["state"] = in.State
	}
	if in.Labels != nil {
		body["labels"] = in.Labels
	}
	_, err := a.call(ctx, repo, http.MethodPatch, "/issues/"+strconv.Itoa(number), body, nil)
	return err
}

// Comment ajoute un commentaire à une issue.
func (a *App) Comment(ctx context.Context, repo app.RepoRef, number int, body string) error {
	_, err := a.call(ctx, repo, http.MethodPost, "/issues/"+strconv.Itoa(number)+"/comments", map[string]string{"body": body}, nil)
	return err
}
