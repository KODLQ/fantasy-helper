package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SourceProfile struct {
	SeasonID          int        `json:"seasonId"`
	SeasonName        string     `json:"seasonName"`
	Kind              SourceKind `json:"kind"`
	BaseLocation      string     `json:"baseLocation"`
	SupportedDatasets []string   `json:"supportedDatasets"`
	AllowLiveRefresh  bool       `json:"allowLiveRefresh"`
}

func (profile SourceProfile) Validate() error {
	if profile.SeasonID <= 0 || strings.TrimSpace(profile.SeasonName) == "" {
		return fmt.Errorf("source profile requires seasonId and seasonName")
	}
	switch profile.Kind {
	case SourceOfficialCurrent:
		if !profile.AllowLiveRefresh {
			return fmt.Errorf("official-current source profile must allow live refresh")
		}
	case SourceRetainedSnapshot, SourceHistoricalArchive:
		if profile.AllowLiveRefresh {
			return fmt.Errorf("historical source profile cannot allow live refresh")
		}
	default:
		return fmt.Errorf("unsupported source kind %q", profile.Kind)
	}
	return nil
}

type ArchivePayload struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type HistoricalArchiveManifest struct {
	Version           int                       `json:"version"`
	SeasonID          int                       `json:"seasonId"`
	SeasonName        string                    `json:"seasonName"`
	SourceKind        SourceKind                `json:"sourceKind"`
	SupportedDatasets []string                  `json:"supportedDatasets"`
	Payloads          map[string]ArchivePayload `json:"payloads"`
}

func LoadHistoricalArchiveManifest(path string) (HistoricalArchiveManifest, string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return HistoricalArchiveManifest{}, "", fmt.Errorf("read historical archive manifest: %w", err)
	}
	var manifest HistoricalArchiveManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return HistoricalArchiveManifest{}, "", fmt.Errorf("decode historical archive manifest: %w", err)
	}
	if manifest.Version != 1 {
		return HistoricalArchiveManifest{}, "", fmt.Errorf("unsupported historical archive version %d", manifest.Version)
	}
	profile := SourceProfile{SeasonID: manifest.SeasonID, SeasonName: manifest.SeasonName, Kind: manifest.SourceKind, SupportedDatasets: manifest.SupportedDatasets}
	if err := profile.Validate(); err != nil {
		return HistoricalArchiveManifest{}, "", err
	}
	if manifest.SourceKind == SourceOfficialCurrent {
		return HistoricalArchiveManifest{}, "", fmt.Errorf("historical archive cannot declare official-current source kind")
	}
	for _, required := range []string{"bootstrap-static", "fixtures"} {
		if _, ok := manifest.Payloads[required]; !ok {
			return HistoricalArchiveManifest{}, "", fmt.Errorf("historical archive is missing %s payload", required)
		}
	}
	checksum := sha256.Sum256(body)
	return manifest, hex.EncodeToString(checksum[:]), nil
}

func readArchivePayload(root, key string, payload ArchivePayload) ([]byte, error) {
	if strings.TrimSpace(payload.Path) == "" || len(payload.SHA256) != 64 {
		return nil, fmt.Errorf("archive payload %s has invalid path or checksum", key)
	}
	resolved := filepath.Clean(filepath.Join(root, payload.Path))
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("archive payload %s escapes manifest directory", key)
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("read archive payload %s: %w", key, err)
	}
	checksum := sha256.Sum256(body)
	if !strings.EqualFold(hex.EncodeToString(checksum[:]), payload.SHA256) {
		return nil, fmt.Errorf("archive payload %s checksum mismatch", key)
	}
	return body, nil
}

