import { cloneElement, Component, useId, useState, type ReactElement, type ReactNode } from 'react';
import { ApiError } from '../api/client';
import type { Resource } from '../api/useResource';
import { ErrorBanner, Skeleton } from '../components/ui';
import { useApp } from '../store/AppStore';
import { errorMessage } from './format';

/** Affiche le chargement, l'erreur ou le contenu d'une ressource. */
export function Loaded<T>({ r, children, what }: { r: Resource<T>; children: (data: T) => ReactNode; what: string }) {
  if (r.error) {
    const notFound = r.error instanceof ApiError && r.error.status === 404;
    return (
      <ErrorBanner onRetry={notFound ? undefined : r.reload}>
        <strong style={{ fontWeight: 600 }}>{notFound ? `${what} introuvable.` : `${what} n'a pas pu se charger.`}</strong>{' '}
        <span className="muted">{notFound ? "Il n'existe pas ou tu n'y as pas accès." : errorMessage(r.error)}</span>
      </ErrorBanner>
    );
  }
  if (r.data === undefined) {
    return (
      <div aria-busy="true" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <Skeleton style={{ width: '40%', height: 22 }} />
        <Skeleton style={{ width: '100%', height: 120 }} />
        <Skeleton style={{ width: '70%', height: 16 }} />
      </div>
    );
  }
  return <>{children(r.data)}</>;
}

/**
 * Isole une page : une erreur de rendu affiche un message au lieu de démonter
 * toute l'application (écran vide). Remise à zéro par `key` à chaque navigation.
 */
export class PageBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };
  static getDerivedStateFromError(error: Error) {
    return { error };
  }
  componentDidCatch(error: Error) {
    console.error('Erreur de rendu de la page', error);
  }
  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <ErrorBanner onRetry={() => this.setState({ error: null })}>
        <strong style={{ fontWeight: 600 }}>Cette page n'a pas pu s'afficher.</strong>{' '}
        <span className="muted">{error.message}</span>
      </ErrorBanner>
    );
  }
}

/**
 * Exécute une action utilisateur : bouton occupé pendant l'appel, toast de
 * succès, message d'erreur de l'API sinon.
 */
export function useAction() {
  const { showToast } = useApp();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function run<T>(fn: () => Promise<T>, success?: string): Promise<T | undefined> {
    setBusy(true);
    setError(null);
    try {
      const out = await fn();
      if (success) showToast(success, 'ok');
      return out;
    } catch (e) {
      setError(errorMessage(e));
      return undefined;
    } finally {
      setBusy(false);
    }
  }
  return { busy, error, run, clear: () => setError(null) };
}

export function FormError({ children }: { children: string | null }) {
  if (!children) return null;
  return (
    <span role="alert" style={{ fontSize: 12.5, color: 'var(--fail)' }}>
      {children}
    </span>
  );
}

/** Champ de formulaire : libellé explicite, aide reliée par aria-describedby. */
export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactElement<{ id?: string; 'aria-describedby'?: string }> }) {
  const id = useId();
  const hintId = `${id}-aide`;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 13 }}>
      <label htmlFor={id} style={{ fontWeight: 500 }}>{label}</label>
      {cloneElement(children, { id, 'aria-describedby': hint ? hintId : undefined })}
      {hint && <span id={hintId} className="faint" style={{ fontSize: 12 }}>{hint}</span>}
    </div>
  );
}

export const small = { height: 30, padding: '0 12px', borderRadius: 7, fontSize: 12.5, gap: 6 } as const;
