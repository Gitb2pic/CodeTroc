import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useResource } from '../../api/useResource';
import { Pill } from '../../components/ui';
import { fmtTime, MOVEMENT, shortId } from '../format';
import { FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

export function WalletPage() {
  const balance = useResource(() => call(() => api.GET('/me/compte')), []);
  const first = useResource(() => call(() => api.GET('/me/compte/mouvements', { params: { query: { limit: 30 } } })), []);
  const [more, setMore] = useState<S['Movement'][]>([]);
  const [cursor, setCursor] = useState<string | null | undefined>(undefined);
  const act = useAction();
  const next = cursor === undefined ? first.data?.next_cursor : cursor;

  const loadMore = async () => {
    const p = await act.run(() => call(() => api.GET('/me/compte/mouvements', { params: { query: { limit: 30, cursor: next ?? undefined } } })));
    if (p) {
      setMore((m) => [...m, ...p.items]);
      setCursor(p.next_cursor);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <h1 className="h1">Portefeuille</h1>
        <span className="muted">Chaque mouvement est une écriture du grand livre, reliée à son échange.</span>
      </div>
      <Loaded r={balance} what="Le solde">
        {(b) => (
          <div className="stats stats-3">
            <div style={{ background: 'var(--s1)', padding: 18, display: 'flex', flexDirection: 'column', gap: 6 }}>
              <span className="kicker">Disponible</span>
              <span style={{ font: '600 34px/1 var(--mono)', color: 'var(--ac-tx)', whiteSpace: 'nowrap' }}>{b.disponible} <span style={{ fontSize: 16 }}>cr</span></span>
            </div>
            <div style={{ background: 'var(--s1)', padding: 18, display: 'flex', flexDirection: 'column', gap: 6 }}>
              <span className="kicker">Séquestre sortant</span>
              <span style={{ font: '600 22px/1.2 var(--mono)' }}>{b.sequestre_sortant} cr</span>
            </div>
            <div style={{ background: 'var(--s1)', padding: 18, display: 'flex', flexDirection: 'column', gap: 6 }}>
              <span className="kicker">Séquestre entrant</span>
              <span style={{ font: '600 22px/1.2 var(--mono)' }}>{b.sequestre_entrant} cr</span>
            </div>
          </div>
        )}
      </Loaded>
      <Loaded r={first} what="L'historique">
        {(p) => (
          <section className="card" style={{ overflow: 'hidden' }}>
            <div className="table-scroll">
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <caption className="h2" style={{ textAlign: 'left', padding: '14px 18px' }}>Mouvements</caption>
              <thead>
                <tr className="kicker" style={{ textAlign: 'left' }}>
                  <th style={{ padding: '8px 18px' }}>Date</th><th>Type</th><th>Échange</th><th style={{ textAlign: 'right', padding: '8px 18px' }}>Montant</th>
                </tr>
              </thead>
              <tbody>
                {[...p.items, ...more].map((m) => {
                  const [label, tone] = MOVEMENT[m.type];
                  return (
                    <tr key={m.id} style={{ borderTop: '1px solid var(--bd)' }}>
                      <td className="mono faint" style={{ padding: '10px 18px', whiteSpace: 'nowrap' }}>{fmtTime(m.date)}</td>
                      <td><Pill tone={tone}>{label}</Pill></td>
                      <td className="mono">{m.id_echange ? <Link to={R.exchange(m.id_echange)}>{shortId(m.id_echange)}</Link> : '—'}</td>
                      <td className="mono" style={{ textAlign: 'right', padding: '10px 18px', color: m.montant > 0 ? 'var(--ok)' : 'var(--tx)' }}>{m.montant > 0 ? '+' : '−'}{Math.abs(m.montant)} cr</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            </div>
            {next && (
              <div style={{ padding: 12, display: 'flex', justifyContent: 'center' }}>
                <button className="btn btn-secondary" style={small} disabled={act.busy} onClick={loadMore}>Charger plus</button>
              </div>
            )}
            <FormError>{act.error}</FormError>
          </section>
        )}
      </Loaded>
    </div>
  );
}
