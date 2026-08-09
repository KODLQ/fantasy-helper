import { useEffect, useMemo, useState } from 'react';
import { api, Player, PlayerResearchItem, positionName, Team } from '../api';
import { DataTable, DataTableColumn, DataTableFilters, DataTablePagination, DataTableSort } from '../components/data-table';
import { normalizeFilters, totalPages } from '../data-table-state.mjs';
import '../interactive.css';

export function Research({ seasonId, onSelect, onCompare, onSquad }: { seasonId: number; onSelect: (id: number) => void; onCompare: (id: number) => void; onSquad: () => void }) {
  const [filters, setFilters] = useState<DataTableFilters>({});
  const [sort, setSort] = useState<DataTableSort>({ key: 'form', direction: 'desc' });
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [viewMode, setViewMode] = useState<'table' | 'cards'>('table');
  const [items, setItems] = useState<PlayerResearchItem[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => { setPage(1); }, [seasonId]);
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      const active = normalizeFilters(filters) as DataTableFilters;
      const params = new URLSearchParams({ seasonId: String(seasonId), page: String(page), pageSize: String(pageSize), sort: sort.key, direction: sort.direction });
      Object.entries(active).forEach(([key, value]) => { if (value) params.set(key, value); });
      const cacheKey = `fantasy-helper:players:v2:${seasonId}:${params.toString()}`;
      const cached = window.localStorage.getItem(cacheKey);
      if (cached) {
        const saved = JSON.parse(cached) as { items: PlayerResearchItem[]; teams: Team[]; total: number };
        setItems(saved.items); setTeams(saved.teams); setTotal(saved.total); setLoading(false);
      }
      setLoading(true);
      api.players(params, controller.signal).then((result) => {
        setItems(result.items); setTeams(result.teams); setTotal(result.total); setError('');
        window.localStorage.setItem(cacheKey, JSON.stringify({ items: result.items, teams: result.teams, total: result.total }));
        const lastPage = totalPages(result.total, result.pageSize);
        if (page > lastPage) setPage(lastPage);
      }).catch((reason) => { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : 'Could not load players.'); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    }, 240);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [seasonId, filters, sort, page, pageSize]);

  const changeFilter = (id: string, value: string) => { setFilters((current) => ({ ...current, [id]: value })); setPage(1); };
  const changeSort = (next: DataTableSort) => { setSort(next); setPage(1); };
  const clearFilters = () => { setFilters({}); setPage(1); };
  const changePageSize = (size: number) => { setPageSize(size); setPage(1); };
  const playerActions = (player: Player) => <div className="row-actions"><button title="Compare" aria-label="Compare player" onClick={() => onCompare(player.id)}>＋</button><button title="Inspect" aria-label="Inspect player" onClick={() => onSelect(player.id)}>→</button></div>;
  const columns = useMemo<DataTableColumn<PlayerResearchItem>[]>(() => [
    { id: 'player', header: 'Player', sortKey: 'name', filter: { type: 'text', id: 'search', label: 'Filter players', placeholder: 'Name…' }, cell: ({ player }, index) => <button className="player-cell" onClick={() => onSelect(player.id)}><span className={`player-avatar avatar-${index % 5}`}>{player.webName.slice(0, 2).toUpperCase()}</span><span><strong>{player.webName}</strong><small>{player.firstName} {player.secondName}</small></span></button> },
    { id: 'club', header: 'Club', filter: { type: 'select', id: 'teamId', label: 'Filter by club', options: teams.map((team) => ({ value: String(team.id), label: team.name })) }, cell: ({ team }) => <span title={team.name}>{team.shortName}</span> },
    { id: 'position', header: 'Pos', filter: { type: 'select', id: 'position', label: 'Filter by position', options: [{ value: '1', label: 'GK' }, { value: '2', label: 'DEF' }, { value: '3', label: 'MID' }, { value: '4', label: 'FWD' }] }, cell: ({ player }) => <span className="position-tag">{positionName(player.position)}</span> },
    { id: 'price', header: 'Price', sortKey: 'price', defaultSortDirection: 'desc', filter: { type: 'number-range', minId: 'minPrice', maxId: 'maxPrice', label: 'Price', min: 0, step: .1 }, cell: ({ player }) => <span className="mono">£{player.price.toFixed(1)}</span> },
    { id: 'form', header: 'Form', sortKey: 'form', defaultSortDirection: 'desc', filter: { type: 'number', id: 'minForm', label: 'Minimum form', min: 0, step: .1 }, cell: ({ player }) => <strong className="metric-green">{player.form.toFixed(1)}</strong> },
    { id: 'points', header: 'Pts', sortKey: 'points', defaultSortDirection: 'desc', filter: { type: 'number', id: 'minPoints', label: 'Minimum points', min: 0, step: 1 }, className: 'mono', cell: ({ player }) => player.totalPoints },
    { id: 'minutes', header: 'Min', sortKey: 'minutes', defaultSortDirection: 'desc', filter: { type: 'number', id: 'minMinutes', label: 'Minimum minutes', min: 0, step: 1 }, className: 'mono', cell: ({ player }) => player.minutes },
    { id: 'value', header: 'Value', sortKey: 'value', defaultSortDirection: 'desc', filter: { type: 'number', id: 'minValue', label: 'Minimum value', min: 0, step: .1 }, cell: ({ player }) => <span className="value-pill">{player.value.toFixed(1)}</span> },
    { id: 'availability', header: 'Availability', filter: { type: 'select', id: 'status', label: 'Filter by availability', options: [{ value: 'a', label: 'Available' }, { value: 'd', label: 'Doubtful' }, { value: 'i', label: 'Injured' }, { value: 's', label: 'Suspended' }, { value: 'u', label: 'Unavailable' }] }, cell: ({ player }) => <span className="availability"><i />{availabilityName(player.status)}</span> },
    { id: 'actions', header: <span className="visually-hidden">Actions</span>, cell: ({ player }) => playerActions(player) },
  ], [onCompare, onSelect, teams]);

  return <section className="page-section"><div className="hero-grid"><div className="hero-copy"><span className="eyebrow accent">RESEARCH DESK</span><h2>Find the edge<br /><em>before deadline.</em></h2><p>Build conviction from form, minutes, fixtures, and value. Every signal stays visible so your decisions remain yours.</p><button className="primary-button" onClick={onSquad}>Open squad planner <span>→</span></button></div><div className="hero-card"><div className="hero-card-top"><span>Decision lens</span><span className="live-pill"><i /> live snapshot</span></div><div className="lens-score">8.4 <span>/ 10</span></div><div className="lens-label">Research readiness</div><div className="signal-bars"><Signal label="Form" value={82} color="green" /><Signal label="Minutes" value={94} color="blue" /><Signal label="Fixtures" value={68} color="orange" /></div><div className="hero-card-footer">{total} players in this research view <span>↗</span></div></div></div>
    <div className="section-heading"><div><span className="eyebrow">THE PLAYER POOL</span><h3>Research all players <small>{total} tracked</small></h3></div><div className="view-toggle"><button className={viewMode === 'table' ? 'selected' : ''} onClick={() => setViewMode('table')}>▤ Table</button><button className={viewMode === 'cards' ? 'selected' : ''} onClick={() => setViewMode('cards')}>▦ Cards</button></div></div>
    {viewMode === 'table' ? <DataTable caption="Player research results" columns={columns} rows={items} rowKey={({ player }) => player.id} sort={sort} filters={filters} loading={loading} error={error} loadingMessage="Loading research universe…" emptyMessage="No players match those filters." page={page} pageSize={pageSize} total={total} onSortChange={changeSort} onFilterChange={changeFilter} onClearFilters={clearFilters} onPageChange={setPage} onPageSizeChange={changePageSize} testId="player-table" /> : <><div className="player-cards" data-testid="player-cards">{loading ? <div className="table-state">Loading research universe…</div> : error ? <div className="table-state error-state">{error}</div> : items.length === 0 ? <div className="table-state">No players match those filters.</div> : items.map(({ player, team }, index) => <article className="player-card" key={player.id}><button className="player-cell" onClick={() => onSelect(player.id)}><span className={`large-avatar avatar-${index % 5}`}>{player.webName.slice(0, 2).toUpperCase()}</span><span><strong>{player.webName}</strong><small>{team.shortName} · {positionName(player.position)} · £{player.price.toFixed(1)}</small></span></button><div className="card-metrics"><span>Form <strong>{player.form.toFixed(1)}</strong></span><span>Points <strong>{player.totalPoints}</strong></span><span>Value <strong>{player.value.toFixed(1)}</strong></span></div>{playerActions(player)}</article>)}</div><DataTablePagination page={page} pageSize={pageSize} total={total} returned={items.length} onPageChange={setPage} onPageSizeChange={changePageSize} /></>}
  </section>;
}

const availabilityName = (status: string) => ({ a: 'Available', d: 'Doubtful', i: 'Injured', s: 'Suspended', u: 'Unavailable' }[status] ?? 'Unknown');
function Signal({ label, value, color }: { label: string; value: number; color: string }) { return <div className="signal-row"><span>{label}</span><div className="signal-track"><i className={color} style={{ width: `${value}%` }} /></div><strong>{value}</strong></div>; }
