import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Avatar, EmptyState, EscrowBar, Icon, Pill } from '../components/ui';
import { ME, MILESTONE_DUE } from '../data/fixtures';
import { exchangeAmounts, milestoneState, nowCard, percent, repRing, splitMilestones, tone } from '../domain/logic';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';

interface Msg {
  sys: boolean;
  me?: boolean;
  who?: string;
  ini?: string;
  t: string;
  time: string;
  icon?: string;
}

const smallBtn = { height: 26, padding: '0 10px', borderRadius: 6, fontSize: 12, fontFamily: 'var(--mono)', gap: 5 } as const;

export function ExchangePage() {
  const navigate = useNavigate();
  const { exPack: pack, flow, sent, simulateAccept, simulateDelivery, validateMilestone, replay, sendMessage } = useApp();
  const [chat, setChat] = useState('');
  const first = pack.name.split(' ')[0];
  const ms = splitMilestones(pack);
  const amounts = exchangeAmounts(pack, flow);
  const now = nowCard(pack, flow);

  if (flow === 0) {
    return (
      <EmptyState icon="arrow-left-right" title="Pas encore d'échange ouvert" padding="64px 24px" actions={<button className="btn btn-primary" onClick={() => navigate(ROUTES.cat)} style={{ fontSize: 13 }}>Ouvrir le catalogue</button>}>
        Choisis un pack dans le catalogue et clique sur « Demander ce pack ».
      </EmptyState>
    );
  }

  const ciRunning = flow === 3;
  const ciBadge = ciRunning ? { l: 'CI en cours', icon: 'loader-circle', spin: true, c: tone('wait') } : { l: 'CI réussie · 3/3', icon: 'circle-check', spin: false, c: tone('ok') };
  const checks = [
    ['lint', '18s'],
    ['test (node 18)', '1m02'],
    ['test (node 20)', '58s'],
  ].map(([name, dur], i) => {
    const run = ciRunning && i > 0;
    return { name, dur: run ? 'en cours' : dur, icon: run ? 'loader-circle' : 'circle-check', spin: run, fg: run ? 'var(--wait)' : 'var(--ok)', label: run ? 'En cours' : 'Réussi' };
  });

  const msgs: Msg[] = [];
  msgs.push({ sys: true, t: `Demande envoyée. ${amounts.total} cr bloqués en séquestre.`, time: 'lun. 09:12', icon: 'lock' });
  if (flow >= 2) {
    msgs.push({ sys: true, t: `${first} a accepté la demande. Accès au dépôt partagé via le coffre (72 h).`, time: 'lun. 10:40', icon: 'key-round' });
    msgs.push({ sys: false, who: first, ini: pack.ini, t: "Salut ! J'ai bien l'accès au dépôt. Je commence par le workflow de base, PR prévue demain.", time: 'lun. 10:42' });
    msgs.push({ sys: false, me: true, who: 'Toi', ini: ME.ini, t: 'Top. Les tests sont dans /test, `npm test` suffit. Vise Node 18 et 20 stp.', time: 'lun. 10:55' });
  }
  if (flow >= 3) msgs.push({ sys: true, t: `${first} a ouvert la PR #42 sur acme/shop-api`, time: 'mar. 16:08', icon: 'git-pull-request' });
  if (flow >= 4) msgs.push({ sys: true, t: 'CI réussie · 3/3 checks · PR #42 mergée', time: 'mar. 16:11', icon: 'circle-check' });
  if (flow >= 5) msgs.push({ sys: true, t: `Tu as validé le jalon 1. ${amounts.first} cr libérés à ${first}.`, time: 'mar. 16:20', icon: 'lock-open' });
  sent.forEach((t) => msgs.push({ sys: false, me: true, who: 'Toi', ini: ME.ini, t, time: "à l'instant" }));

  const send = (e: FormEvent) => {
    e.preventDefault();
    sendMessage(chat);
    setChat('');
  };

  const curCr = flow >= 5 && ms[1] ? ms[1].cr : ms[0].cr;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px', borderRadius: 9, border: '1px dashed var(--bd2)', font: '12px var(--mono)', color: 'var(--tx3)' }}>
        <Icon name="play" size={13} />
        <span style={{ flex: 1 }}>Prototype · simule les actions de l'autre partie</span>
        {flow === 1 && <button className="btn btn-secondary" onClick={simulateAccept} style={smallBtn}>{first} accepte →</button>}
        {flow === 2 && <button className="btn btn-secondary" onClick={simulateDelivery} style={smallBtn}>{first} ouvre la PR #42 →</button>}
        {flow === 3 && <span style={{ color: 'var(--wait)' }}>CI en cours…</span>}
        {flow >= 5 && (
          <button className="btn btn-secondary" onClick={() => { replay(); navigate(ROUTES.cat); }} style={smallBtn}>
            <Icon name="rotate-ccw" size={12} />
            Rejouer le parcours
          </button>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap' }}>
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className="mono faint" style={{ fontSize: 12 }}>EX-0142 · ouvert le 14 oct.</span>
          <h1 className="h1">{pack.title}</h1>
        </div>
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <span title="Toi"><Avatar ini={ME.ini} size={32} border="2px solid var(--bg)" fontSize={10} /></span>
          <span title={pack.name} style={{ marginLeft: -8 }}><Avatar ini={pack.ini} size={32} border="2px solid var(--bg)" fontSize={10} /></span>
        </div>
        <span className="muted" style={{ fontSize: 13 }}>Toi ⇄ {pack.name}</span>
      </div>

      {now.you ? (
        <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '16px 18px', borderRadius: 12, border: '1px solid var(--ac)', background: 'color-mix(in oklab,var(--ac) 12%,var(--s1))' }}>
          <span style={{ width: 36, height: 36, borderRadius: '50%', background: 'var(--ac)', color: 'var(--on-ac)', display: 'grid', placeItems: 'center', font: '700 11px var(--mono)', flex: 'none' }}>{ME.ini}</span>
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span className="kicker" style={{ color: 'var(--ac-tx)' }}>Maintenant · à toi</span>
            <span style={{ fontSize: 16, fontWeight: 600 }}>{now.t}</span>
            <span className="muted" style={{ fontSize: 13 }}>{now.sub}</span>
          </div>
        </div>
      ) : (
        <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '16px 18px', borderRadius: 12, border: '1px solid var(--bd2)', background: 'var(--s1)' }}>
          <Avatar ini={now.ini} size={36} fontSize={11} />
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span className="kicker">Maintenant · à {now.who}</span>
            <span style={{ fontSize: 16, fontWeight: 600 }}>{now.t}</span>
            <span className="muted" style={{ fontSize: 13 }}>{now.sub}</span>
          </div>
          <span className="mono faint" style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: 12 }}>
            <Icon name="clock" size={13} />
            Rien à faire pour toi
          </span>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 340px', gap: 24, alignItems: 'start' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <section className="card" style={{ padding: 20 }}>
            <h2 className="h2" style={{ marginBottom: 18 }}>Jalons</h2>
            <ol style={{ margin: 0, padding: 0, listStyle: 'none' }}>
              {ms.map((m, i) => {
                const s = milestoneState(i, flow);
                const c = tone(s.tone);
                const spin = s.icon === 'loader-circle';
                const hasProof = i === 0 && flow >= 3;
                return (
                  <li key={m.n} style={{ display: 'grid', gridTemplateColumns: '28px minmax(0,1fr)', gap: 14 }}>
                    <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                      <span style={{ width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', flex: 'none', color: c.fg, background: c.bg, border: `1px solid ${c.bd}` }}>
                        <Icon name={s.icon} size={15} spin={spin} />
                      </span>
                      <span style={{ flex: 1, width: 2, minHeight: 18, background: i === 0 && flow >= 5 ? 'var(--ok)' : 'var(--bd2)', margin: '4px 0' }} />
                    </span>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '3px 0 20px', minWidth: 0 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                        <span className="mono faint" style={{ fontSize: 12 }}>J{m.n}</span>
                        <span style={{ fontWeight: 600, flex: 1, minWidth: 0 }}>{m.t}</span>
                        <Pill colors={c} icon={s.icon}>{s.label}</Pill>
                        <span style={{ font: '600 13px var(--mono)', color: 'var(--ac-tx)' }}>{m.cr} cr</span>
                      </div>
                      <span className="mono faint" style={{ fontSize: 12 }}>échéance {MILESTONE_DUE[i]}</span>
                      {hasProof && (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: 12, borderRadius: 10, border: '1px solid var(--bd)', background: 'var(--bg)' }}>
                          <a href="#" className="hov-none" onClick={(e) => e.preventDefault()} style={{ display: 'flex', alignItems: 'center', gap: 10, color: 'var(--tx)' }}>
                            {flow >= 4 ? <Icon name="git-merge" style={{ color: 'var(--ac-tx)' }} /> : <Icon name="git-pull-request" style={{ color: 'var(--ok)' }} />}
                            <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                              <span style={{ fontSize: 13.5, fontWeight: 500 }}>ci: workflow lint + tests</span>
                              <span className="mono faint" style={{ fontSize: 12 }}>acme/shop-api #42 · feat/ci → main · +184 −3</span>
                            </span>
                            <Pill colors={ciBadge.c} icon={ciBadge.icon} spin={ciBadge.spin}>{ciBadge.l}</Pill>
                            <Icon name="external-link" size={14} style={{ color: 'var(--tx3)' }} />
                          </a>
                          <div style={{ display: 'flex', flexDirection: 'column', borderTop: '1px solid var(--bd)', paddingTop: 8, gap: 2 }}>
                            {checks.map((ck) => (
                              <div key={ck.name} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0', font: '12.5px var(--mono)' }}>
                                <Icon name={ck.icon} size={14} spin={ck.spin} style={{ color: ck.fg }} />
                                <span style={{ flex: 1 }}>{ck.name}</span>
                                <span style={{ color: ck.fg }}>{ck.label}</span>
                                <span className="faint" style={{ width: 64, textAlign: 'right' }}>{ck.dur}</span>
                              </div>
                            ))}
                          </div>
                          <div className="mono faint" style={{ display: 'flex', gap: 16, fontSize: 12, borderTop: '1px solid var(--bd)', paddingTop: 8 }}>
                            <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><Icon name="git-commit-horizontal" size={13} /><a href="#" onClick={(e) => e.preventDefault()}>a3f9c1e</a> ci: add lint and test jobs</span>
                            <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><Icon name="git-commit-horizontal" size={13} /><a href="#" onClick={(e) => e.preventDefault()}>7be20d4</a> ci: cache npm deps</span>
                          </div>
                        </div>
                      )}
                    </div>
                  </li>
                );
              })}
            </ol>
          </section>

          <section className="card" style={{ overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '14px 18px', borderBottom: '1px solid var(--bd)' }}>
              <Icon name="message-square" style={{ color: 'var(--tx3)' }} />
              <h2 className="h2">Discussion</h2>
            </div>
            <div aria-live="polite" style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: 18 }}>
              {msgs.map((m, i) =>
                m.sys ? (
                  <div key={i} className="mono faint" style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12 }}>
                    <span style={{ flex: 1, height: 1, background: 'var(--bd)' }} />
                    <Icon name={m.icon!} size={13} />
                    <span>{m.t}</span>
                    <span>· {m.time}</span>
                    <span style={{ flex: 1, height: 1, background: 'var(--bd)' }} />
                  </div>
                ) : (
                  <div key={i} style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
                    <Avatar ini={m.ini!} size={28} fontSize={10} />
                    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4, padding: '10px 12px', borderRadius: 10, border: '1px solid var(--bd)', background: m.me ? 'color-mix(in oklab, var(--ac) 10%, var(--s1))' : 'var(--s1)' }}>
                      <span style={{ fontSize: 12.5 }}>
                        <strong style={{ fontWeight: 600 }}>{m.who}</strong> <span className="mono faint" style={{ fontSize: 11.5 }}>{m.time}</span>
                      </span>
                      <span style={{ fontSize: 13.5, textWrap: 'pretty' }}>{m.t}</span>
                    </div>
                  </div>
                ),
              )}
            </div>
            <form onSubmit={send} style={{ display: 'flex', gap: 8, padding: '12px 18px', borderTop: '1px solid var(--bd)' }}>
              <button type="button" aria-label="Joindre un fichier" style={{ width: 36, height: 36, display: 'grid', placeItems: 'center', borderRadius: 8, border: '1px solid var(--bd2)', background: 'var(--s2)', color: 'var(--tx2)', cursor: 'pointer' }}>
                <Icon name="paperclip" />
              </button>
              <input className="field" value={chat} onChange={(e) => setChat(e.target.value)} placeholder={`Écris à ${first}…`} aria-label="Message" style={{ flex: 1, height: 36, fontSize: 13.5 }} />
              <button type="submit" className="btn btn-primary" style={{ height: 36, fontSize: 13, gap: 6 }}>
                <Icon name="send" size={14} />
                Envoyer
              </button>
            </form>
          </section>
        </div>

        <aside style={{ position: 'sticky', top: 84, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Icon name="lock" style={{ color: 'var(--ac-tx)' }} />
              <h2 className="h2" style={{ flex: 1 }}>Séquestre</h2>
              <span style={{ font: '600 14px var(--mono)' }}>{amounts.total} cr</span>
            </div>
            <span role="img" aria-label={`Libéré ${amounts.released} cr, bloqué ${amounts.locked} cr`}>
              <EscrowBar released={percent(amounts.released, amounts.total)} current={percent(amounts.first, amounts.total)} />
            </span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 13 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--ac)' }} />
                <span className="muted" style={{ flex: 1 }}>Libéré</span>
                <span className="mono">{amounts.released} cr</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ width: 10, height: 10, borderRadius: 3, background: 'repeating-linear-gradient(135deg,var(--ac) 0 2px,color-mix(in oklab,var(--ac) 35%,transparent) 2px 4px)' }} />
                <span className="muted" style={{ flex: 1 }}>Jalon en cours</span>
                <span className="mono">{curCr} cr</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ width: 10, height: 10, borderRadius: 3, background: 'var(--s3)', border: '1px solid var(--bd2)', boxSizing: 'border-box' }} />
                <span className="muted" style={{ flex: 1 }}>Total bloqué</span>
                <span className="mono">{amounts.locked} cr</span>
              </div>
            </div>
            {flow === 4 ? (
              <>
                <button className="btn btn-accent hov-bright" onClick={validateMilestone} style={{ height: 42, borderRadius: 9, fontSize: 14, gap: 8 }}>
                  <Icon name="lock-open" />
                  Valider le jalon 1 · libérer {amounts.first} cr
                </button>
                <span className="faint" style={{ fontSize: 12, textAlign: 'center' }}>Lié à la PR #42 · a3f9c1e · CI 3/3</span>
              </>
            ) : (
              <button disabled aria-disabled="true" className="btn btn-secondary" style={{ height: 42, borderRadius: 9, color: 'var(--tx3)', gap: 8 }}>
                <Icon name="lock" />
                Validation dispo après preuve
              </button>
            )}
            {flow >= 5 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 12px', borderRadius: 9, border: '1px solid color-mix(in oklab,var(--ok) 35%,transparent)', background: 'color-mix(in oklab,var(--ok) 10%,transparent)', fontSize: 13 }}>
                <Icon name="circle-check" style={{ color: 'var(--ok)' }} />
                <span style={{ flex: 1 }}>{amounts.first} cr libérés à {first}</span>
                <a href="#" onClick={(e) => e.preventDefault()} className="mono" style={{ fontSize: 12 }}>PR #42</a>
              </div>
            )}
          </div>
          <div className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Icon name="vault" style={{ color: 'var(--tx3)' }} />
              <h2 className="h2" style={{ flex: 1 }}>Accès partagés</h2>
              <button onClick={() => navigate(ROUTES.vault)} style={{ border: 0, background: 'none', color: 'var(--ac-tx)', fontSize: 12.5, cursor: 'pointer' }}>Gérer</button>
            </div>
            {[
              ['folder-git-2', 'acme/shop-api · write', '5 j', false],
              ['server', 'staging-01 · deploy', '2 j', false],
              ['key-round', 'GHCR_PUSH_TOKEN', '23 h', true],
            ].map(([icon, label, left, warn]) => (
              <div key={String(label)} style={{ display: 'flex', alignItems: 'center', gap: 8, font: '12.5px var(--mono)' }}>
                <Icon name={String(icon)} size={14} style={{ color: warn ? 'var(--wait)' : 'var(--tx3)' }} />
                <span style={{ flex: 1 }}>{label}</span>
                <span style={{ color: warn ? 'var(--wait)' : 'var(--tx3)' }}>{left}</span>
              </div>
            ))}
          </div>
          <div className="card" style={{ padding: 16, display: 'flex', alignItems: 'center', gap: 10 }}>
            <Avatar ini={pack.ini} size={32} ring={repRing(pack.rep)} fontSize={9} />
            <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
              <span style={{ fontSize: 13, fontWeight: 500 }}>{pack.name}</span>
              <span className="mono faint" style={{ fontSize: 11.5 }}>@{pack.handle} · {pack.rep.toFixed(1)} ★</span>
            </span>
          </div>
          <button className="btn" onClick={() => navigate(ROUTES.dispute)} style={{ height: 36, fontSize: 13, border: '1px solid color-mix(in oklab,var(--disp) 40%,transparent)', background: 'transparent', color: 'var(--disp)' }}>
            <Icon name="flag" size={14} />
            Ouvrir un litige
          </button>
          <span className="faint" style={{ fontSize: 11.5, textAlign: 'center', textWrap: 'pretty' }}>Le séquestre est gelé pendant l'arbitrage. 3 pairs tranchent sous 72 h.</span>
        </aside>
      </div>
    </div>
  );
}
