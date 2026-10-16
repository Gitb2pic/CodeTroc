import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, call, type S } from '../../api/client';
import { useSession } from '../../api/session';
import { useResource } from '../../api/useResource';
import { EmptyState, PageTitle, Pill } from '../../components/ui';
import { fmtDate, shortId } from '../format';
import { FormError, Loaded, small, useAction } from '../kit';
import { LIVE_ROUTES as R } from '../routes';

export function AdminPage() {
  const { user } = useSession();
  const [q, setQ] = useState('');
  const users = useResource(() => call(() => api.GET('/admin/utilisateurs', { params: { query: { q, limit: 50 } } })), [q]);
  const queue = useResource(() => call(() => api.GET('/admin/litiges')), []);
  if (user?.role !== 'ADMIN') return <EmptyState icon="shield" title="Réservé aux administrateurs" />;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      <PageTitle title="Administration" sub="Rôles, suspensions et litiges sans arbitre." />
      <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <h2 className="h2">Litiges en attente d'arbitre</h2>
        <Loaded r={queue} what="La file">
          {(ds) => (ds.length === 0 ? <span className="faint">File vide.</span> : (
            <ul style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 10 }}>
              {ds.map((d) => <QueueRow key={d.id} d={d} arbiters={(users.data?.items ?? []).filter((u) => u.role !== 'MEMBRE' && u.statut === 'ACTIF')} onDone={queue.reload} />)}
            </ul>
          ))}
        </Loaded>
      </section>
      <section className="card" style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <h2 className="h2">Membres</h2>
        <input className="field" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Pseudo ou login GitHub" aria-label="Rechercher un membre" />
        <Loaded r={users} what="Les membres">
          {(p) => (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead><tr className="kicker" style={{ textAlign: 'left' }}><th>Membre</th><th>Inscrit</th><th>Rôle</th><th>Statut</th><th /></tr></thead>
              <tbody>{p.items.map((u) => <UserRow key={u.id} u={u} self={u.id === user.id} onDone={users.reload} />)}</tbody>
            </table>
          )}
        </Loaded>
      </section>
    </div>
  );
}

function UserRow({ u, self, onDone }: { u: S['AdminUser']; self: boolean; onDone(): void }) {
  const act = useAction();
  const patch = (body: { role?: S['AdminUser']['role']; statut?: 'ACTIF' | 'SUSPENDU' }) =>
    act.run(() => call(() => api.PATCH('/admin/utilisateurs/{id}', { params: { path: { id: u.id } }, body })), 'Membre mis à jour').then(onDone);
  return (
    <tr style={{ borderTop: '1px solid var(--bd)' }}>
      <td style={{ padding: '8px 0' }}><Link to={R.member(u.id)}>{u.pseudo}</Link> <span className="mono faint">{u.github_login ? `@${u.github_login}` : u.email}</span></td>
      <td className="mono faint">{fmtDate(u.date_inscription)}</td>
      <td>
        <select aria-label={`Rôle de ${u.pseudo}`} value={u.role} disabled={self || u.statut === 'SUPPRIME' || act.busy} onChange={(e) => patch({ role: e.target.value as S['AdminUser']['role'] })}>
          <option value="MEMBRE">Membre</option><option value="ARBITRE">Arbitre</option><option value="ADMIN">Admin</option>
        </select>
      </td>
      <td><Pill tone={u.statut === 'ACTIF' ? 'ok' : 'mute'}>{u.statut}</Pill></td>
      <td style={{ textAlign: 'right' }}>
        {!self && u.statut !== 'SUPPRIME' && (
          <button className="btn btn-secondary" style={small} disabled={act.busy} onClick={() => patch({ statut: u.statut === 'ACTIF' ? 'SUSPENDU' : 'ACTIF' })}>
            {u.statut === 'ACTIF' ? 'Suspendre' : 'Réactiver'}
          </button>
        )}
        <FormError>{act.error}</FormError>
      </td>
    </tr>
  );
}

function QueueRow({ d, arbiters, onDone }: { d: S['Dispute']; arbiters: S['AdminUser'][]; onDone(): void }) {
  const [arb, setArb] = useState('');
  const act = useAction();
  return (
    <li style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
      <Link to={R.dispute(d.id)} style={{ flex: 1 }}>{shortId(d.id_echange)} · « {d.jalon} » · {d.montant} cr</Link>
      <select aria-label="Arbitre" value={arb} onChange={(e) => setArb(e.target.value)}>
        <option value="">Choisir un arbitre</option>
        {arbiters.map((a) => <option key={a.id} value={a.id}>{a.pseudo}</option>)}
      </select>
      <button className="btn btn-primary" style={small} disabled={!arb || act.busy}
        onClick={() => act.run(() => call(() => api.POST('/litiges/{id}/affecter', { params: { path: { id: d.id } }, body: { id_arbitre: arb } })), 'Arbitre affecté').then(onDone)}>
        Affecter
      </button>
      <FormError>{act.error}</FormError>
    </li>
  );
}
