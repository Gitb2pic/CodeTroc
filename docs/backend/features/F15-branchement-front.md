# F15 — Branchement du front

Statut : **livré**.

## Deux modes, un seul code

| Mode | Activation | Usage |
|---|---|---|
| Démonstration | par défaut (`npm run dev`, `build:artifact`) | prototype scénarisé sur `src/data/fixtures.ts`, publié en bac à sable |
| API | `VITE_DATA=api` (`npm run dev:api`, `build:api`, image Docker) | écrans de `src/live/` branchés sur `/api/v1` |

Le prototype reste intact (27 tests) ; le mode API réutilise ses composants
(`PackCard`, `Pill`, `Avatar`, `EscrowBar`, `EmptyState`…) et sa feuille de style.

## Client typé

- `src/api/schema.ts` est **généré** depuis `backend/api/openapi.yaml`
  (`npm run api:types`, openapi-typescript) ; la CI échoue si la spec et le client divergent.
- `src/api/client.ts` (openapi-fetch) : jeton d'accès **en mémoire seulement**,
  rafraîchi via le cookie HttpOnly `SameSite=Strict` ; sur 401, un seul
  rafraîchissement partagé puis rejeu ; erreurs RFC 9457 → `ApiError` (code, détail) ;
  `Idempotency-Key` neuve par intention sur accepter, refuser, annuler, valider, trancher.
- Le Bearer n'est jamais envoyé aux routes `/auth/*` (le test l'a révélé).

## Écrans branchés

Connexion GitHub, tableau de bord (solde, « à toi de jouer »), catalogue (recherche,
prix, délai), offre (critères, jalons, demande), **échange** (prochaine étape,
accepter/refuser, livrer une PR ou une note, revérifier, valider, contester,
annuler, coffre, messages, avis ; rafraîchi toutes les 20 s pour suivre les
webhooks), portefeuille (grand livre paginé), notifications (lu/non lu,
navigation vers l'échange ou le litige), litiges (réponse, décision de l'arbitre
avec aperçu du partage), profil (édition, compétences, suppression du compte),
création d'offre (pack, jalons, prix borné), administration (rôles, suspension,
affectation des litiges).

Sécurité côté client : aucun `dangerouslySetInnerHTML` (messages et avis rendus
tels quels, échappés par React) ; liens externes en `rel="noreferrer noopener"` ;
valeur d'un secret révélée effacée de l'écran après 30 s, jamais conservée ;
champs de formulaire avec libellés explicites et aides en `aria-describedby`.

## Déploiement

- `frontend/Dockerfile` : build Vite puis **nginx non privilégié**, conteneur en
  lecture seule ; `frontend/nginx.conf` : CSP stricte (`script-src 'self'`, pas de
  script en ligne), `nosniff`, `Referrer-Policy`, `Permissions-Policy`, COOP ;
  assets versionnés en cache long, `index.html` jamais en cache.
- `compose.yaml` : service `web` derrière Traefik sur la **même origine** que l'API
  (`/api` → API, le reste → web), condition du cookie `SameSite=Strict`.

## Tests

- Unitaires (Vitest) : client (Bearer, rafraîchissement unique et rejeu, problèmes
  RFC 9457, idempotence), mise en forme (offre → carte, notifications, dates).
- **E2E (Playwright) contre le vrai backend** : `backend/cmd/e2e` (compilé avec
  `-tags e2e`, absent des images) assemble le vrai service et une vraie base avec un
  faux GitHub (OAuth et API) et sert le front sur la même origine. Parcours :
  Léa publie une offre → Sami la demande → Léa accepte (séquestre) → Sami partage
  un jeton par le coffre, Léa le révèle → messages → PR vérifiée → validation ×2 →
  échange terminé → avis → portefeuille de Léa à 100 cr → notifications. Plus un
  test de cloisonnement (échange d'un autre : introuvable).
- CI : `.github/workflows/frontend.yml` (typecheck, tests, builds, client à jour,
  `npm audit`, e2e avec PostgreSQL).

## Limites connues

- Images construites et stack Compose vérifiée le 14 oct. 2026 (voir le rapport de pentest, § images).
- Pas de temps réel (SSE) : interrogation périodique de l'échange et du compteur.
- Mise en page bureau (≥ 1024 px) héritée de la maquette ; le responsive mobile reste à faire.
