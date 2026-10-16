import { describe, expect, it } from 'vitest';
import { burndownPoints } from './ScrumPanel';

describe('burndownPoints', () => {
  const sprint = { debut_prevu: '2026-10-20T10:00:00Z', fin_prevue: '2026-10-27T10:00:00Z' };
  const stories = [
    { points: 5, date_terminee: '2026-10-21T15:00:00Z' },
    { points: 3, date_terminee: null },
    { points: null, date_terminee: '2026-10-20T11:00:00Z' },
  ];

  it('part du total et retire les points terminés, jour par jour, jusqu’à aujourd’hui', () => {
    const r = burndownPoints(sprint, stories, Date.parse('2026-10-23T12:00:00Z'));
    expect(r.days).toBe(7);
    expect(r.total).toBe(8);
    expect(r.actual).toEqual([
      { day: 0, remaining: 8 },
      { day: 1, remaining: 8 },
      { day: 2, remaining: 3 },
      { day: 3, remaining: 3 },
    ]);
  });

  it('s’arrête à la fin du sprint', () => {
    expect(burndownPoints(sprint, stories, Date.parse('2026-11-30T00:00:00Z')).actual).toHaveLength(8);
  });
});
