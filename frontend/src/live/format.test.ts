import { describe, expect, it } from 'vitest';
import type { S } from '../api/client';
import { fmtRelative, initials, notificationText, offerToPack, shortId } from './format';

describe('mise en forme du mode API', () => {
  it('initiales', () => {
    expect(initials('lea')).toBe('LE');
    expect(initials('sami-benali')).toBe('SB');
    expect(initials('Léa Martin')).toBe('LM');
  });

  it('identifiant court', () => {
    expect(shortId('ea6d288f-1ffd-459d-b855-9f8d0e9f858f')).toBe('EX-EA6D28');
  });

  it('dates relatives', () => {
    const now = Date.parse('2026-10-14T12:00:00Z');
    expect(fmtRelative('2026-10-14T11:59:30Z', now)).toBe("à l'instant");
    expect(fmtRelative('2026-10-14T11:15:00Z', now)).toBe('il y a 45 min');
    expect(fmtRelative('2026-10-12T12:00:00Z', now)).toBe('il y a 2 j');
  });

  it("une offre devient une carte de catalogue", () => {
    const o = {
      id: 'o1', titre: 'Pipeline', description: '', prix_credits: 120, delai_jours: 5, statut: 'PUBLIEE', date_publication: '',
      jalons: [{ titre: 'a', montant: 60 }, { titre: 'b', montant: 60 }],
      pack: { id: 'p', titre: 'P', type_preuve: 'PR_CI', competences: ['Docker'] },
      prestataire: { id: 'u', pseudo: 'lea', github_login: 'leam', score_reputation: 4.9, livraisons_verifiees: 38 },
    } as S['Offer'];
    expect(offerToPack(o)).toMatchObject({ id: 'o1', price: 120, ms: 2, stack: ['Docker'], name: 'lea', handle: 'leam', rep: 4.9, deliv: 38 });
  });

  it('texte des notifications, sans jamais de valeur secrète', () => {
    const n = (type: string, donnees: Record<string, unknown>) => ({ id: 'n', type, id_echange: 'x', donnees, lue: false, date: '' }) as S['Notification'];
    expect(notificationText(n('JALON_LIBERE', { montant: 40, jalon: 'Workflow', pack: 'CI' })).text).toBe('40 cr libérés pour le jalon « Workflow »');
    expect(notificationText(n('ECHANGE_DEMANDE', { pack: 'Pipeline' })).text).toBe("Nouvelle demande d'échange · Pipeline");
    expect(notificationText(n('INCONNU', {})).icon).toBe('bell');
  });
});
