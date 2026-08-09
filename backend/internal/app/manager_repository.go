package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type ManagerDataRepository interface {
	UpsertManagerScope(context.Context, int64, ManagerScope) (ManagerScope, error)
	ListManagerScopes(context.Context, int64) ([]ManagerScope, error)
	UpsertManagerConnection(context.Context, int64, int, string, RemoteSessionState) error
	SetManagerConnectionState(context.Context, int64, int, RemoteSessionState) error
	StartManagerRun(context.Context, int64, string) (int64, error)
	FinishManagerRun(context.Context, int64, string, string) error
	EnqueueManagerWork(context.Context, int64, []ManagerWorkItem) error
	PersistEntry(context.Context, int64, int, sourceEntry, string, time.Time) error
	PersistHistory(context.Context, int64, int, int, sourceEntryHistory, string, time.Time) error
	PersistTransfers(context.Context, int64, int, int, []sourceTransfer, string) error
	PersistPicks(context.Context, int64, int, int, int, sourcePicks, string, time.Time, string) (int64, error)
	PersistActiveTeam(context.Context, int64, int, int, int, sourceMyTeam, sourcePicks, string, time.Time) (int64, error)
	PersistLeaguePage(context.Context, int64, int, int, int, int, sourceLeagueStandings, string, time.Time) error
	ExportManagerData(context.Context, int64) (map[string]any, error)
	DeleteManagerData(context.Context, int64) error
}

type ManagerWorkItem struct {
	NaturalKey, Stage, Endpoint string
	Checkpoint                  map[string]any
}

func (r *PostgresRepository) UpsertManagerScope(ctx context.Context, userID int64, scope ManagerScope) (ManagerScope, error) {
	if userID <= 0 || scope.SourceID <= 0 || (scope.Type != "entry" && scope.Type != "league") {
		return ManagerScope{}, fmt.Errorf("valid owner, scope type, and source ID are required")
	}
	if scope.MemberLimit <= 0 {
		scope.MemberLimit = 50
	}
	err := r.db.QueryRowContext(ctx, `INSERT INTO manager_sync_scopes (user_id,scope_type,source_id,enabled,member_limit) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (user_id,scope_type,source_id) DO UPDATE SET enabled=EXCLUDED.enabled,member_limit=EXCLUDED.member_limit,updated_at=NOW() RETURNING id,scope_type,source_id,enabled,member_limit`, userID, scope.Type, scope.SourceID, scope.Enabled, scope.MemberLimit).Scan(&scope.ID, &scope.Type, &scope.SourceID, &scope.Enabled, &scope.MemberLimit)
	return scope, err
}

func (r *PostgresRepository) ListManagerScopes(ctx context.Context, userID int64) ([]ManagerScope, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,scope_type,source_id,enabled,member_limit FROM manager_sync_scopes WHERE user_id=$1 ORDER BY scope_type,source_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ManagerScope{}
	for rows.Next() {
		var item ManagerScope
		if err := rows.Scan(&item.ID, &item.Type, &item.SourceID, &item.Enabled, &item.MemberLimit); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) UpsertManagerConnection(ctx context.Context, userID int64, entryID int, provider string, state RemoteSessionState) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO manager_connections (user_id,entry_source_id,provider_type,state,last_validated_at) VALUES ($1,$2,$3,$4,CASE WHEN $4='connected' THEN NOW() END) ON CONFLICT (user_id,entry_source_id) DO UPDATE SET provider_type=EXCLUDED.provider_type,state=EXCLUDED.state,last_validated_at=EXCLUDED.last_validated_at,revoked_at=NULL,updated_at=NOW()`, userID, entryID, provider, state)
	return err
}
func (r *PostgresRepository) SetManagerConnectionState(ctx context.Context, userID int64, entryID int, state RemoteSessionState) error {
	_, err := r.db.ExecContext(ctx, `UPDATE manager_connections SET state=$3,last_validated_at=CASE WHEN $3='connected' THEN NOW() ELSE last_validated_at END,revoked_at=CASE WHEN $3='revoked' THEN NOW() ELSE revoked_at END,updated_at=NOW() WHERE user_id=$1 AND entry_source_id=$2`, userID, entryID, state)
	return err
}

func (r *PostgresRepository) StartManagerRun(ctx context.Context, userID int64, correlationID string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO manager_sync_runs (user_id,correlation_id) VALUES ($1,$2) RETURNING id`, userID, nullableString(correlationID)).Scan(&id)
	return id, err
}
func (r *PostgresRepository) FinishManagerRun(ctx context.Context, runID int64, status, warning string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE manager_sync_runs SET status=$2,warning=$3,finished_at=NOW() WHERE id=$1`, runID, status, nullableString(warning))
	return err
}
func (r *PostgresRepository) EnqueueManagerWork(ctx context.Context, runID int64, items []ManagerWorkItem) error {
	for _, item := range items {
		checkpoint, _ := json.Marshal(item.Checkpoint)
		if _, err := r.db.ExecContext(ctx, `INSERT INTO manager_sync_work_items (run_id,natural_key,stage,endpoint,checkpoint) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (run_id,natural_key) DO NOTHING`, runID, item.NaturalKey, item.Stage, item.Endpoint, checkpoint); err != nil {
			return err
		}
	}
	return nil
}

func resolveManagerEntry(tx *sql.Tx, ctx context.Context, userID int64, seasonSourceID, entrySourceID int) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT me.id FROM manager_entries me JOIN seasons s ON s.id=me.season_id WHERE me.user_id=$1 AND s.source_id=$2 AND me.source_id=$3`, userID, seasonSourceID, entrySourceID).Scan(&id)
	return id, err
}

