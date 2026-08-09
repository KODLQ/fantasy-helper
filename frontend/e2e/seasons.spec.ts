import { expect, Page, test } from '@playwright/test';

const seasons = [
  { id: 2026, name: '2026/27', state: 'current', availableGameweeks: [{ id: 1, name: 'Gameweek 1', finished: false, isCurrent: false }, { id: 2, name: 'Gameweek 2', finished: false, isCurrent: false }], defaultGameweek: 1, sourceKind: 'official-current', freshness: { status: 'fresh', state: 'actual' }, completeness: {}, missingInputs: [], warnings: [] },
  { id: 2025, name: '2025/26', state: 'historical', availableGameweeks: [{ id: 1, name: 'Gameweek 1', finished: true, isCurrent: false }], defaultGameweek: 1, sourceKind: 'retained-snapshot', freshness: { status: 'partial', state: 'partial' }, completeness: {}, missingInputs: ['live'], warnings: ['Live detail unavailable'] },
];

const player = (seasonId: number) => ({ id: 10, firstName: 'Test', secondName: 'Player', webName: seasonId === 2026 ? 'Current Player' : 'Historical Player', position: 3, teamId: 1, price: 7, totalPoints: seasonId === 2026 ? 10 : 200, form: 5, minutes: 900, value: 10, status: 'a', news: '', expectedMinutes: 1, recentReturns: 0, goalsScored: 0, assists: 0, cleanSheets: 0, saves: 0 });

async function mockMultiSeason(page: Page, delayCurrent = 0) {
  await page.route('**/api/v1/seasons', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: seasons }, meta: { requestId: 'seasons' } }) }));
  await page.route('**/api/v1/players?*', async (route) => {
    const seasonId = Number(new URL(route.request().url()).searchParams.get('seasonId'));
    if (seasonId === 2026 && delayCurrent) await new Promise((resolve) => setTimeout(resolve, delayCurrent));
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: [player(seasonId)], total: 1, page: 1, pageSize: 25, freshness: { status: 'fresh', state: 'actual' } }, meta: { requestId: `players-${seasonId}`, scope: { seasonId } } }) });
  });
  await page.route('**/api/v1/players/10?*', async (route) => {
    const seasonId = Number(new URL(route.request().url()).searchParams.get('seasonId'));
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { player: player(seasonId), team: { name: seasonId === 2026 ? 'Current FC' : 'History FC', shortName: 'TST' }, history: [{ gameweek: 1, totalPoints: seasonId === 2026 ? 10 : 20, minutes: 90 }], fixtures: [{ homeTeam: 1, awayTeam: 2, homeDifficulty: seasonId === 2026 ? 2 : 5, awayDifficulty: 3 }], freshness: { status: 'fresh', state: 'actual' } }, meta: { requestId: `player-${seasonId}`, scope: { seasonId } } }) });
  });
}

