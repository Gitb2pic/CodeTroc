import { useEffect, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { api, call, idem, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { Avatar, EscrowBar, Icon, Pill } from '../../components/ui';
import { percent } from '../../domain/logic';
import { EXCHANGE_STATUS, fmtDate, fmtTime, initials, MILESTONE_STATUS, shortId } from '../format';
import { Field, FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';
import { VaultCard } from './VaultCard';

const POLL_MS = 20_000;

export function ExchangePage() {
  const { id = '' } = useParams();
  const ex = useResource(() => call(() => api.GET('/echanges/{id}', { params: { path: { id } } })), [id]);
  // Les livraisons arrivent par webhook : on rafraîchit régulièrement.
  useEffect(() => {
    const t = setInterval(ex.reload, POLL_MS);
    return () => clearInterval(t);
  }, [ex.reload]);
  return <Loaded r={ex} what="Cet échange">{(x) => <ExchangeView x={x} reload={ex.reload} />}</Loaded>;
}

type Role = 'DEMANDEUR' | 'PRESTATAIRE' | 'ARBITRE';

function ExchangeView({ x, reload }: { x: S['Exchange']; reload(): void }) {
  const { user } = useSession();
  const me = user!.id;
  const role: Role = me === x.demandeur.id ? 'DEMANDEUR' : me === x.prestataire.id ? 'PRESTATAIRE' : 'ARBITRE';
  const other = role === 'DEMANDEUR' ? x.prestataire : x.demandeur;
  const [label, tone] = EXCHANGE_STATUS[x.statut];
  const yourTurn = x.a_qui_de_jouer === role;
  const total = x.montant_total;
  const current = x.jalons.find((m) => m.statut === 'EN_COURS' || m.statut === 'LIVRE');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap' }}>
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className="mono faint" style={{ fontSize: 12 }}>{shortId(x.id)} · ouvert le {fmtDate(x.date_creation)} · {x.depot_url.replace('https://github.com/', '')}</span>
          <h1 className="h1">{x.offre.titre}</h1>
        </div>
        <Pill tone={tone}>{label}</Pill>
        <span className="muted" style={{ fontSize: 13 }}>
          {role === 'ARBITRE' ? `${x.demandeur.pseudo} ⇄ ${x.prestataire.pseudo}` : <>Toi ⇄ <Link to={R.member(other.id)}>{other.pseudo}</Link></>}
        </span>
      </div>

      <NextStep x={x} role={role} yourTurn={yourTurn} reload={reload} />

      <div className="split split-340">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <section className="card" style={{ padding: 20 }}>
            <h2 className="h2" style={{ marginBottom: 18 }}>Jalons</h2>
            <ol style={{ margin: 0, padding: 0, listStyle: 'none' }}>
              {x.jalons.map((m) => (
                <MilestoneRow key={m.id} x={x} m={m} role={role} reload={reload} />
              ))}
            </ol>
          </section>
          {x.besoin && (
            <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 8 }}>
              <h2 className="h2">Besoin exprimé</h2>
              <p style={{ margin: 0, whiteSpace: 'pre-wrap', fontSize: 13.5 }}>{x.besoin}</p>
            </section>
          )}
          <Messages exchangeId={x.id} canWrite={role !== 'ARBITRE'} other={other.pseudo} />
        </div>

        <aside className="sticky-aside">
          <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Icon name="lock" style={{ color: 'var(--ac-tx)' }} />
              <h2 className="h2" style={{ flex: 1 }}>Séquestre</h2>
              <span style={{ font: '600 14px var(--mono)' }}>{total} cr</span>
            </div>
            <span role="img" aria-label={`Libéré ${x.sequestre.libere} cr, bloqué ${x.sequestre.bloque} cr`}>
              <EscrowBar released={percent(x.sequestre.libere, total)} current={percent(current?.montant ?? 0, total)} />
            </span>
            <dl style={{ margin: 0, display: 'grid', gridTemplateColumns: '1fr auto', gap: '6px 8px', fontSize: 13 }}>
              <dt className="muted">Libéré</dt><dd className="mono" style={{ margin: 0 }}>{x.sequestre.libere} cr</dd>
              <dt className="muted">Encore bloqué</dt><dd className="mono" style={{ margin: 0 }}>{x.sequestre.bloque} cr</dd>
              <dt className="muted">Remboursé</dt><dd className="mono" style={{ margin: 0 }}>{x.sequestre.rembourse} cr</dd>
            </dl>
          </div>
          {role !== 'ARBITRE' && ['EN_COURS', 'EN_LITIGE', 'TERMINE', 'ANNULE'].includes(x.statut) && (
            <VaultCard x={x} role={role} />
          )}
          <Link to={R.member(other.id)} className="card hov-card" style={{ padding: 16, display: 'flex', alignItems: 'center', gap: 10, color: 'var(--tx)' }}>
            <Avatar ini={initials(other.pseudo)} size={32} fontSize={9} />
            <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
              <span style={{ fontSize: 13, fontWeight: 500 }}>{other.pseudo}</span>
              <span className="mono faint" style={{ fontSize: 11.5 }}>@{other.github_login} · {other.score_reputation.toFixed(1)} ★</span>
            </span>
          </Link>
          {x.statut === 'TERMINE' && role !== 'ARBITRE' && <ReviewForm exchangeId={x.id} other={other.pseudo} />}
          {x.statut === 'EN_COURS' && role !== 'ARBITRE' && <CancelButton x={x} me={me} reload={reload} />}
        </aside>
      </div>
    </div>
  );
}

