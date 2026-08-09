import { expect, Page } from '@playwright/test';

let authSequence = 0;
export const testPassword = 'Playwright-local-password-42';

export async function openProfileMenu(page: Page) {
  await page.getByRole('button', { name: /Sign in or create account|Open profile menu/ }).click();
  const menu = page.getByTestId('profile-menu');
  await expect(menu).toBeVisible();
  await expect(menu.locator('[data-testid="auth-panel"], [data-testid="authenticated-user"]')).toBeVisible();
  return menu;
}

export async function openApp(page: Page) {
  await page.goto('/?season=1&gameweek=1');
  await expect(page.getByTestId('app-shell')).toBeVisible();
  const profile = await openProfileMenu(page);
  if (await profile.getByTestId('auth-panel').count()) {
    const email = `playwright-${Date.now()}-${authSequence++}@example.test`;
    const panel = profile.getByTestId('auth-panel');
    await panel.getByRole('button', { name: 'Create a local account' }).click();
    await panel.getByLabel('Display name').fill('Playwright User');
    await panel.getByLabel('Email').fill(email);
    await panel.getByLabel('Password').fill(testPassword);
    await panel.getByRole('button', { name: 'Create account', exact: true }).click();
    await expect(profile.getByTestId('authenticated-user')).toBeVisible();
  }
  await profile.getByRole('button', { name: 'Close profile menu' }).click();
}

export async function openSquadPlanner(page: Page) {
  await page.locator('nav').getByRole('button', { name: 'Squad planner' }).click();
  await expect(page.getByRole('heading', { name: 'Your squad, your call.' })).toBeVisible();
  await expect(page.getByTestId('squad-summary')).toBeVisible();
}

export async function loadDemoSquad(page: Page) {
  await openSquadPlanner(page);
  await page.getByRole('button', { name: 'Load demo squad' }).click();
  await expect(page.getByTestId('squad-summary')).toContainText('15/15');
}
