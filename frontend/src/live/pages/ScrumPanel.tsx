import { useMemo, useState, type DragEvent, type FormEvent } from 'react';
import { ApiError, api, call, idem, type S } from '../../api/client';
import { useResource } from '../../api/useResource';
import { EmptyState, Icon, Pill, SegButtons } from '../../components/ui';
import type { Tone } from '../../domain/types';
import { fmtDate } from '../format';
import { Field, FormError, Loaded, small, useAction } from '../kit';

// F16 — cadre Scrum de l'échange : tableau, backlog, Gantt, burndown,
// vélocité, rétrospectives, synchronisation GitHub et avenants de planning.

export type ScrumTab = 'tableau' | 'gantt' | 'retro';
type Story = S['Story'];
type Status = S['StoryStatus'];

const COLUMN: Record<Status, [string, Tone]> = {
  BACKLOG: ['Backlog', 'mute'],
  A_FAIRE: ['À faire', 'mute'],
  EN_COURS: ['En cours', 'ac'],
  EN_REVUE: ['En revue', 'wait'],
  TERMINE: ['Terminé', 'ok'],
};

const SPRINT: Record<S['SprintStatus'], [string, Tone]> = {
  PLANIFIE: ['Planifié', 'mute'],
  ACTIF: ['Actif', 'ac'],
  TERMINE: ['Terminé', 'ok'],
  ANNULE: ['Annulé', 'mute'],
};

/** Déplacements permis par rôle (R-S6), miroir des règles du serveur. */
const MOVES: Record<'PRODUCT_OWNER' | 'EQUIPE_DEV', [Status, Status][]> = {
  EQUIPE_DEV: [['A_FAIRE', 'EN_COURS'], ['EN_COURS', 'A_FAIRE'], ['EN_COURS', 'EN_REVUE'], ['EN_REVUE', 'EN_COURS']],
  PRODUCT_OWNER: [['EN_REVUE', 'TERMINE'], ['EN_REVUE', 'EN_COURS'], ['TERMINE', 'EN_COURS']],
};

function canMove(role: S['ScrumProject']['mon_role'], from: Status, to: Status) {
  return role !== 'ARBITRE' && MOVES[role].some(([f, t]) => f === from && t === to);
}

/** Charge le projet, les sprints et les stories, rechargés ensemble. */
function useScrum(exchangeId: string) {
  const path = { params: { path: { id: exchangeId } } };
  const project = useResource(() => call(() => api.GET('/echanges/{id}/scrum', path)), [exchangeId]);
  const sprints = useResource(() => call(() => api.GET('/echanges/{id}/scrum/sprints', path)), [exchangeId]);
  const stories = useResource(() => call(() => api.GET('/echanges/{id}/scrum/stories', path)), [exchangeId]);
  const reload = () => {
    project.reload();
    sprints.reload();
    stories.reload();
  };
  return { project, sprints, stories, reload };
}

export function ScrumPanel({ x, tab }: { x: S['Exchange']; tab: ScrumTab }) {
  const sc = useScrum(x.id);
  if (sc.project.error instanceof ApiError && sc.project.error.code === 'projet-scrum-absent') {
    return (
      <EmptyState icon="list-checks" title="Pas de projet Scrum pour cet échange">
        Le cadre Scrum est créé à l'acceptation d'un échange ; cet échange a été accepté avant son arrivée.
      </EmptyState>
    );
  }
  return (
    <Loaded r={sc.project} what="Le projet Scrum">
      {(p) => (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <SyncBanner p={p} exchangeId={x.id} reload={sc.reload} />
          {p.avenant_en_cours && <AmendmentCard p={p} a={p.avenant_en_cours} x={x} reload={sc.reload} />}
          <Loaded r={sc.sprints} what="Les sprints">
            {(ss) => (
              <Loaded r={sc.stories} what="Les stories">
                {(st) =>
                  tab === 'tableau' ? (
                    <Board p={p} x={x} sprints={ss.items} stories={st.items} reload={sc.reload} />
                  ) : tab === 'gantt' ? (
                    <PlanningView p={p} x={x} sprints={ss.items} reload={sc.reload} />
                  ) : (
                    <Retros p={p} x={x} sprints={ss.items} stories={st.items} reload={sc.reload} />
                  )
                }
              </Loaded>
            )}
          </Loaded>
        </div>
      )}
    </Loaded>
  );
}

