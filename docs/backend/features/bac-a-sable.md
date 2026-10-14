# Bac à sable navigateur

Le **vrai backend** (mêmes cas d'utilisation, même routeur HTTP, mêmes migrations)
tourne dans le navigateur, compilé en WebAssembly, sur **PGlite** (PostgreSQL 18 en
WebAssembly). Rien n'est envoyé à un serveur : la base vit dans l'IndexedDB du visiteur.

```
front React (mode API) ──fetch /api/v1──▶ intercepté (src/sandbox/runtime.ts)
        ▼
codetrocHandle() ──▶ routeur Gin (backend/cmd/sandbox, GOOS=js GOARCH=wasm)
        ▼ pgx, connexion « net.Conn » vers JavaScript (pgbridge.go)
codetrocPg() ──▶ PGlite.execProtocolRawStream (une session, appels en file)
```

- **Construire** : `cd frontend && npm run build:sandbox` → `dist-sandbox/`.
- **Simulé** : GitHub (OAuth et API, `internal/devfake`, partagé avec le serveur e2e) :
  la connexion propose des personas ; toute PR sur le dépôt d'un échange en cours est
  « mergée, CI au vert » et attribuée à son prestataire.
- **Données de départ** : six personas (demandeur, trois prestataires, arbitre,
  administratrice), six offres, un échange en cours entre Sami et Léa ; bonus de
  bienvenue porté à 300 cr pour jouer plusieurs échanges.
- **Adaptations au bac à sable** : pool d'une connexion (PGlite n'a qu'une session),
  pas d'instructions préparées nommées, démarrage et fin de connexion absorbés par le
  pont ; cookies gérés par le runtime (session de l'onglet) ; les binaires sont
  publiés en `.wasm`, seul type binaire accepté par l'hébergement des Artifacts.
- **Limites** : un seul visiteur par base (pas de partage entre navigateurs) ;
  premier démarrage ≈ 8 s ; « Réinitialiser » efface la base locale.
