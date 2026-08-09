import { expect, Page } from '@playwright/test';

export async function openApp(page: Page) {
  await page.goto('/?season=1&gameweek=1');
  await expect(page.getByTestId('app-shell')).toBeVisible();
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
