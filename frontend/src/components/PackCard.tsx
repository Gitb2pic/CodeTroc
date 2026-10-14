import { plural, repRing } from '../domain/logic';
import type { Pack } from '../domain/types';
import { Avatar, Icon, Tag } from './ui';

/** Carte de pack du catalogue (aussi utilisée en aperçu de création). */
export function PackCard({ pack, onOpen, as = 'button' }: { pack: Pack; onOpen?: () => void; as?: 'button' | 'div' }) {
  const body = (
    <>
      <span style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
        {pack.stack.map((t) => (
          <Tag key={t}>{t}</Tag>
        ))}
      </span>
      <span style={{ fontSize: 15, fontWeight: 600, lineHeight: 1.3, letterSpacing: '-.01em', minHeight: as === 'button' ? 40 : undefined, textWrap: 'pretty' }}>{pack.title}</span>
      <span style={{ display: 'flex', gap: 14, font: '12px var(--mono)', color: 'var(--tx2)' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><Icon name="clock" size={13} />{pack.days} j</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><Icon name="git-commit-horizontal" size={13} />{plural(pack.ms, 'jalon')}</span>
        {pack.crit > 0 && <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}><Icon name="list-checks" size={13} />{pack.crit} critères</span>}
      </span>
      <span style={{ display: 'flex', alignItems: 'center', gap: 10, paddingTop: 12, borderTop: '1px solid var(--bd)' }}>
        <Avatar ini={pack.ini} size={28} ring={repRing(pack.rep)} fontSize={9} />
        <span style={{ flex: 1, display: 'flex', flexDirection: 'column', lineHeight: 1.25, minWidth: 0 }}>
          <span style={{ fontSize: 12.5, fontWeight: 500 }}>
            {pack.name} <span className="mono muted" style={{ fontWeight: 400 }}>· {pack.rep.toFixed(1)} ★</span>
          </span>
          {as === 'button' && (
            <span className="mono faint" style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11.5 }}>
              <Icon name="shield-check" size={12} style={{ color: 'var(--ok)' }} />
              {pack.deliv} livraisons vérifiées
            </span>
          )}
        </span>
        <span style={{ font: '600 16px var(--mono)', color: 'var(--ac-tx)', letterSpacing: '-.02em' }}>{pack.price} cr</span>
      </span>
    </>
  );
  const style = { display: 'flex', flexDirection: 'column', gap: 14, padding: 16, borderRadius: 12, background: 'var(--s1)', color: 'var(--tx)', textAlign: 'left', font: 'inherit' } as const;
  if (as === 'div') return <div style={{ ...style, border: '1px solid var(--bd2)' }}>{body}</div>;
  return (
    <button className="hov-card" onClick={onOpen} style={{ ...style, border: '1px solid var(--bd)', cursor: 'pointer' }}>
      {body}
    </button>
  );
}
