import { useEffect, useState } from 'react';
import { api, Freshness } from '../api';

export type FreshnessState = {
  status: string;
  warning?: string;
  freshness?: Freshness;
};

export function useFreshness(): FreshnessState {
  const [state, setState] = useState<FreshnessState>({ status: 'unavailable' });

  useEffect(() => {
    let active = true;
    api.syncStatus()
      .then((result) => {
        if (active) setState(result);
      })
      .catch(() => {
        if (active) setState({ status: 'offline', warning: 'Backend is not reachable.' });
      });
    return () => {
      active = false;
    };
  }, []);

  return state;
}
