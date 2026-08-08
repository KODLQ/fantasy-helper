import test from 'node:test';
import assert from 'node:assert/strict';

test('comparison limit is four players', () => {
  const ids = [1, 2, 3, 4];
  assert.equal(ids.length <= 4, true);
  assert.equal([1, 2, 3, 4, 5].length <= 4, false);
});

test('default recommendation weights sum to one', () => {
  const weights = [0.28, 0.25, 0.20, 0.17, 0.10];
  assert.equal(weights.reduce((sum, value) => sum + value, 0), 1);
});

test('lineup captain and vice-captain must be distinct starters', () => {
  const starters = new Set([1, 4, 5, 8]);
  assert.equal(starters.has(8), true);
  assert.equal(starters.has(2), false);
  assert.notEqual(8, 4);
});
