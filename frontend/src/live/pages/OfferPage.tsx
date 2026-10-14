import { useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { Avatar, Icon, Pill, Tag } from '../../components/ui';
import { plural, repRing } from '../../domain/logic';
import { initials } from '../format';
import { Field, FormError, Loaded, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

const VERIF: Record<S['Criterion']['verification'], string> = { CI: 'CI', SCAN: 'Scan', REVUE: 'Revue' };

export function OfferPage() {
  const { id = '' } = useParams();
  const offer = useResource(async () => {
    const o = await call(() => api.GET('/offres/{id}', { params: { path: { id } } }));
    const pack = await call(() => api.GET('/packs/{id}', { params: { path: { id: o.pack.id } } }));
    return { o, pack };
  }, [id]);
  return <Loaded r={offer} what="Cette offre">{({ o, pack }) => <OfferView o={o} pack={pack} />}</Loaded>;
}

function OfferView({ o, pack }: { o: S['Offer']; pack: S['Pack'] }) {
  const { user } = useSession();
  const navigate = useNavigate();
  const mine = user?.id === o.prestataire.id;
  const [repo, setRepo] = useState('');
  const [need, setNeed] = useState('');
  const req = useAction();
  const withdraw = useAction();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const x = await req.run(
      () => call(() => api.POST('/echanges', { body: { id_offre: o.id, depot_url: repo.trim(), besoin: need.trim() || undefined } })),
      `Demande envoyée à ${o.prestataire.pseudo}`,
    );
    if (x) navigate(R.exchange(x.id));
  };

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 360px', gap: 28, alignItems: 'start' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        <Link to={R.cat} className="mono faint" style={{ fontSize: 12.5 }}>← Catalogue</Link>
        <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>{o.pack.competences.map((c) => <Tag key={c}>{c}</Tag>)}</span>
        <h1 className="h1">{o.titre}</h1>
        <p className="muted" style={{ margin: 0, lineHeight: 1.55, whiteSpace: 'pre-wrap' }}>{o.description || pack.description}</p>
        <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h2 className="h2">Livrable</h2>
          <span style={{ fontSize: 14 }}>{pack.livrable}</span>
          <h2 className="h2" style={{ marginTop: 8 }}>Critères d'acceptation</h2>
          <ul style={{ margin: 0, paddingLeft: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 8 }}>
            {pack.criteres.map((c) => (
              <li key={c.texte} style={{ display: 'flex', gap: 10, alignItems: 'center', fontSize: 13.5 }}>
                <Icon name="circle-check" size={15} style={{ color: 'var(--ok)' }} />
                <span style={{ flex: 1 }}>{c.texte}</span>
                <Pill tone={c.verification === 'REVUE' ? 'mute' : 'ac'}>{VERIF[c.verification]}</Pill>
              </li>
            ))}
          </ul>
          <span className="faint" style={{ fontSize: 12 }}>
            Preuve : {pack.type_preuve === 'PR_CI' ? 'pull request mergée sur ton dépôt, CI au vert' : 'livraison déclarée, validée par toi'}.
          </span>
        </section>
        <section className="card" style={{ padding: 20 }}>
          <h2 className="h2" style={{ marginBottom: 12 }}>Jalons</h2>
          <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 10 }}>
            {o.jalons.map((m, i) => (
              <li key={i} style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                <span className="mono faint" style={{ fontSize: 12 }}>J{i + 1}</span>
                <span style={{ flex: 1 }}>{m.titre}</span>
                <span style={{ font: '600 13px var(--mono)', color: 'var(--ac-tx)' }}>{m.montant} cr</span>
              </li>
            ))}
          </ol>
        </section>
      </div>
      <aside style={{ position: 'sticky', top: 84, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div className="card" style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 14 }}>
          <span style={{ font: '600 28px var(--mono)', color: 'var(--ac-tx)' }}>{o.prix_credits} cr</span>
          <span className="mono muted" style={{ fontSize: 12.5 }}>{o.delai_jours} j · {plural(o.jalons.length, 'jalon')} · bloqués en séquestre à l'acceptation</span>
          {mine ? (
            <>
              <span className="muted" style={{ fontSize: 13 }}>C'est ton offre.</span>
              {o.statut === 'PUBLIEE' && (
                <button className="btn btn-secondary" disabled={withdraw.busy}
                  onClick={async () => (await withdraw.run(() => call(() => api.POST('/offres/{id}/retirer', { params: { path: { id: o.id } } })), 'Offre retirée du catalogue')) !== undefined && navigate(R.cat)}>
                  Retirer l'offre
                </button>
              )}
              <FormError>{withdraw.error}</FormError>
            </>
          ) : (
            <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <Field label="Dépôt GitHub concerné" hint="https://github.com/organisation/depot">
                <input className="field" required value={repo} onChange={(e) => setRepo(e.target.value)} placeholder="https://github.com/acme/shop-api" />
              </Field>
              <Field label="Ton besoin (facultatif)">
                <textarea className="field" value={need} onChange={(e) => setNeed(e.target.value)} maxLength={4000} rows={4} style={{ resize: 'vertical' }} />
              </Field>
              <FormError>{req.error}</FormError>
              <button type="submit" className="btn btn-primary" disabled={req.busy || o.statut !== 'PUBLIEE'} style={{ height: 42, fontSize: 14 }}>
                <Icon name="send" />
                Demander ce pack
              </button>
              <span className="faint" style={{ fontSize: 12 }}>Rien n'est débité tant que {o.prestataire.pseudo} n'a pas accepté.</span>
            </form>
          )}
        </div>
        <Link to={R.member(o.prestataire.id)} className="card hov-card" style={{ padding: 16, display: 'flex', alignItems: 'center', gap: 10, color: 'var(--tx)' }}>
          <Avatar ini={initials(o.prestataire.pseudo)} size={34} ring={repRing(o.prestataire.score_reputation)} fontSize={10} />
          <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
            <span style={{ fontSize: 13.5, fontWeight: 500 }}>{o.prestataire.pseudo}</span>
            <span className="mono faint" style={{ fontSize: 11.5 }}>
              {o.prestataire.score_reputation.toFixed(1)} ★ · {plural(o.prestataire.livraisons_verifiees, 'livraison vérifiée', 'livraisons vérifiées')}
            </span>
          </span>
        </Link>
      </aside>
    </div>
  );
}
