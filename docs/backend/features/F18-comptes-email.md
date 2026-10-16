# F18 — Comptes par email et mot de passe

> Statut : **implémentée** · Taille : M · Dépend de : F1, F2 · Écrans : Accueil (connexion / inscription), Confirmer l'email, Réinitialiser le mot de passe, Profil (lier GitHub)

Jusqu'ici, seul GitHub ouvrait un compte. F18 ouvre l'inscription par **email
et mot de passe** aux personnes qui ont besoin d'aide sans avoir de compte
GitHub. GitHub reste la preuve d'identité des **livraisons** : un compte email
est demandeur seulement, tant qu'il n'a pas lié de compte GitHub.

## Décisions

| # | Décision |
|---|---|
| D22 | Email + mot de passe, confirmation de l'adresse par lien, réinitialisation par lien. Pas de « lien magique ». |
| D23 | Sans relais SMTP configuré, l'inscription et la connexion par email sont **fermées** en production (503 `inscription-email-indisponible`, formulaires masqués via `GET /auth/methodes`). En développement, les emails sont écrits dans le journal. |
| D24 | Bonus de bienvenue versé à la **confirmation** de l'adresse (pas à l'inscription), une seule fois ; pas de bonus pour une adresse dont le compte a été supprimé. |
| D25 | Sans GitHub lié : demander une offre, publier une annonce, payer, valider, contester, oui ; proposer une offre et candidater, non (403 `github-requis`). La liaison se fait depuis le profil. |

## Modèle (migration `0017_comptes_email`)

- `utilisateur` : `github_id`, `github_login`, `github_cree_le` deviennent facultatifs ; `mot_de_passe_hash` (Argon2id, format PHC) et `email_verifie_le` apparaissent. `CHECK` : un compte actif a GitHub ou un mot de passe ; un mot de passe implique un email.
- `jeton_email` : jetons à usage unique **hachés** (SHA-256), type `CONFIRMATION` (48 h) ou `REINITIALISATION` (1 h). Un nouveau lien invalide les précédents du même type.
- `oauth_state.id_utilisateur` : un state porteur d'un membre signifie « lier GitHub à ce compte » au lieu de « se connecter ».
- Suppression de compte (D13) : empreinte `sha256('email:' || adresse)` dans `github_supprime` (bloque un second bonus), jetons effacés, hash effacé.

## Règles

- **Mot de passe** : 10 à 128 caractères, différent de l'adresse. Argon2id m = 19 Mio, t = 2, p = 1 (recommandation OWASP).
- **Inscription** : réponse 202 identique que l'adresse soit libre ou non ; le titulaire d'une adresse déjà inscrite reçoit un email « compte existant » (pas d'énumération). Pseudo facultatif, déduit de l'adresse sinon (suffixe en cas de collision).
- **Connexion** : 401 `identifiants-invalides` sans distinguer adresse inconnue et mauvais mot de passe ; un hachage factice égalise les temps de réponse. 403 `email-non-confirme` tant que l'adresse n'est pas confirmée. Session identique à GitHub (jeton d'accès + cookie `ct_refresh`).
- **Réinitialisation** : 202 identique pour une adresse inconnue ; le nouveau mot de passe **ferme toutes les sessions** du compte ; le lien prouve la possession de l'adresse et la confirme si besoin (bonus compris, une fois).
- **Liaison GitHub** : `POST /me/github/lier` renvoie l'URL d'autorisation ; au retour, l'identité est liée au compte (même contrôle d'ancienneté qu'à la connexion). 409 `github-deja-lie` si ce compte GitHub appartient à un autre membre. Aucune fusion automatique de comptes par adresse.
- **Limitation de débit** : toutes les routes `/auth/*` sont limitées par adresse IP.
- **Emails** : texte brut UTF-8, en-têtes refusant CR/LF ; SMTP chiffré (TLS sur 465, STARTTLS obligatoire sinon). Un échec d'envoi est journalisé sans être révélé au client.

## Contrat

| Méthode | Chemin | Réponse |
|---|---|---|
| GET | `/auth/methodes` | `{github, email}` |
| POST | `/auth/inscription` | 202 |
| POST | `/auth/connexion` | 200 `AccessToken` + cookie |
| POST | `/auth/email/confirmer` | 204 |
| POST | `/auth/email/renvoyer` | 202 |
| POST | `/auth/mot-de-passe/oubli` | 202 |
| POST | `/auth/mot-de-passe/reinitialiser` | 204 |
| POST | `/me/github/lier` | 200 `{url}` + cookie state |

`Me` gagne `github_lie` et `email_verifie`.

## Configuration

`SMTP_HOST`, `SMTP_PORT` (587 par défaut), `SMTP_USER`, `SMTP_PASSWORD` (ou `SMTP_PASSWORD_FILE`), `SMTP_FROM`. Voir `docs/deploiement-vps.md`.
