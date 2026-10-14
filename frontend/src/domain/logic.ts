// Règles d'affichage pures, testables sans DOM.
import { ESCROW_IN, GENERIC_MILESTONES, P1_MILESTONES, START_BALANCE } from '../data/fixtures';
import type { Flow, IconName, Milestone, Pack, Tone } from './types';

export interface ToneColors {
  fg: string;
  bg: string;
  bd: string;
}

const TONE_VAR: Record<Tone, string> = {
  ok: '--ok',
  wait: '--wait',
  fail: '--fail',
  disp: '--disp',
  ac: '--ac',
  mute: '--tx3',
};

export function tone(t: Tone): ToneColors {
  const v = TONE_VAR[t];
  return {
    fg: t === 'ac' ? 'var(--ac-tx)' : t === 'mute' ? 'var(--tx2)' : `var(${v})`,
    bg: t === 'mute' ? 'var(--s2)' : `color-mix(in oklab, var(${v}) 13%, transparent)`,
    bd: t === 'mute' ? 'var(--bd2)' : `color-mix(in oklab, var(${v}) 38%, transparent)`,
  };
}

export function plural(n: number, singular: string, pluralForm = singular + 's'): string {
  return `${n} ${n > 1 ? pluralForm : singular}`;
}

/** Anneau de réputation autour d'un avatar (proportion de la note sur 5). */
export function repRing(rep: number): string {
  return `conic-gradient(var(--ac) ${(rep / 5) * 100}%, var(--bd2) 0)`;
}

export function milestoneTitles(pack: Pack): string[] {
  return pack.id === 'p1' ? P1_MILESTONES : GENERIC_MILESTONES.slice(0, pack.ms);
}

/**
 * Répartit le prix d'un pack entre ses jalons : parts égales arrondies
 * à l'entier inférieur, le reste va au dernier jalon. La somme vaut
 * toujours le prix (RG10).
 */
export function splitMilestones(pack: Pack): Milestone[] {
  const base = Math.floor(pack.price / pack.ms);
  return milestoneTitles(pack).map((t, i) => ({
    n: i + 1,
    t,
    cr: i === pack.ms - 1 ? pack.price - base * (pack.ms - 1) : base,
  }));
}

export interface PackFilters {
  stack: string;
  q: string;
  maxPrice: number;
  maxDays: number;
  minRep: number;
}

export const DEFAULT_FILTERS: PackFilters = { stack: 'Toutes', q: '', maxPrice: 200, maxDays: 7, minRep: 0 };

export function filterPacks(packs: Pack[], f: PackFilters): Pack[] {
  const q = f.q.trim().toLowerCase();
  return packs.filter(
    (x) =>
      (f.stack === 'Toutes' || x.stack.includes(f.stack)) &&
      x.price <= f.maxPrice &&
      x.days <= f.maxDays &&
      x.rep >= f.minRep &&
      (!q || `${x.title} ${x.stack.join(' ')} ${x.name}`.toLowerCase().includes(q)),
  );
}

export interface MilestoneState {
  label: string;
  tone: Tone;
  icon: IconName;
}

export function milestoneState(index: number, flow: Flow): MilestoneState {
  if (index === 0) {
    if (flow <= 1) return { label: 'À venir', tone: 'mute', icon: 'circle' };
    if (flow === 2) return { label: 'En cours', tone: 'ac', icon: 'circle-dot' };
    if (flow === 3) return { label: 'CI en cours', tone: 'wait', icon: 'loader-circle' };
    if (flow === 4) return { label: 'À valider', tone: 'wait', icon: 'hourglass' };
    return { label: 'Validé', tone: 'ok', icon: 'circle-check' };
  }
  if (index === 1 && flow >= 5) return { label: 'En cours', tone: 'ac', icon: 'circle-dot' };
  return { label: 'À venir', tone: 'mute', icon: 'circle' };
}

export interface ExchangeAmounts {
  total: number;
  /** Crédits déjà libérés au prestataire. */
  released: number;
  /** Crédits encore bloqués en séquestre. */
  locked: number;
  /** Montant du premier jalon. */
  first: number;
}

export function exchangeAmounts(pack: Pack, flow: Flow): ExchangeAmounts {
  const first = splitMilestones(pack)[0].cr;
  const released = flow >= 5 ? first : 0;
  return { total: pack.price, released, locked: pack.price - released, first };
}

export interface Balances {
  available: number;
  escrowOut: number;
  escrowIn: number;
  escrowTotal: number;
}

export function balances(pack: Pack, flow: Flow): Balances {
  const { total, locked } = exchangeAmounts(pack, flow);
  const available = flow >= 1 ? START_BALANCE - total : START_BALANCE;
  const escrowOut = flow === 0 ? 0 : locked;
  return { available, escrowOut, escrowIn: ESCROW_IN, escrowTotal: escrowOut + ESCROW_IN };
}

export function percent(part: number, total: number): string {
  return `${total ? (part / total) * 100 : 0}%`;
}

export interface NowCard {
  you: boolean;
  who: string;
  ini: string;
  t: string;
  sub: string;
}

/** Carte « Maintenant » : qui doit agir sur l'échange et pourquoi. */
export function nowCard(pack: Pack, flow: Flow): NowCard {
  const first = pack.name.split(' ')[0];
  const ms = splitMilestones(pack);
  const m1 = ms[0].cr;
  switch (flow) {
    case 0:
      return { you: false, who: '—', ini: '—', t: 'Aucun échange en cours', sub: '' };
    case 1:
      return { you: false, who: first, ini: pack.ini, t: `${first} doit accepter ta demande`, sub: `Tes ${pack.price} cr sont bloqués en séquestre. Sans réponse sous 48 h, ils te reviennent automatiquement.` };
    case 2:
      return { you: false, who: first, ini: pack.ini, t: `${first} doit livrer le jalon 1 · ${ms[0].t}`, sub: 'Livraison attendue avant le 17 oct. Tu seras notifié·e dès que la PR est ouverte.' };
    case 3:
      return { you: false, who: 'CI', ini: 'CI', t: 'La CI vérifie la PR #42', sub: "3 checks en cours sur le commit a3f9c1e. Rien à faire pour toi pour l'instant." };
    case 4:
      return { you: true, who: 'Toi', ini: 'SB', t: 'À toi : vérifie et valide le jalon 1', sub: `PR #42 mergée, CI au vert. Valide pour libérer ${m1} cr à ${first}, ou ouvre un litige si un critère n'est pas rempli.` };
    case 5:
      return { you: false, who: first, ini: pack.ini, t: `${first} doit livrer le jalon 2 · ${ms[1] ? ms[1].t : ''}`, sub: `Jalon 1 validé : ${m1} cr libérés, reliés à la PR #42 et au commit a3f9c1e.` };
  }
}
