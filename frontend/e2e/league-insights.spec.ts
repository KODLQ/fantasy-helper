import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

const members = Array.from({ length: 12 }, (_, index) => ({ entryId: 101 + index, entryName: `${String.fromCharCode(65 + index)} Research XI`, playerName: `Manager ${index + 1}`, rank: index + 1, lastRank: index + 2, points: 120 - index }));
const snapshots = Object.fromEntries(members.map((member) => [member.entryId, `pick-${member.entryId}-gw1`]));

test.describe('league gameweek insights', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/seasons', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ items: [{ id: 1, name: '2026/27', state: 'current', availableGameweeks: [{ id: 1, name: 'Gameweek 1', finished: false, isCurrent: true }], defaultGameweek: 1, sourceKind: 'official-current', freshness: { status: 'fresh', state: 'actual' }, completeness: {}, missingInputs: [], warnings: [] }] }) }));
    await page.route('**/api/v1/players?*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ items: [], teams: [], total: 0, page: 1, pageSize: 25, freshness: { status: 'fresh', state: 'actual' } }) }));
    await page.route('**/api/v1/sync/status', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ status: 'success', freshness: { status: 'fresh', state: 'actual' } }) }));
    await page.route('**/api/v1/auth/config', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ registrationEnabled: true, emailProviderConfigured: false, minimumPasswordLength: 12 }) }));
    await page.route('**/api/v1/auth/me', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ user: { id: 1, email: 'insights@example.test', displayName: 'Insight Tester', status: 'active', createdAt: '2026-08-16T09:00:00Z', updatedAt: '2026-08-16T09:00:00Z' }, csrfToken: 'test-csrf', expiresAt: '2026-08-17T09:00:00Z' }) }));
    await page.route('**/api/v1/manager/scopes', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: { items: [{ type: 'entry', sourceId: 101, enabled: true, memberLimit: 50 }, { type: 'league', sourceId: 202, enabled: true, memberLimit: 50 }] }, meta: { requestId: 'insight-scopes' } }) }));
  });

  test('filters and selects rivals, compares teams, and explains a gameweek', async ({ page }) => {
    await page.route('**/api/v1/analysis/leagues/202/summary**', async (route) => {
      expect(new URL(route.request().url()).searchParams.get('memberLimit')).toBe('10');
      await new Promise((resolve) => setTimeout(resolve, 120));
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ leagueId: 202, name: 'Research League', seasonId: 1, gameweek: 1, members, selectedEntryIds: members.slice(0, 10).map((member) => member.entryId), omittedEntryIds: [111, 112], snapshotIds: snapshots, coverage: { requested: 10, selected: 10, complete: 10, omitted: 2, missingEntryIds: [] }, outcomeState: 'actual', formulaVersions: ['team-overlap-v1', 'differential-contribution-v1', 'team-points-difference-v1'], missingInputs: [], warnings: [] }) });
    });
    await page.route('**/api/v1/analysis/leagues/202/comparison**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ leagueId: 202, seasonId: 1, gameweek: 1, selectedEntryIds: [101, 102], omittedEntryIds: members.slice(2).map((member) => member.entryId), failedMemberEntryIds: [], ownership: [], sourceSnapshotAt: '2026-08-16T10:00:00Z', snapshotIds: { 101: snapshots[101], 102: snapshots[102] }, coverage: { requested: 2, selected: 2, complete: 2, omitted: 10, missingEntryIds: [] }, formulaVersions: ['team-overlap-v1', 'differential-contribution-v1', 'team-points-difference-v1'], warnings: [], missingInputs: [], outcomeState: 'actual', comparisons: [comparison(101, 102, 7), comparison(102, 101, -7)] }) }));
    await page.route('**/api/v1/analysis/gameweeks/1/autopsy**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ entryId: 101, rivalEntryId: 102, seasonId: 1, gameweek: 1, grossPoints: 78, transferCost: 4, netPoints: 74, captainId: 1, captainDelta: 8, benchPoints: 6, activeChip: '', transfers: [{ gameweek: 1, playerIn: 8, playerOut: 9, playerInCost: 70, playerOutCost: 65, madeAt: '2026-08-15T10:00:00Z' }], automaticSubstitutions: [{ playerIn: 12, playerOut: 4, impact: 3 }], contributions: [{ playerId: 1, basePoints: 8, multiplier: 2, effectivePoints: 16 }], rivalComparison: comparison(101, 102, 7), outcomeState: 'actual', snapshotIds: { 101: snapshots[101], 102: snapshots[102] }, formulaVersions: ['player-points-v1', 'captain-delta-v1', 'automatic-substitution-impact-v1'], missingInputs: [], warnings: [], metricsAvailable: true }) }));

    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'League insights' }).focus();
    await page.keyboard.press('Enter');
    const workspace = page.getByTestId('league-insights');
    await expect(workspace).toBeVisible();
    await workspace.getByLabel('Member limit').selectOption('10');
    await workspace.getByRole('button', { name: 'Load league overview' }).click();
    await expect(workspace.getByRole('button', { name: 'Loading…' })).toBeDisabled();
    await expect(page.getByTestId('league-member-table')).toContainText('A Research XI');
    await expect(workspace.getByRole('heading', { name: 'Research League' })).toBeVisible();
    await expect(workspace).toContainText('10/10 selected teams');
    await page.getByTestId('league-member-table').getByLabel('Filter league members').fill('Manager 12');
    await expect(page.getByTestId('league-member-table')).toContainText('L Research XI');
    await expect(page.getByTestId('league-member-table')).not.toContainText('A Research XI');
    await page.getByTestId('league-member-table').getByRole('button', { name: /Rank/ }).click();
    await page.getByTestId('league-member-table').getByRole('button', { name: 'Clear filters' }).click();
    await workspace.getByRole('button', { name: 'Compare 4 selected teams' }).click();
    const table = page.getByTestId('insight-comparison-table');
    await expect(table).toContainText('73%');
    await expect(table).toContainText('64%');
    await expect(table).toContainText('+5');
    await expect(table).toContainText('+7');
    await table.getByRole('button', { name: /Point gap/ }).focus();
    await page.keyboard.press('Enter');
    await workspace.getByRole('button', { name: 'Explain GW 1' }).click();
    const autopsy = page.getByTestId('gameweek-autopsy');
    await expect(autopsy).toContainText('Gross78');
    await expect(autopsy).toContainText('Hits-4');
    await expect(autopsy).toContainText('4→12 (+3)');
    await expect(autopsy).toContainText('Against entry 102: +7 net points');
    await expect(autopsy).toContainText('automatic-substitution-impact-v1');
    await expect(workspace.locator('.analysis-provenance')).toContainText('Omitted: 111, 112');
    await expect(workspace.locator('.analysis-provenance')).toContainText('101: pick-101-gw1');
  });

  test('labels stale, provisional, partial, and unavailable data without presenting it as final', async ({ page }) => {
    await page.route('**/api/v1/analysis/leagues/202/summary**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ leagueId: 202, seasonId: 1, gameweek: 1, members: members.slice(0, 3), selectedEntryIds: [101, 102, 103], omittedEntryIds: [], snapshotIds: { 101: snapshots[101], 102: snapshots[102] }, coverage: { requested: 3, selected: 3, complete: 2, omitted: 0, missingEntryIds: [103] }, outcomeState: 'stale', formulaVersions: ['team-overlap-v1'], missingInputs: ['entry:103:picks'], warnings: ['Manager snapshots are stale.'] }) }));
    await page.route('**/api/v1/analysis/leagues/202/comparison**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ leagueId: 202, seasonId: 1, gameweek: 1, selectedEntryIds: [101, 102], omittedEntryIds: [103], failedMemberEntryIds: [], ownership: [], snapshotIds: { 101: snapshots[101], 102: snapshots[102] }, coverage: { requested: 3, selected: 2, complete: 2, omitted: 1, missingEntryIds: [103] }, formulaVersions: ['team-overlap-v1'], warnings: ['One selected rival is missing.'], missingInputs: ['entry:103:picks'], outcomeState: 'partial', comparisons: [comparison(101, 102, 2)] }) }));
    await page.route('**/api/v1/analysis/gameweeks/1/autopsy**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ entryId: 101, seasonId: 1, gameweek: 1, grossPoints: 45, transferCost: 0, netPoints: 45, captainDelta: 6, benchPoints: 3, transfers: [], automaticSubstitutions: [], contributions: [], outcomeState: 'provisional', snapshotIds: { 101: snapshots[101] }, formulaVersions: ['player-points-v1'], missingInputs: [], warnings: ['The gameweek is live.'], metricsAvailable: true, unfinishedFixtureIds: [501, 502] }) }));
    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'League insights' }).click();
    const workspace = page.getByTestId('league-insights');
    await workspace.getByRole('button', { name: 'Load league overview' }).click();
    await expect(workspace.locator('.analysis-state.stale')).toContainText('Manager snapshots are stale');
    await expect(page.getByTestId('league-member-table').getByLabel('Select C Research XI')).toBeDisabled();
    await workspace.getByRole('button', { name: 'Compare 2 selected teams' }).click();
    await expect(page.getByTestId('insight-comparison').locator('.analysis-state.partial')).toContainText('One selected rival is missing');
    await workspace.getByRole('button', { name: 'Explain GW 1' }).click();
    await expect(page.getByTestId('gameweek-autopsy')).toContainText('GW 1 autopsy · provisional');
    await expect(page.getByTestId('gameweek-autopsy')).toContainText('Unfinished fixtures: 501, 502');
  });

  test('renders empty and request-error states', async ({ page }) => {
    await page.route('**/api/v1/analysis/leagues/202/summary**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ leagueId: 202, seasonId: 1, gameweek: 1, members: [], selectedEntryIds: [], omittedEntryIds: [], snapshotIds: {}, coverage: { requested: 0, selected: 0, complete: 0, omitted: 0, missingEntryIds: [] }, outcomeState: 'unavailable', formulaVersions: ['team-overlap-v1'], missingInputs: ['league-standings-snapshot'], warnings: [] }) }));
    await openApp(page);
    await page.locator('nav').getByRole('button', { name: 'League insights' }).click();
    const workspace = page.getByTestId('league-insights');
    await workspace.getByRole('button', { name: 'Load league overview' }).click();
    await expect(page.getByTestId('league-member-table')).toContainText('No results match those filters');
    await expect(workspace.locator('.analysis-state.unavailable')).toContainText('league-standings-snapshot');
    await page.unroute('**/api/v1/analysis/leagues/202/summary**');
    await page.route('**/api/v1/analysis/leagues/202/summary**', (route) => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'league_analysis_unavailable', message: 'League analysis is unavailable.' }, meta: { requestId: 'insight-error' } }) }));
    await workspace.getByRole('button', { name: 'Load league overview' }).click();
    await expect(workspace.getByRole('alert')).toContainText('League analysis is unavailable');
  });
});

function comparison(entryId: number, opponentEntryId: number, pointDifference: number) {
  return { entryId, opponentEntryId, sharedPlayers: [1, 2], differentials: [8], benchPlayerIds: [12, 13, 14, 15], benchDifferences: [15], captainId: 1, viceCaptainId: 2, captainDifferent: true, viceCaptainDifferent: false, formation: '3-4-3', formationDifferent: false, overlap: .73, startingXIOverlap: .64, differentialContribution: 5, captainDelta: 8, grossPoints: 78, transferCost: 4, netPoints: 74, pointDifference, playerContributions: { 1: 16 }, outcomeState: 'actual' };
}

function envelope(data: unknown) { return JSON.stringify({ data, meta: { requestId: 'league-insight', scope: { seasonId: 1, gameweek: 1, dataset: 'manager-fpl' }, freshness: { state: 'fresh', status: 'fresh' } } }); }
