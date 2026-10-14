import { Navigate, Route, Routes } from 'react-router-dom';
import { AppLayout } from './components/AppLayout';
import { Icon } from './components/ui';
import { CatalogPage } from './pages/CatalogPage';
import { CreatePackPage } from './pages/CreatePackPage';
import { DashboardPage } from './pages/DashboardPage';
import { DisputePage } from './pages/DisputePage';
import { ExchangePage } from './pages/ExchangePage';
import { LandingPage } from './pages/LandingPage';
import { NotificationsPage } from './pages/NotificationsPage';
import { OnboardingPage } from './pages/OnboardingPage';
import { PackDetailPage } from './pages/PackDetailPage';
import { ProfilePage } from './pages/ProfilePage';
import { VaultPage } from './pages/VaultPage';
import { WalletPage } from './pages/WalletPage';
import { ROUTES } from './routes';
import { AppStoreProvider, useApp } from './store/AppStore';

function Shell() {
  const { theme, toast } = useApp();
  return (
    <div className="app-root" data-theme={theme}>
      <Routes>
        <Route path={ROUTES.landing} element={<LandingPage />} />
        <Route path={ROUTES.onboarding} element={<OnboardingPage />} />
        <Route element={<AppLayout />}>
          <Route path={ROUTES.dash} element={<DashboardPage />} />
          <Route path={ROUTES.cat} element={<CatalogPage />} />
          <Route path="/catalogue/:packId" element={<PackDetailPage />} />
          <Route path={ROUTES.create} element={<CreatePackPage />} />
          <Route path="/echanges/:id" element={<ExchangePage />} />
          <Route path="/echanges" element={<Navigate to={ROUTES.ex} replace />} />
          <Route path={ROUTES.vault} element={<VaultPage />} />
          <Route path={ROUTES.wallet} element={<WalletPage />} />
          <Route path="/membres/:handle" element={<ProfilePage />} />
          <Route path="/litiges/:id" element={<DisputePage />} />
          <Route path={ROUTES.notif} element={<NotificationsPage />} />
        </Route>
        <Route path="*" element={<Navigate to={ROUTES.landing} replace />} />
      </Routes>
      {/* Région toujours présente pour que les lecteurs d'écran annoncent chaque toast. */}
      <div role="status" aria-live="polite">
        {toast && (
          <div style={{ position: 'fixed', bottom: 24, right: 24, zIndex: 50, display: 'flex', alignItems: 'center', gap: 10, padding: '12px 16px', borderRadius: 10, background: 'var(--tx)', color: 'var(--bg)', fontSize: 13.5, fontWeight: 500, boxShadow: '0 12px 40px rgba(0,0,0,.35)' }}>
            <Icon name={toast.icon} />
            {toast.msg}
          </div>
        )}
      </div>
    </div>
  );
}

export function App() {
  return (
    <AppStoreProvider>
      <Shell />
    </AppStoreProvider>
  );
}
