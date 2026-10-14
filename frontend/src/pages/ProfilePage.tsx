import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Avatar, Icon, Tag } from '../components/ui';
import { PACKS, PROFILE_LEA } from '../data/fixtures';
import { plural, repRing } from '../domain/logic';
import { ROUTES } from '../routes';

const TABS = [
  ['Packs', '3'],
  ['Livraisons', '38'],
  ['Avis', '31'],
] as const;

const STATS = [
  ['Réputation', '4.9', ' / 5', 'sur 31 avis liés à des échanges'],
  ['Livraisons vérifiées', '38', '', 'PR mergées · CI au vert'],
  ["À l'heure", '97%', '', '37 jalons sur 38'],
  ['Litiges', '1', '', 'tranché en sa faveur'],
];

export function ProfilePage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<(typeof TABS)[number][0]>('Packs');
  const p = PROFILE_LEA;
  const packs = [PACKS[0], ...p.extraPacks];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'auto minmax(0,1fr) auto', gap: 20, alignItems: 'center', paddingBottom: 24, borderBottom: '1px solid var(--bd)' }}>
        <Avatar ini={p.ini} size={84} ring={repRing(4.9)} fontSize={22} />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <h1 style={{ margin: 0, fontSize: 26, letterSpacing: '-.025em', fontWeight: 600 }}>{p.name}</h1>
            <span className="pill" style={{ gap: 4, color: 'var(--ok)', background: 'color-mix(in oklab,var(--ok) 13%,transparent)', border: '1px solid color-mix(in oklab,var(--ok) 38%,transparent)' }}>
              <Icon name="shield-check" size={12} />
              Identité GitHub vérifiée
            </span>
          </div>
          <span className="mono muted" style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
            <Icon name="github" size={14} />
            <a href={`https://github.com/${p.handle}`} target="_blank" rel="noreferrer">@{p.handle}</a>
            <span>·</span>
            <span>Lyon</span>
            <span>·</span>
            <span>membre depuis mars 2025</span>
          </span>
          <p className="muted" style={{ margin: 0, maxWidth: 620, textWrap: 'pretty' }}>
            DevOps freelance. Je transforme des déploiements manuels en pipelines que tu comprends et que tu peux maintenir seul·e.
          </p>
        </div>
        <button className="btn btn-secondary" style={{ fontSize: 13 }}>Envoyer un message</button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: 12 }}>
        {STATS.map(([k, v, suffix, sub]) => (
          <div key={k} className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 6 }}>
            <span className="kicker">{k}</span>
            <span style={{ font: '600 28px/1 var(--mono)' }}>
              {v}
              {suffix && <span className="faint" style={{ fontSize: 14 }}>{suffix}</span>}
            </span>
            <span className="muted" style={{ fontSize: 12 }}>{sub}</span>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <span className="kicker">Compétences prouvées · livraisons par stack</span>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {p.skills.map(([n, c]) => (
            <span key={n} className="card" style={{ display: 'inline-flex', alignItems: 'center', gap: 8, height: 28, padding: '0 10px', borderRadius: 7, font: '500 12.5px var(--mono)' }}>
              {n}
              <span style={{ color: 'var(--ok)' }}>{c} ✓</span>
            </span>
          ))}
        </div>
      </div>

      <div role="tablist" style={{ display: 'flex', gap: 20, borderBottom: '1px solid var(--bd)' }}>
        {TABS.map(([t, n]) => {
          const on = tab === t;
          return (
            <button key={t} role="tab" aria-selected={on} onClick={() => setTab(t)} style={{ height: 40, padding: '0 2px', border: 0, borderBottom: `2px solid ${on ? 'var(--tx)' : 'transparent'}`, marginBottom: -1, background: 'none', color: on ? 'var(--tx)' : 'var(--tx2)', font: '500 13.5px var(--sans)', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6 }}>
              {t}
              <span className="mono faint" style={{ fontSize: 11 }}>{n}</span>
            </button>
          );
        })}
      </div>

      {tab === 'Packs' && (
        <div role="tabpanel" style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 12 }}>
          {packs.map((c) => (
            <button key={c.id} className="card hov-bd2" onClick={() => navigate(ROUTES.pack('p1'))} style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: 16, color: 'var(--tx)', textAlign: 'left', cursor: 'pointer' }}>
              <span style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
                {c.stack.map((t) => (
                  <Tag key={t}>{t}</Tag>
                ))}
              </span>
              <span style={{ fontSize: 15, fontWeight: 600, lineHeight: 1.3, minHeight: 40 }}>{c.title}</span>
              <span className="mono muted" style={{ display: 'flex', alignItems: 'center', gap: 14, fontSize: 12, paddingTop: 12, borderTop: '1px solid var(--bd)', width: '100%' }}>
                <span>{c.days} j</span>
                <span>{plural(c.ms, 'jalon')}</span>
                <span style={{ flex: 1 }} />
                <span style={{ font: '600 16px var(--mono)', color: 'var(--ac-tx)' }}>{c.price} cr</span>
              </span>
            </button>
          ))}
        </div>
      )}
      {tab === 'Livraisons' && (
        <div role="tabpanel" style={{ border: '1px solid var(--bd)', borderRadius: 12, overflow: 'hidden' }}>
          {p.deliveries.map((d) => (
            <a key={d.pr} href="#" onClick={(e) => e.preventDefault()} className="hov-s2 hov-none" style={{ display: 'grid', gridTemplateColumns: '20px minmax(0,1fr) 120px 90px 80px', gap: 12, alignItems: 'center', padding: '12px 16px', borderBottom: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)' }}>
              <Icon name="git-merge" style={{ color: 'var(--ac-tx)' }} />
              <span style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
                <span style={{ fontSize: 13.5, fontWeight: 500 }}>{d.t}</span>
                <span className="mono faint" style={{ fontSize: 12 }}>{d.pr} · {d.ex} · {d.d}</span>
              </span>
              <span className="pill" style={{ justifySelf: 'start', color: 'var(--ok)', background: 'color-mix(in oklab,var(--ok) 13%,transparent)', border: '1px solid color-mix(in oklab,var(--ok) 38%,transparent)' }}>
                <Icon name="circle-check" size={12} />
                CI {d.ci}
              </span>
              <span style={{ font: '600 13px var(--mono)', color: 'var(--ac-tx)' }}>{d.cr}</span>
              <Icon name="external-link" size={14} style={{ justifySelf: 'end', color: 'var(--tx3)' }} />
            </a>
          ))}
        </div>
      )}
      {tab === 'Avis' && (
        <div role="tabpanel" style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 12 }}>
          {p.reviews.map((r) => (
            <figure key={r.ex} className="card" style={{ margin: 0, padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
              <blockquote style={{ margin: 0, fontSize: 14, textWrap: 'pretty' }}>« {r.t} »</blockquote>
              <figcaption style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 'auto' }}>
                <Avatar ini={r.ini} size={24} fontSize={9} />
                <span style={{ flex: 1, fontSize: 12.5 }}>{r.who}</span>
                <a href="#" onClick={(e) => e.preventDefault()} className="mono" style={{ fontSize: 11.5 }}>{r.ex}</a>
                <span style={{ font: '500 12px var(--mono)' }}>{r.stars}</span>
              </figcaption>
            </figure>
          ))}
        </div>
      )}
    </div>
  );
}
