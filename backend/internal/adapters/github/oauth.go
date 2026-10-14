package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// OAuth réalise la connexion « Se connecter avec GitHub » (OAuth App ou
// identifiants OAuth de la GitHub App).
type OAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	WebBase      string // https://github.com par défaut
	APIBase      string // https://api.github.com par défaut
	HTTP         *http.Client
}

var _ app.GitHubOAuth = (*OAuth)(nil)

func (o *OAuth) web() string { return orDefault(o.WebBase, defaultWebBase) }
func (o *OAuth) api() string { return orDefault(o.APIBase, defaultAPIBase) }
func (o *OAuth) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return DefaultHTTPClient()
}

// AuthorizeURL renvoie l'URL d'autorisation GitHub (lecture du profil et de l'e-mail).
func (o *OAuth) AuthorizeURL(state string) string {
	q := url.Values{
		"client_id":    {o.ClientID},
		"redirect_uri": {o.RedirectURL},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}
	return o.web() + "/login/oauth/authorize?" + q.Encode()
}

// Exchange échange le code contre un jeton puis lit l'identité GitHub.
// Le jeton GitHub de l'utilisateur n'est jamais conservé.
func (o *OAuth) Exchange(ctx context.Context, code string) (app.GitHubIdentity, error) {
	form := url.Values{"client_id": {o.ClientID}, "client_secret": {o.ClientSecret}, "code": {code}, "redirect_uri": {o.RedirectURL}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.web()+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return app.GitHubIdentity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if _, err := do(o.client(), req, &tok); err != nil {
		return app.GitHubIdentity{}, err
	}
	if tok.AccessToken == "" {
		return app.GitHubIdentity{}, fmt.Errorf("github : échange du code refusé (%s)", orDefault(tok.Error, "sans motif"))
	}
	auth := "Bearer " + tok.AccessToken

	var u struct {
		ID        int64     `json:"id"`
		Login     string    `json:"login"`
		CreatedAt time.Time `json:"created_at"`
	}
	if _, err := getJSON(ctx, o.client(), o.api()+"/user", auth, &u); err != nil {
		return app.GitHubIdentity{}, err
	}
	if u.ID == 0 || u.Login == "" {
		return app.GitHubIdentity{}, errors.New("github : profil incomplet")
	}
	id := app.GitHubIdentity{ID: u.ID, Login: u.Login, CreatedAt: u.CreatedAt}

	// L'e-mail est facultatif (D12) : un échec ici ne bloque pas la connexion.
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if _, err := getJSON(ctx, o.client(), o.api()+"/user/emails", auth, &emails); err == nil {
		for _, e := range emails {
			if e.Primary && e.Verified {
				id.Email = e.Email
			}
		}
	}
	return id, nil
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
