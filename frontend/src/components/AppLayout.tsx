import { useEffect } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { ME } from '../data/fixtures';
import { balances, repRing } from '../domain/logic';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';
import { Avatar, Icon, Logo } from './ui';

type NavKey = 'dash' | 'cat' | 'ex' | 'vault' | 'wallet' | 'notif' | 'dispute' | 'profile';

function screenOf(path: string): { key: NavKey | ''; title: string } {
  if (path.startsWith('/catalogue/')) return { key: 'cat', title: 'Catalogue / Détail du pack' };
  if (path === ROUTES.cat) return { key: 'cat', title: 'Catalogue des packs' };
  if (path === ROUTES.create) return { key: '', title: 'Proposer un pack' };
  if (path.startsWith('/echanges')) return { key: 'ex', title: 'Échanges / EX-0142' };
  if (path === ROUTES.vault) return { key: 'vault', title: "Coffre d'accès" };
  if (path === ROUTES.wallet) return { key: 'wallet', title: 'Portefeuille' };
  if (path.startsWith('/membres')) return { key: 'profile', title: 'Profil public' };
  if (path.startsWith('/litiges')) return { key: 'dispute', title: 'Litiges / EX-0131' };
  if (path === ROUTES.notif) return { key: 'notif', title: 'Notifications' };
  return { key: 'dash', title: 'Tableau de bord' };
}

