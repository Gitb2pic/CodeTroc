# CodeTroc — Plan de développement du backend

> Statut : **proposition à valider** avant toute ligne de code.
> Ce plan remplace la stack back-end de la documentation technique (FastAPI)
> par **Go + Gin**. Le modèle de données, les règles de gestion (RGxx) et les
> patterns (hexagonal, Unit of Work, État, grand livre) restent ceux de la doc
> technique, corrigés des points relevés en relecture (section 2).

## Sommaire

1. Stack et choix techniques
2. Décisions de conception à valider
3. Architecture du code
4. Méthode : une feature à la fois, en TDD
5. Stratégie de tests
6. Sécurité : DevSecOps et pentest
7. Backlog feature par feature
8. Ordre, dépendances et jalons
9. Ce que j'attends de toi pour démarrer

---

## 1. Stack et choix techniques

| Besoin | Choix | Pourquoi |
|---|---|---|
| Langage | Go 1.25 | Typage fort, binaire unique, concurrence simple pour le worker |
| HTTP | Gin | Demandé ; routeur rapide, middlewares simples |
| Base | PostgreSQL 16 | Transactions ACID, contraintes CHECK, index partiels, verrous `FOR UPDATE` |
| Accès SQL | pgx v5 + **sqlc** | SQL écrit à la main et vérifié à la compilation ; pas d'ORM qui masque les verrous |
| Migrations | golang-migrate | Fichiers SQL versionnés `up`/`down`, exécutés en CI et au déploiement |
| GitHub | go-github + GitHub App (JWT d'app, jetons d'installation) | OAuth pour l'identité, App pour les webhooks et la lecture des PR/checks |
| Auth | golang-jwt v5 (accès court) + refresh opaque en base | Rotation et révocation possibles (impossible avec un refresh JWT pur) |
| Config | Variables d'environnement + fichiers de secrets Docker | Aucun secret dans le dépôt |
| Logs | `log/slog` (JSON) + identifiant de requête | Bibliothèque standard, corrélation avec Traefik |
| Métriques | Prometheus (client_golang) | Déjà prévu dans la doc technique |
| Doc API | OpenAPI 3.1 écrit en premier (`api/openapi.yaml`) | Contrat partagé avec le front React ; client TypeScript généré |
| Tests | `testing` + testify, **rapid** (tests de propriétés), PostgreSQL réel (service CI, `CODETROC_TEST_DATABASE_URL`), `httptest`, fuzzing natif Go | Voir section 5 |
| Qualité / sécurité | golangci-lint (dont gosec, errcheck, staticcheck), govulncheck, gitleaks, Trivy, OWASP ZAP | Voir section 6 |
| Exécution | Docker multi-stage (image distroless, utilisateur non root) + Docker Compose + Traefik | Conforme à la section 14 de la doc technique |

Deux binaires : `cmd/api` (HTTP sans état, plusieurs instances possibles) et
`cmd/worker` (validation tacite, échéances, expiration du coffre,
réconciliation GitHub ; une seule instance active grâce à
`pg_try_advisory_lock`).

---

## 2. Décisions de conception à valider

Ces points corrigent les contradictions et les failles relevées dans la
relecture des documents. Chacun a une recommandation ; ceux marqués
**à trancher** bloquent une feature précise.

