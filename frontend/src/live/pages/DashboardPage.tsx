import { useNavigate } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { EmptyState, Icon, PageTitle, Pill } from '../../components/ui';
import { EXCHANGE_STATUS, fmtDate, shortId } from '../format';
import { Loaded } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

export function DashboardPage() {
  const { user } = useSession();
  const navigate = useNavigate();
  const data = useResource(async () => {
    const [balance, exchanges] = await Promise.all([
      call(() => api.GET('/me/compte')),
      call(() => api.GET('/echanges', { params: { query: { limit: 50 } } })),
    ]);
    return { balance, exchanges: exchanges.items };
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <PageTitle title={`Bonjour ${user?.pseudo ?? ''}`} sub="Ce qui t'attend, et ce qui attend les autres." />
      <Loaded r={data} what="Le tableau de bord">
        {({ balance, exchanges }) => {
          const role = (x: S['Exchange']) => (x.demandeur.id === user?.id ? 'DEMANDEUR' : 'PRESTATAIRE');
          const mine = exchanges.filter((x) => x.a_qui_de_jouer === role(x));
          const active = exchanges.filter((x) => ['PROPOSE', 'EN_COURS', 'EN_LITIGE'].includes(x.statut));
          return (
            <>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 1, background: 'var(--bd)', border: '1px solid var(--bd)', borderRadius: 12, overflow: 'hidden' }}>
                {[
                  ['Disponible', `${balance.disponible} cr`],
                  ['Séquestre sortant', `${balance.sequestre_sortant} cr`],
                  ['Séquestre entrant', `${balance.sequestre_entrant} cr`],
                  ['À toi de jouer', String(mine.length)],
                ].map(([k, v]) => (
                  <div key={k} style={{ background: 'var(--s1)', padding: 18, display: 'flex', flexDirection: 'column', gap: 6 }}>
                    <span className="kicker">{k}</span>
                    <span style={{ font: '600 22px/1.2 var(--mono)' }}>{v}</span>
                  </div>
                ))}
              </div>
              <section className="card" style={{ overflow: 'hidden' }}>
                <div style={{ padding: '14px 18px', borderBottom: '1px solid var(--bd)' }}>
                  <h2 className="h2">Échanges en cours</h2>
                </div>
                {active.length === 0 ? (
                  <EmptyState icon="arrow-left-right" title="Aucun échange en cours" dashed={false} actions={<button className="btn btn-primary" onClick={() => navigate(R.cat)} style={{ fontSize: 13 }}>Ouvrir le catalogue</button>}>
                    Demande un pack, ou publie une offre pour recevoir des demandes.
                  </EmptyState>
                ) : (
                  <ul style={{ margin: 0, padding: 0, listStyle: 'none' }}>
                    {active.map((x) => <ExchangeRow key={x.id} x={x} yourTurn={x.a_qui_de_jouer === role(x)} asProvider={role(x) === 'PRESTATAIRE'} />)}
                  </ul>
                )}
              </section>
              {exchanges.length > active.length && (
                <section className="card" style={{ overflow: 'hidden' }}>
                  <div style={{ padding: '14px 18px', borderBottom: '1px solid var(--bd)' }}>
                    <h2 className="h2">Historique</h2>
                  </div>
                  <ul style={{ margin: 0, padding: 0, listStyle: 'none' }}>
                    {exchanges.filter((x) => !active.includes(x)).map((x) => <ExchangeRow key={x.id} x={x} yourTurn={false} asProvider={role(x) === 'PRESTATAIRE'} />)}
                  </ul>
                </section>
              )}
            </>
          );
        }}
      </Loaded>
    </div>
  );
}

function ExchangeRow({ x, yourTurn, asProvider }: { x: S['Exchange']; yourTurn: boolean; asProvider: boolean }) {
  const navigate = useNavigate();
  const [label, tone] = EXCHANGE_STATUS[x.statut];
  const other = asProvider ? x.demandeur : x.prestataire;
  return (
    <li>
      <button className="hov-nav" onClick={() => navigate(R.exchange(x.id))} style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 18px', border: 0, borderBottom: '1px solid var(--bd)', background: 'none', color: 'var(--tx)', cursor: 'pointer', textAlign: 'left', font: 'inherit' }}>
        <Icon name={asProvider ? 'arrow-down-left' : 'arrow-up-right'} style={{ color: 'var(--tx3)' }} />
        <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontWeight: 500 }}>{x.offre.titre}</span>
          <span className="mono faint" style={{ fontSize: 12 }}>{shortId(x.id)} · {asProvider ? 'pour' : 'avec'} {other.pseudo} · {fmtDate(x.date_creation)}</span>
        </span>
        {yourTurn && <Pill tone="ac" icon="hand">À toi</Pill>}
        <Pill tone={tone}>{label}</Pill>
        <span className="mono" style={{ fontSize: 13, width: 70, textAlign: 'right' }}>{x.montant_total} cr</span>
      </button>
    </li>
  );
}
