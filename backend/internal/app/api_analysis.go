package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const maxLeagueComparisonMembers = 8

func (a *API) leagueAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use GET for league analysis.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/analysis/leagues/"), "/"), "/")
	leagueID, parseErr := positiveIntPart(parts, 0)
	seasonID := parseInt(r.URL.Query().Get("seasonId"), 0)
	gameweek := parseInt(r.URL.Query().Get("gameweek"), 0)
	if parseErr != nil || len(parts) != 2 || seasonID <= 0 || gameweek <= 0 {
		writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "analysis_scope_required", "league, seasonId, gameweek, and analysis type are required.", false, nil)
		return
	}
	scope := Scope{SeasonID: seasonID, Gameweek: gameweek, Dataset: "manager-fpl"}
	freshness := service.Status(session.User.ID).Freshness
	switch parts[1] {
	case "summary":
		limit, limitErr := parseOptionalBounded(r.URL.Query().Get("memberLimit"), 50, 1, 200)
		if limitErr != nil {
			writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "invalid_member_limit", "memberLimit must be between 1 and 200.", false, nil)
			return
		}
		result, err := service.LeagueSummary(r.Context(), session.User.ID, seasonID, leagueID, gameweek, limit)
		if err != nil {
			writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "league_analysis_unavailable", "League analysis is unavailable.", true, nil)
			return
		}
		if freshness.State == "stale" && result.OutcomeState != "partial" && result.OutcomeState != "unavailable" {
			result.OutcomeState = ResolveAnalysisOutcome(result.OutcomeState, false, true)
			result.Warnings = append(result.Warnings, "Manager snapshots are stale; synchronize before relying on this analysis.")
		}
		writeEnvelopeWithWarnings(w, http.StatusOK, requestIDFrom(w), scope, freshness, append(result.MissingInputs, result.Warnings...), result)
	case "comparison":
		entryIDs, err := parseStrictEntryIDs(r.URL.Query().Get("entryIds"))
		if err != nil || len(entryIDs) > maxLeagueComparisonMembers {
			writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "invalid_entry_ids", fmt.Sprintf("entryIds must contain unique positive integers with at most %d members.", maxLeagueComparisonMembers), false, nil)
			return
		}
		limit, limitErr := parseOptionalBounded(r.URL.Query().Get("memberLimit"), maxLeagueComparisonMembers, 2, maxLeagueComparisonMembers)
		if limitErr != nil {
			writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "invalid_member_limit", fmt.Sprintf("memberLimit must be between 2 and %d.", maxLeagueComparisonMembers), false, nil)
			return
		}
		rankFrom, rankFromErr := parseOptionalBounded(r.URL.Query().Get("rankFrom"), 0, 1, 20000000)
		rankTo, rankToErr := parseOptionalBounded(r.URL.Query().Get("rankTo"), 0, 1, 20000000)
		if rankFromErr != nil || rankToErr != nil || (rankFrom > 0 && rankTo > 0 && rankFrom > rankTo) {
			writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "invalid_rank_range", "rankFrom and rankTo must be positive and ordered.", false, nil)
			return
		}
		result, err := service.CompareLeague(r.Context(), session.User.ID, seasonID, leagueID, gameweek, entryIDs, rankFrom, rankTo, limit)
		if err != nil {
			writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "comparison_unavailable", "League comparison is unavailable.", true, nil)
			return
		}
		if freshness.State == "stale" && result.OutcomeState != "partial" && result.OutcomeState != "unavailable" {
			result.OutcomeState = ResolveAnalysisOutcome(result.OutcomeState, false, true)
			result.Warnings = append(result.Warnings, "Manager snapshots are stale; synchronize before relying on this comparison.")
		}
		writeEnvelopeWithWarnings(w, http.StatusOK, requestIDFrom(w), scope, freshness, append(result.MissingInputs, result.Warnings...), result)
	default:
		writeContractError(w, http.StatusNotFound, requestIDFrom(w), "analysis_route_not_found", "League analysis type was not found.", false, nil)
	}
}

func (a *API) gameweekAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use GET for gameweek analysis.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/analysis/gameweeks/"), "/"), "/")
	gameweek, parseErr := positiveIntPart(parts, 0)
	seasonID := parseInt(r.URL.Query().Get("seasonId"), 0)
	entryID := parseInt(r.URL.Query().Get("entryId"), 0)
	rivalEntryID := parseInt(r.URL.Query().Get("rivalEntryId"), 0)
	rivalRaw := strings.TrimSpace(r.URL.Query().Get("rivalEntryId"))
	if parseErr != nil || len(parts) != 2 || parts[1] != "autopsy" || seasonID <= 0 || entryID <= 0 || (rivalRaw != "" && rivalEntryID <= 0) || rivalEntryID == entryID {
		writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "analysis_scope_required", "gameweek, seasonId, and a distinct positive entryId are required.", false, nil)
		return
	}
	result, err := service.GameweekAutopsy(r.Context(), session.User.ID, seasonID, entryID, rivalEntryID, gameweek)
	if err != nil {
		writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "autopsy_unavailable", "Gameweek autopsy is unavailable.", true, nil)
		return
	}
	freshness := service.Status(session.User.ID).Freshness
	if freshness.State == "stale" && result.OutcomeState != "partial" && result.OutcomeState != "unavailable" {
		result.OutcomeState = ResolveAnalysisOutcome(result.OutcomeState, false, true)
		result.Warnings = append(result.Warnings, "Manager snapshots are stale; synchronize before relying on this autopsy.")
	}
	writeEnvelopeWithWarnings(w, http.StatusOK, requestIDFrom(w), Scope{SeasonID: seasonID, Gameweek: gameweek, Dataset: "manager-fpl"}, freshness, append(result.MissingInputs, result.Warnings...), result)
}

func positiveIntPart(parts []string, index int) (int, error) {
	if index >= len(parts) {
		return 0, fmt.Errorf("missing path part")
	}
	value, err := strconv.Atoi(parts[index])
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid positive integer")
	}
	return value, nil
}

func parseStrictEntryIDs(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return []int{}, nil
	}
	seen := map[int]bool{}
	result := []int{}
	for _, raw := range strings.Split(value, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || id <= 0 || seen[id] {
			return nil, fmt.Errorf("invalid entry ID")
		}
		seen[id] = true
		result = append(result, id)
	}
	return result, nil
}

func parseOptionalBounded(raw string, fallback, minimum, maximum int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("value is outside the supported range")
	}
	return value, nil
}
