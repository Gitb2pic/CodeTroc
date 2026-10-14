import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { EmptyState, Icon } from '../components/ui';
import { exchangeAmounts, tone } from '../domain/logic';
import type { Tone } from '../domain/types';
import { ROUTES } from '../routes';
import { useApp } from '../store/AppStore';

const TABS = ['Tout', 'Jalons', 'CI', 'Crédits', 'Accès', 'Litiges'] as const;
type Cat = Exclude<(typeof TABS)[number], 'Tout'>;

interface Notif {
  cat: Cat;
  icon: string;
  tone: Tone;
  t: string;
  ref: string;
  time: string;
  today: boolean;
  show: boolean;
  cta: string;
  to: string;
  unread: boolean;
}

export function NotificationsPage() {
  const navigate = useNavigate();
  const { exPack, flow, readAll, markAllRead } = useApp();
  const [tab, setTab] = useState<(typeof TABS)[number]>('Tout');
  const m1 = exchangeAmounts(exPack, flow).first;

  const all: Notif[] = [
    { cat: 'Crédits', icon: 'lock-open', tone: 'ac', t: `${m1} cr libérés à Léa pour le jalon 1`, ref: 'EX-0142 · PR #42', time: "à l'instant", today: true, show: flow >= 5, cta: 'Voir la transaction', to: ROUTES.wallet, unread: true },
    { cat: 'CI', icon: 'circle-check', tone: 'ok', t: 'CI réussie sur la PR #42 · 3/3 checks', ref: 'acme/shop-api · a3f9c1e', time: 'il y a 2 min', today: true, show: flow >= 4, cta: 'Voir les checks', to: ROUTES.ex, unread: true },
    { cat: 'Jalons', icon: 'git-pull-request', tone: 'ac', t: 'Léa a livré le jalon 1 : PR #42 ouverte', ref: 'EX-0142', time: 'il y a 5 min', today: true, show: flow >= 3, cta: "Ouvrir l'échange", to: ROUTES.ex, unread: true },
    { cat: 'Accès', icon: 'timer', tone: 'wait', t: 'GHCR_PUSH_TOKEN expire dans 23 h', ref: 'Partagé avec Léa · EX-0142', time: 'il y a 1 h', today: true, show: true, cta: 'Prolonger ou révoquer', to: ROUTES.vault, unread: true },
    { cat: 'CI', icon: 'circle-x', tone: 'fail', t: 'CI en échec sur la PR #19 · test_auth_refresh', ref: 'hlambert/api · 9c04b2e', time: 'hier', today: false, show: true, cta: 'Voir le log', to: ROUTES.dispute, unread: true },
    { cat: 'Litiges', icon: 'scale', tone: 'disp', t: 'Hugo a ouvert un litige sur le jalon 3', ref: 'EX-0131 · 110 cr gelés', time: 'il y a 2 j', today: false, show: true, cta: 'Voir le litige', to: ROUTES.dispute, unread: true },
    { cat: 'Crédits', icon: 'arrow-down-left', tone: 'ok', t: 'Karim a libéré 35 cr pour ton jalon 1', ref: 'EX-0138 · PR #7', time: 'il y a 3 j', today: false, show: true, cta: 'Voir', to: ROUTES.wallet, unread: false },
    { cat: 'Jalons', icon: 'rotate-ccw', tone: 'wait', t: 'Tu as demandé des changements à Nora', ref: 'EX-0136 · jalon 1', time: 'il y a 4 j', today: false, show: true, cta: 'Voir', to: ROUTES.ex, unread: false },
  ].filter((n): n is Notif => n.show);

  const list = all.filter((n) => tab === 'Tout' || n.cat === tab);
  const unreadCount = readAll ? 0 : all.filter((n) => n.unread).length;

  const group = (title: string, items: Notif[]) =>
    items.length > 0 && (
      <section style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <h2 className="kicker" style={{ margin: '0 0 4px' }}>{title}</h2>
        {items.map((n) => {
          const c = tone(n.tone);
          const un = n.unread && !readAll;
          return (
            <div key={n.t} style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '12px 14px', borderRadius: 10, border: '1px solid var(--bd)', background: un ? 'color-mix(in oklab, var(--ac) 5%, var(--s1))' : 'var(--s1)' }}>
              <span style={{ width: 32, height: 32, flex: 'none', borderRadius: 9, display: 'grid', placeItems: 'center', color: c.fg, background: c.bg, border: `1px solid ${c.bd}` }}>
                <Icon name={n.icon} />
              </span>
              <span style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
                <span style={{ fontSize: 13.5, fontWeight: un ? 600 : 400 }}>{n.t}</span>
                <span className="mono faint" style={{ fontSize: 12 }}>{n.cat} · {n.ref} · {n.time}</span>
              </span>
              {un && <span role="img" aria-label="Non lue" style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--ac)' }} />}
              <button className="btn btn-secondary" onClick={() => navigate(n.to)} style={{ height: 28, padding: '0 10px', borderRadius: 7, fontSize: 12 }}>{n.cta}</button>
            </div>
          );
        })}
      </section>
    );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 860 }}>
      <div style={{ display: 'flex', alignItems: 'flex-end', gap: 16 }}>
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <h1 className="h1">Notifications</h1>
          <span className="mono muted" style={{ fontSize: 13 }}>{unreadCount} non lues</span>
        </div>
        <button className="btn btn-secondary" onClick={markAllRead} style={{ height: 32, padding: '0 12px', fontSize: 13 }}>
          <Icon name="check-check" size={14} />
          Tout marquer comme lu
        </button>
      </div>
      <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
        {TABS.map((t) => {
          const on = tab === t;
          return (
            <button key={t} onClick={() => setTab(t)} aria-pressed={on} style={{ height: 30, padding: '0 12px', borderRadius: 7, border: `1px solid ${on ? 'var(--bd2)' : 'transparent'}`, background: on ? 'var(--s3)' : 'transparent', color: on ? 'var(--tx)' : 'var(--tx2)', font: '500 13px var(--sans)', cursor: 'pointer' }}>
              {t}
            </button>
          );
        })}
      </div>
      {group("Aujourd'hui", list.filter((n) => n.today))}
      {group('Cette semaine', list.filter((n) => !n.today))}
      {list.length === 0 && (
        <EmptyState icon="bell" title="Rien dans cette catégorie" padding="48px 24px">
          Tu es à jour.
        </EmptyState>
      )}
    </div>
  );
}
