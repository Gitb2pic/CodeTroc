import type { ReactNode } from 'react';
import { Avatar, Icon, Pill } from '../components/ui';
import { tone } from '../domain/logic';
import type { Tone } from '../domain/types';

const EVIDENCE: { icon: string; tone: Tone; t: string; ref: string; by: string }[] = [
  { icon: 'circle-x', tone: 'fail', t: 'Check test_auth_refresh en échec', ref: 'hlambert/api #19 · 9c04b2e', by: 'Joint par toi' },
  { icon: 'list-checks', tone: 'wait', t: 'Critère 4 : « Refresh token avec rotation »', ref: 'Fiche du pack · v2', by: 'Joint par toi' },
  { icon: 'git-merge', tone: 'ok', t: 'Jalons 1 et 2 mergés, CI au vert', ref: 'PR #11, #15 · 7/7 checks', by: 'Joint par Hugo' },
  { icon: 'message-square', tone: 'mute', t: 'Échange du 9 oct. sur le périmètre du jalon 3', ref: 'Fil de discussion · 4 messages', by: 'Joint par Hugo' },
];

const ARBITERS = [
  { id: 'A', rep: '4.9', n: '61 livraisons', vote: 'Livraison partielle · 60/40', done: true, why: 'Jalons 1–2 prouvés par la CI. Le jalon 3 échoue sur un critère explicite.' },
  { id: 'B', rep: '4.8', n: '33 livraisons', vote: 'Livraison partielle · 60/40', done: true, why: 'Le critère 4 était clair dans la fiche. Le travail livré reste exploitable.' },
  { id: 'C', rep: '4.9', n: '47 livraisons', vote: 'Vote en attente', done: false, why: 'Délai : 18 h' },
];

const FROZEN = 110;
const PROVIDER_SHARE = 60;

function Position({ ini, title, claim, children }: { ini: string; title: string; claim: string; children: ReactNode }) {
  return (
    <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <Avatar ini={ini} size={28} fontSize={10} />
        <span style={{ flex: 1, fontWeight: 600 }}>{title}</span>
        <span className="mono faint" style={{ fontSize: 12 }}>{claim}</span>
      </div>
      <p className="muted" style={{ margin: 0, fontSize: 13.5, textWrap: 'pretty' }}>{children}</p>
    </div>
  );
}

