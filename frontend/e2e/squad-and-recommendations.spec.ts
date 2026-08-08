import { expect, test } from '@playwright/test';
import { loadDemoSquad, openApp } from './helpers';

test.describe('squad planning and recommendations', () => {
  test('loads a valid squad and saves lineup controls', async ({ page }) => {
    await openApp(page);
    await loadDemoSquad(page);

    const controls = page.locator('.lineup-controls');
    await expect(controls).toContainText('Lineup controls');
    await controls.locator('select').nth(0).selectOption('3-4-3');
    await controls.getByRole('button', { name: 'Save lineup' }).click();
    await expect(page.getByTestId('squad-summary')).toContainText('15/15');
    await expect(page.getByTestId('squad-summary')).toContainText('3-4-3');
  });

  test('generates a lineup and captain recommendation', async ({ page }) => {
    await openApp(page);
    await loadDemoSquad(page);
    await page.getByRole('button', { name: /Optimize lineup/ }).click();
    await page.getByRole('button', { name: /Run recommendation/ }).click();

    await expect(page.getByTestId('recommendation-banner')).toBeVisible();
    await expect(page.getByTestId('recommendation-banner')).toContainText('RECOMMENDED CAPTAIN');
    await expect(page.getByRole('heading', { name: 'Starting XI' })).toBeVisible();
    await expect(page.locator('.heuristic-notice')).toContainText(/transparent heuristic/i);
  });
});
