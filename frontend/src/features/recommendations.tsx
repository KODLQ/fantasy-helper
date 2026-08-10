import { useEffect, useRef, useState } from 'react';
import { api, apiErrorMessage, positionName, Recommendation, RecommendationPlayer } from '../api';
import '../interactive.css';

const defaultWeights: Record<string, number> = { form: 0.28, minutes: 0.25, fixture: 0.2, recentReturns: 0.17, value: 0.1 };

export function Recommendations({ seasonId }: { seasonId: number }) {
  const [recommendation, setRecommendation] = useState<Recommendation | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [weights, setWeights] = useState<Record<string, number>>(defaultWeights);
  const request = useRef<AbortController | null>(null);

  useEffect(() => {
    request.current?.abort();
    setRecommendation(null);
    setError('');
    setLoading(false);
    return () => request.current?.abort();
  }, [seasonId]);

  const weightTotal = Object.values(weights).reduce((sum, value) => sum + value, 0);
  const weightsValid = Object.values(weights).every((value) => Number.isFinite(value) && value >= 0 && value <= 1) && Math.abs(weightTotal - 1) < 0.001;
  const updateWeight = (name: string, value: string) => {
    setWeights((current) => ({ ...current, [name]: Number(value) }));
    setRecommendation(null);
    setError('');
  };
  const run = () => {
    if (!weightsValid) {
      setRecommendation(null);
      setError('Signal weights must each be between 0 and 1 and total exactly 1.00.');
      return;
    }
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setLoading(true);
    setError('');
    api.recommend(seasonId, weights, controller.signal)
      .then((result) => {
        if (!controller.signal.aborted) setRecommendation(result.recommendation);
      })
      .catch((reason) => {
        if (!controller.signal.aborted) {
          setRecommendation(null);
          setError(apiErrorMessage(reason, 'Build a valid squad first.'));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
  };

  return <section className="page-section">
    <div className="section-heading compare-heading">
      <div><span className="eyebrow accent">MODEL OUTPUT · LINEUP OPTIMIZER</span><h2>A lineup with its work shown.</h2><p className="section-subtitle">A transparent heuristic for this gameweek—not a promise of points.</p></div>
      <button className="primary-button" onClick={run} disabled={loading || !weightsValid}>{loading ? 'Thinking…' : 'Run recommendation'} <span>✦</span></button>
    </div>
    <div className="weight-controls">
      <div><strong>Signal weights</strong><span>Adjust the lens, then rerun.</span></div>
      {Object.entries(weights).map(([name, value]) => <label key={name}>{name === 'recentReturns' ? 'Returns' : name}<input type="number" min="0" max="1" step="0.01" value={value} onChange={(event) => updateWeight(name, event.target.value)} /></label>)}
      <b className={weightsValid ? 'weight-total valid' : 'weight-total'}>{weightTotal.toFixed(2)} total</b>
    </div>
    {!weightsValid && !error && <div className="inline-error" role="alert">Signal weights must total exactly 1.00.</div>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {!recommendation && !error && weightsValid && <div className="empty-panel"><div className="empty-icon">✦</div><h3>Ready when your squad is.</h3><p>Run the baseline once you have a valid 15-player squad.</p></div>}
    {recommendation && <>
      <div className="recommendation-banner" data-testid="recommendation-banner"><div><span className="eyebrow">RECOMMENDED CAPTAIN</span><h3>{recommendation.captain.player.webName} <span>×2</span></h3><p>{recommendation.captain.explanation}</p></div><div className="captain-score">{recommendation.captain.score}<span>score</span></div></div>
      <div className="recommendation-layout">
        <div className="lineup-panel"><div className="section-heading compact"><h3>Starting XI</h3><span className="muted">{recommendation.formation} · {recommendation.startingXI.length} starters</span></div><div className="lineup-list">{recommendation.startingXI.map((item) => <PlayerRecommendation key={item.player.id} item={item} captain={item.player.id === recommendation.captain.player.id} vice={item.player.id === recommendation.viceCaptain.player.id} />)}</div></div>
        <div className="lineup-panel bench-panel"><div className="section-heading compact"><h3>Substitutes</h3><span className="muted">3 outfield · 1 goalkeeper</span></div><div className="lineup-list">{recommendation.bench.map((item, index) => <PlayerRecommendation key={item.player.id} item={item} rank={item.player.position === 1 ? 'GK' : recommendation.bench.slice(0, index + 1).filter((candidate) => candidate.player.position !== 1).length} />)}</div></div>
      </div>
      <p className="heuristic-notice">ⓘ {recommendation.heuristicNotice} Algorithm {recommendation.algorithmVersion} · snapshot {new Date(recommendation.snapshotAt).toLocaleString()}</p>
    </>}
  </section>;
}

function PlayerRecommendation({ item, captain, vice, rank }: { item: RecommendationPlayer; captain?: boolean; vice?: boolean; rank?: number | string }) {
  return <div className="recommendation-player"><span className="rank-badge">{rank ?? positionName(item.player.position)}</span><span className="player-avatar avatar-2">{item.player.webName.slice(0, 2).toUpperCase()}</span><div className="recommendation-name"><strong>{item.player.webName} {captain && <b className="role-badge captain">C</b>}{vice && <b className="role-badge vice">V</b>}</strong><small>{item.fixture}</small></div><div className="factor-preview">{item.factors.slice(0, 3).map((factor) => <span key={factor.name} title={`${factor.name}: ${factor.signal}`}>{factor.name.slice(0, 3).toUpperCase()}</span>)}</div><strong className="rec-score">{item.score}</strong></div>;
}
