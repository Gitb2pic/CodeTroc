// Bac à sable : le vrai backend CodeTroc (Go compilé en WebAssembly) tourne
// dans le navigateur, sur PGlite (PostgreSQL en WebAssembly). Les appels du
// front vers /api/v1 lui sont remis directement, sans réseau.
import type { PGlite as PGliteType } from '@electric-sql/pglite';

export const SANDBOX = import.meta.env.VITE_SANDBOX === '1';

const DB_NAME = 'codetroc-sandbox';
const STARTUP_CODE = 196608; // protocole 3.0

declare global {
  // Fonctions exposées par le backend WASM (backend/cmd/sandbox).
  var codetrocBoot: () => Promise<void>;
  var codetrocHandle: (method: string, url: string, headers: string, body: Uint8Array | null) => Promise<{ status: number; headers: string; body: Uint8Array }>;
  // Pont vers PGlite, appelé par le backend.
  var codetrocPg: (msg: Uint8Array) => Promise<Uint8Array>;
  var Go: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
}

/** Branche le protocole PostgreSQL du backend sur PGlite (une seule session, appels en file). */
function bridge(db: PGliteType) {
  let startup: Uint8Array | null = null;
  let queue: Promise<unknown> = Promise.resolve();
  const raw = async (msg: Uint8Array) => {
    const parts: Uint8Array[] = [];
    await db.execProtocolRawStream(msg, { onRawData: (d: Uint8Array) => parts.push(d.slice()) });
    const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
    let i = 0;
    for (const p of parts) {
      out.set(p, i);
      i += p.length;
    }
    return out;
  };
  const exec = async (msg: Uint8Array) => {
    const view = new DataView(msg.buffer, msg.byteOffset, msg.byteLength);
    // Chaque nouvelle connexion du pool rejoue le démarrage : la session existe déjà.
    if (msg.length >= 8 && msg[0] === 0 && view.getInt32(4) === STARTUP_CODE) {
      startup ??= await raw(msg);
      return startup;
    }
    // Fin de connexion : la session PGlite reste ouverte pour la suivante.
    if (msg.length === 5 && msg[0] === 0x58) return new Uint8Array(0);
    return raw(msg);
  };
  globalThis.codetrocPg = (msg) => {
    const p = queue.then(() => exec(msg));
    queue = p.catch(() => undefined);
    return p;
  };
}

function loadScript(src: string) {
  return new Promise<void>((resolve, reject) => {
    const s = document.createElement('script');
    s.src = src;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error(`chargement de ${src} impossible`));
    document.head.append(s);
  });
}

// Les binaires sont publiés en .wasm (seul type binaire servi par l'hébergeur) :
// codetroc.gz.wasm est le backend compressé, pglite-fs.wasm le système de fichiers de PGlite.
async function wasmBytes(): Promise<ArrayBuffer> {
  const res = await fetch(new URL('codetroc.gz.wasm', document.baseURI));
  if (!res.ok || !res.body) throw new Error('backend WASM introuvable');
  return new Response(res.body.pipeThrough(new DecompressionStream('gzip'))).arrayBuffer();
}

// ---------- Cookies : le navigateur ne les gère pas pour des réponses synthétiques ----------

const JAR_KEY = 'codetroc-sandbox-cookies';
const jar = new Map<string, string>(readJar());

// La session survit au rechargement de l'onglet, comme un vrai cookie de session.
function readJar(): [string, string][] {
  try {
    return JSON.parse(sessionStorage.getItem(JAR_KEY) ?? '[]');
  } catch {
    return [];
  }
}

function saveJar() {
  try {
    sessionStorage.setItem(JAR_KEY, JSON.stringify([...jar]));
  } catch {
    // stockage indisponible : la session ne survivra pas au rechargement
  }
}

function storeCookies(setCookie: string[] = []) {
  for (const line of setCookie) {
    const [pair, ...attrs] = line.split(';');
    const eq = pair.indexOf('=');
    const name = pair.slice(0, eq).trim();
    const value = pair.slice(eq + 1).trim();
    const expired = attrs.some((a) => /^\s*max-age=(-|0\b)/i.test(a));
    if (expired || value === '') jar.delete(name);
    else jar.set(name, value);
  }
  saveJar();
}

const cookieHeader = () => [...jar].map(([k, v]) => `${k}=${v}`).join('; ');

