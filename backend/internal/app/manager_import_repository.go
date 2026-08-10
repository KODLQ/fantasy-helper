package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
)

func (r *PostgresRepository) ImportActiveTeam(ctx context.Context, userID int64, seasonID int, snapshotID int64, mode string, squad Squad) (SquadImportResult, error) {
	if userID <= 0 || seasonID <= 0 || snapshotID <= 0 || (mode != "draft" && mode != "replace") {
		return SquadImportResult{}, fmt.Errorf("valid owner, season, snapshot, and import mode are required")
	}
	result := SquadImportResult{SnapshotID: snapshotID, Mode: mode, Squad: squad}
	err := r.WithTransaction(ctx, func(tx *sql.Tx) error {
		var existingPlan, existingDraft sql.NullInt64
		var existingVersion int64
		err := tx.QueryRowContext(ctx, `SELECT plan_id,draft_id,resulting_version FROM squad_import_events WHERE user_id=$1 AND snapshot_id=$2 AND mode=$3`, userID, snapshotID, mode).Scan(&existingPlan, &existingDraft, &existingVersion)
		if err == nil {
			result.PlanID = existingPlan.Int64
			result.DraftID = existingDraft.Int64
			result.ResultingVersion = existingVersion
			result.Idempotent = true
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		var seasonDB int64
		if err := tx.QueryRowContext(ctx, `SELECT s.id FROM active_team_snapshots ats JOIN manager_entries me ON me.id=ats.entry_id JOIN seasons s ON s.id=me.season_id WHERE ats.id=$1 AND ats.user_id=$2 AND s.source_id=$3`, snapshotID, userID, seasonID).Scan(&seasonDB); err != nil {
			return fmt.Errorf("resolve owned import snapshot: %w", err)
		}
		result.ResultingVersion = snapshotID
		if mode == "draft" {
			payload, err := json.Marshal(squad)
			if err != nil {
				return err
			}
			if err = tx.QueryRowContext(ctx, `INSERT INTO squad_import_drafts (user_id,snapshot_id,season_id,name,squad) VALUES ($1,$2,$3,$4,$5) RETURNING id`, userID, snapshotID, seasonDB, squad.Name, payload).Scan(&result.DraftID); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO squad_import_events (user_id,snapshot_id,draft_id,mode,resulting_version) VALUES ($1,$2,$3,'draft',$4)`, userID, snapshotID, result.DraftID, result.ResultingVersion)
			return err
		}
		planID, err := r.saveImportedSquadTx(ctx, tx, userID, seasonID, squad)
		if err != nil {
			return err
		}
		result.PlanID = planID
		_, err = tx.ExecContext(ctx, `INSERT INTO squad_import_events (user_id,snapshot_id,plan_id,mode,resulting_version) VALUES ($1,$2,$3,'replace',$4)`, userID, snapshotID, planID, result.ResultingVersion)
		return err
	})
	return result, err
}

func (r *PostgresRepository) saveImportedSquadTx(ctx context.Context, tx *sql.Tx, userID int64, seasonSourceID int, squad Squad) (int64, error) {
	var planID, seasonDBID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM seasons WHERE source_id=$1`, seasonSourceID).Scan(&seasonDBID); err != nil {
		return 0, err
	}
	err := tx.QueryRowContext(ctx, `SELECT id FROM squad_plans WHERE season_id=$1 AND user_id=$2 ORDER BY id LIMIT 1`, seasonDBID, userID).Scan(&planID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `INSERT INTO squad_plans (name,budget,season_id,user_id) VALUES ($1,$2,$3,$4) RETURNING id`, squad.Name, squad.Budget, seasonDBID, userID).Scan(&planID)
	}
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE squad_plans SET name=$1,budget=$2,updated_at=NOW() WHERE id=$3`, squad.Name, squad.Budget, planID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM squad_plan_players WHERE plan_id=$1`, planID); err != nil {
		return 0, err
	}
	ids := make([]int, 0, len(squad.PurchasePrices))
	for id := range squad.PurchasePrices {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, sourceID := range ids {
		dbID, resolveErr := r.resolvePlayerID(ctx, tx, seasonSourceID, sourceID)
		if resolveErr != nil {
			return 0, resolveErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO squad_plan_players (plan_id,player_id,purchase_price) VALUES ($1,$2,$3)`, planID, dbID, squad.PurchasePrices[sourceID]); err != nil {
			return 0, err
		}
	}
	starts, err := r.resolvePlayerIDs(ctx, tx, seasonSourceID, squad.StartingPlayerIDs)
	if err != nil {
		return 0, err
	}
	bench, err := r.resolvePlayerIDs(ctx, tx, seasonSourceID, squad.BenchPlayerIDs)
	if err != nil {
		return 0, err
	}
	captain, err := r.resolveOptionalPlayerID(ctx, tx, seasonSourceID, squad.CaptainID)
	if err != nil {
		return 0, err
	}
	vice, err := r.resolveOptionalPlayerID(ctx, tx, seasonSourceID, squad.ViceCaptainID)
	if err != nil {
		return 0, err
	}
	query := `INSERT INTO squad_lineups (plan_id,starting_player_ids,bench_player_ids,captain_player_id,vice_captain_player_id,formation,updated_at) VALUES ($1,` + postgresArray(starts) + `,` + postgresArray(bench) + `,$2,$3,$4,NOW()) ON CONFLICT (plan_id) DO UPDATE SET starting_player_ids=EXCLUDED.starting_player_ids,bench_player_ids=EXCLUDED.bench_player_ids,captain_player_id=EXCLUDED.captain_player_id,vice_captain_player_id=EXCLUDED.vice_captain_player_id,formation=EXCLUDED.formation,updated_at=NOW()`
	if _, err = tx.ExecContext(ctx, query, planID, nullableInt64(captain), nullableInt64(vice), nullableString(squad.Formation)); err != nil {
		return 0, err
	}
	return planID, nil
}
