import { expect, test, type Page } from '@playwright/test';

/** Lien du dernier email reçu par une adresse (boîte du serveur e2e), en chemin relatif. */
async function lastLink(page: Page, to: string): Promise<string> {
  const res = await page.request.get(`/fake-mail/dernier?a=${encodeURIComponent(to)}`);
  expect(res.ok()).toBeTruthy();
  const { Text } = (await res.json()) as { Text: string };
  const m = Text.match(/https?:\/\/[^\s]+jeton=[\w-]+/);
  expect(m).not.toBeNull();
  const u = new URL(m![0]);
  return u.pathname + u.search;
}

test('compte email : inscription, confirmation, connexion, liaison GitHub, mot de passe oublié', async ({ page }) => {
  const email = 'claire@example.com';
  await page.goto('/');
  await page.getByRole('tab', { name: 'Créer un compte' }).click();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Mot de passe').fill('une phrase de passe');
  await page.getByLabel('Pseudo (facultatif)').fill('claire');
  await page.getByRole('button', { name: 'Créer mon compte' }).click();
  await expect(page.getByText(`Un lien de confirmation a été envoyé à ${email}`)).toBeVisible();

  // Connexion refusée tant que l'adresse n'est pas confirmée.
  await page.getByRole('button', { name: 'Retour à la connexion' }).click();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Mot de passe').fill('une phrase de passe');
  await page.getByRole('button', { name: 'Se connecter', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Renvoyer le lien de confirmation' })).toBeVisible();

  await page.goto(await lastLink(page, email));
  await expect(page.getByText('Adresse confirmée')).toBeVisible();
  await page.getByRole('link', { name: 'Se connecter' }).click();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Mot de passe').fill('une phrase de passe');
  await page.getByRole('button', { name: 'Se connecter', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Bonjour claire' })).toBeVisible();
  await expect(page.getByRole('button', { name: /Solde : 50 crédits disponibles/ })).toBeVisible();

  // Sans GitHub : pas d'offre, liaison depuis le profil.
  await page.goto('/offres/nouvelle');
  await expect(page.getByRole('heading', { name: 'Compte GitHub requis' })).toBeVisible();
  await page.getByRole('link', { name: 'Aller à mon profil' }).click();
  await page.getByRole('button', { name: 'Lier mon compte GitHub' }).click();
  await page.getByRole('link', { name: 'Continuer en tant que zoe' }).click();
  await expect(page.getByRole('heading', { name: 'Bonjour claire' })).toBeVisible();
  await page.goto('/offres/nouvelle');
  await expect(page.getByRole('button', { name: "Publier l'offre" })).toBeVisible();

  // Mot de passe oublié depuis une session vierge.
  await page.context().clearCookies();
  await page.goto('/?vue=oubli');
  await page.getByLabel('Email').fill(email);
  await page.getByRole('button', { name: 'Recevoir un lien' }).click();
  await expect(page.getByText(/un lien de réinitialisation vient d’y être envoyé/)).toBeVisible();
  await page.goto(await lastLink(page, email));
  await page.getByLabel('Nouveau mot de passe').fill('un tout nouveau secret');
  await page.getByLabel('Confirmer').fill('un tout nouveau secret');
  await page.getByRole('button', { name: 'Enregistrer' }).click();
  await expect(page.getByText('Mot de passe changé')).toBeVisible();
  await page.getByRole('button', { name: 'Se connecter' }).click();
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Mot de passe').fill('un tout nouveau secret');
  await page.getByRole('button', { name: 'Se connecter', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Bonjour claire' })).toBeVisible();
});