| # | Sujet | Recommandation | Bloque |
|---|---|---|---|
| D1 | **Politique monétaire** (bonus ou avance) | **À trancher.** Recommandé : bonus de bienvenue de 50 cr débité du compte ÉMISSION (c'est ce que montre l'UI), solde jamais négatif, plafond de solde positif paramétrable. L'alternative du dossier projet (avance à −10 cr, aucun crédit créé) reste possible. | F2 |
| D2 | **Séquestre** | Un compte SÉQUESTRE **par échange**, et non un compte global : solde vérifiable par échange et pas de ligne unique verrouillée par toutes les transactions. | F2, F6 |
| D3 | **Traçabilité des mouvements** | Chaque mouvement porte `id_echange` (et `id_jalon` quand il y en a un). | F2 |
| D4 | **Idempotence** | Clés dérivées par le serveur pour les mouvements (`blocage:{echange}`, `liberation:{jalon}`…) + table `idempotency_key` limitée au couple (utilisateur, route) pour les en-têtes `Idempotency-Key`, vérifiée **après** le contrôle d'accès. | F2, F6 |
| D5 | **GitHub App** | Une GitHub App (permissions : `pull_requests:read`, `checks:read`, `metadata:read`) installée sur le dépôt du demandeur à l'acceptation. Sans installation, pas de preuve automatique. | F7 |
| D6 | **Validité d'une preuve** | PR du dépôt de l'échange (comparé par **id** de dépôt), auteur = prestataire, créée après l'acceptation, jamais utilisée pour un autre jalon, mergée, checks du commit en succès, **au moins un check**. | F7 |
| D7 | **Livrables non-code** | Champ `type_preuve` sur le pack : `PR_CI` ou `MANUELLE` (revue, audit). Une preuve manuelle passe par la validation du demandeur avec le même délai tacite. | F4, F7 |
| D8 | **Abandon par le prestataire** | Échéance par jalon (issue de `delai_jours`). Après échéance + délai de grâce, le demandeur peut annuler seul : remboursement des jalons non livrés. | F8 |
| D9 | **Arbitrage** | **À trancher.** La doc technique prévoit 1 arbitre ; l'UI montre un vote de 3 pairs à la majorité. Recommandé : 1 arbitre au MVP, jury de 3 en V2 (le modèle de données le permet). | F9 |
| D10 | **Répartition PARTAGER** | Part du prestataire arrondie à l'inférieur, le reste au demandeur ; la somme vaut toujours le montant du jalon. | F9 |
| D11 | **Erreurs 403 / 404** | Toujours 404 sur une ressource que l'utilisateur ne peut pas voir (anti-énumération), 403 seulement quand il la voit mais n'a pas le droit d'agir. | F0 |
| D12 | **E-mail GitHub privé** | `email` nullable ; scope OAuth `read:user user:email` pour récupérer l'adresse vérifiée quand elle existe. | F1 |
| D13 | **Suppression de compte** | Anonymisation (pseudo, e-mail, bio effacés) et non suppression physique, pour garder le grand livre intègre. | F14 |

---

## 3. Architecture du code

Architecture hexagonale, comme dans la doc technique : le domaine ne dépend ni
de Gin, ni de pgx, ni de GitHub.

```
backend/
├── cmd/
│   ├── api/                 # main HTTP (Gin)
│   └── worker/              # tâches planifiées
├── api/openapi.yaml         # contrat, écrit avant le code de chaque feature
├── internal/
│   ├── domain/              # Go pur : entités, machines à états, ledger, erreurs métier
│   │   ├── ledger/
│   │   ├── exchange/        # échange + jalons + transitions
│   │   ├── catalog/
│   │   ├── dispute/
│   │   └── vault/
│   ├── app/                 # cas d'utilisation (services) + ports (interfaces)
│   ├── adapters/
│   │   ├── http/            # handlers Gin, DTO, middlewares (auth, request id, RFC 9457)
│   │   ├── postgres/        # requêtes sqlc, Unit of Work, repositories
│   │   ├── github/          # OAuth, App, vérification des preuves, webhooks
│   │   └── crypto/          # AES-256-GCM, enveloppe DEK/KEK
│   └── platform/            # config, logs, métriques, horloge injectable
├── migrations/              # 0001_init.up.sql …
├── testdata/                # charges utiles GitHub enregistrées
└── Dockerfile
```

Règles :
- Les handlers ne contiennent aucune règle métier : ils valident l'entrée,
  appellent un service, traduisent le résultat.
- Un cas d'utilisation = une transaction (Unit of Work). Verrous `FOR UPDATE`
  toujours pris dans le même ordre : échange, puis comptes triés par id.
- L'horloge est injectée (`Clock`) : les délais (validation tacite,
  échéances) se testent sans attendre.
- Toute transition d'état passe par la machine à états du domaine.

---

## 4. Méthode : une feature à la fois, en TDD

Chaque feature suit le même cycle. On ne commence la suivante que lorsque
la précédente est mergée.

### Definition of Ready (avant de coder)

