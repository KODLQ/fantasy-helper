import { useEffect, useRef, useState } from 'react';
import { api, SyncStatus } from './api';
import { AuthPanel, useAuth } from './auth-context';
import { DataState } from './components/data-state';
import { Compare } from './features/compare';
import { PlayerDrawer } from './features/player-drawer';
import { Recommendations } from './features/recommendations';
import { Research } from './features/research';
import { SquadPlanner } from './features/squad';
import { SyncControl } from './features/sync-control';
import { ManagerWorkspace } from './features/manager-workspace';
import { LeagueInsights } from './features/league-insights';
import { useSeason } from './season-context';
import { seasonStatusLabel } from './season-selection.mjs';
import './interactive.css';
import './sync-control.css';

type View = 'research' | 'compare' | 'squad' | 'recommendations' | 'manager' | 'league-insights';
const demoIDs = [1, 2, 4, 5, 6, 7, 22, 8, 9, 10, 11, 12, 13, 14, 15];
const demoStarting = [1, 4, 5, 6, 8, 9, 10, 11, 13, 14, 15];
const demoBench = [2, 7, 12, 22];

function App() {
  const auth = useAuth();
  const seasonContext = useSeason();
  const { season, seasonId, gameweek } = seasonContext;
  const [view, setView] = useState<View>('research');
  const [freshness, setFreshness] = useState<SyncStatus>({ status: 'empty', freshness: { status: 'unavailable', state: 'unavailable' } });
  const [compareIDs, setCompareIDs] = useState<number[]>([]);
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [squadPlayerID, setSquadPlayerID] = useState<number | null>(null);
  const [notice, setNotice] = useState('');
  const [profileOpen, setProfileOpen] = useState(false);

  useEffect(() => { setCompareIDs([]); setSelectedID(null); setSquadPlayerID(null); }, [seasonId]);

  const freshnessState = freshness.freshness?.state ?? freshness.freshness?.status ?? freshness.status;
  const freshnessTitle = freshness.status === 'offline' ? 'Backend connection unavailable' : freshnessState === 'partial' ? 'Some warehouse inputs are missing' : freshnessState === 'stale' ? 'Warehouse data is stale' : freshnessState === 'unavailable' || freshness.status === 'empty' ? 'Demo snapshot is active' : 'Warehouse data needs attention';

  const addCompare = (id: number) => { if (compareIDs.includes(id)) return; if (compareIDs.length >= 4) { setNotice('Comparison is limited to four players.'); return; } setCompareIDs([...compareIDs, id]); setNotice(''); };
  const removeCompare = (id: number) => setCompareIDs(compareIDs.filter((current) => current !== id));
  const loadDemoSquad = async () => { if (!seasonId) return; if (!auth.user) { setView('squad'); return; } try { await api.saveSquad(seasonId, { name: 'My research squad', budget: 100, purchasePrices: Object.fromEntries(demoIDs.map((id) => [id, 5])), startingPlayerIds: demoStarting, benchPlayerIds: demoBench, captainId: 13, viceCaptainId: 8, formation: '3-4-3' }); setNotice('Demo squad loaded. Open Recommendations to inspect the heuristic.'); setView('squad'); } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not save squad.'); } };

  return <div className="app-shell" data-testid="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark">FH</div><div><strong>Fantasy Helper</strong><span>Research desk</span></div></div>
      <div className="side-label">Workspace</div>
      <nav>{([['research', 'Research', '⌕'], ['compare', `Compare${compareIDs.length ? ` · ${compareIDs.length}` : ''}`, '◫'], ['squad', 'Squad planner', '♙'], ['recommendations', 'Recommendations', '✦'], ['manager', 'Manager & leagues', '◎'], ['league-insights', 'League insights', '≈']] as [View, string, string][]).map(([key, label, icon]) => <button key={key} className={view === key ? 'nav-item active' : 'nav-item'} onClick={() => setView(key)}><span>{icon}</span>{label}</button>)}</nav>
      <div className="sidebar-footer"><SyncControl onNotice={setNotice} onStatus={setFreshness} /></div>
    </aside>
    <main className="main-content">
      <header className="topbar"><div><span className="eyebrow">FPL / {season?.name ?? 'select season'}</span><h1>{view === 'research' ? 'Player research' : view === 'compare' ? 'Compare players' : view === 'squad' ? 'Squad planner' : view === 'manager' ? 'Manager & leagues' : view === 'league-insights' ? 'League insights' : 'Recommendations'}</h1></div><div className="topbar-actions"><SeasonSelector /><div className="week-chip">{gameweek ? `GW ${gameweek}` : 'No gameweek'} <span>·</span> {season?.state === 'historical' ? 'historical' : 'decision window'}</div><ProfileMenu open={profileOpen} onOpenChange={setProfileOpen} /></div></header>
      {freshnessState !== 'actual' && freshnessState !== 'fresh' && <div className="freshness-banner" data-testid="freshness-banner"><span className="banner-icon">!</span><div><strong>{freshnessTitle}</strong><span>{freshness.warning ?? freshness.freshness?.warning ?? 'Sync official data to replace the sample research snapshot with the latest FPL data.'}</span></div></div>}
      {notice && <div className="toast" onClick={() => setNotice('')}>{notice}<span>×</span></div>}
      {seasonContext.notice && <div className="scope-notice" role="status">{seasonContext.notice}</div>}
      {seasonContext.loading ? <DataState status="loading" message="Loading available FPL seasons…" /> : seasonContext.error ? <div className="empty-panel season-state" role="alert"><h3>Could not load seasons</h3><p>{seasonContext.error}</p><button className="primary-button" onClick={seasonContext.retry}>Try again</button></div> : seasonContext.unknownSeason ? <div className="empty-panel season-state" data-testid="season-not-found"><h3>Season {seasonContext.unknownSeason} is not available</h3><p>Choose one of the imported seasons. The requested URL was not silently redirected.</p>{seasonContext.seasons.map((item) => <button className="secondary-button" key={item.id} onClick={() => seasonContext.selectSeason(item.id)}>{item.name}</button>)}</div> : !seasonId ? <div className="empty-panel season-state"><h3>No season data available</h3><p>Sync the current official season or import a historical archive to begin.</p></div> : season?.missingInputs.includes('catalogue') ? <div className="empty-panel season-state" data-testid="season-data-unavailable" role="alert"><h3>{season.name} data is unavailable</h3><p>This season is known, but its queryable catalogue has not been imported. Choose another season or import its archive.</p></div> : <>
        {view === 'research' && <Research seasonId={seasonId} onSelect={setSelectedID} onCompare={addCompare} onSquad={() => setView('squad')} />}
        {view === 'compare' && <Compare seasonId={seasonId} ids={compareIDs} onRemove={removeCompare} onBack={() => setView('research')} />}
        {view === 'squad' && (auth.user ? <SquadPlanner seasonId={seasonId} onRecommend={() => setView('recommendations')} onLoadDemo={loadDemoSquad} initialPlayerId={squadPlayerID} onInitialPlayerHandled={() => setSquadPlayerID(null)} /> : <ProtectedWorkspace title="Sign in to plan your squad" onSignIn={() => setProfileOpen(true)} />)}
        {view === 'recommendations' && (auth.user ? <Recommendations seasonId={seasonId} /> : <ProtectedWorkspace title="Sign in to save private recommendations" onSignIn={() => setProfileOpen(true)} />)}
        {view === 'manager' && (auth.user ? <ManagerWorkspace seasonId={seasonId} gameweek={gameweek ?? 1} /> : <ProtectedWorkspace title="Sign in to sync your FPL manager and leagues" onSignIn={() => setProfileOpen(true)} />)}
        {view === 'league-insights' && (auth.user ? <LeagueInsights seasonId={seasonId} gameweek={gameweek ?? 1} /> : <ProtectedWorkspace title="Sign in to research your league" onSignIn={() => setProfileOpen(true)} />)}
        {selectedID && <PlayerDrawer seasonId={seasonId} id={selectedID} onClose={() => setSelectedID(null)} onCompare={addCompare} onAddToSquad={() => { setSquadPlayerID(selectedID); setSelectedID(null); setView('squad'); setNotice('Squad planner opened.'); }} />}
      </>}
    </main>
  </div>;
}

function ProfileMenu({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const auth = useAuth();
  const container = useRef<HTMLDivElement>(null);
  const label = auth.user ? 'Open profile menu' : 'Sign in or create account';

  useEffect(() => {
    if (!open) return;
    const closeOutside = (event: MouseEvent) => { if (!container.current?.contains(event.target as Node)) onOpenChange(false); };
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onOpenChange(false); };
    document.addEventListener('mousedown', closeOutside);
    document.addEventListener('keydown', closeOnEscape);
    return () => { document.removeEventListener('mousedown', closeOutside); document.removeEventListener('keydown', closeOnEscape); };
  }, [open, onOpenChange]);

  return <div className="profile-menu" ref={container}>
    <button className="avatar" aria-label={label} aria-haspopup="dialog" aria-expanded={open} onClick={() => onOpenChange(!open)}>{auth.user ? (auth.user.displayName || auth.user.email).slice(0, 2).toUpperCase() : '◯'}</button>
    {open && <div className="profile-popover" role="dialog" aria-label="Profile and authentication" data-testid="profile-menu"><button className="profile-close" aria-label="Close profile menu" onClick={() => onOpenChange(false)}>×</button><AuthPanel /></div>}
  </div>;
}

function ProtectedWorkspace({ title, onSignIn }: { title: string; onSignIn: () => void }) {
  return <section className="protected-workspace" data-testid="protected-workspace"><div><span className="eyebrow accent">PRIVATE WORKSPACE</span><h2>{title}</h2><p>Public player and fixture research stays available without an account. Your squads and future manager imports are isolated by local user.</p><button className="primary-button" onClick={onSignIn}>Open sign in</button></div></section>;
}

function SeasonSelector() {
  const { seasons, season, gameweek, loading, error, selectSeason, selectGameweek } = useSeason();
  return <div className="season-controls">
    <label>Season<select aria-label="Season" data-testid="season-selector" value={season?.id ?? ''} disabled={loading || Boolean(error) || seasons.length === 0} onChange={(event) => selectSeason(Number(event.target.value))}>{!season && <option value="">Select season</option>}{seasons.map((item) => <option value={item.id} key={item.id}>{item.name}{item.state === 'historical' ? ' · Historical' : ''}{item.freshness.state === 'partial' ? ' · Partial' : ''}</option>)}</select></label>
    <label>Gameweek<select aria-label="Gameweek" value={gameweek ?? ''} disabled={!season || season.availableGameweeks.length === 0} onChange={(event) => selectGameweek(Number(event.target.value))}>{season?.availableGameweeks.map((item) => <option value={item.id} key={item.id}>GW {item.id}</option>)}</select></label>
    {season && <span className="season-status" role="status">{seasonStatusLabel(season)}</span>}
  </div>;
}

export default App;
