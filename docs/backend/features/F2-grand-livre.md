# F2 — Grand livre et comptes

> Statut : terminée · Taille : L · Dépend de : F1 · Écrans : Portefeuille, barre de solde

## Objectif

Aucun crédit ne se crée ni ne se perd hors des règles : chaque mouvement est
une écriture en partie double, en ajout seul, protégée par le domaine **et**
par la base.

## Règles

- RG01 : un compte UTILISATEUR par membre, créé à l'inscription.
- RG02 / D1 : bonus de bienvenue de 50 cr débité du compte ÉMISSION, versé une seule fois.
- RG03 : un compte UTILISATEUR ou SÉQUESTRE n'est jamais négatif (domaine + contrainte `ck_solde_positif`).
- RG04 : montant strictement positif, débit ≠ crédit, somme des soldes = 0.
- D2 : un compte SÉQUESTRE par échange (créé avec F6).
- D3 : chaque mouvement porte `id_echange` / `id_jalon` quand il en a un.
- D4 : clé d'idempotence dérivée par le serveur (`bonus:<user>`, `blocage:<échange>`…) ; une clé rejouée ne produit aucun effet.
- D10 : partage arrondi à l'inférieur pour le prestataire, reste au demandeur.

## Contrat

| Méthode | Chemin | Rôle |
|---|---|---|
| GET | `/me/compte` | Disponible, séquestre sortant, séquestre entrant |
| GET | `/me/compte/mouvements?limit&cursor` | Historique paginé (curseur stable date + id) |

## Tests écrits avant le code

- Domaine : transfert nominal ; montant nul ou négatif, solde insuffisant, même compte, clé vide → refusés sans toucher aux soldes ; ÉMISSION peut être négatif ; `Split` sur les cas limites.
- **Propriétés (rapid)** : sur des suites aléatoires de transferts, somme nulle et aucun solde UTILISATEUR / SÉQUESTRE négatif ; `Split` conserve le montant.
- Intégration : bonus versé une fois, y compris sur 6 premières connexions simultanées ; clé rejouée sans effet ; **100 débits concurrents de 1 cr sur 50 cr → exactement 50 réussites** ; transferts croisés sans interblocage ; `UPDATE`/`DELETE` sur `mouvement` refusés par trigger ; solde négatif refusé par la base ; pagination et curseur forgé.
- Contrôle d'intégrité (`LedgerIntegrity`) exécuté à la fin des tests qui déplacent des crédits.

## Menaces

| Menace | Parade |
|---|---|
| Course sur le solde (double dépense) | `SELECT … FOR UPDATE` dans un ordre fixe, contrôle de solde dans le domaine et en base |
| Rejeu d'une opération financière | Clés d'idempotence dérivées par le serveur, contrainte `UNIQUE` |
| Falsification de l'historique | Trigger d'ajout seul sur `mouvement` |
| Double bonus par connexions parallèles | Création d'utilisateur `ON CONFLICT`, clé `bonus:<user>` |
