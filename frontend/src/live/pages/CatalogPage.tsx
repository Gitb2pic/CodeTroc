import { useState, type ReactNode } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api, call } from '../../api/client';
import { useResource } from '../../api/useResource';
import { PackCard } from '../../components/PackCard';
import { EmptyState, Icon, PageTitle, SegButtons } from '../../components/ui';
import { plural } from '../../domain/logic';
import { offerToPack } from '../format';
import { Loaded } from '../kit';
import { LIVE_ROUTES as R } from '../routes';
import { AnnouncementCard } from './AnnouncementPages';

export function CatalogPage() {
  const [params, setParams] = useSearchParams();
  const view = params.get('vue') === 'annonces' ? 'annonces' : 'offres';
  const setView = (v: 'offres' | 'annonces') => setParams(v === 'annonces' ? { vue: 'annonces' } : {}, { replace: true });
  const toggle = <SegButtons options={[['offres', 'Offres des prestataires'], ['annonces', "Demandes d'aide"]]} value={view} onChange={setView} />;
  return view === 'annonces' ? <AnnouncementsView toggle={toggle} /> : <OffersView toggle={toggle} />;
}

function AnnouncementsView({ toggle }: { toggle: ReactNode }) {
  const navigate = useNavigate();
  const [q, setQ] = useState('');
  const [scope, setScope] = useState<'toutes' | 'miennes'>('toutes');
  const mine = scope === 'miennes';
  const list = useResource(() => call(() => api.GET('/annonces', { params: { query: { q: q || undefined, mes: mine || undefined, limit: 50 } } })), [q, mine]);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageTitle
        title="Catalogue"
        sub="Des membres décrivent leur besoin : propose ton prix, ton délai et tes jalons."
        action={
          <button className="btn btn-primary" onClick={() => navigate(R.createAnnouncement)}>
            <Icon name="plus" />
            Publier une annonce
          </button>
        }
      />
      {toggle}
      <div className="filters">
        <label className="search" style={{ display: 'flex', alignItems: 'center', gap: 8, height: 40, padding: '0 12px', borderRadius: 9, border: '1px solid var(--bd2)', background: 'var(--s1)', color: 'var(--tx3)' }}>
          <Icon name="search" />
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="pipeline, audit, api…" aria-label="Rechercher une annonce" style={{ flex: 1, border: 0, background: 'none', color: 'var(--tx)', fontSize: 14, outline: 'none' }} />
        </label>
        <SegButtons options={[['toutes', 'Toutes'], ['miennes', 'Mes annonces']]} value={scope} onChange={setScope} />
      </div>
      <Loaded r={list} what="Les annonces">
        {(page) =>
          page.items.length === 0 ? (
            <EmptyState icon="hand" title="Aucune annonce" actions={<button className="btn btn-primary" onClick={() => navigate(R.createAnnouncement)} style={{ fontSize: 13 }}>Publier une annonce</button>}>
              Tu as besoin d'aide ? Décris ton besoin, les prestataires viendront à toi.
            </EmptyState>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: 12 }}>
              {page.items.map((a) => <AnnouncementCard key={a.id} a={a} onOpen={() => navigate(R.announcement(a.id))} />)}
            </div>
          )
        }
      </Loaded>
    </div>
  );
}

function OffersView({ toggle }: { toggle: ReactNode }) {
  const navigate = useNavigate();
  const [q, setQ] = useState('');
  const [maxPrice, setMaxPrice] = useState(0);
  const [maxDays, setMaxDays] = useState(0);
  const offers = useResource(
    () => call(() => api.GET('/offres', { params: { query: { q: q || undefined, prix_max: maxPrice || undefined, delai_max: maxDays || undefined, limit: 50 } } })),
    [q, maxPrice, maxDays],
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageTitle
        title="Catalogue"
        sub="Des livrables standardisés, payés en crédits, libérés sur preuve."
        action={
          <button className="btn btn-primary" onClick={() => navigate(R.create)}>
            <Icon name="plus" />
            Proposer une offre
          </button>
        }
      />
      {toggle}
      <div className="filters">
        <label className="search" style={{ display: 'flex', alignItems: 'center', gap: 8, height: 40, padding: '0 12px', borderRadius: 9, border: '1px solid var(--bd2)', background: 'var(--s1)', color: 'var(--tx3)' }}>
          <Icon name="search" />
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="docker, react, terraform…" aria-label="Rechercher une offre" style={{ flex: 1, border: 0, background: 'none', color: 'var(--tx)', fontSize: 14, outline: 'none' }} />
        </label>
        <SegButtons options={[[0, 'Tout prix'], [60, '≤ 60 cr'], [120, '≤ 120 cr']]} value={maxPrice} onChange={setMaxPrice} />
        <SegButtons options={[[0, 'Tout délai'], [3, '≤ 3 j'], [7, '≤ 7 j']]} value={maxDays} onChange={setMaxDays} />
      </div>
      <div data-testid="catalogue-resultats" aria-busy={offers.loading} style={{ opacity: offers.loading && offers.data ? 0.5 : 1, transition: 'opacity .15s', pointerEvents: offers.loading ? 'none' : undefined }}>
      <Loaded r={offers} what="Le catalogue">
        {(page) =>
          page.items.length === 0 ? (
            <EmptyState icon="search" title="Aucune offre ne correspond" actions={<button className="btn btn-primary" onClick={() => navigate(R.create)} style={{ fontSize: 13 }}>Proposer une offre</button>}>
              Élargis les filtres, ou propose toi-même ce pack.
            </EmptyState>
          ) : (
            <>
              <span className="mono muted" style={{ fontSize: 12.5 }} aria-live="polite">{plural(page.items.length, 'offre')}</span>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: 12 }}>
                {page.items.map((o) => (
                  <PackCard key={o.id} pack={offerToPack(o)} onOpen={() => navigate(R.offer(o.id))} />
                ))}
              </div>
            </>
          )
        }
      </Loaded>
      </div>
    </div>
  );
}
