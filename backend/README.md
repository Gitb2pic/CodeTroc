# CodeTroc — backend

API REST de CodeTroc en Go (Gin, PostgreSQL). Plan : `docs/backend/PLAN.md`.
Fiches des features : `docs/backend/features/`.

## Démarrer avec Docker

```bash
./scripts/dev-secrets.sh        # une fois : génère ./secrets (non commité)
docker compose up --build
curl http://localhost:8000/api/v1/ready
```

Compose lance PostgreSQL (réseau interne, sans port publié), applique les
migrations, puis démarre l'API derrière Traefik.

## Développer sans Docker

```bash
cd backend
export DATABASE_URL="postgres://user:pass@localhost:5432/codetroc?sslmode=disable"
go run ./cmd/migrate up
go run ./cmd/api                # http://localhost:8080/api/v1/health
```

| Variable | Défaut | Rôle |
|---|---|---|
| `DATABASE_URL` ou `DATABASE_URL_FILE` | obligatoire | Connexion PostgreSQL |
| `APP_ENV` | `development` | `development`, `test` ou `production` (masque `openapi.yaml`) |
| `HTTP_ADDR` | `:8080` | Adresse d'écoute |
| `MAX_BODY_BYTES` | `1048576` | Taille maximale d'un corps de requête |
| `TRUSTED_PROXIES` | aucun | IP ou CIDR des proxys dont on lit `X-Forwarded-For` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

Tout secret `NOM` peut être lu depuis un fichier avec `NOM_FILE`.

## Tests

```bash
go test -race ./...                     # unitaires ; l'intégration est sautée
CODETROC_TEST_DATABASE_URL=postgres://… go test -race ./...   # tout
golangci-lint run ./...
```

En CI, `CODETROC_REQUIRE_DB=1` fait échouer les tests d'intégration au lieu
de les sauter si la base manque.

## Organisation

```
cmd/api, cmd/migrate        binaires
api/openapi.yaml            contrat (écrit avant le code)
internal/adapters/http      routeur Gin, middlewares, erreurs RFC 9457
internal/adapters/postgres  pool pgx, migrations
internal/platform           configuration, logs
migrations/                 SQL versionné, embarqué dans les binaires
```
