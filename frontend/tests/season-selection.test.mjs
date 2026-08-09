import test from 'node:test';
import assert from 'node:assert/strict';
import { chooseDefaultSeason, chooseGameweek, positiveInteger, resolveSeasonSelection, seasonStatusLabel, sortSeasons } from '../src/season-selection.mjs';

const historical = { id: 2025, state: 'historical', availableGameweeks: [{ id: 1 }], defaultGameweek: 1 };
const current = { id: 2026, state: 'current', availableGameweeks: [{ id: 1 }, { id: 2 }], defaultGameweek: 2 };

test('catalogue sorting and deterministic defaults', () => {
  assert.deepEqual(sortSeasons([historical, current]).map(({ id }) => id), [2026, 2025]);
  assert.equal(chooseDefaultSeason([historical, current]), current);
  assert.equal(chooseDefaultSeason([{ ...historical, id: 2024 }, historical]), historical);
  assert.equal(chooseDefaultSeason([]), undefined);
  assert.equal(chooseGameweek(current, 1), 1);
  assert.equal(chooseGameweek(current, 99), 2);
});

test('URL selection wins, then remembered, current, and newest', () => {
  assert.equal(resolveSeasonSelection([current, historical], 2025, 2026, 1).season, historical);
  assert.equal(resolveSeasonSelection([current, historical], undefined, 2025).season, historical);
  assert.equal(resolveSeasonSelection([historical, current]).season, current);
  assert.equal(resolveSeasonSelection([historical, { ...historical, id: 2024 }]).season, historical);
});

test('unknown explicit and invalid remembered seasons are distinguished', () => {
  assert.deepEqual(resolveSeasonSelection([current], 1999, 2026), { unknownSeason: 1999 });
  const recovered = resolveSeasonSelection([current], undefined, 1999);
  assert.equal(recovered.season, current);
  assert.equal(recovered.discardRemembered, true);
  assert.equal(positiveInteger('2'), 2);
  assert.equal(positiveInteger('-1'), undefined);
});

test('status labels expose current, historical, partial, and unavailable states as text', () => {
  assert.equal(seasonStatusLabel({ state: 'current', freshness: { state: 'actual' }, missingInputs: [] }), 'Current season');
  assert.equal(seasonStatusLabel({ state: 'historical', freshness: { state: 'partial' }, missingInputs: [] }), 'Historical season · Partial data');
  assert.equal(seasonStatusLabel({ state: 'historical', freshness: { state: 'unavailable' }, missingInputs: ['catalogue'] }), 'Historical season · Data unavailable');
});
