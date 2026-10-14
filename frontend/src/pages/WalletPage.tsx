import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { EmptyState, ErrorBanner, EscrowBar, Icon, Skeleton, useEtat } from '../components/ui';
import { WELCOME_BONUS } from '../data/fixtures';
import { balances, exchangeAmounts, percent, splitMilestones, tone } from '../domain/logic';
import type { Tone } from '../domain/types';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';

type Kind = 'Reçu' | 'Séquestre' | 'Libéré' | 'Arbitrage' | 'Bonus';
interface Tx {
  date: string;
  t: string;
  ex: string;
  proof: string;
  proofIcon: string;
  amt: string;
  kind: Kind;
  tone: Tone;
  from: string;
}

const TABS = ['Tout', 'Reçu', 'Séquestre', 'Libéré', 'Arbitrage'] as const;
const COLS = '100px minmax(0,1fr) 90px 120px 80px';

export function WalletPage() {
  const navigate = useNavigate();
  const st = useEtat();
  const { exPack: pack, flow } = useApp();
  const [tab, setTab] = useState<(typeof TABS)[number]>('Tout');
  const bal = balances(pack, flow);
  const amounts = exchangeAmounts(pack, flow);
  const ms = splitMilestones(pack);

  const tx: Tx[] = [];
  if (flow >= 5) tx.push({ date: "Aujourd'hui", t: 'Jalon 1 libéré → Léa Martin', ex: 'EX-0142', proof: 'PR #42', proofIcon: 'git-merge', amt: `−${amounts.first}`, kind: 'Libéré', tone: 'ac', from: 'séquestre' });
  if (flow >= 1) tx.push({ date: 'lun. 14 oct.', t: `Mise en séquestre · ${pack.title}`, ex: 'EX-0142', proof: 'Demande', proofIcon: 'file-text', amt: `−${amounts.total}`, kind: 'Séquestre', tone: 'wait', from: 'disponible' });
  tx.push(
    { date: 'dim. 13 oct.', t: 'Jalon 1 reçu de Karim Haddad', ex: 'EX-0138', proof: 'PR #7', proofIcon: 'git-merge', amt: '+35', kind: 'Reçu', tone: 'ok', from: 'disponible' },
    { date: 'ven. 11 oct.', t: 'Remboursement après arbitrage', ex: 'EX-0117', proof: 'Décision #A-23', proofIcon: 'gavel', amt: '+44', kind: 'Arbitrage', tone: 'disp', from: 'disponible' },
    { date: 'mer. 9 oct.', t: 'Mise en séquestre · API REST FastAPI', ex: 'EX-0131', proof: 'Demande', proofIcon: 'file-text', amt: '−110', kind: 'Séquestre', tone: 'wait', from: 'disponible' },
    { date: 'lun. 7 oct.', t: 'Pack livré à Inès Roy (3 jalons)', ex: 'EX-0124', proof: 'PR #31', proofIcon: 'git-merge', amt: '+150', kind: 'Reçu', tone: 'ok', from: 'disponible' },
    { date: '28 sept.', t: 'Pack livré à Tom Weber', ex: 'EX-0109', proof: 'PR #12', proofIcon: 'git-merge', amt: '+90', kind: 'Reçu', tone: 'ok', from: 'disponible' },
    { date: '20 sept.', t: 'Bonus de bienvenue', ex: '—', proof: 'Inscription', proofIcon: 'user', amt: `+${WELCOME_BONUS}`, kind: 'Bonus', tone: 'mute', from: 'disponible' },
  );
  const shown = tx.filter((x) => tab === 'Tout' || x.kind === tab);

  const escrows = [
    ...(flow > 0 ? [{ ex: 'EX-0142', t: pack.title, who: `Vers ${pack.name}`, dir: 'Sortant', locked: `${amounts.locked} cr`, relW: percent(amounts.released, amounts.total), curW: percent(amounts.first, amounts.total), sub: `${flow >= 5 ? 1 : 0}/${ms.length} jalons libérés`, to: ROUTES.ex }] : []),
    { ex: 'EX-0138', t: 'Formulaire React accessible', who: 'De Karim Haddad', dir: 'Entrant', locked: '35 cr', relW: '50%', curW: '50%', sub: '1/2 jalons libérés', to: ROUTES.ex },
    { ex: 'EX-0136', t: 'Suite de tests E2E Playwright', who: 'Vers Nora Diallo', dir: 'Sortant', locked: '60 cr', relW: '0%', curW: '50%', sub: '0/2 jalons libérés', to: ROUTES.ex },
    { ex: 'EX-0131', t: 'API REST FastAPI + auth JWT', who: 'Vers Hugo Lambert · gelé', dir: 'En litige', locked: '110 cr', relW: '0%', curW: '0%', sub: "Gelé pendant l'arbitrage", to: ROUTES.dispute },
  ];

  const statCell = { background: 'var(--s1)', padding: 18, display: 'flex', flexDirection: 'column', gap: 6 } as const;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <h1 className="h1">Portefeuille</h1>
        <span className="muted">Chaque mouvement est relié à un échange et à sa preuve.</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1.3fr 1fr 1fr 1fr', gap: 1, background: 'var(--bd)', border: '1px solid var(--bd)', borderRadius: 12, overflow: 'hidden' }}>
        <div style={statCell}>
          <span className="kicker">Disponible</span>
          <span style={{ font: '600 34px/1 var(--mono)', letterSpacing: '-.03em', color: 'var(--ac-tx)' }}>
            {bal.available} <span style={{ fontSize: 16 }}>cr</span>
          </span>
        </div>
        <div style={statCell}>
          <span className="kicker">Séquestre sortant</span>
          <span style={{ font: '600 22px/1.2 var(--mono)' }}>{bal.escrowOut} cr</span>
        </div>
        <div style={statCell}>
          <span className="kicker">Séquestre entrant</span>
          <span style={{ font: '600 22px/1.2 var(--mono)' }}>{bal.escrowIn} cr</span>
        </div>
        <div style={statCell}>
          <span className="kicker">30 derniers jours</span>
          <span style={{ font: '500 14px/1.6 var(--mono)' }}>
            <span style={{ color: 'var(--ac-tx)' }}>+369</span> gagnés
            <br />
            {flow >= 1 ? `−${110 + amounts.total}` : '−110'} engagés
          </span>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 340px', gap: 24, alignItems: 'start' }}>
        <section className="card" style={{ overflow: 'hidden' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '10px 12px', borderBottom: '1px solid var(--bd)' }}>
            {TABS.map((t) => {
              const on = tab === t;
              return (
                <button key={t} onClick={() => setTab(t)} aria-pressed={on} style={{ height: 28, padding: '0 10px', borderRadius: 6, border: 0, background: on ? 'var(--s3)' : 'transparent', color: on ? 'var(--tx)' : 'var(--tx2)', font: '500 12.5px var(--sans)', cursor: 'pointer' }}>
                  {t}
                </button>
              );
            })}
          </div>
          {st.normal && (
            <div role="table" aria-label="Mouvements de crédits">
              <div role="row" className="mono faint" style={{ display: 'grid', gridTemplateColumns: COLS, gap: 12, padding: '8px 18px', fontSize: 11, fontWeight: 500, letterSpacing: '.04em', textTransform: 'uppercase', borderBottom: '1px solid var(--bd)' }}>
                <span role="columnheader">Date</span>
                <span role="columnheader">Opération</span>
                <span role="columnheader">Échange</span>
                <span role="columnheader">Preuve</span>
                <span role="columnheader" style={{ textAlign: 'right' }}>Crédits</span>
              </div>
              {shown.map((x) => {
                const c = tone(x.tone);
                return (
                  <div key={`${x.date}-${x.t}`} role="row" style={{ display: 'grid', gridTemplateColumns: COLS, gap: 12, alignItems: 'center', padding: '12px 18px', borderBottom: '1px solid var(--bd)' }}>
                    <span role="cell" className="mono faint" style={{ fontSize: 12 }}>{x.date}</span>
                    <span role="cell" style={{ display: 'flex', flexDirection: 'column', gap: 3, minWidth: 0 }}>
                      <span className="ellipsis" style={{ fontSize: 13.5 }}>{x.t}</span>
                      <span style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                        <span style={{ height: 18, padding: '0 6px', display: 'inline-flex', alignItems: 'center', borderRadius: 5, font: '500 10.5px/1 var(--mono)', color: c.fg, background: c.bg, border: `1px solid ${c.bd}` }}>{x.kind}</span>
                        <span className="mono faint" style={{ fontSize: 11 }}>depuis {x.from}</span>
                      </span>
                    </span>
                    <a role="cell" href="#" onClick={(e) => e.preventDefault()} className="mono" style={{ fontSize: 12 }}>{x.ex}</a>
                    <a role="cell" href="#" onClick={(e) => e.preventDefault()} className="mono" style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12 }}>
                      <Icon name={x.proofIcon} size={13} />
                      {x.proof}
                    </a>
                    <span role="cell" style={{ textAlign: 'right', font: '600 14px var(--mono)', color: x.amt[0] === '+' ? 'var(--ac-tx)' : 'var(--tx)' }}>{x.amt}</span>
                  </div>
                );
              })}
            </div>
          )}
          {st.empty && (
            <EmptyState icon="wallet" title="Aucun mouvement pour l'instant" dashed={false} padding="48px 24px">
              Ton bonus de bienvenue de {WELCOME_BONUS} cr apparaîtra ici dès ta connexion.
            </EmptyState>
          )}
          {st.loading && (
            <div aria-busy="true" style={{ padding: '12px 18px', display: 'flex', flexDirection: 'column', gap: 14 }}>
              {TABS.map((t) => (
                <Skeleton key={t} style={{ height: 34, borderRadius: 6 }} />
              ))}
            </div>
          )}
          {st.error && (
            <ErrorBanner style={{ margin: 16 }}>
              <span style={{ fontSize: 13.5 }}>Historique indisponible. Ton solde affiché reste exact.</span>
            </ErrorBanner>
          )}
        </section>

        <aside style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h2 className="h2" style={{ marginBottom: 4 }}>Séquestres actifs</h2>
          {escrows.map((e) => (
            <button key={e.ex} className="card hov-bd2" onClick={() => navigate(e.to)} style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: 14, color: 'var(--tx)', textAlign: 'left', cursor: 'pointer' }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 8, width: '100%' }}>
                <span className="mono faint" style={{ fontSize: 12 }}>{e.ex}</span>
                <span className="mono muted" style={{ fontSize: 11, fontWeight: 500, padding: '1px 6px', border: '1px solid var(--bd2)', borderRadius: 5 }}>{e.dir}</span>
                <span style={{ flex: 1 }} />
                <span style={{ font: '600 14px var(--mono)' }}>{e.locked}</span>
              </span>
              <span style={{ fontSize: 13.5, fontWeight: 500 }}>{e.t}</span>
              <EscrowBar released={e.relW} current={e.curW} height={6} />
              <span className="mono faint" style={{ fontSize: 11.5 }}>{e.who} · {e.sub}</span>
            </button>
          ))}
        </aside>
      </div>
    </div>
  );
}
