import { useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { PackCard } from '../../components/PackCard';
import { Avatar, Icon, Tag } from '../../components/ui';
import { plural, repRing } from '../../domain/logic';
import { fmtDate, initials, offerToPack } from '../format';
import { Field, FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

const LEVELS: S['Level'][] = ['DEBUTANT', 'INTERMEDIAIRE', 'CONFIRME', 'EXPERT'];
const LEVEL_LABEL: Record<S['Level'], string> = { DEBUTANT: 'débutant', INTERMEDIAIRE: 'intermédiaire', CONFIRME: 'confirmé', EXPERT: 'expert' };

export function MemberPage() {
  const { id = '' } = useParams();
  const { user } = useSession();
  const navigate = useNavigate();
  const data = useResource(async () => {
    const [profile, reviews, offers] = await Promise.all([
      call(() => api.GET('/utilisateurs/{id}', { params: { path: { id } } })),
      call(() => api.GET('/utilisateurs/{id}/avis', { params: { path: { id }, query: { limit: 20 } } })),
      call(() => api.GET('/offres', { params: { query: { prestataire: id, limit: 20 } } })),
    ]);
    return { profile, reviews: reviews.items, offers: offers.items };
  }, [id]);
  const isMe = user?.id === id;

  return (
    <Loaded r={data} what="Ce profil">
      {({ profile: p, reviews, offers }) => (
        <div className="split split-side-first">
          <aside className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
            <Avatar ini={initials(p.pseudo)} size={84} ring={repRing(p.score_reputation)} />
            <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span style={{ fontSize: 20, fontWeight: 600 }}>{p.pseudo}</span>
              {p.github_login && <a className="mono faint" style={{ fontSize: 12.5 }} href={`https://github.com/${p.github_login}`} target="_blank" rel="noreferrer noopener">@{p.github_login}</a>}
            </span>
            {p.bio && <p style={{ margin: 0, fontSize: 13.5, whiteSpace: 'pre-wrap' }}>{p.bio}</p>}
            <span className="mono muted" style={{ fontSize: 12.5 }}>
              {p.score_reputation.toFixed(1)} ★ · {plural(p.avis_recus, 'avis', 'avis')} · {plural(p.livraisons_verifiees, 'livraison vérifiée', 'livraisons vérifiées')}
            </span>
            <span className="faint" style={{ fontSize: 12 }}>Membre depuis le {fmtDate(p.date_inscription)}</span>
            <span style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {p.competences.map((c) => <Tag key={c.id}>{c.libelle} · {LEVEL_LABEL[c.niveau]}</Tag>)}
            </span>
          </aside>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
            {isMe && user && !user.github_lie && <LinkGitHub />}
            {isMe && <EditProfile profile={p} onSaved={data.reload} onDeleted={() => navigate(R.landing)} />}
            <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <h2 className="h2">Offres publiées</h2>
              {offers.length === 0 ? <span className="faint">Aucune offre publiée.</span> : (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))', gap: 12 }}>
                  {offers.map((o) => <PackCard key={o.id} pack={offerToPack(o)} onOpen={() => navigate(R.offer(o.id))} />)}
                </div>
              )}
            </section>
            <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <h2 className="h2">Avis reçus</h2>
              {reviews.length === 0 && <span className="faint">Pas encore d'avis.</span>}
              {reviews.map((r) => (
                <div key={r.id} className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <span style={{ fontSize: 13 }}>
                    <strong>{r.auteur}</strong> <span className="mono" style={{ color: 'var(--ac-tx)' }}>{r.note}/5</span> <span className="faint mono" style={{ fontSize: 11.5 }}>· {fmtDate(r.date)}</span>
                  </span>
                  {r.commentaire && <span style={{ fontSize: 13.5, whiteSpace: 'pre-wrap' }}>{r.commentaire}</span>}
                </div>
              ))}
            </section>
          </div>
        </div>
      )}
    </Loaded>
  );
}

function EditProfile({ profile, onSaved, onDeleted }: { profile: S['PublicProfile']; onSaved(): void; onDeleted(): void }) {
  const { setUser, logout } = useSession();
  const [pseudo, setPseudo] = useState(profile.pseudo);
  const [bio, setBio] = useState(profile.bio ?? '');
  const catalog = useResource(() => call(() => api.GET('/competences')), []);
  const [skills, setSkills] = useState<Record<string, S['Level']>>(() => Object.fromEntries(profile.competences.map((c) => [c.id, c.niveau])));
  const save = useAction();
  const del = useAction();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const ok = await save.run(async () => {
      const me = await call(() => api.PATCH('/me', { body: { pseudo: pseudo.trim(), bio } }));
      await call(() => api.PUT('/me/competences', { body: { competences: Object.entries(skills).map(([id_competence, niveau]) => ({ id_competence, niveau })) } }));
      setUser(me);
      return me;
    }, 'Profil enregistré');
    if (ok) onSaved();
  };
  const remove = async () => {
    if ((await del.run(() => call(() => api.DELETE('/me')))) !== undefined) {
      await logout();
      onDeleted();
    }
  };

  return (
    <form onSubmit={submit} className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
      <h2 className="h2">Mon profil</h2>
      <Field label="Pseudo" hint="3 à 50 caractères : lettres, chiffres, - et _">
        <input className="field" required minLength={3} maxLength={50} value={pseudo} onChange={(e) => setPseudo(e.target.value)} />
      </Field>
      <Field label="Bio"><textarea className="field" rows={3} maxLength={1000} value={bio} onChange={(e) => setBio(e.target.value)} /></Field>
      <fieldset style={{ border: 0, margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
        <legend style={{ fontWeight: 500, fontSize: 13, marginBottom: 6 }}>Compétences</legend>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {catalog.data?.map((c) => {
            const on = c.id in skills;
            return (
              <span key={c.id} style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                <button type="button" aria-pressed={on} onClick={() => setSkills((s) => { const n = { ...s }; if (on) delete n[c.id]; else n[c.id] = 'INTERMEDIAIRE'; return n; })}
                  style={{ height: 28, padding: '0 10px', borderRadius: 99, border: `1px solid ${on ? 'var(--tx)' : 'var(--bd2)'}`, background: on ? 'var(--tx)' : 'var(--s1)', color: on ? 'var(--bg)' : 'var(--tx2)', font: '500 12px var(--mono)', cursor: 'pointer' }}>
                  {c.libelle}
                </button>
                {on && (
                  <select aria-label={`Niveau ${c.libelle}`} value={skills[c.id]} onChange={(e) => setSkills((s) => ({ ...s, [c.id]: e.target.value as S['Level'] }))} style={{ fontSize: 11.5 }}>
                    {LEVELS.map((l) => <option key={l} value={l}>{LEVEL_LABEL[l]}</option>)}
                  </select>
                )}
              </span>
            );
          })}
        </div>
      </fieldset>
      <FormError>{save.error}</FormError>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <button type="submit" className="btn btn-primary" disabled={save.busy} style={small}>Enregistrer</button>
        <span style={{ flex: 1 }} />
        {confirmDelete ? (
          <>
            <span className="muted" style={{ fontSize: 12.5 }}>Anonymisation définitive. Sûr ?</span>
            <button type="button" className="btn" disabled={del.busy} onClick={remove} style={{ ...small, background: 'var(--fail)', color: '#fff' }}>Supprimer mon compte</button>
            <button type="button" className="btn btn-secondary" onClick={() => setConfirmDelete(false)} style={small}>Non</button>
          </>
        ) : (
          <button type="button" onClick={() => setConfirmDelete(true)} style={{ border: 0, background: 'none', color: 'var(--fail)', fontSize: 12.5, cursor: 'pointer' }}>
            <Icon name="trash-2" size={13} /> Supprimer mon compte
          </button>
        )}
      </div>
      <FormError>{del.error}</FormError>
    </form>
  );
}

/** Compte email sans GitHub (F18) : la liaison ouvre l'offre et la candidature. */
function LinkGitHub() {
  const link = useAction();
  const start = async () => {
    const r = await link.run(() => call(() => api.POST('/me/github/lier')));
    if (r) window.location.assign(r.url);
  };
  return (
    <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 className="h2">Lier GitHub</h2>
      <span className="muted" style={{ fontSize: 13.5 }}>
        Pour proposer tes services (offres, candidatures), lie ton compte GitHub : c’est lui qui prouve tes livraisons. Tu garderas ta connexion par email.
      </span>
      <FormError>{link.error}</FormError>
      <button type="button" className="btn btn-primary" disabled={link.busy} onClick={start} style={{ ...small, alignSelf: 'flex-start' }}>
        <Icon name="github" /> Lier mon compte GitHub
      </button>
    </section>
  );
}
