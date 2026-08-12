package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

func TestOpenAndSave(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	s.SetArtifactHash("rule1", "client1", "abc123")
	updatedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetLastCheck(updatedAt)
	s.SetRuleCheck("rule1", RuleUpdated, updatedAt, true)
	s.SetGeositeUpdate("v2fly", GeositeFailed, updatedAt)

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if h := s2.GetArtifactHash("rule1", "client1"); h != "abc123" {
		t.Errorf("hash = %q, want abc123", h)
	}
	if _, ok := s2.LastCheck(); !ok {
		t.Error("expected LastCheck to be set")
	}
	result, checkedAt, versionAt, ok := s2.RuleUpdate("rule1")
	if !ok || result != RuleUpdated || !checkedAt.Equal(updatedAt) || !versionAt.Equal(updatedAt) {
		t.Fatalf("rule update = %q, checked %v, version %v, %t", result, checkedAt, versionAt, ok)
	}
	if result, checkedAt, ok := s2.GeositeUpdate("v2fly"); !ok || result != GeositeFailed || !checkedAt.Equal(updatedAt) {
		t.Fatalf("geosite update = %q, %v, %t", result, checkedAt, ok)
	}
}

func TestFirstSuccessfulCheckEstablishesVersion(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	failedAt := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	s.SetRuleCheck("rule", RuleFailed, failedAt, false)
	_, _, versionAt, _ := s.RuleUpdate("rule")
	if !versionAt.IsZero() {
		t.Fatalf("failed first check established version %v", versionAt)
	}

	checkedAt := failedAt.Add(time.Hour)
	s.SetRuleCheck("rule", RuleUnchanged, checkedAt, false)
	result, gotCheckedAt, versionAt, ok := s.RuleUpdate("rule")
	if !ok || result != RuleUnchanged || !gotCheckedAt.Equal(checkedAt) || !versionAt.Equal(checkedAt) {
		t.Fatalf("baseline = result %q, checked %v, version %v, ok %t", result, gotCheckedAt, versionAt, ok)
	}

	nextCheck := checkedAt.Add(time.Hour)
	s.SetRuleCheck("rule", RuleUnchanged, nextCheck, false)
	_, gotCheckedAt, stableVersion, _ := s.RuleUpdate("rule")
	if !gotCheckedAt.Equal(nextCheck) || !stableVersion.Equal(checkedAt) {
		t.Fatalf("unchanged check = checked %v, version %v", gotCheckedAt, stableVersion)
	}
}

func TestSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []ir.Entry{
		{Kind: ir.KindDomain, Value: "example.com"},
		{Kind: ir.KindDomainSuffix, Value: "google.com"},
	}
	if err := s.SaveSnapshot("test-rule", entries); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	loaded, err := s.LoadSnapshot("test-rule")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("len(loaded) = %d, want 2", len(loaded))
	}
	if loaded[0].Value != "example.com" {
		t.Errorf("loaded[0].Value = %q", loaded[0].Value)
	}
}

func TestDeleteArtifactHash(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.SetArtifactHash("rule1", "client1", "abc123")
	s.DeleteArtifactHash("rule1", "client1")
	if hash := s.GetArtifactHash("rule1", "client1"); hash != "" {
		t.Fatalf("hash = %q", hash)
	}
}

func TestLoadSnapshotMissing(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := s.LoadSnapshot("nonexistent")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if entries != nil {
		t.Error("expected nil for missing snapshot")
	}
}

