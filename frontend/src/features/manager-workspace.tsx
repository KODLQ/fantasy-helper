import { FormEvent, useEffect, useState } from 'react';
import { api, ImportPreview, LeagueComparison, LeagueStandings, ManagerStatus } from '../api';

export function ManagerWorkspace({ seasonId, gameweek }: { seasonId: number; gameweek: number }) {
  const [status, setStatus] = useState<ManagerStatus | null>(null);
  const [entryId, setEntryId] = useState('');
  const [leagueId, setLeagueId] = useState('');
  const [session, setSession] = useState('');
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [standings, setStandings] = useState<LeagueStandings | null>(null);
  const [selected, setSelected] = useState<number[]>([]);
  const [comparison, setComparison] = useState<LeagueComparison | null>(null);
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  const load = () => Promise.all([api.managerScopes(), api.managerStatus()]).then(([scopeResult, statusResult]) => {
    setStatus(statusResult);
    const entry = scopeResult.items.find((item) => item.type === 'entry');
    const league = scopeResult.items.find((item) => item.type === 'league');
    if (entry) setEntryId(String(entry.sourceId));
    if (league) setLeagueId(String(league.sourceId));
  }).catch((error) => setMessage(error instanceof Error ? error.message : 'Manager data is unavailable.'));

  useEffect(() => { void load(); }, []);
  const run = async (action: () => Promise<void>) => { setBusy(true); setMessage(''); try { await action(); } catch (error) { setMessage(error instanceof Error ? error.message : 'Manager request failed.'); } finally { setBusy(false); } };
  const saveScope = (event: FormEvent, type: 'entry' | 'league') => { event.preventDefault(); const sourceId = Number(type === 'entry' ? entryId : leagueId); void run(async () => { await api.saveManagerScope({ type, sourceId, enabled: true, memberLimit: 50 }); await load(); setMessage(`${type === 'entry' ? 'Entry' : 'League'} scope saved.`); }); };

  return <section className="manager-workspace" data-testid="manager-workspace">
    <div className="section-heading"><div><span className="eyebrow accent">PRIVATE FPL WORKSPACE</span><h2>Manager & league sync</h2><p>Read-only research. Fantasy Helper never changes your FPL account.</p></div><span className={`status-pill ${status?.status ?? 'idle'}`}>{status?.status ?? 'idle'}</span></div>
    {message && <div role="status" className="scope-notice">{message}</div>}
    <div className="manager-grid">
      <article className="auth-panel"><h3>Manager entry</h3><form onSubmit={(event) => saveScope(event, 'entry')}><label>FPL entry ID<input aria-label="FPL entry ID" inputMode="numeric" value={entryId} onChange={(event) => setEntryId(event.target.value)} required /></label><button className="secondary-button" disabled={busy}>Save entry</button></form><label>Optional FPL session<input aria-label="FPL session" type="password" autoComplete="off" value={session} onChange={(event) => setSession(event.target.value)} placeholder="Used in memory only" /></label><button className="secondary-button" disabled={busy || !entryId || !session} onClick={() => void run(async () => { await api.connectManager(Number(entryId), session); setSession(''); setMessage('FPL session connected.'); })}>Connect session</button><button className="primary-button" disabled={busy || !entryId} onClick={() => void run(async () => { const value = await api.managerSync(seasonId, gameweek); setStatus(value); setMessage(value.warning || 'Manager sync completed.'); })}>Sync manager data</button><button className="text-button" disabled={busy || !entryId} onClick={() => void run(async () => setPreview(await api.importPreview(seasonId, gameweek, Number(entryId))))}>Preview active team</button></article>
      <article className="auth-panel"><h3>Classic league</h3><form onSubmit={(event) => saveScope(event, 'league')}><label>League ID<input aria-label="League ID" inputMode="numeric" value={leagueId} onChange={(event) => setLeagueId(event.target.value)} required /></label><button className="secondary-button" disabled={busy}>Save league</button></form><button className="primary-button" disabled={busy || !leagueId} onClick={() => void run(async () => { const value = await api.leagueStandings(seasonId, gameweek, Number(leagueId)); setStandings(value); setSelected(value.members.slice(0, 4).map((item) => item.entryId)); setComparison(null); })}>Load league teams</button>{standings && <div className="member-list"><strong>{standings.name}</strong>{standings.members.map((member) => <label key={member.entryId}><input type="checkbox" checked={selected.includes(member.entryId)} onChange={() => setSelected((ids) => ids.includes(member.entryId) ? ids.filter((id) => id !== member.entryId) : [...ids, member.entryId].slice(0, 8))} />{member.rank}. {member.entryName}</label>)}<button className="secondary-button" disabled={selected.length < 2} onClick={() => void run(async () => setComparison(await api.leagueComparison(seasonId, gameweek, Number(leagueId), selected, 8)))}>Compare selected teams</button></div>}</article>
    </div>
    {preview && <article className="manager-result" data-testid="import-preview"><h3>Active-team import preview</h3><p>{preview.hasChanges ? 'Review these differences before importing.' : 'Your planning squad already matches this snapshot.'}</p><p>Added: {preview.addedPlayerIds.join(', ') || 'None'} · Removed: {preview.removedPlayerIds.join(', ') || 'None'} · Lineup: {preview.lineupChanged ? 'changed' : 'unchanged'} · Captaincy: {preview.captainChanged ? 'changed' : 'unchanged'}</p>{preview.validation.length > 0 && <div role="alert">Import blocked: {preview.validation.map((item) => item.message).join(' ')}</div>}<p>Import actions remain disabled until the atomic provenance transaction is available.</p></article>}
    {comparison && <article className="manager-result" data-testid="league-comparison"><h3>{comparison.outcomeState} point differences</h3>{comparison.omittedEntryIds.length > 0 && <p role="status">Omitted entries: {comparison.omittedEntryIds.join(', ')}</p>}{comparison.missingInputs.length > 0 && <p role="alert">Missing: {comparison.missingInputs.join(', ')}</p>}{comparison.comparisons.map((item, index) => <div key={`${item.entryId}-${index}`}><strong>Entry {item.entryId}</strong> · {Math.round(item.overlap * 100)}% overlap · {item.pointDifference > 0 ? '+' : ''}{item.pointDifference} points · {item.differentials.length} differentials</div>)}</article>}
  </section>;
}