function NextStep({ x, role, yourTurn, reload }: { x: S['Exchange']; role: Role; yourTurn: boolean; reload(): void }) {
  const act = useAction();
  const path = { params: { path: { id: x.id }, header: idem() } };
  let title = '';
  let sub = '';
  if (x.statut === 'PROPOSE') {
    title = role === 'PRESTATAIRE' ? 'Accepte ou refuse cette demande' : `En attente de ${x.prestataire.pseudo}`;
    sub = `${x.montant_total} cr seront bloqués en séquestre à l'acceptation.`;
  } else if (x.statut === 'EN_COURS') {
    const live = x.jalons.find((m) => m.statut === 'EN_COURS' || m.statut === 'LIVRE');
    if (live?.statut === 'LIVRE') {
      title = role === 'DEMANDEUR' ? `Valide le jalon « ${live.titre} »` : `${x.demandeur.pseudo} vérifie ta livraison`;
      sub = 'Sans réponse sous 72 h, le jalon est validé automatiquement.';
    } else if (live) {
      title = role === 'PRESTATAIRE' ? `Livre le jalon « ${live.titre} »` : `${x.prestataire.pseudo} travaille sur « ${live.titre} »`;
      sub = live.echeance ? `Échéance : ${fmtDate(live.echeance)}` : '';
    }
  } else if (x.statut === 'EN_LITIGE') {
    title = 'Litige en cours';
    sub = "Le séquestre est gelé jusqu'à la décision de l'arbitre.";
  } else {
    title = EXCHANGE_STATUS[x.statut][0];
    sub = x.date_cloture ? `Clos le ${fmtDate(x.date_cloture)}` : '';
  }

  return (
    <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '16px 18px', borderRadius: 12, border: `1px solid ${yourTurn ? 'var(--ac)' : 'var(--bd2)'}`, background: yourTurn ? 'color-mix(in oklab,var(--ac) 12%,var(--s1))' : 'var(--s1)' }}>
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span className="kicker" style={{ color: yourTurn ? 'var(--ac-tx)' : undefined }}>{yourTurn ? 'Maintenant · à toi' : 'Maintenant'}</span>
        <span style={{ fontSize: 16, fontWeight: 600 }}>{title}</span>
        {sub && <span className="muted" style={{ fontSize: 13 }}>{sub}</span>}
        <FormError>{act.error}</FormError>
      </div>
      {x.statut === 'PROPOSE' && role === 'PRESTATAIRE' && (
        <>
          <button className="btn btn-secondary" disabled={act.busy} onClick={() => act.run(() => call(() => api.POST('/echanges/{id}/refuser', path)), 'Demande refusée').then(reload)}>Refuser</button>
          <button className="btn btn-primary" disabled={act.busy} onClick={() => act.run(() => call(() => api.POST('/echanges/{id}/accepter', path)), `Échange accepté · ${x.montant_total} cr en séquestre`).then(reload)}>
            <Icon name="check" />
            Accepter
          </button>
        </>
      )}
      {x.statut === 'PROPOSE' && role === 'DEMANDEUR' && (
        <button className="btn btn-secondary" disabled={act.busy} onClick={() => act.run(() => call(() => api.POST('/echanges/{id}/annuler', path)), 'Demande retirée').then(reload)}>Retirer la demande</button>
      )}
    </div>
  );
}