1. **Fiche feature** validée par toi : objectif, règles RGxx concernées,
   endpoints, critères d'acceptation.
2. **Contrat OpenAPI** de ses endpoints ajouté à `api/openapi.yaml`.
3. **Migration SQL** prévue (tables, contraintes, index).
4. **Mini modèle de menaces** (STRIDE ciblé sur la feature).
5. **Liste des tests** écrite avant le code (section 7, rubrique « Tests TDD »).

### Boucle de développement

1. **Rouge** : écrire le test du domaine qui échoue.
2. **Vert** : le code minimal qui le fait passer.
3. **Refactor** : nettoyer, tests toujours verts.
4. Monter d'un étage : test d'intégration (Postgres réel), puis test d'API.
5. Ajouter les tests de sécurité de la feature (matrice d'accès, abus).

### Definition of Done

- Tous les tests de la fiche écrits et verts, avec `-race`.
- Couverture : `domain/` ≥ 90 %, `app/` ≥ 80 %.
- Lint, gosec, govulncheck, gitleaks propres.
- Contrat OpenAPI respecté (tests d'API validés contre le schéma).
- Checklist sécurité de la feature cochée (section 6).
- Une PR par feature, relue, CI au vert. CodeTroc s'applique sa propre règle.

---

## 5. Stratégie de tests

| Niveau | Outil | Cible | Exemple |
|---|---|---|---|
| Unitaires domaine | `testing`, testify | Règles, transitions, ledger, sans base | `LIVRE → REMBOURSE` est refusé |
| Propriétés | rapid | Invariants sur des séquences aléatoires | Après N opérations, la somme des soldes vaut 0 et aucun solde utilisateur n'est négatif |
| Fuzzing | `go test -fuzz` | Entrées hostiles | Parseur d'URL de PR, payload de webhook, en-têtes de signature |
| Intégration | PostgreSQL 16 réel (service de la CI, base locale en dev) | Repositories, contraintes SQL, verrous | Deux acceptations concurrentes ne créent qu'un mouvement |
| API | `httptest` + routeur Gin réel | Codes HTTP, format RFC 9457, autorisations | Un tiers reçoit 404 sur `GET /echanges/{id}` |
| Contrat | validation des réponses contre `openapi.yaml` | Dérive entre code et contrat | Champ manquant ou en trop = échec |
| Webhooks | charges utiles GitHub enregistrées (`testdata/`) | Signature, doublons, CI en échec | Signature invalide → 401, livraison rejouée → ignorée |
| Concurrence | goroutines + `-race` | Courses sur les crédits | 50 validations simultanées du même jalon → 1 seule libération |
| Bout en bout | Playwright (front) contre l'API en staging | Parcours de démo complet | Demande → acceptation → preuve → libération |

Contrôle permanent : la requête d'intégrité du grand livre (somme des
soldes = 0, solde = crédits − débits) tourne après chaque test d'intégration
et en tâche planifiée en production.

---

## 6. Sécurité : DevSecOps et pentest

### 6.1 Contrôles automatiques à chaque PR

| Étape | Outil | Échec si |
|---|---|---|
| Analyse statique | golangci-lint + gosec | Toute alerte gosec non justifiée |
| Dépendances Go | govulncheck | Vulnérabilité atteignable |
| Secrets | gitleaks (pre-commit + CI) | Tout secret détecté |
| Image | Trivy | Vulnérabilité CRITICAL |
| Tests de sécurité | suites `*_security_test.go` | Un test de la matrice d'accès échoue |
| DAST | OWASP ZAP baseline sur staging (branche main) | Alerte de niveau High |

### 6.2 Tests de sécurité écrits avec chaque feature

- **Matrice d'accès** : pour chaque route, un test par rôle (anonyme, membre
  tiers, demandeur, prestataire, arbitre, admin) avec le code attendu.
- **IDOR / BOLA** : accès aux ressources d'un autre utilisateur par id.
- **Mass assignment** : champs non autorisés dans le corps (montant, statut, rôle) ignorés ou refusés.
- **Rejeu** : même `Idempotency-Key`, même `X-GitHub-Delivery`.
- **Course** : requêtes parallèles sur les opérations financières.
- **Entrées hostiles** : fuzzing des parseurs, tailles maximales, Unicode.

