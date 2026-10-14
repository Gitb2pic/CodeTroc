import { useEffect, useState, type FormEvent } from 'react';
import { api, call, type S } from '../../api/client';
import { useResource } from '../../api/useResource';
import { Icon, Pill } from '../../components/ui';
import { fmtDate, fmtTime } from '../format';
import { Field, FormError, small, useAction } from '../kit';

const KINDS: [S['Secret']['type'], string, string][] = [
  ['JETON', 'Jeton', 'key-round'],
  ['CLE_API', 'Clé API', 'key-square'],
  ['MOT_DE_PASSE', 'Mot de passe', 'lock-keyhole'],
  ['AUTRE', 'Autre', 'file-lock'],
];
const STATUS: Record<S['Secret']['statut'], ['ok' | 'mute' | 'wait', string]> = { ACTIF: ['ok', 'Actif'], EXPIRE: ['mute', 'Expiré'], REVOQUE: ['mute', 'Révoqué'] };
/** Durée d'affichage d'une valeur révélée. */
const REVEAL_MS = 30_000;

/** Coffre d'accès d'un échange : le demandeur dépose, le prestataire révèle. */
export function VaultCard({ x, role }: { x: S['Exchange']; role: 'DEMANDEUR' | 'PRESTATAIRE' }) {
  const secrets = useResource(() => call(() => api.GET('/echanges/{id}/secrets', { params: { path: { id: x.id } } })), [x.id, x.statut]);
  const [adding, setAdding] = useState(false);
  const open = x.statut === 'EN_COURS' || x.statut === 'EN_LITIGE';

  return (
    <div className="card" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Icon name="vault" style={{ color: 'var(--tx3)' }} />
        <h2 className="h2" style={{ flex: 1 }}>Accès partagés</h2>
        {role === 'DEMANDEUR' && open && !adding && (
          <button onClick={() => setAdding(true)} style={{ border: 0, background: 'none', color: 'var(--ac-tx)', fontSize: 12.5, cursor: 'pointer' }}>+ Partager</button>
        )}
      </div>
      {adding && <ShareForm exchangeId={x.id} onDone={() => { setAdding(false); secrets.reload(); }} />}
      {secrets.data?.length === 0 && !adding && (
        <span className="faint" style={{ fontSize: 12.5 }}>
          {role === 'DEMANDEUR' ? 'Partage ici un jeton ou une clé : chiffré, journalisé, révoqué à la clôture.' : 'Aucun accès partagé pour le moment.'}
        </span>
      )}
      {secrets.data?.map((s) => <SecretRow key={s.id} s={s} role={role} onChange={secrets.reload} />)}
      {secrets.error && <FormError>Les accès n'ont pas pu se charger.</FormError>}
    </div>
  );
}

function ShareForm({ exchangeId, onDone }: { exchangeId: string; onDone(): void }) {
  const [label, setLabel] = useState('');
  const [kind, setKind] = useState<S['Secret']['type']>('JETON');
  const [scope, setScope] = useState('');
  const [value, setValue] = useState('');
  const [days, setDays] = useState(3);
  const act = useAction();
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const expires = new Date(Date.now() + days * 86_400_000).toISOString();
    const ok = await act.run(
      () => call(() => api.POST('/echanges/{id}/secrets', { params: { path: { id: exchangeId } }, body: { libelle: label.trim(), type: kind, portee: scope.trim() || undefined, valeur: value, date_expiration: expires } })),
      'Accès partagé, chiffré dans le coffre',
    );
    if (ok) {
      setValue('');
      onDone();
    }
  };
  return (
    <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: 12, borderRadius: 10, border: '1px solid var(--bd)', background: 'var(--bg)' }}>
      <Field label="Libellé"><input className="field" required maxLength={100} value={label} onChange={(e) => setLabel(e.target.value)} placeholder="GHCR_PUSH_TOKEN" /></Field>
      <Field label="Type">
        <select className="field" value={kind} onChange={(e) => setKind(e.target.value as S['Secret']['type'])}>
          {KINDS.map(([k, l]) => <option key={k} value={k}>{l}</option>)}
        </select>
      </Field>
      <Field label="Portée" hint="Ce que l'accès permet, pour le journal"><input className="field" maxLength={300} value={scope} onChange={(e) => setScope(e.target.value)} placeholder="packages:write, lecture seule…" /></Field>
      <Field label="Valeur" hint="Chiffrée côté serveur, jamais réaffichée pour toi">
        <input className="field" required type="password" autoComplete="off" maxLength={8192} value={value} onChange={(e) => setValue(e.target.value)} />
      </Field>
      <Field label="Expire dans">
        <select className="field" value={days} onChange={(e) => setDays(+e.target.value)}>
          {[1, 3, 7, 14, 30].map((d) => <option key={d} value={d}>{d} j</option>)}
        </select>
      </Field>
      <FormError>{act.error}</FormError>
      <div style={{ display: 'flex', gap: 8 }}>
        <button type="submit" className="btn btn-primary" disabled={act.busy} style={small}>Chiffrer et partager</button>
        <button type="button" className="btn btn-secondary" onClick={onDone} style={small}>Annuler</button>
      </div>
    </form>
  );
}

