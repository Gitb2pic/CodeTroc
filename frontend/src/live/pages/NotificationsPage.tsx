import { useNavigate } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useResource } from '../../api/useResource';
import { EmptyState, Icon, PageTitle } from '../../components/ui';
import { tone as toneColors } from '../../domain/logic';
import { fmtRelative, notificationText } from '../format';
import { Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

export function NotificationsPage() {
  const navigate = useNavigate();
  const list = useResource(() => call(() => api.GET('/notifications', { params: { query: { limit: 50 } } })), []);
  const act = useAction();

  const markAll = () => act.run(() => call(() => api.POST('/notifications/lire', { body: {} })), 'Tout est marqué comme lu').then(list.reload);
  const open = async (n: S['Notification']) => {
    if (!n.lue) await call(() => api.POST('/notifications/lire', { body: { ids: [n.id] } })).catch(() => undefined);
    const d = n.donnees as Record<string, unknown>;
    if (typeof d.id_litige === 'string') navigate(R.dispute(d.id_litige));
    else if (n.id_echange) navigate(R.exchange(n.id_echange));
    else list.reload();
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageTitle title="Notifications" action={<button className="btn btn-secondary" style={small} disabled={act.busy} onClick={markAll}>Tout marquer comme lu</button>} />
      <Loaded r={list} what="Les notifications">
        {(p) =>
          p.items.length === 0 ? (
            <EmptyState icon="bell" title="Rien de neuf">Les demandes, livraisons et décisions apparaîtront ici.</EmptyState>
          ) : (
            <ul className="card" style={{ margin: 0, padding: 0, listStyle: 'none', overflow: 'hidden' }}>
              {p.items.map((n) => {
                const { icon, text, tone } = notificationText(n);
                const c = toneColors(tone);
                return (
                  <li key={n.id}>
                    <button onClick={() => open(n)} className="hov-nav" style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 18px', border: 0, borderBottom: '1px solid var(--bd)', background: n.lue ? 'none' : 'color-mix(in oklab,var(--ac) 6%,transparent)', color: 'var(--tx)', cursor: 'pointer', textAlign: 'left', font: 'inherit' }}>
                      <span style={{ width: 30, height: 30, borderRadius: '50%', display: 'grid', placeItems: 'center', color: c.fg, background: c.bg, border: `1px solid ${c.bd}` }}>
                        <Icon name={icon} size={14} />
                      </span>
                      <span style={{ flex: 1, fontSize: 13.5, fontWeight: n.lue ? 400 : 600 }}>{text}</span>
                      <span className="mono faint" style={{ fontSize: 12 }}>{fmtRelative(n.date)}</span>
                      {!n.lue && <span aria-label="non lue" style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--ac)' }} />}
                    </button>
                  </li>
                );
              })}
            </ul>
          )
        }
      </Loaded>
    </div>
  );
}