// ---------- Synchronisation GitHub ----------

function SyncBanner({ p, exchangeId, reload }: { p: S['ScrumProject']; exchangeId: string; reload(): void }) {
  const act = useAction();
  if (p.statut_synchro === 'ACTIVE' && p.appels_github_en_attente === 0) return null;
  const resync = async () => {
    if (await act.run(() => call(() => api.POST('/echanges/{id}/scrum/synchro', { params: { path: { id: exchangeId }, header: idem() } })), 'Synchronisation relancée')) reload();
  };
  const tone: Record<S['ScrumProject']['statut_synchro'], [string, Tone, string]> = {
    INACTIVE: ['Synchronisation GitHub désactivée sur ce serveur : le Scrum fonctionne ici seulement.', 'mute', 'info'],
    ACTIVE: [`${p.appels_github_en_attente} mise(s) à jour GitHub en attente.`, 'ac', 'refresh-cw'],
    PERMISSION_MANQUANTE: ['La GitHub App n\'a pas la permission « Issues : lecture et écriture » sur ce dépôt.', 'wait', 'triangle-alert'],
    ERREUR: [p.detail_synchro ?? 'GitHub a refusé une opération : nouvel essai automatique.', 'fail', 'triangle-alert'],
  };
  const [text, t, icon] = tone[p.statut_synchro];
  return (
    <div role="status" className="card" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', borderColor: t === 'fail' ? 'var(--fail)' : t === 'wait' ? 'var(--wait)' : undefined }}>
      <Icon name={icon} />
      <span style={{ flex: 1, minWidth: 220, fontSize: 13 }}>{text}</span>
      {p.statut_synchro === 'PERMISSION_MANQUANTE' && (
        <a className="btn btn-secondary" style={small} href="https://github.com/settings/installations" target="_blank" rel="noreferrer noopener">
          Accorder la permission <Icon name="external-link" size={12} />
        </a>
      )}
      {p.statut_synchro !== 'INACTIVE' && <button className="btn btn-secondary" style={small} onClick={resync} disabled={act.busy}>Relancer</button>}
      <FormError>{act.error}</FormError>
    </div>
  );
}

// ---------- Tableau ----------

function issueUrl(x: S['Exchange'], n: number) {
  return `${x.depot_url}/issues/${n}`;
}

