package store

import (
	"context"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestRecordArtifactAttempts_ConsecutiveFailures_Increment verifies that each
// failed attempt bumps the consecutive_failures counter by 1.
func TestRecordArtifactAttempts_ConsecutiveFailures_Increment(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
			RuleName:    "test-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "timeout",
		}}); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		meta, err := s.GetArtifactMeta(ctx, "test-rule", "clash_meta")
		if err != nil {
			t.Fatalf("get meta after attempt %d: %v", i, err)
		}
		if meta.ConsecutiveFailures != i {
			t.Errorf("after %d failures, expected consecutiveFailures=%d, got %d", i, i, meta.ConsecutiveFailures)
		}
		if meta.LastAttemptStatus != "failed" {
			t.Errorf("expected lastAttemptStatus=failed, got %q", meta.LastAttemptStatus)
		}
	}
}

// TestRecordArtifactAttempts_ConsecutiveFailures_ResetOnSuccess verifies that
// a successful attempt resets consecutive_failures to 0, and a subsequent
// failure starts counting from 1 again.
func TestRecordArtifactAttempts_ConsecutiveFailures_ResetOnSuccess(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// Accumulate 3 failures.
	for i := 1; i <= 3; i++ {
		if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
			RuleName:    "test-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "err",
		}}); err != nil {
			t.Fatalf("fail %d: %v", i, err)
		}
	}

	// A successful attempt resets the counter.
	if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
		RuleName:    "test-rule",
		Client:      "clash_meta",
		AttemptedAt: "2024-01-01T00:01:00Z",
		Status:      "success",
	}}); err != nil {
		t.Fatalf("success attempt: %v", err)
	}
	meta, err := s.GetArtifactMeta(ctx, "test-rule", "clash_meta")
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if meta.ConsecutiveFailures != 0 {
		t.Errorf("after success, expected consecutiveFailures=0, got %d", meta.ConsecutiveFailures)
	}

	// Next failure starts at 1 again.
	if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
		RuleName:    "test-rule",
		Client:      "clash_meta",
		AttemptedAt: "2024-01-01T00:02:00Z",
		Status:      "failed",
		Error:       "again",
	}}); err != nil {
		t.Fatalf("post-success failure: %v", err)
	}
	meta, _ = s.GetArtifactMeta(ctx, "test-rule", "clash_meta")
	if meta.ConsecutiveFailures != 1 {
		t.Errorf("after 1 failure post-recovery, expected consecutiveFailures=1, got %d", meta.ConsecutiveFailures)
	}
}

// TestSaveArtifactMetas_ResetsConsecutiveFailures verifies that
// SaveArtifactMetas (used on successful publish) always resets the counter to
// 0, even when the row previously had a high failure count.
func TestSaveArtifactMetas_ResetsConsecutiveFailures(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// Seed 4 failures.
	for i := 0; i < 4; i++ {
		if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
			RuleName:    "test-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "err",
		}}); err != nil {
			t.Fatalf("fail %d: %v", i, err)
		}
	}
	meta, _ := s.GetArtifactMeta(ctx, "test-rule", "clash_meta")
	if meta.ConsecutiveFailures != 4 {
		t.Fatalf("precondition: expected 4 failures, got %d", meta.ConsecutiveFailures)
	}

	// A successful publish via SaveArtifactMetas must reset to 0.
	size := int64(42)
	if err := s.SaveArtifactMetas(ctx, []schema.ArtifactMeta{{
		RuleName:      "test-rule",
		Client:        "clash_meta",
		LastHash:      "abc123",
		LastUpdatedAt: "2024-01-01T00:05:00Z",
		BlobPath:      "/Rules/clash_meta/test-rule.list",
		SizeBytes:     &size,
	}}); err != nil {
		t.Fatalf("SaveArtifactMetas: %v", err)
	}
	meta, _ = s.GetArtifactMeta(ctx, "test-rule", "clash_meta")
	if meta.ConsecutiveFailures != 0 {
		t.Errorf("after SaveArtifactMetas, expected consecutiveFailures=0, got %d", meta.ConsecutiveFailures)
	}
	if meta.LastAttemptStatus != "success" {
		t.Errorf("expected lastAttemptStatus=success, got %q", meta.LastAttemptStatus)
	}
}

// TestRecordArtifactAttempts_FirstAttemptFailed seeds at 1 verifies that a
// brand-new row whose first-ever touch is a failure starts at 1 (not 0).
func TestRecordArtifactAttempts_FirstAttemptFailedSeedsAt1(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
		RuleName:    "fresh-rule",
		Client:      "clash_meta",
		AttemptedAt: "2024-01-01T00:00:00Z",
		Status:      "failed",
		Error:       "first fail",
	}}); err != nil {
		t.Fatalf("first fail: %v", err)
	}
	meta, err := s.GetArtifactMeta(ctx, "fresh-rule", "clash_meta")
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if meta.ConsecutiveFailures != 1 {
		t.Errorf("first-ever failure should seed consecutiveFailures=1, got %d", meta.ConsecutiveFailures)
	}
	if meta.LastHash != "" || meta.BlobPath != "" {
		t.Errorf("placeholder fields should be empty for a never-published rule, got hash=%q path=%q", meta.LastHash, meta.BlobPath)
	}
}