func TestUpdateHistoryPersistsPrunesAndReturnsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	for _, record := range []UpdateHistoryRecord{
		{ID: "expired", Status: "completed", StartedAt: now.Add(-9 * 24 * time.Hour).Format(time.RFC3339), FinishedAt: now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)},
		{ID: "first", Status: "completed", StartedAt: now.Add(-2 * time.Hour).Format(time.RFC3339), FinishedAt: now.Add(-time.Hour).Format(time.RFC3339)},
		{ID: "latest", Status: "completed_with_warnings", StartedAt: now.Add(-time.Minute).Format(time.RFC3339), FinishedAt: now.Format(time.RFC3339), Warnings: []string{"warning"}},
	} {
		s.PutUpdateHistory(record, 7*24*time.Hour, 200, now)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	history := reopened.ListUpdateHistory(7*24*time.Hour, 200, now)
	if len(history) != 2 || history[0].ID != "latest" || history[1].ID != "first" || len(history[0].Warnings) != 1 {
		t.Fatalf("history = %+v", history)
	}
}

func TestUpdateHistoryRetentionKeepsActiveAndAppliesLimit(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i, id := range []string{"one", "two", "three"} {
		finished := now.Add(time.Duration(i-3) * time.Hour)
		s.PutUpdateHistory(UpdateHistoryRecord{ID: id, Status: "completed", StartedAt: finished.Add(-time.Minute).Format(time.RFC3339), FinishedAt: finished.Format(time.RFC3339)}, 30*24*time.Hour, 2, now)
	}
	s.PutUpdateHistory(UpdateHistoryRecord{ID: "active", Status: "running", StartedAt: now.Add(-60 * 24 * time.Hour).Format(time.RFC3339)}, 7*24*time.Hour, 2, now)
	history := s.ListUpdateHistory(7*24*time.Hour, 2, now)
	if len(history) != 3 || history[0].ID != "active" || history[1].ID != "three" || history[2].ID != "two" {
		t.Fatalf("retained history = %+v", history)
	}
}

func TestMarkInterruptedUpdates(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s.PutUpdateHistory(UpdateHistoryRecord{ID: "running", Status: "running", StartedAt: now.Add(-time.Hour).Format(time.RFC3339)}, 24*time.Hour, 10, now)
	s.PutUpdateHistory(UpdateHistoryRecord{ID: "cancelling", Status: "cancelling", StartedAt: now.Add(-time.Hour).Format(time.RFC3339)}, 24*time.Hour, 10, now)
	if !s.MarkInterruptedUpdates(now) {
		t.Fatal("unfinished records were unchanged")
	}
	for _, id := range []string{"running", "cancelling"} {
		record, _ := s.GetUpdateHistory(id)
		if record.Status != "interrupted" || record.FinishedAt == "" {
			t.Fatalf("record %s = %+v", id, record)
		}
	}
}

func TestLegacyErrorHistoryIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".state")
	if err := os.MkdirAll(filepath.Join(stateDir, "snapshots"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"error_history":[{"run_id":"old","message":"secret"}],"rule_updates":{}}`
	if err := os.WriteFile(filepath.Join(stateDir, "update.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "update.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "error_history") || strings.Contains(string(data), "secret") {
		t.Fatalf("legacy state remains: %s", data)
	}
	if len(s.ListUpdateHistory(24*time.Hour, 10, time.Now())) != 0 {
		t.Fatal("legacy errors became update history")
	}
}

func TestBackfillEntryCountsFromSnapshots(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.SetRuleCheck("rule", RuleUpdated, time.Now(), true)
	if err := s.SaveSnapshot("rule", []ir.Entry{{Kind: ir.KindDomain, Value: "one.example"}, {Kind: ir.KindDomain, Value: "two.example"}}); err != nil {
		t.Fatal(err)
	}
	changed, err := s.BackfillEntryCounts([]string{"rule", "missing"})
	if err != nil || !changed {
		t.Fatalf("backfill changed=%t err=%v", changed, err)
	}
	if count, ok := s.RuleEntryCount("rule"); !ok || count != 2 {
		t.Fatalf("entry count=%d ok=%t", count, ok)
	}
	changed, err = s.BackfillEntryCounts([]string{"rule"})
	if err != nil || changed {
		t.Fatalf("second backfill changed=%t err=%v", changed, err)
	}
}
