package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type archiveTestRepository struct {
	snapshot Snapshot
	dataset  DatasetSnapshot
}

func (r *archiveTestRepository) EnsureSchema(context.Context) error { return nil }
func (r *archiveTestRepository) LoadSnapshot(context.Context) (Snapshot, bool, error) {
	return r.snapshot, r.snapshot.Season.ID > 0, nil
}
func (r *archiveTestRepository) LoadSquad(context.Context) (Squad, bool, error) {
	return Squad{}, false, nil
}
func (r *archiveTestRepository) SaveSquad(context.Context, Squad) error             { return nil }
func (r *archiveTestRepository) RecordSyncStatus(context.Context, SyncStatus) error { return nil }
func (r *archiveTestRepository) UpsertSnapshot(_ context.Context, snapshot Snapshot) error {
	r.snapshot = snapshot
	return nil
}
func (r *archiveTestRepository) ListDatasetSnapshots(context.Context, Scope) ([]DatasetSnapshot, error) {
	return []DatasetSnapshot{r.dataset}, nil
}
func (r *archiveTestRepository) CreateDatasetSnapshot(_ context.Context, item DatasetSnapshot) error {
	r.dataset = item
	return nil
}

func writeArchiveTestFile(t *testing.T, root, name, contents string) ArchivePayload {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	checksum := sha256.Sum256([]byte(contents))
	return ArchivePayload{Path: name, SHA256: hex.EncodeToString(checksum[:])}
}

func TestImportHistoricalArchiveValidatesAndNormalizes(t *testing.T) {
	root := t.TempDir()
	bootstrap := `{"season_id":2024,"season_name":"2024/25","events":[{"id":1,"name":"Gameweek 1","finished":true}],"phases":[],"game_settings":{},"element_types":[{"id":3,"singular_name":"Midfielder","plural_name":"Midfielders"}],"teams":[{"id":1,"name":"Old North","short_name":"OLD"},{"id":2,"name":"Old South","short_name":"SOU"}],"elements":[{"id":10,"first_name":"A","second_name":"Past","web_name":"Past","element_type":3,"team":1,"now_cost":70,"form":"3.0","value_form":"1.0","status":"a"}]}`
	fixtures := `[{"id":100,"event":1,"finished":true,"team_h":1,"team_a":2}]`
	summary := `{"history":[{"element":10,"round":1,"fixture":100,"opponent_team":2,"was_home":true,"minutes":90,"total_points":8,"value":70}]}`
	manifest := HistoricalArchiveManifest{Version: 1, SeasonID: 2024, SeasonName: "2024/25", SourceKind: SourceHistoricalArchive, SupportedDatasets: []string{"catalogue", "fixtures", "player-history"}, Payloads: map[string]ArchivePayload{
		"bootstrap-static":   writeArchiveTestFile(t, root, "bootstrap.json", bootstrap),
		"fixtures":           writeArchiveTestFile(t, root, "fixtures.json", fixtures),
		"element-summary/10": writeArchiveTestFile(t, root, "player-10.json", summary),
	}}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifestPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &archiveTestRepository{}
	item, err := ImportHistoricalArchive(context.Background(), manifestPath, repository)
	if err != nil {
		t.Fatal(err)
	}
	if repository.snapshot.Season.IsCurrent || repository.snapshot.Season.ID != 2024 || repository.snapshot.Season.SourceKind != SourceHistoricalArchive || repository.snapshot.Players[0].WebName != "Past" {
		t.Fatalf("unexpected archive snapshot: %#v", repository.snapshot)
	}
	if item.State != "partial" || item.ManifestChecksum == "" || item.SourceKind != SourceHistoricalArchive || len(item.MissingInputs) != 1 || item.MissingInputs[0] != "dataset:live" {
		t.Fatalf("unexpected dataset provenance: %#v", item)
	}
	second, err := ImportHistoricalArchive(context.Background(), manifestPath, repository)
	if err != nil || second.SeasonID != item.SeasonID || repository.snapshot.Season.ID != 2024 {
		t.Fatalf("repeated archive import was not idempotent: %#v err=%v", second, err)
	}
}

func TestHistoricalArchiveRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	manifest := HistoricalArchiveManifest{Version: 1, SeasonID: 2024, SeasonName: "2024/25", SourceKind: SourceHistoricalArchive, Payloads: map[string]ArchivePayload{
		"bootstrap-static": {Path: "bootstrap.json", SHA256: "0000000000000000000000000000000000000000000000000000000000000000"},
		"fixtures":         writeArchiveTestFile(t, root, "fixtures.json", `[]`),
	}}
	if err := os.WriteFile(filepath.Join(root, "bootstrap.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(manifest)
	path := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportHistoricalArchive(context.Background(), path, &archiveTestRepository{}); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestHistoricalArchiveRejectsIdentityMismatchAndUnavailablePayload(t *testing.T) {
	root := t.TempDir()
	bootstrap := writeArchiveTestFile(t, root, "bootstrap.json", `{"season_id":2023,"season_name":"2023/24","events":[],"teams":[],"elements":[]}`)
	fixtures := writeArchiveTestFile(t, root, "fixtures.json", `[]`)
	manifest := HistoricalArchiveManifest{Version: 1, SeasonID: 2024, SeasonName: "2024/25", SourceKind: SourceHistoricalArchive, SupportedDatasets: []string{"catalogue", "fixtures"}, Payloads: map[string]ArchivePayload{"bootstrap-static": bootstrap, "fixtures": fixtures}}
	body, _ := json.Marshal(manifest)
	path := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportHistoricalArchive(context.Background(), path, &archiveTestRepository{}); err == nil {
		t.Fatal("expected archive identity mismatch")
	}

	manifest.SeasonID = 2023
	manifest.SeasonName = "2023/24"
	manifest.Payloads["fixtures"] = ArchivePayload{Path: "missing.json", SHA256: fixtures.SHA256}
	body, _ = json.Marshal(manifest)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportHistoricalArchive(context.Background(), path, &archiveTestRepository{}); err == nil {
		t.Fatal("expected unavailable archive payload")
	}
}
