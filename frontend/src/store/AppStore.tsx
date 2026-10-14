import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { PACKS } from '../data/fixtures';
import { exchangeAmounts } from '../domain/logic';
import type { Flow, LogEntry, Pack } from '../domain/types';

export type Theme = 'dark' | 'light';
type ToastKind = 'ok' | 'warn' | 'lock' | 'info';

export interface Toast {
  msg: string;
  icon: string;
}

const TOAST_ICON: Record<ToastKind, string> = { ok: 'circle-check', warn: 'triangle-alert', lock: 'lock-open', info: 'info' };
const STORAGE_KEY = 'codetroc-proto';
/** Durée simulée de la CI sur la PR #42. */
export const CI_DURATION_MS = 3000;
const TOAST_DURATION_MS = 3800;

interface Persisted {
  flow: Flow;
  exPack: string;
  theme: Theme;
}

function load(): Persisted {
  let s: Partial<Persisted> = {};
  try {
    s = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
  } catch {
    // Stockage indisponible (navigation privée, aperçu) : valeurs par défaut.
  }
  const flow = typeof s.flow === 'number' && s.flow >= 0 && s.flow <= 5 ? (s.flow as Flow) : 2;
  return { flow, exPack: s.exPack || 'p1', theme: s.theme === 'light' ? 'light' : 'dark' };
}

interface AppState {
  theme: Theme;
  flow: Flow;
  /** Pack de l'échange EX-0142. */
  exPack: Pack;
  revoked: Record<string, boolean>;
  logExtra: LogEntry[];
  sent: string[];
  readAll: boolean;
  skills: string[];
  toast: Toast | null;
}

interface AppActions {
  toggleTheme(): void;
  showToast(msg: string, kind?: ToastKind): void;
  requestPack(pack: Pack): void;
  simulateAccept(): void;
  simulateDelivery(): void;
  validateMilestone(): void;
  replay(): void;
  revokeAccess(id: string, name: string, to: string): void;
  sendMessage(text: string): void;
  markAllRead(): void;
  toggleSkill(skill: string): void;
}

const Ctx = createContext<(AppState & AppActions) | null>(null);

export function findPack(id: string | undefined): Pack {
  return PACKS.find((p) => p.id === id) || PACKS[0];
}

export function AppStoreProvider({ children }: { children: ReactNode }) {
  const initial = useMemo(load, []);
  const [theme, setTheme] = useState<Theme>(initial.theme);
  const [flow, setFlow] = useState<Flow>(initial.flow);
  const [exPackId, setExPackId] = useState(initial.exPack);
  const [revoked, setRevoked] = useState<Record<string, boolean>>({});
  const [logExtra, setLogExtra] = useState<LogEntry[]>([]);
  const [sent, setSent] = useState<string[]>([]);
  const [readAll, setReadAll] = useState(false);
  const [skills, setSkills] = useState(['Docker', 'GitHub Actions', 'Node.js', 'TypeScript']);
  const [toast, setToast] = useState<Toast | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const exPack = findPack(exPackId);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ flow, exPack: exPackId, theme }));
    } catch {
      // Préférence non conservée : sans conséquence.
    }
  }, [flow, exPackId, theme]);

  const showToast = useCallback((msg: string, kind: ToastKind = 'info') => {
    clearTimeout(toastTimer.current);
    setToast({ msg, icon: TOAST_ICON[kind] });
    toastTimer.current = setTimeout(() => setToast(null), TOAST_DURATION_MS);
  }, []);

  useEffect(() => () => clearTimeout(toastTimer.current), []);

  // Simulation de la CI : la PR #42 passe au vert après quelques secondes.
  useEffect(() => {
    if (flow !== 3) return;
    const t = setTimeout(() => {
      setFlow(4);
      showToast('CI réussie sur la PR #42 · 3/3 checks', 'ok');
    }, CI_DURATION_MS);
    return () => clearTimeout(t);
  }, [flow, showToast]);

  const first = exPack.name.split(' ')[0];

  const value: AppState & AppActions = {
    theme,
    flow,
    exPack,
    revoked,
    logExtra,
    sent,
    readAll,
    skills,
    toast,
    toggleTheme: () => setTheme((t) => (t === 'dark' ? 'light' : 'dark')),
    showToast,
    requestPack: (pack) => {
      setExPackId(pack.id);
      setFlow(1);
      setSent([]);
      showToast(`Demande envoyée · ${pack.price} cr bloqués en séquestre`, 'info');
    },
    simulateAccept: () => {
      setFlow(2);
      showToast(`${first} a accepté ta demande`, 'ok');
    },
    simulateDelivery: () => {
      setFlow(3);
      showToast(`${first} a ouvert la PR #42`, 'info');
    },
    validateMilestone: () => {
      setFlow(5);
      showToast(`${exchangeAmounts(exPack, 4).first} cr libérés à ${first} · preuve : PR #42`, 'lock');
    },
    replay: () => {
      setFlow(0);
      setExPackId('p1');
    },
    revokeAccess: (id, name, to) => {
      setRevoked((r) => ({ ...r, [id]: true }));
      setLogExtra((l) => [{ time: "à l'instant", who: 'Toi', act: 'Accès révoqué', res: name, ip: '—', tone: 'fail' }, ...l]);
      showToast(`Accès révoqué · ${to.split(' ')[0]} a été notifiée et ses sessions coupées`, 'ok');
    },
    sendMessage: (text) => {
      const t = text.trim();
      if (t) setSent((s) => [...s, t]);
    },
    markAllRead: () => setReadAll(true),
    toggleSkill: (skill) => setSkills((s) => (s.includes(skill) ? s.filter((x) => x !== skill) : [...s, skill])),
  };

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useApp() {
  const v = useContext(Ctx);
  if (!v) throw new Error('useApp doit être utilisé dans <AppStoreProvider>');
  return v;
}
