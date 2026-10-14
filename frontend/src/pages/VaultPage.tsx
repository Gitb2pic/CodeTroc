import { useState } from 'react';
import { Avatar, EmptyState, ErrorBanner, Icon, PageTitle, Pill, useEtat } from '../components/ui';
import { PRE_REVOKED, RECEIVED_ACCESS, VAULT, VAULT_LOG } from '../data/fixtures';
import { tone } from '../domain/logic';
import type { VaultAccess } from '../domain/types';
import { useApp } from '../store/AppStore';

const COLS = '40px minmax(0,1.6fr) minmax(0,1fr) 180px 130px';
const LOG_COLS = '110px 150px minmax(0,1fr) 180px 130px';
const LOG_ICON = { ok: 'check', ac: 'plus', fail: 'ban', mute: 'clock', wait: 'clock', disp: 'clock' } as const;

function ScopeTags({ scope }: { scope: string[] }) {
  return (
    <span style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
      {scope.map((sc) => (
        <span key={sc} className="mono muted" style={{ height: 20, padding: '0 6px', display: 'inline-flex', alignItems: 'center', borderRadius: 5, background: 'var(--s2)', border: '1px solid var(--bd)', fontSize: 11 }}>{sc}</span>
      ))}
    </span>
  );
}

function TypeIcon({ icon }: { icon: string }) {
  return (
    <span style={{ width: 40, height: 40, borderRadius: 10, background: 'var(--s3)', display: 'grid', placeItems: 'center' }}>
      <Icon name={icon} size={18} />
    </span>
  );
}

function Stat({ icon, color, value, label }: { icon: string; color: string; value: string; label: string }) {
  return (
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: 14, padding: 16 }}>
      <span style={{ width: 38, height: 38, borderRadius: 10, display: 'grid', placeItems: 'center', color, background: `color-mix(in oklab,${color} 13%,transparent)` }}>
        <Icon name={icon} size={18} />
      </span>
      <span style={{ display: 'flex', flexDirection: 'column' }}>
        <span style={{ font: '600 22px/1.1 var(--mono)' }}>{value}</span>
        <span className="muted" style={{ fontSize: 12.5 }}>{label}</span>
      </span>
    </div>
  );
}

