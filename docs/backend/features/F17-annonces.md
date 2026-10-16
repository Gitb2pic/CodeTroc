# F17 — Annonces : demandes d'aide

> Statut : **implémentée (backend)** · Taille : M · Dépend de : F4, F6 · Prérequis de F16 pour le flux « annonce » · Écrans : Catalogue (onglet Annonces), Création d'annonce

Seul le prestataire publiait (offre) et le demandeur le sollicitait.
L'annonce inverse le sens : **une personne qui a besoin d'aide** publie sa
demande, des prestataires candidatent, elle choisit l'un d'eux, ce qui crée un
**échange** (puis son projet Scrum, F16).

## Modèle (migration `0015_annonces`)

- **`annonce`** : titre, besoin, dépôt (`https://github.com/<owner>/<repo>`, même parseur anti-SSRF que F6), pack (fixe le type de preuve et la fourchette de prix), budget, délai (1 à 90 j), jalons souhaités (somme = budget, RG10), durée de sprint (7 ou 14 j, D20), expiration à 30 jours. Statuts : OUVERTE, POURVUE, RETIREE, EXPIREE.
- **`candidature`** : prix (dans la fourchette du pack), délai, jalons (somme = prix), message (≤ 2000). Statuts : EN_ATTENTE, ACCEPTEE, REFUSEE, RETIREE. Une seule candidature en attente par prestataire et par annonce (index unique partiel).
- **`echange`** : `id_offre` devient facultatif, `id_annonce` apparaît, `CHECK` « exactement l'un des deux » ; `id_pack` est recopié dans l'échange (il porte le type de preuve quelle que soit l'origine). Index unique : une annonce produit au plus un échange.

## Règles

- **Publication** : budget couvert par le solde disponible (comme UC06) ; rien n'est bloqué avant l'acceptation.
- **Candidature** : annonce OUVERTE ; pas sur sa propre annonce (RG09, 422 `propre-annonce`) ; 20 candidatures au plus par prestataire et par 24 h (422 `limite-candidatures`) ; doublon en attente → 409 `candidature-existante`. L'auteur est notifié (`CANDIDATURE_RECUE`).
- **Acceptation par l'auteur** (`Idempotency-Key` obligatoire) → une seule transaction : échange créé **directement EN_COURS** (jalons et délai de la candidature, séquestre bloqué D2/D4, échéances D8, premier jalon démarré), candidature ACCEPTEE, autres candidatures REFUSEE, annonce POURVUE. Notifications `CANDIDATURE_ACCEPTEE` au retenu, `CANDIDATURE_REFUSEE` aux autres. Solde devenu insuffisant → 422, rien ne change, l'annonce reste OUVERTE.
- **Concurrence** : l'annonce est verrouillée (`FOR UPDATE` à l'acceptation, `FOR SHARE` à la candidature) ; deux acceptations simultanées → un seul échange, un seul BLOCAGE, l'autre reçoit 409.
- **Retrait** : l'auteur retire son annonce ouverte (candidatures en attente refusées) ; un candidat retire sa candidature en attente.
- **Expiration** : le planificateur passe EXPIREE les annonces ouvertes arrivées à échéance et refuse leurs candidatures en attente.
- **Visibilité** : annonce ouverte publique ; fermée, visible de son auteur et de ses candidats seulement (404 sinon, D11). L'auteur voit toutes les candidatures, un prestataire la sienne seulement.
- **Historique** : chaque transition d'annonce et de candidature est écrite dans `historique_transition` (ENF04).

## Contrat

| Méthode | Chemin | Accès |
|---|---|---|
| GET | `/annonces?pack&q&mes` | Public (`mes=true` : connecté) |
| POST | `/annonces` | Connecté |
| GET | `/annonces/{id}` | Public si ouverte |
| POST | `/annonces/{id}/retirer` | Auteur |
| GET | `/annonces/{id}/candidatures` | Auteur (toutes), candidat (la sienne) |
| POST | `/annonces/{id}/candidatures` | Prestataire |
| POST | `/annonces/{id}/candidatures/{cid}/accepter` | Auteur, `Idempotency-Key` |
| POST | `/annonces/{id}/candidatures/{cid}/refuser` | Auteur |
| POST | `/annonces/{id}/candidatures/{cid}/retirer` | Candidat |

L'échange expose `id_annonce` ; `offre.id` vaut `null` pour un échange né d'une annonce (`offre.titre` reprend alors le titre de l'annonce).

## Tests

Domaine (validation annonce et candidature), intégration (acceptation complète, solde insuffisant, deux acceptations concurrentes, RG09, doublon, limite quotidienne, refus, retrait, expiration idempotente, visibilité), API (contrat OpenAPI, BOLA, affectation de masse, clé d'idempotence).

## Menaces

| Menace | Parade |
|---|---|
| Spam de candidatures | 20 par prestataire et par 24 h, une seule en attente par annonce |
| BOLA sur les candidatures | Seul l'auteur voit toutes les candidatures et décide ; tiers → 404 |
| SSRF via l'URL du dépôt | Parseur F6 : seuls owner et repo sont conservés, aucun appel sortant |
| Double acceptation | Verrou de ligne sur l'annonce + index unique `ux_echange_annonce` + clé de blocage idempotente |
