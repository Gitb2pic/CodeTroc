import { useEffect } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { api, call } from '../api/client';
import { useSession } from '../api/session';
import { useResource } from '../api/useResource';
import { Avatar, Icon, Logo } from '../components/ui';
import { repRing } from '../domain/logic';
import { useApp } from '../store/AppStore';
import { initials } from './format';
import { resetSandbox, SANDBOX } from '../sandbox/runtime';
import { LIVE_ROUTES as R } from './routes';

type NavKey = 'dash' | 'cat' | 'wallet' | 'notif' | 'disputes' | 'profile' | 'admin';

export function LiveLayout() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useSession();
  const { toggleTheme, theme } = useApp();
  const balance = useResource(() => call(() => api.GET('/me/compte')), [pathname]);
  const unread = useResource(() => call(() => api.GET('/notifications/compteur')), [pathname]);

  useEffect(() => window.scrollTo({ top: 0 }), [pathname]);
  if (!user) return null;

  const nav: [NavKey, string, string, string, number?][] = [
    ['dash', 'Tableau de bord', 'layout-dashboard', R.dash],
    ['cat', 'Catalogue', 'package', R.cat],
    ['wallet', 'Portefeuille', 'wallet', R.wallet],
    ['notif', 'Notifications', 'bell', R.notif, unread.data?.non_lues],
    ['disputes', 'Litiges', 'scale', R.disputes],
    ['profile', 'Profil', 'user', R.member(user.id)],
  ];
  if (user.role === 'ADMIN') nav.push(['admin', 'Administration', 'shield', R.admin]);
  const b = balance.data;

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '236px minmax(0,1fr)', minHeight: '100vh' }}>
      <aside style={{ position: 'sticky', top: 0, height: '100vh', boxSizing: 'border-box', borderRight: '1px solid var(--bd)', background: 'var(--s1)', display: 'flex', flexDirection: 'column', gap: 2, padding: '14px 10px' }}>
        <button onClick={() => navigate(R.dash)} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '4px 8px 14px', border: 0, background: 'none', color: 'var(--tx)', font: '600 15px var(--sans)', cursor: 'pointer' }}>
          <Logo />
          CodeTroc
        </button>
        <nav aria-label="Navigation principale" style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {nav.map(([k, label, icon, to, count]) => {
            const on = pathname === to || (k === 'cat' && pathname.startsWith('/catalogue'));
            return (
              <button key={k} className="hov-nav" onClick={() => navigate(to)} aria-current={on ? 'page' : undefined}
                style={{ display: 'flex', alignItems: 'center', gap: 10, height: 34, padding: '0 10px', borderRadius: 7, border: 0, cursor: 'pointer', font: '500 13.5px/1 var(--sans)', textAlign: 'left', background: on ? 'var(--s3)' : 'transparent', color: on ? 'var(--tx)' : 'var(--tx2)' }}>
                <Icon name={icon} />
                <span style={{ flex: 1 }}>{label}</span>
                {!!count && <span style={{ minWidth: 18, height: 18, padding: '0 5px', boxSizing: 'border-box', borderRadius: 99, background: 'var(--ac)', color: 'var(--on-ac)', font: '500 11px/18px var(--mono)', textAlign: 'center' }}>{count}</span>}
              </button>
            );
          })}
        </nav>
        <button className="btn btn-secondary" onClick={() => navigate(R.create)} style={{ marginTop: 12, fontSize: 13 }}>
          <Icon name="plus" />
          Proposer une offre
        </button>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 8px 2px', borderTop: '1px solid var(--bd)' }}>
          <Avatar ini={initials(user.pseudo)} size={30} ring={repRing(user.score_reputation)} fontSize={10} />
          <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.25, minWidth: 0 }}>
            <span style={{ fontSize: 13, fontWeight: 500 }}>{user.pseudo}</span>
            <span className="mono faint" style={{ fontSize: 11.5 }}>@{user.github_login}</span>
          </span>
          <button onClick={() => logout().then(() => navigate(R.landing))} aria-label="Se déconnecter" title="Se déconnecter" style={{ border: 0, background: 'none', color: 'var(--tx3)', cursor: 'pointer' }}>
            <Icon name="log-out" />
          </button>
        </div>
      </aside>
      <main style={{ minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        {SANDBOX && (
          <div role="note" style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', padding: '8px 28px', borderBottom: '1px dashed var(--bd2)', font: '12.5px var(--mono)', color: 'var(--tx2)' }}>
            <Icon name="play" size={13} />
            <span style={{ flex: 1, minWidth: 0 }}>Bac à sable · backend Go et PostgreSQL dans ton navigateur · tu joues <strong style={{ color: 'var(--tx)' }}>{user.pseudo}</strong></span>
            <button className="btn btn-secondary" onClick={() => logout().then(() => navigate(R.landing))} style={{ height: 26, padding: '0 10px', fontSize: 12 }}>Changer de persona</button>
            <button className="btn btn-secondary" onClick={() => void resetSandbox()} style={{ height: 26, padding: '0 10px', fontSize: 12 }}>Réinitialiser</button>
          </div>
        )}
        <div style={{ position: 'sticky', top: 0, zIndex: 5, height: 56, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 12, padding: '0 28px', borderBottom: '1px solid var(--bd)', background: 'color-mix(in oklab,var(--bg) 85%,transparent)', backdropFilter: 'blur(12px)' }}>
          <button onClick={() => navigate(R.wallet)} aria-label={b ? `Solde : ${b.disponible} crédits disponibles, ${b.sequestre_sortant + b.sequestre_entrant} en séquestre` : 'Solde'}
            style={{ display: 'flex', alignItems: 'center', gap: 10, height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)', cursor: 'pointer', font: '500 12.5px var(--mono)' }}>
            <span style={{ color: 'var(--ac-tx)' }}>{b ? b.disponible : '…'} cr</span>
            <span style={{ width: 1, height: 14, background: 'var(--bd2)' }} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, color: 'var(--tx2)' }}>
              <Icon name="lock" size={12} />
              {b ? b.sequestre_sortant + b.sequestre_entrant : '…'} cr
            </span>
          </button>
          <button onClick={() => navigate(R.notif)} aria-label={`Notifications${unread.data?.non_lues ? ` : ${unread.data.non_lues} non lues` : ''}`} style={{ position: 'relative', width: 32, height: 32, display: 'grid', placeItems: 'center', borderRadius: 8, border: '1px solid var(--bd)', background: 'var(--s1)', color: 'var(--tx)', cursor: 'pointer' }}>
            <Icon name="bell" />
            {!!unread.data?.non_lues && <span style={{ position: 'absolute', top: 6, right: 6, width: 7, height: 7, borderRadius: '50%', background: 'var(--ac)', border: '2px solid var(--s1)' }} />}
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