func (r *PostgresRepository) PersistEntry(ctx context.Context, userID int64, seasonID int, value sourceEntry, checksum string, fetched time.Time) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		var entryID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO manager_entries (user_id,season_id,source_id,player_first_name,player_last_name,entry_name,started_gameweek,normalized_at) SELECT $1,s.id,$3,$4,$5,$6,$7,NOW() FROM seasons s WHERE s.source_id=$2 ON CONFLICT (user_id,season_id,source_id) DO UPDATE SET player_first_name=EXCLUDED.player_first_name,player_last_name=EXCLUDED.player_last_name,entry_name=EXCLUDED.entry_name,started_gameweek=EXCLUDED.started_gameweek,normalized_at=NOW() RETURNING id`, userID, seasonID, value.ID, value.PlayerFirstName, value.PlayerLastName, value.Name, nullableInt(value.StartedEvent)).Scan(&entryID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO manager_season_summaries (entry_id,overall_points,overall_rank,value,bank,source_endpoint,source_checksum,source_fetched_at,normalized_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW()) ON CONFLICT (entry_id,source_checksum) DO NOTHING`, entryID, value.SummaryOverallPoints, nullableInt(value.SummaryOverallRank), value.LastDeadlineValue, value.LastDeadlineBank, fmt.Sprintf("/entry/%d/", value.ID), checksum, fetched)
		return err
	})
}

