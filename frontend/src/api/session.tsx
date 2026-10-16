import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api, call, LOGIN_URL, onSessionExpired, refreshSession, setAccessToken, type S } from './client';

interface SessionState {
  status: 'loading' | 'anonymous' | 'authenticated';
  user: S['Me'] | null;
  login(): void;
  logout(): Promise<void>;
  /** Relit la session depuis le cookie (bac à sable, après une connexion simulée). */
  reload(): Promise<void>;
  setUser(u: S['Me']): void;
  /** Ouvre la session reçue d'une connexion par email (F18). */
  signIn(t: S['AccessToken']): void;
}

const Ctx = createContext<SessionState | null>(null);

const OUTCOMES: Record<string, string> = {
  'acces-refuse': 'Connexion annulée sur GitHub.',
  'compte-github-trop-recent': 'Ton compte GitHub est trop récent pour rejoindre CodeTroc.',
  'compte-suspendu': 'Ce compte est suspendu.',
  'etat-oauth-invalide': 'La connexion a expiré, recommence.',
  'github-refuse': "GitHub a refusé la connexion (configuration de l'OAuth App côté serveur).",
  'github-non-configure': "La connexion GitHub n'est pas configurée sur ce serveur.",
  'erreur-interne': 'Erreur du serveur pendant la connexion, réessaie plus tard.',
  'github-deja-lie': 'Ce compte GitHub est déjà associé à un autre membre CodeTroc.',
  'github-deja-present': 'Ton compte est déjà lié à GitHub.',
};

/** Message à afficher après le retour de GitHub (?connexion=…), puis nettoyage de l'URL. */
export function consumeLoginOutcome(): string | null {
  const url = new URL(window.location.href);
  const v = url.searchParams.get('connexion');
  if (!v) return null;
  url.searchParams.delete('connexion');
  window.history.replaceState(null, '', url.toString());
  return v === 'ok' ? null : OUTCOMES[v] ?? 'La connexion a échoué, réessaie.';
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionState['status']>('loading');
  const [user, setUser] = useState<S['Me'] | null>(null);

  useEffect(() => {
    onSessionExpired(() => {
      setUser(null);
      setStatus('anonymous');
    });
    refreshSession().then((s) => {
      setUser(s?.user ?? null);
      setStatus(s ? 'authenticated' : 'anonymous');
    });
  }, []);

  const logout = useCallback(async () => {
    await call(() => api.POST('/auth/logout')).catch(() => undefined);
    setAccessToken(null);
    setUser(null);
    setStatus('anonymous');
  }, []);

  const reload = useCallback(async () => {
    const s = await refreshSession();
    setUser(s?.user ?? null);
    setStatus(s ? 'authenticated' : 'anonymous');
  }, []);

  const value = useMemo<SessionState>(
    () => ({
      status,
      user,
      login: () => window.location.assign(LOGIN_URL),
      logout,
      reload,
      setUser: (u) => setUser(u),
      signIn: (t) => {
        setAccessToken(t.access_token);
        setUser(t.user);
        setStatus('authenticated');
      },
    }),
    [status, user, logout, reload],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useSession() {
  const v = useContext(Ctx);
  if (!v) throw new Error('useSession doit être utilisé dans <SessionProvider>');
  return v;
}
