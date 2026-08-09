import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

const envelope = (data: unknown) => ({ data, meta: { requestId: 'pw-sync' } });

test('starts a manual sync and shows stage progress', async ({ page }) => {
  await page.route('**/api/v1/sync/status', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ status: 'running', runId: 12, currentStage: 'fixtures', completedStages: ['catalog'], completedWork: 4, totalWork: 10, freshness: { status: 'partial', state: 'partial' } })) }));
  await page.route('**/api/v1/sync', (route) => route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify(envelope({ status: 'running', runId: 12, currentStage: 'catalog', freshness: { status: 'unavailable', state: 'unavailable' } })) }));
  await openApp(page);
  await expect(page.getByTestId('sync-progress')).toContainText('Stage: fixtures');
  await expect(page.getByTestId('sync-progress')).toContainText('4/10 work items');
  await expect(page.getByRole('button', { name: 'Sync official data' })).toBeDisabled();
});

test('shows partial failure and retries failed work', async ({ page }) => {
  await page.route('**/api/v1/sync/status', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ status: 'partial', runId: 7, warning: 'Two player histories failed.', freshness: { status: 'partial', state: 'partial' } })) }));
  let retried = false;
  await page.route('**/api/v1/sync/runs/7/retry', (route) => { retried = true; return route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify(envelope({ status: 'running', runId: 7, currentStage: 'player-history', freshness: { status: 'partial', state: 'partial' } })) }); });
  await openApp(page);
  await expect(page.getByTestId('sync-progress')).toContainText('Two player histories failed.');
  await page.getByRole('button', { name: 'Retry failed work' }).click();
  await expect.poll(() => retried).toBeTruthy();
  await expect(page.getByTestId('sync-progress')).toContainText('Stage: player-history');
});

for (const [state, title] of [['stale', 'Warehouse data is stale'], ['unavailable', 'Demo snapshot is active']] as const) {
  test(`labels ${state} freshness`, async ({ page }) => {
    await page.route('**/api/v1/sync/status', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ status: state === 'unavailable' ? 'empty' : 'success', freshness: { status: state, state, warning: `${state} test warning` } })) }));
    await openApp(page);
    await expect(page.getByTestId('freshness-banner')).toContainText(title);
    await expect(page.getByTestId('freshness-banner')).toContainText(`${state} test warning`);
  });
}

test('surfaces sync response errors', async ({ page }) => {
  await page.route('**/api/v1/sync/status', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ status: 'success', freshness: { status: 'fresh', state: 'actual' } })) }));
  await page.route('**/api/v1/sync', (route) => route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: { code: 'sync_in_progress', message: 'A sync is already running.' }, meta: { requestId: 'pw-error' } }) }));
  await openApp(page);
  await page.getByRole('button', { name: 'Sync official data' }).click();
  await expect(page.getByText('A sync is already running.')).toBeVisible();
});
