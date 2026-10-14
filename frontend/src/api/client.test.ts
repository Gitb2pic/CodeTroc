import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, call, idem, refreshSession, setAccessToken } from './client';

const json = (status: number, body: unknown, type = 'application/json') =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': type } });

const me = { id: 'u1', pseudo: 'sami', email: null, github_login: 'sami', bio: null, role: 'MEMBRE', score_reputation: 0, date_inscription: '2026-10-14T09:00:00Z' };

afterEach(() => {
  vi.restoreAllMocks();
  setAccessToken(null);
});

describe('client API', () => {
  it('envoie le jeton en Bearer et rafraîchit une fois sur 401, puis rejoue', async () => {
    setAccessToken('expiré');
    const seen: string[] = [];
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const req = input as Request;
      seen.push(`${req.method} ${new URL(req.url).pathname} ${req.headers.get('Authorization') ?? '-'}`);
      if (req.url.endsWith('/auth/refresh')) return json(200, { access_token: 'neuf', token_type: 'Bearer', expires_in: 900, user: me });
      if (req.headers.get('Authorization') === 'Bearer expiré') return json(401, { type: '/problems/non-authentifie', title: 'Non authentifié', status: 401, request_id: 'r' }, 'application/problem+json');
      return json(200, { disponible: 50, sequestre_sortant: 0, sequestre_entrant: 0 });
    });

    const b = await call(() => api.GET('/me/compte'));
    expect(b.disponible).toBe(50);
    expect(seen).toEqual(['GET /api/v1/me/compte Bearer expiré', 'POST /api/v1/auth/refresh -', 'GET /api/v1/me/compte Bearer neuf']);
  });

  it('transforme un problème RFC 9457 en ApiError avec son code', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      json(409, { type: '/problems/solde-insuffisant', title: 'Solde insuffisant', status: 409, detail: 'Il te manque 20 cr.', request_id: 'r' }, 'application/problem+json'),
    );
    const err = await call(() => api.POST('/echanges', { body: { id_offre: 'o', depot_url: 'https://github.com/a/b' } })).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe('solde-insuffisant');
    expect(err.status).toBe(409);
    expect(err.message).toBe('Il te manque 20 cr.');
  });

  it('partage un seul rafraîchissement entre appels simultanés', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async () => json(200, { access_token: 't', token_type: 'Bearer', expires_in: 900, user: me }));
    const [a, b] = await Promise.all([refreshSession(), refreshSession()]);
    expect(a?.access_token).toBe('t');
    expect(b).toBe(a);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('sans session, le rafraîchissement renvoie null', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(401, { type: '/problems/non-authentifie', title: 'x', status: 401, request_id: 'r' }, 'application/problem+json'));
    expect(await refreshSession()).toBeNull();
  });

  it('génère une clé d\'idempotence neuve à chaque intention', () => {
    expect(idem()['Idempotency-Key']).not.toBe(idem()['Idempotency-Key']);
  });
});
