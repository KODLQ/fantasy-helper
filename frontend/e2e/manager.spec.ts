import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

test.describe('manager and league workspace', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/manager/scopes', async (route) => {
      if (route.request().method() === 'PUT') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: JSON.parse(route.request().postData() ?? '{}'), meta: { requestId: 'scope' } }) });
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: [{ id: 1, type: 'entry', sourceId: 101, enabled: true, memberLimit: 50 }, { id: 2, type: 'league', sourceId: 202, enabled: true, memberLimit: 50 }] }, meta: { requestId: 'scopes' } }) });
    });
    await page.route('**/api/v1/manager/status', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { status: 'idle', completedWork: 0, failedWork: 0, freshness: { status: 'unavailable', state: 'unavailable' } }, meta: { requestId: 'status' } }) }));
    await page.route('**/api/v1/manager/sync', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { status: 'success', completedWork: 2, failedWork: 0, freshness: { status: 'fresh', state: 'fresh' } }, meta: { requestId: 'sync' } }) }));
    await page.route('**/api/v1/squad/import/preview**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { snapshot: { snapshotId: 44, entryId: 101, gameweek: 1, state: 'complete', conflictState: 'none', picks: [] }, proposed: {}, addedPlayerIds: [8], removedPlayerIds: [9], lineupChanged: true, captainChanged: true, validation: [], hasChanges: true }, meta: { requestId: 'preview' } }) }));
    await page.route('**/api/v1/manager/leagues/202/standings**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { leagueId: 202, name: 'Research League', page: 1, hasNext: false, members: [{ entryId: 101, entryName: 'Alpha XI', playerName: 'Alpha', rank: 1, lastRank: 2, points: 100 }, { entryId: 102, entryName: 'Beta XI', playerName: 'Beta', rank: 2, lastRank: 1, points: 95 }] }, meta: { requestId: 'standings' } }) }));
    await page.route('**/api/v1/manager/leagues/202/comparison**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { leagueId: 202, seasonId: 1, gameweek: 1, selectedEntryIds: [101, 102], omittedEntryIds: [103], outcomeState: 'actual', missingInputs: [], comparisons: [{ entryId: 101, sharedPlayers: [1, 2], differentials: [8], overlap: .5, netPoints: 70, pointDifference: 5, outcomeState: 'actual' }] }, meta: { requestId: 'comparison' } }) }));
  });

  test('configures, syncs, previews, and compares without exposing an overwrite action', async ({ page }) => {
    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'Manager & leagues' }).click();
    const workspace = page.getByTestId('manager-workspace');
    await expect(workspace).toBeVisible();
    await workspace.getByRole('button', { name: 'Sync manager data' }).click();
    await expect(workspace.getByRole('status')).toContainText('Manager sync completed');
    await workspace.getByRole('button', { name: 'Preview active team' }).click();
    await expect(page.getByTestId('import-preview')).toContainText('Added: 8');
    await expect(page.getByRole('button', { name: /replace planning squad/i })).toHaveCount(0);
    await workspace.getByRole('button', { name: 'Load league teams' }).click();
    await expect(workspace).toContainText('Research League');
    await workspace.getByRole('button', { name: 'Compare selected teams' }).click();
    await expect(page.getByTestId('league-comparison')).toContainText('50% overlap');
    await expect(page.getByTestId('league-comparison')).toContainText('Omitted entries: 103');
  });
});
