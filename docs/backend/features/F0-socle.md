# F0 — Socle technique

> Statut : terminée · Taille : M · Dépend de : rien

## Objectif

Un service Go qui démarre, se teste, se déploie et se surveille, sans aucune
règle métier. Toutes les features suivantes s'appuient dessus.

## Périmètre

| Élément | Contenu |
|---|---|
| Arborescence | `backend/` hexagonal (voir PLAN, section 3) |
| Config | Variables d'environnement ; tout secret lisible via `NOM_FILE` (secrets Docker) |
| Logs | JSON via `log/slog`, une ligne par requête avec `request_id`, méthode, route, statut, durée |
| Request id | En-tête `X-Request-Id` repris s'il est valide, sinon généré ; renvoyé dans la réponse |
| Erreurs | Format RFC 9457 (`application/problem+json`) partout, y compris 404, 405, 413 et panique |
| Santé | `GET /api/v1/health` (processus vivant) et `GET /api/v1/ready` (base joignable) |
| Migrations | golang-migrate, fichiers SQL embarqués, binaire `cmd/migrate` |
| Sécurité HTTP | En-têtes de sécurité, corps limité (1 Mo par défaut), proxys de confiance explicites, délais serveur |
| Contrat | `backend/api/openapi.yaml` ; servi seulement hors production |
| Exécution | Dockerfile multi-stage (distroless, non root), `compose.yaml` (Postgres, migration, API, Traefik) |
| CI | Lint (golangci-lint, dont gosec), tests `-race` avec Postgres, govulncheck, gitleaks, build d'image + Trivy |

Hors périmètre : métriques Prometheus (ajoutées avec F2), TLS Let's Encrypt
(configuration de déploiement), tout endpoint métier.

## Contrat

| Méthode | Chemin | Réponses |
|---|---|---|
| GET | `/api/v1/health` | 200 `{"status":"ok"}` |
| GET | `/api/v1/ready` | 200 `{"status":"ready"}` ; 503 problème `service-indisponible` |
| GET | `/api/v1/openapi.yaml` | 200 hors production ; 404 en production |
| * | toute autre route | 404 `introuvable` ; 405 `methode-non-autorisee` |

Format d'erreur :

```json
{
  "type": "/problems/introuvable",
  "title": "Ressource introuvable",
  "status": 404,
  "detail": "…",
  "instance": "/api/v1/inconnu",
  "request_id": "7f3a91c2…"
}
```

## Tests écrits avant le code

**Config**
- [x] refuse de démarrer sans `DATABASE_URL`
- [x] lit un secret depuis `DATABASE_URL_FILE` (espaces de fin retirés)
- [x] refuse `DATABASE_URL` et `DATABASE_URL_FILE` définis en même temps
- [x] refuse un `APP_ENV` inconnu, une taille de corps invalide
- [x] valeurs par défaut : `:8080`, 1 Mo, `development`

**HTTP**
- [x] erreur au format RFC 9457 avec `request_id`
- [x] 404 et 405 en RFC 9457
- [x] une panique renvoie 500 RFC 9457 sans détail interne, et elle est journalisée
- [x] le request id entrant valide est repris ; un id invalide (trop long, caractères de contrôle) est remplacé
- [x] chaque réponse porte les en-têtes de sécurité
- [x] corps trop gros → 413
- [x] `/health` → 200 ; `/ready` → 200 si la base répond, 503 sinon
- [x] `openapi.yaml` servi en développement, 404 en production
- [x] les réponses respectent `openapi.yaml` (test de contrat)
- [x] le journal de requête ne contient ni la chaîne de requête ni les en-têtes

**Base (intégration, PostgreSQL réel)**
- [x] les migrations montent puis redescendent sans erreur
- [x] `/ready` → 200 contre une vraie base

## Menaces (STRIDE ciblé)

| Menace | Scénario | Parade |
|---|---|---|
| Falsification | Injection de lignes de log via `X-Request-Id` | Id validé (`[A-Za-z0-9._-]{1,64}`), sinon régénéré |
| Divulgation | Pile d'appel ou erreur SQL renvoyée au client | Récupération des paniques, 500 générique, détail seulement dans les logs |
| Divulgation | Jeton dans une chaîne de requête copié dans les logs | Le log de requête n'enregistre que le chemin |
| Divulgation | Contrat d'API exposé en production | `openapi.yaml` non servi en production |
| Déni de service | Corps énorme, connexion lente (Slowloris) | Limite de corps, `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` |
| Usurpation | Fausse IP cliente via `X-Forwarded-For` | Aucun proxy de confiance par défaut ; liste explicite `TRUSTED_PROXIES` |
| Élévation | Conteneur compromis | Image distroless, utilisateur non root, système de fichiers en lecture seule, base sur réseau interne |

## Critères d'acceptation

- `go test -race ./...` vert, intégration comprise avec `CODETROC_TEST_DATABASE_URL`.
- `golangci-lint run` propre (gosec compris), `govulncheck` sans vulnérabilité atteignable.
- `docker compose config` valide.
- CI GitHub Actions définie pour lint, tests, sécurité et image.
