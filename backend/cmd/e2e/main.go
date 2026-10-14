//go:build e2e

// Commande e2e : serveur de test de bout en bout du front. Il assemble le vrai
// service et la vraie base, mais un faux GitHub (OAuth et API) et une clé de
// coffre éphémère, et sert le front construit sur la même origine.
//
// Compilé seulement avec -tags e2e : il n'entre jamais dans l'image de production.
//
//	DATABASE_URL=… FRONT_DIR=../frontend/dist go run -tags e2e ./cmd/e2e
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gitb2pic/codetroc/backend/api"
	httpapi "github.com/gitb2pic/codetroc/backend/internal/adapters/http"
	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/adapters/vault"
	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

const frontCSP = "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// Membres proposés par la fausse page d'autorisation GitHub.
var members = []string{"sami", "lea", "arbitre", "admin"}

// Le faux GitHub attribue les PR à « lea », la prestataire du scénario.
const prAuthor = "lea"

func githubID(login string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(login))
	return 100_000 + int64(h.Sum32()%1_000_000)
}

type fakeOAuth struct{}

func (fakeOAuth) AuthorizeURL(state string) string { return "/fake-github/authorize?state=" + state }

func (fakeOAuth) Exchange(_ context.Context, code string) (app.GitHubIdentity, error) {
	return app.GitHubIdentity{ID: githubID(code), Login: code, CreatedAt: time.Now().AddDate(-3, 0, 0)}, nil
}

type fakeGitHub struct{}

func repoID(owner, name string) int64 { return githubID("repo:" + owner + "/" + name) }

func (fakeGitHub) Repository(_ context.Context, owner, name string) (app.Repository, error) {
	return app.Repository{ID: repoID(owner, name), FullName: owner + "/" + name}, nil
}

func (fakeGitHub) PullRequest(_ context.Context, owner, name string, n int) (app.PullRequest, error) {
	return app.PullRequest{ID: repoID(owner, name)*1000 + int64(n), Number: n, BaseRepoID: repoID(owner, name), AuthorID: githubID(prAuthor),
		CreatedAt: time.Now(), Merged: true, HeadSHA: fmt.Sprintf("e2e%04d", n), MergeSHA: fmt.Sprintf("m2e%04d", n)}, nil
}

func (fakeGitHub) CheckRuns(context.Context, string, string, string) ([]app.CheckRun, error) {
	return []app.CheckRun{{Name: "lint", Status: "completed", Conclusion: "success"}, {Name: "test", Status: "completed", Conclusion: "success"}}, nil
}

var authorize = template.Must(template.New("a").Parse(`<!doctype html><meta charset="utf-8"><title>Faux GitHub</title>
<h1>Faux GitHub (e2e)</h1>{{range .Members}}<p><a href="/api/v1/auth/github/callback?state={{$.State}}&code={{.}}">Continuer en tant que {{.}}</a></p>{{end}}`))

func main() {
	if err := run(); err != nil {
		slog.Error("e2e", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if os.Getenv("E2E_RESET") == "1" { // base jetable : on repart de zéro
		if err := postgres.MigrateDown(dbURL); err != nil {
			return err
		}
	}
	if err := postgres.MigrateUp(dbURL); err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	sealer, err := vault.New(map[int][]byte{1: key}, 1)
	if err != nil {
		return err
	}
	tokens, err := token.NewIssuer(key, 15*time.Minute, time.Now)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	svc := app.New(app.Deps{DB: pool, Tokens: tokens, OAuth: fakeOAuth{}, GitHub: fakeGitHub{}, Vault: sealer, Settings: app.DefaultSettings()})
	router, err := httpapi.NewRouter(httpapi.Options{
		Logger: logger, DB: pool, MaxBodyBytes: 1 << 20, ServeSpec: true, Spec: api.Spec, App: svc,
		FrontendURL: "/tableau-de-bord", CookieSecure: false,
	})
	if err != nil {
		return err
	}

	router.GET("/fake-github/authorize", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = authorize.Execute(c.Writer, map[string]any{"State": c.Query("state"), "Members": members})
	})
	// Rôle d'un membre, pour préparer un scénario d'arbitrage.
	router.POST("/fake-admin/role", func(c *gin.Context) {
		_, err := pool.Exec(c, `UPDATE utilisateur SET role = $2 WHERE github_login = $1`, c.Query("login"), c.Query("role"))
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	})

	front := os.Getenv("FRONT_DIR")
	router.NoRoute(func(c *gin.Context) {
		// Même politique que frontend/nginx.conf (le front a la sienne, l'API garde default-src 'none').
		c.Header("Content-Security-Policy", frontCSP)
		p := filepath.Join(front, filepath.Clean("/"+c.Request.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() && !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.File(p)
			return
		}
		c.File(filepath.Join(front, "index.html")) // routes de la SPA
	})

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	logger.Warn("serveur e2e prêt", slog.String("addr", addr))
	srv := &http.Server{Addr: addr, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}
