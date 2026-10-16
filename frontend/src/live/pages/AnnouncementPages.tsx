import { useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { api, call, idem, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { Avatar, EmptyState, Icon, PageTitle, Pill, Tag } from '../../components/ui';
import type { Tone } from '../../domain/types';
import { fmtDate, initials } from '../format';
import { Field, FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';
import { NeedsGitHub } from './EmailAuth';

// F17 — annonces : une personne qui a besoin d'aide publie sa demande,
// des prestataires candidatent, elle en retient un.

export const ANNOUNCEMENT_STATUS: Record<S['Announcement']['statut'], [string, Tone]> = {
  OUVERTE: ['Ouverte', 'ac'],
  POURVUE: ['Pourvue', 'ok'],
  RETIREE: ['Retirée', 'mute'],
  EXPIREE: ['Expirée', 'mute'],
};

const APPLICATION_STATUS: Record<S['Application']['statut'], [string, Tone]> = {
  EN_ATTENTE: ['En attente', 'wait'],
  ACCEPTEE: ['Retenue', 'ok'],
  REFUSEE: ['Non retenue', 'mute'],
  RETIREE: ['Retirée', 'mute'],
};


/** Éditeur de jalons : la somme des montants fait le prix (RG10). */
export function MilestonesEditor({ value, onChange, max = 10 }: { value: S['Milestone'][]; onChange(ms: S['Milestone'][]): void; max?: number }) {
  const set = (i: number, patch: Partial<S['Milestone']>) => onChange(value.map((m, j) => (j === i ? { ...m, ...patch } : m)));
  return (
    <fieldset style={{ border: 0, margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
      <legend style={{ fontWeight: 500, fontSize: 13, marginBottom: 6 }}>Jalons (le total est leur somme)</legend>
      {value.map((m, i) => (
        <div key={i} className="jalon-row">
          <input required maxLength={120} value={m.titre} onChange={(e) => set(i, { titre: e.target.value })} placeholder={`Jalon ${i + 1}`} aria-label={`Titre du jalon ${i + 1}`} className="field jalon-titre" />
          <input required type="number" min={1} value={m.montant || ''} onChange={(e) => set(i, { montant: +e.target.value })} aria-label={`Montant du jalon ${i + 1}`} className="field jalon-montant" />
          {value.length > 1 && <button type="button" className="btn btn-secondary" style={small} onClick={() => onChange(value.filter((_, j) => j !== i))} aria-label={`Retirer le jalon ${i + 1}`}>×</button>}
        </div>
      ))}
      {value.length < max && <button type="button" className="btn btn-secondary" style={{ ...small, alignSelf: 'flex-start' }} onClick={() => onChange([...value, { titre: '', montant: 0 }])}>+ Jalon</button>}
    </fieldset>
  );
}

const sum = (ms: S['Milestone'][]) => ms.reduce((s, m) => s + (m.montant || 0), 0);

/** Carte d'annonce du catalogue. */
export function AnnouncementCard({ a, onOpen }: { a: S['Announcement']; onOpen(): void }) {
  const [label, tone] = ANNOUNCEMENT_STATUS[a.statut];
  return (
    <button onClick={onOpen} className="card hov-card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10, textAlign: 'left', color: 'var(--tx)', font: 'inherit', cursor: 'pointer' }}>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Icon name="hand" style={{ color: 'var(--ac-tx)' }} />
        <span className="mono faint" style={{ flex: 1, fontSize: 11.5 }}>{a.pack.titre}</span>
        {a.statut !== 'OUVERTE' && <Pill tone={tone}>{label}</Pill>}
      </span>
      <span style={{ fontSize: 15, fontWeight: 600, lineHeight: 1.3 }}>{a.titre}</span>
      <span className="muted" style={{ fontSize: 13, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{a.besoin}</span>
      <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
        <Tag>{a.budget_credits} cr</Tag>
        <Tag>{a.delai_jours} j</Tag>
        <Tag>{a.jalons.length} jalon{a.jalons.length > 1 ? 's' : ''}</Tag>
        <Tag>{a.candidatures_en_attente} candidature{a.candidatures_en_attente > 1 ? 's' : ''}</Tag>
      </span>
      <span className="faint" style={{ fontSize: 12 }}>par {a.auteur.pseudo} · jusqu'au {fmtDate(a.date_expiration)}</span>
    </button>
  );
}

export function CreateAnnouncementPage() {
  const packs = useResource(() => call(() => api.GET('/packs', { params: { query: { limit: 100 } } })), []);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 760 }}>
      <PageTitle title="Publier une annonce" sub="Décris ton besoin : des prestataires te proposent leur prix, leur délai et leurs jalons." />
      <Loaded r={packs} what="Les packs">{(p) => <AnnouncementForm packs={p.items} />}</Loaded>
    </div>
  );
}

function AnnouncementForm({ packs }: { packs: S['Pack'][] }) {
  const navigate = useNavigate();
  const [packId, setPackId] = useState(packs[0]?.id ?? '');
  const pack = useMemo(() => packs.find((p) => p.id === packId), [packs, packId]);
  const [title, setTitle] = useState('');
  const [need, setNeed] = useState('');
  const [repo, setRepo] = useState('https://github.com/');
  const [days, setDays] = useState(14);
  const [milestones, setMilestones] = useState<S['Milestone'][]>([{ titre: '', montant: 0 }]);
  const act = useAction();
  const budget = sum(milestones);
  const inRange = pack ? budget >= pack.prix_min && budget <= pack.prix_max : false;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const a = await act.run(
      () => call(() => api.POST('/annonces', { body: { id_pack: packId, titre: title.trim(), besoin: need.trim(), depot_url: repo.trim(), budget_credits: budget, delai_jours: days, jalons: milestones.map((m) => ({ titre: m.titre.trim(), montant: m.montant })) } })),
      'Annonce publiée',
    );
    if (a) navigate(R.announcement(a.id));
  };

  return (
    <form onSubmit={submit} className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Field label="Pack" hint={pack ? `${pack.livrable} · budget entre ${pack.prix_min} et ${pack.prix_max} cr` : undefined}>
        <select className="field" value={packId} onChange={(e) => setPackId(e.target.value)}>
          {packs.map((p) => <option key={p.id} value={p.id}>{p.titre}</option>)}
        </select>
      </Field>
      <Field label="Titre"><input className="field" required minLength={5} maxLength={120} value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Pipeline CI pour notre API Node" /></Field>
      <Field label="Ton besoin" hint="Contexte, contraintes, ce qui est attendu"><textarea className="field" required rows={5} maxLength={4000} value={need} onChange={(e) => setNeed(e.target.value)} /></Field>
      <Field label="Dépôt GitHub" hint="Le travail et les preuves se font sur ce dépôt"><input className="field" required value={repo} onChange={(e) => setRepo(e.target.value)} /></Field>
      <Field label="Délai (jours)"><input className="field" type="number" min={1} max={90} value={days} onChange={(e) => setDays(+e.target.value)} style={{ width: 120 }} /></Field>
      <MilestonesEditor value={milestones} onChange={setMilestones} />
      <span className="mono" style={{ fontSize: 13, color: inRange ? 'var(--ac-tx)' : 'var(--wait)' }}>
        Budget : {budget} cr{pack && !inRange ? ` (attendu entre ${pack.prix_min} et ${pack.prix_max})` : ''} · rien n'est bloqué avant que tu retiennes un prestataire
      </span>
      <FormError>{act.error}</FormError>
      <button type="submit" className="btn btn-primary" disabled={act.busy || !inRange} style={{ alignSelf: 'flex-start' }}>Publier l'annonce</button>
    </form>
  );
}

export function AnnouncementPage() {
  const { id = '' } = useParams();
  const a = useResource(() => call(() => api.GET('/annonces/{id}', { params: { path: { id } } })), [id]);
  return <Loaded r={a} what="Cette annonce">{(v) => <AnnouncementView a={v} reload={a.reload} />}</Loaded>;
}

function AnnouncementView({ a, reload }: { a: S['Announcement']; reload(): void }) {
  const { user } = useSession();
  const navigate = useNavigate();
  const mine = user?.id === a.auteur.id;
  const [label, tone] = ANNOUNCEMENT_STATUS[a.statut];
  const act = useAction();
  const withdraw = async () => {
    if (await act.run(() => call(() => api.POST('/annonces/{id}/retirer', { params: { path: { id: a.id } } })), 'Annonce retirée')) reload();
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap' }}>
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className="mono faint" style={{ fontSize: 12 }}>Annonce · {a.pack.titre} · publiée le {fmtDate(a.date_publication)}</span>
          <h1 className="h1">{a.titre}</h1>
        </div>
        <Pill tone={tone}>{label}</Pill>
        {mine && a.statut === 'OUVERTE' && <button className="btn btn-secondary" style={small} onClick={withdraw} disabled={act.busy}>Retirer l'annonce</button>}
      </div>
      <FormError>{act.error}</FormError>
      {a.id_echange && (
        <button className="btn btn-primary" style={{ alignSelf: 'flex-start' }} onClick={() => navigate(R.exchange(a.id_echange!))}>
          <Icon name="arrow-right" /> Ouvrir l'échange
        </button>
      )}
      <div className="split split-340">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
            <h2 className="h2">Besoin</h2>
            <p style={{ margin: 0, whiteSpace: 'pre-wrap', fontSize: 13.5 }}>{a.besoin}</p>
            <a href={a.depot_url} target="_blank" rel="noreferrer noopener" className="mono" style={{ fontSize: 12.5 }}>{a.depot_url.replace('https://github.com/', '')} <Icon name="external-link" size={12} /></a>
          </section>
          {mine ? <Applications a={a} reload={reload} /> : a.statut === 'OUVERTE' || a.ma_candidature ? <MyApplication a={a} reload={reload} /> : null}
        </div>
        <aside className="sticky-aside">
          <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
            <dl style={{ margin: 0, display: 'grid', gridTemplateColumns: '1fr auto', gap: '6px 8px', fontSize: 13 }}>
              <dt className="muted">Budget</dt><dd className="mono" style={{ margin: 0 }}>{a.budget_credits} cr</dd>
              <dt className="muted">Délai</dt><dd className="mono" style={{ margin: 0 }}>{a.delai_jours} j</dd>
              <dt className="muted">Candidatures</dt><dd className="mono" style={{ margin: 0 }}>{a.candidatures_en_attente}</dd>
              <dt className="muted">Expire le</dt><dd className="mono" style={{ margin: 0 }}>{fmtDate(a.date_expiration)}</dd>
            </dl>
            <h3 style={{ margin: 0, fontSize: 13 }}>Jalons souhaités</h3>
            <ol style={{ margin: 0, paddingLeft: 18, fontSize: 13, display: 'flex', flexDirection: 'column', gap: 4 }}>
              {a.jalons.map((m, i) => <li key={i}>{m.titre} <span className="mono faint">· {m.montant} cr</span></li>)}
            </ol>
          </div>
          <Link to={R.member(a.auteur.id)} className="card hov-card" style={{ padding: 16, display: 'flex', alignItems: 'center', gap: 10, color: 'var(--tx)' }}>
            <Avatar ini={initials(a.auteur.pseudo)} size={32} fontSize={9} />
            <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
              <span style={{ fontSize: 13, fontWeight: 500 }}>{a.auteur.pseudo}</span>
              {a.auteur.github_login && <span className="mono faint" style={{ fontSize: 11.5 }}>@{a.auteur.github_login}</span>}
            </span>
          </Link>
        </aside>
      </div>
    </div>
  );
}

/** Candidatures reçues (auteur de l'annonce). */
function Applications({ a, reload }: { a: S['Announcement']; reload(): void }) {
  const navigate = useNavigate();
  const apps = useResource(() => call(() => api.GET('/annonces/{id}/candidatures', { params: { path: { id: a.id } } })), [a.id]);
  const act = useAction();
  const decide = async (cid: string, accept: boolean) => {
    if (accept) {
      const x = await act.run(() => call(() => api.POST('/annonces/{id}/candidatures/{cid}/accepter', { params: { path: { id: a.id, cid }, header: idem() } })), 'Prestataire retenu : échange démarré');
      if (x) navigate(R.exchange(x.id));
      return;
    }
    if (await act.run(() => call(() => api.POST('/annonces/{id}/candidatures/{cid}/refuser', { params: { path: { id: a.id, cid } } })), 'Candidature refusée')) {
      apps.reload();
      reload();
    }
  };
  return (
    <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <h2 className="h2">Candidatures</h2>
      <FormError>{act.error}</FormError>
      <Loaded r={apps} what="Les candidatures">
        {(p) =>
          p.items.length === 0 ? (
            <EmptyState icon="inbox" title="Pas encore de candidature" padding="24px">Les prestataires intéressés apparaîtront ici.</EmptyState>
          ) : (
            <ul style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 10 }}>
              {p.items.map((c) => (
                <li key={c.id} style={{ border: '1px solid var(--bd)', borderRadius: 10, padding: 14, display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <ApplicationSummary c={c} />
                  {c.statut === 'EN_ATTENTE' && a.statut === 'OUVERTE' && (
                    <span style={{ display: 'flex', gap: 8 }}>
                      <button className="btn btn-primary" style={small} disabled={act.busy} onClick={() => decide(c.id, true)}>Retenir ({c.prix_credits} cr bloqués)</button>
                      <button className="btn btn-secondary" style={small} disabled={act.busy} onClick={() => decide(c.id, false)}>Refuser</button>
                    </span>
                  )}
                </li>
              ))}
            </ul>
          )
        }
      </Loaded>
    </section>
  );
}

function ApplicationSummary({ c }: { c: S['Application'] }) {
  const [label, tone] = APPLICATION_STATUS[c.statut];
  return (
    <>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        <Link to={R.member(c.prestataire.id)} style={{ fontWeight: 600, fontSize: 13.5 }}>{c.prestataire.pseudo}</Link>
        <span className="mono faint" style={{ fontSize: 11.5 }}>{c.prestataire.score_reputation.toFixed(1)} ★ · {c.prestataire.livraisons_verifiees} livraisons</span>
        <span style={{ flex: 1 }} />
        <Pill tone={tone}>{label}</Pill>
      </span>
      <span className="mono" style={{ fontSize: 12.5 }}>{c.prix_credits} cr · {c.delai_jours} j · {c.jalons.map((m) => `${m.titre} (${m.montant})`).join(' → ')}</span>
      {c.message && <p className="muted" style={{ margin: 0, fontSize: 13, whiteSpace: 'pre-wrap' }}>{c.message}</p>}
    </>
  );
}

/** Candidature de l'appelant : formulaire, ou état et retrait. */
function MyApplication({ a, reload }: { a: S['Announcement']; reload(): void }) {
  const { user } = useSession();
  const apps = useResource(() => call(() => api.GET('/annonces/{id}/candidatures', { params: { path: { id: a.id } } })), [a.id, a.ma_candidature]);
  return (
    <Loaded r={apps} what="Ta candidature">
      {(p) => {
        const mine = p.items[p.items.length - 1];
        if (mine && (mine.statut !== 'RETIREE' || a.statut !== 'OUVERTE')) {
          return <OwnApplication a={a} c={mine} reload={() => { apps.reload(); reload(); }} />;
        }
        if (user && !user.github_lie) return <NeedsGitHub what="candidater" />;
        return <ApplyForm a={a} reload={() => { apps.reload(); reload(); }} />;
      }}
    </Loaded>
  );
}

function OwnApplication({ a, c, reload }: { a: S['Announcement']; c: S['Application']; reload(): void }) {
  const act = useAction();
  const withdraw = async () => {
    if (await act.run(() => call(() => api.POST('/annonces/{id}/candidatures/{cid}/retirer', { params: { path: { id: a.id, cid: c.id } } })), 'Candidature retirée')) reload();
  };
  return (
    <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 className="h2">Ta candidature</h2>
      <ApplicationSummary c={c} />
      <FormError>{act.error}</FormError>
      {c.statut === 'EN_ATTENTE' && <button className="btn btn-secondary" style={{ ...small, alignSelf: 'flex-start' }} onClick={withdraw} disabled={act.busy}>Retirer ma candidature</button>}
    </section>
  );
}

function ApplyForm({ a, reload }: { a: S['Announcement']; reload(): void }) {
  const [days, setDays] = useState(a.delai_jours);
  const [message, setMessage] = useState('');
  const [milestones, setMilestones] = useState<S['Milestone'][]>(a.jalons);
  const act = useAction();
  const price = sum(milestones);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const c = await act.run(
      () => call(() => api.POST('/annonces/{id}/candidatures', { params: { path: { id: a.id } }, body: { prix_credits: price, delai_jours: days, message: message.trim(), jalons: milestones.map((m) => ({ titre: m.titre.trim(), montant: m.montant })) } })),
      'Candidature envoyée',
    );
    if (c) reload();
  };
  return (
    <form onSubmit={submit} className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
      <h2 className="h2">Candidater</h2>
      <span className="muted" style={{ fontSize: 13 }}>Ajuste les jalons et le délai si besoin : ton prix est la somme des jalons.</span>
      <MilestonesEditor value={milestones} onChange={setMilestones} />
      <Field label="Délai (jours)"><input className="field" type="number" min={1} max={90} value={days} onChange={(e) => setDays(+e.target.value)} style={{ width: 120 }} /></Field>
      <Field label="Message" hint="Ton expérience sur ce type de livrable, tes disponibilités"><textarea className="field" rows={3} maxLength={2000} value={message} onChange={(e) => setMessage(e.target.value)} /></Field>
      <span className="mono" style={{ fontSize: 13 }}>Prix proposé : {price} cr {price > a.budget_credits ? `(au-dessus du budget de ${a.budget_credits} cr)` : ''}</span>
      <FormError>{act.error}</FormError>
      <button type="submit" className="btn btn-primary" disabled={act.busy || price <= 0} style={{ alignSelf: 'flex-start' }}>Envoyer ma candidature</button>
    </form>
  );
}
