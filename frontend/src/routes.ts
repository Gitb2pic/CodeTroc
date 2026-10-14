export const ROUTES = {
  landing: '/',
  onboarding: '/onboarding',
  dash: '/tableau-de-bord',
  cat: '/catalogue',
  pack: (id: string) => `/catalogue/${id}`,
  create: '/packs/nouveau',
  ex: '/echanges/EX-0142',
  vault: '/coffre',
  wallet: '/portefeuille',
  profile: '/membres/leamartin',
  dispute: '/litiges/EX-0131',
  notif: '/notifications',
};