export function VaultPage() {
  const st = useEtat();
  const { revoked, logExtra, revokeAccess } = useApp();
  const [confirm, setConfirm] = useState<string | null>(null);

  const rows = VAULT.map((v) => {
    const pre = PRE_REVOKED.has(v.id);
    const rev = !!revoked[v.id] || pre;
    return { ...v, rev, pre };
  });
  const shared = rows.filter((v) => !RECEIVED_ACCESS.has(v.id));
  const received = rows.filter((v) => RECEIVED_ACCESS.has(v.id));
  const active = rows.filter((v) => !v.rev);
  const log = [...logExtra, ...VAULT_LOG];

  const doRevoke = (v: VaultAccess) => {
    revokeAccess(v.id, v.name, v.to);
    setConfirm(null);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <PageTitle
        title="Coffre d'accès"
        sub="Chaque accès est temporaire, limité et journalisé. Tu peux le couper à tout moment."
        action={
          <button className="btn btn-primary">
            <Icon name="plus" />
            Partager un accès
          </button>
        }
      />
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 12 }}>
        <Stat icon="shield-check" color="var(--ok)" value={String(active.length)} label="accès actifs" />
        <Stat icon="timer" color="var(--wait)" value={String(active.filter((v) => v.warn).length)} label="expire dans moins de 24 h" />
        <Stat icon="ban" color="var(--fail)" value="1" label="tentative bloquée cette semaine" />
      </div>

      <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <h2 className="h2">Accès que tu partages</h2>
        {st.normal && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {shared.map((v) => {
              const c = v.rev ? tone('mute') : v.warn ? tone('wait') : tone('ok');
              const conf = confirm === v.id && !v.rev;
              return (
                <div key={v.id} className="card" style={{ opacity: v.rev ? 0.6 : 1 }}>
                  <div style={{ display: 'grid', gridTemplateColumns: COLS, gap: 16, alignItems: 'center', padding: '14px 16px' }}>
                    <TypeIcon icon={v.icon} />
                    <span style={{ display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0 }}>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                        <span className="ellipsis" style={{ font: '500 13.5px var(--mono)' }}>{v.name}</span>
                        <Pill colors={c} icon={v.rev ? 'ban' : v.warn ? 'timer' : 'shield-check'} style={{ height: 20, padding: '0 7px', fontSize: 11, gap: 4, flex: 'none' }}>
                          {v.rev ? 'Révoqué' : v.warn ? 'Expire bientôt' : 'Actif'}
                        </Pill>
                      </span>
                      <ScopeTags scope={v.scope} />
                    </span>
                    <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
                      <Avatar ini={v.ini} size={24} fontSize={9} />
                      <span style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.3, minWidth: 0 }}>
                        <span style={{ fontSize: 13 }}>{v.to}</span>
                        <span className="mono faint" style={{ fontSize: 11.5 }}>{v.ex} · {v.last}</span>
                      </span>
                    </span>
                    <span style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                      <span className="mono muted" style={{ fontSize: 12 }}>{v.rev ? (v.pre ? 'révoqué il y a 3 j' : "révoqué à l'instant") : `expire dans ${v.left}`}</span>
                      <span style={{ height: 5, borderRadius: 99, background: 'var(--s3)' }}>
                        <span style={{ display: 'block', height: '100%', borderRadius: 99, width: `${v.rev ? 0 : v.pct}%`, background: v.warn ? 'var(--wait)' : 'var(--ac)' }} />
                      </span>
                    </span>
                    <span style={{ display: 'flex', justifyContent: 'flex-end' }}>
                      {v.rev ? (
                        <span className="mono faint" style={{ fontSize: 12 }}>—</span>
                      ) : (
                        !conf && (
                          <button className="btn btn-danger hov-fail" onClick={() => setConfirm(v.id)} aria-label={`Révoquer ${v.name}`} style={{ height: 32, padding: '0 12px', borderRadius: 7, fontSize: 13, gap: 6 }}>
                            <Icon name="ban" size={14} />
                            Révoquer
                          </button>
                        )
                      )}
                    </span>
                  </div>
                  {conf && (
                    <div role="alertdialog" aria-label="Confirmer la révocation" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', borderTop: '1px solid color-mix(in oklab,var(--fail) 35%,transparent)', background: 'color-mix(in oklab,var(--fail) 8%,transparent)', borderRadius: '0 0 12px 12px' }}>
                      <Icon name="triangle-alert" style={{ color: 'var(--fail)' }} />
                      <span style={{ flex: 1, fontSize: 13 }}>
                        <strong style={{ fontWeight: 600 }}>Révoquer cet accès maintenant ?</strong>{' '}
                        <span className="muted">Les sessions ouvertes de {v.to} seront coupées immédiatement. Elle sera notifiée.</span>
                      </span>
                      <button className="btn btn-secondary" onClick={() => setConfirm(null)} style={{ height: 30, padding: '0 12px', borderRadius: 7, fontSize: 12.5 }}>Annuler</button>
                      <button className="btn" onClick={() => doRevoke(v)} autoFocus style={{ height: 30, padding: '0 12px', borderRadius: 7, border: '1px solid var(--fail)', background: 'var(--fail)', color: '#fff', fontSize: 12.5, fontWeight: 600 }}>Révoquer maintenant</button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
        {st.empty && (
          <EmptyState icon="vault" title="Aucun accès partagé" padding="40px 24px">
            Quand un échange nécessite un accès serveur ou dépôt, partage-le d'ici plutôt que par message : il expirera tout seul.
          </EmptyState>
        )}
        {st.loading && (
          <div aria-busy="true" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {shared.map((v) => (
              <div key={v.id} data-pulse="" className="card" style={{ height: 70 }} />
            ))}
          </div>
        )}
        {st.error && (
          <ErrorBanner>
            <strong style={{ fontWeight: 600 }}>Le coffre est momentanément inaccessible.</strong>{' '}
            <span className="muted">Par sécurité, aucun nouvel accès ne peut être ouvert. Les révocations restent possibles par e-mail de secours.</span>
          </ErrorBanner>
        )}
      </section>

      <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <h2 className="h2">Accès que tu as reçus</h2>
        {received.map((v) => (
          <div key={v.id} className="card" style={{ display: 'grid', gridTemplateColumns: COLS, gap: 16, alignItems: 'center', padding: '14px 16px' }}>
            <TypeIcon icon={v.icon} />
            <span style={{ display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0 }}>
              <span style={{ font: '500 13.5px var(--mono)' }}>{v.name}</span>
              <ScopeTags scope={v.scope} />
            </span>
            <span className="muted" style={{ fontSize: 13 }}>De {v.to} · {v.ex}</span>
            <span className="mono muted" style={{ fontSize: 12 }}>expire dans {v.left}</span>
            <span style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button className="btn btn-secondary" style={{ height: 32, padding: '0 12px', borderRadius: 7, fontSize: 13 }}>Rendre l'accès</button>
            </span>
          </div>
        ))}
      </section>

      <section className="card" style={{ overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '14px 18px', borderBottom: '1px solid var(--bd)' }}>
          <Icon name="history" style={{ color: 'var(--tx3)' }} />
          <h2 className="h2" style={{ flex: 1 }}>Journal d'accès</h2>
          <span className="mono faint" style={{ fontSize: 12 }}>conservé 90 jours · exportable</span>
        </div>
        <div role="table" aria-label="Journal d'accès" style={{ display: 'flex', flexDirection: 'column' }}>
          <div role="row" className="mono faint" style={{ display: 'grid', gridTemplateColumns: LOG_COLS, gap: 12, padding: '8px 18px', fontSize: 11, fontWeight: 500, letterSpacing: '.04em', textTransform: 'uppercase', borderBottom: '1px solid var(--bd)' }}>
            <span role="columnheader">Heure</span>
            <span role="columnheader">Qui</span>
            <span role="columnheader">Action</span>
            <span role="columnheader">Ressource</span>
            <span role="columnheader">IP</span>
          </div>
          {log.map((l, i) => (
            <div key={i} role="row" style={{ display: 'grid', gridTemplateColumns: LOG_COLS, gap: 12, alignItems: 'center', padding: '9px 18px', borderBottom: '1px solid var(--bd)', font: '12.5px var(--mono)' }}>
              <span role="cell" className="faint">{l.time}</span>
              <span role="cell" style={{ fontFamily: 'var(--sans)', fontSize: 13 }}>{l.who}</span>
              <span role="cell" style={{ display: 'flex', alignItems: 'center', gap: 6, color: tone(l.tone).fg }}>
                <Icon name={LOG_ICON[l.tone]} size={13} />
                <span style={{ color: 'var(--tx)' }}>{l.act}</span>
              </span>
              <span role="cell" className="muted ellipsis">{l.res}</span>
              <span role="cell" className="faint">{l.ip}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