export function AppLayout() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { exPack, flow, readAll, toggleTheme, theme } = useApp();
  const { key: active, title } = screenOf(pathname);
  const bal = balances(exPack, flow);

  useEffect(() => {
    window.scrollTo({ top: 0 });
  }, [pathname]);

  const nav: [NavKey, string, string, string | null][] = [
    ['dash', 'Tableau de bord', 'layout-dashboard', null],
    ['cat', 'Catalogue', 'package', null],
    ['ex', 'Échanges', 'arrow-left-right', '3'],
    ['vault', "Coffre d'accès", 'vault', null],
    ['wallet', 'Portefeuille', 'wallet', null],
    ['notif', 'Notifications', 'bell', readAll ? null : '5'],
    ['dispute', 'Litiges', 'scale', '1'],
    ['profile', 'Profil', 'user', null],
  ];

  const sideLink = { display: 'flex', alignItems: 'center', gap: 10, height: 30, padding: '0 10px', border: 0, borderRadius: 7, background: 'none', color: 'var(--tx2)', font: '13px var(--sans)', cursor: 'pointer' } as const;

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '236px minmax(0,1fr)', minHeight: '100vh', minWidth: 1280 }}>
      <aside style={{ position: 'sticky', top: 0, height: '100vh', boxSizing: 'border-box', borderRight: '1px solid var(--bd)', background: 'var(--s1)', display: 'flex', flexDirection: 'column', gap: 2, padding: '14px 10px' }}>
        <button onClick={() => navigate(ROUTES.landing)} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '4px 8px 14px', border: 0, background: 'none', color: 'var(--tx)', font: '600 15px var(--sans)', cursor: 'pointer' }}>
          <Logo />
          CodeTroc
        </button>
        <nav aria-label="Navigation principale" style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {nav.map(([k, label, icon, count]) => {
            const on = active === k;
            return (
              <button
                key={k}
                className="hov-nav"
                onClick={() => navigate(ROUTES[k])}
                aria-current={on ? 'page' : undefined}
                style={{ display: 'flex', alignItems: 'center', gap: 10, height: 34, padding: '0 10px', borderRadius: 7, border: 0, cursor: 'pointer', font: '500 13.5px/1 var(--sans)', textAlign: 'left', background: on ? 'var(--s3)' : 'transparent', color: on ? 'var(--tx)' : 'var(--tx2)' }}
              >
                <Icon name={icon} />
                <span style={{ flex: 1 }}>{label}</span>
                {count && <span style={{ minWidth: 18, height: 18, padding: '0 5px', boxSizing: 'border-box', borderRadius: 99, background: 'var(--s3)', color: 'var(--tx)', font: '500 11px/18px var(--mono)', textAlign: 'center' }}>{count}</span>}
              </button>
            );
          })}
        </nav>
        <button className="btn btn-secondary" onClick={() => navigate(ROUTES.create)} style={{ marginTop: 12, fontSize: 13 }}>
          <Icon name="plus" />
          Proposer un pack
        </button>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '10px 0', borderTop: '1px solid var(--bd)' }}>
          <span className="kicker" style={{ padding: '4px 10px', fontSize: 10.5 }}>Pages publiques</span>
          <button className="hov-nav" onClick={() => navigate(ROUTES.landing)} style={sideLink}>
            <Icon name="external-link" size={14} />
            Landing
          </button>
          <button className="hov-nav" onClick={() => navigate(ROUTES.onboarding)} style={sideLink}>
            <Icon name="play" size={14} />
            Onboarding
          </button>
        </div>
        <button onClick={() => navigate(ROUTES.profile)} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 8px 2px', border: 0, borderTop: '1px solid var(--bd)', background: 'none', color: 'var(--tx)', cursor: 'pointer', textAlign: 'left' }}>
          <Avatar ini={ME.ini} size={30} ring={repRing(ME.rep)} fontSize={10} />
          <span style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.25 }}>
            <span style={{ fontSize: 13, fontWeight: 500 }}>{ME.name}</span>
            <span className="mono faint" style={{ fontSize: 11.5 }}>@{ME.handle} · {ME.rep} ★</span>
          </span>
        </button>
      </aside>

      <main style={{ minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <div style={{ position: 'sticky', top: 0, zIndex: 5, height: 56, display: 'flex', alignItems: 'center', gap: 16, padding: '0 28px', borderBottom: '1px solid var(--bd)', background: 'color-mix(in oklab,var(--bg) 85%,transparent)', backdropFilter: 'blur(12px)' }}>
          <span style={{ fontWeight: 600, fontSize: 14, whiteSpace: 'nowrap' }}>{title}</span>
          <div style={{ flex: '1 1 0', minWidth: 0, display: 'flex', justifyContent: 'center' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, width: '100%', maxWidth: 380, height: 32, padding: '0 10px', borderRadius: 8, border: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx3)', fontSize: 13, boxSizing: 'border-box' }}>
              <Icon name="search" size={14} />
              <input placeholder="Rechercher un pack, un membre, un échange…" aria-label="Recherche globale" style={{ flex: 1, minWidth: 0, border: 0, background: 'none', color: 'var(--tx)', fontSize: 13, outline: 'none' }} />
              <span className="mono" style={{ fontSize: 11, padding: '2px 5px', border: '1px solid var(--bd2)', borderRadius: 4 }}>⌘K</span>
            </label>
          </div>
          <button onClick={() => navigate(ROUTES.wallet)} aria-label={`Solde : ${bal.available} crédits disponibles, ${bal.escrowTotal} en séquestre`} style={{ flex: 'none', display: 'flex', alignItems: 'center', gap: 10, height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)', cursor: 'pointer', font: '500 12.5px var(--mono)', whiteSpace: 'nowrap' }}>
            <span style={{ color: 'var(--ac-tx)' }}>{bal.available} cr</span>
            <span style={{ width: 1, height: 14, background: 'var(--bd2)' }} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, color: 'var(--tx2)' }}>
              <Icon name="lock" size={12} />
              {bal.escrowTotal} cr
            </span>
          </button>
          <button onClick={() => navigate(ROUTES.notif)} aria-label="Notifications" style={{ position: 'relative', width: 32, height: 32, display: 'grid', placeItems: 'center', borderRadius: 8, border: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)', cursor: 'pointer' }}>
            <Icon name="bell" />
            {!readAll && <span style={{ position: 'absolute', top: 6, right: 6, width: 7, height: 7, borderRadius: '50%', background: 'var(--ac)', border: '2px solid var(--s1)' }} />}
          </button>
          <button onClick={toggleTheme} aria-label="Changer de thème" style={{ width: 32, height: 32, display: 'grid', placeItems: 'center', borderRadius: 8, border: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)', cursor: 'pointer' }}>
            <Icon name={theme === 'dark' ? 'sun' : 'moon'} />
          </button>
        </div>
        <div style={{ width: '100%', maxWidth: 1240, margin: '0 auto', padding: '28px 32px 80px', boxSizing: 'border-box' }}>
          <Outlet />
        </div>
      </main>
    </div>
  );
}
