import { expect, test } from '@playwright/test';
import { loadDemoSquad, openApp } from './helpers';

const meta = { requestId: 'transfer-research-test', scope: { seasonId: 1, gameweek: 1, dataset: 'public-fpl' }, freshness: { status: 'fresh', state: 'actual' } };

test.describe('transfer and fixture research', () => {
  test.beforeEach(async ({ page }) => {
    await openApp(page);
    await loadDemoSquad(page);
    await page.route('**/api/v1/analysis/fixtures/swing**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: { items: [
      { team: { id: 1, name: 'Northbridge', shortName: 'NOR' }, ease: .82, fixtureCount: 5, gameweekCount: 5, blankGameweeks: [], doubleGameweeks: [], fixtures: [{ id: 1, gameweek: 1, homeTeam: 1, awayTeam: 2, homeDifficulty: 2, awayDifficulty: 4 }] },
      { team: { id: 2, name: 'Riverside', shortName: 'RIV' }, ease: .42, fixtureCount: 4, gameweekCount: 5, blankGameweeks: [3], doubleGameweeks: [], fixtures: [{ id: 2, gameweek: 1, homeTeam: 1, awayTeam: 2, homeDifficulty: 2, awayDifficulty: 4 }] },
    ], gameweekFrom: 1, gameweekTo: 5, horizon: 5, state: 'actual', snapshotId: 'snapshot-fixture', observedAt: '2026-08-14T12:00:00Z', formulaVersion: 'fixture-research-v1', missingInputs: [] }, meta }) }));
    await page.route('**/api/v1/analysis/differentials**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: { items: [{ player: { id: 16, webName: 'Nash', firstName: 'Ivo', secondName: 'Nash', position: 3, teamId: 2, price: 5, totalPoints: 45, form: 4.1, minutes: 500, value: 9, status: 'a', news: '', expectedMinutes: .62, recentReturns: .26, goalsScored: 0, assists: 0, cleanSheets: 0, saves: 0, selectedByPercent: 3.2 }, team: { id: 2, name: 'Riverside', shortName: 'RIV' }, score: .76, components: { pointsPer90: 8.1, minutesShare: .93, fixtureEase: .42, ownershipSignal: .968, availability: 1 }, explanation: '8.1 points/90, 93% minutes share, 3.2% ownership' }], peerCount: 1, state: 'actual', snapshotId: 'snapshot-differential', observedAt: '2026-08-14T12:00:00Z', formulaVersion: 'differential-opportunity-v1', researchNotice: 'Research ranking only; this is not an official FPL prediction.', missingInputs: [] }, meta }) }));
    await page.route('**/api/v1/analysis/transfers/simulate', async (route) => {
      const request = route.request().postDataJSON();
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: { simulationId: 'sim-playwright', algorithmVersion: 'transfer-simulation-v1', before: { players: [], purchasePrices: {}, remainingBudget: 25 }, after: { players: [], purchasePrices: {}, remainingBudget: 25.5, validation: [] }, transfers: request.transfers, freeTransfers: request.freeTransfers, freeTransfersUsed: 0, paidTransfers: 1, pointsHit: 4, fixtureEaseBefore: .5, fixtureEaseAfter: .64, fixtureEaseDelta: .14, state: 'actual', snapshotId: 'snapshot-transfer', observedAt: '2026-08-14T12:00:00Z', formulaVersions: ['fixture-research-v1', 'transfer-simulation-v1'], assumptions: ['Outgoing value uses the saved purchase price.'], missingInputs: [] }, meta }) });
    });
    await page.route('**/api/v1/planning/scenarios', (route) => route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ data: { id: 7, name: 'My move', simulationId: 'sim-playwright', seasonId: 1, gameweek: 1, createdAt: '2026-08-14T12:00:00Z' }, meta }) }));
    await page.locator('nav').getByRole('button', { name: 'Transfers & fixtures' }).click();
  });

  test('calculates, explains, resets, and saves a read-only transfer', async ({ page }) => {
	await page.getByRole('button', { name: 'Add another transfer' }).click();
	await expect(page.getByLabel('Transfer out 2')).toBeVisible();
	await page.getByRole('button', { name: 'Remove transfer 2' }).click();
    await page.getByLabel('Transfer out 1').selectOption('5');
    await page.getByLabel('Transfer in 1').selectOption('17');
    await page.getByLabel('Free transfers').fill('0');
    await page.getByRole('button', { name: 'Calculate move' }).click();
    const result = page.getByTestId('simulation-result');
    await expect(result).toContainText('Cole → Oak');
    await expect(result).toContainText('−4');
    await expect(result).toContainText('snapshot-transfer');
    await page.getByLabel('Scenario name').fill('My move');
	await page.getByRole('button', { name: 'Save to planning' }).click();
	await expect(page.getByRole('alert')).toContainText('active squad stays unchanged');
	await page.getByRole('button', { name: 'Cancel' }).click();
	await expect(page.getByRole('button', { name: 'Confirm save' })).toHaveCount(0);
	await page.getByRole('button', { name: 'Save to planning' }).click();
	await page.getByRole('button', { name: 'Confirm save' }).click();
    await expect(page.getByRole('alert')).toContainText('Saved “My move”');
    await page.getByRole('button', { name: 'Reset' }).click();
    await expect(result).toHaveCount(0);
    await expect(page.getByLabel('Transfer out 1')).toHaveValue('');
  });

  test('shows sortable and filterable fixture and differential tables with provenance', async ({ page }) => {
    await page.getByRole('tab', { name: 'Fixture swings' }).click();
    const fixtures = page.getByTestId('fixture-table');
    await expect(fixtures).toContainText('Northbridge');
    await expect(page.locator('.formula-provenance')).toContainText('fixture-research-v1');
    await fixtures.getByLabel('Filter clubs').fill('River');
    await expect(fixtures).toContainText('Riverside');
    await expect(fixtures).not.toContainText('Northbridge');
    await fixtures.getByRole('button', { name: /Ease/ }).click();
    await expect(fixtures.getByRole('columnheader', { name: /Ease/ })).toHaveAttribute('aria-sort', 'ascending');

    await page.getByRole('tab', { name: 'Differentials' }).click();
    const differentials = page.getByTestId('differential-table');
    await expect(differentials).toContainText('Nash');
    await expect(page.locator('.formula-provenance')).toContainText('not an official FPL prediction');
	const filteredRequest = page.waitForRequest((request) => new URL(request.url()).pathname === '/api/v1/analysis/differentials' && new URL(request.url()).searchParams.get('position') === '3');
	await page.getByLabel('Differential position').selectOption('3');
	await filteredRequest;
    await expect(differentials).toContainText('8.1 points/90');
  });

	test('exposes loading, empty, partial, stale, unavailable, validation, and request errors', async ({ page }) => {
		await page.unroute('**/api/v1/analysis/fixtures/swing**');
		await page.route('**/api/v1/analysis/fixtures/swing**', async (route) => {
			const horizon = Number(new URL(route.request().url()).searchParams.get('horizon'));
			await new Promise((resolve) => setTimeout(resolve, horizon === 5 ? 350 : 0));
			const state = horizon === 5 ? 'partial' : horizon === 4 ? 'stale' : 'unavailable';
			const items = horizon === 5 ? [] : [{ team: { id: 1, name: 'Northbridge', shortName: 'NOR' }, ease: .5, fixtureCount: 1, gameweekCount: horizon, blankGameweeks: [], doubleGameweeks: [], fixtures: [{ id: 1, gameweek: 1, homeTeam: 1, awayTeam: 2, homeDifficulty: 2, awayDifficulty: 4 }] }];
			await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: { items, gameweekFrom: 1, gameweekTo: horizon, horizon, state, snapshotId: `snapshot-${state}`, observedAt: '2026-08-14T12:00:00Z', formulaVersion: 'fixture-research-v1', missingInputs: state === 'partial' ? ['fixture_coverage'] : [] }, meta }) });
		});
		await page.getByRole('tab', { name: 'Fixture swings' }).click();
		await expect(page.getByTestId('fixture-table')).toContainText('Loading results');
		await expect(page.getByTestId('fixture-table')).toContainText('No results match');
		await expect(page.locator('.formula-provenance')).toContainText('partial');
		await page.getByLabel('Research horizon').selectOption('4');
		await expect(page.locator('.formula-provenance')).toContainText('stale');
		await page.getByLabel('Research horizon').selectOption('3');
		await expect(page.locator('.formula-provenance')).toContainText('unavailable');

		await page.unroute('**/api/v1/analysis/differentials**');
		await page.route('**/api/v1/analysis/differentials**', (route) => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'research_unavailable', message: 'Differential inputs are unavailable.' }, meta }) }));
		await page.getByRole('tab', { name: 'Differentials' }).click();
		await expect(page.getByTestId('differential-table').getByRole('alert')).toContainText('Differential inputs are unavailable');

		await page.unroute('**/api/v1/analysis/transfers/simulate');
		await page.route('**/api/v1/analysis/transfers/simulate', (route) => route.fulfill({ status: 422, contentType: 'application/json', body: JSON.stringify({ error: { code: 'simulation_invalid', message: 'The transfer exceeds the available budget.' }, meta }) }));
		await page.getByRole('tab', { name: 'Transfer lab' }).click();
		await page.getByLabel('Transfer out 1').selectOption('5');
		await page.getByLabel('Transfer in 1').selectOption('17');
		await page.getByRole('button', { name: 'Calculate move' }).click();
		await expect(page.getByRole('alert')).toContainText('exceeds the available budget');
	});
});
