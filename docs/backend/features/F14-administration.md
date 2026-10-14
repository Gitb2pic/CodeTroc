# F14 — Administration et modération

Statut : **livré**. Décision : D13.

## Endpoints

| Endpoint | Accès | Effet |
|---|---|---|
| `GET /admin/utilisateurs?q=` | ADMIN | recherche par pseudo ou login GitHub (motifs `%` `_` échappés), avec statut |
| `PATCH /admin/utilisateurs/{id}` | ADMIN | `role` (MEMBRE, ARBITRE, ADMIN) et/ou `statut` (ACTIF, SUSPENDU) |
| `GET /admin/litiges` | ADMIN | file des litiges ouverts sans arbitre (affectation : `POST /litiges/{id}/affecter`, F9) |
| `DELETE /me` | membre | suppression du compte par anonymisation |

Le contrôle du rôle est fait **dans le service** (pas seulement dans le routeur) :
un membre ou un arbitre reçoit 403 `reserve-admin` sur chaque opération.

## Règles

- **Jamais sur soi-même** (422 `auto-modification`) : un administrateur ne peut ni se
  rétrograder ni se suspendre, ce qui évite de perdre le dernier accès d'administration.
- **Suspension** : toutes les sessions de rafraîchissement sont révoquées, et le jeton
  d'accès cesse de fonctionner tout de suite (le statut est relu à chaque requête, F1).
  Les offres du membre disparaissent du catalogue (filtre existant).
- **Arbitre suspendu ou rétrogradé** : ses litiges ouverts retournent dans la file.
- Chaque changement de rôle ou de statut est tracé dans `historique_transition`
  (entité `UTILISATEUR`, auteur = l'administrateur).
- `SUPPRIME` ne s'obtient que par `DELETE /me` ; un compte supprimé n'est plus modifiable (409).

## Suppression de compte (D13)

- Refusée si un échange est `PROPOSE`, `EN_COURS` ou `EN_LITIGE` (409 `echanges-en-cours`).
- Effacés : pseudo (→ `membre-supprime-xxxxxxxxxxxx`), e-mail, bio, login et identifiant
  GitHub (remplacé par une valeur négative aléatoire), compétences, notifications.
  Offres retirées, sessions révoquées, litiges arbitrés rendus à la file.
- Conservés : grand livre (soldes et écritures intacts, vérifié par le contrôle
  d'intégrité), avis et messages, rattachés au pseudo anonyme.
- **Réinscription** : l'empreinte SHA-256 de l'identifiant GitHub est gardée dans
  `github_supprime` ; un nouveau compte peut être créé mais **sans second bonus de
  bienvenue** (sinon : 50 cr gratuits à chaque suppression). L'empreinte d'un entier
  est réversible par force brute : c'est une donnée pseudonyme, à mentionner dans
  la politique de confidentialité.

## Tests

`app/admin_test.go` (droits, validations, auto-modification, suspension ⇒ sessions
révoquées et litiges désaffectés, recherche, suppression : anonymisation, grand livre
intact, avis conservés, réinscription sans bonus) et `http/admin_api_test.go`
(élévation de privilèges, mass assignment sur `/me`, suspension immédiate, cookie effacé).

## Hors périmètre MVP

Modération des avis et des messages (signalement, masquage), journal d'audit consultable,
export des données personnelles (RGPD art. 20).