function MilestoneRow({ x, m, role, reload }: { x: S['Exchange']; m: S['ExchangeMilestone']; role: Role; reload(): void }) {
  const navigate = useNavigate();
  const [label, tone, icon] = MILESTONE_STATUS[m.statut];
  const act = useAction();
  const [prUrl, setPrUrl] = useState('');
  const [note, setNote] = useState('');
  const [reason, setReason] = useState('');
  const [disputing, setDisputing] = useState(false);
  const mpath = { params: { path: { id: m.id } } };
  const manual = x.offre.type_preuve === 'MANUELLE';

  const deliver = async (e: FormEvent) => {
    e.preventDefault();
    const body = manual ? { note: note.trim() } : { pr_url: prUrl.trim() };
    if ((await act.run(() => call(() => api.POST('/jalons/{id}/preuves', { ...mpath, body })), 'Preuve envoyée, vérification en cours')) !== undefined) {
      setPrUrl('');
      setNote('');
      reload();
    }
  };
  const openDispute = async (e: FormEvent) => {
    e.preventDefault();
    const d = await act.run(() => call(() => api.POST('/jalons/{id}/litiges', { ...mpath, body: { motif: reason.trim() } })), 'Litige ouvert, le séquestre est gelé');
    if (d) navigate(R.dispute(d.id));
  };

  return (
    <li style={{ display: 'grid', gridTemplateColumns: '28px minmax(0,1fr)', gap: 14 }}>
      <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <span style={{ width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', border: '1px solid var(--bd2)', background: 'var(--s2)' }}>
          <Icon name={icon} size={15} spin={m.statut === 'EN_COURS'} />
        </span>
        <span style={{ flex: 1, width: 2, minHeight: 18, background: 'var(--bd2)', margin: '4px 0' }} />
      </span>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '3px 0 20px', minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
          <span className="mono faint" style={{ fontSize: 12 }}>J{m.ordre}</span>
          <span style={{ fontWeight: 600, flex: 1, minWidth: 0 }}>{m.titre}</span>
          <Pill tone={tone}>{label}</Pill>
          <span style={{ font: '600 13px var(--mono)', color: 'var(--ac-tx)' }}>{m.montant} cr</span>
        </div>
        {m.echeance && <span className="mono faint" style={{ fontSize: 12 }}>échéance {fmtDate(m.echeance)}</span>}
        {m.preuves.map((p) => (
          <div key={p.id} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 10, border: '1px solid var(--bd)', background: 'var(--bg)', fontSize: 13 }}>
            <Icon name={p.statut === 'VALIDE' ? 'git-merge' : p.statut === 'REJETEE' ? 'circle-x' : 'git-pull-request'} style={{ color: p.statut === 'VALIDE' ? 'var(--ok)' : p.statut === 'REJETEE' ? 'var(--fail)' : 'var(--wait)' }} />
            <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
              {p.pr_url ? <a href={p.pr_url} target="_blank" rel="noreferrer noopener">PR #{p.pr_numero}{p.sha_commit ? ` · ${p.sha_commit.slice(0, 7)}` : ''}</a> : <span>{p.note}</span>}
              {p.detail && <span className="faint" style={{ fontSize: 12 }}>{p.detail}</span>}
            </span>
            <span className="mono faint" style={{ fontSize: 11.5 }}>{fmtTime(p.date_verification ?? p.date_creation)}</span>
          </div>
        ))}
        {role === 'PRESTATAIRE' && m.statut === 'EN_COURS' && (
          <form onSubmit={deliver} style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
            {manual ? (
              <input className="field" required value={note} onChange={(e) => setNote(e.target.value)} maxLength={2000} placeholder="Décris ce qui a été livré" aria-label="Note de livraison" style={{ flex: 1 }} />
            ) : (
              <input className="field" required value={prUrl} onChange={(e) => setPrUrl(e.target.value)} placeholder={`${x.depot_url}/pull/42`} aria-label="URL de la pull request" style={{ flex: 1 }} />
            )}
            <button type="submit" className="btn btn-primary" disabled={act.busy} style={{ ...small, height: 38 }}>Livrer</button>
            {m.preuves.some((p) => p.statut === 'EN_ATTENTE') && (
              <button type="button" className="btn btn-secondary" disabled={act.busy} style={small}
                onClick={() => act.run(() => call(() => api.POST('/jalons/{id}/reverifier', mpath)), 'Vérification relancée').then(reload)}>
                <Icon name="refresh-cw" size={13} />
                Revérifier
              </button>
            )}
          </form>
        )}
        {role === 'DEMANDEUR' && m.statut === 'LIVRE' && !disputing && (
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-accent hov-bright" disabled={act.busy} style={{ ...small, height: 34 }}
              onClick={() => act.run(() => call(() => api.POST('/jalons/{id}/valider', { params: { path: { id: m.id }, header: idem() } })), `${m.montant} cr libérés à ${x.prestataire.pseudo}`).then(reload)}>
              <Icon name="lock-open" size={14} />
              Valider · libérer {m.montant} cr
            </button>
            <button className="btn" onClick={() => setDisputing(true)} style={{ ...small, height: 34, border: '1px solid color-mix(in oklab,var(--disp) 40%,transparent)', background: 'transparent', color: 'var(--disp)' }}>
              <Icon name="flag" size={13} />
              Contester
            </button>
          </div>
        )}
        {disputing && (
          <form onSubmit={openDispute} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <Field label="Motif du litige" hint="10 caractères minimum : quel critère n'est pas rempli ?">
              <textarea className="field" required minLength={10} maxLength={4000} rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
            </Field>
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="submit" className="btn btn-primary" disabled={act.busy} style={small}>Ouvrir le litige</button>
              <button type="button" className="btn btn-secondary" onClick={() => setDisputing(false)} style={small}>Annuler</button>
            </div>
          </form>
        )}
        <FormError>{act.error}</FormError>
      </div>
    </li>
  );
}

function Messages({ exchangeId, canWrite, other }: { exchangeId: string; canWrite: boolean; other: string }) {
  const { user } = useSession();
  const msgs = useResource(() => call(() => api.GET('/echanges/{id}/messages', { params: { path: { id: exchangeId }, query: { limit: 50 } } })), [exchangeId]);
  const [text, setText] = useState('');
  const act = useAction();
  useEffect(() => {
    const t = setInterval(msgs.reload, POLL_MS);
    return () => clearInterval(t);
  }, [msgs.reload]);

  const send = async (e: FormEvent) => {
    e.preventDefault();
    if (!text.trim()) return;
    if ((await act.run(() => call(() => api.POST('/echanges/{id}/messages', { params: { path: { id: exchangeId } }, body: { contenu: text } })))) !== undefined) {
      setText('');
      msgs.reload();
    }
  };

  const items = [...(msgs.data?.items ?? [])].reverse();
  return (
    <section className="card" style={{ overflow: 'hidden' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '14px 18px', borderBottom: '1px solid var(--bd)' }}>
        <Icon name="message-square" style={{ color: 'var(--tx3)' }} />
        <h2 className="h2">Discussion</h2>
      </div>
      <div aria-live="polite" style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: 18 }}>
        {items.length === 0 && <span className="faint" style={{ fontSize: 13 }}>Aucun message pour l'instant.</span>}
        {items.map((m) => {
          const mine = m.id_auteur === user?.id;
          return (
            <div key={m.id} style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
              <Avatar ini={initials(m.auteur)} size={28} fontSize={10} />
              <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4, padding: '10px 12px', borderRadius: 10, border: '1px solid var(--bd)', background: mine ? 'color-mix(in oklab, var(--ac) 10%, var(--s1))' : 'var(--s1)' }}>
                <span style={{ fontSize: 12.5 }}>
                  <strong style={{ fontWeight: 600 }}>{mine ? 'Toi' : m.auteur}</strong> <span className="mono faint" style={{ fontSize: 11.5 }}>{fmtTime(m.date)}</span>
                </span>
                {/* Texte rendu tel quel : React échappe le contenu. */}
                <span style={{ fontSize: 13.5, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{m.contenu}</span>
              </div>
            </div>
          );
        })}
      </div>
      {canWrite && (
        <form onSubmit={send} style={{ display: 'flex', gap: 8, padding: '12px 18px', borderTop: '1px solid var(--bd)' }}>
          <input className="field" value={text} onChange={(e) => setText(e.target.value)} maxLength={5000} placeholder={`Écris à ${other}…`} aria-label="Message" style={{ flex: 1, height: 36, fontSize: 13.5 }} />
          <button type="submit" className="btn btn-primary" disabled={act.busy} style={{ height: 36, fontSize: 13, gap: 6 }}>
            <Icon name="send" size={14} />
            Envoyer
          </button>
        </form>
      )}
      {act.error && <div style={{ padding: '0 18px 12px' }}><FormError>{act.error}</FormError></div>}
    </section>
  );
}