### 6.3 Pentest

Trois temps, toujours sur **staging**, jamais en production, avec des
comptes GitHub de test.

1. **Pentest de fin de feature critique** (après F2, F6, F7, F8, F12) :
   1 à 2 h ciblées sur la surface ajoutée, avec la checklist ci-dessous.
2. **Pentest de release MVP** : une passe complète guidée par
   OWASP API Security Top 10 (2023) et ASVS niveau 2 (chapitres
   authentification, sessions, contrôle d'accès, validation, cryptographie,
   logique métier).
3. **Re-test** de chaque vulnérabilité corrigée, avec un test automatisé de
   non-régression ajouté à la suite.

Checklist pentest CodeTroc :

| Zone | Scénarios |
|---|---|
| Auth | `state` OAuth absent ou rejoué ; JWT `alg: none`, signature modifiée, jeton expiré ; refresh volé puis réutilisé après rotation |
| Contrôle d'accès | BOLA sur échanges, jalons, secrets, litiges ; élévation vers ARBITRE ou ADMIN |
| Logique métier | Accepter sa propre offre ; jalons dont la somme ≠ prix ; libérer deux fois ; valider un jalon non livré ; preuve avec la PR d'un autre ; double bonus par comptes multiples |
| Concurrence | Rafales parallèles sur accepter / valider / décision (outil Go maison) |
| Webhooks | Signature forgée, payload modifié, rejeu, webhook d'un autre dépôt |
| SSRF | `depot_url` vers autre chose que github.com, redirections, IP internes |
| Coffre | Lecture après révocation, après clôture, par un tiers ; secret copié vers un autre échange (AAD) ; valeur dans les logs |
| Infra | En-têtes de sécurité, TLS, CORS, limitation de débit, docs OpenAPI exposées en production |

Outils : OWASP ZAP ou Burp Community, `curl`/scripts Go pour les rafales,
nuclei pour les mauvaises configurations. Chaque constat devient une issue
avec sévérité (CVSS), correctif et test de non-régression.

---

## 7. Backlog feature par feature

Taille : S (≤ 1 j), M (2–3 j), L (4–5 j). Les écrans cités sont ceux du front.

### F0 — Socle technique · M

- **Objectif** : un service qui démarre, se teste et se déploie, sans métier.
- **Contenu** : arborescence, config, `slog`, request id, erreurs RFC 9457,
  `/api/v1/health` et `/ready`, migrations, Docker Compose (Postgres,
  API, Traefik), pipeline CI complet (section 6.1), squelette OpenAPI.
- **Tests TDD** : format d'erreur RFC 9457 ; `/ready` en 503 si la base est
  absente ; le middleware attribue un request id ou propage celui de Traefik ;
  la config refuse de démarrer sans secret obligatoire.
- **Sécurité** : en-têtes de sécurité, taille de corps limitée, docs OpenAPI désactivées en production.

### F1 — Connexion GitHub et session · M · écran Onboarding

- **Endpoints** : `GET /auth/github/login`, `GET /auth/github/callback`,
  `POST /auth/refresh`, `POST /auth/logout`, `GET /me`.
- **Règles** : RG01 ; D12.
- **Tests TDD** : `state` signé et à usage unique ; création de
  l'utilisateur à la première connexion (une seule fois même si deux
  callbacks arrivent en même temps) ; compte suspendu refusé ; rotation du
  refresh, réutilisation d'un ancien refresh = révocation de toute la famille ;
  JWT expiré ou mal signé → 401.
- **Pentest** : CSRF OAuth, vol et rejeu de refresh, `alg: none`.

### F2 — Grand livre et comptes · L · écrans Portefeuille, barre de solde

