import { useEffect } from 'react';
import { Navigate, Outlet, Route, Routes } from 'react-router-dom';
import { consumeLoginOutcome, SessionProvider, useSession } from '../api/session';
import { useApp } from '../store/AppStore';
import { LiveLayout } from './LiveLayout';
import { AdminPage } from './pages/AdminPage';
import { AnnouncementPage, CreateAnnouncementPage } from './pages/AnnouncementPages';
import { CatalogPage } from './pages/CatalogPage';
import { CreateOfferPage } from './pages/CreateOfferPage';
import { DashboardPage } from './pages/DashboardPage';
import { DisputePage, DisputesPage } from './pages/DisputesPage';
import { ExchangePage } from './pages/ExchangePage';
import { LoginPage } from './pages/LoginPage';
import { MemberPage } from './pages/MemberPage';
import { NotificationsPage } from './pages/NotificationsPage';
import { OfferPage } from './pages/OfferPage';
import { WalletPage } from './pages/WalletPage';
import { LIVE_ROUTES as R } from './routes';

/** Pages réservées aux membres connectés ; sinon retour à l'écran de connexion. */
function RequireSession() {
  const { status } = useSession();
  const { showToast } = useApp();
  useEffect(() => {
    // Un échec de connexion est affiché par l'écran de connexion ; un succès ici.
    if (new URLSearchParams(window.location.search).get('connexion') === 'ok') {
      consumeLoginOutcome();
      showToast('Connecté avec GitHub', 'ok');
    }
  }, [showToast]);
  if (status === 'loading') return <div aria-busy="true" style={{ minHeight: '100vh' }} />;
  // La requête (?connexion=<code>) suit : l'écran de connexion affiche la cause de l'échec.
  if (status === 'anonymous') return <Navigate to={{ pathname: R.landing, search: window.location.search }} replace />;
  return <Outlet />;
}

/** /profil : alias de la page membre de la personne connectée. */
function MyProfile() {
  const { user } = useSession();
  return user ? <Navigate to={R.member(user.id)} replace /> : null;
}

export function LiveRoutes() {
  return (
    <SessionProvider>
      <Routes>
        <Route path={R.landing} element={<LoginPage />} />
        <Route element={<RequireSession />}>
          <Route element={<LiveLayout />}>
            <Route path={R.dash} element={<DashboardPage />} />
            <Route path={R.cat} element={<CatalogPage />} />
            <Route path="/catalogue/:id" element={<OfferPage />} />
            <Route path={R.create} element={<CreateOfferPage />} />
            <Route path={R.createAnnouncement} element={<CreateAnnouncementPage />} />
            <Route path="/annonces/:id" element={<AnnouncementPage />} />
            <Route path="/echanges/:id" element={<ExchangePage />} />
            <Route path={R.wallet} element={<WalletPage />} />
            <Route path={R.notif} element={<NotificationsPage />} />
            <Route path={R.disputes} element={<DisputesPage />} />
            <Route path="/litiges/:id" element={<DisputePage />} />
            <Route path="/membres/:id" element={<MemberPage />} />
            <Route path={R.profile} element={<MyProfile />} />
            <Route path={R.admin} element={<AdminPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to={R.dash} replace />} />
      </Routes>
    </SessionProvider>
  );
}
