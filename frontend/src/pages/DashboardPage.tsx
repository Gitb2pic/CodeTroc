import { useNavigate } from 'react-router-dom';
import { EmptyState, Icon, PageTitle, Pill, Skeleton, useEtat } from '../components/ui';
import { ME } from '../data/fixtures';
import { balances, exchangeAmounts, milestoneState, nowCard, percent, tone, type ToneColors } from '../domain/logic';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';

interface Row {
  id: string;
  title: string;
  who: string;
  ini: string;
  role: string;
  step: string;
  state: string;
  icon: string;
  spin?: boolean;
  colors: ToneColors;
  next: string;
  nextYou: boolean;
  cr: string;
  to: string;
}

const COLS = 'minmax(0,1fr) 150px 110px 90px';

export function DashboardPage() {
  const navigate = useNavigate();
  const st = useEtat();
  const { exPack, flow } = useApp();
  const bal = balances(exPack, flow);
  const amounts = exchangeAmounts(exPack, flow);
  const now = nowCard(exPack, flow);

  const rows: Row[] = [];
  if (flow > 0) {
    const cur = milestoneState(flow >= 5 ? 1 : 0, flow);
    rows.push({ id: 'EX-0142', title: exPack.title, who: exPack.name, ini: exPack.ini, role: 'Tu reçois', step: flow >= 5 ? 'Jalon 2/3' : 'Jalon 1/3', state: cur.label, icon: cur.icon, spin: cur.icon === 'loader-circle', colors: tone(cur.tone), next: now.you ? 'Toi' : now.who, nextYou: now.you, cr: `${amounts.released}/${amounts.total}`, to: ROUTES.ex });
  }
  rows.push(
    { id: 'EX-0138', title: 'Formulaire React accessible', who: 'Karim Haddad', ini: 'KH', role: 'Tu livres', step: 'Jalon 2/2', state: 'En cours', icon: 'circle-dot', colors: tone('ac'), next: 'Toi', nextYou: true, cr: '35/70', to: ROUTES.ex },
    { id: 'EX-0136', title: 'Suite de tests E2E Playwright', who: 'Nora Diallo', ini: 'ND', role: 'Tu reçois', step: 'Jalon 1/2', state: 'Changements demandés', icon: 'rotate-ccw', colors: tone('wait'), next: 'Nora', nextYou: false, cr: '0/60', to: ROUTES.ex },
    { id: 'EX-0131', title: 'API REST FastAPI + auth JWT', who: 'Hugo Lambert', ini: 'HL', role: 'Tu reçois', step: 'Jalon 3/3', state: 'En litige', icon: 'scale', colors: tone('disp'), next: 'Arbitres', nextYou: false, cr: '0/110', to: ROUTES.dispute },
  );

  const todos = [
    ...(flow === 4 ? [{ icon: 'circle-check', t: 'Valider le jalon 1 de EX-0142', sub: 'PR #42 mergée · CI au vert', cta: 'Vérifier', to: ROUTES.ex, hot: true }] : []),
    { icon: 'git-pull-request', t: 'Livrer le jalon 2 de EX-0138', sub: 'Pour Karim · avant le 18 oct.', cta: 'Ouvrir', to: ROUTES.ex, hot: false },
    { icon: 'key-round', t: 'Un accès expire dans 23 h', sub: 'GHCR_PUSH_TOKEN · partagé avec Léa', cta: 'Gérer', to: ROUTES.vault, hot: false },
    { icon: 'scale', t: 'Ajouter ta réponse au litige EX-0131', sub: 'Les arbitres votent sous 18 h', cta: 'Répondre', to: ROUTES.dispute, hot: false },
  ];

  const activity = [
    ...(flow >= 5 ? [{ icon: 'lock-open', t: `${amounts.first} cr libérés à Léa`, ref: 'PR #42', time: "à l'instant", to: ROUTES.wallet }] : []),
    ...(flow >= 4 ? [{ icon: 'circle-check', t: 'CI réussie sur acme/shop-api', ref: 'a3f9c1e', time: 'il y a 2 min', to: ROUTES.ex }] : []),
    { icon: 'arrow-down-left', t: 'Karim a libéré 35 cr', ref: 'PR #7', time: 'hier', to: ROUTES.wallet },
    { icon: 'star', t: 'Nouvel avis 5/5 de Karim', ref: 'EX-0129', time: 'hier', to: ROUTES.profile },
    { icon: 'scale', t: 'Litige ouvert par Hugo', ref: 'EX-0131', time: 'il y a 2 j', to: ROUTES.dispute },
    { icon: 'key-round', t: 'Accès SSH créé pour Léa', ref: 'staging-01', time: 'il y a 2 j', to: ROUTES.vault },
  ];

  const exCount = `${rows.length} en cours`;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <PageTitle
        title={`Salut ${ME.first}`}
        sub={
          <>
            Tu as <strong style={{ color: 'var(--tx)', fontWeight: 600 }}>{todos.length} actions</strong> à faire et {exCount}.
          </>
        }
        action={
          <button className="btn btn-secondary" onClick={() => navigate(ROUTES.cat)}>
            <Icon name="package" />
            Parcourir les packs
          </button>
        }
      />

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 12 }}>
        <button className="card hov-bd2" onClick={() => navigate(ROUTES.wallet)} style={{ textAlign: 'left', cursor: 'pointer', padding: 18, display: 'flex', flexDirection: 'column', gap: 10, color: 'var(--tx)' }}>
          <span className="kicker" style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="coins" size={13} />Disponible</span>
          <span style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
            <span style={{ font: '600 34px/1 var(--mono)', letterSpacing: '-.03em', color: 'var(--ac-tx)' }}>{bal.available}</span>
            <span style={{ font: '500 15px var(--mono)', color: 'var(--ac-tx)' }}>cr</span>
          </span>
          <span className="muted" style={{ fontSize: 12.5 }}>Dépensable chez n'importe quel membre</span>
        </button>
        <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="kicker" style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="lock" size={13} />En séquestre</span>
          <div style={{ display: 'flex', gap: 20 }}>
            <span style={{ display: 'flex', flexDirection: 'column' }}>
              <span style={{ font: '600 20px/1.2 var(--mono)' }}>{bal.escrowOut} cr</span>
              <span className="faint" style={{ fontSize: 12 }}>sortant · tu reçois</span>
            </span>
            <span style={{ display: 'flex', flexDirection: 'column' }}>
              <span style={{ font: '600 20px/1.2 var(--mono)' }}>{bal.escrowIn} cr</span>
              <span className="faint" style={{ fontSize: 12 }}>entrant · tu livres</span>
            </span>
          </div>
          <div style={{ display: 'flex', height: 6, borderRadius: 99, overflow: 'hidden', gap: 2, background: 'var(--s3)' }}>
            <div style={{ width: percent(bal.escrowOut, bal.escrowTotal), background: 'repeating-linear-gradient(135deg,var(--ac) 0 3px,color-mix(in oklab,var(--ac) 40%,transparent) 3px 6px)' }} />
            <div style={{ width: percent(bal.escrowIn, bal.escrowTotal), background: 'var(--tx3)' }} />
          </div>
        </div>
        <button className="card hov-bd2" onClick={() => navigate(ROUTES.profile)} style={{ textAlign: 'left', cursor: 'pointer', padding: 18, display: 'flex', flexDirection: 'column', gap: 10, color: 'var(--tx)' }}>
          <span className="kicker" style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="star" size={13} />Réputation</span>
          <span style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
            <span style={{ font: '600 34px/1 var(--mono)', letterSpacing: '-.03em' }}>{ME.rep}</span>
            <span className="muted" style={{ fontSize: 13 }}>/ 5</span>
          </span>
          <span className="muted" style={{ fontSize: 12.5 }}>14 livraisons vérifiées · 0 litige perdu</span>
        </button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 340px', gap: 20, alignItems: 'start' }}>
        <div className="card" style={{ overflow: 'hidden' }}>
          <div style={{ display: 'flex', alignItems: 'center', padding: '14px 18px', borderBottom: '1px solid var(--bd)' }}>
            <h2 className="h2">Échanges en cours</h2>
            <div style={{ flex: 1 }} />
            <span className="mono faint" style={{ fontSize: 12 }}>{exCount}</span>
          </div>
          {st.normal && (
            <>
              <div className="mono faint" style={{ display: 'grid', gridTemplateColumns: COLS, gap: 12, padding: '8px 18px', fontSize: 11, fontWeight: 500, letterSpacing: '.04em', textTransform: 'uppercase', borderBottom: '1px solid var(--bd)' }}>
                <span>Échange</span>
                <span>Jalon actuel</span>
                <span>À qui de jouer</span>
                <span style={{ textAlign: 'right' }}>Libéré</span>
              </div>
              {rows.map((r) => (
                <button key={r.id} className="hov-s2" onClick={() => navigate(r.to)} style={{ width: '100%', display: 'grid', gridTemplateColumns: COLS, gap: 12, alignItems: 'center', padding: '14px 18px', border: 0, borderBottom: '1px solid var(--bd)', background: 'none', color: 'var(--tx)', textAlign: 'left', cursor: 'pointer' }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
                    <span style={{ width: 28, height: 28, flex: 'none', borderRadius: '50%', background: 'var(--s3)', display: 'grid', placeItems: 'center', font: '600 10px var(--mono)' }}>{r.ini}</span>
                    <span style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
                      <span className="ellipsis" style={{ fontWeight: 500 }}>{r.title}</span>
                      <span className="mono faint" style={{ fontSize: 12 }}>{r.id} · {r.role} · {r.who}</span>
                    </span>
                  </span>
                  <span style={{ display: 'flex', flexDirection: 'column', gap: 3, alignItems: 'flex-start' }}>
                    <span className="mono faint" style={{ fontSize: 12 }}>{r.step}</span>
                    <Pill colors={r.colors} icon={r.icon} spin={r.spin}>{r.state}</Pill>
                  </span>
                  <span>
                    <span style={{ display: 'inline-flex', alignItems: 'center', height: 24, padding: '0 9px', borderRadius: 6, font: '500 12px/1 var(--sans)', color: r.nextYou ? 'var(--ac-tx)' : 'var(--tx2)', background: r.nextYou ? 'color-mix(in oklab, var(--ac) 14%, transparent)' : 'var(--s2)' }}>{r.next}</span>
                  </span>
                  <span style={{ font: '500 12.5px var(--mono)', textAlign: 'right', color: 'var(--tx2)' }}>{r.cr}</span>
                </button>
              ))}
            </>
          )}
          {st.loading && (
            <div aria-busy="true" aria-label="Chargement des échanges" style={{ display: 'flex', flexDirection: 'column' }}>
              {rows.map((r) => (
                <div key={r.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '16px 18px', borderBottom: '1px solid var(--bd)' }}>
                  <Skeleton style={{ width: 28, height: 28, borderRadius: '50%' }} />
                  <span style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
                    <Skeleton style={{ width: '55%', height: 10 }} />
                    <Skeleton style={{ width: '30%', height: 8 }} />
                  </span>
                  <Skeleton style={{ width: 80, height: 20, borderRadius: 99 }} />
                </div>
              ))}
            </div>
          )}
          {st.empty && (
            <EmptyState
              icon="arrow-left-right"
              title="Aucun échange en cours"
              dashed={false}
              padding="48px 24px"
              actions={
                <>
                  <button className="btn btn-primary" onClick={() => navigate(ROUTES.cat)} style={{ fontSize: 13 }}>Parcourir le catalogue</button>
                  <button className="btn btn-secondary" onClick={() => navigate(ROUTES.create)} style={{ fontSize: 13 }}>Proposer un pack</button>
                </>
              }
            >
              Demande un pack pour démarrer, ou propose le tien pour gagner tes premiers crédits.
            </EmptyState>
          )}
          {st.error && (
            <div role="alert" style={{ margin: 16, padding: '14px 16px', borderRadius: 10, border: '1px solid color-mix(in oklab,var(--fail) 40%,transparent)', background: 'color-mix(in oklab,var(--fail) 9%,transparent)', display: 'flex', gap: 12, alignItems: 'flex-start' }}>
              <Icon name="wifi-off" style={{ color: 'var(--fail)', marginTop: 2 }} />
              <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span style={{ fontWeight: 600 }}>Impossible de charger tes échanges</span>
                <span className="muted" style={{ fontSize: 13 }}>L'API GitHub ne répond pas (statut 502). Tes crédits et séquestres ne sont pas affectés.</span>
                <span className="mono faint" style={{ fontSize: 11.5, marginTop: 4 }}>req_id 7f3a-91c2</span>
              </div>
              <button className="btn btn-secondary" style={{ height: 30, padding: '0 12px', borderRadius: 7, fontSize: 12.5 }}>
                <Icon name="refresh-cw" size={14} />
                Réessayer
              </button>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <h2 className="h2" style={{ marginBottom: 4 }}>À faire</h2>
            {todos.map((t) => (
              <div key={t.t} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderRadius: 10, border: `1px solid ${t.hot ? 'color-mix(in oklab, var(--ac) 45%, transparent)' : 'var(--bd)'}`, background: t.hot ? 'color-mix(in oklab, var(--ac) 8%, var(--s1))' : 'var(--s1)' }}>
                <Icon name={t.icon} style={{ color: 'var(--tx2)' }} />
                <span style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
                  <span style={{ fontSize: 13.5, fontWeight: 500 }}>{t.t}</span>
                  <span className="mono faint" style={{ fontSize: 12 }}>{t.sub}</span>
                </span>
                <button className="btn btn-secondary" onClick={() => navigate(t.to)} style={{ height: 28, padding: '0 10px', borderRadius: 7, fontSize: 12 }}>{t.cta}</button>
              </div>
            ))}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <h2 className="h2" style={{ marginBottom: 8 }}>Activité récente</h2>
            {activity.map((a) => (
              <button key={a.t} className="hov-s2" onClick={() => navigate(a.to)} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 6px', border: 0, borderRadius: 7, background: 'none', color: 'var(--tx)', textAlign: 'left', cursor: 'pointer' }}>
                <Icon name={a.icon} size={14} style={{ color: 'var(--tx3)' }} />
                <span style={{ flex: 1, fontSize: 13 }}>
                  {a.t} <span className="mono" style={{ color: 'var(--ac-tx)', fontSize: 12 }}>{a.ref}</span>
                </span>
                <span className="mono faint" style={{ fontSize: 11.5 }}>{a.time}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
