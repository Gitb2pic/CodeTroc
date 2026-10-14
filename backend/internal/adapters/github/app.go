package github

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// App lit dépôts, PR et checks avec les permissions de la GitHub App
// CodeTroc (pull_requests:read, checks:read, metadata:read).
type App struct {
	AppID   int64
	APIBase string
	HTTP    *http.Client

	key   *rsa.PrivateKey
	mu    sync.Mutex
	cache map[string]installToken // dépôt → jeton d'installation
}

type installToken struct {
	value   string
	expires time.Time
}

var _ app.GitHubAPI = (*App)(nil)

// NewApp charge la clé privée PEM de la GitHub App.
func NewApp(appID int64, privateKeyPEM []byte) (*App, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, errors.New("github : clé privée de la GitHub App illisible")
	}
	return &App{AppID: appID, key: key, cache: map[string]installToken{}}, nil
}

func (a *App) api() string { return orDefault(a.APIBase, defaultAPIBase) }
func (a *App) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return DefaultHTTPClient()
}

// appJWT signe un jeton d'application de courte durée (10 min maximum chez GitHub).
func (a *App) appJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    fmt.Sprint(a.AppID),
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(a.key)
}

func repoPath(owner, name string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

// token renvoie un jeton d'installation valide pour le dépôt (mis en cache).
func (a *App) token(ctx context.Context, owner, name string) (string, error) {
	cacheKey := strings.ToLower(owner + "/" + name)
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
	if status, err := getJSON(ctx, a.client(), a.api()+repoPath(owner, name)+"/installation", "Bearer "+jwtTok, &inst); err != nil {
		if status == http.StatusNotFound {
			return "", app.ErrGitHubNotFound
		}
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", a.api(), inst.ID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwtTok)
	var tok struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if _, err := do(a.client(), req, &tok); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.cache[cacheKey] = installToken{value: tok.Token, expires: tok.ExpiresAt}
	a.mu.Unlock()
	return tok.Token, nil
}

func (a *App) get(ctx context.Context, owner, name, path string, out any) error {
	tok, err := a.token(ctx, owner, name)
	if err != nil {
		return err
	}
	status, err := getJSON(ctx, a.client(), a.api()+repoPath(owner, name)+path, "token "+tok, out)
	if status == http.StatusNotFound {
		return app.ErrGitHubNotFound
	}
	return err
}

// Repository lit l'identifiant du dépôt.
func (a *App) Repository(ctx context.Context, owner, name string) (app.Repository, error) {
	var r struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	}
	if err := a.get(ctx, owner, name, "", &r); err != nil {
		return app.Repository{}, err
	}
	return app.Repository{ID: r.ID, FullName: r.FullName}, nil
}

// PullRequest lit les faits d'une PR.
func (a *App) PullRequest(ctx context.Context, owner, name string, number int) (app.PullRequest, error) {
	var p struct {
		ID        int64     `json:"id"`
		Number    int       `json:"number"`
		Merged    bool      `json:"merged"`
		CreatedAt time.Time `json:"created_at"`
		MergeSHA  string    `json:"merge_commit_sha"`
		User      struct {
			ID int64 `json:"id"`
		} `json:"user"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Repo struct {
				ID int64 `json:"id"`
			} `json:"repo"`
		} `json:"base"`
	}
	if err := a.get(ctx, owner, name, fmt.Sprintf("/pulls/%d", number), &p); err != nil {
		return app.PullRequest{}, err
	}
	return app.PullRequest{ID: p.ID, Number: p.Number, BaseRepoID: p.Base.Repo.ID, AuthorID: p.User.ID, CreatedAt: p.CreatedAt,
		Merged: p.Merged, HeadSHA: p.Head.SHA, MergeSHA: p.MergeSHA}, nil
}

// CheckRuns lit les checks d'un commit (100 premiers).
func (a *App) CheckRuns(ctx context.Context, owner, name, sha string) ([]app.CheckRun, error) {
	var r struct {
		CheckRuns []struct {
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := a.get(ctx, owner, name, "/commits/"+url.PathEscape(sha)+"/check-runs?per_page=100", &r); err != nil {
		return nil, err
	}
	out := make([]app.CheckRun, len(r.CheckRuns))
	for i, c := range r.CheckRuns {
		out[i] = app.CheckRun{Name: c.Name, Status: c.Status}
		if c.Conclusion != nil {
			out[i].Conclusion = *c.Conclusion
		}
	}
	return out, nil
}

// Sign calcule la signature X-Hub-Signature-256 d'un corps.
func Sign(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// VerifySignature vérifie X-Hub-Signature-256 en temps constant. Un secret
// vide refuse tout.
func VerifySignature(secret, body []byte, header string) bool {
	if len(secret) == 0 {
		return false
	}
	return hmac.Equal([]byte(header), []byte(Sign(secret, body)))
}
