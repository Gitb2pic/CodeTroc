# F6 — Échanges : demande, acceptation, séquestre

> Statut : terminée · Taille : L · Dépend de : F2, F5 · Écrans : Espace d'échange, Tableau de bord

## Contrat

| Méthode | Chemin | Accès | Idempotency-Key |
|---|---|---|---|
| POST | `/echanges` | Membre (pas sur sa propre offre) | — |
| GET | `/echanges?role&statut&limit&cursor` | Membre : ses échanges | — |
| GET | `/echanges/{id}` | Parties (et arbitre d'un litige, F9) ; 404 pour les autres | — |
| POST | `/echanges/{id}/accepter` | Prestataire | obligatoire |
| POST | `/echanges/{id}/refuser` | Prestataire | obligatoire |
| POST | `/echanges/{id}/annuler` | Parties (règles ci-dessous) | obligatoire |

La réponse expose `a_qui_de_jouer` (DEMANDEUR, PRESTATAIRE, ARBITRE, AUCUN)
pour la carte « Maintenant » de l'UI, et le résumé du séquestre (bloqué,
libéré, remboursé).

## Règles

- RG09 : demandeur ≠ prestataire (contrôle applicatif **et** contrainte SQL).
- RG10 : jalons de l'offre par défaut, ou jalons ajustés dont la somme égale le prix.
- UC06 : aucun crédit bloqué à la demande ; le solde doit cependant couvrir le prix.
- RG11 / D2 : à l'acceptation, création du compte SÉQUESTRE de l'échange et mouvement BLOCAGE (clé `blocage:<échange>`). Solde devenu insuffisant → 422, l'échange reste PROPOSE.
- Échéances (D8) : jalon *i* dû à `ceil(délai × i / n)` jours après l'acceptation.
- RG18 / D8 : annulation
  - PROPOSE : par le demandeur ;
  - EN_COURS : accord des deux parties (la première demande est enregistrée), ou **abandon** : le demandeur annule seul quand le jalon en cours a dépassé son échéance de plus de 48 h ;
  - refusée tant qu'un jalon livré ou contesté attend une décision ;
  - les jalons non soldés passent REMBOURSE et le reste du séquestre revient au demandeur.
- ENF04 : chaque transition d'échange et de jalon est enregistrée dans `historique_transition` (qui, quand, motif).
- D4 : `Idempotency-Key` propre à l'utilisateur ; même requête rejouée → même réponse (`Idempotent-Replayed: true`) ; même clé pour une autre requête → 422 ; requête en cours → 409 ; erreur serveur → clé libérée.

## Tests écrits avant le code

- Domaine : toutes les paires de transitions d'échange et de jalon (autorisées / interdites), `ParseRepoURL` (16 URL hostiles : autre hôte, port, identifiants, requête, fragment, IP de métadonnées, `..`), **fuzzing** (≈ 950 000 entrées en 20 s : toute URL acceptée a une forme canonique stable), échéances, `NextActor`.
- Intégration : demande nominale sans blocage ; jalons incohérents, dépôt invalide, propre offre, solde insuffisant, offre retirée ; acceptation (séquestre, soldes des deux parties, échéances) ; **10 acceptations simultanées → un seul BLOCAGE** ; solde devenu insuffisant ; refus ; les quatre cas d'annulation ; visibilité ; historique.
- API : clé d'idempotence absente, rejouée, réutilisée ailleurs, propre à l'utilisateur ; **8 acceptations HTTP parallèles avec des clés différentes → 1 succès, 7 × 409** ; URL de dépôt hostiles (SSRF) ; affectation de masse (`montant_total`) ; contrat OpenAPI.

## Menaces

| Menace | Parade |
|---|---|
| Double blocage (double clic, rejeu, courses) | `FOR UPDATE` sur l'échange, machine à états, clé de mouvement dérivée, Idempotency-Key |
| BOLA sur un échange | Contrôle de partie à chaque cas d'utilisation ; 404 pour un tiers |
| SSRF via `depot_url` | Seul `https://github.com/<owner>/<repo>` est accepté ; seuls owner et repo sont conservés, aucune requête vers l'URL fournie |
| Prestataire qui disparaît avec un séquestre bloqué | Échéances et abandon unilatéral après délai de grâce |
| Montant ou statut forgé par le client | Montant recopié de l'offre côté serveur ; JSON strict |
