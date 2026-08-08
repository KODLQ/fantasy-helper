export type Player = {
  id: number;
  firstName: string;
  secondName: string;
  webName: string;
  position: number;
  teamId: number;
  price: number;
  totalPoints: number;
  form: number;
  minutes: number;
  value: number;
  status: string;
  news: string;
  expectedMinutes: number;
  recentReturns: number;
  goalsScored: number;
  assists: number;
  cleanSheets: number;
  saves: number;
};

export type Freshness = { status: string; lastSuccessfulSync?: string; warning?: string; snapshotAt?: string };
export type ValidationError = { code: string; rule: string; message: string; current?: unknown; required?: unknown; playerId?: number };
export type Squad = { name: string; budget: number; players: Player[]; purchasePrices: Record<number, number>; startingPlayerIds: number[]; benchPlayerIds: number[]; captainId: number; viceCaptainId: number; formation: string; totalCost: number; remainingBudget: number; validation: ValidationError[] };
export type RecommendationPlayer = { player: Player; score: number; factors: { name: string; signal: number; weight: number; contribution: number }[]; fixture: string; explanation: string };
export type Recommendation = { algorithmVersion: string; weights: Record<string, number>; startingXI: RecommendationPlayer[]; bench: RecommendationPlayer[]; captain: RecommendationPlayer; viceCaptain: RecommendationPlayer; heuristicNotice: string; snapshotAt: string };

const baseURL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${baseURL}${path}`, { headers: { 'Content-Type': 'application/json' }, ...options });
  const body = await response.json();
  if (!response.ok) throw new Error(body?.error?.message ?? 'Request failed');
  return body as T;
}

export const api = {
  players: (params: URLSearchParams) => request<{ items: Player[]; total: number; page: number; pageSize: number; freshness: Freshness }>(`/api/v1/players?${params}`),
  player: (id: number) => request<{ player: Player; team: { name: string; shortName: string }; history: { gameweek: number; totalPoints: number; minutes: number }[]; fixtures: { homeTeam: number; awayTeam: number; homeDifficulty: number; awayDifficulty: number }[]; freshness: Freshness }>(`/api/v1/players/${id}`),
  compare: (ids: number[]) => request<{ items: { player: Player; team: { shortName: string }; history: unknown[] }[]; freshness: Freshness }>(`/api/v1/players/compare?ids=${ids.join(',')}`),
  squad: () => request<Squad>('/api/v1/squad'),
  saveSquad: (squad: Partial<Squad>) => request<Squad>('/api/v1/squad', { method: 'PUT', body: JSON.stringify(squad) }),
  recommend: (weights?: Record<string, number>) => request<{ recommendation: Recommendation; freshness: Freshness }>('/api/v1/recommendations', { method: 'POST', body: JSON.stringify({ weights }) }),
  sync: () => request('/api/v1/sync', { method: 'POST' }),
  syncStatus: () => request<{ status: string; warning?: string; freshness: Freshness }>('/api/v1/sync/status'),
};

export const positionName = (position: number) => ({ 1: 'GK', 2: 'DEF', 3: 'MID', 4: 'FWD' }[position] ?? '—');
export const fullPositionName = (position: number) => ({ 1: 'Goalkeeper', 2: 'Defender', 3: 'Midfielder', 4: 'Forward' }[position] ?? 'Unknown');

