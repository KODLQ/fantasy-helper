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

export type RequestMeta = { requestId?: string; freshness?: Freshness; [key: string]: unknown };
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly requestId?: string;
  readonly details: unknown;

  constructor(message: string, status: number, code = 'request_failed', requestId?: string, details?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.requestId = requestId;
    this.details = details;
  }
}

export class StaleResponseError extends Error {
  constructor(operation: string) {
    super(`Ignored stale response for ${operation}.`);
    this.name = 'StaleResponseError';
  }
}

type RequestOptions = RequestInit & { timeoutMs?: number; operation?: string; staleKey?: string };
type Envelope<T> = { data: T; meta?: RequestMeta };
const baseURL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080';
const latestRequests = new Map<string, number>();

function isEnvelope<T>(body: unknown): body is Envelope<T> {
  return Boolean(body && typeof body === 'object' && 'data' in body && 'meta' in body);
}

async function parseBody(response: Response): Promise<unknown> {
  const contentType = response.headers.get('content-type') ?? '';
  const text = await response.text();
  if (!text) return undefined;
  if (!contentType.includes('json')) throw new ApiError('The server returned an unexpected response.', response.status, 'invalid_response', response.headers.get('x-request-id') ?? undefined);
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new ApiError('The server returned invalid JSON.', response.status, 'invalid_response', response.headers.get('x-request-id') ?? undefined);
  }
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { timeoutMs = 15000, operation = path, staleKey, signal: externalSignal, ...init } = options;
  const sequence = staleKey ? (latestRequests.get(staleKey) ?? 0) + 1 : 0;
  if (staleKey) latestRequests.set(staleKey, sequence);
  const controller = new AbortController();
  const abortFromCaller = () => controller.abort(externalSignal?.reason);
  externalSignal?.addEventListener('abort', abortFromCaller, { once: true });
  const timeout = window.setTimeout(() => controller.abort(new DOMException('Request timed out.', 'TimeoutError')), timeoutMs);
  const requestId = globalThis.crypto?.randomUUID?.() ?? `web-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  try {
    const response = await fetch(`${baseURL}${path}`, {
      ...init,
      signal: controller.signal,
      headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId, ...(init.headers ?? {}) },
    });
    const body = await parseBody(response);
    if (!response.ok) {
      const error = body && typeof body === 'object' && 'error' in body ? (body as { error?: { code?: string; message?: string; details?: unknown } }).error : undefined;
      throw new ApiError(error?.message ?? `Request failed with status ${response.status}.`, response.status, error?.code, response.headers.get('x-request-id') ?? requestId, error?.details);
    }
    if (staleKey && latestRequests.get(staleKey) !== sequence) throw new StaleResponseError(operation);
    return (isEnvelope<T>(body) ? body.data : body) as T;
  } catch (reason) {
    if (reason instanceof DOMException && reason.name === 'AbortError') throw new ApiError('The request was cancelled or timed out.', 0, 'request_cancelled', requestId);
    throw reason;
  } finally {
    window.clearTimeout(timeout);
    externalSignal?.removeEventListener('abort', abortFromCaller);
  }
}

export const api = {
  players: (params: URLSearchParams, signal?: AbortSignal) => request<{ items: Player[]; total: number; page: number; pageSize: number; freshness: Freshness }>(`/api/v1/players?${params}`, { signal, operation: 'players', staleKey: `players:${params.toString()}` }),
  player: (id: number, signal?: AbortSignal) => request<{ player: Player; team: { name: string; shortName: string }; history: { gameweek: number; totalPoints: number; minutes: number }[]; fixtures: { homeTeam: number; awayTeam: number; homeDifficulty: number; awayDifficulty: number }[]; freshness: Freshness }>(`/api/v1/players/${id}`, { signal, operation: 'player', staleKey: `player:${id}` }),
  compare: (ids: number[], signal?: AbortSignal) => request<{ items: { player: Player; team: { shortName: string }; history: unknown[] }[]; freshness: Freshness }>(`/api/v1/players/compare?ids=${ids.join(',')}`, { signal, operation: 'compare', staleKey: 'compare' }),
  squad: (signal?: AbortSignal) => request<Squad>('/api/v1/squad', { signal, operation: 'squad' }),
  saveSquad: (squad: Partial<Squad>) => request<Squad>('/api/v1/squad', { method: 'PUT', body: JSON.stringify(squad), operation: 'save-squad' }),
  recommend: (weights?: Record<string, number>) => request<{ recommendation: Recommendation; freshness: Freshness }>('/api/v1/recommendations', { method: 'POST', body: JSON.stringify({ weights }), operation: 'recommendation' }),
  sync: () => request('/api/v1/sync', { method: 'POST', operation: 'sync' }),
  syncStatus: () => request<{ status: string; warning?: string; freshness: Freshness }>('/api/v1/sync/status', { operation: 'sync-status', staleKey: 'sync-status' }),
};

export const positionName = (position: number) => ({ 1: 'GK', 2: 'DEF', 3: 'MID', 4: 'FWD' }[position] ?? '—');
export const fullPositionName = (position: number) => ({ 1: 'Goalkeeper', 2: 'Defender', 3: 'Midfielder', 4: 'Forward' }[position] ?? 'Unknown');
