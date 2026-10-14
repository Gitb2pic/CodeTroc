// Package devfake fournit un faux GitHub (OAuth et API) pour le serveur e2e et
// le bac à sable navigateur. Il n'est jamais assemblé par cmd/api ni cmd/worker.
package devfake

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// GitHubID donne un identifiant GitHub stable pour un login.
func GitHubID(login string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(login))
	return 100_000 + int64(h.Sum32()%1_000_000)
}

func repoID(owner, name string) int64 { return GitHubID("repo:" + owner + "/" + name) }

// OAuth accepte comme « code » le login choisi sur la fausse page d'autorisation.
type OAuth struct{ AuthorizePath string }

func (o OAuth) AuthorizeURL(state string) string { return o.AuthorizePath + "?state=" + state }

func (OAuth) Exchange(_ context.Context, code string) (app.GitHubIdentity, error) {
	return app.GitHubIdentity{ID: GitHubID(code), Login: code, CreatedAt: time.Now().AddDate(-3, 0, 0)}, nil
}

// Querier lit la base (pool ou connexion).
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GitHub répond qu'une PR est mergée, CI au vert, et l'attribue au prestataire
// de l'échange en cours sur ce dépôt : le parcours de livraison réussit.
type GitHub struct{ DB Querier }

func (GitHub) Repository(_ context.Context, owner, name string) (app.Repository, error) {
	return app.Repository{ID: repoID(owner, name), FullName: owner + "/" + name}, nil
}

func (g GitHub) PullRequest(ctx context.Context, owner, name string, n int) (app.PullRequest, error) {
	var author int64
	err := g.DB.QueryRow(ctx, `
		SELECT u.github_id FROM echange e JOIN utilisateur u ON u.id_utilisateur = e.id_prestataire
		WHERE lower(e.depot_owner) = lower($1) AND lower(e.depot_nom) = lower($2) AND e.statut_echange IN ('EN_COURS', 'EN_LITIGE')
		ORDER BY e.date_acceptation DESC NULLS LAST LIMIT 1`, owner, name).Scan(&author)
	if err != nil {
		return app.PullRequest{}, app.ErrGitHubNotFound
	}
	return app.PullRequest{ID: repoID(owner, name)*1000 + int64(n), Number: n, BaseRepoID: repoID(owner, name), AuthorID: author,
		CreatedAt: time.Now(), Merged: true, HeadSHA: fmt.Sprintf("f%06x", n*7919%0xffffff), MergeSHA: fmt.Sprintf("m%06x", n)}, nil
}

func (GitHub) CheckRuns(context.Context, string, string, string) ([]app.CheckRun, error) {
	return []app.CheckRun{{Name: "lint", Status: "completed", Conclusion: "success"}, {Name: "test", Status: "completed", Conclusion: "success"}}, nil
}
