import { useState } from 'react';
import { api, SyncStatus } from './api';
import { Compare } from './features/compare';
import { PlayerDrawer } from './features/player-drawer';
import { Recommendations } from './features/recommendations';
import { Research } from './features/research';
import { SquadPlanner } from './features/squad';
import { SyncControl } from './features/sync-control';
import './interactive.css';
import './sync-control.css';

type View = 'research' | 'compare' | 'squad' | 'recommendations';
const demoIDs = [1, 2, 4, 5, 6, 7, 22, 8, 9, 10, 11, 12, 13, 14, 15];
const demoStarting = [1, 4, 5, 6, 8, 9, 10, 11, 13, 14, 15];
const demoBench = [2, 7, 12, 22];

function App() {
  const [view, setView] = useState<View>('research');
  const [freshness, setFreshness] = useState<SyncStatus>({ status: 'empty', freshness: { status: 'unavailable', state: 'unavailable' } });
  const [compareIDs, setCompareIDs] = useState<number[]>([]);
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [notice, setNotice] = useState('');

  const freshnessState = freshness.freshness?.state ?? freshness.freshness?.status ?? freshness.status;
  const freshnessTitle = freshness.status === 'offline' ? 'Backend connection unavailable' : freshnessState === 'partial' ? 'Some warehouse inputs are missing' : freshnessState === 'stale' ? 'Warehouse data is stale' : freshnessState === 'unavailable' || freshness.status === 'empty' ? 'Demo snapshot is active' : 'Warehouse data needs attention';

  const addCompare = (id: number) => { if (compareIDs.includes(id)) return; if (compareIDs.length >= 4) { setNotice('Comparison is limited to four players.'); return; } setCompareIDs([...compareIDs, id]); setNotice(''); };
  const removeCompare = (id: number) => setCompareIDs(compareIDs.filter((current) => current !== id));
  const loadDemoSquad = async () => { try { await api.saveSquad({ name: 'My research squad', budget: 100, purchasePrices: Object.fromEntries(demoIDs.map((id) => [id, 5])), startingPlayerIds: demoStarting, benchPlayerIds: demoBench, captainId: 13, viceCaptainId: 8, formation: '3-4-3' }); setNotice('Demo squad loaded. Open Recommendations to inspect the heuristic.'); setView('squad'); } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not save squad.'); } };

  return <div className="app-shell" data-testid="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark">FH</div><div><strong>Fantasy Helper</strong><span>Research desk</span></div></div>
      <div className="side-label">Workspace</div>
      <nav>{([['research', 'Research', '⌕'], ['compare', `Compare${compareIDs.length ? ` · ${compareIDs.length}` : ''}`, '◫'], ['squad', 'Squad planner', '♙'], ['recommendations', 'Recommendations', '✦']] as [View, string, string][]).map(([key, label, icon]) => <button key={key} className={view === key ? 'nav-item active' : 'nav-item'} onClick={() => setView(key)}><span>{icon}</span>{label}</button>)}</nav>
      <div className="sidebar-footer"><SyncControl onNotice={setNotice} onStatus={setFreshness} /></div>
    </aside>
    <main className="main-content">
      <header className="topbar"><div><span className="eyebrow">FPL / 2025—26</span><h1>{view === 'research' ? 'Player research' : view === 'compare' ? 'Compare players' : view === 'squad' ? 'Squad planner' : 'Recommendations'}</h1></div><div className="topbar-actions"><div className="week-chip">GW 1 <span>·</span> decision window</div><div className="avatar">OS</div></div></header>
      {freshnessState !== 'actual' && freshnessState !== 'fresh' && <div className="freshness-banner" data-testid="freshness-banner"><span className="banner-icon">!</span><div><strong>{freshnessTitle}</strong><span>{freshness.warning ?? freshness.freshness?.warning ?? 'Sync official data to replace the sample research snapshot with the latest FPL data.'}</span></div></div>}
      {notice && <div className="toast" onClick={() => setNotice('')}>{notice}<span>×</span></div>}
      {view === 'research' && <Research onSelect={setSelectedID} onCompare={addCompare} onSquad={loadDemoSquad} />}
      {view === 'compare' && <Compare ids={compareIDs} onRemove={removeCompare} onBack={() => setView('research')} />}
      {view === 'squad' && <SquadPlanner onRecommend={() => setView('recommendations')} onLoadDemo={loadDemoSquad} />}
      {view === 'recommendations' && <Recommendations />}
      {selectedID && <PlayerDrawer id={selectedID} onClose={() => setSelectedID(null)} onCompare={addCompare} onAddToSquad={() => { setSelectedID(null); setView('squad'); setNotice('Use the Squad planner to manage this player.'); }} />}
    </main>
  </div>;
}

export default App;