export function DisputePage() {
  const toProvider = Math.round((FROZEN * PROVIDER_SHARE) / 100);
  const toMe = FROZEN - toProvider;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 16, flexWrap: 'wrap' }}>
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span className="mono faint" style={{ fontSize: 12 }}>EX-0131 · jalon 3 « Refresh token & rotation » · ouvert le 12 oct.</span>
          <h1 className="h1">Litige : API REST FastAPI + auth JWT</h1>
        </div>
        <Pill tone="disp" icon="scale" style={{ height: 26, padding: '0 10px', fontSize: 12, gap: 6 }}>En arbitrage · 2/3 votes</Pill>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px', borderRadius: 12, border: '1px solid var(--bd2)', background: 'var(--s1)' }}>
        <Icon name="lock" style={{ color: 'var(--tx2)' }} />
        <span style={{ flex: 1, fontSize: 13.5 }}>
          <strong style={{ fontWeight: 600 }}>{FROZEN} cr gelés</strong> <span className="muted">jusqu'à la décision. Décision finale au 3e vote, ou dans 18 h à la majorité.</span>
        </span>
        <span className="mono faint" style={{ fontSize: 12 }}>Maintenant · à l'arbitre C</span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <Position ini="SB" title="Ta position" claim="demande : 100 % remboursé">
          Le critère 4 exigeait une rotation des refresh tokens. Le check <span className="mono" style={{ color: 'var(--tx)' }}>test_auth_refresh</span> échoue sur la PR #19, et le jalon 3 portait précisément là-dessus.
        </Position>
        <Position ini="HL" title="Position de Hugo" claim="demande : 100 % libéré">
          La rotation fonctionne en local. Le test échoue à cause d'une horloge CI non mockée, hors périmètre du pack. Jalons 1 et 2 validés sans réserve.
        </Position>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 380px', gap: 24, alignItems: 'start' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <h2 className="h2">Preuves jointes</h2>
            <div style={{ border: '1px solid var(--bd)', borderRadius: 12, overflow: 'hidden' }}>
              {EVIDENCE.map((e) => {
                const c = tone(e.tone);
                return (
                  <a key={e.t} href="#" onClick={(ev) => ev.preventDefault()} className="hov-s2 hov-none" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', borderBottom: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)' }}>
                    <span style={{ width: 30, height: 30, borderRadius: 8, display: 'grid', placeItems: 'center', color: c.fg, background: c.bg }}>
                      <Icon name={e.icon} size={15} />
                    </span>
                    <span style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
                      <span style={{ fontSize: 13.5, fontWeight: 500 }}>{e.t}</span>
                      <span className="mono faint" style={{ fontSize: 12 }}>{e.ref}</span>
                    </span>
                    <span className="faint" style={{ fontSize: 12 }}>{e.by}</span>
                    <Icon name="external-link" size={14} style={{ color: 'var(--tx3)' }} />
                  </a>
                );
              })}
            </div>
          </section>
          <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <h2 className="h2">Vote des arbitres</h2>
            <span className="muted" style={{ fontSize: 12.5, marginTop: -4 }}>3 pairs tirés au sort parmi les membres ≥ 4.5 ★ ayant livré sur cette stack. Anonymes jusqu'à la décision.</span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {ARBITERS.map((a) => (
                <div key={a.id} className="card" style={{ display: 'flex', gap: 12, padding: '14px 16px' }}>
                  <span style={{ width: 30, height: 30, flex: 'none', borderRadius: '50%', border: '1px dashed var(--bd2)', display: 'grid', placeItems: 'center', font: '600 11px var(--mono)', color: 'var(--tx2)' }}>{a.id}</span>
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ fontWeight: 600, color: a.done ? 'var(--tx)' : 'var(--tx3)' }}>{a.vote}</span>
                      {a.done ? <Icon name="check" size={14} style={{ color: 'var(--ok)' }} /> : <Icon name="hourglass" size={14} style={{ color: 'var(--wait)' }} />}
                      <span style={{ flex: 1 }} />
                      <span className="mono faint" style={{ fontSize: 11.5 }}>Arbitre {a.id} · {a.rep} ★ · {a.n}</span>
                    </div>
                    <span className="muted" style={{ fontSize: 13 }}>{a.why}</span>
                  </div>
                </div>
              ))}
            </div>
          </section>
        </div>

        <aside className="card" style={{ position: 'sticky', top: 84, padding: 20, borderColor: 'color-mix(in oklab,var(--disp) 40%,transparent)', display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name="gavel" style={{ color: 'var(--disp)' }} />
            <h2 className="h2" style={{ flex: 1 }}>Décision provisoire</h2>
            <span className="mono faint" style={{ fontSize: 11.5 }}>majorité 2/3</span>
          </div>
          <span style={{ fontSize: 18, fontWeight: 600 }}>Livraison partielle · {PROVIDER_SHARE} / {100 - PROVIDER_SHARE}</span>
          <div role="img" aria-label={`${toProvider} crédits pour Hugo, ${toMe} crédits pour toi`} style={{ display: 'flex', height: 12, borderRadius: 99, overflow: 'hidden', gap: 2 }}>
            <div style={{ width: `${PROVIDER_SHARE}%`, background: 'var(--ac)' }} />
            <div style={{ width: `${100 - PROVIDER_SHARE}%`, background: 'var(--tx3)' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, fontSize: 13.5 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--ac)' }} />
              <span style={{ flex: 1 }}>Libérés à Hugo</span>
              <span style={{ font: '600 14px var(--mono)' }}>{toProvider} cr</span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--tx3)' }} />
              <span style={{ flex: 1 }}>Remboursés à toi</span>
              <span style={{ font: '600 14px var(--mono)' }}>{toMe} cr</span>
            </div>
          </div>
          <div className="muted" style={{ padding: 12, borderRadius: 9, background: 'var(--s2)', fontSize: 12.5, textWrap: 'pretty' }}>
            La décision est publique et motivée. Elle ne compte pas comme « litige perdu » pour une répartition partielle.
          </div>
          <button className="btn btn-secondary" style={{ height: 38, fontSize: 13 }}>
            <Icon name="paperclip" size={14} />
            Ajouter une preuve
          </button>
          <button className="btn btn-primary" style={{ height: 38, fontSize: 13 }}>Accepter la répartition maintenant</button>
        </aside>
      </div>
    </div>
  );
}
