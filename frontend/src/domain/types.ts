export type Tone = 'ok' | 'wait' | 'fail' | 'disp' | 'ac' | 'mute';

export type IconName = string;

export interface Pack {
  id: string;
  title: string;
  stack: string[];
  price: number;
  days: number;
  /** Nombre de jalons. */
  ms: number;
  /** Nombre de critères d'acceptation. */
  crit: number;
  name: string;
  handle: string;
  ini: string;
  rep: number;
  deliv: number;
}

export type CriterionKind = 'CI' | 'Revue' | 'Scan';

export interface Criterion {
  t: string;
  v: CriterionKind;
}

export interface Milestone {
  n: number;
  t: string;
  cr: number;
}

/**
 * Avancement du parcours de démonstration d'un échange :
 * 0 aucun échange, 1 demandé, 2 accepté, 3 CI en cours,
 * 4 jalon 1 à valider, 5 jalon 1 validé.
 */
export type Flow = 0 | 1 | 2 | 3 | 4 | 5;

export type Etat = 'normal' | 'vide' | 'chargement' | 'erreur';

export interface VaultAccess {
  id: string;
  icon: IconName;
  type: string;
  name: string;
  to: string;
  ini: string;
  scope: string[];
  left: string;
  pct: number;
  ex: string;
  last: string;
  warn: boolean;
}

export interface LogEntry {
  time: string;
  who: string;
  act: string;
  res: string;
  ip: string;
  tone: Tone;
}
