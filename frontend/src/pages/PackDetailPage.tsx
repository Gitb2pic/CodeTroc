import type { ReactNode } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Avatar, Icon, Tag } from '../components/ui';
import { GENERIC_CRITERIA, P1_CRITERIA, RECENT_PROOFS, START_BALANCE } from '../data/fixtures';
import { plural, repRing, splitMilestones } from '../domain/logic';
import { ROUTES } from '../routes';
import { findPack, useApp } from '../store/AppStore';

const CRIT_ICON = { CI: 'circle-check', Scan: 'shield-check', Revue: 'eye' };

function Section({ icon, title, extra, children }: { icon: string; title: string; extra?: string; children: ReactNode }) {
  return (
    <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center' }}>
        <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 15, fontWeight: 600, flex: 1 }}>
          <Icon name={icon} style={{ color: 'var(--tx3)' }} />
          {title}
        </h2>
        {extra && <span className="mono faint" style={{ fontSize: 12 }}>{extra}</span>}
      </div>
      {children}
    </section>
  );
}

export function PackDetailPage() {
  const navigate = useNavigate();
  const { packId } = useParams();
  const { requestPack } = useApp();
  const pack = findPack(packId);
  const crit = pack.id === 'p1' ? P1_CRITERIA : GENERIC_CRITERIA;
  const autoCount = crit.filter((c) => c.v !== 'Revue').length;
  const ms = splitMilestones(pack);

  const request = () => {
    requestPack(pack);
    navigate(ROUTES.ex);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <button onClick={() => navigate(ROUTES.cat)} style={{ alignSelf: 'flex-start', display: 'inline-flex', alignItems: 'center', gap: 6, border: 0, background: 'none', color: 'var(--tx2)', fontSize: 13, cursor: 'pointer', padding: 0 }}>
        <Icon name="chevron-left" size={14} />
        Catalogue
      </button>
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 360px', gap: 32, alignItems: 'start' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 28 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {pack.stack.map((t) => (
                <Tag key={t} style={{ height: 22, padding: '0 7px', borderRadius: 6, fontSize: 11.5 }}>{t}</Tag>
              ))}
            </div>
            <h1 style={{ margin: 0, fontSize: 30, lineHeight: 1.15, letterSpacing: '-.03em', fontWeight: 600, textWrap: 'balance' }}>{pack.title}</h1>
            <p className="muted" style={{ margin: 0, fontSize: 15, maxWidth: 640, textWrap: 'pretty' }}>
              {pack.id === 'p1' ? (
                <>
                  Un workflow GitHub Actions prêt pour la prod sur ton dépôt existant : lint et tests sur chaque PR, image Docker publiée sur GHCR, déploiement staging automatique. Tu gardes la main, tout est versionné dans <span className="mono" style={{ color: 'var(--tx)' }}>.github/workflows</span>.
                </>
              ) : (
                'Livré par pull request sur ton dépôt, jalon par jalon. Chaque critère vérifiable est contrôlé par la CI avant que tu valides.'
              )}
            </p>
          </div>

          <Section icon="package" title="Livrable">
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 1, background: 'var(--bd)', border: '1px solid var(--bd)', borderRadius: 10, overflow: 'hidden' }}>
              {[
                ['FORMAT', 'PR sur ton dépôt'],
                ['ACCÈS REQUIS', 'Dépôt write · via le coffre'],
                ['RÉVISIONS', '2 allers-retours inclus'],
              ].map(([k, v]) => (
                <div key={k} style={{ background: 'var(--s1)', padding: 14, display: 'flex', flexDirection: 'column', gap: 4 }}>
                  <span className="mono faint" style={{ fontSize: 11 }}>{k}</span>
                  <span style={{ fontWeight: 500 }}>{v}</span>
                </div>
              ))}
            </div>
          </Section>

          <Section icon="list-checks" title="Critères d'acceptation" extra={`${autoCount}/${crit.length} vérifiés automatiquement`}>
            <div style={{ border: '1px solid var(--bd)', borderRadius: 10, overflow: 'hidden' }}>
              {crit.map((c) => (
                <div key={c.t} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderBottom: '1px solid var(--bd)', background: 'var(--s1)' }}>
                  <Icon name={CRIT_ICON[c.v]} style={{ color: 'var(--tx3)' }} />
                  <span style={{ flex: 1 }}>{c.t}</span>
                  <Tag style={{ borderColor: 'var(--bd2)', background: 'transparent', padding: '0 7px' }}>{c.v}</Tag>
                </div>
              ))}
            </div>
          </Section>

          <Section icon="git-commit-horizontal" title="Jalons">
            <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column' }}>
              {ms.map((m) => (
                <li key={m.n} style={{ display: 'grid', gridTemplateColumns: '28px minmax(0,1fr) auto', gap: 12, alignItems: 'start', paddingBottom: 16 }}>
                  <span style={{ width: 24, height: 24, borderRadius: '50%', border: '1.5px solid var(--bd2)', display: 'grid', placeItems: 'center', font: '600 11px var(--mono)', color: 'var(--tx2)' }}>{m.n}</span>
                  <span style={{ display: 'flex', flexDirection: 'column', gap: 2, paddingTop: 2 }}>
                    <span style={{ fontWeight: 500 }}>{m.t}</span>
                    <span className="faint" style={{ fontSize: 12.5 }}>Libéré quand la PR est mergée, la CI au vert et que tu valides</span>
                  </span>
                  <span style={{ font: '600 13px var(--mono)', color: 'var(--ac-tx)', paddingTop: 3 }}>{m.cr} cr</span>
                </li>
              ))}
            </ol>
          </Section>
        </div>

        <aside style={{ position: 'sticky', top: 84, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 16 }}>
            <span style={{ font: '600 36px/1 var(--mono)', letterSpacing: '-.03em', color: 'var(--ac-tx)' }}>{pack.price} cr</span>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span className="mono faint" style={{ fontSize: 11 }}>DÉLAI</span>
                <span style={{ font: '500 14px var(--mono)' }}>{pack.days} j</span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span className="mono faint" style={{ fontSize: 11 }}>JALONS</span>
                <span style={{ font: '500 14px var(--mono)' }}>{plural(pack.ms, 'jalon')}</span>
              </div>
            </div>
            <div style={{ display: 'flex', gap: 10, padding: 12, borderRadius: 9, background: 'var(--s2)', fontSize: 12.5, color: 'var(--tx2)' }}>
              <Icon name="lock" style={{ marginTop: 2, color: 'var(--ac-tx)' }} />
              <span>Tes crédits sont bloqués en séquestre et libérés jalon par jalon, uniquement sur preuve GitHub vérifiée.</span>
            </div>
            <button className="btn btn-accent hov-bright" onClick={request} style={{ height: 42, borderRadius: 9, fontSize: 14, gap: 8 }}>
              Demander ce pack
              <Icon name="arrow-right" />
            </button>
            <span className="mono faint" style={{ textAlign: 'center', fontSize: 12 }}>Solde après demande : {START_BALANCE - pack.price} cr</span>
          </div>
          <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Avatar ini={pack.ini} size={44} ring={repRing(pack.rep)} fontSize={12} />
              <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
                <span style={{ fontWeight: 600 }}>{pack.name}</span>
                <span className="mono faint" style={{ fontSize: 12 }}>@{pack.handle} · {pack.rep.toFixed(1)} ★</span>
              </span>
              <button className="btn btn-secondary" onClick={() => navigate(ROUTES.profile)} style={{ height: 28, padding: '0 10px', borderRadius: 7, fontSize: 12 }}>Profil</button>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 8, textAlign: 'center' }}>
              {[
                [String(pack.deliv), 'livraisons'],
                ['97%', "à l'heure"],
                ['1', 'litige'],
              ].map(([v, l]) => (
                <div key={l} style={{ padding: 8, borderRadius: 8, background: 'var(--s2)' }}>
                  <div style={{ font: '600 15px var(--mono)' }}>{v}</div>
                  <div className="faint" style={{ fontSize: 11 }}>{l}</div>
                </div>
              ))}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span className="kicker" style={{ marginBottom: 6 }}>Preuves récentes</span>
              {RECENT_PROOFS.map((r) => (
                <a key={r.pr} href="#" className="hov-s2 hov-none" onClick={(e) => e.preventDefault()} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: 8, margin: '0 -8px', borderRadius: 7, color: 'var(--tx)' }}>
                  <Icon name="git-merge" size={14} style={{ color: 'var(--ok)' }} />
                  <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                    <span className="mono ellipsis" style={{ fontSize: 12.5 }}>{r.pr}</span>
                    <span className="faint" style={{ fontSize: 11.5 }}>{r.t} · {r.d}</span>
                  </span>
                  <span style={{ font: '500 12px var(--mono)', color: 'var(--ac-tx)' }}>{r.cr}</span>
                </a>
              ))}
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
