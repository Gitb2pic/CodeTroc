import type { CSSProperties, ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import { tone as toneColors, type ToneColors } from '../domain/logic';
import type { Etat, Tone } from '../domain/types';

/** Icône Lucide servie par masque CSS (voir styles/icons.css). */
export function Icon({ name, size, spin, style }: { name: string; size?: number; spin?: boolean; style?: CSSProperties }) {
  return (
    <i
      data-i={name}
      data-spin={spin ? '' : undefined}
      aria-hidden="true"
      style={size ? { width: size, height: size, ...style } : style}
    />
  );
}

export { Logo } from './Brand';

/** Initiales dans un rond ; `ring` dessine l'anneau de réputation. */
export function Avatar({ ini, size = 28, ring, fontSize, border }: { ini: string; size?: number; ring?: string; fontSize?: number; border?: string }) {
  const fs = fontSize ?? Math.max(9, Math.round(size / 3.2));
  if (!ring) {
    return (
      <span
        style={{
          width: size,
          height: size,
          flex: 'none',
          borderRadius: '50%',
          background: 'var(--s3)',
          border,
          boxSizing: 'border-box',
          display: 'grid',
          placeItems: 'center',
          font: `600 ${fs}px var(--mono)`,
        }}
      >
        {ini}
      </span>
    );
  }
  const pad = size >= 80 ? 3 : 2;
  return (
    <span style={{ width: size, height: size, flex: 'none', borderRadius: '50%', padding: pad, boxSizing: 'border-box', background: ring }}>
      <span
        style={{
          width: '100%',
          height: '100%',
          borderRadius: '50%',
          background: 'var(--s3)',
          border: `${pad}px solid ${size >= 80 ? 'var(--bg)' : 'var(--s1)'}`,
          boxSizing: 'border-box',
          display: 'grid',
          placeItems: 'center',
          font: `600 ${fs}px var(--mono)`,
        }}
      >
        {ini}
      </span>
    </span>
  );
}

export function Pill({ tone, colors, icon, spin, children, style }: { tone?: Tone; colors?: ToneColors; icon?: string; spin?: boolean; children: ReactNode; style?: CSSProperties }) {
  const c = colors ?? toneColors(tone ?? 'mute');
  return (
    <span className="pill" style={{ color: c.fg, background: c.bg, border: `1px solid ${c.bd}`, ...style }}>
      {icon && <Icon name={icon} size={12} spin={spin} />}
      {children}
    </span>
  );
}

export function Tag({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return (
    <span className="tag" style={style}>
      {children}
    </span>
  );
}

export function PageTitle({ title, sub, action }: { title: string; sub?: ReactNode; action?: ReactNode }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', gap: 16, flexWrap: 'wrap' }}>
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
        <h1 className="h1">{title}</h1>
        {sub && <span className="muted">{sub}</span>}
      </div>
      {action}
    </div>
  );
}

export function EmptyState({ icon, title, children, actions, dashed = true, padding = '56px 24px' }: { icon: string; title: string; children?: ReactNode; actions?: ReactNode; dashed?: boolean; padding?: string }) {
  return (
    <div
      style={{
        padding,
        border: dashed ? '1px dashed var(--bd2)' : undefined,
        borderRadius: 12,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 10,
        textAlign: 'center',
      }}
    >
      <Icon name={icon} size={22} style={{ color: 'var(--tx3)' }} />
      <span style={{ fontWeight: 600 }}>{title}</span>
      {children && <span style={{ color: 'var(--tx2)', fontSize: 13.5, maxWidth: 380 }}>{children}</span>}
      {actions && <div style={{ display: 'flex', gap: 8, marginTop: 6 }}>{actions}</div>}
    </div>
  );
}

export function ErrorBanner({ icon = 'triangle-alert', children, onRetry, style }: { icon?: string; children: ReactNode; onRetry?: () => void; style?: CSSProperties }) {
  return (
    <div
      role="alert"
      style={{
        padding: '14px 16px',
        borderRadius: 10,
        border: '1px solid color-mix(in oklab,var(--fail) 40%,transparent)',
        background: 'color-mix(in oklab,var(--fail) 9%,transparent)',
        display: 'flex',
        gap: 12,
        alignItems: 'center',
        ...style,
      }}
    >
      <Icon name={icon} style={{ color: 'var(--fail)' }} />
      <span style={{ flex: 1 }}>{children}</span>
      <button className="btn btn-secondary" onClick={onRetry} style={{ height: 30, padding: '0 12px', borderRadius: 7, fontSize: 12.5 }}>
        <Icon name="refresh-cw" size={14} />
        Réessayer
      </button>
    </div>
  );
}

export function Skeleton({ style }: { style?: CSSProperties }) {
  return <span data-pulse="" style={{ display: 'block', borderRadius: 4, background: 'var(--s3)', ...style }} />;
}

/**
 * État d'affichage des écrans clés (normal, vide, chargement, erreur).
 * Pilotable par `?etat=` pour la démo et les tests visuels, comme la
 * propriété « etat » de la maquette.
 */
export function useEtat() {
  const [params] = useSearchParams();
  const raw = params.get('etat');
  const etat: Etat = raw === 'vide' || raw === 'chargement' || raw === 'erreur' ? raw : 'normal';
  return { normal: etat === 'normal', empty: etat === 'vide', loading: etat === 'chargement', error: etat === 'erreur' };
}

export function SegButtons<T extends string | number>({ options, value, onChange }: { options: [T, string][]; value: T; onChange: (v: T) => void }) {
  return (
    <div style={{ display: 'flex', padding: 3, border: '1px solid var(--bd)', borderRadius: 8, gap: 2 }}>
      {options.map(([v, label]) => {
        const on = v === value;
        return (
          <button
            key={String(v)}
            onClick={() => onChange(v)}
            aria-pressed={on}
            style={{
              flex: 1,
              height: 26,
              border: 0,
              borderRadius: 6,
              background: on ? 'var(--s3)' : 'transparent',
              color: on ? 'var(--tx)' : 'var(--tx2)',
              font: '500 12px var(--mono)',
              cursor: 'pointer',
            }}
          >
            {label}
          </button>
        );
      })}
    </div>
  );
}

/** Barre de séquestre : part libérée pleine, jalon en cours hachuré. */
export function EscrowBar({ released, current, height = 10, gap = 2 }: { released: string; current: string; height?: number; gap?: number }) {
  return (
    <span style={{ display: 'flex', width: '100%', height, borderRadius: 99, overflow: 'hidden', gap, background: 'var(--s3)' }}>
      <span style={{ width: released, background: 'var(--ac)' }} />
      <span style={{ width: current, background: 'repeating-linear-gradient(135deg,var(--ac) 0 3px,color-mix(in oklab,var(--ac) 35%,transparent) 3px 6px)' }} />
    </span>
  );
}
