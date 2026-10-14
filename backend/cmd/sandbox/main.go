//go:build js && wasm

// Commande sandbox : le vrai backend CodeTroc compilé en WebAssembly, pour le
// bac à sable navigateur. La base est PGlite (PostgreSQL en WASM) et GitHub est
// simulé (internal/devfake). Les requêtes du front arrivent par la fonction
// JavaScript codetrocHandle au lieu du réseau.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"syscall/js"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/gitb2pic/codetroc/backend/api"
	httpapi "github.com/gitb2pic/codetroc/backend/internal/adapters/http"
	"github.com/gitb2pic/codetroc/backend/internal/adapters/postgres"
	"github.com/gitb2pic/codetroc/backend/internal/adapters/vault"
	"github.com/gitb2pic/codetroc/backend/internal/app"
	"github.com/gitb2pic/codetroc/backend/internal/devfake"
	"github.com/gitb2pic/codetroc/backend/internal/platform/token"
)

// AuthorizePath est la fausse page d'autorisation GitHub, gérée par le front.
const authorizePath = "/sandbox/github/authorize"

var router *gin.Engine

func connConfig() (*pgx.ConnConfig, error) {
	cfg, err := pgx.ParseConfig("postgres://postgres@pglite/postgres?sslmode=disable")
	if err != nil {
		return nil, err
	}
	cfg.DialFunc = func(context.Context, string, string) (net.Conn, error) { return newJSConn(), nil }
	cfg.LookupFunc = func(context.Context, string) ([]string, error) { return []string{"127.0.0.1"}, nil }
	// Une seule session PostgreSQL partagée : pas d'instructions préparées nommées.
	cfg.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	return cfg, nil
}

func boot(ctx context.Context) error {
	cc, err := connConfig()
	if err != nil {
		return err
	}
	mdb := stdlib.OpenDB(*cc)
	mdb.SetMaxOpenConns(1)
	if err := postgres.MigrateUpDB(mdb); err != nil {
		return err
	}
	_ = mdb.Close()

	pcfg, err := pgxpool.ParseConfig("postgres://postgres@pglite/postgres?sslmode=disable&pool_max_conns=1")
	if err != nil {
		return err
	}
	pcfg.ConnConfig = cc
	pcfg.MaxConnLifetime, pcfg.MaxConnIdleTime, pcfg.HealthCheckPeriod = 24*time.Hour, 24*time.Hour, time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return err
	}

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
	settings := app.DefaultSettings()
	settings.WelcomeBonus = 300 // de quoi demander plusieurs packs dans la démo
	svc := app.New(app.Deps{DB: pool, Tokens: tokens, OAuth: devfake.OAuth{AuthorizePath: authorizePath},
		GitHub: devfake.GitHub{DB: pool}, Vault: sealer, Settings: settings})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	router, err = httpapi.NewRouter(httpapi.Options{Logger: logger, DB: pool, MaxBodyBytes: 1 << 20, ServeSpec: true, Spec: api.Spec,
		App: svc, FrontendURL: "/#/tableau-de-bord", CookieSecure: false})
	if err != nil {
		return err
	}
	if err := seed(ctx, svc, pool); err != nil {
		return fmt.Errorf("données de démonstration : %w", err)
	}
	go func() { // validations tacites, expirations, contrôle du grand livre
		for {
			time.Sleep(time.Minute)
			_, _ = svc.RunScheduledJobs(ctx)
		}
	}()
	return nil
}

// handle exécute une requête HTTP sur le routeur, sans réseau.
func handle(method, url string, headers map[string]string, body []byte) (int, map[string][]string, []byte) {
	req := httptest.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = "127.0.0.1:4242"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	out, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, rec.Header(), out
}

func promise(run func() (any, error)) js.Value {
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			v, err := run()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolve.Invoke(v)
		}()
		executor.Release()
		return nil
	})
	return js.Global().Get("Promise").New(executor)
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	js.Global().Set("codetrocBoot", js.FuncOf(func(js.Value, []js.Value) any {
		return promise(func() (any, error) { return nil, boot(context.Background()) })
	}))
	js.Global().Set("codetrocHandle", js.FuncOf(func(_ js.Value, args []js.Value) any {
		method, url, rawHeaders := args[0].String(), args[1].String(), args[2].String()
		var body []byte
		if len(args) > 3 && !args[3].IsNull() && !args[3].IsUndefined() {
			body = make([]byte, args[3].Get("length").Int())
			js.CopyBytesToGo(body, args[3])
		}
		return promise(func() (any, error) {
			var headers map[string]string
			if err := json.Unmarshal([]byte(rawHeaders), &headers); err != nil {
				return nil, err
			}
			if router == nil {
				return nil, fmt.Errorf("backend non démarré")
			}
			status, h, out := handle(method, url, headers, body)
			hj, _ := json.Marshal(h)
			arr := js.Global().Get("Uint8Array").New(len(out))
			js.CopyBytesToJS(arr, out)
			res := js.Global().Get("Object").New()
			res.Set("status", status)
			res.Set("headers", string(hj))
			res.Set("body", arr)
			return res, nil
		})
	}))
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("codetroc-wasm-ready"))
	select {}
}