function Board({ p, x, sprints, stories, reload }: { p: S['ScrumProject']; x: S['Exchange']; sprints: S['Sprint'][]; stories: Story[]; reload(): void }) {
  const open = sprints.filter((s) => s.statut === 'ACTIF' || s.statut === 'PLANIFIE');
  const active = sprints.find((s) => s.statut === 'ACTIF');
  const [selected, setSelected] = useState<number>(active?.numero ?? open[0]?.numero ?? sprints[sprints.length - 1]?.numero ?? 1);
  const sprint = sprints.find((s) => s.numero === selected);
  const inSprint = stories.filter((s) => s.sprint === selected);
  const backlog = stories.filter((s) => s.sprint === null);
  const writable = x.statut === 'EN_COURS' || x.statut === 'EN_LITIGE';
  const act = useAction();
  const [dragging, setDragging] = useState<Story | null>(null);

  const patch = (s: Story, body: Record<string, unknown>, ok?: string) =>
    act.run(() => call(() => api.PATCH('/echanges/{id}/scrum/stories/{sid}', { params: { path: { id: x.id, sid: s.id } }, body })), ok).then((r) => r && reload());

  const drop = (to: Status) => (e: DragEvent) => {
    e.preventDefault();
    if (dragging && dragging.statut !== to && canMove(p.mon_role, dragging.statut, to)) patch(dragging, { statut: to });
    setDragging(null);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <SegButtons options={sprints.map((s) => [s.numero, `Sprint ${s.numero}${s.statut === 'ACTIF' ? ' ●' : ''}`] as [number, string])} value={selected} onChange={setSelected} />
        {sprint && (
          <span className="muted" style={{ fontSize: 12.5 }}>
            Jalon {sprint.jalon} · {fmtDate(sprint.debut_prevu)} → {fmtDate(sprint.fin_prevue)} · <Pill tone={SPRINT[sprint.statut][1]}>{SPRINT[sprint.statut][0]}</Pill>
            {sprint.github_issue && <> · <a href={issueUrl(x, sprint.github_issue)} target="_blank" rel="noreferrer noopener" className="mono">#{sprint.github_issue}</a></>}
          </span>
        )}
      </div>
      {sprint && <SprintGoal x={x} sprint={sprint} editable={writable && (sprint.statut === 'ACTIF' || sprint.statut === 'PLANIFIE') && p.mon_role !== 'ARBITRE'} reload={reload} />}
      <FormError>{act.error}</FormError>
      <div className="scrum-board" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(200px,1fr))', gap: 10 }}>
        {(['A_FAIRE', 'EN_COURS', 'EN_REVUE', 'TERMINE'] as Status[]).map((col) => {
          const items = inSprint.filter((s) => s.statut === col);
          const target = dragging && canMove(p.mon_role, dragging.statut, col);
          return (
            <section key={col} aria-label={`Colonne ${COLUMN[col][0]}`} onDragOver={(e) => target && e.preventDefault()} onDrop={drop(col)}
              style={{ background: 'var(--s1)', border: `1px ${target ? 'dashed var(--ac)' : 'solid var(--bd)'}`, borderRadius: 10, padding: 10, display: 'flex', flexDirection: 'column', gap: 8, minHeight: 140 }}>
              <h3 style={{ margin: 0, fontSize: 12.5, display: 'flex', justifyContent: 'space-between' }}>
                <span>{COLUMN[col][0]}</span>
                <span className="mono faint">{items.reduce((t, s) => t + (s.points ?? 0), 0)} pts</span>
              </h3>
              {items.map((s) => (
                <StoryCard key={s.id} s={s} p={p} x={x} writable={writable} sprints={open} onDrag={setDragging} patch={patch} />
              ))}
            </section>
          );
        })}
      </div>
      {active && active.numero === selected && <Burndown sprint={active} stories={inSprint} />}
      <section className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
        <h2 className="h2">Backlog produit <span className="mono faint" style={{ fontSize: 12 }}>{backlog.length}</span></h2>
        {p.mon_role === 'PRODUCT_OWNER' && writable && <StoryForm x={x} p={p} sprints={open} reload={reload} />}
        {backlog.length === 0 ? (
          <span className="faint" style={{ fontSize: 13 }}>Backlog vide.</span>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {backlog.map((s) => <StoryCard key={s.id} s={s} p={p} x={x} writable={writable} sprints={open} onDrag={setDragging} patch={patch} />)}
          </div>
        )}
      </section>
    </div>
  );
}

function SprintGoal({ x, sprint, editable, reload }: { x: S['Exchange']; sprint: S['Sprint']; editable: boolean; reload(): void }) {
  const [goal, setGoal] = useState(sprint.objectif);
  const act = useAction();
  if (!editable) return sprint.objectif ? <p style={{ margin: 0, fontSize: 13 }}><strong>Objectif :</strong> {sprint.objectif}</p> : null;
  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (await act.run(() => call(() => api.PATCH('/echanges/{id}/scrum/sprints/{numero}', { params: { path: { id: x.id, numero: sprint.numero } }, body: { objectif: goal } })), 'Objectif enregistré')) reload();
  };
  return (
    <form onSubmit={save} style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
      <input className="field" style={{ flex: 1, minWidth: 220 }} maxLength={1000} value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="Objectif du sprint (Sprint Planning)" aria-label={`Objectif du sprint ${sprint.numero}`} />
      <button className="btn btn-secondary" style={small} disabled={act.busy || goal === sprint.objectif}>Enregistrer</button>
      <FormError>{act.error}</FormError>
    </form>
  );
}

