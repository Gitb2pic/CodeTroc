// Données de démonstration reprises de la maquette « CodeTroc App ».
// Elles seront remplacées par les appels à l'API FastAPI (/api/v1).
import type { Criterion, LogEntry, Pack, VaultAccess } from '../domain/types';

export const ME = { name: 'Sami Benali', first: 'Sami', handle: 'samib', ini: 'SB', rep: 4.8 };

/** Solde disponible avant toute demande dans le parcours de démo. */
export const START_BALANCE = 360;
/** Séquestre entrant (échanges où Sami livre). */
export const ESCROW_IN = 70;
export const WELCOME_BONUS = 50;

export const PACKS: Pack[] = [
  { id: 'p1', title: 'Pipeline CI/CD GitHub Actions pour app Node', stack: ['GitHub Actions', 'Node.js', 'Docker'], price: 120, days: 5, ms: 3, crit: 6, name: 'Léa Martin', handle: 'leamartin', ini: 'LM', rep: 4.9, deliv: 38 },
  { id: 'p2', title: 'Dockeriser une app Next.js + Postgres', stack: ['Docker', 'Next.js', 'Postgres'], price: 90, days: 3, ms: 2, crit: 5, name: 'Karim Haddad', handle: 'khaddad', ini: 'KH', rep: 4.8, deliv: 21 },
  { id: 'p3', title: 'Formulaire React accessible (WCAG AA)', stack: ['React', 'TypeScript'], price: 70, days: 4, ms: 2, crit: 7, name: 'Inès Roy', handle: 'inesroy', ini: 'IR', rep: 4.7, deliv: 12 },
  { id: 'p4', title: 'Infra Terraform AWS minimale (VPC + ECS)', stack: ['Terraform', 'AWS'], price: 180, days: 7, ms: 4, crit: 8, name: 'Tom Weber', handle: 'tweber', ini: 'TW', rep: 5.0, deliv: 54 },
  { id: 'p5', title: 'Suite de tests E2E Playwright', stack: ['Playwright', 'TypeScript'], price: 60, days: 3, ms: 2, crit: 5, name: 'Nora Diallo', handle: 'nora-d', ini: 'ND', rep: 4.6, deliv: 9 },
  { id: 'p6', title: 'API REST FastAPI + auth JWT', stack: ['Python', 'FastAPI', 'Postgres'], price: 110, days: 5, ms: 3, crit: 6, name: 'Hugo Lambert', handle: 'hlambert', ini: 'HL', rep: 4.5, deliv: 27 },
  { id: 'p7', title: 'Monitoring Prometheus + Grafana', stack: ['Docker', 'Prometheus'], price: 100, days: 4, ms: 3, crit: 6, name: 'Karim Haddad', handle: 'khaddad', ini: 'KH', rep: 4.8, deliv: 21 },
  { id: 'p8', title: 'Schéma Prisma versionné + migrations', stack: ['Prisma', 'Postgres', 'TypeScript'], price: 50, days: 2, ms: 1, crit: 4, name: 'Inès Roy', handle: 'inesroy', ini: 'IR', rep: 4.7, deliv: 12 },
  { id: 'p9', title: "Revue de sécurité d'une API Express", stack: ['Node.js', 'Sécurité'], price: 80, days: 3, ms: 2, crit: 6, name: 'Tom Weber', handle: 'tweber', ini: 'TW', rep: 5.0, deliv: 54 },
];

export const STACK_FILTERS = ['Toutes', 'Docker', 'GitHub Actions', 'Node.js', 'React', 'TypeScript', 'Postgres', 'Terraform', 'Python'];

export const P1_MILESTONES = ['Workflow CI de base (lint + tests)', 'Build & push image Docker', 'Déploiement staging + doc'];
export const GENERIC_MILESTONES = ['Cadrage & mise en place', 'Livraison principale', 'Finitions & documentation', 'Passation'];

