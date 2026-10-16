import type { S } from '../api/client';
import type { Pack, Tone } from '../domain/types';

export function initials(name: string): string {
  const parts = name.replace(/[-_.]/g, ' ').trim().split(/\s+/);
  return ((parts[0]?.[0] ?? '?') + (parts[1]?.[0] ?? parts[0]?.[1] ?? '')).toUpperCase();
}

const dateFmt = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' });
const timeFmt = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

export const fmtDate = (iso: string | null | undefined) => (iso ? dateFmt.format(new Date(iso)) : '—');
export const fmtTime = (iso: string | null | undefined) => (iso ? timeFmt.format(new Date(iso)) : '—');

export function fmtRelative(iso: string, now = Date.now()): string {
  const s = Math.round((now - new Date(iso).getTime()) / 1000);
  if (s < 60) return "à l'instant";
  if (s < 3600) return `il y a ${Math.floor(s / 60)} min`;
  if (s < 86400) return `il y a ${Math.floor(s / 3600)} h`;
  if (s < 7 * 86400) return `il y a ${Math.floor(s / 86400)} j`;
  return fmtDate(iso);
}

/** Identifiant court d'un échange, pour l'affichage. */
export const shortId = (id: string) => `EX-${id.slice(0, 6).toUpperCase()}`;

/** Une offre de l'API vue comme une carte du catalogue. */
export function offerToPack(o: S['Offer']): Pack {
  return {
    id: o.id,
    title: o.titre,
    stack: o.pack.competences,
    price: o.prix_credits,
    days: o.delai_jours,
    ms: o.jalons.length,
    crit: 0,
    name: o.prestataire.pseudo,
    handle: o.prestataire.github_login,
    ini: initials(o.prestataire.pseudo),
    rep: o.prestataire.score_reputation,
    deliv: o.prestataire.livraisons_verifiees,
  };
}

export const EXCHANGE_STATUS: Record<S['ExchangeStatus'], [string, Tone]> = {
  PROPOSE: ['Proposé', 'wait'],
  REFUSE: ['Refusé', 'mute'],
  EN_COURS: ['En cours', 'ac'],
  EN_LITIGE: ['En litige', 'disp'],
  TERMINE: ['Terminé', 'ok'],
  ANNULE: ['Annulé', 'mute'],
};

export const MILESTONE_STATUS: Record<S['MilestoneStatus'], [string, Tone, string]> = {
  EN_ATTENTE: ['À venir', 'mute', 'circle-dashed'],
  EN_COURS: ['En cours', 'wait', 'loader-circle'],
  LIVRE: ['Livré · à valider', 'ac', 'git-pull-request'],
  CONTESTE: ['Contesté', 'disp', 'flag'],
  LIBERE: ['Libéré', 'ok', 'circle-check'],
  REMBOURSE: ['Remboursé', 'mute', 'undo-2'],
  PARTAGE: ['Partagé', 'disp', 'scale'],
};

export const MOVEMENT: Record<S['Movement']['type'], [string, Tone]> = {
  BONUS: ['Bonus', 'mute'],
  BLOCAGE: ['Séquestre', 'wait'],
  LIBERATION: ['Libéré', 'ok'],
  REMBOURSEMENT: ['Remboursement', 'disp'],
};

/** Texte et icône d'une notification ; les données viennent de l'API (jamais de secret). */
export function notificationText(n: S['Notification']): { icon: string; text: string; tone: Tone } {
  const d = n.donnees as Record<string, unknown>;
  const pack = typeof d.pack === 'string' ? ` · ${d.pack}` : '';
  const jalon = typeof d.jalon === 'string' ? ` « ${d.jalon} »` : '';
  const map: Record<string, [string, string, Tone]> = {
    ECHANGE_DEMANDE: ['inbox', `Nouvelle demande d'échange${pack}`, 'ac'],
    ECHANGE_ACCEPTE: ['circle-check', `Ta demande a été acceptée${pack}`, 'ok'],
    ECHANGE_REFUSE: ['circle-x', `Ta demande a été refusée${pack}`, 'mute'],
    ANNULATION_DEMANDEE: ['undo-2', `Annulation demandée par l'autre partie${pack}`, 'wait'],
    ECHANGE_ANNULE: ['undo-2', `Échange annulé${pack}`, 'mute'],
    ECHANGE_TERMINE: ['party-popper', `Échange terminé${pack}`, 'ok'],
    JALON_LIVRE: ['git-pull-request', `Jalon${jalon} livré, à valider${pack}`, 'ac'],
    JALON_LIBERE: ['lock-open', `${d.montant ?? ''} cr libérés pour le jalon${jalon}`, 'ok'],
    PREUVE_REJETEE: ['triangle-alert', `Preuve rejetée : ${String(d.detail ?? '')}`, 'fail'],
    LITIGE_OUVERT: ['flag', `Litige ouvert sur le jalon${jalon}`, 'disp'],
    LITIGE_AFFECTE: ['gavel', `Litige à arbitrer${jalon}`, 'disp'],
    LITIGE_REPONSE: ['message-square', `Réponse au litige${jalon}`, 'disp'],
    LITIGE_DECIDE: ['scale', `Décision rendue sur le jalon${jalon}`, 'disp'],
    MESSAGE_RECU: ['message-square', `Message de ${String(d.auteur ?? 'ton binôme')}${pack}`, 'mute'],
    SECRET_PARTAGE: ['key-round', `Accès partagé : ${String(d.libelle ?? '')}`, 'ac'],
    SECRET_REVELE: ['eye', `Ton accès a été consulté${pack}`, 'wait'],
    AVIS_RECU: ['star', `Nouvel avis : ${d.note ?? '?'}/5${pack}`, 'ok'],
    CANDIDATURE_RECUE: ['hand', `Nouvelle candidature à ton annonce « ${String(d.annonce ?? '')} »`, 'ac'],
    CANDIDATURE_ACCEPTEE: ['circle-check', `Ta candidature est retenue : « ${String(d.annonce ?? '')} »`, 'ok'],
    CANDIDATURE_REFUSEE: ['circle-x', `Candidature non retenue : « ${String(d.annonce ?? '')} »`, 'mute'],
    SPRINT_TERMINE: ['list-checks', `Sprint ${String(d.sprint ?? '')} terminé : ${String(d.points_faits ?? 0)}/${String(d.points_prevus ?? 0)} points, place à la rétro${pack}`, 'ac'],
    JALON_A_RISQUE: ['triangle-alert', `Jalon${jalon} à risque de retard${pack}`, 'wait'],
    JALON_EN_RETARD: ['timer', `Échéance dépassée pour le jalon${jalon}${pack}`, 'fail'],
    AVENANT_PROPOSE: ['history', `Nouvelles échéances proposées${pack}`, 'wait'],
    AVENANT_DECIDE: ['history', `Avenant de planning ${d.statut === 'ACCEPTE' ? 'accepté' : d.statut === 'RETIRE' ? 'retiré' : 'refusé'}${pack}`, 'mute'],
  };
  const [icon, text, tone] = map[n.type] ?? ['bell', n.type, 'mute'];
  return { icon, text, tone };
}

/** Message utilisateur d'une erreur d'API. */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : 'Une erreur est survenue.';
}
