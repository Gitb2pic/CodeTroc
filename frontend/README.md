# CodeTroc — front-end

Application web React (SPA) de CodeTroc, implémentée à partir de la maquette
Claude Design « CodeTroc App » (12 écrans, thèmes sombre et clair).

Stack : React 19, React Router 7, TypeScript, Vite, Vitest + Testing Library.

## Lancer

```bash
npm install
npm run dev        # http://localhost:5173
npm test           # tests unitaires et parcours
npm run build      # typecheck + build de production dans dist/
```

## Écrans

| Route | Écran |
|---|---|
| `/` | 01 Landing |
| `/onboarding` | 02 Onboarding (connexion GitHub, compétences, premier pack, solde) |
| `/tableau-de-bord` | 03 Tableau de bord |
| `/catalogue` | 04 Catalogue (recherche, filtres stack / prix / délai / réputation) |
| `/catalogue/:packId` | 05 Détail du pack |
| `/packs/nouveau` | 06 Création d'un pack (4 étapes, aperçu en direct) |
| `/echanges/EX-0142` | 07 Espace d'échange (jalons, preuve PR + CI, séquestre, discussion) |
| `/coffre` | 08 Coffre d'accès (révocation avec confirmation, journal) |
| `/portefeuille` | 09 Portefeuille |
| `/membres/leamartin` | 10 Profil public |
| `/litiges/EX-0131` | 11 Litige et arbitrage |
| `/notifications` | 12 Notifications |

Les états vide, chargement et erreur des écrans clés (tableau de bord,
catalogue, coffre, portefeuille) s'affichent avec `?etat=vide`,
`?etat=chargement` ou `?etat=erreur`, comme la propriété « etat » de la maquette.

## Parcours de démonstration

L'espace d'échange rejoue le scénario du hackathon : demande du pack,
acceptation, PR #42, CI (3 s simulées), puis validation du jalon 1 et
libération des crédits. Le bandeau « Prototype » simule les actions de
l'autre partie. L'avancement et le thème sont conservés dans le
`localStorage` du navigateur.

## Organisation

```
src/
├── data/fixtures.ts     # données de démo (à remplacer par l'API /api/v1)
├── domain/              # règles pures et testées : répartition des jalons,
│                        # filtres, soldes, statuts
├── store/AppStore.tsx   # état partagé (parcours, coffre, notifications, toasts)
├── components/          # primitives d'UI, mise en page, carte de pack
├── pages/               # un fichier par écran
└── styles/              # jetons de couleur, thèmes, icônes Lucide (masques CSS)
```

## Écarts avec la maquette

- La publication d'un pack est bloquée tant que la somme des jalons ne vaut
  pas le prix (règle RG10 de la documentation technique) ; la maquette
  permettait de publier quand même.
- La description du détail de pack est générique pour les packs autres que
  « Pipeline CI/CD » ; la maquette affichait le texte de ce pack partout.
- Les liens vers les fichiers « Design System » et « Écrans mobiles » de la
  maquette ne sont pas repris : ces écrans ne font pas partie de cette
  implémentation.
- Le front n'est pas encore branché à l'API FastAPI : toutes les données
  viennent de `src/data/fixtures.ts`.
