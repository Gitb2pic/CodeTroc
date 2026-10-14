# F12 — Coffre d'accès

Statut : **livré**. Règle : RG19.

## Parcours

| Endpoint | Accès | Effet |
|---|---|---|
| `POST /echanges/{id}/secrets` | demandeur, échange `EN_COURS`/`EN_LITIGE` | chiffre et dépose (libellé, type, portée, valeur ≤ 8 Kio, expiration ≤ 30 j) |
| `GET /echanges/{id}/secrets` | parties | **métadonnées seulement** (statut `ACTIF`/`EXPIRE`/`REVOQUE`) |
| `POST /secrets/{id}/reveler` | prestataire | renvoie la valeur, `Cache-Control: no-store`, accès journalisé |
| `POST /secrets/{id}/revoquer` | déposant | révoque et **efface** la matière chiffrée (idempotent) |
| `GET /secrets/{id}/journal` | parties | CREATION, REVELATION, REFUS, REVOCATION, EXPIRATION, avec utilisateur et IP |

Codes : tiers → 404 (D11) ; partie sans le droit → 403 `acces-refuse` (journalisé `REFUS`) ;
secret révoqué, expiré ou échange clos → 410 `secret-indisponible` (journalisé `REFUS`) ;
coffre non configuré → 503 `coffre-non-configure`.

## Chiffrement (`internal/adapters/vault`)

- **Enveloppe AES-256-GCM** : une DEK aléatoire de 32 octets par secret chiffre la valeur ;
  la KEK (`VAULT_KEK`, 32 octets base64, obligatoire en production, hors base) chiffre la DEK.
  Nonces aléatoires de 96 bits ; la DEK en clair est effacée de la mémoire après usage.
- **AAD = id de l'échange ‖ id du secret**, appliquée aux deux couches : un chiffré
  copié vers un autre échange ou un autre secret ne se déchiffre pas.
- **Rotation** : `kek_version` est stockée ; `vault.New` accepte plusieurs KEK (lecture)
  et une version courante (écriture). L'exposition de plusieurs KEK par la configuration
  viendra avec la première rotation réelle.

## Cycle de vie

- Révocation manuelle par le déposant ; **clôture de l'échange** (terminé ou annulé)
  ⇒ révocation de tous ses secrets dans la même transaction (`onExchangeClosed`) ;
  **expiration** traitée par le worker (`ExpireSecrets`, toutes les minutes).
- Contrainte `ck_secret_matiere` : un secret actif a toute sa matière chiffrée, un secret
  révoqué n'en a plus — impossible de « révoquer » en gardant le chiffré.
- `secret_journal` est en ajout seul (trigger).

## Ce qui protège la valeur

- Jamais renvoyée ailleurs que par `reveler`, jamais journalisée (le logger d'accès
  n'enregistre pas les corps), jamais stockée par le middleware d'idempotence
  (non appliqué à `reveler`, qui n'en a pas besoin : lecture sans effet monétaire).
- Le décodage strict refuse les champs inconnus ; la valeur est `writeOnly` dans le contrat.

## Tests

- `adapters/vault/envelope_test.go` : aller-retour, DEK et nonce neufs à chaque secret,
  altération du chiffré, de la DEK, du nonce, de la version ou du contexte ⇒ échec,
  rotation (ancienne KEK relisible), taille de clé.
- `app/vault_test.go` : droits (demandeur dépose, prestataire révèle, déposant révoque,
  tiers 404), validations, journal complet et ordonné, matière effacée, expiration
  avant et après passage du worker, révocation à l'annulation, coffre absent.
- `http/vault_api_test.go` : parcours complet avec le vrai chiffreur ; la valeur témoin
  n'apparaît **ni dans les logs capturés, ni dans la liste, ni en clair en base**
  (secrets et clés d'idempotence) ; `no-store` sur la révélation ; contrat OpenAPI.

## Pentest (à rejouer en fin de jalon C)

Lecture par un tiers ou par le déposant, rejeu de `reveler` après révocation ou clôture,
fuite dans les logs, les erreurs et les caches intermédiaires, déplacement d'un chiffré
entre lignes en base (AAD).
