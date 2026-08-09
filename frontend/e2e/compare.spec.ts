import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

test.describe('player comparison', () => {
  test('compares four players side by side', async ({ page }) => {
    await openApp(page);
    const compareButtons = page.locator('button[title="Compare"]');
    for (let index = 0; index < 4; index += 1) await compareButtons.nth(index).click();

    await page.locator('nav').getByRole('button', { name: /Compare/ }).click();
    await expect(page.getByRole('heading', { name: 'Compare the shortlist' })).toBeVisible();
    await expect(page.getByTestId('comparison-card')).toHaveCount(4);
    await expect(page.getByTestId('comparison-card').first()).toContainText('Fixture not loaded');
    await expect(page.getByTestId('comparison-card').first()).not.toContainText('Difficulty 3/5');
  });

  test('prevents a fifth player from being compared', async ({ page }) => {
    await openApp(page);
    const compareButtons = page.locator('button[title="Compare"]');
    for (let index = 0; index < 5; index += 1) await compareButtons.nth(index).click();

    await expect(page.getByText('Comparison is limited to four players.')).toBeVisible();
    await page.locator('nav').getByRole('button', { name: /Compare/ }).click();
    await expect(page.getByTestId('comparison-card')).toHaveCount(4);
  });
});
