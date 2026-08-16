import { FormEvent, useEffect, useMemo, useState } from 'react';
import { api, GameweekAutopsy, LeagueAnalysisComparison, LeagueAnalysisSummary } from '../api';
import { DataTable, DataTableColumn, DataTableFilters, DataTableSort } from '../components/data-table';

type Member = LeagueAnalysisSummary['members'][number];
type ComparisonRow = LeagueAnalysisComparison['comparisons'][number];

export function LeagueInsights({ seasonId, gameweek }: { seasonId: number; gameweek: number }) {
  const [leagueId, setLeagueId] = useState('');
  const [entryId, setEntryId] = useState('');
  const [memberLimit, setMemberLimit] = useState(20);
  const [summary, setSummary] = useState<LeagueAnalysisSummary | null>(null);
  const [selected, setSelected] = useState<number[]>([]);
  const [comparison, setComparison] = useState<LeagueAnalysisComparison | null>(null);
  const [autopsy, setAutopsy] = useState<GameweekAutopsy | null>(null);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [memberPage, setMemberPage] = useState(1);
  const [memberPageSize, setMemberPageSize] = useState(10);
  const [memberSort, setMemberSort] = useState<DataTableSort>({ key: 'rank', direction: 'asc' });
  const [memberFilters, setMemberFilters] = useState<DataTableFilters>({});
  const [comparisonSort, setComparisonSort] = useState<DataTableSort>({ key: 'pointDifference', direction: 'desc' });

  useEffect(() => {
    api.managerScopes().then(({ items }) => {
      const league = items.find((item) => item.type === 'league');
      const entry = items.find((item) => item.type === 'entry');
      if (league) setLeagueId(String(league.sourceId));
      if (entry) setEntryId(String(entry.sourceId));
    }).catch(() => undefined);
  }, []);
  useEffect(() => { setSummary(null); setComparison(null); setAutopsy(null); setSelected([]); }, [seasonId, gameweek]);

  const run = async (name: string, action: () => Promise<void>) => {
    setBusy(name); setError('');
    try { await action(); } catch (reason) { setError(reason instanceof Error ? reason.message : 'Analysis is unavailable.'); }
    finally { setBusy(''); }
  };
  const loadSummary = (event: FormEvent) => {
    event.preventDefault();
    if (!positive(leagueId)) { setError('Enter the numeric classic league ID from the standings URL.'); return; }
    void run('summary', async () => {
      const value = await api.leagueAnalysisSummary(seasonId, gameweek, Number(leagueId), memberLimit);
      setSummary({ ...value, members: value.members ?? [], warnings: value.warnings ?? [], missingInputs: value.missingInputs ?? [] });
      setSelected((value.selectedEntryIds ?? []).filter((id) => value.snapshotIds?.[id]).slice(0, 4));
      setComparison(null); setAutopsy(null); setMemberPage(1);
    });
  };
  const compare = () => void run('comparison', async () => setComparison(await api.leagueAnalysisComparison(seasonId, gameweek, Number(leagueId), selected)));
  const inspectAutopsy = () => {
    if (!positive(entryId)) { setError('Enter your numeric FPL entry ID.'); return; }
    const rival = selected.find((id) => id !== Number(entryId));
    void run('autopsy', async () => setAutopsy(await api.gameweekAutopsy(seasonId, gameweek, Number(entryId), rival)));
  };

  const filteredMembers = useMemo(() => {
    const query = (memberFilters.member ?? '').toLowerCase();
    return [...(summary?.members ?? [])].filter((member) => `${member.entryName} ${member.playerName} ${member.entryId}`.toLowerCase().includes(query)).sort(compareBy(memberSort));
  }, [summary, memberFilters, memberSort]);
  const memberRows = filteredMembers.slice((memberPage - 1) * memberPageSize, memberPage * memberPageSize);
  const comparisonRows = useMemo(() => [...(comparison?.comparisons ?? [])].sort(compareBy(comparisonSort)), [comparison, comparisonSort]);

  const memberColumns: DataTableColumn<Member>[] = [
    { id: 'select', header: 'Select', cell: (member) => <input type="checkbox" aria-label={`Select ${member.entryName}`} checked={selected.includes(member.entryId)} disabled={!summary?.snapshotIds?.[member.entryId] && !selected.includes(member.entryId)} onChange={() => setSelected((ids) => ids.includes(member.entryId) ? ids.filter((id) => id !== member.entryId) : ids.length < 8 ? [...ids, member.entryId] : ids)} /> },
    { id: 'rank', header: 'Rank', sortKey: 'rank', cell: (member) => member.rank },
    { id: 'member', header: 'Team / manager', sortKey: 'entryName', filter: { type: 'text', id: 'member', label: 'Filter league members', placeholder: 'Team, manager, or ID' }, cell: (member) => <><strong>{member.entryName}</strong><br /><small>{member.playerName} · #{member.entryId}</small></> },
    { id: 'points', header: 'Total points', sortKey: 'points', defaultSortDirection: 'desc', cell: (member) => member.points },
    { id: 'snapshot', header: 'Pick snapshot', cell: (member) => summary?.snapshotIds?.[member.entryId] ?? 'Missing' },
  ];
  const comparisonColumns: DataTableColumn<ComparisonRow>[] = [
    { id: 'matchup', header: 'Matchup', sortKey: 'entryId', cell: (row) => `${row.entryId} vs ${row.opponentEntryId}` },
    { id: 'roster', header: 'Roster overlap', sortKey: 'overlap', defaultSortDirection: 'desc', cell: (row) => `${Math.round(row.overlap * 100)}%` },
    { id: 'xi', header: 'XI overlap', sortKey: 'startingXIOverlap', defaultSortDirection: 'desc', cell: (row) => `${Math.round(row.startingXIOverlap * 100)}%` },
    { id: 'differential', header: 'Differential impact', sortKey: 'differentialContribution', defaultSortDirection: 'desc', cell: (row) => signed(row.differentialContribution) },
    { id: 'captain', header: 'Captain bonus', sortKey: 'captainDelta', defaultSortDirection: 'desc', cell: (row) => signed(row.captainDelta) },
    { id: 'net', header: 'Net points', sortKey: 'netPoints', defaultSortDirection: 'desc', cell: (row) => row.netPoints },
    { id: 'gap', header: 'Point gap', sortKey: 'pointDifference', defaultSortDirection: 'desc', cell: (row) => signed(row.pointDifference) },
  ];

  return <section className="manager-workspace league-insights" data-testid="league-insights">
    <div className="section-heading"><div><span className="eyebrow accent">LEAGUE RESEARCH</span><h2>See where every point differs</h2><p>Compare synchronized teams from exactly GW {gameweek} of the selected season.</p></div></div>
    <form className="insight-controls" onSubmit={loadSummary}>
      <label>Classic league ID<input aria-label="Insights league ID" inputMode="numeric" value={leagueId} onChange={(event) => setLeagueId(event.target.value)} required /></label>
      <label>Members to inspect<select aria-label="Member limit" value={memberLimit} onChange={(event) => setMemberLimit(Number(event.target.value))}>{[10, 20, 50, 100].map((value) => <option key={value}>{value}</option>)}</select></label>
      <button className="primary-button" disabled={busy !== ''}>{busy === 'summary' ? 'Loading…' : 'Load league overview'}</button>
    </form>
    {error && <div className="scope-notice" role="alert">{error}</div>}
    {summary && <>
      {summary.name && <h3>{summary.name}</h3>}<AnalysisState state={summary.outcomeState} coverage={summary.coverage} warnings={[...(summary.warnings ?? []), ...(summary.missingInputs ?? [])]} />
      <DataTable caption="League members" columns={memberColumns} rows={memberRows} rowKey={(row) => row.entryId} sort={memberSort} filters={memberFilters} loading={busy === 'summary'} onSortChange={(sort) => { setMemberSort(sort); setMemberPage(1); }} onFilterChange={(id, value) => { setMemberFilters({ ...memberFilters, [id]: value }); setMemberPage(1); }} onClearFilters={() => setMemberFilters({})} page={memberPage} pageSize={memberPageSize} total={filteredMembers.length} pageSizeOptions={[10, 20, 50]} onPageChange={setMemberPage} onPageSizeChange={(size) => { setMemberPageSize(size); setMemberPage(1); }} testId="league-member-table" />
      <div className="button-row"><button className="secondary-button" disabled={selected.length < 2 || busy !== ''} onClick={compare}>Compare {selected.length} selected teams</button><span>{selected.length}/8 rivals selected</span></div>
      <details className="analysis-provenance"><summary>Calculation and snapshot details</summary><p>Formulas: {summary.formulaVersions.join(', ')}</p><p>Selected: {summary.selectedEntryIds.join(', ') || 'None'} · Omitted: {summary.omittedEntryIds.join(', ') || 'None'}</p><p>Snapshots: {snapshotText(summary.snapshotIds)}</p></details>
    </>}
    {comparison && <article className="manager-result" data-testid="insight-comparison"><h3>Team similarities and differences</h3><AnalysisState state={comparison.outcomeState} coverage={comparison.coverage} warnings={[...(comparison.warnings ?? []), ...(comparison.missingInputs ?? [])]} /><DataTable caption="Team comparison" columns={comparisonColumns} rows={comparisonRows} rowKey={(row) => `${row.entryId}-${row.opponentEntryId}`} sort={comparisonSort} filters={{}} onSortChange={setComparisonSort} onFilterChange={() => undefined} onClearFilters={() => undefined} page={1} pageSize={Math.max(1, comparisonRows.length)} total={comparisonRows.length} pageSizeOptions={[Math.max(1, comparisonRows.length)]} onPageChange={() => undefined} onPageSizeChange={() => undefined} testId="insight-comparison-table" /><p>Formulas: {comparison.formulaVersions.join(', ')}</p><p>Snapshots: {snapshotText(comparison.snapshotIds)}</p></article>}
    {summary && <article className="manager-result"><h3>Gameweek autopsy</h3><label>Your FPL entry ID<input aria-label="Autopsy entry ID" inputMode="numeric" value={entryId} onChange={(event) => setEntryId(event.target.value)} /></label><p>The first selected rival other than your entry is used for the point-gap breakdown.</p><button className="secondary-button" disabled={busy !== ''} onClick={inspectAutopsy}>{busy === 'autopsy' ? 'Calculating…' : `Explain GW ${gameweek}`}</button></article>}
    {autopsy && <Autopsy result={autopsy} />}
  </section>;
}

function AnalysisState({ state, coverage, warnings }: { state: string; coverage: LeagueAnalysisSummary['coverage']; warnings: string[] }) {
  return <div className={`analysis-state ${state}`} role="status"><strong>{state}</strong><span>{coverage.complete}/{coverage.selected} selected teams have compatible snapshots.</span>{warnings.length > 0 && <span>{warnings.join(' ')}</span>}</div>;
}

function Autopsy({ result }: { result: GameweekAutopsy }) {
  return <article className="manager-result autopsy" data-testid="gameweek-autopsy"><h3>GW {result.gameweek} autopsy · {result.outcomeState}</h3>{(result.warnings?.length ?? 0) > 0 && <p role="status">{result.warnings.join(' ')}</p>}{result.metricsAvailable ? <div className="autopsy-metrics"><span>Gross<strong>{result.grossPoints}</strong></span><span>Hits<strong>-{result.transferCost}</strong></span><span>Net<strong>{result.netPoints}</strong></span><span>Captain bonus<strong>{signed(result.captainDelta)}</strong></span><span>Bench<strong>{result.benchPoints}</strong></span></div> : <p role="status">Score metrics are unavailable because required gameweek facts are missing.</p>}<p>Captain: {result.captainId ?? 'Unavailable'} · Chip: {result.activeChip || 'None'} · Transfers: {result.transfers?.length ?? 0} · Auto-subs: {result.automaticSubstitutions?.map((item) => `${item.playerOut}→${item.playerIn} (${signed(item.impact)})`).join(', ') || 'None'}</p>{result.outcomeState === 'provisional' && <p>Unfinished fixtures: {result.unfinishedFixtureIds?.join(', ') || 'None reported'}</p>}{result.rivalComparison && <p><strong>Against entry {result.rivalEntryId}:</strong> {signed(result.rivalComparison.pointDifference)} net points, {Math.round(result.rivalComparison.overlap * 100)}% roster overlap, {signed(result.rivalComparison.differentialContribution)} from differentials.</p>}<p>Player contributions: {result.contributions?.map((item) => `${item.playerId}: ${item.effectivePoints}`).join(' · ') || 'Unavailable'}</p><p>Formulas: {result.formulaVersions.join(', ')}</p><p>Snapshots: {snapshotText(result.snapshotIds)}</p></article>;
}

function positive(value: string) { return /^[1-9]\d*$/.test(value); }
function signed(value: number) { return value > 0 ? `+${value}` : String(value); }
function snapshotText(values: Record<number, string>) { return Object.entries(values ?? {}).map(([entry, snapshot]) => `${entry}: ${snapshot}`).join(' · ') || 'None'; }
function compareBy<T extends Record<string, unknown>>(sort: DataTableSort) { return (left: T, right: T) => { const a = left[sort.key]; const b = right[sort.key]; const direction = sort.direction === 'asc' ? 1 : -1; return typeof a === 'number' && typeof b === 'number' ? (a - b) * direction : String(a ?? '').localeCompare(String(b ?? '')) * direction; }; }
