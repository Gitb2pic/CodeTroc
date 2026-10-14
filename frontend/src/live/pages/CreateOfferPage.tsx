import { useMemo, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useResource } from '../../api/useResource';
import { PageTitle } from '../../components/ui';
import { Field, FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

export function CreateOfferPage() {
  const packs = useResource(() => call(() => api.GET('/packs', { params: { query: { limit: 100 } } })), []);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 760 }}>
      <PageTitle title="Proposer une offre" sub="Choisis un pack standard, fixe ton prix et découpe-le en jalons." />
      <Loaded r={packs} what="Les packs">{(p) => <OfferForm packs={p.items} />}</Loaded>
    </div>
  );
}

function OfferForm({ packs }: { packs: S['Pack'][] }) {
  const navigate = useNavigate();
  const [packId, setPackId] = useState(packs[0]?.id ?? '');
  const pack = useMemo(() => packs.find((p) => p.id === packId), [packs, packId]);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [days, setDays] = useState(5);
  const [milestones, setMilestones] = useState<S['Milestone'][]>([{ titre: '', montant: 0 }]);
  const act = useAction();
  const price = milestones.reduce((s, m) => s + (m.montant || 0), 0);
  const inRange = pack ? price >= pack.prix_min && price <= pack.prix_max : false;

  const setMs = (i: number, patch: Partial<S['Milestone']>) => setMilestones((ms) => ms.map((m, j) => (j === i ? { ...m, ...patch } : m)));
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const o = await act.run(
      () => call(() => api.POST('/offres', { body: { id_pack: packId, titre: title.trim() || pack!.titre, description: description.trim() || undefined, prix_credits: price, delai_jours: days, jalons: milestones.map((m) => ({ titre: m.titre.trim(), montant: m.montant })) } })),
      'Offre publiée au catalogue',
    );
    if (o) navigate(R.offer(o.id));
  };

  return (
    <form onSubmit={submit} className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Field label="Pack" hint={pack ? `${pack.livrable} · prix entre ${pack.prix_min} et ${pack.prix_max} cr` : undefined}>
        <select className="field" value={packId} onChange={(e) => setPackId(e.target.value)}>
          {packs.map((p) => <option key={p.id} value={p.id}>{p.titre}</option>)}
        </select>
      </Field>
      <Field label="Titre de l'offre" hint="Par défaut, le titre du pack"><input className="field" maxLength={120} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={pack?.titre} /></Field>
      <Field label="Description"><textarea className="field" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} /></Field>
      <Field label="Délai (jours)"><input className="field" type="number" min={1} max={60} value={days} onChange={(e) => setDays(+e.target.value)} /></Field>
      <fieldset style={{ border: 0, margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
        <legend style={{ fontWeight: 500, fontSize: 13, marginBottom: 6 }}>Jalons (le prix est leur somme)</legend>
        {milestones.map((m, i) => (
          <div key={i} style={{ display: 'flex', gap: 8 }}>
            <input className="field" required maxLength={120} value={m.titre} onChange={(e) => setMs(i, { titre: e.target.value })} placeholder={`Jalon ${i + 1}`} aria-label={`Titre du jalon ${i + 1}`} style={{ flex: 1 }} />
            <input className="field" required type="number" min={1} value={m.montant || ''} onChange={(e) => setMs(i, { montant: +e.target.value })} aria-label={`Montant du jalon ${i + 1}`} style={{ width: 110 }} />
            {milestones.length > 1 && <button type="button" className="btn btn-secondary" style={small} onClick={() => setMilestones((ms) => ms.filter((_, j) => j !== i))} aria-label={`Retirer le jalon ${i + 1}`}>×</button>}
          </div>
        ))}
        {milestones.length < 6 && <button type="button" className="btn btn-secondary" style={{ ...small, alignSelf: 'flex-start' }} onClick={() => setMilestones((ms) => [...ms, { titre: '', montant: 0 }])}>+ Jalon</button>}
      </fieldset>
      <span className="mono" style={{ fontSize: 13, color: inRange ? 'var(--ac-tx)' : 'var(--wait)' }}>
        Prix : {price} cr{pack && !inRange ? ` (attendu entre ${pack.prix_min} et ${pack.prix_max})` : ''}
      </span>
      <FormError>{act.error}</FormError>
      <button type="submit" className="btn btn-primary" disabled={act.busy || !inRange} style={{ alignSelf: 'flex-start' }}>Publier l'offre</button>
    </form>
  );
}