function StoryCard({ s, p, x, writable, sprints, onDrag, patch }: {
  s: Story; p: S['ScrumProject']; x: S['Exchange']; writable: boolean; sprints: S['Sprint'][];
  onDrag(s: Story | null): void; patch(s: Story, body: Record<string, unknown>, ok?: string): void;
}) {
  const moves = (['A_FAIRE', 'EN_COURS', 'EN_REVUE', 'TERMINE'] as Status[]).filter((to) => to !== s.statut && canMove(p.mon_role, s.statut, to));
  const draggable = writable && moves.length > 0;
  return (
    <article draggable={draggable} onDragStart={() => onDrag(s)} onDragEnd={() => onDrag(null)} className="card"
      style={{ padding: 10, display: 'flex', flexDirection: 'column', gap: 6, cursor: draggable ? 'grab' : undefined, background: 'var(--s2, var(--s1))' }}>
      <span style={{ display: 'flex', gap: 6, alignItems: 'flex-start' }}>
        <span style={{ flex: 1, fontSize: 13, fontWeight: 500, lineHeight: 1.35 }}>{s.titre}</span>
        {s.points !== null && <span className="mono" style={{ fontSize: 11, padding: '1px 6px', borderRadius: 99, background: 'var(--s3)' }}>{s.points}</span>}
      </span>
      {s.criteres.length > 0 && <span className="faint" style={{ fontSize: 11.5 }}>{s.criteres.length} critère{s.criteres.length > 1 ? 's' : ''}</span>}
      <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center' }}>
        {s.github_issue && <a href={issueUrl(x, s.github_issue)} target="_blank" rel="noreferrer noopener" className="mono" style={{ fontSize: 11.5 }}><Icon name="github" size={12} /> #{s.github_issue}</a>}
        {writable && moves.map((to) => (
          <button key={to} className="btn btn-secondary" style={{ ...small, height: 24, fontSize: 11.5 }} onClick={() => patch(s, { statut: to })} aria-label={`Passer « ${s.titre} » en ${COLUMN[to][0]}`}>
            → {COLUMN[to][0]}
          </button>
        ))}
        {writable && p.mon_role === 'EQUIPE_DEV' && (
          <select className="field" style={{ height: 24, fontSize: 11.5, width: 'auto', padding: '0 6px' }} value={s.points ?? ''} aria-label={`Points de « ${s.titre} »`}
            onChange={(e) => patch(s, { points: +e.target.value })}>
            <option value="" disabled>pts</option>
            {p.points_possibles.map((n) => <option key={n} value={n}>{n} pts</option>)}
          </select>
        )}
        {writable && p.mon_role === 'PRODUCT_OWNER' && s.statut !== 'TERMINE' && (
          <select className="field" style={{ height: 24, fontSize: 11.5, width: 'auto', padding: '0 6px' }} value={s.sprint ?? ''} aria-label={`Sprint de « ${s.titre} »`}
            onChange={(e) => patch(s, { sprint: e.target.value === '' ? null : +e.target.value }, e.target.value === '' ? 'Renvoyée au backlog' : 'Story planifiée')}>
            <option value="">Backlog</option>
            {sprints.map((sp) => <option key={sp.numero} value={sp.numero}>Sprint {sp.numero}</option>)}
          </select>
        )}
      </span>
    </article>
  );
}

function StoryForm({ x, p, sprints, reload }: { x: S['Exchange']; p: S['ScrumProject']; sprints: S['Sprint'][]; reload(): void }) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [criteria, setCriteria] = useState('');
  const [sprint, setSprint] = useState<string>('');
  const act = useAction();
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const crit = criteria.split('\n').map((c) => c.trim()).filter(Boolean);
    const ok = await act.run(() => call(() => api.POST('/echanges/{id}/scrum/stories', {
      params: { path: { id: x.id }, header: idem() },
      body: { titre: title.trim(), description: description.trim(), criteres: crit, sprint: sprint ? +sprint : undefined },
    })), 'Story ajoutée');
    if (ok) {
      setTitle('');
      setDescription('');
      setCriteria('');
      reload();
    }
  };
  return (
    <details>
      <summary style={{ cursor: 'pointer', fontSize: 13, fontWeight: 500 }}>+ Nouvelle story</summary>
      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 10 }}>
        <Field label="Titre" hint={p.gabarit_story}><input className="field" required minLength={3} maxLength={160} value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
        <Field label="Description"><textarea className="field" rows={2} maxLength={4000} value={description} onChange={(e) => setDescription(e.target.value)} /></Field>
        <Field label="Critères d'acceptation" hint="Un par ligne (Given / When / Then)"><textarea className="field" rows={3} value={criteria} onChange={(e) => setCriteria(e.target.value)} /></Field>
        <Field label="Sprint">
          <select className="field" value={sprint} onChange={(e) => setSprint(e.target.value)}>
            <option value="">Backlog</option>
            {sprints.map((s) => <option key={s.numero} value={s.numero}>Sprint {s.numero}</option>)}
          </select>
        </Field>
        <FormError>{act.error}</FormError>
        <button className="btn btn-primary" style={{ ...small, alignSelf: 'flex-start' }} disabled={act.busy}>Ajouter</button>
      </form>
    </details>
  );
}

