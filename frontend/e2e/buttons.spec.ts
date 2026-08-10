import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

test.describe('application buttons', () => {
  test('workspace navigation and primary actions work', async ({ page }) => {
    await openApp(page);

    await page.getByRole('button', { name: /Open squad planner/ }).click();
    await expect(page.getByRole('heading', { name: 'Squad planner' })).toBeVisible();

    await page.locator('nav').getByRole('button', { name: 'Research' }).click();
    await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();

    await page.locator('nav').getByRole('button', { name: 'Recommendations' }).click();
    await expect(page.getByRole('heading', { name: 'Recommendations' })).toBeVisible();
    await page.locator('nav').getByRole('button', { name: 'Research' }).click();

    await page.locator('nav').getByRole('button', { name: /Compare/ }).click();
    await expect(page.getByRole('heading', { name: 'Compare players' })).toBeVisible();
    await page.getByRole('button', { name: 'Find players' }).click();
    await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();

    await page.route('**/api/v1/sync', (route) => route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ status: 'running', currentStage: 'catalog', freshness: { status: 'unavailable', state: 'unavailable' } }),
    }));
    await page.getByRole('button', { name: 'Sync official data' }).click();
    await expect(page.getByText('Official data sync started.')).toBeVisible();
  });

  test('research view, filter, card, and pagination buttons work', async ({ page }) => {
    await openApp(page);

    await page.getByRole('button', { name: /Cards/ }).click();
    await expect(page.getByTestId('player-cards')).toBeVisible();
    await expect(page.getByRole('button', { name: /Cards/ })).toHaveClass(/selected/);

    await page.getByTestId('player-cards').getByRole('button', { name: /Mason|Stone|Grant/ }).first().click();
    await expect(page.getByTestId('player-drawer')).toBeVisible();
    await page.getByRole('button', { name: 'Close player profile' }).click();

    await page.getByRole('button', { name: /Table/ }).click();
    await expect(page.getByTestId('player-table')).toBeVisible();

    const table = page.getByTestId('player-table');
    await table.getByLabel('Minimum form').fill('8');
    await expect(table).toContainText('Filters applied');
    await table.getByRole('button', { name: 'Clear filters' }).click();
    await expect(table.getByLabel('Minimum form')).toHaveValue('');

    await expect(table.getByRole('button', { name: 'Previous page' })).toBeDisabled();
    await expect(table.getByRole('button', { name: 'Next page' })).toBeEnabled();
    await table.getByRole('button', { name: 'Next page' }).click();
    await expect(table.getByRole('button', { name: 'Page 2' })).toHaveAttribute('aria-current', 'page');
    await expect(table.getByRole('button', { name: 'Previous page' })).toBeEnabled();
  });

  test('player, drawer, comparison, and empty-state buttons work', async ({ page }) => {
    await openApp(page);

    await page.locator('button[title="Compare"]').first().click();
    await page.locator('nav').getByRole('button', { name: /Compare/ }).click();
    await expect(page.getByTestId('comparison-card')).toHaveCount(1);
    await page.getByRole('button', { name: 'Back to research' }).click();
    await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();

    await page.getByRole('button', { name: 'Inspect player' }).nth(1).click();
    const drawer = page.getByTestId('player-drawer');
    await expect(drawer).toBeVisible();
    await drawer.getByRole('button', { name: '＋ Compare' }).click();
    await drawer.getByRole('button', { name: 'Close player profile' }).click();

    await page.locator('nav').getByRole('button', { name: /Compare/ }).click();
    await expect(page.getByTestId('comparison-card')).toHaveCount(2);
    await page.getByTestId('comparison-card').first().getByRole('button', { name: /Remove .* from comparison/ }).click();
    await expect(page.getByTestId('comparison-card')).toHaveCount(1);
    await page.getByTestId('comparison-card').first().getByRole('button', { name: /Remove .* from comparison/ }).click();
    await expect(page.getByRole('heading', { name: 'Your comparison is empty' })).toBeVisible();
    await page.getByRole('button', { name: 'Find players' }).click();
    await expect(page.getByRole('heading', { name: 'Player research' })).toBeVisible();
  });

  test('drawer add-to-squad button starts a planning draft with that player', async ({ page }) => {
    await openApp(page);
    await page.getByRole('button', { name: 'Inspect player' }).first().click();
    const playerName = (await page.getByTestId('player-drawer').getByRole('heading').first().textContent()) ?? '';
    await page.getByTestId('player-drawer').getByRole('button', { name: 'Add to squad' }).click();
    await expect(page.getByRole('heading', { name: 'Squad planner' })).toBeVisible();
    await expect(page.getByText('Squad planner opened.')).toBeVisible();
    await expect(page.getByTestId('squad-builder')).toContainText(playerName);
  });

  test('squad and recommendation action buttons work', async ({ page }) => {
    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'Squad planner' }).click();
    await page.getByRole('button', { name: 'Load demo squad' }).click();
    await expect(page.getByTestId('squad-summary')).toContainText('15/15');
    await page.getByRole('button', { name: 'Save lineup' }).click();
    await expect(page.getByTestId('squad-summary')).toContainText('3-4-3');

    await page.getByRole('button', { name: /Optimize lineup/ }).click();
    await page.getByRole('button', { name: /Run recommendation/ }).click();
    await expect(page.getByTestId('recommendation-banner')).toBeVisible();
  });
});