function ReviewForm({ exchangeId, other }: { exchangeId: string; other: string }) {
  const [rating, setRating] = useState(5);
  const [comment, setComment] = useState('');
  const [done, setDone] = useState(false);
  const act = useAction();
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if ((await act.run(() => call(() => api.POST('/echanges/{id}/avis', { params: { path: { id: exchangeId } }, body: { note: rating, commentaire: comment.trim() || undefined } })), 'Merci pour ton avis')) !== undefined) setDone(true);
  };
  if (done) return null;
  return (
    <form onSubmit={submit} className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 className="h2">Ton avis sur {other}</h2>
      <div role="radiogroup" aria-label="Note" style={{ display: 'flex', gap: 4 }}>
        {[1, 2, 3, 4, 5].map((n) => (
          <button key={n} type="button" role="radio" aria-checked={rating === n} aria-label={`${n} sur 5`} onClick={() => setRating(n)}
            style={{ border: 0, background: 'none', cursor: 'pointer', color: n <= rating ? 'var(--ac-tx)' : 'var(--tx3)', fontSize: 20 }}>★</button>
        ))}
      </div>
      <textarea className="field" rows={3} maxLength={2000} value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Facultatif" aria-label="Commentaire" />
      <FormError>{act.error}</FormError>
      <button type="submit" className="btn btn-primary" disabled={act.busy} style={small}>Publier l'avis</button>
    </form>
  );
}

function CancelButton({ x, me, reload }: { x: S['Exchange']; me: string; reload(): void }) {
  const act = useAction();
  const asked = x.annulation_demandee_par;
  const text = asked === me ? 'Annulation demandée, en attente' : asked ? "Accepter l'annulation" : "Demander l'annulation";
  return (
    <>
      <button className="btn btn-secondary" disabled={act.busy || asked === me} style={{ height: 36, fontSize: 13 }}
        onClick={() => act.run(() => call(() => api.POST('/echanges/{id}/annuler', { params: { path: { id: x.id }, header: idem() } })), asked ? 'Échange annulé, crédits restants remboursés' : "Demande d'annulation envoyée").then(reload)}>
        <Icon name="undo-2" size={14} />
        {text}
      </button>
      <span className="faint" style={{ fontSize: 11.5, textAlign: 'center' }}>Les jalons déjà libérés restent acquis ; le reste est remboursé.</span>
      <FormError>{act.error}</FormError>
    </>
  );
}