- **Endpoints** : `GET /me/compte`, `GET /me/compte/mouvements` (paginé).
- **Règles** : RG02 à RG05 ; D1 à D4.
- **Tests TDD** (le cœur du produit) :
  - un transfert débite et crédite le même montant strictement positif ;
  - un compte utilisateur ne passe jamais sous 0 (domaine **et** contrainte SQL) ;
  - une clé d'idempotence rejouée ne crée pas de second mouvement ;
  - propriété : N transferts aléatoires → somme des soldes = 0 ;
  - concurrence : 100 transferts parallèles depuis le même compte → solde exact, jamais négatif ;
  - la requête d'intégrité ne renvoie aucune ligne ;
  - bonus de bienvenue versé exactement une fois.
- **Pentest** : course sur le solde, double bonus.

### F3 — Profil et compétences · S · écrans Onboarding, Profil

- **Endpoints** : `PATCH /me`, `PUT /me/competences`, `GET /competences`, `GET /utilisateurs/{id}`.
- **Tests TDD** : niveaux valides seulement ; remplacement atomique de la liste ; profil public sans e-mail.
- **Pentest** : mass assignment sur `role`, XSS stocké dans `bio` (échappement côté front, longueur bornée côté API).

### F4 — Catalogue de packs · M · écrans Catalogue, Détail du pack

- **Endpoints** : `GET /packs` (filtres stack, prix, délai, réputation ; pagination par curseur), `GET /packs/{id}`, `POST/PATCH /packs` (admin).
- **Règles** : RG07 ; D7.
- **Tests TDD** : `prix_min ≤ prix_max` ; au moins une compétence ; filtres combinés ; curseur stable ; pack inactif invisible.
- **Pentest** : création de pack par un membre, injection dans les filtres.

### F5 — Offres · S · écrans Création d'un pack, Profil

- **Endpoints** : `GET /offres`, `POST /offres`, `POST /offres/{id}/retirer`.
- **Règles** : RG08.
- **Tests TDD** : prix dans la fourchette du pack ; seul l'auteur retire ; offre retirée invisible.

### F6 — Échanges : demande, acceptation, séquestre · L · écrans Espace d'échange, Tableau de bord

- **Endpoints** : `POST /echanges`, `GET /echanges`, `GET /echanges/{id}`,
  `POST /echanges/{id}/accepter|refuser|annuler`.
- **Règles** : RG09 à RG11, RG18 ; D2, D4.
- **Tests TDD** : demandeur ≠ prestataire ; somme des jalons = prix ;
  acceptation → mouvement BLOCAGE vers le séquestre de l'échange ;
  solde devenu insuffisant → 422 ; double acceptation simultanée → un seul
  blocage ; transitions interdites → 409 ; annulation → remboursement des
  jalons non libérés ; `depot_url` limité à `https://github.com/{owner}/{repo}`.
- **Pentest** : BOLA sur les échanges, course sur l'acceptation, SSRF.

### F7 — Preuves GitHub et webhooks · L · écran Espace d'échange (bloc PR + checks)