test.describe('multi-season navigation', () => {
  test('defaults to current season and canonicalizes the URL', async ({ page }) => {
    await mockMultiSeason(page);
    await page.goto('/');
    await expect(page.getByTestId('season-selector')).toHaveValue('2026');
    await expect(page).toHaveURL(/season=2026.*gameweek=1/);
    await expect(page.getByText('Current Player')).toBeVisible();
    await page.getByRole('button', { name: 'Current Player' }).click();
    await expect(page.getByTestId('player-drawer')).toContainText('Current FC');
    await expect(page.getByTestId('player-drawer')).toContainText('Difficulty 2/5');
  });

  test('switches season, reconciles gameweek, and restores browser history', async ({ page }) => {
    let syncRequests = 0;
    page.on('request', (request) => {
      const url = new URL(request.url());
      if (request.method() === 'POST' && url.pathname === '/api/v1/sync') syncRequests += 1;
    });
    await mockMultiSeason(page);
    await page.goto('/?season=2026&gameweek=2');
    await expect(page.getByText('Current Player')).toBeVisible();
    await page.getByTestId('season-selector').selectOption('2025');
    await expect(page).toHaveURL(/season=2025.*gameweek=1/);
    await expect(page.getByText('Historical Player')).toBeVisible();
    expect(syncRequests).toBe(0);
    await expect(page.getByRole('status', { name: '' }).filter({ hasText: 'Historical season' })).toBeVisible();
    await page.getByTestId('season-selector').selectOption('2026');
    await expect(page).toHaveURL(/season=2026.*gameweek=1/);
    await expect(page.getByText('Current Player')).toBeVisible();
    await page.goBack();
    await expect(page.getByTestId('season-selector')).toHaveValue('2025');
    await page.goBack();
    await expect(page.getByTestId('season-selector')).toHaveValue('2026');
    await expect(page).toHaveURL(/season=2026.*gameweek=2/);
    await expect(page.getByText('Current Player')).toBeVisible();
  });

  test('clears prior-season player and comparison selections', async ({ page }) => {
    await mockMultiSeason(page);
    await page.goto('/?season=2026&gameweek=1');
    await page.getByRole('button', { name: 'Current Player' }).click();
    await page.getByRole('button', { name: '＋ Compare' }).click();
    await expect(page.getByTestId('player-drawer')).toBeVisible();
    await expect(page.locator('nav').getByRole('button', { name: /Compare · 1/ })).toBeVisible();
    await page.getByTestId('season-selector').selectOption('2025');
    await expect(page.getByTestId('player-drawer')).toHaveCount(0);
    const compareNavigation = page.locator('nav').getByRole('button', { name: /Compare/ });
    await expect(compareNavigation).toBeVisible();
    await expect(compareNavigation).not.toContainText('· 1');
    await expect(page.getByText('Historical Player')).toBeVisible();
  });

  test('preserves an explicit unknown season without redirecting', async ({ page }) => {
    await mockMultiSeason(page);
    await page.goto('/?season=1999&gameweek=1');
    await expect(page.getByTestId('season-not-found')).toContainText('Season 1999 is not available');
    await expect(page).toHaveURL(/season=1999/);
    await page.getByRole('button', { name: '2025/26' }).click();
    await expect(page.getByText('Historical Player')).toBeVisible();
  });

  test('supports keyboard-only season selection and textual partial status', async ({ page }) => {
    await mockMultiSeason(page);
    await page.goto('/?season=2026&gameweek=1');
    const selector = page.getByTestId('season-selector');
    await selector.focus();
    await expect(selector).toBeFocused();
    await page.keyboard.type('2025');
    await page.keyboard.press('Enter');
    await expect(selector).toHaveValue('2025');
    await expect(page.getByText(/Historical season · Partial data/)).toBeVisible();
  });

  test('discards a delayed response from the previous season', async ({ page }) => {
    await mockMultiSeason(page, 500);
    await page.goto('/?season=2026&gameweek=1');
    await page.getByTestId('season-selector').selectOption('2025');
    await expect(page.getByText('Historical Player')).toBeVisible();
    await expect(page.getByText('Current Player')).toHaveCount(0);
  });

  test('keeps different season scopes in separate tabs', async ({ context }) => {
    const current = await context.newPage();
    const historical = await context.newPage();
    await mockMultiSeason(current);
    await mockMultiSeason(historical);
    await current.goto('/?season=2026&gameweek=1');
    await historical.goto('/?season=2025&gameweek=1');
    await expect(current.getByText('Current Player')).toBeVisible();
    await expect(historical.getByText('Historical Player')).toBeVisible();
  });

  test('restores a remembered selection on an unscoped reload', async ({ page }) => {
    await mockMultiSeason(page);
    await page.goto('/?season=2025&gameweek=1');
    await expect(page.getByText('Historical Player')).toBeVisible();
    await page.goto('/');
    await expect(page.getByTestId('season-selector')).toHaveValue('2025');
    await expect(page).toHaveURL(/season=2025.*gameweek=1/);
    await page.reload();
    await expect(page.getByText('Historical Player')).toBeVisible();
  });

  test('shows empty and unavailable catalogue states without hiding navigation', async ({ page }) => {
    await page.route('**/api/v1/seasons', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: [] }, meta: { requestId: 'empty' } }) }));
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'No season data available' })).toBeVisible();
    await expect(page.getByTestId('season-selector')).toBeDisabled();

    const unavailable = { ...seasons[1], missingInputs: ['catalogue'] };
    await page.unroute('**/api/v1/seasons');
    await page.route('**/api/v1/seasons', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: [seasons[0], unavailable] }, meta: { requestId: 'unavailable' } }) }));
    await page.goto('/?season=2025&gameweek=1');
    await expect(page.getByTestId('season-data-unavailable')).toHaveAttribute('role', 'alert');
    await expect(page.getByTestId('season-selector')).toBeEnabled();
  });

  test('keeps the selector usable after a content request fails', async ({ page }) => {
    await page.route('**/api/v1/seasons', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: seasons }, meta: { requestId: 'seasons' } }) }));
    await page.route('**/api/v1/players?*', async (route) => {
      const seasonId = Number(new URL(route.request().url()).searchParams.get('seasonId'));
      if (seasonId === 2026) {
        await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'SEASON_DATA_UNAVAILABLE', message: 'Current catalogue unavailable.' } }) });
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: [player(seasonId)], total: 1, page: 1, pageSize: 25 }, meta: { requestId: 'recovered', scope: { seasonId } } }) });
    });
    await page.goto('/?season=2026&gameweek=1');
    await expect(page.getByText('Current catalogue unavailable.')).toBeVisible();
    await page.getByTestId('season-selector').selectOption('2025');
    await expect(page.getByText('Historical Player')).toBeVisible();
  });

  test('announces catalogue loading and recovers from a catalogue error', async ({ page }) => {
    let recover = false;
    await page.route('**/api/v1/seasons', async (route) => {
      if (!recover) {
        await new Promise((resolve) => setTimeout(resolve, 1500));
        await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'season_catalogue_unavailable', message: 'Catalogue temporarily unavailable.' } }) });
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: seasons }, meta: { requestId: 'recovered' } }) });
    });
    await page.goto('/');
    await expect(page.getByText('Loading available FPL seasons…')).toBeVisible();
    await expect(page.getByRole('alert')).toContainText('Catalogue temporarily unavailable.');
    recover = true;
    await page.getByRole('button', { name: 'Try again' }).click();
    await expect(page.getByTestId('season-selector')).toHaveValue('2026');
    await expect(page).toHaveURL(/season=2026.*gameweek=1/);
  });
});
