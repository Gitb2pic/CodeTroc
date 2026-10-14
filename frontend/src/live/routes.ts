export const LIVE_ROUTES = {
  landing: '/',
  dash: '/tableau-de-bord',
  cat: '/catalogue',
  offer: (id: string) => `/catalogue/${id}`,
  create: '/offres/nouvelle',
  exchange: (id: string) => `/echanges/${id}`,
  wallet: '/portefeuille',
  notif: '/notifications',
  disputes: '/litiges',
  dispute: (id: string) => `/litiges/${id}`,
  member: (id: string) => `/membres/${id}`,
  admin: '/administration',
};
