import { expect, Page, test } from '@playwright/test';
import { openProfileMenu, testPassword } from './helpers';

const uniqueEmail = (label: string) => `${label}-${Date.now()}-${Math.random().toString(16).slice(2)}@example.test`;

async function register(page: Page, email: string, password = testPassword) {
  await page.goto('/?season=1&gameweek=1');
  const profile = await openProfileMenu(page);
  const panel = profile.getByTestId('auth-panel');
  await panel.getByRole('button', { name: 'Create a local account' }).click();
  await panel.getByLabel('Display name').fill('Authentication Test');
  await panel.getByLabel('Email').fill(email);
  await panel.getByLabel('Password').fill(password);
  const responsePromise = page.waitForResponse((response) => response.url().endsWith('/api/v1/auth/register') && response.request().method() === 'POST');
  await panel.getByRole('button', { name: 'Create account', exact: true }).click();
  const response = await responsePromise;
  const body = await response.json() as { data: { csrfToken: string } };
  await expect(profile.getByTestId('authenticated-user')).toContainText(email);
  return body.data.csrfToken;
}

test.describe('local authentication lifecycle', () => {
  test('registers, persists across reload, rejects duplicates, and logs out', async ({ page }) => {
    const email = uniqueEmail('lifecycle');
    await register(page, email);
    await page.reload();
    const profile = await openProfileMenu(page);
    await expect(profile.getByTestId('authenticated-user')).toContainText(email);
    await profile.getByTestId('authenticated-user').getByRole('button', { name: 'Sign out' }).click();
    await expect(page.getByTestId('auth-panel')).toBeVisible();

    await page.getByTestId('auth-panel').getByRole('button', { name: 'Create a local account' }).click();
    await page.getByTestId('auth-panel').getByLabel('Display name').fill('Duplicate');
    await page.getByTestId('auth-panel').getByLabel('Email').fill(email.toUpperCase());
    await page.getByTestId('auth-panel').getByLabel('Password').fill(testPassword);
    await page.getByTestId('auth-panel').getByRole('button', { name: 'Create account', exact: true }).click();
    await expect(page.getByTestId('auth-panel').getByRole('alert')).toContainText('cannot be created');
  });

  test('uses generic invalid credentials and rotates sessions after password change', async ({ page }) => {
    const email = uniqueEmail('password');
    const nextPassword = 'Playwright-new-password-84';
    await register(page, email);
    const userPanel = page.getByTestId('authenticated-user');
    await userPanel.getByRole('button', { name: 'Change password' }).click();
    await userPanel.getByLabel('Current password').fill(testPassword);
    await userPanel.getByLabel('New password').fill(nextPassword);
    await userPanel.getByRole('button', { name: 'Update password' }).click();
    await expect(userPanel).toContainText('Password changed');
    await userPanel.getByRole('button', { name: 'Sign out' }).click();

    const panel = page.getByTestId('auth-panel');
    await panel.getByLabel('Email').fill(email);
    await panel.getByLabel('Password').fill(testPassword);
    await panel.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(panel.getByRole('alert')).toContainText('Email or password is incorrect');
    await panel.getByLabel('Password').fill(nextPassword);
    await panel.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(page.getByTestId('authenticated-user')).toContainText(email);
  });

  test('keeps public research open and denies private workspaces after expiry', async ({ page }) => {
    await page.goto('/?season=1&gameweek=1');
    await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();
    await page.locator('nav').getByRole('button', { name: 'Squad planner' }).click();
    await expect(page.getByTestId('protected-workspace')).toBeVisible();

    const email = uniqueEmail('expiry');
    await register(page, email);
    await page.getByRole('button', { name: 'Close profile menu' }).click();
    await page.route('**/api/v1/squad**', (route) => route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: { code: 'authentication_required', message: 'Sign in to access this workspace.' }, meta: { requestId: 'expired' } }) }));
    await page.locator('nav').getByRole('button', { name: 'Research' }).click();
    await page.locator('nav').getByRole('button', { name: 'Squad planner' }).click();
    await expect(page.getByTestId('protected-workspace')).toBeVisible();
    await expect(page.getByTestId('protected-workspace').getByTestId('auth-panel')).toHaveCount(0);
    await page.getByTestId('protected-workspace').getByRole('button', { name: 'Open sign in' }).click();
    await expect(page.getByTestId('profile-menu').getByTestId('auth-panel')).toContainText('session expired');
  });

  test('honors disabled registration configuration', async ({ page }) => {
    await page.route('**/api/v1/auth/config', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { registrationEnabled: false, emailProviderConfigured: false, minimumPasswordLength: 12 }, meta: { requestId: 'config' } }) }));
    await page.route('**/api/v1/auth/me', (route) => route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: { code: 'authentication_required', message: 'Sign in.' }, meta: { requestId: 'me' } }) }));
    await page.goto('/');
    await openProfileMenu(page);
    await expect(page.getByTestId('auth-panel')).toContainText('Registration is disabled');
    await expect(page.getByRole('button', { name: 'Create a local account' })).toHaveCount(0);
  });

  test('opens authentication from the profile ring and not the sidebar', async ({ page }) => {
    await page.goto('/?season=1&gameweek=1');
    await expect(page.locator('.sidebar').getByTestId('auth-panel')).toHaveCount(0);
    await expect(page.getByTestId('auth-panel')).toHaveCount(0);
    const profile = await openProfileMenu(page);
    await expect(profile.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(profile).toBeHidden();
  });

  test('isolates two private squads while sharing public warehouse reads', async ({ browser }) => {
    const firstContext = await browser.newContext();
    const secondContext = await browser.newContext();
    const firstPage = await firstContext.newPage();
    const secondPage = await secondContext.newPage();
    try {
      const firstCSRF = await register(firstPage, uniqueEmail('first-owner'));
      const secondCSRF = await register(secondPage, uniqueEmail('second-owner'));
      const baseSquad = { budget: 100, purchasePrices: Object.fromEntries([1, 2, 4, 5, 6, 7, 22, 8, 9, 10, 11, 12, 13, 14, 15].map((id) => [id, 5])), startingPlayerIds: [1, 4, 5, 6, 8, 9, 10, 11, 13, 14, 15], benchPlayerIds: [2, 7, 12, 22], captainId: 13, viceCaptainId: 8, formation: '3-4-3' };
      const firstSave = await firstPage.request.put('http://localhost:8080/api/v1/squad?seasonId=1', { headers: { Origin: 'http://localhost:5173', 'X-CSRF-Token': firstCSRF }, data: { ...baseSquad, name: 'First owner squad' } });
      const secondSave = await secondPage.request.put('http://localhost:8080/api/v1/squad?seasonId=1', { headers: { Origin: 'http://localhost:5173', 'X-CSRF-Token': secondCSRF }, data: { ...baseSquad, name: 'Second owner squad' } });
      expect(firstSave.ok()).toBeTruthy();
      expect(secondSave.ok()).toBeTruthy();
      const firstRead = await firstPage.request.get('http://localhost:8080/api/v1/squad?seasonId=1');
      const secondRead = await secondPage.request.get('http://localhost:8080/api/v1/squad?seasonId=1');
      expect((await firstRead.json()).data.name).toBe('First owner squad');
      expect((await secondRead.json()).data.name).toBe('Second owner squad');
      expect((await firstPage.request.get('http://localhost:8080/api/v1/players?seasonId=1')).ok()).toBeTruthy();
      expect((await secondPage.request.get('http://localhost:8080/api/v1/players?seasonId=1')).ok()).toBeTruthy();
    } finally {
      await firstContext.close();
      await secondContext.close();
    }
  });
});
