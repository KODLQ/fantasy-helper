import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

test.describe('player research', () => {
  test('searches, filters, sorts, and opens a player profile', async ({ page }) => {
    await openApp(page);
    await expect(page.getByTestId('freshness-banner')).toContainText('Demo snapshot is active');
    await expect(page.getByTestId('player-table')).toBeVisible();

    await page.getByPlaceholder('Search by player or club...').fill('Mason');
    await page.locator('.filter-bar select').nth(0).selectOption('3');
    await page.locator('.filter-bar select').nth(1).selectOption('points');

    await expect(page.getByRole('button', { name: /Mason/ }).first()).toBeVisible();
    await page.getByRole('button', { name: /Mason/ }).first().click();

    const drawer = page.getByTestId('player-drawer');
    await expect(drawer).toContainText('Mason');
    await expect(drawer).toContainText('Recent output');
    await expect(drawer).toContainText('Next fixture');
  });
});
