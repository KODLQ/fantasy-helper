package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (r *PostgresRepository) LoadManagerSummary(ctx context.Context, userID int64, seasonID, entryID int) (ManagerEntry, bool, error) {
	var item ManagerEntry
	item.EntryID = entryID
	item.SeasonID = seasonID
	err := r.db.QueryRowContext(ctx, `SELECT me.entry_name,me.player_first_name,me.player_last_name,x.overall_points,COALESCE(x.overall_rank,0),x.value,x.bank,x.source_fetched_at,x.normalized_at,x.id::text,x.conflict_state FROM manager_entries me JOIN seasons s ON s.id=me.season_id JOIN LATERAL (SELECT * FROM manager_season_summaries ss WHERE ss.entry_id=me.id ORDER BY ss.normalized_at DESC,ss.id DESC LIMIT 1) x ON TRUE WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3`, userID, seasonID, entryID).Scan(&item.EntryName, &item.PlayerFirstName, &item.PlayerLastName, &item.OverallPoints, &item.OverallRank, &item.TeamValue, &item.Bank, &item.SourceFetchedAt, &item.NormalizedAt, &item.SnapshotID, &item.State)
	if err == sql.ErrNoRows {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	if item.State == "none" {
		item.State = "complete"
	}
	return item, true, nil
}

func (r *PostgresRepository) LoadManagerHistory(ctx context.Context, userID int64, seasonID, entryID int) ([]ManagerGameweek, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT g.source_id,x.points,COALESCE(x.rank,0),COALESCE(x.overall_rank,0),x.bank,x.value,x.transfers,x.transfer_cost,x.bench_points,COALESCE((SELECT chip_name FROM manager_chips c WHERE c.entry_id=me.id AND c.gameweek_id=g.id LIMIT 1),'') FROM manager_entries me JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.season_id=s.id JOIN LATERAL (SELECT * FROM manager_gameweek_summaries h WHERE h.entry_id=me.id AND h.gameweek_id=g.id ORDER BY h.normalized_at DESC,id DESC LIMIT 1) x ON TRUE WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3 ORDER BY g.source_id`, userID, seasonID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ManagerGameweek{}
	for rows.Next() {
		item := ManagerGameweek{EntryID: entryID, SeasonID: seasonID}
		if err := rows.Scan(&item.Gameweek, &item.Points, &item.Rank, &item.OverallRank, &item.Bank, &item.TeamValue, &item.Transfers, &item.TransferCost, &item.BenchPoints, &item.ActiveChip); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LoadManagerPicks(ctx context.Context, userID int64, seasonID, entryID, gameweek int) ([]ManagerPick, error) {
	rows, err := r.db.QueryContext(ctx, `WITH latest AS (SELECT ps.id FROM manager_pick_snapshots ps JOIN manager_entries me ON me.id=ps.entry_id JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.id=ps.gameweek_id WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3 AND g.source_id=$4 ORDER BY ps.normalized_at DESC,ps.id DESC LIMIT 1) SELECT p.source_id,p.position,mp.position,mp.multiplier,mp.is_captain,mp.is_vice_captain FROM latest l JOIN manager_picks mp ON mp.snapshot_id=l.id JOIN players p ON p.id=mp.player_id ORDER BY mp.position`, userID, seasonID, entryID, gameweek)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ManagerPick{}
	for rows.Next() {
		var item ManagerPick
		if err := rows.Scan(&item.PlayerID, &item.PlayerPosition, &item.Position, &item.Multiplier, &item.Captain, &item.ViceCaptain); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LoadManagerTransfers(ctx context.Context, userID int64, seasonID, entryID int) ([]ManagerTransfer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT g.source_id,pin.source_id,pout.source_id,t.player_in_cost,t.player_out_cost,t.made_at FROM manager_transfers t JOIN manager_entries me ON me.id=t.entry_id JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.id=t.gameweek_id JOIN players pin ON pin.id=t.player_in_id JOIN players pout ON pout.id=t.player_out_id WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3 ORDER BY t.made_at`, userID, seasonID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ManagerTransfer{}
	for rows.Next() {
		var item ManagerTransfer
		if err := rows.Scan(&item.Gameweek, &item.PlayerIn, &item.PlayerOut, &item.PlayerInCost, &item.PlayerOutCost, &item.MadeAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LoadAutomaticSubstitutions(ctx context.Context, userID int64, seasonID, entryID, gameweek int) ([]AutomaticSubstitution, error) {
	rows, err := r.db.QueryContext(ctx, `WITH latest AS (SELECT ps.id FROM manager_pick_snapshots ps JOIN manager_entries me ON me.id=ps.entry_id JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.id=ps.gameweek_id WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3 AND g.source_id=$4 ORDER BY ps.normalized_at DESC,ps.id DESC LIMIT 1) SELECT pin.source_id,pout.source_id FROM latest l JOIN manager_automatic_substitutions a ON a.snapshot_id=l.id JOIN players pin ON pin.id=a.player_in_id JOIN players pout ON pout.id=a.player_out_id ORDER BY pin.source_id,pout.source_id`, userID, seasonID, entryID, gameweek)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AutomaticSubstitution{}
	for rows.Next() {
		var item AutomaticSubstitution
		if err := rows.Scan(&item.PlayerIn, &item.PlayerOut); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LoadLeagueStandings(ctx context.Context, userID int64, seasonID, leagueID, gameweek, page int) (LeagueStandings, bool, error) {
	var snapshot int64
	result := LeagueStandings{Members: []LeagueMember{}}
	result.LeagueID = leagueID
	result.Page = page
	err := r.db.QueryRowContext(ctx, `SELECT ls.id,l.name,ls.has_next,ls.id::text FROM league_standing_snapshots ls JOIN classic_leagues l ON l.id=ls.league_id JOIN seasons s ON s.id=l.season_id JOIN gameweeks g ON g.id=ls.gameweek_id WHERE l.user_id=$1 AND s.source_id=$2 AND l.source_id=$3 AND g.source_id=$4 AND ls.page=$5 ORDER BY ls.normalized_at DESC,ls.id DESC LIMIT 1`, userID, seasonID, leagueID, gameweek, page).Scan(&snapshot, &result.Name, &result.HasNext, &result.SnapshotID)
	if err == sql.ErrNoRows {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT entry_source_id,entry_name,player_name,rank,COALESCE(last_rank,0),total_points FROM league_standing_members WHERE snapshot_id=$1 ORDER BY rank,entry_source_id`, snapshot)
	if err != nil {
		return result, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var item LeagueMember
		if err := rows.Scan(&item.EntryID, &item.EntryName, &item.PlayerName, &item.Rank, &item.LastRank, &item.Points); err != nil {
			return result, false, err
		}
		result.Members = append(result.Members, item)
	}
	return result, true, rows.Err()
}

func (r *PostgresRepository) LoadActiveTeam(ctx context.Context, userID int64, seasonID, entryID, gameweek int) (ActiveTeamSnapshot, bool, error) {
	var result ActiveTeamSnapshot
	var missingInputs []byte
	result.EntryID = entryID
	result.SeasonID = seasonID
	result.Gameweek = gameweek
	result.PurchasePrices = map[int]float64{}
	err := r.db.QueryRowContext(ctx, `SELECT ats.id,ats.bank,ats.team_value,COALESCE(ats.active_chip,''),ats.source_fetched_at,ats.normalized_at,ats.state,array_to_json(ats.missing_inputs),ats.conflict_state FROM active_team_snapshots ats JOIN manager_entries me ON me.id=ats.entry_id JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.id=ats.gameweek_id WHERE ats.user_id=$1 AND s.source_id=$2 AND me.source_id=$3 AND g.source_id=$4 ORDER BY ats.normalized_at DESC,ats.id DESC LIMIT 1`, userID, seasonID, entryID, gameweek).Scan(&result.SnapshotID, &result.Bank, &result.TeamValue, &result.ActiveChip, &result.SourceFetchedAt, &result.NormalizedAt, &result.State, &missingInputs, &result.ConflictState)
	if err == sql.ErrNoRows {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if err = json.Unmarshal(missingInputs, &result.MissingInputs); err != nil {
		return result, false, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT p.source_id,x.position,x.multiplier,x.is_captain,x.is_vice_captain,x.purchase_price FROM active_team_snapshot_players x JOIN players p ON p.id=x.player_id WHERE x.snapshot_id=$1 ORDER BY x.position`, result.SnapshotID)
	if err != nil {
		return result, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var pick ManagerPick
		var price int
		if err := rows.Scan(&pick.PlayerID, &pick.Position, &pick.Multiplier, &pick.Captain, &pick.ViceCaptain, &price); err != nil {
			return result, false, err
		}
		result.Picks = append(result.Picks, pick)
		result.PurchasePrices[pick.PlayerID] = float64(price) / 10
	}
	return result, true, rows.Err()
}

func (r *PostgresRepository) LoadLeagueMembers(ctx context.Context, userID int64, seasonID, leagueID, gameweek int) ([]LeagueMember, error) {
	rows, err := r.db.QueryContext(ctx, `WITH latest_pages AS (
SELECT DISTINCT ON (ls.page) ls.id FROM league_standing_snapshots ls
JOIN classic_leagues l ON l.id=ls.league_id JOIN seasons s ON s.id=l.season_id JOIN gameweeks g ON g.id=ls.gameweek_id
WHERE l.user_id=$1 AND s.source_id=$2 AND l.source_id=$3 AND g.source_id=$4
ORDER BY ls.page,ls.normalized_at DESC,ls.id DESC
), members AS (
SELECT DISTINCT ON (m.entry_source_id) m.entry_source_id,m.entry_name,m.player_name,m.rank,COALESCE(m.last_rank,0) AS last_rank,m.total_points
FROM latest_pages lp JOIN league_standing_members m ON m.snapshot_id=lp.id
ORDER BY m.entry_source_id,m.rank
)
SELECT entry_source_id,entry_name,player_name,rank,last_rank,total_points FROM members ORDER BY rank,entry_source_id`, userID, seasonID, leagueID, gameweek)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LeagueMember{}
	for rows.Next() {
		var item LeagueMember
		if err := rows.Scan(&item.EntryID, &item.EntryName, &item.PlayerName, &item.Rank, &item.LastRank, &item.Points); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LoadPlayerGameweekPoints(ctx context.Context, seasonID, gameweek int) (map[int]int, string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.source_id,f.total_points,f.finalized FROM player_gameweek_facts f JOIN players p ON p.id=f.player_id JOIN gameweeks g ON g.id=f.gameweek_id JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$1 AND g.source_id=$2`, seasonID, gameweek)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	points := map[int]int{}
	state := "actual"
	seen := false
	for rows.Next() {
		var id, value int
		var finalized bool
		if err := rows.Scan(&id, &value, &finalized); err != nil {
			return nil, "", err
		}
		points[id] = value
		seen = true
		if !finalized {
			state = "provisional"
		}
	}
	if !seen {
		state = "estimated"
		estimateRows, estimateErr := r.db.QueryContext(ctx, `SELECT p.source_id,ROUND(COALESCE(ps.form,0))::INTEGER FROM players p JOIN seasons s ON s.id=p.season_id LEFT JOIN LATERAL (SELECT form FROM player_snapshots x JOIN dataset_snapshots d ON d.id=x.snapshot_id WHERE x.player_id=p.id ORDER BY d.normalized_at DESC LIMIT 1) ps ON TRUE WHERE s.source_id=$1`, seasonID)
		if estimateErr != nil {
			return nil, "", estimateErr
		}
		defer estimateRows.Close()
		for estimateRows.Next() {
			var id, value int
			if scanErr := estimateRows.Scan(&id, &value); scanErr != nil {
				return nil, "", scanErr
			}
			points[id] = value
		}
		if estimateErr = estimateRows.Err(); estimateErr != nil {
			return nil, "", estimateErr
		}
	}
	return points, state, rows.Err()
}

func (r *PostgresRepository) LoadUnfinishedFixtureIDs(ctx context.Context, seasonID, gameweek int) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT f.source_id FROM fixtures f JOIN seasons s ON s.id=f.season_id JOIN gameweeks g ON g.id=f.gameweek_id WHERE s.source_id=$1 AND g.source_id=$2 AND f.finished=FALSE ORDER BY f.kickoff_time NULLS LAST,f.source_id`, seasonID, gameweek)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LoadManagerDecisionAnalysis(ctx context.Context, userID int64, seasonID, entryID, gameweek int) (ManagerDecisionAnalysis, error) {
	picks, err := r.LoadManagerPicks(ctx, userID, seasonID, entryID, gameweek)
	if err != nil {
		return ManagerDecisionAnalysis{}, err
	}
	points, state, err := r.LoadPlayerGameweekPoints(ctx, seasonID, gameweek)
	if err != nil {
		return ManagerDecisionAnalysis{}, err
	}
	analysis := ManagerDecisionAnalysis{EntryID: entryID, SeasonID: seasonID, Gameweek: gameweek, Picks: picks, OutcomeState: state, FormulaVersions: []string{"player-points-v1", "team-points-v1", "captain-delta-v1", "transfer-cost-v1"}}
	metaErr := r.db.QueryRowContext(ctx, `SELECT ps.source_fetched_at,ps.normalized_at,ps.id::text,ps.conflict_state FROM manager_pick_snapshots ps JOIN manager_entries me ON me.id=ps.entry_id JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.id=ps.gameweek_id WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3 AND g.source_id=$4 ORDER BY ps.normalized_at DESC,ps.id DESC LIMIT 1`, userID, seasonID, entryID, gameweek).Scan(&analysis.SourceFetchedAt, &analysis.NormalizedAt, &analysis.SnapshotID, &analysis.ConflictState)
	if metaErr != nil && metaErr != sql.ErrNoRows {
		return analysis, metaErr
	}
	err = r.db.QueryRowContext(ctx, `SELECT h.transfer_cost,h.bench_points,COALESCE((SELECT c.chip_name FROM manager_chips c WHERE c.entry_id=me.id AND c.gameweek_id=g.id LIMIT 1),'') FROM manager_entries me JOIN seasons s ON s.id=me.season_id JOIN gameweeks g ON g.season_id=s.id AND g.source_id=$4 JOIN LATERAL (SELECT * FROM manager_gameweek_summaries x WHERE x.entry_id=me.id AND x.gameweek_id=g.id ORDER BY x.normalized_at DESC,x.id DESC LIMIT 1) h ON TRUE WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3`, userID, seasonID, entryID, gameweek).Scan(&analysis.TransferCost, &analysis.BenchPoints, &analysis.ActiveChip)
	if err == sql.ErrNoRows {
		err = nil
	}
	if err != nil {
		return analysis, err
	}
	for _, pick := range picks {
		analysis.GrossPoints += points[pick.PlayerID] * pick.Multiplier
	}
	analysis.NetPoints = analysis.GrossPoints - analysis.TransferCost
	if state != "actual" {
		analysis.Warning = "Public player points are not finalized and may change."
	}
	return analysis, nil
}

func (r *PostgresRepository) LoadLeagueMemberFailures(ctx context.Context, userID int64, seasonID, leagueID, gameweek int) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT l.entry_source_id FROM league_member_pick_links l JOIN classic_leagues cl ON cl.id=l.league_id JOIN seasons s ON s.id=cl.season_id JOIN gameweeks g ON g.id=l.gameweek_id WHERE cl.user_id=$1 AND s.source_id=$2 AND cl.source_id=$3 AND g.source_id=$4 AND l.state='failed' ORDER BY l.entry_source_id`, userID, seasonID, leagueID, gameweek)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func activeTeamSquad(snapshot ActiveTeamSnapshot) Squad {
	squad := Squad{Name: "Imported FPL team", Budget: float64(snapshot.TeamValue+snapshot.Bank) / 10, PurchasePrices: snapshot.PurchasePrices}
	for _, pick := range snapshot.Picks {
		if pick.Multiplier > 0 {
			squad.StartingPlayerIDs = append(squad.StartingPlayerIDs, pick.PlayerID)
		} else {
			squad.BenchPlayerIDs = append(squad.BenchPlayerIDs, pick.PlayerID)
		}
		if pick.Captain {
			squad.CaptainID = pick.PlayerID
		}
		if pick.ViceCaptain {
			squad.ViceCaptainID = pick.PlayerID
		}
	}
	return squad
}

func BuildImportPreview(snapshot ActiveTeamSnapshot, current Squad, domain *Store) (SquadImportPreview, error) {
	if domain == nil {
		return SquadImportPreview{}, fmt.Errorf("planning domain is required")
	}
	proposed := activeTeamSquad(snapshot)
	proposed = domain.EnrichSquad(proposed)
	counts := map[int]int{}
	for _, id := range proposed.StartingPlayerIDs {
		if player, ok := domain.Player(id); ok {
			counts[player.Position]++
		}
	}
	proposed.Formation = fmt.Sprintf("%d-%d-%d", counts[Defender], counts[Midfielder], counts[Forward])
	validation := domain.ValidatePlan(proposed)
	if validation == nil {
		validation = []ValidationError{}
	}
	currentIDs := map[int]bool{}
	for id := range current.PurchasePrices {
		currentIDs[id] = true
	}
	nextIDs := map[int]bool{}
	for id := range proposed.PurchasePrices {
		nextIDs[id] = true
	}
	preview := SquadImportPreview{Snapshot: snapshot, Proposed: proposed, AddedPlayerIDs: []int{}, RemovedPlayerIDs: []int{}, Validation: validation}
	for id := range nextIDs {
		if !currentIDs[id] {
			preview.AddedPlayerIDs = append(preview.AddedPlayerIDs, id)
		}
	}
	for id := range currentIDs {
		if !nextIDs[id] {
			preview.RemovedPlayerIDs = append(preview.RemovedPlayerIDs, id)
		}
	}
	preview.LineupChanged = !sameIDs(current.StartingPlayerIDs, proposed.StartingPlayerIDs)
	preview.CaptainChanged = current.CaptainID != proposed.CaptainID || current.ViceCaptainID != proposed.ViceCaptainID
	preview.HasChanges = len(preview.AddedPlayerIDs) > 0 || len(preview.RemovedPlayerIDs) > 0 || preview.LineupChanged || preview.CaptainChanged
	return preview, nil
}

func sameIDs(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[int]int{}
	for _, id := range left {
		seen[id]++
	}
	for _, id := range right {
		seen[id]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