// ---------- Burndown et vélocité ----------

const DAY = 86_400_000;

/** Points restants jour par jour : total moins les stories terminées à cette date. */
export function burndownPoints(sprint: Pick<S['Sprint'], 'debut_prevu' | 'fin_prevue'>, stories: Pick<Story, 'points' | 'date_terminee'>[], now = Date.now()) {
  const start = Date.parse(sprint.debut_prevu);
  const end = Date.parse(sprint.fin_prevue);
  const days = Math.max(1, Math.round((end - start) / DAY));
  const total = stories.reduce((t, s) => t + (s.points ?? 0), 0);
  const actual: { day: number; remaining: number }[] = [];
  for (let d = 0; d <= days; d++) {
    const at = start + d * DAY;
    if (at > now) break;
    const done = stories.filter((s) => s.date_terminee && Date.parse(s.date_terminee) <= at).reduce((t, s) => t + (s.points ?? 0), 0);
    actual.push({ day: d, remaining: total - done });
  }
  return { days, total, actual };
}

function Burndown({ sprint, stories }: { sprint: S['Sprint']; stories: Story[] }) {
  const { days, total, actual } = burndownPoints(sprint, stories);
  if (total === 0) return null;
  const W = 520, H = 160, P = 28;
  const x = (d: number) => P + (d / days) * (W - 2 * P);
  const y = (v: number) => H - P - (v / total) * (H - 2 * P);
  const path = actual.map((a, i) => `${i ? 'L' : 'M'}${x(a.day).toFixed(1)} ${y(a.remaining).toFixed(1)}`).join(' ');
  const last = actual[actual.length - 1];
  return (
    <section className="card" style={{ padding: 16 }}>
      <h2 className="h2" style={{ marginBottom: 8 }}>Burndown du sprint {sprint.numero}</h2>
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={`Burndown : ${last?.remaining ?? total} points restants sur ${total}`} style={{ width: '100%', maxWidth: W, height: 'auto' }}>
        <line x1={P} y1={H - P} x2={W - P} y2={H - P} stroke="var(--bd2)" />
        <line x1={P} y1={P} x2={P} y2={H - P} stroke="var(--bd2)" />
        <line x1={x(0)} y1={y(total)} x2={x(days)} y2={y(0)} stroke="var(--tx3)" strokeDasharray="4 4" />
        <path d={path} fill="none" stroke="var(--ac)" strokeWidth={2.5} />
        {actual.map((a) => <circle key={a.day} cx={x(a.day)} cy={y(a.remaining)} r={3.5} fill="var(--ac)" />)}
        <text x={P - 6} y={y(total) + 4} textAnchor="end" fontSize={10} fill="var(--tx3)">{total}</text>
        <text x={P - 6} y={H - P + 4} textAnchor="end" fontSize={10} fill="var(--tx3)">0</text>
        <text x={W - P} y={H - 8} textAnchor="end" fontSize={10} fill="var(--tx3)">J+{days}</text>
      </svg>
    </section>
  );
}

