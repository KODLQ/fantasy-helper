import { expect, Page } from '@playwright/test';

let authSequence = 0;
export const testPassword = 'Playwright-local-password-42';

export async function openApp(page: Page) {
  await page.goto('/?season=1&gameweek=1');
  await expect(page.getByTestId('app-shell')).toBeVisible();
  if (await page.getByTestId('auth-panel').count()) {
    const email = `playwright-${Date.now()}-${authSequence++}@example.test`;
    await page.getByTestId('auth-panel').getByRole('button', { name: 'Create a local account' }).click();
    await page.getByTestId('auth-panel').getByLabel('Display name').fill('Playwright User');
    await page.getByTestId('auth-panel').getByLabel('Email').fill(email);
    await page.getByTestId('auth-panel').getByLabel('Password').fill(testPassword);
    await page.getByTestId('auth-panel').getByRole('button', { name: 'Create account', exact: true }).click();
    await expect(page.getByTestId('authenticated-user')).toBeVisible();
  }
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
