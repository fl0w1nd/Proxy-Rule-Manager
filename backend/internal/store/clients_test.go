package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestUpdateClientRenameMovesDirectoriesAndCommitsDB exercises the happy path
// of the two-phase rename: directories move first, then the DB cascade
// commits.
func TestUpdateClientRenameMovesDirectoriesAndCommitsDB(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "old_id", DisplayName: "Old"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "old_id"), 0o755); err != nil {
		t.Fatalf("mkdir rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.RulesDir, "old_id", "Sample.list"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.ClientFileDir, "old_id"), 0o755); err != nil {
		t.Fatalf("mkdir clientfile: %v", err)
	}

	if err := s.UpdateClient(ctx, "old_id", schema.ClientConfig{ID: "new_id", DisplayName: "New"}); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}

	if _, err := os.Stat(filepath.Join(s.RulesDir, "old_id")); !os.IsNotExist(err) {
		t.Fatalf("old rules dir should be gone, stat err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(s.RulesDir, "new_id", "Sample.list")); err != nil || string(data) != "payload" {
		t.Fatalf("rule not moved to new path: data=%q err=%v", string(data), err)
	}
	if _, err := os.Stat(filepath.Join(s.ClientFileDir, "new_id")); err != nil {
		t.Fatalf("client file dir not moved: %v", err)
	}
}