function Velocity({ sprints }: { sprints: S['Sprint'][] }) {
  const done = sprints.filter((s) => s.statut === 'TERMINE');
  if (done.length === 0) return null;
  const max = Math.max(1, ...done.map((s) => Math.max(s.points_prevus, s.points_faits)));
  return (
    <section className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 className="h2">Vélocité</h2>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 12, height: 120 }} role="img" aria-label={done.map((s) => `Sprint ${s.numero} : ${s.points_faits} sur ${s.points_prevus} points`).join(', ')}>
        {done.map((s) => (
          <div key={s.numero} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, flex: '0 0 44px' }}>
            <span className="mono" style={{ fontSize: 11 }}>{s.points_faits}</span>
            <span style={{ display: 'flex', alignItems: 'flex-end', gap: 3, height: 80 }}>
              <span title="Prévus" style={{ width: 12, height: `${(s.points_prevus / max) * 80}px`, background: 'var(--s3)', borderRadius: 3 }} />
              <span title="Faits" style={{ width: 12, height: `${(s.points_faits / max) * 80}px`, background: 'var(--ac)', borderRadius: 3 }} />
            </span>
            <span className="mono faint" style={{ fontSize: 11 }}>S{s.numero}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

// ---------- Gantt ----------

function PlanningView({ p, x, sprints, reload }: { p: S['ScrumProject']; x: S['Exchange']; sprints: S['Sprint'][]; reload(): void }) {
  const plan = useResource(() => call(() => api.GET('/echanges/{id}/scrum/planning', { params: { path: { id: x.id } } })), [x.id, sprints.length]);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Loaded r={plan} what="Le planning">{(g) => <Gantt g={g} />}</Loaded>
      <Velocity sprints={sprints} />
      {p.mon_role !== 'ARBITRE' && !p.avenant_en_cours && (x.statut === 'EN_COURS' || x.statut === 'EN_LITIGE') && <AmendmentForm x={x} reload={() => { plan.reload(); reload(); }} />}
    </div>
  );
}

const SPRINT_FILL: Record<S['SprintStatus'], string> = {
  PLANIFIE: 'var(--s3)',
  ACTIF: 'var(--ac)',
  TERMINE: 'color-mix(in oklab,var(--ok) 70%,var(--s1))',
  ANNULE: 'color-mix(in oklab,var(--tx3) 25%,var(--s1))',
};

/** Gantt SVG : une ligne par jalon (barre, échéance en losange, montant), ses sprints dessous. */
export function Gantt({ g }: { g: S['ScrumPlanning'] }) {
  const start = Date.parse(g.debut);
  const end = Math.max(Date.parse(g.fin), Date.parse(g.aujourd_hui));
  const W = 760, L = 150, R = 96, ROW = 26;
  const span = Math.max(DAY, end - start);
  const x = (iso: string | number) => L + ((typeof iso === 'number' ? iso : Date.parse(iso)) - start) / span * (W - L - R);
  const rows = g.jalons.reduce((n, j) => n + 1 + j.sprints.length, 0);
  const H = 30 + rows * ROW + 10;
  const today = x(g.aujourd_hui);
  let row = 0;
  const y = () => 30 + row++ * ROW;
  return (
    <section className="card" style={{ padding: 16, overflowX: 'auto' }}>
      <h2 className="h2" style={{ marginBottom: 8 }}>Planning</h2>
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={`Gantt de ${g.jalons.length} jalons`} style={{ width: '100%', minWidth: 560, height: 'auto', fontFamily: 'var(--mono)' }}>
        <defs>
          <pattern id="retard" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="3" height="6" fill="var(--fail)" opacity="0.45" />
          </pattern>
        </defs>
        <text x={L} y={16} fontSize={10} fill="var(--tx3)">{fmtDate(g.debut)}</text>
        <text x={W - R} y={16} fontSize={10} fill="var(--tx3)" textAnchor="end">{fmtDate(g.fin)}</text>
        {g.jalons.map((j) => {
          const jy = y();
          const late = j.en_retard ? <rect x={x(j.echeance)} y={jy + 4} width={Math.max(0, today - x(j.echeance))} height={14} fill="url(#retard)" /> : null;
          return (
            <g key={j.ordre}>
              <text x={8} y={jy + 15} fontSize={11.5} fill="var(--tx)" fontWeight={600}>J{j.ordre} · {j.titre.slice(0, 16)}</text>
              <rect x={x(j.debut)} y={jy + 4} width={Math.max(2, x(j.echeance) - x(j.debut))} height={14} rx={4} fill="var(--s3)" stroke={j.en_retard ? 'var(--fail)' : 'var(--bd2)'} />
              {late}
              <rect x={x(j.echeance) - 5} y={jy + 6} width={10} height={10} transform={`rotate(45 ${x(j.echeance)} ${jy + 11})`} fill={['LIBERE', 'PARTAGE'].includes(j.statut_jalon) ? 'var(--ok)' : 'var(--tx2)'} />
              <text x={x(j.echeance) + 10} y={jy + 15} fontSize={10.5} fill="var(--tx2)">{j.montant} cr{j.risque !== 'AUCUN' ? ` · risque ${j.risque === 'ELEVE' ? 'élevé' : 'moyen'}` : ''}{j.en_retard ? ' · en retard' : ''}</text>
              {j.sprints.map((s) => {
                const sy = y();
                return (
                  <g key={s.numero}>
                    <text x={22} y={sy + 15} fontSize={11} fill="var(--tx2)">Sprint {s.numero}</text>
                    <rect x={x(s.debut_prevu)} y={sy + 6} width={Math.max(2, x(s.fin_prevue) - x(s.debut_prevu) - 2)} height={10} rx={3} fill={SPRINT_FILL[s.statut]}>
                      <title>{`Sprint ${s.numero} · ${SPRINT[s.statut][0]} · ${s.points_faits}/${s.points_prevus} pts`}</title>
                    </rect>
                    {s.debut_reel && s.debut_reel < s.debut_prevu && <circle cx={x(s.debut_reel)} cy={sy + 11} r={3} fill="var(--ac)" />}
                  </g>
                );
              })}
            </g>
          );
        })}
        {g.marqueurs.map((m, i) => (
          <g key={i}>
            <line x1={x(m.date)} y1={22} x2={x(m.date)} y2={H - 6} stroke="var(--ok)" strokeDasharray="2 3" />
            <title>{`PR #${m.pr} · jalon ${m.jalon}`}</title>
          </g>
        ))}
        <line x1={today} y1={22} x2={today} y2={H - 4} stroke="var(--ac)" strokeWidth={1.5} />
        <text x={today + 4} y={28} fontSize={10} fill="var(--ac-tx)">aujourd'hui</text>
      </svg>
    </section>
  );
}

