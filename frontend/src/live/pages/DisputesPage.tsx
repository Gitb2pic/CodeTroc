import { useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { api, call, idem, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { EmptyState, Icon, PageTitle, Pill } from '../../components/ui';
import { fmtDate, fmtTime, shortId } from '../format';
import { Field, FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

const DECISION: Record<NonNullable<S['Dispute']['decision']>, string> = { LIBERER: 'Libéré au prestataire', REMBOURSER: 'Remboursé au demandeur', PARTAGER: 'Partagé' };

export function DisputesPage() {
  const navigate = useNavigate();
  const list = useResource(() => call(() => api.GET('/litiges')), []);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageTitle title="Litiges" sub="Tes litiges, et ceux que tu arbitres." />
      <Loaded r={list} what="Les litiges">
        {(ds) =>
          ds.length === 0 ? (
            <EmptyState icon="scale" title="Aucun litige">Tant mieux. Un litige se lance depuis un jalon livré que tu contestes.</EmptyState>
          ) : (
            <ul className="card" style={{ margin: 0, padding: 0, listStyle: 'none', overflow: 'hidden' }}>
              {ds.map((d) => (
                <li key={d.id}>
                  <button className="hov-nav" onClick={() => navigate(R.dispute(d.id))} style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 18px', border: 0, borderBottom: '1px solid var(--bd)', background: 'none', color: 'var(--tx)', cursor: 'pointer', textAlign: 'left', font: 'inherit' }}>
                    <Icon name="flag" style={{ color: 'var(--disp)' }} />
                    <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                      <span style={{ fontWeight: 500 }}>Jalon « {d.jalon} »</span>
                      <span className="mono faint" style={{ fontSize: 12 }}>{shortId(d.id_echange)} · ouvert le {fmtDate(d.date_ouverture)}</span>
                    </span>
                    <Pill tone={d.statut === 'OUVERT' ? 'disp' : 'mute'}>{d.statut === 'OUVERT' ? 'Ouvert' : 'Clos'}</Pill>
                    <span className="mono" style={{ width: 70, textAlign: 'right' }}>{d.montant} cr</span>
                  </button>
                </li>
              ))}
            </ul>
          )
        }
      </Loaded>
    </div>
  );
}

export function DisputePage() {
  const { id = '' } = useParams();
  const data = useResource(async () => {
    const d = await call(() => api.GET('/litiges/{id}', { params: { path: { id } } }));
    const x = await call(() => api.GET('/echanges/{id}', { params: { path: { id: d.id_echange } } }));
    return { d, x };
  }, [id]);
  return <Loaded r={data} what="Ce litige">{({ d, x }) => <DisputeView d={d} x={x} reload={data.reload} />}</Loaded>;
}

function DisputeView({ d, x, reload }: { d: S['Dispute']; x: S['Exchange']; reload(): void }) {
  const { user } = useSession();
  const me = user!.id;
  const isArbiter = d.id_arbitre === me;
  const canRespond = d.statut === 'OUVERT' && !d.reponse && me !== d.ouvert_par && (me === x.demandeur.id || me === x.prestataire.id);
  const opener = d.ouvert_par === x.demandeur.id ? x.demandeur : x.prestataire;
  const proofs = x.jalons.find((m) => m.id === d.id_jalon)?.preuves ?? [];

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 360px', gap: 24, alignItems: 'start' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        <Link to={R.exchange(x.id)} className="mono faint" style={{ fontSize: 12.5 }}>← {shortId(x.id)} · {x.offre.titre}</Link>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <h1 className="h1" style={{ flex: 1 }}>Litige sur « {d.jalon} »</h1>
          <Pill tone={d.statut === 'OUVERT' ? 'disp' : 'mute'}>{d.statut === 'OUVERT' ? 'Ouvert' : 'Clos'}</Pill>
        </div>
        <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="kicker">Motif · {opener.pseudo} · {fmtTime(d.date_ouverture)}</span>
          <p style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{d.motif}</p>
        </section>
        <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="kicker">Réponse</span>
          {d.reponse ? <p style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{d.reponse}</p> : <span className="faint">Pas encore de réponse.</span>}
          {canRespond && <RespondForm id={d.id} onDone={reload} />}
        </section>
        <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span className="kicker">Preuves du jalon</span>
          {proofs.length === 0 && <span className="faint">Aucune preuve.</span>}
          {proofs.map((p) => (
            <span key={p.id} style={{ fontSize: 13 }}>
              {p.pr_url ? <a href={p.pr_url} target="_blank" rel="noreferrer noopener">PR #{p.pr_numero}</a> : p.note} · {p.statut}{p.detail ? ` · ${p.detail}` : ''}
            </span>
          ))}
        </section>
      </div>
      <aside style={{ position: 'sticky', top: 84, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span className="kicker">Montant gelé</span>
          <span style={{ font: '600 26px var(--mono)', color: 'var(--ac-tx)' }}>{d.montant} cr</span>
          <span className="muted" style={{ fontSize: 13 }}>{d.id_arbitre ? (isArbiter ? 'Tu arbitres ce litige.' : 'Un arbitre est affecté.') : "En attente d'un arbitre."}</span>
        </div>
        {d.statut === 'CLOS' && d.decision && (
          <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span className="kicker">Décision · {fmtDate(d.date_cloture)}</span>
            <span style={{ fontWeight: 600 }}>{DECISION[d.decision]}{d.decision === 'PARTAGER' ? ` · ${d.part_prestataire} % au prestataire` : ''}</span>
            <p className="muted" style={{ margin: 0, fontSize: 13, whiteSpace: 'pre-wrap' }}>{d.justification}</p>
          </div>
        )}
        {isArbiter && d.statut === 'OUVERT' && <DecideForm d={d} onDone={reload} />}
      </aside>
    </div>
  );
}

function RespondForm({ id, onDone }: { id: string; onDone(): void }) {
  const [text, setText] = useState('');
  const act = useAction();
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if ((await act.run(() => call(() => api.POST('/litiges/{id}/reponse', { params: { path: { id } }, body: { reponse: text.trim() } })), 'Réponse envoyée à l\'arbitre')) !== undefined) onDone();
  };
  return (
    <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <textarea className="field" required rows={4} maxLength={4000} value={text} onChange={(e) => setText(e.target.value)} aria-label="Ta réponse" placeholder="Ta version des faits (une seule réponse possible)" />
      <FormError>{act.error}</FormError>
      <button type="submit" className="btn btn-primary" disabled={act.busy} style={small}>Répondre</button>
    </form>
  );
}

function DecideForm({ d, onDone }: { d: S['Dispute']; onDone(): void }) {
  const [kind, setKind] = useState<NonNullable<S['Dispute']['decision']>>('LIBERER');
  const [share, setShare] = useState(50);
  const [why, setWhy] = useState('');
  const act = useAction();
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const body = { decision: kind, justification: why.trim(), ...(kind === 'PARTAGER' ? { part_prestataire: share } : {}) };
    if ((await act.run(() => call(() => api.POST('/litiges/{id}/decision', { params: { path: { id: d.id }, header: idem() }, body })), 'Décision appliquée au séquestre')) !== undefined) onDone();
  };
  const provider = kind === 'LIBERER' ? d.montant : kind === 'REMBOURSER' ? 0 : Math.floor((d.montant * share) / 100);
  return (
    <form onSubmit={submit} className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <h2 className="h2">Trancher</h2>
      <Field label="Décision">
        <select className="field" value={kind} onChange={(e) => setKind(e.target.value as typeof kind)}>
          <option value="LIBERER">Libérer au prestataire</option>
          <option value="REMBOURSER">Rembourser le demandeur</option>
          <option value="PARTAGER">Partager</option>
        </select>
      </Field>
      {kind === 'PARTAGER' && (
        <Field label={`Part du prestataire : ${share} %`}>
          <input type="range" min={1} max={99} value={share} onChange={(e) => setShare(+e.target.value)} />
        </Field>
      )}
      <span className="mono muted" style={{ fontSize: 12.5 }}>{provider} cr au prestataire · {d.montant - provider} cr au demandeur</span>
      <Field label="Justification" hint="Visible des deux parties, 10 caractères minimum">
        <textarea className="field" required minLength={10} maxLength={4000} rows={4} value={why} onChange={(e) => setWhy(e.target.value)} />
      </Field>
      <FormError>{act.error}</FormError>
      <button type="submit" className="btn btn-primary" disabled={act.busy}>
        <Icon name="gavel" />
        Rendre la décision
      </button>
    </form>
  );
}
