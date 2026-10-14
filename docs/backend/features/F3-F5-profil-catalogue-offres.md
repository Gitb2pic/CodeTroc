# F3 à F5 — Profil, catalogue, offres

> Statut : terminées · Dépendent de : F1 · Écrans : Onboarding, Profil, Catalogue, Détail du pack, Création d'un pack

## F3 — Profil et compétences (S)

| Méthode | Chemin | Accès |
|---|---|---|
| PATCH | `/me` | Membre — pseudo (`^[A-Za-z0-9][A-Za-z0-9_-]{2,49}$`, unique) et bio (≤ 1000, vide = effacée) |
| PUT | `/me/competences` | Membre — remplacement atomique, 30 max, niveaux DEBUTANT → EXPERT (RG06) |
| GET | `/competences` | Public — référentiel (20 compétences de départ) |
| GET | `/utilisateurs/{id}` | Public — profil sans e-mail, compétences, livraisons vérifiées, avis reçus |

## F4 — Catalogue de packs (M)

| Méthode | Chemin | Accès |
|---|---|---|
| GET | `/packs?competence&q&limit&cursor` | Public — packs actifs |
| GET | `/packs/{id}` | Public ; un pack inactif n'est visible que des admins |
| POST / PATCH | `/packs`, `/packs/{id}` | Admin (UC16) |

Règles : RG07 (au moins une compétence, `0 < prix_min ≤ prix_max`), critères
typés CI / SCAN / REVUE comme dans l'UI, `type_preuve` PR_CI ou MANUELLE (D7).
Huit packs de départ (dossier projet), prix à l'échelle de l'interface.

## F5 — Offres (S)

| Méthode | Chemin | Accès |
|---|---|---|
| GET | `/offres?pack&prestataire&competence&q&prix_max&delai_max&reputation_min` | Public — filtres de l'écran Catalogue |
| GET | `/offres/{id}` | Public ; une offre retirée n'est visible que de son auteur |
| POST | `/offres` | Membre (UC04) |
| POST | `/offres/{id}/retirer` | Auteur (403 pour un autre membre) |

Règles : RG08 (prix dans la fourchette du pack), délai 1 à 90 jours, jalons
proposés par le prestataire (1 à 10, montants > 0, somme = prix — RG10),
repris par défaut à la demande d'échange.

## Tests écrits avant le code

- Domaine `catalog` : validation des packs (13 cas), des jalons (somme, bornes, titres), des offres (fourchette, délai), répartition par défaut.
- Intégration : pseudo pris → 409, pseudos hostiles refusés ; compétences remplacées d'un bloc (rien ne change si une compétence est inconnue) ; packs réservés aux admins, pack inactif invisible ; filtres combinés et pagination ; recherche insensible aux jokers SQL (`%`) ; offre hors fourchette ou jalons incohérents refusés ; retrait réservé à l'auteur et idempotent.
- API : affectation de masse refusée (`role` dans `PATCH /me`, `id_prestataire` dans `POST /offres`) ; plusieurs objets JSON refusés ; profil public sans e-mail ; jeton invalide refusé même sur une route publique ; identifiant mal formé → 404 ; toutes les réponses validées contre `openapi.yaml`.

## Menaces

| Menace | Parade |
|---|---|
| Affectation de masse (rôle, statut, prestataire) | Décodage JSON strict : tout champ inconnu → 400 |
| Élévation (créer un pack, modifier le catalogue) | Rôle ADMIN relu en base à chaque requête |
| Énumération des offres retirées | 404 pour tout autre que l'auteur (D11) |
| Injection dans la recherche | Requêtes paramétrées, `position()` au lieu de `LIKE` |
| XSS stocké (bio, titres) | Longueurs bornées ; échappement à l'affichage par React |