// ---------- Avenants ----------

function AmendmentForm({ x, reload }: { x: S['Exchange']; reload(): void }) {
  const [dates, setDates] = useState(() => x.jalons.map((j) => (j.echeance ?? '').slice(0, 10)));
  const [reason, setReason] = useState('');
  const act = useAction();
  const editable = (j: S['ExchangeMilestone']) => j.statut === 'EN_ATTENTE' || j.statut === 'EN_COURS';
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    // Une date modifiée prend l'heure de l'échéance d'origine ; inchangée, l'échéance exacte est conservée.
    const echeances = x.jalons.map((j, i) => (j.echeance && j.echeance.slice(0, 10) === dates[i] ? j.echeance : `${dates[i]}${(j.echeance ?? 'T12:00:00Z').slice(10)}`));
    if (await act.run(() => call(() => api.POST('/echanges/{id}/scrum/avenants', { params: { path: { id: x.id }, header: idem() }, body: { echeances, motif: reason.trim() } })), "Avenant proposé à l'autre partie")) reload();
  };
  return (
    <details className="card" style={{ padding: 16 }}>
      <summary style={{ cursor: 'pointer', fontWeight: 600, fontSize: 14 }}>Proposer de nouvelles échéances (avenant)</summary>
      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 12 }}>
        <span className="muted" style={{ fontSize: 13 }}>L'autre partie doit accepter. Les montants ne changent pas ; les sprints à venir sont redécoupés et leurs stories reviennent au backlog.</span>
        {x.jalons.map((j, i) => (
          <Field key={j.id} label={`Jalon ${j.ordre} · ${j.titre}`}>
            <input className="field" type="date" value={dates[i]} disabled={!editable(j)} onChange={(e) => setDates((d) => d.map((v, k) => (k === i ? e.target.value : v)))} style={{ width: 180 }} />
          </Field>
        ))}
        <Field label="Motif"><textarea className="field" rows={2} maxLength={1000} value={reason} onChange={(e) => setReason(e.target.value)} /></Field>
        <FormError>{act.error}</FormError>
        <button className="btn btn-primary" style={{ ...small, alignSelf: 'flex-start' }} disabled={act.busy}>Proposer</button>
      </form>
    </details>
  );
}

