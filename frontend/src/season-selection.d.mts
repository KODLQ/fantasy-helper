type SeasonSelectionItem = {
  id: number;
  state: 'current' | 'historical';
  defaultGameweek?: number;
  availableGameweeks: { id: number }[];
};

type SeasonStatusItem = {
  state: 'current' | 'historical';
  freshness?: { state?: string };
  missingInputs?: string[];
};

export function positiveInteger(value: unknown): number | undefined;
export function sortSeasons<T extends SeasonSelectionItem>(items: T[]): T[];
export function chooseDefaultSeason<T extends SeasonSelectionItem>(items: T[]): T | undefined;
export function chooseGameweek(season: SeasonSelectionItem, requested?: number): number | undefined;
export function resolveSeasonSelection<T extends SeasonSelectionItem>(items: T[], explicit?: number, remembered?: number, requestedGameweek?: number): {
  season?: T;
  gameweek?: number;
  unknownSeason?: number;
  discardRemembered?: boolean;
};
export function seasonStatusLabel(season: SeasonStatusItem): string;
