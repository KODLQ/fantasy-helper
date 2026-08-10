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
    await page.route('**/api/v1/squad/import/preview**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { snapshot: { snapshotId: 44, entryId: 101, gameweek: 1, state: 'partial', missingInputs: ['public-gameweek-picks'], conflictState: 'none', picks: [] }, proposed: {}, addedPlayerIds: [8], removedPlayerIds: [9], lineupChanged: true, captainChanged: true, validation: [], hasChanges: true }, meta: { requestId: 'preview' } }) }));
    await page.route('**/api/v1/squad/import', async (route) => { const input = JSON.parse(route.request().postData() ?? '{}'); return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { snapshotId: 44, mode: input.mode, draftId: input.mode === 'draft' ? 7 : undefined, planId: input.mode === 'replace' ? 8 : undefined, resultingVersion: 44, squad: {}, idempotent: false }, meta: { requestId: 'import' } }) }); });
    await page.route('**/api/v1/manager/connect', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { entryId: 101, state: 'connected', providerType: 'memory' }, meta: { requestId: 'connect' } }) }));
    await page.route('**/api/v1/manager/leagues/202/standings**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { leagueId: 202, name: 'Research League', page: 1, hasNext: false, members: Array.from({ length: 9 }, (_, index) => ({ entryId: 101 + index, entryName: `${String.fromCharCode(65 + index)} XI`, playerName: `Manager ${index + 1}`, rank: index + 1, lastRank: index + 1, points: 100 - index })) }, meta: { requestId: 'standings' } }) }));
    await page.route('**/api/v1/manager/leagues/202/comparison**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { leagueId: 202, seasonId: 1, gameweek: 1, selectedEntryIds: [101, 102], omittedEntryIds: [103], failedMemberEntryIds: [], ownership: [{ playerId: 1, count: 2, rate: 1 }], outcomeState: 'actual', missingInputs: [], comparisons: [{ entryId: 101, opponentEntryId: 102, sharedPlayers: [1, 2], differentials: [8], benchPlayerIds: [12, 13, 14, 15], benchDifferences: [15], captainId: 1, viceCaptainId: 2, formation: '3-4-3', overlap: .5, grossPoints: 74, transferCost: 4, netPoints: 70, pointDifference: 5, outcomeState: 'actual' }] }, meta: { requestId: 'comparison' } }) }));
  });

  test('configures, syncs, previews, confirms imports, and compares teams', async ({ page }) => {
    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'Manager & leagues' }).click();
    const workspace = page.getByTestId('manager-workspace');
    await expect(workspace).toBeVisible();
    const remoteSecret = 'sessionid=playwright-private-value';
    await workspace.getByLabel('FPL session').fill(remoteSecret);
    await workspace.getByRole('button', { name: 'Connect session' }).click();
    await expect(workspace.getByLabel('FPL session')).toHaveValue('');
    await expect(workspace.getByRole('status')).toContainText('FPL session connected');
    const browserStorage = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
    expect(browserStorage).not.toContain(remoteSecret);
    await workspace.getByRole('button', { name: 'Sync manager data' }).click();
    await expect(workspace.getByRole('status')).toContainText('Manager sync completed');
    await workspace.getByRole('button', { name: 'Preview active team' }).click();
    await expect(page.getByTestId('import-preview')).toContainText('Added: 8');
    await expect(page.getByTestId('import-preview')).toContainText('Missing: public-gameweek-picks');
    await page.getByRole('button', { name: 'Import as new draft' }).click();
    await expect(workspace.getByText('Planning draft created.')).toBeVisible();
    await page.getByRole('button', { name: 'Replace planning squad' }).click();
    await expect(page.getByRole('alert')).toContainText('Replace your saved planning squad');
    await page.getByRole('button', { name: 'Confirm replace' }).click();
    await expect(workspace.getByText('Planning squad replaced.')).toBeVisible();
    await workspace.getByRole('button', { name: 'Load league teams' }).click();
    await expect(workspace).toContainText('Research League');
    for (const name of ['E XI', 'F XI', 'G XI', 'H XI']) await workspace.getByLabel(new RegExp(name)).check();
    await expect(workspace.locator('input[type="checkbox"]:checked')).toHaveCount(8);
    await workspace.getByLabel(/I XI/).click();
    await expect(workspace.getByLabel(/I XI/)).not.toBeChecked();
    await workspace.getByRole('button', { name: 'Compare selected teams' }).click();
    await expect(page.getByTestId('league-comparison')).toContainText('50% overlap');
    await expect(page.getByTestId('league-comparison')).toContainText('3-4-3');
    await expect(page.getByTestId('league-comparison')).toContainText('captain 1');
    await expect(page.getByTestId('league-comparison')).toContainText('bench differences 1');
    await expect(page.getByTestId('league-comparison')).toContainText('net 70');
    await expect(page.getByTestId('league-comparison')).toContainText('Omitted entries: 103');
  });

  test('reports remote sync and missing active-team errors without retaining the submitted session', async ({ page }) => {
    await page.unroute('**/api/v1/manager/connect');
    await page.unroute('**/api/v1/manager/sync');
    await page.unroute('**/api/v1/squad/import/preview**');
    await page.route('**/api/v1/manager/connect', (route) => route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: { code: 'remote_authentication_failed', message: 'The FPL session could not be validated.' }, meta: { requestId: 'connect-failure' } }) }));
    await page.route('**/api/v1/manager/sync', (route) => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'manager_sync_failed', message: 'Manager synchronization failed.' }, meta: { requestId: 'sync-failure' } }) }));
    await page.route('**/api/v1/squad/import/preview**', (route) => route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'active_team_not_found', message: 'No synchronized active team was found.' }, meta: { requestId: 'preview-missing' } }) }));
    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'Manager & leagues' }).click();
    const workspace = page.getByTestId('manager-workspace');
    await workspace.getByLabel('FPL session').fill('sessionid=rejected-private-value');
    await workspace.getByRole('button', { name: 'Connect session' }).click();
    await expect(workspace.getByLabel('FPL session')).toHaveValue('');
    await expect(workspace.getByRole('status')).toContainText('could not be validated');
    await workspace.getByRole('button', { name: 'Sync manager data' }).click();
    await expect(workspace.getByRole('status')).toContainText('Manager synchronization failed');
    await workspace.getByRole('button', { name: 'Preview active team' }).click();
    await expect(workspace.getByRole('status')).toContainText('No synchronized active team was found');
  });

  test('renders empty preseason and no-change responses without crashing', async ({ page }) => {
    await page.unroute('**/api/v1/manager/leagues/202/standings**');
    await page.unroute('**/api/v1/squad/import/preview**');
    await page.route('**/api/v1/manager/leagues/202/standings**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { leagueId: 202, name: 'Preseason League', page: 1, hasNext: false, members: null }, meta: { requestId: 'empty-standings' } }) }));
    await page.route('**/api/v1/squad/import/preview**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { snapshot: { snapshotId: 44, entryId: 101, gameweek: 1, state: 'partial', missingInputs: ['public-gameweek-picks'], conflictState: 'none', picks: [] }, proposed: {}, addedPlayerIds: null, removedPlayerIds: null, lineupChanged: false, captainChanged: false, validation: null, hasChanges: false }, meta: { requestId: 'no-change-preview' } }) }));

    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'Manager & leagues' }).click();
    const workspace = page.getByTestId('manager-workspace');
    await workspace.getByRole('button', { name: 'Preview active team' }).click();
    await expect(page.getByTestId('import-preview')).toContainText('Added: None · Removed: None');
    await workspace.getByRole('button', { name: 'Load league teams' }).click();
    await expect(workspace.getByText('No ranked teams are available yet.')).toBeVisible();
    await expect(workspace.getByRole('button', { name: 'Compare selected teams' })).toBeDisabled();
  });
});
