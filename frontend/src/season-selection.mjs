export function positiveInteger(value) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

export function sortSeasons(items) {
  return [...items].sort((left, right) => right.id - left.id);
}

export function chooseDefaultSeason(items) {
  return items.find((item) => item.state === 'current') ?? sortSeasons(items)[0];
}

export function chooseGameweek(season, requested) {
  if (requested && season.availableGameweeks.some((item) => item.id === requested)) return requested;
  return season.defaultGameweek;
}

export function resolveSeasonSelection(items, explicit, remembered, requestedGameweek) {
  if (explicit && !items.some((item) => item.id === explicit)) {
    return { unknownSeason: explicit };
  }
  const rememberedSeason = items.find((item) => item.id === remembered);
  const season = items.find((item) => item.id === explicit) ?? rememberedSeason ?? chooseDefaultSeason(items);
  return {
    season,
    gameweek: season ? chooseGameweek(season, requestedGameweek) : undefined,
    discardRemembered: Boolean(remembered && !rememberedSeason),
  };
}

export function seasonStatusLabel(season) {
  const parts = [season.state === 'historical' ? 'Historical season' : 'Current season'];
  if (season.missingInputs?.includes('catalogue')) parts.push('Data unavailable');
  else if (season.freshness?.state === 'partial') parts.push('Partial data');
  return parts.join(' · ');
}
