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

export type Team = {
  id: number;
  name: string;
  shortName: string;
  strength?: number;
  strengthOverallHome?: number;
  strengthOverallAway?: number;
  strengthAttackHome?: number;
  strengthAttackAway?: number;
  strengthDefenceHome?: number;
  strengthDefenceAway?: number;
};

export type PlayerResearchItem = { player: Player; team: Team };
export type Fixture = { homeTeam: number; awayTeam: number; homeDifficulty: number; awayDifficulty: number };

export type Freshness = { status: string; state?: string; lastSuccessfulSync?: string; warning?: string; warnings?: string[]; snapshotAt?: string };
export type Gameweek = { id: number; name: string; finished: boolean; isCurrent: boolean };
export type Season = {
  id: number;
  name: string;
  state: 'current' | 'historical';
  availableGameweeks: Gameweek[];
  defaultGameweek?: number;
  sourceKind: 'official-current' | 'retained-snapshot' | 'historical-archive';
  lastImportedAt?: string;
  freshness: Freshness;
  completeness: Record<string, unknown>;
  missingInputs: string[];
  warnings: string[];
};
export type SyncStatus = { status: string; runId?: number; currentStage?: string; completedStages?: string[]; failedStages?: string[]; completedWork?: number; totalWork?: number; warning?: string; freshness: Freshness };
export type AuthUser = { id: number; email: string; displayName: string; status: string; createdAt: string; updatedAt: string; lastLoginAt?: string };
export type AuthSession = { user: AuthUser; csrfToken: string; expiresAt: string };
export type AuthConfig = { registrationEnabled: boolean; emailProviderConfigured: boolean; minimumPasswordLength: number };
export type ManagerScope = { id?: number; type: 'entry' | 'league'; sourceId: number; enabled: boolean; memberLimit: number };
export type ManagerStatus = { status: string; runId?: number; completedWork: number; failedWork: number; warning?: string; freshness: Freshness };
export type ManagerPick = { playerId: number; position: number; multiplier: number; captain: boolean; viceCaptain: boolean };
export type ImportPreview = { snapshot: { snapshotId: number; entryId: number; gameweek: number; state: string; conflictState: string; picks: ManagerPick[] }; proposed: Squad; addedPlayerIds: number[]; removedPlayerIds: number[]; lineupChanged: boolean; captainChanged: boolean; validation: ValidationError[]; hasChanges: boolean };
export type LeagueStandings = { leagueId: number; name: string; page: number; hasNext: boolean; members: { entryId: number; entryName: string; playerName: string; rank: number; lastRank: number; points: number }[] };
export type LeagueComparison = { leagueId: number; seasonId: number; gameweek: number; selectedEntryIds: number[]; omittedEntryIds: number[]; comparisons: { entryId: number; sharedPlayers: number[]; differentials: number[]; overlap: number; netPoints: number; pointDifference: number; outcomeState: string }[]; outcomeState: string; algorithmVersion?: string; missingInputs: string[] };
export type ValidationError = { code: string; rule: string; message: string; current?: unknown; required?: unknown; playerId?: number };
export type Squad = { name: string; budget: number; players: Player[]; purchasePrices: Record<number, number>; startingPlayerIds: number[]; benchPlayerIds: number[]; captainId: number; viceCaptainId: number; formation: string; totalCost: number; remainingBudget: number; validation: ValidationError[] };
export type RecommendationPlayer = { player: Player; score: number; factors: { name: string; signal: number; weight: number; contribution: number }[]; fixture: string; explanation: string };
export type Recommendation = { algorithmVersion: string; weights: Record<string, number>; startingXI: RecommendationPlayer[]; bench: RecommendationPlayer[]; captain: RecommendationPlayer; viceCaptain: RecommendationPlayer; heuristicNotice: string; snapshotAt: string };

export type RequestMeta = { requestId?: string; freshness?: Freshness; scope?: { seasonId?: number; gameweek?: number; dataset?: string }; warnings?: string[]; [key: string]: unknown };
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

export type RequestTelemetry = {
  operation: string;
  requestId: string;
  durationMs: number;
  outcome: 'success' | 'failure' | 'cancelled' | 'stale';
  cancellationClass?: 'timeout' | 'caller';
};

export type RequestMetrics = {
  requests: number;
  succeeded: number;
  failed: number;
  cancelled: number;
  stale: number;
};

let telemetryListener: ((event: RequestTelemetry) => void) | undefined;
let authenticationFailureListener: (() => void) | undefined;
let csrfToken = '';
const requestMetrics: RequestMetrics = { requests: 0, succeeded: 0, failed: 0, cancelled: 0, stale: 0 };

export function setRequestTelemetryListener(listener?: (event: RequestTelemetry) => void) {
  telemetryListener = listener;
}

export function getRequestMetrics(): RequestMetrics {
  return { ...requestMetrics };
}

export function setAuthenticationFailureListener(listener?: () => void) {
  authenticationFailureListener = listener;
}

