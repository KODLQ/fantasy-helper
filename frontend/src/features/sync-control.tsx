import { useCallback, useEffect, useState } from 'react';
import { api, SyncStatus } from '../api';
import { useAuth } from '../auth-context';

export function SyncControl({ onNotice, onStatus }: { onNotice: (message: string) => void; onStatus: (status: SyncStatus) => void }) {
  const auth = useAuth();
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const update = useCallback((next: SyncStatus) => { setStatus(next); onStatus(next); }, [onStatus]);
  const refresh = useCallback(async () => {
    try { update(await api.syncStatus()); }
    catch { update({ status: 'offline', warning: 'Backend is not reachable.', freshness: { status: 'unavailable', state: 'unavailable' } }); }
  }, [update]);
  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => {
    if (status?.status !== 'running') return;
    const timer = window.setInterval(() => { void refresh(); }, 1000);
    return () => window.clearInterval(timer);
  }, [refresh, status?.status]);
  const start = async () => {
    try { update(await api.sync()); onNotice('Official data sync started.'); }
    catch (error) { onNotice(error instanceof Error ? error.message : 'Could not start sync.'); }
  };
  const retry = async () => {
    if (!status?.runId) return;
    try { update(await api.retrySync(status.runId)); onNotice('Failed sync work was queued for retry.'); }
    catch (error) { onNotice(error instanceof Error ? error.message : 'Could not retry sync.'); }
  };
  const state = status?.freshness?.state ?? status?.freshness?.status ?? status?.status ?? 'unavailable';
  const showDetails = status && ['running', 'partial', 'failed', 'offline'].includes(status.status);
  return <div className="sync-control">
    <div className="mini-status" data-testid="sync-state"><span className={`status-dot ${state === 'actual' || state === 'fresh' || status?.status === 'success' ? 'fresh' : ''}`} />{status?.status === 'running' ? 'Sync running' : state}</div>
    <button className="text-button" disabled={!auth.user || status?.status === 'running'} onClick={start}>{auth.user ? '↻ Sync official data' : 'Sign in to sync data'}</button>
    {showDetails && <div className="sync-progress" data-testid="sync-progress"><strong>{status.status === 'running' ? `Stage: ${status.currentStage ?? 'starting'}` : status.status === 'offline' ? 'Connection error' : `Sync ${status.status}`}</strong>{status.totalWork ? <span>{status.completedWork ?? 0}/{status.totalWork} work items</span> : null}{status.warning && <span>{status.warning}</span>}{(status.status === 'partial' || status.status === 'failed') && status.runId && <button className="text-button" onClick={retry}>Retry failed work</button>}</div>}
  </div>;
}
