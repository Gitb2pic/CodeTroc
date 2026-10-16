# F16 — Gestion de projet agile Scrum

> Statut : **implémentée (backend)**, D14 à D21 validées le 16 oct. 2026 · Taille : L · Dépend de : F6, F7, F8, F11, F13, F17 · Écrans : Espace d'échange (onglets Tableau Scrum, Gantt, Rétro), Tableau de bord

## Objectif

Donner à chaque échange accepté un cadre Scrum prêt à l'emploi : jalons
découpés en sprints, backlog, tableau, Gantt, et une synchronisation avec le
dépôt GitHub de l'échange (une issue étiquetée générée automatiquement au
démarrage de chaque sprint). Le demandeur pilote le « quoi » (Product Owner),
le prestataire le « comment » (équipe de dev).

## Décisions

| # | Sujet | Décision |
|---|---|---|
| D14 | Jalon ↔ sprints | Un jalon contient 1 à n sprints ; un sprint ne déborde jamais sur le jalon suivant ; la fin du dernier sprint d'un jalon est son échéance. Séquestre, preuve et libération restent par jalon. |
| D15 | Génération des issues | L'issue d'un sprint est générée par le worker au démarrage du sprint. Labels et milestones (un par jalon) sont mis en file à l'acceptation. |
| D16 | Annonce | Voir F17. |
| D17 | Démarrage des sprints | Automatique, à la date prévue ; le sprint 1 démarre dans la transaction d'acceptation. |
| D18 | Permissions GitHub | `issues:write` sur la GitHub App ; jeton demandé pour le seul dépôt de l'échange. Sans la permission : Scrum fonctionne en local, synchro « PERMISSION_MANQUANTE ». Jamais de `contents:write`. |
| D19 | Appels GitHub | Jamais dans la transaction métier : table `outbox_github` remplie dans la transaction, vidée par le worker (reprises, backoff, clés uniques). |
| D20 | Durée des sprints | 7 jours par défaut, 14 possible, choisie dans l'offre ou l'annonce, figée dans l'échange. |
| D21 | Avenant de planning | Proposition d'une partie, acceptation de l'autre ; les sprints à venir sont redécoupés ; les montants ne changent jamais. |
| Q1 | Durée libre ? | Non : 7 ou 14 jours. |
| Q2 | Jalon livré en avance | Le premier sprint du jalon suivant démarre tout de suite (`debut_reel` avancé) ; les échéances contractuelles ne bougent pas. |

## Découpage d'un jalon en sprints (R-S3)

