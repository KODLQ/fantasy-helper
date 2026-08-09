import { expect, test } from '@playwright/test';
import { openApp } from './helpers';

test.describe('player research', () => {
  test('searches, filters, sorts, and opens a player profile', async ({ page }) => {
    await openApp(page);
    const table = page.getByTestId('player-table');
    await expect(table).toBeVisible();

    await table.getByLabel('Filter players').fill('Mason');
    await table.getByLabel('Filter by position').selectOption('3');
    await table.getByRole('button', { name: /Pts/ }).click();
    await expect(table.getByRole('columnheader', { name: /Pts/ })).toHaveAttribute('aria-sort', 'descending');

    await expect(table.getByRole('button', { name: /Mason/ }).first()).toBeVisible();
    await table.getByRole('button', { name: /Mason/ }).first().click();

    const drawer = page.getByTestId('player-drawer');
    await expect(drawer).toContainText('Mason');
    await expect(drawer).toContainText('Recent output');
    await expect(drawer).toContainText('Next fixture');
  });

  test('sends pagination, sorting, filtering, and page size to the player endpoint', async ({ page }) => {
    await openApp(page);
    const table = page.getByTestId('player-table');
    await expect(table.getByRole('button', { name: 'Next page' })).toBeEnabled();

    const secondPage = page.waitForRequest((request) => {
      const url = new URL(request.url());
      return url.pathname === '/api/v1/players' && url.searchParams.get('page') === '2';
    });
    await table.getByRole('button', { name: 'Next page' }).click();
    await secondPage;
    await expect(table.getByRole('button', { name: 'Page 2' })).toHaveAttribute('aria-current', 'page');

    const priceDescending = page.waitForRequest((request) => {
      const url = new URL(request.url());
      return url.pathname === '/api/v1/players' && url.searchParams.get('sort') === 'price' && url.searchParams.get('direction') === 'desc' && url.searchParams.get('page') === '1';
    });
    await table.getByRole('button', { name: 'Price', exact: true }).click();
    await priceDescending;

    const priceAscending = page.waitForRequest((request) => {
      const url = new URL(request.url());
      return url.pathname === '/api/v1/players' && url.searchParams.get('sort') === 'price' && url.searchParams.get('direction') === 'asc';
    });
    await table.getByRole('button', { name: 'Price', exact: true }).click();
    await priceAscending;

    const filtered = page.waitForRequest((request) => {
      const url = new URL(request.url());
      return url.pathname === '/api/v1/players' && url.searchParams.get('search') === 'Mason' && url.searchParams.get('position') === '3';
    });
    await table.getByLabel('Filter players').fill(' Mason ');
    await table.getByLabel('Filter by position').selectOption('3');
    await filtered;

    const resized = page.waitForRequest((request) => {
      const url = new URL(request.url());
      return url.pathname === '/api/v1/players' && url.searchParams.get('pageSize') === '25' && url.searchParams.get('page') === '1';
    });
    await table.getByLabel('Rows per page').selectOption('25');
    await resized;
  });
});