/** Appel direct au backend WASM, cookies compris. */
async function call(method: string, path: string, headers: Record<string, string>, body: Uint8Array | null) {
  if (jar.size) headers.cookie = cookieHeader();
  const r = await globalThis.codetrocHandle(method, path, JSON.stringify(headers), body);
  const h = JSON.parse(r.headers) as Record<string, string[]>;
  storeCookies(h['Set-Cookie']);
  return { status: r.status, headers: h, body: r.body };
}

/** Remplace fetch pour /api/v1 : la requête va au backend WASM. */
function installFetch() {
  const native = globalThis.fetch.bind(globalThis);
  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = new Request(input, init);
    const url = new URL(req.url);
    const at = url.pathname.indexOf('/api/v1/');
    if (at < 0) return native(req);
    const headers: Record<string, string> = {};
    req.headers.forEach((v, k) => (headers[k] = v));
    const buf = req.method === 'GET' || req.method === 'HEAD' ? null : new Uint8Array(await req.arrayBuffer());
    const r = await call(req.method, url.pathname.slice(at) + url.search, headers, buf && buf.length ? buf : null);
    const out = new Headers();
    for (const [k, vs] of Object.entries(r.headers)) if (k.toLowerCase() !== 'set-cookie') vs.forEach((v) => out.append(k, v));
    const nullBody = r.status === 204 || r.status === 304;
    return new Response(nullBody ? null : new Uint8Array(r.body), { status: r.status, headers: out });
  };
}

/** Connexion d'un persona par le faux GitHub : même parcours OAuth que la prod. */
export async function sandboxLogin(login: string) {
  const start = await call('GET', '/api/v1/auth/github/login', {}, null);
  const location = start.headers.Location?.[0] ?? '';
  const state = new URL(location, 'http://sandbox').searchParams.get('state') ?? '';
  const back = await call('GET', `/api/v1/auth/github/callback?state=${encodeURIComponent(state)}&code=${encodeURIComponent(login)}`, {}, null);
  if (!jar.has('ct_refresh')) throw new Error(`connexion refusée (${back.status})`);
}

/** Efface la base locale du bac à sable puis recharge la page. */
export async function resetSandbox() {
  jar.clear();
  saveJar();
  await new Promise((resolve) => {
    const r = indexedDB.deleteDatabase(`/pglite/${DB_NAME}`);
    r.onsuccess = r.onerror = r.onblocked = resolve;
  });
  location.reload();
}

/** Démarre PGlite puis le backend ; à appeler avant le rendu du front. */
export async function bootSandbox(onStep: (step: string) => void) {
  if (typeof WebAssembly === 'undefined') throw new Error("WebAssembly n'est pas disponible dans ce navigateur.");
  onStep('Démarrage de PostgreSQL (PGlite)…');
  const { PGlite } = await import('@electric-sql/pglite');
  const fs = await fetch(new URL('pglite-fs.wasm', document.baseURI));
  if (!fs.ok) throw new Error('système de fichiers PostgreSQL introuvable');
  const fsBundle = await fs.blob();
  let db: PGliteType;
  try {
    db = await PGlite.create(`idb://${DB_NAME}`, { relaxedDurability: true, fsBundle });
  } catch {
    db = await PGlite.create({ fsBundle }); // stockage indisponible : base en mémoire
  }
  bridge(db);
  onStep('Chargement du backend Go (WebAssembly)…');
  await loadScript(new URL('wasm_exec.js', document.baseURI).toString());
  const go = new globalThis.Go();
  const ready = new Promise((r) => addEventListener('codetroc-wasm-ready', r, { once: true }));
  const { instance } = await WebAssembly.instantiate(await wasmBytes(), go.importObject);
  void go.run(instance);
  await ready;
  onStep('Migrations et données de démonstration…');
  await globalThis.codetrocBoot();
  installFetch();
}

export const PERSONAS = [
  { login: 'sami', role: 'Demandeur', hint: 'a un échange en cours avec Léa' },
  { login: 'lea', role: 'Prestataire', hint: 'DevOps, livre par pull request' },
  { login: 'karim', role: 'Prestataire', hint: 'Docker et tests E2E' },
  { login: 'ines', role: 'Prestataire', hint: 'React et accessibilité' },
  { login: 'arbitre', role: 'Arbitre', hint: 'tranche les litiges' },
  { login: 'admin', role: 'Administratrice', hint: 'rôles et suspensions' },
];