export const P1_CRITERIA: Criterion[] = [
  { t: 'Lint et tests lancés à chaque PR', v: 'CI' },
  { t: 'Matrice Node 18 et 20', v: 'CI' },
  { t: 'Image Docker publiée sur GHCR, taguée par SHA', v: 'CI' },
  { t: 'Déploiement staging automatique après merge sur main', v: 'CI' },
  { t: 'Aucun secret en clair dans le dépôt', v: 'Scan' },
  { t: 'README : comment lancer et modifier le pipeline', v: 'Revue' },
];

export const GENERIC_CRITERIA: Criterion[] = [
  { t: 'Code livré via PR sur ton dépôt', v: 'CI' },
  { t: 'CI au vert sur la branche principale', v: 'CI' },
  { t: 'Aucun secret en clair dans le dépôt', v: 'Scan' },
  { t: 'README de mise en route', v: 'Revue' },
  { t: 'Revue de ta part sous 48 h après livraison', v: 'Revue' },
];

export const RECENT_PROOFS = [
  { pr: 'acme/shop-api #42', t: 'ci: workflow lint + tests', d: 'il y a 3 j', cr: '+40 cr' },
  { pr: 'nora-d/blog #17', t: 'ci: deploy preview Vercel', d: 'il y a 9 j', cr: '+60 cr' },
  { pr: 'khaddad/infra-lab #88', t: 'ci: matrix build arm64', d: 'il y a 2 sem.', cr: '+50 cr' },
];

export const MILESTONE_DUE = ['17 oct.', '19 oct.', '21 oct.', '23 oct.'];

export const VAULT: VaultAccess[] = [
  { id: 'v1', icon: 'server', type: 'Serveur', name: 'ssh deploy@staging-01.acme.dev', to: 'Léa Martin', ini: 'LM', scope: ['utilisateur deploy', 'sans sudo', 'port 22'], left: '2 j 4 h', pct: 62, ex: 'EX-0142', last: 'il y a 12 min', warn: false },
  { id: 'v2', icon: 'folder-git-2', type: 'Dépôt', name: 'github.com/acme/shop-api', to: 'Léa Martin', ini: 'LM', scope: ['write', 'branches feat/*', 'sans force-push'], left: '5 j', pct: 85, ex: 'EX-0142', last: 'il y a 2 h', warn: false },
  { id: 'v3', icon: 'key-round', type: 'Secret', name: 'GHCR_PUSH_TOKEN', to: 'Léa Martin', ini: 'LM', scope: ['packages:write', 'masqué dans les logs'], left: '23 h', pct: 12, ex: 'EX-0142', last: 'jamais utilisé', warn: true },
  { id: 'v4', icon: 'folder-git-2', type: 'Dépôt', name: 'github.com/khaddad/a11y-forms', to: 'Karim Haddad', ini: 'KH', scope: ['read', 'issues'], left: '3 j', pct: 50, ex: 'EX-0138', last: 'il y a 1 j', warn: false },
  { id: 'v5', icon: 'server', type: 'Serveur', name: 'ssh root@prod-db-02', to: 'Hugo Lambert', ini: 'HL', scope: ['lecture seule'], left: '—', pct: 0, ex: 'EX-0131', last: 'il y a 4 j', warn: false },
];

/** Accès déjà révoqués avant l'ouverture de l'écran. */
export const PRE_REVOKED = new Set(['v5']);
/** Accès reçus par Sami (les autres sont partagés par lui). */
export const RECEIVED_ACCESS = new Set(['v4']);

