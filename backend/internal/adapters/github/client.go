// Package github implémente les ports GitHub : connexion OAuth, lecture des
// dépôts, PR et checks via la GitHub App, signature des webhooks.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultWebBase = "https://github.com"
	defaultAPIBase = "https://api.github.com"
	maxBody        = 1 << 20
)

// DefaultHTTPClient borne la durée des appels à GitHub.
func DefaultHTTPClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

// getJSON exécute une requête authentifiée et décode la réponse JSON.
func getJSON(ctx context.Context, c *http.Client, url, auth string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", auth)
	return do(c, req, out)
}

func do(c *http.Client, req *http.Request, out any) (int, error) {
	resp, err := c.Do(req)
	if err != nil {
		return 0, fmt.Errorf("github : %s %s : %w", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("github : %s %s : statut %d", req.Method, req.URL.Path, resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return resp.StatusCode, fmt.Errorf("github : réponse illisible : %w", err)
		}
	}
	return resp.StatusCode, nil
}