- **Endpoints** : `POST /jalons/{id}/preuves`, `POST /jalons/{id}/reverifier`, `POST /webhooks/github`.
- **Règles** : RG12, RG13 ; D5, D6, D7.
- **Tests TDD** : signature HMAC en temps constant ; `X-GitHub-Delivery`
  déjà vu → ignoré ; le corps du webhook n'est qu'un signal (décision prise
  par appel à l'API) ; chaque condition de D6 a son test d'échec ; CI en
  échec → preuve REJETEE ; zéro check → REJETEE ; API GitHub indisponible →
  preuve laissée EN_ATTENTE et nouvel essai ; jalons livrés dans l'ordre.
- **Fuzzing** : parseur d'URL de PR, décodage des payloads.
- **Pentest** : webhook forgé ou rejoué, PR d'un autre auteur ou d'un autre dépôt.

### F8 — Validation, libération, délais · M · écrans Espace d'échange, Portefeuille

- **Endpoints** : `POST /jalons/{id}/valider` ; worker.
- **Règles** : RG14, RG21 ; D8.
- **Tests TDD** (horloge simulée) : libération = mouvement séquestre →
  prestataire ; validation tacite après le délai ; litige ouvert = pas de
  validation tacite ; dernier jalon → échange TERMINE ; échéance dépassée →
  annulation possible par le demandeur ; deux workers → une seule exécution
  (verrou consultatif).
- **Pentest** : validation par le prestataire, double libération concurrente.

### F9 — Litiges et arbitrage · M · écran Litige et arbitrage

- **Endpoints** : `POST /jalons/{id}/litiges`, `GET /litiges`, `POST /litiges/{id}/decision`.
- **Règles** : RG15 à RG17 ; D9, D10.
- **Tests TDD** : un seul litige ouvert par jalon ; arbitre jamais partie à
  l'échange ; LIBERER / REMBOURSER / PARTAGER produisent les bons mouvements ;
  propriété : la somme des parts = montant du jalon ; aucun arbitre → file admin.
- **Pentest** : décision par un non-arbitre, arbitre partie prenante.

### F10 — Avis et réputation · S · écrans Profil, Tableau de bord

- **Endpoints** : `POST /echanges/{id}/avis`.
- **Règles** : RG20, RG21.
- **Tests TDD** : un avis par partie et par échange, seulement si TERMINE ; note 1–5 ; calcul de réputation déterministe.

### F11 — Messagerie d'échange · S · écran Espace d'échange (Discussion)

- **Endpoints** : `GET/POST /echanges/{id}/messages`.
- **Règles** : RG22.
- **Tests TDD** : seules les parties écrivent ; l'arbitre lit pendant un litige ; 5 000 caractères max ; pagination.

### F12 — Coffre d'accès · L · écran Coffre d'accès

- **Endpoints** : `GET/POST /echanges/{id}/secrets`, `POST /secrets/{id}/reveler|revoquer` ; worker d'expiration.
- **Règles** : RG19.
- **Tests TDD** : chiffrement d'enveloppe (DEK par secret, KEK hors base) ;
  l'id d'échange en AAD : un secret copié ailleurs ne se déchiffre pas ;
  valeur jamais dans les logs (test qui capture les logs) ; révélation
  tracée (qui, quand, IP) ; lecture refusée après révocation, clôture ou
  expiration ; valeur effacée à la révocation.
- **Pentest** : lecture par un tiers, rejeu après révocation, fuite dans les logs et les erreurs.

### F13 — Notifications · M · écran Notifications, cloche

- **Endpoints** : `GET /notifications`, `POST /notifications/lire`.
- **Contenu** : événements de domaine (jalon livré, CI, crédits, accès, litige) enregistrés dans la même transaction que l'action (outbox).
- **Tests TDD** : chaque événement crée la bonne notification pour le bon destinataire ; « tout marquer comme lu ».

### F14 — Administration et modération · S

- **Endpoints** : `PATCH /admin/utilisateurs/{id}`, file des litiges sans arbitre, suppression de compte (D13).
- **Tests TDD** : seul ADMIN accède ; suspension = sessions révoquées ; anonymisation sans casser le grand livre.

### F15 — Branchement du front · M

- Client TypeScript généré depuis `openapi.yaml`, remplacement de
  `src/data/fixtures.ts` écran par écran, tests Playwright du parcours de démo
  contre l'API en staging.

---

## 8. Ordre, dépendances et jalons

```
F0 → F1 → F2 → F3 → F4 → F5 → F6 → F7 → F8   ← parcours de démo complet (jalon A)
                                    ↘ F11
F8 → F9 → F10                                  ← confiance (jalon B)
F6 → F12 ; F13 ; F14                           ← jalon C
F15 au fil de l'eau, dès F4
```

| Jalon | Features | Démontre |
|---|---|---|
| A — Démo hackathon | F0 à F8 (+ F11) | Demande → séquestre → PR + CI → libération |
| B — Confiance | F9, F10 | Litiges et réputation |
| C — Complet MVP | F12, F13, F14, pentest de release | Coffre, notifications, admin |

Si le temps manque pour le hackathon, le jalon A suffit : c'est le
parcours que le dossier projet met au centre du pitch.

---

## 9. Ce que j'attends de toi pour démarrer

1. Valider ou ajuster ce plan (stack, ordre des features, méthode).
2. Trancher **D1** (bonus ou avance) et **D9** (1 arbitre ou jury de 3).
3. Valider la fiche de **F0** : je la détaille alors (contrat, tests, menaces)
   et je ne code qu'après ton accord.
