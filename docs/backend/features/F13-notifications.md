# F13 — Notifications

Statut : **livré**.

## Principe

Une notification est **écrite dans la même transaction** que l'action qui la cause :
si l'action échoue, aucune notification n'existe ; si elle réussit, la notification
existe forcément (outbox locale, sans courtier de messages au MVP). L'auteur d'une
action n'est jamais notifié de sa propre action. Les identifiants sont des UUIDv7
(ordonnés dans le temps) pour départager les notifications d'une même transaction.

| Type | Destinataire | Émis par |
|---|---|---|
| ECHANGE_DEMANDE | prestataire | demande d'échange |
| ECHANGE_ACCEPTE / ECHANGE_REFUSE | demandeur | réponse du prestataire |
| ANNULATION_DEMANDEE | l'autre partie | première demande d'annulation |
| ECHANGE_ANNULE / ECHANGE_TERMINE | les parties | clôture |
| JALON_LIVRE | demandeur | preuve validée |
| JALON_LIBERE | prestataire | validation manuelle ou tacite (pas après litige) |
| PREUVE_REJETEE | prestataire | vérification GitHub négative |
| LITIGE_OUVERT | autre partie + arbitre | ouverture |
| LITIGE_AFFECTE / LITIGE_REPONSE | arbitre | affectation, réponse |
| LITIGE_DECIDE | les parties | décision |
| MESSAGE_RECU | l'autre partie | message |
| SECRET_PARTAGE / SECRET_REVELE | prestataire / déposant | coffre |
| AVIS_RECU | destinataire de l'avis | avis |

Les `donnees` portent le contexte (titre du pack ajouté automatiquement, jalon,
montant, id de litige ou de secret) ; **jamais de valeur secrète ni d'e-mail** (testé).
Le front compose le texte à partir du type.

## API

- `GET /notifications?non_lues=true` (paginé), `GET /notifications/compteur` → `{non_lues}`.
- `POST /notifications/lire` `{ids?: [...]}` : les ids donnés (100 max) ou toutes ;
  la requête est filtrée sur le membre connecté, marquer celles d'un autre ne fait rien.
- Le worker purge les notifications lues de plus de 90 jours.

## Tests

`app/notifications_test.go` (cycle complet d'un échange pour chaque partie, litige,
messages, coffre, absence de secret, états lu/non lu, cloisonnement) et
`http/notifications_api_test.go` (compteur, filtre, marquage, cloisonnement, contrat).

## Hors périmètre MVP

Temps réel (SSE/WebSocket : le front interroge le compteur), e-mails, préférences.