// TestRecordArtifactAttempts_MultipleClientsIndependant verifies that each
// (rule, client) pair tracks consecutive_failures independently.
func TestRecordArtifactAttempts_MultipleClientsIndependent(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// Fail rule for clash_meta 3 times.
	for i := 0; i < 3; i++ {
		if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
			RuleName:    "multi-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "err",
		}}); err != nil {
			t.Fatalf("clash_meta fail %d: %v", i, err)
		}
	}
	// Fail same rule for surge only once.
	if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
		RuleName:    "multi-rule",
		Client:      "surge",
		AttemptedAt: "2024-01-01T00:00:00Z",
		Status:      "failed",
		Error:       "err",
	}}); err != nil {
		t.Fatalf("surge fail: %v", err)
	}

	cm, _ := s.GetArtifactMeta(ctx, "multi-rule", "clash_meta")
	sg, _ := s.GetArtifactMeta(ctx, "multi-rule", "surge")
	if cm.ConsecutiveFailures != 3 {
		t.Errorf("clash_meta: expected 3, got %d", cm.ConsecutiveFailures)
	}
	if sg.ConsecutiveFailures != 1 {
		t.Errorf("surge: expected 1, got %d", sg.ConsecutiveFailures)
	}
}

// TestSaveArtifactMetas_ThenRecordAttempts_Ordering verifies the actual
// engine call order: SaveArtifactMetas (successful rows) runs first, then
// RecordArtifactAttempts (failed rows). A successful row must not be
// overwritten by a later failure on the same (rule, client), and a failed row
// that was never touched by SaveArtifactMetas must still increment correctly.
func TestSaveArtifactMetas_ThenRecordAttempts_Ordering(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// Simulate one rule with two clients: one succeeds, one fails.
	size := int64(100)
	if err := s.SaveArtifactMetas(ctx, []schema.ArtifactMeta{{
		RuleName:      "order-rule",
		Client:        "clash_meta",
		LastHash:      "hash1",
		LastUpdatedAt: "2024-01-01T00:00:00Z",
		BlobPath:      "/Rules/clash_meta/order-rule.list",
		SizeBytes:     &size,
	}}); err != nil {
		t.Fatalf("SaveArtifactMetas: %v", err)
	}

	if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
		RuleName:    "order-rule",
		Client:      "surge",
		AttemptedAt: "2024-01-01T00:00:00Z",
		Status:      "failed",
		Error:       "write error",
	}}); err != nil {
		t.Fatalf("RecordArtifactAttempts: %v", err)
	}

	cm, _ := s.GetArtifactMeta(ctx, "order-rule", "clash_meta")
	sg, _ := s.GetArtifactMeta(ctx, "order-rule", "surge")

	if cm.ConsecutiveFailures != 0 {
		t.Errorf("successful client should have consecutiveFailures=0, got %d", cm.ConsecutiveFailures)
	}
	if cm.LastAttemptStatus != "success" {
		t.Errorf("successful client lastAttemptStatus should be success, got %q", cm.LastAttemptStatus)
	}
	if sg.ConsecutiveFailures != 1 {
		t.Errorf("failed client should have consecutiveFailures=1, got %d", sg.ConsecutiveFailures)
	}
	if sg.LastAttemptStatus != "failed" {
		t.Errorf("failed client lastAttemptStatus should be failed, got %q", sg.LastAttemptStatus)
	}
}

// TestRecordArtifactAttempts_StatusOtherThanFailed resets to 0, verifying
// that any non-"failed" status (e.g. "skipped") also resets the counter.
func TestRecordArtifactAttempts_StatusOtherThanFailedResetsTo0(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// 2 failures first.
	for i := 0; i < 2; i++ {
		if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
			RuleName:    "skip-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "err",
		}}); err != nil {
			t.Fatalf("fail %d: %v", i, err)
		}
	}
	// A "skipped" attempt should also reset.
	if err := s.RecordArtifactAttempts(ctx, []ArtifactAttempt{{
		RuleName:    "skip-rule",
		Client:      "clash_meta",
		AttemptedAt: "2024-01-01T00:01:00Z",
		Status:      "skipped",
	}}); err != nil {
		t.Fatalf("skipped attempt: %v", err)
	}
	meta, _ := s.GetArtifactMeta(ctx, "skip-rule", "clash_meta")
	if meta.ConsecutiveFailures != 0 {
		t.Errorf("after skipped status, expected consecutiveFailures=0, got %d", meta.ConsecutiveFailures)
	}
}
