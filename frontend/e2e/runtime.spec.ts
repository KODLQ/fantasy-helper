import { expect, test } from '@playwright/test';

test('local Compose frontend is reachable and exposes season scope', async ({ page }) => {
  const response = await page.goto('/');
  expect(response?.ok()).toBeTruthy();
  await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();
  await expect(page.getByTestId('season-selector')).toBeVisible();
  await expect(page.getByTestId('player-table')).toBeVisible();
});
