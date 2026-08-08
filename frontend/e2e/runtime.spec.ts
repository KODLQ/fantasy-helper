import { expect, test } from '@playwright/test';

test('local Compose frontend is reachable and exposes freshness state', async ({ page }) => {
  const response = await page.goto('/');
  expect(response?.ok()).toBeTruthy();
  await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();
  await expect(page.getByTestId('freshness-banner')).toContainText(/Demo snapshot|Backend connection unavailable/);
});

