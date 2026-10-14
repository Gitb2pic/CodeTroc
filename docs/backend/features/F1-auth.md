# F1 — Connexion GitHub et session

> Statut : terminée · Taille : M · Dépend de : F0 · Écran : Onboarding

## Objectif

Un visiteur se connecte avec son compte GitHub ; l'API l'identifie ensuite
par un jeton d'accès court, renouvelé par un jeton de rafraîchissement.

## Règles

- RG01 : un utilisateur par identifiant GitHub (créé à la première connexion).
- D12 : e-mail facultatif (adresse GitHub vérifiée et principale seulement).
- Compte SUSPENDU ou SUPPRIME : connexion et rafraîchissement refusés, jeton d'accès refusé dès la requête suivante.
- Âge minimal du compte GitHub configurable (`GITHUB_MIN_ACCOUNT_AGE_DAYS`, 0 par défaut) contre les comptes jetables.

## Contrat

| Méthode | Chemin | Rôle |
|---|---|---|
| GET | `/auth/github/login` | Crée un `state` à usage unique, le pose en cookie `HttpOnly`, redirige vers GitHub |
| GET | `/auth/github/callback` | Vérifie `state` (cookie = paramètre = base), échange le code, crée ou met à jour l'utilisateur, pose le cookie de rafraîchissement, redirige vers le front |
| POST | `/auth/refresh` | Échange le cookie contre un jeton d'accès (15 min) et fait tourner le cookie |
| POST | `/auth/logout` | Révoque toute la famille de jetons du cookie |
| GET | `/me` | Profil de l'utilisateur connecté |

Jetons :
- **Accès** : JWT HS256, 15 min, `iss=codetroc`, `aud=codetroc-api`, `sub=<uuid>`. Algorithme imposé à la vérification.
- **Rafraîchissement** : 32 octets aléatoires, stockés **hachés** (SHA-256), 30 jours, cookie `ct_refresh` `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/api/v1/auth`.
- **Rotation** : chaque rafraîchissement révoque l'ancien jeton. Présenter un jeton déjà révoqué révoque toute la famille (vol probable).

## Tests écrits avant le code

- Jeton : émis puis relu ; expiré, signé avec une autre clé, `alg: none`, mauvais émetteur ou audience → refusés.
- Connexion : `state` absent, différent du cookie, expiré ou rejoué → 401 ; création au premier passage, mise à jour du login ensuite ; deux callbacks simultanés pour un nouvel utilisateur → un seul compte ; pseudo déjà pris → suffixe ; compte trop récent → 403 ; compte suspendu → 403.
- Rafraîchissement : rotation ; ancien jeton rejoué → 401 et famille révoquée ; jeton expiré → 401 ; déconnexion → jeton inutilisable.
- `/me` : 401 sans jeton ou avec un jeton invalide (en-tête `WWW-Authenticate`) ; 401 si le compte a été suspendu après l'émission du jeton.
- Adaptateur GitHub (serveur simulé) : échange du code, lecture de `/user` et de l'e-mail principal vérifié, erreurs GitHub.

## Menaces

| Menace | Parade |
|---|---|
| CSRF de connexion (lier la session à l'attaquant) | `state` aléatoire lié au navigateur par cookie et consommé en base |
| Vol du jeton de rafraîchissement | `HttpOnly`, `SameSite=Strict`, chemin restreint, rotation et détection de réutilisation |
| Falsification du JWT | Clé ≥ 32 octets hors dépôt, HS256 imposé, `exp`/`iss`/`aud` vérifiés |
| Compte suspendu qui garde un jeton valide | Statut relu en base à chaque requête authentifiée |
| Fuite du jeton dans l'URL ou les logs | Aucun jeton dans une URL ; le front obtient l'accès via `POST /auth/refresh` |
