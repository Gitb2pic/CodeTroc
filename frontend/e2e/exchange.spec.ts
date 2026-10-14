import { expect, test, type Browser, type Page } from '@playwright/test';

/** Capture d'écran facultative (E2E_SHOTS=dossier) pour la revue visuelle. */
async function shot(page: Page, name: string) {
  if (process.env.E2E_SHOTS) await page.screenshot({ path: `${process.env.E2E_SHOTS}/${name}.png`, fullPage: true });
}

/** Ouvre une session dans un navigateur isolé via la fausse page GitHub. */
async function loginAs(browser: Browser, login: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage();
  await page.goto('/');
  await page.getByRole('button', { name: 'Se connecter avec GitHub' }).click();
  await page.getByRole('link', { name: `Continuer en tant que ${login}` }).click();
  await expect(page).toHaveURL(/\/tableau-de-bord/);
  await expect(page.getByRole('heading', { name: `Bonjour ${login}` })).toBeVisible();
  return page;
}

test('parcours complet : offre, demande, séquestre, PR vérifiée, libération, avis', async ({ browser }) => {
  const lea = await loginAs(browser, 'lea');
  const sami = await loginAs(browser, 'sami');

  // Léa publie une offre à 50 cr en deux jalons.
  await lea.getByRole('button', { name: 'Proposer une offre' }).first().click();
  await lea.getByLabel('Pack', { exact: true }).selectOption({ label: 'Suite de tests E2E Playwright' });
  await lea.getByLabel('Titre du jalon 1').fill('Parcours de connexion');
  await lea.getByLabel('Montant du jalon 1').fill('25');
  await lea.getByRole('button', { name: '+ Jalon' }).click();
  await lea.getByLabel('Titre du jalon 2').fill('Parcours de paiement');
  await lea.getByLabel('Montant du jalon 2').fill('25');
  await lea.getByRole('button', { name: "Publier l'offre" }).click();
  await expect(lea.getByRole('heading', { name: 'Suite de tests E2E Playwright' })).toBeVisible();

  // Sami la trouve au catalogue et la demande.
  await sami.getByRole('button', { name: 'Catalogue', exact: true }).click();
  await sami.getByRole('button', { name: /Suite de tests E2E Playwright/ }).click();
  await sami.getByLabel('Dépôt GitHub concerné').fill('https://github.com/acme/shop-api');
  await sami.getByRole('button', { name: 'Demander ce pack' }).click();
  await expect(sami.getByText('En attente de lea')).toBeVisible();
  await shot(sami, '1-demande');
  const exchangeUrl = sami.url();

  // Léa accepte : les 50 cr de Sami passent en séquestre.
  await lea.goto(exchangeUrl);
  await lea.getByRole('button', { name: 'Accepter' }).click();
  await expect(lea.getByText('Livre le jalon « Parcours de connexion »')).toBeVisible();
  await sami.reload();
  await expect(sami.getByRole('button', { name: /Solde : 0 crédits disponibles, 50 en séquestre/ })).toBeVisible();

  // Sami partage un jeton par le coffre ; Léa le révèle.
  await sami.getByRole('button', { name: '+ Partager' }).click();
  await sami.getByLabel('Libellé').fill('NPM_TOKEN');
  await sami.getByLabel('Valeur', { exact: true }).fill('npm_e2e_secret_value');
  await sami.getByRole('button', { name: 'Chiffrer et partager' }).click();
  await expect(sami.getByText('NPM_TOKEN')).toBeVisible();
  await expect(sami.getByText('npm_e2e_secret_value')).toHaveCount(0);
  await lea.reload();
  await lea.getByRole('button', { name: 'Révéler (journalisé)' }).click();
  await expect(lea.getByText('npm_e2e_secret_value')).toBeVisible();
  await shot(lea, '2-coffre-revele');

  // Messages.
  await sami.getByLabel('Message').fill('Les tests sont dans /e2e, merci !');
  await sami.getByRole('button', { name: 'Envoyer' }).click();
  await lea.reload();
  await expect(lea.getByText('Les tests sont dans /e2e, merci !')).toBeVisible();

  // Jalon 1 : PR vérifiée par le (faux) GitHub, puis validation par Sami.
  await lea.getByLabel('URL de la pull request').fill('https://github.com/acme/shop-api/pull/42');
  await lea.getByRole('button', { name: 'Livrer' }).click();
  await expect(lea.getByText('acme/shop-api').first()).toBeVisible();
  await sami.reload();
  await expect(sami.getByRole('button', { name: 'Valider · libérer 25 cr' })).toBeVisible();
  await shot(sami, '3-a-valider');
  await sami.getByRole('button', { name: 'Valider · libérer 25 cr' }).click();
  await expect(sami.getByText('Libéré', { exact: true }).first()).toBeVisible();

  // Jalon 2, puis clôture.
  await lea.reload();
  await lea.getByLabel('URL de la pull request').fill('https://github.com/acme/shop-api/pull/43');
  await lea.getByRole('button', { name: 'Livrer' }).click();
  await sami.reload();
  await sami.getByRole('button', { name: 'Valider · libérer 25 cr' }).click();
  await expect(sami.getByText('Terminé').first()).toBeVisible();

  // Avis, puis portefeuille de Léa : 50 de bonus + 50 gagnés.
  await sami.getByRole('radio', { name: '5 sur 5' }).click();
  await sami.getByRole('button', { name: "Publier l'avis" }).click();
  await lea.getByRole('button', { name: 'Portefeuille', exact: true }).click();
  await expect(lea.getByText('100').first()).toBeVisible();
  await expect(lea.getByRole('button', { name: /Notifications : \d+ non lues/ })).toBeVisible();
  await shot(lea, '4-portefeuille');
  await lea.getByRole('button', { name: /Notifications : / }).click();
  await expect(lea.getByText(/cr libérés pour le jalon/).first()).toBeVisible();
  await shot(lea, '5-notifications');
});

test('un tiers ne voit pas un échange qui ne le concerne pas', async ({ browser }) => {
  const admin = await loginAs(browser, 'admin');
  await admin.goto('/echanges/00000000-0000-4000-8000-000000000000');
  await expect(admin.getByText('Cet échange introuvable.')).toBeVisible();
});
