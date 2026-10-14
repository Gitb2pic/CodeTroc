// Client typé de l'API CodeTroc, généré depuis backend/api/openapi.yaml
// (npm run api:types). Le jeton d'accès vit en mémoire seulement ; le jeton
// de rafraîchissement est un cookie HttpOnly que JavaScript ne voit jamais.
import createClient from 'openapi-fetch';
import type { components, paths } from './schema';

export type S = components['schemas'];

/** Erreur d'API au format RFC 9457. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly problem?: S['Problem'];

  constructor(status: number, problem?: S['Problem']) {
    super(problem?.detail || problem?.title || `Erreur ${status}`);
    this.status = status;
    this.problem = problem;
    this.code = problem?.type?.replace(/^\/problems\//, '') ?? 'inconnue';
  }
}

// Même origine que le front : le cookie de rafraîchissement (SameSite=Strict) suit.
/** URL absolue de l'API ; relative si l'origine est opaque (page de démonstration en bac à sable). */
function apiBase(): string {
  const origin = globalThis.location?.origin;
  return origin && origin !== 'null' ? `${origin}/api/v1` : '/api/v1';
}

export const api = createClient<paths>({
  baseUrl: apiBase(),
  credentials: 'same-origin',
  fetch: (request) => globalThis.fetch(request), // résolu à l'appel (remplaçable en test)
});

let accessToken: string | null = null;
let refreshing: Promise<S['AccessToken'] | null> | null = null;
let onSessionLost: () => void = () => {};

api.use({
  onRequest({ request, schemaPath }) {
    // Les routes /auth/* s'appuient sur le cookie, pas sur un jeton (peut-être expiré).
    if (accessToken && !schemaPath.startsWith('/auth/')) request.headers.set('Authorization', `Bearer ${accessToken}`);
    return request;
  },
});

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export function onSessionExpired(cb: () => void) {
  onSessionLost = cb;
}

/** Échange le cookie de rafraîchissement contre un jeton d'accès ; null si aucune session. */
export function refreshSession(): Promise<S['AccessToken'] | null> {
  refreshing ??= (async () => {
    try {
      const { data, response } = await api.POST('/auth/refresh');
      if (!response.ok || !data) {
        setAccessToken(null);
        return null;
      }
      setAccessToken(data.access_token);
      return data;
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

interface Result<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/**
 * Exécute un appel, rafraîchit la session une fois sur 401 puis rejoue, et
 * renvoie les données ou lève une ApiError.
 */
export async function call<T>(fn: () => Promise<Result<T>>): Promise<T> {
  let res = await fn();
  if (res.response.status === 401 && accessToken !== null) {
    const s = await refreshSession();
    if (!s) {
      onSessionLost();
      throw new ApiError(401, res.error as S['Problem']);
    }
    res = await fn();
  }
  if (!res.response.ok) throw new ApiError(res.response.status, res.error as S['Problem']);
  return res.data as T;
}

/** En-tête d'idempotence pour les opérations financières (une clé par intention). */
export function idem(): { 'Idempotency-Key': string } {
  return { 'Idempotency-Key': crypto.randomUUID() };
}

export const LOGIN_URL = '/api/v1/auth/github/login';
