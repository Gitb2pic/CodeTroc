import { useEffect, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { api, call } from '../api/client';
import { useSession } from '../api/session';
import { useResource } from '../api/useResource';
import { Avatar, Icon, Logo } from '../components/ui';
import { repRing } from '../domain/logic';
import { useApp } from '../store/AppStore';
import { initials } from './format';
import { PageBoundary } from './kit';
import { resetSandbox, SANDBOX } from '../sandbox/runtime';
import { LIVE_ROUTES as R } from './routes';

/**
 * Trois présentations du menu :
 * - desktop (≥ 1100 px) : menu large, que le bouton réduit en rail d'icônes ;
 * - tablette (700–1099 px) : rail d'icônes, que le bouton déploie par-dessus la page ;
 * - mobile (< 700 px) : tiroir ouvert par le bouton, et barre d'onglets en bas.
 */
export type LayoutMode = 'desktop' | 'tablet' | 'mobile';
const MOBILE = '(max-width: 699px)';
const DESKTOP = '(min-width: 1100px)';

function currentMode(): LayoutMode {
  if (typeof window === 'undefined' || !window.matchMedia) return 'desktop';
  if (window.matchMedia(MOBILE).matches) return 'mobile';
  return window.matchMedia(DESKTOP).matches ? 'desktop' : 'tablet';
}

function useLayoutMode(): LayoutMode {
  const [mode, setMode] = useState(currentMode);
  useEffect(() => {
    if (!window.matchMedia) return;
    const queries = [window.matchMedia(MOBILE), window.matchMedia(DESKTOP)];
    const update = () => setMode(currentMode());
    queries.forEach((q) => q.addEventListener('change', update));
    return () => queries.forEach((q) => q.removeEventListener('change', update));
  }, []);
  return mode;
}

const NAV_KEY = 'codetroc.menu';

/** Menu réduit en rail sur grand écran ; le choix est mémorisé. */
function useCollapsed() {
  const [collapsed, setCollapsed] = useState(() => {
    try {
      const v = localStorage.getItem(NAV_KEY);
      return v === 'reduit' || v === 'ferme';
    } catch {
      return false;
    }
  });
  const set = (v: boolean) => {
    setCollapsed(v);
    try {
      localStorage.setItem(NAV_KEY, v ? 'reduit' : 'large');
    } catch {
      /* stockage indisponible : le choix vaut pour la session */
    }
  };
  return [collapsed, set] as const;
}

interface NavItem {
  key: string;
  label: string;
  /** Libellé court de la barre d'onglets mobile. */
  short?: string;
  icon: string;
  to: string;
  on: boolean;
  count?: number;
  tab?: boolean;
}

export function LiveLayout() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useSession();
  const { toggleTheme, theme } = useApp();
  const mode = useLayoutMode();
  const [collapsed, setCollapsed] = useCollapsed();
  const [drawer, setDrawer] = useState(false);
  const balance = useResource(() => call(() => api.GET('/me/compte')), [pathname]);
  const unread = useResource(() => call(() => api.GET('/notifications/compteur')), [pathname]);

  // Corps entre accolades : scrollTo renvoie une promesse sur les Chrome récents,
  // que React prendrait pour une fonction de nettoyage (écran noir au changement de route).
  useEffect(() => {
    window.scrollTo({ top: 0 });
  }, [pathname]);
  // Le tiroir se referme après une navigation ou un changement de taille d'écran.
  useEffect(() => {
    setDrawer(false);
  }, [pathname, mode]);
  useEffect(() => {
    if (!drawer) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setDrawer(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [drawer]);
  if (!user) return null;

  const go = (to: string) => {
    setDrawer(false);
    navigate(to);
  };
  const items: Record<string, NavItem> = {
    dash: { key: 'dash', label: 'Tableau de bord', short: 'Accueil', icon: 'layout-dashboard', to: R.dash, on: pathname === R.dash, tab: true },
    cat: { key: 'cat', label: 'Catalogue', icon: 'package', to: R.cat, on: pathname.startsWith('/catalogue'), tab: true },
    wallet: { key: 'wallet', label: 'Portefeuille', icon: 'wallet', to: R.wallet, on: pathname === R.wallet, tab: true },
    notif: { key: 'notif', label: 'Notifications', short: 'Alertes', icon: 'bell', to: R.notif, on: pathname === R.notif, count: unread.data?.non_lues, tab: true },
    disputes: { key: 'disputes', label: 'Litiges', icon: 'scale', to: R.disputes, on: pathname.startsWith('/litiges') },
    profile: { key: 'profile', label: 'Profil', icon: 'user', to: R.member(user.id), on: pathname === R.member(user.id), tab: true },
    admin: { key: 'admin', label: 'Administration', icon: 'shield', to: R.admin, on: pathname === R.admin },
  };
  const groups: [string, NavItem[]][] = [
    ['Principal', [items.dash, items.cat, items.wallet]],
    ['Activité', [items.notif, items.disputes]],
    ['Compte', user.role === 'ADMIN' ? [items.profile, items.admin] : [items.profile]],
  ];
  const tabs = Object.values(items).filter((i) => i.tab);
  const b = balance.data;

  const rail = mode === 'desktop' ? collapsed : mode === 'tablet' ? !drawer : false;
  const expanded = mode === 'desktop' ? !collapsed : drawer;
  const menuLabel = mode === 'desktop' ? (collapsed ? 'Agrandir le menu' : 'Réduire le menu') : drawer ? 'Fermer le menu' : 'Ouvrir le menu';
  const toggleMenu = () => (mode === 'desktop' ? setCollapsed(!collapsed) : setDrawer(!drawer));
  const signOut = () => logout().then(() => navigate(R.landing));

  return (
    <div className="shell" data-mode={mode} data-rail={rail} data-drawer={drawer}>
      {mode !== 'desktop' && drawer && <div className="scrim" onClick={() => setDrawer(false)} aria-hidden="true" />}
      <aside id="menu-principal" className="side" inert={mode === 'mobile' && !drawer}>
        <div className="side-head">
          <button className="side-brand" onClick={() => go(R.dash)} aria-label="codetroc, tableau de bord">
            <span className="side-full"><Logo wordmark size={22} /></span>
            <span className="side-mini"><Logo size={26} /></span>
          </button>
        </div>
        <nav aria-label="Navigation principale" className="side-nav">
          {groups.map(([title, list]) => (
            <div className="side-group" key={title}>
              <span className="side-kicker kicker">{title}</span>
              <span className="side-sep" aria-hidden="true" />
              {list.map((i) => (
                <button key={i.key} className="side-link" onClick={() => go(i.to)} aria-current={i.on ? 'page' : undefined} aria-label={i.label} title={rail ? i.label : undefined}>
                  <Icon name={i.icon} />
                  <span className="side-label">{i.label}</span>
                  {!!i.count && <span className="side-badge" aria-hidden="true">{i.count}</span>}
                </button>
              ))}
            </div>
          ))}
        </nav>
        <button className="btn btn-secondary side-cta" onClick={() => go(R.create)} aria-label="Proposer une offre" title={rail ? 'Proposer une offre' : undefined}>
          <Icon name="plus" />
          <span className="side-label">Proposer une offre</span>
        </button>
        <div className="side-spacer" />
        <div className="side-user">
          <Avatar ini={initials(user.pseudo)} size={32} ring={repRing(user.score_reputation)} fontSize={10} />
          <span className="side-label side-who">
            <span className="ellipsis" style={{ fontSize: 13, fontWeight: 500 }}>{user.pseudo}</span>
            {user.github_login ? <span className="ellipsis mono faint" style={{ fontSize: 11.5 }}>@{user.github_login}</span> : <span className="ellipsis faint" style={{ fontSize: 11.5 }}>{user.email}</span>}
          </span>
          <button className="side-logout" onClick={signOut} aria-label="Se déconnecter" title="Se déconnecter">
            <Icon name="log-out" />
          </button>
        </div>
      </aside>
      <div className="shell-main">
        {SANDBOX && (
          <div role="note" className="sandbox-note">
            <Icon name="play" size={13} />
            <span style={{ flex: 1, minWidth: 200 }}>Bac à sable · backend Go et PostgreSQL dans ton navigateur · tu joues <strong style={{ color: 'var(--tx)' }}>{user.pseudo}</strong></span>
            <button className="btn btn-secondary" onClick={signOut} style={{ height: 26, padding: '0 10px', fontSize: 12 }}>Changer de persona</button>
            <button className="btn btn-secondary" onClick={() => void resetSandbox()} style={{ height: 26, padding: '0 10px', fontSize: 12 }}>Réinitialiser</button>
          </div>
        )}
        <header className="topbar">
          <button className="icon-btn" onClick={toggleMenu} aria-label={menuLabel} aria-expanded={expanded} aria-controls="menu-principal" title={menuLabel}>
            <Icon name={mode !== 'desktop' && drawer ? 'x' : 'menu'} />
          </button>
          <span style={{ flex: 1 }} />
          <button className="balance" onClick={() => go(R.wallet)} aria-label={b ? `Solde : ${b.disponible} crédits disponibles, ${b.sequestre_sortant + b.sequestre_entrant} en séquestre` : 'Solde'}>
            <span style={{ color: 'var(--ac-tx)' }}>{b ? b.disponible : '…'} cr</span>
            <span className="balance-sep" />
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, color: 'var(--tx2)' }}>
              <Icon name="lock" size={12} />
              {b ? b.sequestre_sortant + b.sequestre_entrant : '…'} cr
            </span>
          </button>
          <button className="icon-btn hide-mobile" onClick={() => go(R.notif)} aria-label={`Notifications${unread.data?.non_lues ? ` : ${unread.data.non_lues} non lues` : ''}`}>
            <Icon name="bell" />
            {!!unread.data?.non_lues && <span className="icon-dot" />}
          </button>
          <button className="icon-btn" onClick={toggleTheme} aria-label="Changer de thème">
            <Icon name={theme === 'dark' ? 'sun' : 'moon'} />
          </button>
        </header>
        <main className="content">
          <PageBoundary key={pathname}>
            <Outlet />
          </PageBoundary>
        </main>
      </div>
      {mode === 'mobile' && (
        <nav className="tabbar" aria-label="Navigation rapide">
          {tabs.map((i) => (
            <button key={i.key} className="tab" onClick={() => go(i.to)} aria-current={i.on ? 'page' : undefined} aria-label={i.label}>
              <Icon name={i.icon} size={20} />
              <span aria-hidden="true">{i.short ?? i.label}</span>
              {!!i.count && <span className="side-badge" aria-hidden="true">{i.count}</span>}
            </button>
          ))}
        </nav>
      )}
    </div>
  );
}
