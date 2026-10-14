import type { CSSProperties } from 'react';

/**
 * La pièce fendue : une pièce (la monnaie) coupée par le « / » du code ; les
 * deux moitiés glissent l'une contre l'autre, deux devs qui échangent.
 * Géométrie relevée sur le logo de référence : deux demi-disques de rayon 40,
 * coupe inclinée à 15°, écart de 8 et glissement de 11,2 le long de la coupe.
 */
export function CoinMark({ size = 24, mono, title, style }: { size?: number; mono?: boolean; title?: string; style?: CSSProperties }) {
  const a = mono ? 'currentColor' : 'var(--brand-a)';
  const b = mono ? 'currentColor' : 'var(--brand-b)';
  return (
    <svg width={size} height={size} viewBox="-48 -48 96 96" role={title ? 'img' : undefined} aria-hidden={title ? undefined : true} style={{ flex: 'none', ...style }}>
      {title && <title>{title}</title>}
      <g transform="rotate(15)">
        <path d="M -4 -34.4 A 40 40 0 0 0 -4 45.6 Z" fill={a} />
        <path d="M 4 -45.6 A 40 40 0 0 1 4 34.4 Z" fill={b} />
      </g>
    </svg>
  );
}

/** Symbole seul, ou symbole et mot « codetroc » (graphie officielle, en minuscules). */
export function Logo({ size = 24, wordmark = false, mono }: { size?: number; wordmark?: boolean; mono?: boolean }) {
  if (!wordmark) return <CoinMark size={size} mono={mono} title="codetroc" />;
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: size * 0.38, color: 'var(--tx)' }} aria-label="codetroc">
      <CoinMark size={size} mono={mono} />
      <span aria-hidden="true" style={{ font: `700 ${Math.round(size * 0.82)}px/1 var(--sans)`, letterSpacing: '-.045em' }}>codetroc</span>
    </span>
  );
}
