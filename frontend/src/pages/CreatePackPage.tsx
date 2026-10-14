import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { PackCard } from '../components/PackCard';
import { Icon } from '../components/ui';
import { ME } from '../data/fixtures';
import type { Criterion } from '../domain/types';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';

interface Draft {
  title: string;
  stack: string;
  desc: string;
  kind: string;
  criteria: Criterion[];
  newCrit: string;
  ms: { t: string; cr: number }[];
  price: number;
  days: number;
}

const INITIAL: Draft = {
  title: 'Pipeline CI/CD GitHub Actions pour app Node',
  stack: 'GitHub Actions, Node.js, Docker',
  desc: "Un workflow complet : lint, tests, build d'image Docker et déploiement staging à chaque merge sur main.",
  kind: 'PR sur ton dépôt',
  criteria: [
    { t: 'Lint et tests lancés sur chaque PR', v: 'CI' },
    { t: 'Image Docker publiée sur GHCR, taguée par commit', v: 'CI' },
    { t: 'Déploiement staging après merge sur main', v: 'CI' },
    { t: 'README de mise en route', v: 'Revue' },
  ],
  newCrit: '',
  ms: [
    { t: 'Workflow CI de base', cr: 40 },
    { t: 'Build & push image', cr: 40 },
    { t: 'Déploiement staging + doc', cr: 40 },
  ],
  price: 120,
  days: 5,
};

const STEPS = ['Livrable', 'Critères', 'Jalons', 'Prix'];
const KINDS = ['PR sur ton dépôt', 'Nouveau dépôt', 'Config / infra'];

const label = { display: 'flex', flexDirection: 'column', gap: 6 } as const;
const labelText = { fontSize: 13, fontWeight: 500 } as const;
const hint = { fontSize: 12, color: 'var(--tx3)' } as const;

