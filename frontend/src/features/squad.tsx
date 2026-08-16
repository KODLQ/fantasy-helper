import { useEffect, useMemo, useState } from 'react';
import { api, apiErrorMessage, Player, PlayerResearchItem, positionName, Squad, Team } from '../api';
import '../interactive.css';

const positionLimits: Record<number, number> = { 1: 2, 2: 5, 3: 5, 4: 3 };
const formationCounts: Record<string, [number, number, number]> = {
  '3-4-3': [3, 4, 3], '3-5-2': [3, 5, 2], '4-5-1': [4, 5, 1], '4-4-2': [4, 4, 2],
  '4-3-3': [4, 3, 3], '5-4-1': [5, 4, 1], '5-3-2': [5, 3, 2], '5-2-3': [5, 2, 3],
};

export function SquadPlanner({ seasonId, onRecommend, onLoadDemo, initialPlayerId, onInitialPlayerHandled }: { seasonId: number; onRecommend: () => void; onLoadDemo: () => Promise<void>; initialPlayerId?: number | null; onInitialPlayerHandled?: () => void }) {
  const [squad, setSquad] = useState<Squad | null>(null);
  const [selected, setSelected] = useState<Player[]>([]);
  const [name, setName] = useState('My FPL squad');
  const [formation, setFormation] = useState('3-4-3');
  const [search, setSearch] = useState('');
  const [position, setPosition] = useState(0);
  const [page, setPage] = useState(1);
  const [catalog, setCatalog] = useState<PlayerResearchItem[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [total, setTotal] = useState(0);
  const [loadingCatalog, setLoadingCatalog] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [builderMessage, setBuilderMessage] = useState('');
  const pageSize = 12;

  useEffect(() => {
    const controller = new AbortController();
    setSquad(null); setSelected([]); setError('');
    api.squad(seasonId, controller.signal).then((loaded) => {
      setSquad(loaded); setSelected(loaded.players); setName(loaded.name || 'My FPL squad'); setFormation(loaded.formation || '3-4-3');
    }).catch((reason) => { if (!controller.signal.aborted) setError(apiErrorMessage(reason, 'Could not load squad.')); });
    return () => controller.abort();
  }, [seasonId]);

  useEffect(() => { setPage(1); }, [seasonId, search, position]);
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      const params = new URLSearchParams({ seasonId: String(seasonId), page: String(page), pageSize: String(pageSize), sort: 'points', direction: 'desc' });
      if (search.trim()) params.set('search', search.trim());
      if (position) params.set('position', String(position));
      setLoadingCatalog(true);
      api.players(params, controller.signal).then((result) => { setCatalog(result.items); setTeams(result.teams); setTotal(result.total); })
        .catch((reason) => { if (!controller.signal.aborted) setError(apiErrorMessage(reason, 'Could not load the player pool.')); })
        .finally(() => { if (!controller.signal.aborted) setLoadingCatalog(false); });
    }, 200);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [seasonId, search, position, page]);

  const counts = useMemo(() => selected.reduce<Record<number, number>>((result, player) => ({ ...result, [player.position]: (result[player.position] ?? 0) + 1 }), {}), [selected]);
  const clubCounts = useMemo(() => selected.reduce<Record<number, number>>((result, player) => ({ ...result, [player.teamId]: (result[player.teamId] ?? 0) + 1 }), {}), [selected]);
  const totalCost = selected.reduce((sum, player) => sum + player.price, 0);
  const complete = selected.length === 15 && Object.entries(positionLimits).every(([key, limit]) => (counts[Number(key)] ?? 0) === limit) && Object.values(clubCounts).every((count) => count <= 3) && totalCost <= 100;

  const addPlayer = (player: Player) => {
    setBuilderMessage('');
    if (selected.some((item) => item.id === player.id)) return;
    if (selected.length >= 15) return setBuilderMessage('Remove a player before adding another.');
    if ((counts[player.position] ?? 0) >= positionLimits[player.position]) return setBuilderMessage(`You already have ${positionLimits[player.position]} ${positionName(player.position)} players.`);
    if ((clubCounts[player.teamId] ?? 0) >= 3) return setBuilderMessage('FPL allows at most three players from one club.');
    if (totalCost + player.price > 100.001) return setBuilderMessage(`Adding ${player.webName} would exceed the £100.0 budget.`);
    setSelected((current) => [...current, player]);
    setBuilderMessage(`${player.webName} added to your squad draft.`);
  };

  useEffect(() => {
    if (!initialPlayerId || !squad) return;
    const controller = new AbortController();
    api.player(seasonId, initialPlayerId, controller.signal).then(({ player }) => { addPlayer(player); onInitialPlayerHandled?.(); })
      .catch((reason) => setBuilderMessage(apiErrorMessage(reason, 'Could not add that player.')));
    return () => controller.abort();
    // The player request is consumed once, after the saved draft has loaded.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialPlayerId, squad, seasonId]);

  const removePlayer = (id: number) => { setSelected((current) => current.filter((player) => player.id !== id)); setBuilderMessage(''); };
  const loadDemo = async () => {
    try {
      setError(''); await onLoadDemo();
      const loaded = await api.squad(seasonId);
      setSquad(loaded); setSelected(loaded.players); setName(loaded.name); setFormation(loaded.formation || '3-4-3');
    } catch (reason) { setError(apiErrorMessage(reason, 'Could not load demo squad.')); }
  };
  const saveSquad = async () => {
    if (!complete) return setBuilderMessage('Complete a legal 15-player squad before saving.');
    const lineup = buildLineup(selected, formation);
    setSaving(true); setError(''); setBuilderMessage('');
    try {
      const saved = await api.saveSquad(seasonId, { name: name.trim() || 'My FPL squad', budget: 100, purchasePrices: Object.fromEntries(selected.map((player) => [player.id, player.price])), ...lineup });
      const normalized = { ...saved, validation: saved.validation ?? [] };
      setSquad(normalized); setSelected(normalized.players); setBuilderMessage('Squad saved. You can now optimize the lineup.');
    } catch (reason) { setError(apiErrorMessage(reason, 'Could not save squad.')); }
    finally { setSaving(false); }
  };
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return <section className="page-section">
    <div className="section-heading compare-heading"><div><span className="eyebrow accent">PLANNING ROOM</span><h2>Your squad, your call.</h2><p className="section-subtitle">Choose all 15 players, stay inside FPL constraints, then save your team.</p></div><div className="heading-actions"><button className="secondary-button" onClick={loadDemo}>Load demo squad</button><button className="primary-button" onClick={onRecommend} disabled={!squad || squad.players.length !== 15}>Optimize lineup <span>→</span></button></div></div>
    {error && <div className="inline-error" role="alert">{error}</div>}
    {squad && <>
      <div className="squad-builder" data-testid="squad-builder">
        <div className="builder-heading"><div><h3>Build your team</h3><p>Select 2 goalkeepers, 5 defenders, 5 midfielders, and 3 forwards.</p></div><label>Team name<input aria-label="Team name" value={name} maxLength={80} onChange={(event) => setName(event.target.value)} /></label></div>
        <div className="squad-summary" data-testid="squad-summary"><SummaryStat label="Players" value={`${selected.length}/15`} tone={selected.length === 15 ? 'good' : 'warn'} /><SummaryStat label="Squad cost" value={`£${totalCost.toFixed(1)}`} /><SummaryStat label="Remaining" value={`£${(100 - totalCost).toFixed(1)}`} tone={totalCost <= 100 ? 'good' : 'bad'} /><SummaryStat label="Formation" value={formation} /></div>
        <div className="position-progress">{([1, 2, 3, 4] as const).map((key) => <span key={key} className={(counts[key] ?? 0) === positionLimits[key] ? 'complete' : ''}>{positionName(key)} {counts[key] ?? 0}/{positionLimits[key]}</span>)}</div>
        <div className="builder-layout">
          <div className="pitch"><div className="pitch-label">YOUR 15-PLAYER DRAFT</div>{([1, 2, 3, 4] as const).map((key) => <div className="position-row" key={key}><span className="position-label">{positionName(key)}</span><div className="squad-players">{selected.filter((player) => player.position === key).map((player) => <div className="squad-player" key={player.id}><button className="remove-squad-player" aria-label={`Remove ${player.webName} from squad`} onClick={() => removePlayer(player.id)}>×</button><span className="player-avatar avatar-1">{player.webName.slice(0, 2).toUpperCase()}</span><strong>{player.webName}</strong><small>£{player.price.toFixed(1)}</small></div>)}{Array.from({ length: Math.max(0, positionLimits[key] - (counts[key] ?? 0)) }, (_, index) => <span className="empty-slot" key={index}>Empty</span>)}</div></div>)}</div>
          <div className="player-picker"><div className="picker-filters"><label>Search<input aria-label="Search squad players" value={search} placeholder="Player name…" onChange={(event) => setSearch(event.target.value)} /></label><label>Position<select aria-label="Filter squad players by position" value={position} onChange={(event) => setPosition(Number(event.target.value))}><option value={0}>All</option><option value={1}>GK</option><option value={2}>DEF</option><option value={3}>MID</option><option value={4}>FWD</option></select></label></div>
            <div className="picker-results" aria-live="polite">{loadingCatalog ? <p>Loading players…</p> : catalog.length === 0 ? <p>No players match this search.</p> : catalog.map(({ player, team }) => { const isSelected = selected.some((item) => item.id === player.id); return <div className="picker-player" key={player.id}><div><strong>{player.webName}</strong><small>{team.shortName} · {positionName(player.position)} · £{player.price.toFixed(1)}</small></div><button className="secondary-button" disabled={isSelected} onClick={() => addPlayer(player)}>{isSelected ? 'Selected' : 'Add'}</button></div>; })}</div>
            <div className="picker-pagination"><button aria-label="Previous player page" disabled={page === 1} onClick={() => setPage((current) => current - 1)}>←</button><span>Page {page} of {totalPages}</span><button aria-label="Next player page" disabled={page >= totalPages} onClick={() => setPage((current) => current + 1)}>→</button></div>
          </div>
        </div>
        <div className="builder-actions"><label>Default formation<select aria-label="Default formation" value={formation} onChange={(event) => setFormation(event.target.value)}>{Object.keys(formationCounts).map((item) => <option key={item}>{item}</option>)}</select></label><button className="primary-button" disabled={!complete || saving} onClick={saveSquad}>{saving ? 'Saving…' : squad.players.length ? 'Save squad changes' : 'Create team'}</button></div>
        {builderMessage && <div className={builderMessage.startsWith('Squad saved') || builderMessage.includes(' added ') ? 'builder-message success' : 'builder-message'} role="status">{builderMessage}</div>}
      </div>
      {squad.players.length === 15 && <SquadControls seasonId={seasonId} squad={squad} onSaved={(saved) => { setSquad(saved); setFormation(saved.formation); }} />}
      {squad.validation.length > 0 && squad.players.length > 0 && <div className="validation-panel"><div><strong>Saved-team checks</strong><span>Fix the saved squad to unlock recommendations.</span></div><div className="validation-list">{squad.validation.slice(0, 5).map((item) => <span key={`${item.code}-${item.playerId ?? ''}`}><i />{item.message}</span>)}</div></div>}
    </>}
  </section>;
}

function buildLineup(players: Player[], formation: string) {
  const [defenders, midfielders, forwards] = formationCounts[formation] ?? formationCounts['3-4-3'];
  const ranked = (position: number) => players.filter((player) => player.position === position).sort((left, right) => right.totalPoints - left.totalPoints || left.id - right.id);
  const goalkeepers = ranked(1); const defs = ranked(2); const mids = ranked(3); const fwds = ranked(4);
  const starters = [goalkeepers[0]!, ...defs.slice(0, defenders), ...mids.slice(0, midfielders), ...fwds.slice(0, forwards)];
  const starterIDs = starters.map((player) => player.id);
  const benchPlayerIds = players.filter((player) => !starterIDs.includes(player.id)).sort((left, right) => left.position === 1 ? 1 : right.position === 1 ? -1 : right.totalPoints - left.totalPoints || left.id - right.id).map((player) => player.id);
  const captainOrder = [...starters].sort((left, right) => right.totalPoints - left.totalPoints || left.id - right.id);
  return { startingPlayerIds: starterIDs, benchPlayerIds, captainId: captainOrder[0].id, viceCaptainId: captainOrder[1].id, formation };
}

function SquadControls({ seasonId, squad, onSaved }: { seasonId: number; squad: Squad; onSaved: (squad: Squad) => void }) {
  const [formation, setFormation] = useState(squad.formation || '3-4-3');
  const [captain, setCaptain] = useState(squad.captainId);
  const [viceCaptain, setViceCaptain] = useState(squad.viceCaptainId);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => { setFormation(squad.formation || '3-4-3'); setCaptain(squad.captainId); setViceCaptain(squad.viceCaptainId); }, [squad]);
  const generatedLineup = useMemo(() => buildLineup(squad.players, formation), [squad.players, formation]);
  const starters = squad.players.filter((player) => generatedLineup.startingPlayerIds.includes(player.id));
  useEffect(() => {
    const starterIDs = generatedLineup.startingPlayerIds;
    const nextCaptain = starterIDs.includes(captain) ? captain : generatedLineup.captainId;
    const nextVice = starterIDs.includes(viceCaptain) && viceCaptain !== nextCaptain ? viceCaptain : generatedLineup.viceCaptainId === nextCaptain ? starterIDs.find((id) => id !== nextCaptain) ?? 0 : generatedLineup.viceCaptainId;
    if (nextCaptain !== captain) setCaptain(nextCaptain);
    if (nextVice !== viceCaptain) setViceCaptain(nextVice);
  }, [captain, generatedLineup, viceCaptain]);
  const selectCaptain = (next: number) => { setError(''); if (next === viceCaptain) setViceCaptain(captain); setCaptain(next); };
  const selectViceCaptain = (next: number) => { setError(''); if (next === captain) setCaptain(viceCaptain); setViceCaptain(next); };
  const save = () => {
    if (captain === viceCaptain) return setError('Captain and vice-captain must be different starters.');
    setSaving(true); setError('');
    api.saveSquad(seasonId, { ...squad, ...generatedLineup, captainId: captain, viceCaptainId: viceCaptain }).then(onSaved).catch((reason) => setError(apiErrorMessage(reason, 'Could not save lineup.'))).finally(() => setSaving(false));
  };
  return <div className="lineup-controls"><div><strong>Lineup controls</strong><span>Set formation and armband choices.</span></div><label>Formation<select aria-label="Formation" value={formation} onChange={(event) => { setFormation(event.target.value); setError(''); }}>{Object.keys(formationCounts).map((item) => <option key={item}>{item}</option>)}</select></label><label>Captain<select aria-label="Captain" value={captain} onChange={(event) => selectCaptain(Number(event.target.value))}>{starters.map((player) => <PlayerOption key={player.id} player={player} />)}</select></label><label>Vice-captain<select aria-label="Vice-captain" value={viceCaptain} onChange={(event) => selectViceCaptain(Number(event.target.value))}>{starters.map((player) => <PlayerOption key={player.id} player={player} />)}</select></label><button className="secondary-button" onClick={save} disabled={saving}>{saving ? 'Saving…' : 'Save lineup'}</button>{error && <span className="control-error" role="alert">{error}</span>}</div>;
}

function PlayerOption({ player }: { player: Player }) { return <option value={player.id}>{player.webName}</option>; }
function SummaryStat({ label, value, tone }: { label: string; value: string; tone?: string }) { return <div className="summary-stat"><span>{label}</span><strong className={tone ?? ''}>{value}</strong></div>; }