export const VAULT_LOG: LogEntry[] = [
  { time: '14:32:08', who: 'Léa Martin', act: 'Connexion SSH', res: 'staging-01', ip: '82.64.xx.12', tone: 'ok' },
  { time: '14:31:55', who: 'Léa Martin', act: 'git clone', res: 'acme/shop-api', ip: '82.64.xx.12', tone: 'ok' },
  { time: '11:02:13', who: 'Toi', act: 'Accès créé · 72 h', res: 'GHCR_PUSH_TOKEN', ip: '—', tone: 'ac' },
  { time: '10:58:40', who: 'Toi', act: 'Accès créé · 72 h', res: 'staging-01', ip: '—', tone: 'ac' },
  { time: 'hier 21:47', who: 'Inconnu', act: 'Tentative refusée (clé inconnue)', res: 'staging-01', ip: '185.220.xx.4', tone: 'fail' },
  { time: 'ven. 18:20', who: 'Système', act: "Révocation (fin d'échange)", res: 'prod-db-02', ip: '—', tone: 'mute' },
];

export const PROFILE_LEA = {
  name: 'Léa Martin',
  handle: 'leamartin',
  ini: 'LM',
  skills: [
    ['GitHub Actions', 17],
    ['Docker', 12],
    ['Node.js', 9],
    ['Kubernetes', 4],
    ['Terraform', 3],
    ['Bash', 6],
  ] as [string, number][],
  extraPacks: [
    { id: 'x1', title: 'Revue de workflow GitHub Actions existant', stack: ['GitHub Actions'], price: 40, days: 2, ms: 1, crit: 4, name: 'Léa Martin', handle: 'leamartin', ini: 'LM', rep: 4.9, deliv: 38 },
    { id: 'x2', title: 'Cache, matrices et jobs parallèles en CI', stack: ['GitHub Actions', 'Node.js'], price: 60, days: 3, ms: 2, crit: 5, name: 'Léa Martin', handle: 'leamartin', ini: 'LM', rep: 4.9, deliv: 38 },
  ] as Pack[],
  deliveries: [
    { pr: 'acme/shop-api #42', t: 'ci: workflow lint + tests', ex: 'EX-0142', cr: '+40 cr', d: '15 oct.', ci: '3/3' },
    { pr: 'nora-d/blog #17', t: 'ci: deploy preview Vercel', ex: 'EX-0127', cr: '+60 cr', d: '6 oct.', ci: '5/5' },
    { pr: 'khaddad/infra-lab #88', t: 'ci: matrix build arm64 + amd64', ex: 'EX-0119', cr: '+50 cr', d: '29 sept.', ci: '4/4' },
    { pr: 'inesroy/forms #9', t: 'chore: docker compose dev env', ex: 'EX-0102', cr: '+70 cr', d: '12 sept.', ci: '2/2' },
  ],
  reviews: [
    { who: 'Karim Haddad', ini: 'KH', ex: 'EX-0119', stars: '5/5', t: 'Livré en avance, PR découpée proprement, chaque critère couvert par un check. Je redemande.' },
    { who: 'Nora Diallo', ini: 'ND', ex: 'EX-0127', stars: '5/5', t: "La doc du pipeline est claire, j'ai pu modifier les jobs seule dès le lendemain." },
    { who: 'Tom Weber', ini: 'TW', ex: 'EX-0098', stars: '4/5', t: 'Très bon travail. Un aller-retour sur le cache npm, réglé en une heure.' },
  ],
};

export const ONBOARDING_SKILLS: [string, boolean][] = [
  ['Docker', true],
  ['GitHub Actions', true],
  ['Node.js', true],
  ['TypeScript', true],
  ['React', false],
  ['Postgres', true],
  ['Python', false],
  ['Terraform', false],
  ['Kubernetes', false],
  ['Next.js', false],
  ['Playwright', false],
  ['Sécurité', false],
  ['Rust', false],
  ['Go', false],
];

export const ONBOARDING_PACKS = [
  { title: 'Dockeriser une app Node existante', meta: 'Docker · Node.js · 2 jalons · 3 j', price: '60 cr' },
  { title: 'Mettre en place une CI GitHub Actions', meta: 'GitHub Actions · 2 jalons · 2 j', price: '50 cr' },
  { title: 'Typer un projet JS en TypeScript', meta: 'TypeScript · 3 jalons · 4 j', price: '80 cr' },
];