function AmendmentCard({ p, a, x, reload }: { p: S['ScrumProject']; a: S['Amendment']; x: S['Exchange']; reload(): void }) {
  const act = useAction();
  const mine = (p.mon_role === 'PRODUCT_OWNER' ? x.demandeur.id : x.prestataire.id) === a.propose_par;
  const decide = async (accept: boolean) => {
    const path = { params: { path: { id: x.id, aid: a.id } } };
    const ok = await act.run(
      () => accept
        ? call(() => api.POST('/echanges/{id}/scrum/avenants/{aid}/accepter', { params: { ...path.params, header: idem() } }))
        : call(() => api.POST('/echanges/{id}/scrum/avenants/{aid}/refuser', path)),
      accept ? 'Avenant accepté : planning mis à jour' : mine ? 'Avenant retiré' : 'Avenant refusé',
    );
    if (ok) reload();
  };
  return (
    <div role="status" className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 8, borderColor: 'var(--wait)' }}>
      <strong style={{ fontSize: 14 }}>{mine ? 'Ton avenant attend la réponse de l\'autre partie' : 'Nouvelles échéances proposées'}</strong>
      <span className="mono" style={{ fontSize: 12.5 }}>{a.echeances.map((d, i) => `J${i + 1} : ${fmtDate(d)}`).join(' · ')}</span>
      {a.motif && <span className="muted" style={{ fontSize: 13 }}>{a.motif}</span>}
      <FormError>{act.error}</FormError>
      {p.mon_role !== 'ARBITRE' && (
        <span style={{ display: 'flex', gap: 8 }}>
          {!mine && <button className="btn btn-primary" style={small} onClick={() => decide(true)} disabled={act.busy}>Accepter</button>}
          <button className="btn btn-secondary" style={small} onClick={() => decide(false)} disabled={act.busy}>{mine ? 'Retirer' : 'Refuser'}</button>
        </span>
      )}
    </div>
  );
}

// ---------- Rétrospectives ----------

function Retros({ p, x, sprints, stories, reload }: { p: S['ScrumProject']; x: S['Exchange']; sprints: S['Sprint'][]; stories: Story[]; reload(): void }) {
  const done = useMemo(() => sprints.filter((s) => s.statut === 'TERMINE').reverse(), [sprints]);
  if (done.length === 0) {
    return <EmptyState icon="history" title="Pas encore de sprint terminé">La rétrospective s'ouvre à la fin de chaque sprint.</EmptyState>;
  }
  const backlog = stories.filter((s) => s.sprint === null).length;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Velocity sprints={sprints} />
      {backlog > 0 && <span className="muted" style={{ fontSize: 13 }}>{backlog} story(s) au backlog, dont celles non terminées des sprints passés.</span>}
      {done.map((s) => <RetroForm key={s.numero} x={x} s={s} readOnly={p.mon_role === 'ARBITRE'} reload={reload} />)}
    </div>
  );
}

function RetroForm({ x, s, readOnly, reload }: { x: S['Exchange']; s: S['Sprint']; readOnly: boolean; reload(): void }) {
  const [good, setGood] = useState(s.retro.bien ?? '');
  const [improve, setImprove] = useState(s.retro.a_ameliorer ?? '');
  const [actions, setActions] = useState(s.retro.actions ?? '');
  const act = useAction();
  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (await act.run(() => call(() => api.PUT('/echanges/{id}/scrum/sprints/{numero}/retro', { params: { path: { id: x.id, numero: s.numero } }, body: { bien: good, a_ameliorer: improve, actions } })), 'Rétrospective enregistrée')) reload();
  };
  return (
    <form onSubmit={save} className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 className="h2">Sprint {s.numero} <span className="mono faint" style={{ fontSize: 12 }}>· {s.points_faits}/{s.points_prevus} pts · {fmtDate(s.debut_prevu)} → {fmtDate(s.fin_prevue)}</span></h2>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(200px,1fr))', gap: 10 }}>
        <Field label="Ce qui a bien marché"><textarea className="field" rows={3} maxLength={2000} readOnly={readOnly} value={good} onChange={(e) => setGood(e.target.value)} /></Field>
        <Field label="À améliorer"><textarea className="field" rows={3} maxLength={2000} readOnly={readOnly} value={improve} onChange={(e) => setImprove(e.target.value)} /></Field>
        <Field label="Actions"><textarea className="field" rows={3} maxLength={2000} readOnly={readOnly} value={actions} onChange={(e) => setActions(e.target.value)} /></Field>
      </div>
      <FormError>{act.error}</FormError>
      {!readOnly && <button className="btn btn-secondary" style={{ ...small, alignSelf: 'flex-start' }} disabled={act.busy}>Enregistrer</button>}
    </form>
  );
}
