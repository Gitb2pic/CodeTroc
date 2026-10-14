# F10 — Avis et réputation · F11 — Messagerie d'échange

Statut : **livré**.

## F10 — Avis (UC14, RG20, RG21)

| Endpoint | Accès | Règle |
|---|---|---|
| `POST /echanges/{id}/avis` | parties | échange `TERMINE` (sinon 409 `echange-non-termine`), un avis par auteur et par échange (409 `avis-deja-laisse`) |
| `GET /utilisateurs/{id}/avis` | public | avis reçus, paginés par curseur |

- Le **destinataire est déduit** par le serveur (l'autre partie) : un champ `id_destinataire` envoyé par le client est rejeté (décodage strict → 400).
- Note entière 1..5, commentaire facultatif ≤ 2000 caractères ; contraintes doublées en base (`CHECK`, `UNIQUE (id_echange, id_auteur)`, `CHECK auteur <> destinataire`).
- RG21 : `score_reputation` = moyenne des notes reçues arrondie au centième, recalculée **dans la même transaction** que l'insertion de l'avis. Le profil public expose aussi `avis_recus`.

## F11 — Messages (RG22)

| Endpoint | Accès |
|---|---|
| `GET /echanges/{id}/messages` | parties ; arbitre d'un litige de l'échange (lecture seule) |
| `POST /echanges/{id}/messages` | parties seulement (arbitre → 403 `acces-refuse`, tiers → 404) |

- Contenu 1..5000 caractères (espaces seuls refusés), stocké **tel quel** : l'échappement est la responsabilité du rendu (React échappe par défaut, jamais de `dangerouslySetInnerHTML`). Test : `<script>` revient à l'identique, en JSON.
- Pagination du plus récent au plus ancien, curseur `(date, id)`.

## Tests

- `internal/app/social_test.go` : avis (destinataire, doublon, tiers, bornes de note et de commentaire, échange non terminé), calcul de réputation (table), messages (tiers, vide, trop long, ordre, arbitre lecteur seulement après ouverture du litige).
- `internal/adapters/http/social_api_test.go` : parcours complet par l'API (deux jalons livrés et validés), mass assignment sur `id_destinataire`, contrat OpenAPI vérifié sur chaque réponse.

## Hors périmètre MVP

Modération / signalement d'avis, notifications temps réel (polling côté front), pièces jointes.