func (r *PostgresRepository) PersistHistory(ctx context.Context, userID int64, seasonID, entryID int, value sourceEntryHistory, checksum string, fetched time.Time) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		entryDB, err := resolveManagerEntry(tx, ctx, userID, seasonID, entryID)
		if err != nil {
			return err
		}
		for _, row := range value.Current {
			_, err = tx.ExecContext(ctx, `INSERT INTO manager_gameweek_summaries (entry_id,gameweek_id,points,rank,overall_rank,bank,value,transfers,transfer_cost,bench_points,source_checksum,source_fetched_at,normalized_at) SELECT $1,g.id,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW() FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$2 AND g.source_id=$13 ON CONFLICT (entry_id,gameweek_id,source_checksum) DO NOTHING`, entryDB, seasonID, row.Points, nullableInt(row.Rank), nullableInt(row.OverallRank), row.Bank, row.Value, row.EventTransfers, row.EventTransfersCost, row.PointsOnBench, checksum, fetched, row.Event)
			if err != nil {
				return err
			}
		}
		for _, chip := range value.Chips {
			_, err = tx.ExecContext(ctx, `INSERT INTO manager_chips (entry_id,gameweek_id,chip_name,played_at) SELECT $1,g.id,$4,$5 FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$2 AND g.source_id=$3 ON CONFLICT DO NOTHING`, entryDB, seasonID, chip.Event, chip.Name, chip.Time)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresRepository) PersistTransfers(ctx context.Context, userID int64, seasonID, entryID int, items []sourceTransfer, checksum string) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		entryDB, err := resolveManagerEntry(tx, ctx, userID, seasonID, entryID)
		if err != nil {
			return err
		}
		for _, v := range items {
			_, err = tx.ExecContext(ctx, `INSERT INTO manager_transfers (entry_id,gameweek_id,player_in_id,player_out_id,player_in_cost,player_out_cost,made_at,source_checksum) SELECT $1,g.id,pin.id,pout.id,$4,$5,$6,$7 FROM seasons s JOIN gameweeks g ON g.season_id=s.id AND g.source_id=$3 JOIN players pin ON pin.season_id=s.id AND pin.source_id=$8 JOIN players pout ON pout.season_id=s.id AND pout.source_id=$9 WHERE s.source_id=$2 ON CONFLICT DO NOTHING`, entryDB, seasonID, v.Event, v.ElementInCost, v.ElementOutCost, v.Time, checksum, v.ElementIn, v.ElementOut)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresRepository) PersistPicks(ctx context.Context, userID int64, seasonID, entryID, gameweek int, value sourcePicks, checksum string, fetched time.Time, endpoint string) (int64, error) {
	var snapshotID int64
	err := r.WithTransaction(ctx, func(tx *sql.Tx) error {
		entryDB, err := resolveManagerEntry(tx, ctx, userID, seasonID, entryID)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO manager_pick_snapshots (entry_id,gameweek_id,source_endpoint,source_checksum,source_fetched_at,normalized_at) SELECT $1,g.id,$4,$5,$6,NOW() FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$2 AND g.source_id=$3 ON CONFLICT (entry_id,gameweek_id,source_checksum) DO UPDATE SET normalized_at=EXCLUDED.normalized_at RETURNING id`, entryDB, seasonID, gameweek, endpoint, checksum, fetched).Scan(&snapshotID)
		if err != nil {
			return err
		}
		for _, pick := range value.Picks {
			_, err = tx.ExecContext(ctx, `INSERT INTO manager_picks (snapshot_id,player_id,position,multiplier,is_captain,is_vice_captain) SELECT $1,p.id,$4,$5,$6,$7 FROM players p JOIN seasons s ON s.id=p.season_id WHERE s.source_id=$2 AND p.source_id=$3 ON CONFLICT (snapshot_id,player_id) DO UPDATE SET position=EXCLUDED.position,multiplier=EXCLUDED.multiplier,is_captain=EXCLUDED.is_captain,is_vice_captain=EXCLUDED.is_vice_captain`, snapshotID, seasonID, pick.Element, pick.Position, pick.Multiplier, pick.IsCaptain, pick.IsViceCaptain)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return snapshotID, err
}

func (r *PostgresRepository) PersistActiveTeam(ctx context.Context, userID int64, seasonID, entryID, gameweek int, team sourceMyTeam, picks sourcePicks, checksum string, fetched time.Time) (int64, error) {
	var id int64
	err := r.WithTransaction(ctx, func(tx *sql.Tx) error {
		entryDB, err := resolveManagerEntry(tx, ctx, userID, seasonID, entryID)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO active_team_snapshots (user_id,entry_id,gameweek_id,bank,team_value,source_endpoint_set,source_checksum,source_fetched_at,normalized_at) SELECT $1,$2,g.id,$5,$6,ARRAY[$7,$8],$9,$10,NOW() FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$3 AND g.source_id=$4 ON CONFLICT (user_id,entry_id,gameweek_id,source_checksum) DO UPDATE SET normalized_at=EXCLUDED.normalized_at RETURNING id`, userID, entryDB, seasonID, gameweek, team.Transfers.Bank, team.Transfers.Value, fmt.Sprintf("/my-team/%d/", entryID), fmt.Sprintf("/entry/%d/event/%d/picks/", entryID, gameweek), checksum, fetched).Scan(&id)
		if err != nil {
			return err
		}
		for _, pick := range team.Picks {
			_, err = tx.ExecContext(ctx, `INSERT INTO active_team_snapshot_players (snapshot_id,player_id,position,multiplier,purchase_price,selling_price,is_captain,is_vice_captain) SELECT $1,p.id,$4,$5,$6,$7,$8,$9 FROM players p JOIN seasons s ON s.id=p.season_id WHERE s.source_id=$2 AND p.source_id=$3 ON CONFLICT (snapshot_id,player_id) DO UPDATE SET position=EXCLUDED.position,multiplier=EXCLUDED.multiplier,purchase_price=EXCLUDED.purchase_price,selling_price=EXCLUDED.selling_price,is_captain=EXCLUDED.is_captain,is_vice_captain=EXCLUDED.is_vice_captain`, id, seasonID, pick.Element, pick.Position, pick.Multiplier, pick.PurchasePrice, pick.SellingPrice, pick.IsCaptain, pick.IsViceCaptain)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (r *PostgresRepository) PersistLeaguePage(ctx context.Context, userID int64, seasonID, gameweek, phase, page int, value sourceLeagueStandings, checksum string, fetched time.Time) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		var leagueDB int64
		err := tx.QueryRowContext(ctx, `INSERT INTO classic_leagues (user_id,season_id,source_id,name,closed) SELECT $1,s.id,$3,$4,$5 FROM seasons s WHERE s.source_id=$2 ON CONFLICT (user_id,season_id,source_id) DO UPDATE SET name=EXCLUDED.name,closed=EXCLUDED.closed,updated_at=NOW() RETURNING id`, userID, seasonID, value.League.ID, value.League.Name, value.League.Closed).Scan(&leagueDB)
		if err != nil {
			return err
		}
		var snapshot int64
		err = tx.QueryRowContext(ctx, `INSERT INTO league_standing_snapshots (league_id,gameweek_id,phase,page,has_next,source_checksum,source_fetched_at,normalized_at) SELECT $1,g.id,$4,$5,$6,$7,$8,NOW() FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$2 AND g.source_id=$3 ON CONFLICT (league_id,gameweek_id,phase,page,source_checksum) DO UPDATE SET normalized_at=EXCLUDED.normalized_at RETURNING id`, leagueDB, seasonID, gameweek, phase, page, value.Standings.HasNext, checksum, fetched).Scan(&snapshot)
		if err != nil {
			return err
		}
		for _, m := range value.Standings.Results {
			_, err = tx.ExecContext(ctx, `INSERT INTO league_standing_members (snapshot_id,entry_source_id,entry_name,player_name,rank,last_rank,total_points) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (snapshot_id,entry_source_id) DO UPDATE SET entry_name=EXCLUDED.entry_name,player_name=EXCLUDED.player_name,rank=EXCLUDED.rank,last_rank=EXCLUDED.last_rank,total_points=EXCLUDED.total_points`, snapshot, m.Entry, m.EntryName, m.PlayerName, m.Rank, nullableInt(m.LastRank), m.Total)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresRepository) ExportManagerData(ctx context.Context, userID int64) (map[string]any, error) {
	scopes, err := r.ListManagerScopes(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT entry_source_id,provider_type,state,last_validated_at FROM manager_connections WHERE user_id=$1 ORDER BY entry_source_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	connections := []map[string]any{}
	for rows.Next() {
		var id int
		var provider, state string
		var validated sql.NullTime
		if err := rows.Scan(&id, &provider, &state, &validated); err != nil {
			return nil, err
		}
		connections = append(connections, map[string]any{"entryId": id, "providerType": provider, "state": state, "lastValidatedAt": validated.Time})
	}
	return map[string]any{"scopes": scopes, "connections": connections, "exportedAt": time.Now().UTC()}, rows.Err()
}
func (r *PostgresRepository) DeleteManagerData(ctx context.Context, userID int64) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		tables := []string{"manager_sync_runs", "classic_leagues", "manager_entries", "manager_connections", "manager_sync_scopes"}
		sort.Strings(tables)
		for _, table := range tables {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id=$1`, userID); err != nil {
				return err
			}
		}
		return nil
	})
}
