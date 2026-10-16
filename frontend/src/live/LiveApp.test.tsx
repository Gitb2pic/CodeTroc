import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setAccessToken } from '../api/client';
import { PackCard } from '../components/PackCard';
import { AppStoreProvider, useApp } from '../store/AppStore';
import { LiveRoutes } from './LiveApp';

const me = { id: 'u1', pseudo: 'sami', email: null, github_login: 'sami', github_lie: true, email_verifie: false, bio: 'ancienne bio', role: 'MEMBRE', score_reputation: 4, date_inscription: '2026-10-14T09:00:00Z' };
const offer = {
  id: 'o1', titre: 'Pipeline CI', description: 'desc', prix_credits: 60, delai_jours: 3, statut: 'PUBLIEE', date_publication: '2026-10-14T09:00:00Z',
  jalons: [{ titre: 'J1', montant: 60 }],
  pack: { id: 'p1', titre: 'Pipeline', type_preuve: 'PR_CI', competences: ['Bash'] },
  prestataire: { id: 'u1', pseudo: 'sami', github_login: 'sami', score_reputation: 4, livraisons_verifiees: 1 },
};
const pack = { id: 'p1', titre: 'Pipeline', description: 'd', livrable: 'l', type_preuve: 'PR_CI', criteres: [], prix_min: 10, prix_max: 200, actif: true, competences: [], date_creation: '2026-10-14T09:00:00Z' };

type Handler = (req: Request) => Response | Promise<Response> | undefined;
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

let profile: { bio: string; competences: { id: string; libelle: string; niveau: string }[] };
let override: Handler = () => undefined;

function route(req: Request): Response | Promise<Response> {
  const o = override(req);
  if (o) return o;
  const { pathname } = new URL(req.url);
  const p = pathname.replace('/api/v1', '');
  const key = `${req.method} ${p}`;
  if (key === 'POST /auth/refresh') return json(200, { access_token: 't', token_type: 'Bearer', expires_in: 900, user: me });
  if (key === 'GET /me/compte') return json(200, { disponible: 50, sequestre_sortant: 0, sequestre_entrant: 0 });
  if (key === 'GET /notifications/compteur') return json(200, { non_lues: 0 });
  if (key === 'GET /me/compte/mouvements') return json(200, { items: [], next_cursor: null });
  if (key === 'GET /offres') return json(200, { items: [offer], next_cursor: null });
  if (key === 'GET /offres/o1') return json(200, offer);
  if (key === 'GET /packs/p1') return json(200, pack);
  if (key === 'GET /competences') return json(200, [{ id: 'c1', libelle: 'Bash', categorie: 'Ops' }]);
  if (key === 'GET /utilisateurs/u1') return json(200, { ...me, ...profile, livraisons_verifiees: 0, avis_recus: 0 });
  if (key === 'GET /utilisateurs/u1/avis') return json(200, { items: [], next_cursor: null });
  if (key === 'PATCH /me') return json(200, me);
  if (key === 'PUT /me/competences') return json(200, []);
  return json(404, { type: '/problems/introuvable', title: 'Introuvable', status: 404, request_id: 'r' });
}

function Toast() {
  const { toast } = useApp();
  return toast ? <div role="status">{toast.msg}</div> : null;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppStoreProvider>
        <LiveRoutes />
        <Toast />
      </AppStoreProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  profile = { bio: 'ancienne bio', competences: [] };
  override = () => undefined;
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => route(input as Request));
});

afterEach(() => {
  vi.restoreAllMocks();
  setAccessToken(null);
});

describe('navigation interne (bug 1)', () => {
  it("ne plante pas quand window.scrollTo renvoie une promesse (Chrome récent)", async () => {
    vi.spyOn(window, 'scrollTo').mockImplementation((() => Promise.resolve()) as unknown as typeof window.scrollTo);
    const errors: unknown[] = [];
    const onError = (e: ErrorEvent) => errors.push(e.error);
    window.addEventListener('error', onError);
    try {
      renderAt('/portefeuille');
      expect(await screen.findByRole('heading', { name: 'Portefeuille' })).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Catalogue' }));
      expect(await screen.findByRole('heading', { name: 'Catalogue' })).toBeInTheDocument();
      expect(screen.getByRole('navigation', { name: 'Navigation principale' })).toBeInTheDocument();
      expect(errors).toEqual([]);
    } finally {
      window.removeEventListener('error', onError);
    }
  });
});

function mockViewport(width: number) {
  window.matchMedia = ((query: string) => {
    const min = /\(min-width: (\d+)px\)/.exec(query);
    const max = /\(max-width: (\d+)px\)/.exec(query);
    const matches = min ? width >= +min[1] : max ? width <= +max[1] : false;
    return { matches, media: query, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false } as MediaQueryList;
  }) as typeof window.matchMedia;
}

