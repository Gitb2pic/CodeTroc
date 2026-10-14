import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PackCard } from '../components/PackCard';
import { EmptyState, ErrorBanner, Icon, PageTitle, SegButtons, Skeleton, useEtat } from '../components/ui';
import { PACKS, STACK_FILTERS } from '../data/fixtures';
import { DEFAULT_FILTERS, filterPacks, plural, type PackFilters } from '../domain/logic';
import { ROUTES } from '../routes';

export function CatalogPage() {
  const navigate = useNavigate();
  const st = useEtat();
  const [f, setF] = useState<PackFilters>(DEFAULT_FILTERS);
  const set = (patch: Partial<PackFilters>) => setF((cur) => ({ ...cur, ...patch }));
  const list = filterPacks(PACKS, f);
  const reset = () => setF(DEFAULT_FILTERS);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageTitle
        title="Catalogue des packs"
        sub="Des livrables standardisés, payés en crédits, libérés sur preuve."
        action={
          <button className="btn btn-primary" onClick={() => navigate(ROUTES.create)}>
            <Icon name="plus" />
            Proposer un pack
          </button>
        }
      />
      <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
        <label style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 8, height: 40, padding: '0 12px', borderRadius: 9, border: '1px solid var(--bd2)', background: 'var(--s1)', color: 'var(--tx3)' }}>
          <Icon name="search" />
          <input value={f.q} onChange={(e) => set({ q: e.target.value })} placeholder="docker, react, terraform, nom d'un membre…" aria-label="Rechercher un pack" style={{ flex: 1, border: 0, background: 'none', color: 'var(--tx)', fontSize: 14, outline: 'none' }} />
        </label>
        <span className="mono muted" style={{ fontSize: 12.5, padding: '0 6px' }} aria-live="polite">{plural(list.length, 'pack')}</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '220px minmax(0,1fr)', gap: 24, alignItems: 'start' }}>
        <aside aria-label="Filtres" style={{ position: 'sticky', top: 84, display: 'flex', flexDirection: 'column', gap: 22 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span className="kicker">Stack</span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {STACK_FILTERS.map((n) => {
                const on = f.stack === n;
                const count = n === 'Toutes' ? PACKS.length : PACKS.filter((x) => x.stack.includes(n)).length;
                return (
                  <button key={n} onClick={() => set({ stack: n })} aria-pressed={on} style={{ display: 'flex', alignItems: 'center', gap: 8, height: 30, padding: '0 10px', borderRadius: 7, border: `1px solid ${on ? 'var(--tx)' : 'var(--bd)'}`, background: on ? 'var(--tx)' : 'var(--s1)', color: on ? 'var(--bg)' : 'var(--tx2)', font: '500 12.5px/1 var(--mono)', cursor: 'pointer', textAlign: 'left' }}>
                    <span style={{ flex: 1 }}>{n}</span>
                    <span style={{ opacity: 0.7 }}>{count}</span>
                  </button>
                );
              })}
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span className="kicker" style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span>Prix max</span>
              <span style={{ color: 'var(--ac-tx)', textTransform: 'none' }}>{f.maxPrice} cr</span>
            </span>
            <input type="range" min={40} max={200} step={10} value={f.maxPrice} onChange={(e) => set({ maxPrice: +e.target.value })} aria-label="Prix maximum en crédits" style={{ accentColor: 'var(--ac)', width: '100%' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span className="kicker">Délai</span>
            <SegButtons options={[[3, '≤ 3 j'], [5, '≤ 5 j'], [7, 'Tous']]} value={f.maxDays} onChange={(v) => set({ maxDays: v })} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <span className="kicker">Réputation</span>
            <SegButtons options={[[0, 'Toutes'], [4.5, '4.5+'], [4.8, '4.8+']]} value={f.minRep} onChange={(v) => set({ minRep: v })} />
          </div>
          <button onClick={reset} style={{ alignSelf: 'flex-start', display: 'inline-flex', alignItems: 'center', gap: 6, border: 0, background: 'none', color: 'var(--tx2)', fontSize: 12.5, cursor: 'pointer', padding: 0 }}>
            <Icon name="rotate-ccw" size={13} />
            Réinitialiser
          </button>
        </aside>
        <div>
          {st.normal && list.length > 0 && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: 12 }}>
              {list.map((p) => (
                <PackCard key={p.id} pack={p} onOpen={() => navigate(ROUTES.pack(p.id))} />
              ))}
            </div>
          )}
          {st.normal && list.length === 0 && (
            <EmptyState
              icon="search"
              title="Aucun pack ne correspond"
              actions={
                <>
                  <button className="btn btn-secondary" onClick={reset} style={{ height: 32, padding: '0 12px', borderRadius: 7, fontSize: 13 }}>Réinitialiser les filtres</button>
                  <button className="btn btn-primary" onClick={() => navigate(ROUTES.create)} style={{ height: 32, padding: '0 12px', borderRadius: 7, fontSize: 13 }}>Proposer ce pack</button>
                </>
              }
            >
              Élargis le prix ou le délai, ou propose toi-même ce pack : quelqu'un d'autre le cherche sûrement.
            </EmptyState>
          )}
          {st.loading && (
            <div aria-busy="true" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: 12 }}>
              {Array.from({ length: 6 }, (_, i) => (
                <div key={i} className="card" style={{ height: 196, padding: 16, boxSizing: 'border-box', display: 'flex', flexDirection: 'column', gap: 12 }}>
                  <Skeleton style={{ width: '40%', height: 14 }} />
                  <Skeleton style={{ width: '85%', height: 16 }} />
                  <Skeleton style={{ width: '60%', height: 12 }} />
                  <span style={{ flex: 1 }} />
                  <Skeleton style={{ width: '100%', height: 28, borderRadius: 6 }} />
                </div>
              ))}
            </div>
          )}
          {st.empty && (
            <EmptyState icon="inbox" title="Le catalogue de ta communauté est encore vide" actions={<button className="btn btn-primary" onClick={() => navigate(ROUTES.create)} style={{ height: 32, padding: '0 12px', borderRadius: 7, fontSize: 13 }}>Proposer un pack</button>}>
              Sois le premier ou la première à proposer un pack. Les premiers packs gagnent en visibilité.
            </EmptyState>
          )}
          {st.error && (
            <ErrorBanner>
              <strong style={{ fontWeight: 600 }}>Le catalogue n'a pas pu se charger.</strong> <span className="muted">Vérifie ta connexion puis réessaie.</span>
            </ErrorBanner>
          )}
        </div>
      </div>
    </div>
  );
}
