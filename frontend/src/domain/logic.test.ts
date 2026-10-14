import { PACKS, START_BALANCE } from '../data/fixtures';
import { balances, DEFAULT_FILTERS, exchangeAmounts, filterPacks, milestoneState, nowCard, splitMilestones } from './logic';
import type { Flow } from './types';

const FLOWS: Flow[] = [0, 1, 2, 3, 4, 5];

describe('splitMilestones', () => {
  it.each(PACKS.map((p) => [p.id, p] as const))('la somme des jalons de %s égale le prix (RG10)', (_, pack) => {
    const ms = splitMilestones(pack);
    expect(ms).toHaveLength(pack.ms);
    expect(ms.reduce((a, m) => a + m.cr, 0)).toBe(pack.price);
    expect(ms.every((m) => m.cr > 0)).toBe(true);
  });

  it('donne le reste de la division au dernier jalon', () => {
    const pack = { ...PACKS[0], price: 100, ms: 3 };
    expect(splitMilestones(pack).map((m) => m.cr)).toEqual([33, 33, 34]);
  });
});

describe('filterPacks', () => {
  it('renvoie tout le catalogue avec les filtres par défaut', () => {
    expect(filterPacks(PACKS, DEFAULT_FILTERS)).toHaveLength(PACKS.length);
  });

  it('filtre par stack, prix, délai et réputation', () => {
    const r = filterPacks(PACKS, { ...DEFAULT_FILTERS, stack: 'Docker', maxPrice: 100, maxDays: 3 });
    expect(r.map((p) => p.id)).toEqual(['p2']);
    expect(filterPacks(PACKS, { ...DEFAULT_FILTERS, minRep: 4.8 }).every((p) => p.rep >= 4.8)).toBe(true);
  });

  it('cherche dans le titre, la stack et le nom, sans casse', () => {
    expect(filterPacks(PACKS, { ...DEFAULT_FILTERS, q: '  TERRAFORM ' }).map((p) => p.id)).toEqual(['p4']);
    expect(filterPacks(PACKS, { ...DEFAULT_FILTERS, q: 'inès' }).map((p) => p.id)).toEqual(['p3', 'p8']);
  });
});

describe('crédits du parcours EX-0142', () => {
  const pack = PACKS[0];

  it.each(FLOWS)('aucun crédit ne se crée ni ne se perd (étape %i)', (flow) => {
    const { released, locked, total } = exchangeAmounts(pack, flow);
    expect(released + locked).toBe(total);
    const b = balances(pack, flow);
    // Ce qui a quitté le solde disponible est soit en séquestre, soit libéré.
    expect(START_BALANCE - b.available).toBe(flow === 0 ? 0 : b.escrowOut + released);
  });

  it('libère le premier jalon seulement après validation', () => {
    expect(exchangeAmounts(pack, 4).released).toBe(0);
    expect(exchangeAmounts(pack, 5).released).toBe(40);
  });

  it("ne demande d'action à l'utilisateur qu'au moment de valider", () => {
    expect(FLOWS.filter((f) => nowCard(pack, f).you)).toEqual([4]);
  });

  it('fait progresser le statut du jalon 1', () => {
    expect(FLOWS.map((f) => milestoneState(0, f).label)).toEqual(['À venir', 'À venir', 'En cours', 'CI en cours', 'À valider', 'Validé']);
    expect(milestoneState(1, 5).label).toBe('En cours');
  });
});