function SecretRow({ s, role, onChange }: { s: S['Secret']; role: 'DEMANDEUR' | 'PRESTATAIRE'; onChange(): void }) {
  const [value, setValue] = useState<string | null>(null);
  const [log, setLog] = useState<{ action: string; pseudo: string | null; ip: string; date: string }[] | null>(null);
  const act = useAction();
  const icon = KINDS.find(([k]) => k === s.type)?.[2] ?? 'key-round';
  const [tone, label] = STATUS[s.statut];

  // La valeur révélée s'efface d'elle-même.
  useEffect(() => {
    if (value === null) return;
    const t = setTimeout(() => setValue(null), REVEAL_MS);
    return () => clearTimeout(t);
  }, [value]);

  const reveal = async () => {
    const r = await act.run(() => call(() => api.POST('/secrets/{id}/reveler', { params: { path: { id: s.id } } })));
    if (r) setValue(r.valeur);
  };
  const revoke = async () => {
    if ((await act.run(() => call(() => api.POST('/secrets/{id}/revoquer', { params: { path: { id: s.id } } })), 'Accès révoqué, valeur effacée')) !== undefined) onChange();
  };
  const toggleLog = async () => {
    if (log) return setLog(null);
    const r = await act.run(() => call(() => api.GET('/secrets/{id}/journal', { params: { path: { id: s.id } } })));
    if (r) setLog(r);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, paddingTop: 8, borderTop: '1px solid var(--bd)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, font: '12.5px var(--mono)' }}>
        <Icon name={icon} size={14} style={{ color: 'var(--tx3)' }} />
        <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{s.libelle}</span>
        <Pill tone={tone}>{label}</Pill>
      </div>
      <span className="faint" style={{ fontSize: 11.5 }}>
        {s.portee ? `${s.portee} · ` : ''}expire le {fmtDate(s.date_expiration)}
      </span>
      {value !== null && (
        <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          <code style={{ flex: 1, padding: '6px 8px', borderRadius: 6, background: 'var(--s3)', fontSize: 12, overflowWrap: 'anywhere' }}>{value}</code>
          <button className="btn btn-secondary" style={small} onClick={() => navigator.clipboard?.writeText(value)}>Copier</button>
        </div>
      )}
      <div style={{ display: 'flex', gap: 10, fontSize: 12 }}>
        {role === 'PRESTATAIRE' && s.statut === 'ACTIF' && value === null && (
          <button onClick={reveal} disabled={act.busy} style={{ border: 0, padding: 0, background: 'none', color: 'var(--ac-tx)', cursor: 'pointer' }}>Révéler (journalisé)</button>
        )}
        {role === 'DEMANDEUR' && s.statut === 'ACTIF' && (
          <button onClick={revoke} disabled={act.busy} style={{ border: 0, padding: 0, background: 'none', color: 'var(--fail)', cursor: 'pointer' }}>Révoquer</button>
        )}
        <button onClick={toggleLog} style={{ border: 0, padding: 0, background: 'none', color: 'var(--tx2)', cursor: 'pointer' }}>{log ? 'Masquer le journal' : 'Journal'}</button>
      </div>
      {log && (
        <ul style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 3, font: '11.5px var(--mono)' }}>
          {log.map((l, i) => (
            <li key={i} className="muted">{fmtTime(l.date)} · {l.action} · {l.pseudo ?? 'système'}{l.ip ? ` · ${l.ip}` : ''}</li>
          ))}
        </ul>
      )}
      <FormError>{act.error}</FormError>
    </div>
  );
}