Fenêtre du jalon *i* : `[échéance(i-1), échéance(i)]` (le jalon 1 commence à
l'acceptation). Durée `d` (D20). `L` = longueur en jours :

- `L ≤ d` : un seul sprint ;
- sinon `k = ⌊L/d⌋` sprints de `d` jours ; reste `r` : `r < d/2` absorbé par le dernier sprint, `r ≥ d/2` sprint court supplémentaire ;
- numérotation globale 1..N ;
- une fenêtre vide (délai inférieur au nombre de jalons) ne reçoit pas de sprint.

Exemple : 30 jours, 2 jalons, sprints de 7 jours → J0→J7, J7→J15 | J15→J22, J22→J30.

Propriétés vérifiées par test de propriétés (rapid, 1 à 10 jalons, 1 à 90 jours, d ∈ {7, 14}) : sprints contigus, sans chevauchement, dans leur jalon, dernier = échéance, numérotation sans trou, aucun sprint < d/2 sauf jalon plus court.

## Modèle de données (migration `0016_scrum`)

`projet_scrum` (1-1 avec l'échange, template, objectif produit, DoD copiée, statut de synchro et détail), `sprint` (numéro global, dates prévues et réelles, statut, objectif, issue, bilan figé `points_prevus`/`points_faits`, rétro), `story` (backlog si `id_sprint` NULL, contrainte « backlog ⇔ sans sprint »), `avenant` (un seul ouvert par projet), `outbox_github` (opérations en liste fermée), `jalon.github_milestone_no`, `offre`/`echange.duree_sprint_jours`. Index unique « un seul sprint ACTIF par projet ».

## Cycle de vie (worker et transitions)

`advanceSprints` est l'unique moteur, appelé par le planificateur et dans la transaction de chaque transition de jalon ou d'échange, sous verrou de ligne du projet :

- échange clos : sprints à venir annulés, sprint actif terminé (échange TERMINE) ou annulé ; issues ouvertes fermées ;
- sprint actif à sa date de fin, ou dont le jalon est soldé : clos, bilan figé, stories non terminées renvoyées au backlog (R-S8), issue fermée avec le bilan ; jalon remboursé → sprint annulé ;
- sprints planifiés d'un jalon soldé : annulés ;
- sans sprint actif : le premier sprint planifié d'un jalon EN_COURS démarre à sa date, ou tout de suite si son jalon vient de démarrer en avance (Q2). Les sprints dont la fenêtre est passée pendant l'attente d'un jalon en retard sont annulés, sauf le dernier du jalon : un jalon entièrement en retard n'a pas de sprint actif, le Gantt montre le retard.

Notifications F13 : `SPRINT_TERMINE` à la fin prévue d'un sprint (rétro et planning), `JALON_A_RISQUE` au passage en risque ÉLEVÉ, `JALON_EN_RETARD` à l'échéance dépassée (une seule fois par jalon), `AVENANT_PROPOSE`, `AVENANT_DECIDE`.

## Règles

- **R-S1** : projet, sprints, sprint 1 ACTIF et outbox (labels, un milestone par jalon, issue du sprint 1) dans la transaction d'acceptation (F6 et F17). Un échange refusé n'a pas de projet. Les échanges acceptés avant F16 n'en ont pas non plus (404 `projet-scrum-absent`).
- **R-S2** : dépôt identifié par `github_repo_id`, résolu une fois ; appels sur `/repositories/{id}`.
- **R-S4** : risque MOYEN au-delà de 60 % du temps avec moins de 40 % des points, ÉLEVÉ au-delà de 85 % avec moins de 70 % ; sans point planifié, pas de risque.
- **R-S6** : demandeur = PO (crée les stories, contenu, priorités, sprint, accepte en « Terminé ») ; prestataire = équipe (statut, points) ; tout champ de l'autre rôle → 403 `champ-reserve`. Arbitre d'un litige : lecture seule.
- **R-S7** : à la libération d'un jalon, les issues encore ouvertes de ses stories sont fermées.
- **R-S9** : aucun mouvement du grand livre dans le module.
- **R-S10** : avenant — une échéance par jalon, croissantes, après l'acceptation, ≤ 90 jours au total, futures pour les jalons non livrés, inchangées pour les jalons livrés ou soldés, jalon du sprint actif pas avant sa fin. À l'acceptation : sprints planifiés supprimés et redécoupés, leurs stories retournent au backlog, sprint actif conservé.
- **R-S11** : sprints, stories et avenants dans `historique_transition` (`par = NULL` pour le worker).

## Synchronisation GitHub

- **File** : `outbox_github`, traitée dans l'ordre par échange (un appel en reprise bloque les suivants du même échange) ; backoff exponentiel plafonné à 1 h ; `Retry-After` respecté ; 403 → `PERMISSION_MANQUANTE` ; erreurs reflétées dans le bandeau.
- **Opérations** : `LABELS`, `MILESTONE` (création ou mise à jour de l'échéance), `ISSUE_SPRINT`, `ISSUE_STORY`, `MAJ_ISSUE`, `FERMER_ISSUE`. Toutes idempotentes (une issue déjà créée n'est pas recréée).
- **Issues** : titre `[CodeTroc] Sprint n — Jalon i : <titre>`, labels `codetroc`, `type:sprint`, `sprint:n`, `jalon:i`, milestone du jalon, assignée au prestataire ; corps : objectif, période, jalon, stories (checklist), DoD, lien. Sans story : checklist « Sprint Planning ». Une story affectée à un sprint devient une issue `type:story`.
- **Retour** (webhook `issues`, HMAC et dédoublonnage F7) : `closed` → TERMINE, `reopened` → EN_COURS, label `statut:en-revue` → EN_REVUE, `deleted` → recréée au passage suivant avec une note. Événement d'un autre `repository.id` ignoré. `installation` `new_permissions_accepted` → reprise immédiate des projets bloqués.
- **Échappement** : Markdown échappé, HTML neutralisé, mentions `@` neutralisées, longueurs bornées.

## Contrat

| Méthode | Chemin | Accès |
|---|---|---|
| GET / PATCH | `/echanges/{id}/scrum` | Parties (+ arbitre en lecture) / PO |
| GET | `/echanges/{id}/scrum/planning` | Parties |
| GET | `/echanges/{id}/scrum/sprints` | Parties |
| PATCH | `/echanges/{id}/scrum/sprints/{numero}` | Parties (objectif) |
| PUT | `/echanges/{id}/scrum/sprints/{numero}/retro` | Parties, sprint TERMINE |
| GET / POST | `/echanges/{id}/scrum/stories` | Parties / PO (`Idempotency-Key`) |
| PATCH | `/echanges/{id}/scrum/stories/{sid}` | PO : contenu, priorité, sprint (`null` = backlog) · Équipe : statut, points |
| POST | `/echanges/{id}/scrum/avenants` | Parties (`Idempotency-Key`) |
| POST | `/echanges/{id}/scrum/avenants/{aid}/accepter` | L'autre partie (`Idempotency-Key`) |
| POST | `/echanges/{id}/scrum/avenants/{aid}/refuser` | L'autre partie (refus) ou l'auteur (retrait) |
| POST | `/echanges/{id}/scrum/synchro` | Parties (`Idempotency-Key`) |

Aucun endpoint ne démarre un sprint : c'est le worker (D17).

## Tests

Domaine : découpage (cas et propriétés), redécoupage, machines à états, risque aux bornes, rendu et échappement des issues, labels. Intégration : création à l'acceptation, synchro GitHub complète, cycle du worker (fin, backlog, rétro, retard), jalon livré en avance, annulation, workers concurrents (un seul démarrage, une seule ligne d'outbox), rôles et BOLA, webhooks (autre dépôt, rejeu, suppression), échecs GitHub (permission, quota, 5xx, sans doublon), avenant, durée de sprint d'une annonce. Adaptateur GitHub : jeton limité au dépôt, labels existants, milestone réutilisé, issue supprimée, quota. API : contrat OpenAPI de toutes les routes, affectation de masse, rôles ; toute route montée doit figurer dans `openapi.yaml`.

## Menaces

| Menace | Parade |
|---|---|
| Écriture dans un autre dépôt | Jeton limité à `repository_ids: [github_repo_id]` ; appels par identifiant |
| Élévation via `issues:write` | Permission minimale ; opérations en liste fermée ; aucun relais d'appel GitHub arbitraire |
| Injection Markdown, spam de mentions | Échappement, `@` neutralisés, longueurs bornées, pas de HTML |
| Webhook forgé ou rejoué | HMAC + `webhook_livraison` |
| Quota GitHub | File séquentielle, backoff plafonné, `Retry-After` |
| Démarrage en double | Verrou du projet, index unique « un sprint ACTIF », clés d'outbox uniques |
| Planning manipulé pour échapper à l'abandon | Échéances figées, avenant à deux signatures, abandon calculé sur `jalon.echeance` |
| Confusion GitHub / argent | R-S9 |
