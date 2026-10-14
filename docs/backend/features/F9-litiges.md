# F9 — Litiges et arbitrage

> Statut : terminée · Taille : M · Dépend de : F8 · Écran : Litige et arbitrage

## Contrat

| Méthode | Chemin | Accès |
|---|---|---|
| POST | `/jalons/{id}/litiges` | Parties — jalon LIVRE seulement |
| GET | `/litiges` | Ses litiges (partie ou arbitre) ; admin : aussi la file sans arbitre |
| GET | `/litiges/{id}` | Parties, arbitre affecté, admin si sans arbitre |
| POST | `/litiges/{id}/reponse` | Partie qui n'a pas ouvert le litige, une fois |
| POST | `/litiges/{id}/affecter` | Admin |
| POST | `/litiges/{id}/decision` | Arbitre affecté — `Idempotency-Key` obligatoire |

## Règles

- RG15 : litige sur un jalon LIVRE ; le jalon passe CONTESTE (la validation tacite est gelée), l'échange EN_LITIGE ; un seul litige ouvert par jalon (index unique partiel).
- RG16 / D9 : **un seul arbitre** au MVP, choisi parmi les ARBITRE et ADMIN actifs non parties, le moins chargé d'abord ; sans arbitre, le litige va dans la file de l'administrateur qui l'affecte (UC12 3a).
- RG17 / D10 : LIBERER (tout au prestataire), REMBOURSER (tout au demandeur), PARTAGER (part du prestataire 1–99 %, arrondie à l'inférieur, reste au demandeur) ; justification obligatoire.
- RG22 : l'arbitre affecté lit l'échange (et ses messages, F11).
- Après décision : plus de jalon contesté → l'échange reprend EN_COURS ; tous les jalons soldés → TERMINE.

## Tests écrits avant le code

- Intégration : ouverture (accès, motif trop court, jalon non livré, double ouverture) ; arbitre jamais partie ; file admin et affectation (arbitre partie refusé, non-admin refusé) ; réponse unique de l'autre partie ; **les 4 décisions** avec mouvements, statuts, séquestre et contrôle du grand livre (dont 1 % de 40 cr → 0 au prestataire) ; décision rejouée refusée ; validations de la décision ; seul l'arbitre affecté tranche ; validation tacite gelée pendant 100 jours de litige.
- API : parcours complet ; BOLA (tiers) ; une partie ne tranche pas ; un membre ne s'affecte pas un litige ; contrat OpenAPI.

## Menaces

| Menace | Parade |
|---|---|
| Une partie se fait arbitre | Exclusion des parties à l'affectation et à la décision |
| Élévation vers le rôle d'arbitre | Rôle relu en base ; affectation réservée aux admins |
| Libération pendant le litige | Jalon CONTESTE : ni validation, ni validation tacite |
| Double exécution de la décision | Litige verrouillé, statut CLOS, clés de mouvement par jalon, Idempotency-Key |
