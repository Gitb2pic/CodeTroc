# F7 et F8 — Preuves GitHub, validation, libération et délais

> Statut : terminées · Dépendent de : F6 · Écrans : Espace d'échange, Portefeuille, Notifications

## Contrat

| Méthode | Chemin | Accès |
|---|---|---|
| POST | `/jalons/{id}/preuves` | Prestataire — `pr_url` (pack PR_CI) ou `note` (pack MANUELLE) |
| POST | `/jalons/{id}/reverifier` | Parties — relance la vérification GitHub (UC09 4c) |
| POST | `/jalons/{id}/valider` | Demandeur — `Idempotency-Key` obligatoire |
| POST | `/webhooks/github` | GitHub App — signature `X-Hub-Signature-256` obligatoire |

Le worker (`cmd/worker`) tourne chaque minute sous verrou consultatif
PostgreSQL : validation tacite, réconciliation des preuves en attente,
expiration des secrets (F12), ménage, contrôle du grand livre (alerte en
niveau ERROR s'il est incohérent).

## Règles

- RG12 : seul le jalon EN_COURS se livre ; sa livraison démarre le suivant.
- RG13 / D6, évaluées dans cet ordre :
  1. la PR cible le dépôt de l'échange, comparé par **identifiant GitHub** (résolu à la première vérification) ;
  2. son auteur est le **compte GitHub du prestataire** (par identifiant, pas par login) ;
  3. elle a été **créée après l'acceptation** ;
  4. elle n'a **jamais prouvé un autre jalon** (index unique partiel) ;
  5. elle est mergée (sinon : EN_ATTENTE) ;
  6. le commit livré a **au moins un check**, tous terminés (sinon EN_ATTENTE), aucun en échec, au moins un en succès.
- Le webhook n'est qu'un signal : la décision vient toujours d'un appel authentifié à l'API GitHub. `X-GitHub-Delivery` déjà vu → ignoré.
- GitHub indisponible ou App non installée : la preuve reste EN_ATTENTE avec un message, le worker la revérifie.
- D7 : pack MANUELLE → note obligatoire, jalon LIVRE directement, validation par le demandeur.
- RG14 : validation par le demandeur (LIBERATION séquestre → prestataire, clé `liberation:<jalon>`), ou **tacite après 72 h** sans contestation, attribuée au système. Dernier jalon soldé → échange TERMINE.

## Tests écrits avant le code

- Domaine `proof` : chaque condition de D6 (11 cas), ordre des contrôles, check ignoré toléré ; parseur d'URL de PR (9 URL hostiles) et **fuzzing**.
- Adaptateur GitHub App (serveur simulé) : JWT d'application RS256 vérifié côté serveur, jeton d'installation mis en cache, App non installée → introuvable, lecture PR et checks ; signature HMAC (7 signatures hostiles) et **fuzzing**.
- Intégration : PR valide → LIVRE et jalon suivant démarré ; 5 motifs de rejet ; en attente puis revérification ; GitHub en panne puis réconciliation ; même PR pour deux jalons refusée ; webhook dédupliqué, webhook d'un autre dépôt sans effet, payload illisible ignoré ; livraison manuelle ; validation (accès, état, soldes, profil), dernier jalon → TERMINE, **10 validations simultanées → une libération** ; validation tacite à 71 h (non) et 73 h (oui, une seule fois) ; planificateur à instance unique.
- API : parcours complet avec webhook signé ; webhook non signé, signé avec un autre secret, identifiant de livraison hostile, rejoué ; BOLA sur preuves, revérification et validation ; preuve « VALIDE » forgée par le client refusée ; **6 validations HTTP parallèles → une seule libération**.

## Menaces

| Menace | Parade |
|---|---|
| Webhook forgé pour libérer des crédits | HMAC SHA-256 en temps constant ; décision prise sur l'API GitHub, pas sur le payload |
| Rejeu de webhook | Table `webhook_livraison` (clé `X-GitHub-Delivery`) |
| Preuve frauduleuse (PR d'un autre, ancienne PR, PR réutilisée) | Contrôles D6 par identifiants GitHub, index unique partiel |
| Double libération | Verrous échange → jalon → preuve, machine à états, clé de mouvement dérivée, Idempotency-Key |
| Demandeur qui ne valide jamais | Validation tacite à 72 h par le worker |
| Deux workers en parallèle | `pg_try_advisory_lock` |