export function clearAuthenticationState() {
  csrfToken = '';
}

function recordTelemetry(event: RequestTelemetry) {
  requestMetrics.requests += 1;
  if (event.outcome === 'success') requestMetrics.succeeded += 1;
  if (event.outcome === 'failure') requestMetrics.failed += 1;
  if (event.outcome === 'cancelled') requestMetrics.cancelled += 1;
  if (event.outcome === 'stale') requestMetrics.stale += 1;
  telemetryListener?.(event);
  if (import.meta.env.DEV) console.debug('[fantasy-helper request]', event);
}

type RequestOptions = RequestInit & { timeoutMs?: number; operation?: string; staleKey?: string; expectedSeasonId?: number };
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
  const { timeoutMs = 15000, operation = path, staleKey, expectedSeasonId, signal: externalSignal, ...init } = options;
  const sequence = staleKey ? (latestRequests.get(staleKey) ?? 0) + 1 : 0;
  if (staleKey) latestRequests.set(staleKey, sequence);
  const controller = new AbortController();
  const abortFromCaller = () => controller.abort(externalSignal?.reason);
  externalSignal?.addEventListener('abort', abortFromCaller, { once: true });
  const timeout = window.setTimeout(() => controller.abort(new DOMException('Request timed out.', 'TimeoutError')), timeoutMs);
  const requestId: string = globalThis.crypto?.randomUUID?.() ?? `web-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const startedAt = performance.now();
  let responseRequestId = requestId;
  let outcome: RequestTelemetry['outcome'] = 'success';
  let cancellationClass: RequestTelemetry['cancellationClass'];
  try {
    const response = await fetch(`${baseURL}${path}`, {
      ...init,
      signal: controller.signal,
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId, ...(csrfToken && init.method && init.method !== 'GET' ? { 'X-CSRF-Token': csrfToken } : {}), ...(init.headers ?? {}) },
    });
    responseRequestId = response.headers.get('x-request-id') ?? requestId;
    const body = await parseBody(response);
    if (!response.ok) {
      const error = body && typeof body === 'object' && 'error' in body ? (body as { error?: { code?: string; message?: string; details?: unknown } }).error : undefined;
      if (response.status === 401 && !path.endsWith('/auth/login') && !path.endsWith('/auth/me')) authenticationFailureListener?.();
      throw new ApiError(error?.message ?? `Request failed with status ${response.status}.`, response.status, error?.code, response.headers.get('x-request-id') ?? requestId, error?.details);
    }
    if (staleKey && latestRequests.get(staleKey) !== sequence) {
      outcome = 'stale';
      throw new StaleResponseError(operation);
    }
    if (expectedSeasonId && isEnvelope<T>(body) && body.meta?.scope?.seasonId !== expectedSeasonId) {
      outcome = 'stale';
      throw new StaleResponseError(operation);
    }
    return (isEnvelope<T>(body) ? body.data : body) as T;
  } catch (reason) {
    if (reason instanceof DOMException && (reason.name === 'AbortError' || reason.name === 'TimeoutError')) {
      outcome = 'cancelled';
      cancellationClass = reason.name === 'TimeoutError' ? 'timeout' : 'caller';
      throw new ApiError('The request was cancelled or timed out.', 0, 'request_cancelled', requestId);
    }
    if (reason instanceof StaleResponseError) throw reason;
    outcome = 'failure';
    throw reason;
  } finally {
    window.clearTimeout(timeout);
    externalSignal?.removeEventListener('abort', abortFromCaller);
    recordTelemetry({ operation, requestId: responseRequestId, durationMs: Math.round(performance.now() - startedAt), outcome, cancellationClass });
  }
}

export const api = {
  authConfig: () => request<AuthConfig>('/api/v1/auth/config', { operation: 'auth-config' }),
  me: async () => { const result = await request<AuthSession>('/api/v1/auth/me', { operation: 'auth-me' }); csrfToken = result.csrfToken; return result; },
  register: async (email: string, password: string, displayName: string) => { const result = await request<AuthSession>('/api/v1/auth/register', { method: 'POST', body: JSON.stringify({ email, password, displayName }), operation: 'auth-register' }); csrfToken = result.csrfToken; return result; },
  login: async (email: string, password: string) => { const result = await request<AuthSession>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email, password }), operation: 'auth-login' }); csrfToken = result.csrfToken; return result; },
  logout: async () => { try { await request<{ loggedOut: boolean }>('/api/v1/auth/logout', { method: 'POST', operation: 'auth-logout' }); } finally { clearAuthenticationState(); } },
  changePassword: async (currentPassword: string, newPassword: string) => { const result = await request<AuthSession>('/api/v1/auth/password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword }), operation: 'auth-password' }); csrfToken = result.csrfToken; return result; },
  seasons: (signal?: AbortSignal) => request<{ items: Season[] }>('/api/v1/seasons', { signal, operation: 'seasons', staleKey: 'seasons' }),
  players: (params: URLSearchParams, signal?: AbortSignal) => { const seasonId = Number(params.get('seasonId')); return request<{ items: PlayerResearchItem[]; teams: Team[]; total: number; page: number; pageSize: number; freshness: Freshness }>(`/api/v1/players?${params}`, { signal, operation: 'players', staleKey: `players:${params.toString()}`, expectedSeasonId: seasonId }); },
  player: (seasonId: number, id: number, signal?: AbortSignal) => request<{ player: Player; team: Team; history: { gameweek: number; totalPoints: number; minutes: number }[]; fixtures: Fixture[]; freshness: Freshness }>(`/api/v1/players/${id}?seasonId=${seasonId}`, { signal, operation: 'player', staleKey: `player:${seasonId}:${id}`, expectedSeasonId: seasonId }),
  compare: (seasonId: number, ids: number[], signal?: AbortSignal) => request<{ items: { player: Player; team: Team; history: unknown[]; fixtures: Fixture[] }[]; freshness: Freshness }>(`/api/v1/players/compare?ids=${ids.join(',')}&seasonId=${seasonId}`, { signal, operation: 'compare', staleKey: `compare:${seasonId}`, expectedSeasonId: seasonId }),
  squad: (seasonId: number, signal?: AbortSignal) => request<Squad>(`/api/v1/squad?seasonId=${seasonId}`, { signal, operation: 'squad', staleKey: `squad:${seasonId}`, expectedSeasonId: seasonId }),
  saveSquad: (seasonId: number, squad: Partial<Squad>) => request<Squad>(`/api/v1/squad?seasonId=${seasonId}`, { method: 'PUT', body: JSON.stringify(squad), operation: 'save-squad', expectedSeasonId: seasonId }),
  recommend: (seasonId: number, weights?: Record<string, number>, signal?: AbortSignal) => request<{ recommendation: Recommendation; freshness: Freshness }>(`/api/v1/recommendations?seasonId=${seasonId}`, { method: 'POST', body: JSON.stringify({ weights }), signal, operation: 'recommendation', staleKey: `recommendation:${seasonId}`, expectedSeasonId: seasonId }),
  sync: () => request<SyncStatus>('/api/v1/sync', { method: 'POST', operation: 'sync' }),
  syncStatus: () => request<SyncStatus>('/api/v1/sync/status', { operation: 'sync-status', staleKey: 'sync-status' }),
  retrySync: (runId: number) => request<SyncStatus>(`/api/v1/sync/runs/${runId}/retry`, { method: 'POST', operation: 'sync-retry' }),
  managerScopes: () => request<{ items: ManagerScope[] }>('/api/v1/manager/scopes', { operation: 'manager-scopes' }),
  saveManagerScope: (scope: ManagerScope) => request<ManagerScope>('/api/v1/manager/scopes', { method: 'PUT', body: JSON.stringify(scope), operation: 'manager-scope-save' }),
  connectManager: (entryId: number, session: string) => request<{ entryId: number; state: string; providerType: string }>('/api/v1/manager/connect', { method: 'POST', body: JSON.stringify({ entryId, session }), operation: 'manager-connect' }),
  managerSync: (seasonId: number, gameweek: number) => request<ManagerStatus>('/api/v1/manager/sync', { method: 'POST', body: JSON.stringify({ seasonId, gameweek }), operation: 'manager-sync' }),
  managerStatus: () => request<ManagerStatus>('/api/v1/manager/status', { operation: 'manager-status' }),
  importPreview: (seasonId: number, gameweek: number, entryId: number) => request<ImportPreview>(`/api/v1/squad/import/preview?seasonId=${seasonId}&gameweek=${gameweek}&entryId=${entryId}`, { operation: 'import-preview' }),
  leagueStandings: (seasonId: number, gameweek: number, leagueId: number, page = 1) => request<LeagueStandings>(`/api/v1/manager/leagues/${leagueId}/standings?seasonId=${seasonId}&gameweek=${gameweek}&page=${page}`, { operation: 'league-standings' }),
  leagueComparison: (seasonId: number, gameweek: number, leagueId: number, entryIds: number[], limit: number) => request<LeagueComparison>(`/api/v1/manager/leagues/${leagueId}/comparison?seasonId=${seasonId}&gameweek=${gameweek}&entryIds=${entryIds.join(',')}&limit=${limit}`, { operation: 'league-comparison' }),
};

export const positionName = (position: number) => ({ 1: 'GK', 2: 'DEF', 3: 'MID', 4: 'FWD' }[position] ?? '—');
export const fullPositionName = (position: number) => ({ 1: 'Goalkeeper', 2: 'Defender', 3: 'Midfielder', 4: 'Forward' }[position] ?? 'Unknown');
export const playerFixtureDifficulty = (teamId: number, fixture?: Fixture) => fixture ? (fixture.homeTeam === teamId ? fixture.homeDifficulty : fixture.awayTeam === teamId ? fixture.awayDifficulty : undefined) : undefined;
