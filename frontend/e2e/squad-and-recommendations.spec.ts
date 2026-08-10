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
    await expect(controls.getByRole('combobox', { name: 'Formation' }).locator('option')).toHaveCount(8);
  });

  test('keeps captain and vice-captain distinct by swapping armbands', async ({ page }) => {
    await openApp(page);
    await loadDemoSquad(page);
    const captain = page.getByRole('combobox', { name: 'Captain', exact: true });
    const viceCaptain = page.getByRole('combobox', { name: 'Vice-captain', exact: true });
    const originalCaptain = await captain.inputValue();
    const originalVice = await viceCaptain.inputValue();
    await captain.selectOption(originalVice);
    await expect(captain).toHaveValue(originalVice);
    await expect(viceCaptain).toHaveValue(originalCaptain);
    await page.getByRole('button', { name: 'Save lineup' }).click();
    await expect(page.locator('.lineup-controls').getByRole('alert')).toHaveCount(0);
  });

  test('generates a lineup and captain recommendation', async ({ page }) => {
    await openApp(page);
    await loadDemoSquad(page);
    await page.getByRole('button', { name: /Optimize lineup/ }).click();
    await page.getByRole('button', { name: /Run recommendation/ }).click();

    await expect(page.getByTestId('recommendation-banner')).toBeVisible();
    await expect(page.getByTestId('recommendation-banner')).toContainText('RECOMMENDED CAPTAIN');
    await expect(page.getByRole('heading', { name: 'Starting XI' })).toBeVisible();
    await expect(page.locator('.lineup-panel').first().locator('.muted')).toContainText(/^(3-4-3|3-5-2|4-5-1|4-4-2|4-3-3|5-4-1|5-3-2|5-2-3) · 11 starters$/);
    await expect(page.getByRole('heading', { name: 'Substitutes' })).toBeVisible();
    await expect(page.locator('.bench-panel .rank-badge')).toHaveText(['1', '2', '3', 'GK']);
    await expect(page.locator('.heuristic-notice')).toContainText(/transparent heuristic/i);
  });

  test('blocks invalid weight totals and clears stale recommendations', async ({ page }) => {
    await openApp(page);
    await loadDemoSquad(page);
    await page.getByRole('button', { name: /Optimize lineup/ }).click();
    const run = page.getByRole('button', { name: /Run recommendation/ });
    await run.click();
    await expect(page.getByTestId('recommendation-banner')).toBeVisible();
    await page.getByRole('spinbutton', { name: 'form' }).fill('0.5');
    await expect(run).toBeDisabled();
    await expect(page.getByRole('alert')).toContainText('total exactly 1.00');
    await expect(page.getByTestId('recommendation-banner')).toHaveCount(0);
  });
});
