import { createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { api, Season } from './api';
import { chooseGameweek, positiveInteger, resolveSeasonSelection } from './season-selection.mjs';

const STORAGE_KEY = 'fantasy-helper:selected-season';

type SeasonContextValue = {
  seasons: Season[];
  season?: Season;
  seasonId?: number;
  gameweek?: number;
  loading: boolean;
  error: string;
  notice: string;
  unknownSeason?: number;
  selectSeason: (id: number) => void;
  selectGameweek: (id: number) => void;
  retry: () => void;
};

const SeasonContext = createContext<SeasonContextValue | undefined>(undefined);

export function SeasonProvider({ children }: { children: ReactNode }) {
  const [seasons, setSeasons] = useState<Season[]>([]);
  const [seasonId, setSeasonId] = useState<number>();
  const [gameweek, setGameweek] = useState<number>();
  const [unknownSeason, setUnknownSeason] = useState<number>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [reload, setReload] = useState(0);

  const applyLocation = useCallback((items: Season[], replaceMissing: boolean) => {
    const params = new URLSearchParams(window.location.search);
    const explicit = positiveInteger(params.get('season'));
    const remembered = positiveInteger(window.localStorage.getItem(STORAGE_KEY));
    const resolved = resolveSeasonSelection(items, explicit, remembered, positiveInteger(params.get('gameweek')));
    if (resolved.unknownSeason) {
      setUnknownSeason(resolved.unknownSeason);
      setSeasonId(undefined);
      setGameweek(undefined);
      return;
    }
    setUnknownSeason(undefined);
    const selected = resolved.season;
    if (resolved.discardRemembered) {
      window.localStorage.removeItem(STORAGE_KEY);
      setNotice('Your previously selected season is no longer available. Showing the default season.');
    }
    if (!selected) {
      setSeasonId(undefined);
      setGameweek(undefined);
      return;
    }
    const selectedGameweek = resolved.gameweek;
    setSeasonId(selected.id);
    setGameweek(selectedGameweek);
    window.localStorage.setItem(STORAGE_KEY, String(selected.id));
    if (replaceMissing && (!explicit || positiveInteger(params.get('gameweek')) !== selectedGameweek)) {
      params.set('season', String(selected.id));
      if (selectedGameweek) params.set('gameweek', String(selectedGameweek));
      else params.delete('gameweek');
      window.history.replaceState({}, '', `${window.location.pathname}?${params.toString()}${window.location.hash}`);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    api.seasons(controller.signal).then(({ items }) => {
      setSeasons(items);
      applyLocation(items, true);
    }).catch((reason) => {
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : 'Could not load available seasons.');
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [applyLocation, reload]);

  useEffect(() => {
    const onPopState = () => applyLocation(seasons, false);
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, [applyLocation, seasons]);

  const selectSeason = (id: number) => {
    const selected = seasons.find((item) => item.id === id);
    if (!selected) return;
    const params = new URLSearchParams(window.location.search);
    const nextGameweek = chooseGameweek(selected, gameweek);
    params.set('season', String(id));
    if (nextGameweek) params.set('gameweek', String(nextGameweek));
    else params.delete('gameweek');
    window.history.pushState({}, '', `${window.location.pathname}?${params.toString()}${window.location.hash}`);
    window.localStorage.setItem(STORAGE_KEY, String(id));
    setSeasonId(id);
    setGameweek(nextGameweek);
    setUnknownSeason(undefined);
    setNotice('');
  };

  const selectGameweek = (id: number) => {
    const selected = seasons.find((item) => item.id === seasonId);
    if (!selected?.availableGameweeks.some((item) => item.id === id)) return;
    const params = new URLSearchParams(window.location.search);
    params.set('gameweek', String(id));
    window.history.pushState({}, '', `${window.location.pathname}?${params.toString()}${window.location.hash}`);
    setGameweek(id);
  };

  const value = useMemo<SeasonContextValue>(() => ({ seasons, season: seasons.find((item) => item.id === seasonId), seasonId, gameweek, loading, error, notice, unknownSeason, selectSeason, selectGameweek, retry: () => setReload((value) => value + 1) }), [seasons, seasonId, gameweek, loading, error, notice, unknownSeason]);
  return <SeasonContext.Provider value={value}>{children}</SeasonContext.Provider>;
}

export function useSeason() {
  const value = useContext(SeasonContext);
  if (!value) throw new Error('useSeason must be used inside SeasonProvider');
  return value;
}