func ImportHistoricalArchive(ctx context.Context, manifestPath string, repository Repository) (DatasetSnapshot, error) {
	manifest, manifestChecksum, err := LoadHistoricalArchiveManifest(manifestPath)
	if err != nil {
		return DatasetSnapshot{}, err
	}
	root := filepath.Dir(manifestPath)
	bootstrapBody, err := readArchivePayload(root, "bootstrap-static", manifest.Payloads["bootstrap-static"])
	if err != nil {
		return DatasetSnapshot{}, err
	}
	fixturesBody, err := readArchivePayload(root, "fixtures", manifest.Payloads["fixtures"])
	if err != nil {
		return DatasetSnapshot{}, err
	}
	var bootstrap bootstrapResponse
	if err := json.Unmarshal(bootstrapBody, &bootstrap); err != nil {
		return DatasetSnapshot{}, fmt.Errorf("decode archive bootstrap: %w", err)
	}
	if err := validateBootstrap(bootstrap); err != nil {
		return DatasetSnapshot{}, err
	}
	if bootstrap.SeasonID != 0 && bootstrap.SeasonID != manifest.SeasonID {
		return DatasetSnapshot{}, fmt.Errorf("archive bootstrap season ID %d does not match manifest %d", bootstrap.SeasonID, manifest.SeasonID)
	}
	if bootstrap.SeasonName != "" && bootstrap.SeasonName != manifest.SeasonName {
		return DatasetSnapshot{}, fmt.Errorf("archive bootstrap season name %q does not match manifest %q", bootstrap.SeasonName, manifest.SeasonName)
	}
	var fixtures []SourceFixture
	if err := json.Unmarshal(fixturesBody, &fixtures); err != nil {
		return DatasetSnapshot{}, fmt.Errorf("decode archive fixtures: %w", err)
	}
	source := NewFPLSourceWithSeason("archive://local", manifest.SeasonID, manifest.SeasonName)
	source.Kind = manifest.SourceKind
	season, weeks, teams, players, normalizedFixtures, err := source.NormalizeSnapshot(BootstrapCatalog{SeasonID: bootstrap.SeasonID, SeasonName: bootstrap.SeasonName, TotalPlayers: bootstrap.TotalPlayers, Events: bootstrap.Events, Phases: bootstrap.Phases, Settings: bootstrap.Settings, ElementTypes: bootstrap.ElementTypes, Teams: bootstrap.Teams, Elements: bootstrap.Elements}, FixtureFeed{Fixtures: fixtures})
	if err != nil {
		return DatasetSnapshot{}, err
	}
	season.IsCurrent = false
	season.SourceKind = manifest.SourceKind
	season.LastImportedAt = time.Now().UTC()
	season.Completeness = map[string]interface{}{"supportedDatasets": manifest.SupportedDatasets}
	histories := map[int][]PlayerHistory{}
	missing := []string{}
	supported := map[string]bool{}
	for _, dataset := range manifest.SupportedDatasets {
		supported[dataset] = true
	}
	for _, dataset := range []string{"catalogue", "fixtures", "live", "player-history"} {
		if !supported[dataset] {
			missing = append(missing, "dataset:"+dataset)
		}
	}
	for _, player := range players {
		key := fmt.Sprintf("element-summary/%d", player.ID)
		payload, ok := manifest.Payloads[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		body, readErr := readArchivePayload(root, key, payload)
		if readErr != nil {
			return DatasetSnapshot{}, readErr
		}
		var summary playerSummary
		if err := json.Unmarshal(body, &summary); err != nil {
			return DatasetSnapshot{}, fmt.Errorf("decode archive %s: %w", key, err)
		}
		for _, row := range summary.History {
			histories[player.ID] = append(histories[player.ID], PlayerHistory{Gameweek: row.Round, FixtureID: row.Fixture, OpponentTeam: row.OpponentTeam, IsHome: row.IsHome, KickoffTime: row.KickoffTime, Minutes: row.Minutes, TotalPoints: row.Points, Goals: row.Goals, Assists: row.Assists, CleanSheets: row.CleanSheets, Bonus: row.Bonus, Value: float64(row.Value) / 10})
		}
	}
	sort.Strings(missing)
	season.MissingInputs = append([]string(nil), missing...)
	state := "actual"
	if len(missing) > 0 {
		state = "partial"
		season.Warnings = []string{"Historical archive does not contain every player summary."}
	}
	if err := repository.UpsertSnapshot(ctx, Snapshot{Season: season, Gameweeks: weeks, Teams: teams, Players: players, Fixtures: normalizedFixtures, Histories: histories, Checksum: manifestChecksum}); err != nil {
		return DatasetSnapshot{}, err
	}
	item := DatasetSnapshot{ID: newSnapshotID(), Dataset: "public-fpl", State: state, SeasonID: season.ID, NormalizedAt: time.Now().UTC(), SourceFetchedAt: time.Now().UTC(), NormalizerVersion: "fpl-public-v1", MissingInputs: missing, SourceKind: manifest.SourceKind, SourceVersion: fmt.Sprintf("archive-v%d", manifest.Version), SupportedDatasets: manifest.SupportedDatasets, ManifestChecksum: manifestChecksum}
	if snapshots, ok := repository.(DatasetSnapshotRepository); ok {
		if err := snapshots.CreateDatasetSnapshot(ctx, item); err != nil {
			return DatasetSnapshot{}, err
		}
	}
	return item, nil
}