export function CreatePackPage() {
  const navigate = useNavigate();
  const { showToast } = useApp();
  const [step, setStep] = useState(0);
  const [d, setD] = useState<Draft>(INITIAL);
  const patch = (p: Partial<Draft>) => setD((cur) => ({ ...cur, ...p }));

  const msTotal = d.ms.reduce((a, m) => a + (m.cr || 0), 0);
  const msOk = msTotal === d.price;
  const tags = d.stack.split(',').map((x) => x.trim()).filter(Boolean);
  const ciCount = d.criteria.filter((c) => c.v === 'CI').length;

  const addCrit = (e: FormEvent) => {
    e.preventDefault();
    if (d.newCrit.trim()) patch({ criteria: [...d.criteria, { t: d.newCrit.trim(), v: 'CI' }], newCrit: '' });
  };
  const updateMs = (i: number, p: Partial<{ t: string; cr: number }>) => patch({ ms: d.ms.map((m, j) => (j === i ? { ...m, ...p } : m)) });

  const publish = () => {
    navigate(ROUTES.profile);
    showToast('Pack publié · visible dans le catalogue', 'ok');
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <h1 className="h1">Proposer un pack</h1>
        <span className="muted">Plus tes critères sont vérifiables par la CI, plus ton pack inspire confiance.</span>
      </div>
      <ol aria-label="Étapes" style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', gap: 6, alignItems: 'center' }}>
        {STEPS.map((l, i) => {
          const on = step === i;
          const done = step > i;
          return (
            <li key={l} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <button onClick={() => setStep(i)} aria-current={on ? 'step' : undefined} style={{ display: 'flex', alignItems: 'center', gap: 8, height: 32, padding: '0 12px 0 6px', borderRadius: 99, border: 0, background: 'none', color: on ? 'var(--tx)' : done ? 'var(--tx2)' : 'var(--tx3)', font: '500 13px var(--sans)', cursor: 'pointer' }}>
                <span style={{ width: 22, height: 22, borderRadius: '50%', display: 'grid', placeItems: 'center', font: '600 11px var(--mono)', background: on ? 'var(--tx)' : done ? 'var(--s3)' : 'transparent', color: on ? 'var(--bg)' : 'var(--tx2)', border: `1px solid ${on ? 'var(--tx)' : 'var(--bd2)'}` }}>{done ? '✓' : i + 1}</span>
                {l}
              </button>
              <span style={{ width: 28, height: 1, background: 'var(--bd2)' }} />
            </li>
          );
        })}
      </ol>
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 340px', gap: 32, alignItems: 'start' }}>
        <div className="card" style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 20 }}>
          {step === 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
              <label style={label}>
                <span style={labelText}>Titre du pack</span>
                <input className="field" value={d.title} onChange={(e) => patch({ title: e.target.value })} />
                <span style={hint}>Un verbe + un résultat concret. Ex : « Dockeriser une app Next.js + Postgres ».</span>
              </label>
              <label style={label}>
                <span style={labelText}>Stack</span>
                <input className="field mono" value={d.stack} onChange={(e) => patch({ stack: e.target.value })} style={{ fontSize: 13 }} />
                <span style={hint}>Séparées par des virgules.</span>
              </label>
              <label style={label}>
                <span style={labelText}>Ce que la personne reçoit</span>
                <textarea className="field" value={d.desc} onChange={(e) => patch({ desc: e.target.value })} rows={4} style={{ height: 'auto', padding: '10px 12px', lineHeight: 1.5, resize: 'vertical' }} />
              </label>
              <div style={label}>
                <span style={labelText} id="kind-label">Format du livrable</span>
                <div style={{ display: 'flex', gap: 8 }} role="radiogroup" aria-labelledby="kind-label">
                  {KINDS.map((k) => {
                    const on = d.kind === k;
                    return (
                      <button key={k} role="radio" aria-checked={on} onClick={() => patch({ kind: k })} style={{ flex: 1, height: 38, borderRadius: 8, border: `1px solid ${on ? 'var(--tx2)' : 'var(--bd)'}`, background: on ? 'var(--s3)' : 'var(--s1)', color: 'var(--tx)', font: '500 13px var(--sans)', cursor: 'pointer' }}>
                        {k}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>
          )}

          {step === 1 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div style={{ display: 'flex', gap: 10, padding: 12, borderRadius: 9, background: 'var(--s2)', fontSize: 13, color: 'var(--tx2)' }}>
                <Icon name="info" style={{ marginTop: 2 }} />
                <span>
                  Un critère <span className="mono" style={{ color: 'var(--tx)' }}>CI</span> est vérifié automatiquement par un check. Un critère <span className="mono" style={{ color: 'var(--tx)' }}>Revue</span> demande une validation humaine. Clique sur l'étiquette pour basculer.
                </span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', border: '1px solid var(--bd)', borderRadius: 10, overflow: 'hidden' }}>
                {d.criteria.map((c, i) => (
                  <div key={`${i}-${c.t}`} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderBottom: '1px solid var(--bd)' }}>
                    <Icon name={c.v === 'CI' ? 'circle-check' : 'eye'} style={{ color: 'var(--tx3)' }} />
                    <span style={{ flex: 1, fontSize: 13.5 }}>{c.t}</span>
                    <button
                      onClick={() => patch({ criteria: d.criteria.map((x, j) => (j === i ? { t: x.t, v: x.v === 'CI' ? 'Revue' : 'CI' } : x)) })}
                      aria-label={`Type de vérification : ${c.v}. Basculer`}
                      style={{ height: 22, padding: '0 8px', borderRadius: 5, border: '1px solid var(--bd2)', background: 'var(--s2)', color: 'var(--tx)', font: '500 11px var(--mono)', cursor: 'pointer' }}
                    >
                      {c.v}
                    </button>
                    <button onClick={() => patch({ criteria: d.criteria.filter((_, j) => j !== i) })} aria-label="Supprimer le critère" style={{ width: 26, height: 26, display: 'grid', placeItems: 'center', border: 0, borderRadius: 6, background: 'none', color: 'var(--tx3)', cursor: 'pointer' }}>
                      <Icon name="x" size={14} />
                    </button>
                  </div>
                ))}
              </div>
              <form onSubmit={addCrit} style={{ display: 'flex', gap: 8 }}>
                <input className="field" value={d.newCrit} onChange={(e) => patch({ newCrit: e.target.value })} placeholder="Ajoute un critère vérifiable…" aria-label="Nouveau critère" style={{ flex: 1, height: 36, fontSize: 13.5 }} />
                <button type="submit" className="btn btn-secondary" style={{ height: 36, padding: '0 12px', fontSize: 13, gap: 6 }}>
                  <Icon name="plus" />
                  Ajouter
                </button>
              </form>
            </div>
          )}

          {step === 2 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              {d.ms.map((m, i) => (
                <div key={i} style={{ display: 'grid', gridTemplateColumns: '28px minmax(0,1fr) 110px 30px', gap: 10, alignItems: 'center' }}>
                  <span style={{ width: 24, height: 24, borderRadius: '50%', border: '1.5px solid var(--bd2)', display: 'grid', placeItems: 'center', font: '600 11px var(--mono)', color: 'var(--tx2)' }}>{i + 1}</span>
                  <input className="field" value={m.t} onChange={(e) => updateMs(i, { t: e.target.value })} aria-label={`Titre du jalon ${i + 1}`} style={{ height: 36, fontSize: 13.5 }} />
                  <label style={{ display: 'flex', alignItems: 'center', height: 36, padding: '0 10px', borderRadius: 8, border: '1px solid var(--bd2)', background: 'var(--bg)', gap: 4, boxSizing: 'border-box' }}>
                    <input type="number" min={0} value={m.cr} onChange={(e) => updateMs(i, { cr: +e.target.value })} aria-label={`Crédits du jalon ${i + 1}`} style={{ width: '100%', border: 0, background: 'none', color: 'var(--ac-tx)', font: '500 13.5px var(--mono)', outline: 'none' }} />
                    <span className="mono faint" style={{ fontSize: 12 }}>cr</span>
                  </label>
                  <button onClick={() => patch({ ms: d.ms.filter((_, j) => j !== i) })} aria-label={`Supprimer le jalon ${i + 1}`} style={{ width: 30, height: 30, display: 'grid', placeItems: 'center', border: 0, borderRadius: 6, background: 'none', color: 'var(--tx3)', cursor: 'pointer' }}>
                    <Icon name="trash-2" size={14} />
                  </button>
                </div>
              ))}
              <button onClick={() => patch({ ms: [...d.ms, { t: 'Nouveau jalon', cr: 0 }] })} style={{ alignSelf: 'flex-start', display: 'inline-flex', alignItems: 'center', gap: 6, height: 32, padding: '0 12px', borderRadius: 8, border: '1px dashed var(--bd2)', background: 'none', color: 'var(--tx2)', font: '500 13px var(--sans)', cursor: 'pointer' }}>
                <Icon name="plus" />
                Ajouter un jalon
              </button>
              {msOk ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, font: '12.5px var(--mono)', color: 'var(--ok)' }}>
                  <Icon name="circle-check" size={14} />
                  Total {msTotal} / {d.price} cr · réparti
                </div>
              ) : (
                <div role="alert" style={{ display: 'flex', alignItems: 'center', gap: 8, font: '12.5px var(--mono)', color: 'var(--wait)' }}>
                  <Icon name="triangle-alert" size={14} />
                  Total {msTotal} / {d.price} cr · {msTotal > d.price ? `${msTotal - d.price} cr de trop` : `${d.price - msTotal} cr à répartir`}
                </div>
              )}
            </div>
          )}

          {step === 3 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
                <label style={label}>
                  <span style={labelText}>Prix</span>
                  <span style={{ display: 'flex', alignItems: 'center', height: 44, padding: '0 12px', borderRadius: 8, border: '1px solid var(--bd2)', background: 'var(--bg)' }}>
                    <input type="number" min={1} value={d.price} onChange={(e) => patch({ price: +e.target.value })} style={{ flex: 1, width: '100%', border: 0, background: 'none', color: 'var(--ac-tx)', font: '600 20px var(--mono)', outline: 'none' }} />
                    <span className="mono faint" style={{ fontSize: 14 }}>cr</span>
                  </span>
                </label>
                <label style={label}>
                  <span style={labelText}>Délai</span>
                  <span style={{ display: 'flex', alignItems: 'center', height: 44, padding: '0 12px', borderRadius: 8, border: '1px solid var(--bd2)', background: 'var(--bg)' }}>
                    <input type="number" min={1} value={d.days} onChange={(e) => patch({ days: +e.target.value })} style={{ flex: 1, width: '100%', border: 0, background: 'none', color: 'var(--tx)', font: '600 20px var(--mono)', outline: 'none' }} />
                    <span className="mono faint" style={{ fontSize: 14 }}>jours</span>
                  </span>
                </label>
              </div>
              {!msOk && (
                <div id="ms-mismatch" role="alert" style={{ display: 'flex', alignItems: 'center', gap: 8, font: '12.5px var(--mono)', color: 'var(--wait)' }}>
                  <Icon name="triangle-alert" size={14} />
                  Les jalons totalisent {msTotal} cr : ajuste-les pour égaler le prix avant de publier.
                </div>
              )}
              <div style={{ padding: 14, borderRadius: 10, background: 'var(--s2)', display: 'flex', flexDirection: 'column', gap: 8 }}>
                <span className="kicker">Repères du catalogue</span>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13 }}>
                  <span className="muted">Packs CI/CD similaires</span>
                  <span className="mono">90 – 140 cr</span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13 }}>
                  <span className="muted">Délai médian</span>
                  <span className="mono">4 j</span>
                </div>
              </div>
            </div>
          )}

          <div style={{ display: 'flex', justifyContent: 'space-between', paddingTop: 16, borderTop: '1px solid var(--bd)' }}>
            <button className="btn btn-secondary" onClick={() => setStep(Math.max(0, step - 1))} style={{ height: 36, fontSize: 13, gap: 6 }}>
              <Icon name="chevron-left" />
              Retour
            </button>
            {step < 3 ? (
              <button className="btn btn-primary" onClick={() => setStep(step + 1)} style={{ height: 36, padding: '0 16px', fontSize: 13, gap: 6 }}>
                Continuer
                <Icon name="arrow-right" />
              </button>
            ) : (
              <button className="btn btn-accent" onClick={publish} disabled={!msOk} aria-describedby={msOk ? undefined : 'ms-mismatch'} style={{ height: 36, padding: '0 16px', fontSize: 13, gap: 6, opacity: msOk ? 1 : 0.5 }}>
                <Icon name="check" />
                Publier le pack
              </button>
            )}
          </div>
        </div>

        <aside style={{ position: 'sticky', top: 84, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="kicker" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Icon name="eye" size={13} />
            Aperçu en direct
          </span>
          <PackCard
            as="div"
            pack={{ id: 'draft', title: d.title || 'Titre du pack', stack: tags, price: d.price, days: d.days, ms: d.ms.length, crit: d.criteria.length, name: ME.name, handle: ME.handle, ini: ME.ini, rep: ME.rep, deliv: 0 }}
          />
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 12px', borderRadius: 9, border: '1px solid var(--bd)', font: '12px var(--mono)', color: 'var(--tx2)' }}>
            <Icon name="shield-check" size={14} style={{ color: 'var(--ok)' }} />
            {ciCount}/{d.criteria.length} critères vérifiés par la CI
          </div>
        </aside>
      </div>
    </div>
  );
}