// TestUpdateClient_OutputExtChange_RenamesFilesAndCascadesArtifacts walks
// through the auto-rename-on-save path: a client switches its outputExt from
// "list" to "yaml", and we expect every on-disk artifact + every artifacts
// row (blob_path, blob_url) to land on the new extension atomically.
func TestUpdateClient_OutputExtChange_RenamesFilesAndCascadesArtifacts(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "yaml_client", DisplayName: "YAML Client"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "yaml_client"), 0o755); err != nil {
		t.Fatalf("mkdir rules: %v", err)
	}
	// Drop two .list artifacts (a regular rule + a nested geosite output) so
	// we exercise the recursive walk inside renameClientArtifactFiles.
	regular := filepath.Join(s.RulesDir, "yaml_client", "Sample.list")
	if err := os.WriteFile(regular, []byte("regular"), 0o644); err != nil {
		t.Fatalf("write regular: %v", err)
	}
	geositeDir := filepath.Join(s.RulesDir, "yaml_client", "geosite", "v2fly")
	if err := os.MkdirAll(geositeDir, 0o755); err != nil {
		t.Fatalf("mkdir geosite: %v", err)
	}
	geosite := filepath.Join(geositeDir, "google.list")
	if err := os.WriteFile(geosite, []byte("geosite"), 0o644); err != nil {
		t.Fatalf("write geosite: %v", err)
	}

	// Seed artifact rows pointing at the legacy .list paths so we can
	// assert the cascade rewrote both blob_path and blob_url.
	metas := []schema.ArtifactMeta{
		{
			RuleName:      "Sample",
			Client:        "yaml_client",
			LastHash:      "h1",
			LastUpdatedAt: "t1",
			BlobPath:      "/Rules/yaml_client/Sample.list",
			BlobURL:       "/Rules/yaml_client/Sample.list",
		},
		{
			RuleName:      "geosite_v2fly_google",
			Client:        "yaml_client",
			LastHash:      "h2",
			LastUpdatedAt: "t2",
			BlobPath:      "/Rules/yaml_client/geosite/v2fly/google.list",
			BlobURL:       "/Rules/yaml_client/geosite/v2fly/google.list",
		},
	}
	if err := s.SaveArtifactMetas(ctx, metas); err != nil {
		t.Fatalf("SaveArtifactMetas: %v", err)
	}

	if err := s.UpdateClient(ctx, "yaml_client", schema.ClientConfig{ID: "yaml_client", DisplayName: "YAML Client", OutputExt: "yaml"}); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}

	// Old files are gone, new files exist with identical content.
	if _, err := os.Stat(regular); !os.IsNotExist(err) {
		t.Fatalf("Sample.list should be gone, stat err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(s.RulesDir, "yaml_client", "Sample.yaml")); err != nil || string(data) != "regular" {
		t.Fatalf("Sample.yaml content mismatch: data=%q err=%v", string(data), err)
	}
	if _, err := os.Stat(geosite); !os.IsNotExist(err) {
		t.Fatalf("google.list should be gone, stat err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(s.RulesDir, "yaml_client", "geosite", "v2fly", "google.yaml")); err != nil || string(data) != "geosite" {
		t.Fatalf("google.yaml content mismatch: data=%q err=%v", string(data), err)
	}

	// Artifact metadata was cascaded.
	updated, err := s.GetAllArtifactMetas(ctx)
	if err != nil {
		t.Fatalf("GetAllArtifactMetas: %v", err)
	}
	for _, m := range updated {
		if m.Client != "yaml_client" {
			continue
		}
		if !endsWith(m.BlobPath, ".yaml") {
			t.Errorf("BlobPath %q should end with .yaml", m.BlobPath)
		}
		if !endsWith(m.BlobURL, ".yaml") {
			t.Errorf("BlobURL %q should end with .yaml", m.BlobURL)
		}
	}

	// The client row stores the normalised value so subsequent reads round-trip.
	clients, _ := s.GetClients(ctx)
	for _, c := range clients {
		if c.ID == "yaml_client" && c.ResolvedOutputExt() != "yaml" {
			t.Fatalf("ResolvedOutputExt = %q, want yaml", c.ResolvedOutputExt())
		}
	}
}

// TestUpdateClient_OutputExtChange_NoArtifactsIsNoop guards the common case
// where an admin tweaks ext on a brand-new client that hasn't synced yet.
// We just want the DB row updated; the missing directory must not error.
func TestUpdateClient_OutputExtChange_NoArtifactsIsNoop(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "fresh", DisplayName: "fresh"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := s.UpdateClient(ctx, "fresh", schema.ClientConfig{ID: "fresh", DisplayName: "fresh", OutputExt: "json"}); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}
	clients, _ := s.GetClients(ctx)
	for _, c := range clients {
		if c.ID == "fresh" && c.ResolvedOutputExt() != "json" {
			t.Fatalf("ResolvedOutputExt = %q, want json", c.ResolvedOutputExt())
		}
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// TestClient_DefaultOutputExt_PersistsAsEmpty ensures the DB only stores a
// single canonical representation for "use the default". Without this,
// `AddClient(..., OutputExt: "list")` and `AddClient(..., OutputExt: "")`
// would create two different rows that resolve to the same behaviour,
// making downstream filtering / migrations harder to reason about.
func TestClient_DefaultOutputExt_PersistsAsEmpty(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "explicit_list", DisplayName: "Explicit List", OutputExt: "list"}); err != nil {
		t.Fatalf("AddClient explicit list: %v", err)
	}
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "implicit_default", DisplayName: "Implicit Default"}); err != nil {
		t.Fatalf("AddClient implicit: %v", err)
	}
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "custom_yaml", DisplayName: "Custom YAML", OutputExt: "yaml"}); err != nil {
		t.Fatalf("AddClient yaml: %v", err)
	}

	// Update an existing client back to default — should also collapse to "".
	if err := s.UpdateClient(ctx, "custom_yaml", schema.ClientConfig{ID: "custom_yaml", DisplayName: "Custom YAML", OutputExt: "list"}); err != nil {
		t.Fatalf("UpdateClient back to list: %v", err)
	}

	rows, err := s.DB.QueryContext(ctx, `SELECT id, output_ext FROM clients ORDER BY id`)
	if err != nil {
		t.Fatalf("query clients: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var id, ext string
		if err := rows.Scan(&id, &ext); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = ext
	}
	for _, id := range []string{"explicit_list", "implicit_default", "custom_yaml"} {
		if got[id] != "" {
			t.Errorf("output_ext for %q = %q, want empty (canonical default)", id, got[id])
		}
	}
}

// TestUpdateClientRenameRefusesOnTargetExists ensures the pre-flight check
// surfaces a clear error and leaves both filesystem and DB untouched.
func TestUpdateClientRenameRefusesOnTargetExists(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	if err := s.AddClient(ctx, schema.ClientConfig{ID: "src", DisplayName: "src"}); err != nil {
		t.Fatalf("AddClient src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	// Pre-create the target directory to trigger the conflict.
	if err := os.MkdirAll(filepath.Join(s.RulesDir, "dst"), 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}

	if err := s.UpdateClient(ctx, "src", schema.ClientConfig{ID: "dst", DisplayName: "dst"}); err == nil {
		t.Fatalf("expected error when target rules dir already exists")
	}

	// DB still has src, not dst.
	clients, _ := s.GetClients(ctx)
	hasSrc := false
	for _, c := range clients {
		if c.ID == "dst" {
			t.Fatalf("rename should not have committed: %+v", clients)
		}
		if c.ID == "src" {
			hasSrc = true
		}
	}
	if !hasSrc {
		t.Fatalf("src client missing after failed rename: %+v", clients)
	}
}