describe('menu latéral', () => {
  afterEach(() => {
    // @ts-expect-error jsdom n'a pas matchMedia : on remet l'état d'origine
    delete window.matchMedia;
  });

  it('bureau : le bouton réduit le menu en rail et le choix est mémorisé', async () => {
    localStorage.clear();
    mockViewport(1440);
    const { unmount } = renderAt('/portefeuille');
    await screen.findByRole('heading', { name: 'Portefeuille' });
    const shell = () => document.querySelector('.shell')!;
    expect(shell()).toHaveAttribute('data-mode', 'desktop');
    expect(shell()).toHaveAttribute('data-rail', 'false');

    fireEvent.click(screen.getByRole('button', { name: 'Réduire le menu' }));
    expect(shell()).toHaveAttribute('data-rail', 'true');
    expect(screen.getByRole('button', { name: 'Agrandir le menu' })).toHaveAttribute('aria-expanded', 'false');
    // En rail, chaque entrée garde son nom accessible et une infobulle.
    expect(screen.getByRole('button', { name: 'Catalogue' })).toHaveAttribute('title', 'Catalogue');

    unmount();
    renderAt('/portefeuille');
    await screen.findByRole('heading', { name: 'Portefeuille' });
    expect(shell()).toHaveAttribute('data-rail', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'Agrandir le menu' }));
    expect(shell()).toHaveAttribute('data-rail', 'false');
  });

  it('tablette : rail d\'icônes, déployé par-dessus la page par le bouton', async () => {
    mockViewport(820);
    renderAt('/portefeuille');
    await screen.findByRole('heading', { name: 'Portefeuille' });
    const shell = () => document.querySelector('.shell')!;
    expect(shell()).toHaveAttribute('data-mode', 'tablet');
    expect(shell()).toHaveAttribute('data-rail', 'true');
    expect(screen.queryByRole('navigation', { name: 'Navigation rapide' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Ouvrir le menu' }));
    expect(shell()).toHaveAttribute('data-rail', 'false');
    expect(shell()).toHaveAttribute('data-drawer', 'true');
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(shell()).toHaveAttribute('data-drawer', 'false');
    expect(shell()).toHaveAttribute('data-rail', 'true');
  });

  it('mobile : tiroir fermé hors d\'atteinte, barre d\'onglets, tiroir refermé après navigation', async () => {
    mockViewport(390);
    renderAt('/portefeuille');
    await screen.findByRole('heading', { name: 'Portefeuille' });
    const aside = () => document.querySelector('aside')!;
    expect(document.querySelector('.shell')).toHaveAttribute('data-mode', 'mobile');
    expect(aside()).toHaveAttribute('inert');
    const tabs = screen.getByRole('navigation', { name: 'Navigation rapide' });
    expect(within(tabs).getAllByRole('button')).toHaveLength(5);
    expect(within(tabs).getByRole('button', { name: 'Portefeuille' })).toHaveAttribute('aria-current', 'page');

    fireEvent.click(screen.getByRole('button', { name: 'Ouvrir le menu' }));
    expect(aside()).not.toHaveAttribute('inert');
    fireEvent.click(within(aside()).getByRole('button', { name: 'Litiges' }));
    expect(await screen.findByRole('heading', { name: 'Litiges' })).toBeInTheDocument();
    expect(aside()).toHaveAttribute('inert');
  });
});

describe("retrait d'offre (bug 2)", () => {
  it("n'annonce pas de succès quand l'API répond 503", async () => {
    override = (req) => (req.method === 'POST' && req.url.endsWith('/offres/o1/retirer') ? json(503, { type: '/problems/indisponible', title: 'Service indisponible', status: 503, request_id: 'r' }) : undefined);
    renderAt('/catalogue/o1');
    fireEvent.click(await screen.findByRole('button', { name: "Retirer l'offre" }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Service indisponible');
    expect(screen.queryByText('Offre retirée du catalogue')).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Pipeline CI' })).toBeInTheDocument();
  });
});

describe('profil (bugs 3 et 4)', () => {
  it('rafraîchit la carte publique après « Enregistrer »', async () => {
    override = (req) => {
      if (req.method === 'PATCH') profile = { ...profile, bio: 'nouvelle bio' };
      if (req.method === 'PUT') profile = { ...profile, competences: [{ id: 'c1', libelle: 'Bash', niveau: 'EXPERT' }] };
      return undefined;
    };
    renderAt('/membres/u1');
    const bio = await screen.findByRole('textbox', { name: 'Bio' });
    expect(screen.getByText('ancienne bio', { selector: 'p' })).toBeInTheDocument();
    fireEvent.change(bio, { target: { value: 'nouvelle bio' } });
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }));
    expect(await screen.findByText('nouvelle bio', { selector: 'p' })).toBeInTheDocument();
    expect(screen.getByText('Bash · expert')).toBeInTheDocument();
  });

  it('/profil redirige vers la page membre de la personne connectée', async () => {
    renderAt('/profil');
    expect(await screen.findByRole('textbox', { name: 'Bio' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Profil' })).toHaveAttribute('aria-current', 'page');
  });
});

describe('catalogue (bug 6)', () => {
  it('signale le rechargement quand un filtre change', async () => {
    let release: () => void = () => {};
    renderAt('/catalogue');
    expect(await screen.findByText('1 offre')).toBeInTheDocument();
    override = (req) => (new URL(req.url).searchParams.get('prix_max') ? new Promise<Response>((r) => { release = () => r(json(200, { items: [], next_cursor: null })); }) : undefined);
    fireEvent.click(screen.getByRole('button', { name: '≤ 60 cr' }));
    await waitFor(() => expect(screen.getByTestId('catalogue-resultats')).toHaveAttribute('aria-busy', 'true'));
    await act(async () => release());
    expect(await screen.findByText('Aucune offre ne correspond')).toBeInTheDocument();
    expect(screen.getByTestId('catalogue-resultats')).toHaveAttribute('aria-busy', 'false');
  });
});

describe('pluriels (bug 7)', () => {
  const card = (deliv: number) => ({ id: 'x', title: 't', stack: [], price: 1, days: 1, ms: 1, crit: 0, name: 'n', handle: 'h', ini: 'N', rep: 4, deliv });
  it.each([
    [0, '0 livraison vérifiée'],
    [1, '1 livraison vérifiée'],
    [2, '2 livraisons vérifiées'],
  ])('carte de pack : %i → « %s »', (n, text) => {
    render(<PackCard pack={card(n)} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });
});
