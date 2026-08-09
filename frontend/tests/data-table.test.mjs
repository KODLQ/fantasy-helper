import assert from 'node:assert/strict';
import test from 'node:test';
import { nextSort, normalizeFilters, pageWindow, resultRange, totalPages } from '../src/data-table-state.mjs';

test('totalPages keeps empty and invalid result sets on the first page', () => {
  assert.equal(totalPages(0, 10), 1);
  assert.equal(totalPages(27, 10), 3);
  assert.equal(totalPages(27, 0), 1);
});

test('pageWindow stays within the available pages', () => {
  assert.deepEqual(pageWindow(1, 10), [1, 2, 3, 4, 5]);
  assert.deepEqual(pageWindow(5, 10), [3, 4, 5, 6, 7]);
  assert.deepEqual(pageWindow(10, 10), [6, 7, 8, 9, 10]);
  assert.deepEqual(pageWindow(4, 3), [1, 2, 3]);
});

test('resultRange reports the visible server page and empty state', () => {
  assert.deepEqual(resultRange(2, 10, 27, 10), { start: 11, end: 20 });
  assert.deepEqual(resultRange(3, 10, 27, 7), { start: 21, end: 27 });
  assert.deepEqual(resultRange(1, 10, 0, 0), { start: 0, end: 0 });
});

test('nextSort applies the column default then toggles direction', () => {
  assert.deepEqual(nextSort({ key: 'form', direction: 'desc' }, 'price', 'desc'), { key: 'price', direction: 'desc' });
  assert.deepEqual(nextSort({ key: 'price', direction: 'desc' }, 'price', 'desc'), { key: 'price', direction: 'asc' });
  assert.deepEqual(nextSort(undefined, 'name'), { key: 'name', direction: 'asc' });
});

test('normalizeFilters trims values and removes nullish representations', () => {
  assert.deepEqual(normalizeFilters({ search: '  Mason ', minForm: 8, status: null }), { search: 'Mason', minForm: '8', status: '' });
});
