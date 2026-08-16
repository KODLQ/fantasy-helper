import { useEffect, useState } from 'react';
import { api, fullPositionName, playerFixtureDifficulty } from '../api';
import { DataState } from '../components/data-state';
import '../interactive.css';

export function PlayerDrawer({ seasonId, id, onClose, onCompare, onAddToSquad }: { seasonId: number; id: number; onClose: () => void; onCompare: (id: number) => void; onAddToSquad: () => void }) {
  const [data, setData] = useState<Awaited<ReturnType<typeof api.player>> | null>(null);
  const [error, setError] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    setData(null); setError('');
    api.player(seasonId, id, controller.signal).then(setData).catch((reason) => {
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : 'Could not load player profile.');
    });
    return () => controller.abort();
  }, [seasonId, id]);
  if (!data) return <div className="drawer-backdrop" onClick={onClose}><aside className="drawer loading-drawer" onClick={(event) => event.stopPropagation()}><DataState status={error ? 'error' : 'loading'} message={error || 'Loading player profile…'} /></aside></div>;
  const difficulty = playerFixtureDifficulty(data.player.teamId, data.fixtures[0]);
  return <div className="drawer-backdrop" onClick={onClose}><aside className="drawer" data-testid="player-drawer" onClick={(event) => event.stopPropagation()}>
    <button className="drawer-close" aria-label="Close player profile" onClick={onClose}>×</button>
    <div className="drawer-profile"><span className="large-avatar">{data.player.webName.slice(0, 2).toUpperCase()}</span><div><span className="eyebrow">{fullPositionName(data.player.position)}</span><h2>{data.player.webName}</h2><span className="muted">{data.team.name} · £{data.player.price.toFixed(1)}</span></div></div>
    <div className="drawer-actions"><button className="secondary-button" onClick={() => onCompare(id)}>＋ Compare</button><button className="secondary-button" onClick={onAddToSquad}>Add to squad</button></div>
    <div className="profile-score"><div><span className="eyebrow">FORM</span><strong>{data.player.form.toFixed(1)}</strong></div><div><span className="eyebrow">POINTS</span><strong>{data.player.totalPoints}</strong></div><div><span className="eyebrow">VALUE</span><strong>{data.player.value.toFixed(1)}</strong></div></div>
    <div className="drawer-section"><div className="section-heading compact"><h3>Recent output</h3><span className="muted">Gameweek history</span></div><div className="history-list">{data.history.length === 0 ? <div><span>No completed gameweek history yet.</span></div> : data.history.slice(-5).map((item) => <div key={item.gameweek}><span>GW {item.gameweek}</span><strong>{item.totalPoints} pts</strong><small>{item.minutes} min</small></div>)}</div></div>
    <div className="drawer-section"><div className="section-heading compact"><h3>Next fixture</h3><span className="muted">Fixture context</span></div><div className="fixture-card"><span className="fixture-badge">Next</span><strong>{data.fixtures.length ? data.fixtureContext || 'Opponent unavailable' : 'No upcoming fixture loaded'}</strong><span className="difficulty">{difficulty === undefined ? 'Difficulty unavailable' : `Difficulty ${difficulty}/5`}</span></div></div>
  </aside></div>;
}
