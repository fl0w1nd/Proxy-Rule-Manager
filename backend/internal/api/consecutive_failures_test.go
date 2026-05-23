package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

// TestSummarizeArtifacts_ConsecutiveFailures_MaxAcrossClients verifies that
// summarizeArtifacts returns the maximum consecutive_failures across all
// (rule, client) pairs for a given rule.
func TestSummarizeArtifacts_ConsecutiveFailures_MaxAcrossClients(t *testing.T) {
	ts := "2024-01-01T00:00:00Z"
	cases := []struct {
		name    string
		metas   []schema.ArtifactMeta
		wantMax int
		wantErr bool
	}{
		{
			name:    "empty",
			metas:   nil,
			wantMax: 0,
			wantErr: false,
		},
		{
			name: "single_client_no_failures",
			metas: []schema.ArtifactMeta{{
				RuleName:            "r",
				Client:              "clash_meta",
				LastUpdatedAt:       ts,
				LastAttemptStatus:   "success",
				ConsecutiveFailures: 0,
			}},
			wantMax: 0,
			wantErr: false,
		},
		{
			name: "single_client_3_failures",
			metas: []schema.ArtifactMeta{{
				RuleName:            "r",
				Client:              "clash_meta",
				LastUpdatedAt:       ts,
				LastAttemptStatus:   "failed",
				LastAttemptedAt:     ts,
				LastAttemptError:    "timeout",
				ConsecutiveFailures: 3,
			}},
			wantMax: 3,
			wantErr: true,
		},
		{
			name: "two_clients_max_is_5",
			metas: []schema.ArtifactMeta{{
				RuleName:            "r",
				Client:              "clash_meta",
				LastUpdatedAt:       ts,
				LastAttemptStatus:   "failed",
				LastAttemptedAt:     ts,
				LastAttemptError:    "err1",
				ConsecutiveFailures: 2,
			}, {
				RuleName:            "r",
				Client:              "surge",
				LastUpdatedAt:       ts,
				LastAttemptStatus:   "failed",
				LastAttemptedAt:     ts,
				LastAttemptError:    "err2",
				ConsecutiveFailures: 5,
			}},
			wantMax: 5,
			wantErr: true,
		},
		{
			name: "mixed_success_and_failure",
			metas: []schema.ArtifactMeta{{
				RuleName:            "r",
				Client:              "clash_meta",
				LastUpdatedAt:       ts,
				LastAttemptStatus:   "success",
				ConsecutiveFailures: 0,
			}, {
				RuleName:            "r",
				Client:              "surge",
				LastUpdatedAt:       ts,
				LastAttemptStatus:   "failed",
				LastAttemptedAt:     ts,
				LastAttemptError:    "err",
				ConsecutiveFailures: 1,
			}},
			wantMax: 1,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, hasError, maxCF := summarizeArtifacts(tc.metas)
			if maxCF != tc.wantMax {
				t.Errorf("maxConsecutiveFailures: got %d, want %d", maxCF, tc.wantMax)
			}
			if hasError != tc.wantErr {
				t.Errorf("hasError: got %v, want %v", hasError, tc.wantErr)
			}
		})
	}
}

// TestStatus_ConsecutiveFailures_InPayload verifies that the /api/status
// response includes the consecutiveFailures field on each rule and the
// top-level failureThreshold.
func TestStatus_ConsecutiveFailures_InPayload(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	ctx := context.Background()

	// Seed a rule.
	cfg := schema.DefaultConfig()
	cfg.Rules = []schema.RuleConfig{{
		Name:   "test-rule",
		Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
		Tags:   []string{},
	}}
	if _, err := srv.Store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Record 3 consecutive failures.
	for i := 0; i < 3; i++ {
		if err := srv.Store.RecordArtifactAttempts(ctx, []store.ArtifactAttempt{{
			RuleName:    "test-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "timeout",
		}}); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}

	// Admin payload must include consecutiveFailures and failureThreshold.
	code, body := getJSON(t, ts.URL, "/api/status", "secret")
	if code != http.StatusOK {
		t.Fatalf("admin status: %d", code)
	}
	rules, _ := body["rules"].([]any)
	if len(rules) == 0 {
		t.Fatalf("no rules in admin status")
	}
	first, _ := rules[0].(map[string]any)

	cf, _ := first["consecutiveFailures"].(float64)
	if cf != 3 {
		t.Errorf("admin: expected consecutiveFailures=3, got %v", cf)
	}
	ft, _ := body["failureThreshold"].(float64)
	if ft <= 0 {
		t.Errorf("admin: failureThreshold must be present and >0, got %v", ft)
	}

	// Public payload must also include consecutiveFailures and failureThreshold.
	code, body = getJSON(t, ts.URL, "/api/status", "")
	if code != http.StatusOK {
		t.Fatalf("public status: %d", code)
	}
	rules, _ = body["rules"].([]any)
	if len(rules) == 0 {
		t.Fatalf("no rules in public status")
	}
	first, _ = rules[0].(map[string]any)

	cf, _ = first["consecutiveFailures"].(float64)
	if cf != 3 {
		t.Errorf("public: expected consecutiveFailures=3, got %v", cf)
	}
	ft, _ = body["failureThreshold"].(float64)
	if ft <= 0 {
		t.Errorf("public: failureThreshold must be present and >0, got %v", ft)
	}
}

// TestStatus_FailureThreshold_CustomValue verifies that a custom
// failureThreshold set via system settings is reflected in the /api/status
// response.
func TestStatus_FailureThreshold_CustomValue(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	ctx := context.Background()

	// Set a custom threshold via SaveSystemSettings.
	settings := schema.DefaultSystemSettings()
	settings.Sync.FailureThreshold = 7
	if _, err := srv.Store.SaveSystemSettings(ctx, settings); err != nil {
		t.Fatalf("save system settings: %v", err)
	}

	code, body := getJSON(t, ts.URL, "/api/status", "secret")
	if code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	ft, _ := body["failureThreshold"].(float64)
	if ft != 7 {
		t.Errorf("expected failureThreshold=7, got %v", ft)
	}

	// Public endpoint too.
	code, body = getJSON(t, ts.URL, "/api/status", "")
	if code != http.StatusOK {
		t.Fatalf("public status: %d", code)
	}
	ft, _ = body["failureThreshold"].(float64)
	if ft != 7 {
		t.Errorf("public: expected failureThreshold=7, got %v", ft)
	}
}

// TestStatus_ConsecutiveFailures_Recovery verifies that after a successful
// publish (via SaveArtifactMetas), the consecutiveFailures drops to 0 in the
// status response.
func TestStatus_ConsecutiveFailures_Recovery(t *testing.T) {
	srv, ts := newTestServer(t, "secret")
	ctx := context.Background()

	cfg := schema.DefaultConfig()
	cfg.Rules = []schema.RuleConfig{{
		Name:   "recover-rule",
		Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
		Tags:   []string{},
	}}
	if _, err := srv.Store.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// 3 failures.
	for i := 0; i < 3; i++ {
		if err := srv.Store.RecordArtifactAttempts(ctx, []store.ArtifactAttempt{{
			RuleName:    "recover-rule",
			Client:      "clash_meta",
			AttemptedAt: "2024-01-01T00:00:00Z",
			Status:      "failed",
			Error:       "err",
		}}); err != nil {
			t.Fatalf("fail %d: %v", i, err)
		}
	}

	// Successful publish resets the counter.
	size := int64(100)
	if err := srv.Store.SaveArtifactMetas(ctx, []schema.ArtifactMeta{{
		RuleName:      "recover-rule",
		Client:        "clash_meta",
		LastHash:      "abc",
		LastUpdatedAt: "2024-01-01T01:00:00Z",
		BlobPath:      "/Rules/clash_meta/recover-rule.list",
		SizeBytes:     &size,
	}}); err != nil {
		t.Fatalf("SaveArtifactMetas: %v", err)
	}

	code, body := getJSON(t, ts.URL, "/api/status", "secret")
	if code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	rules, _ := body["rules"].([]any)
	first, _ := rules[0].(map[string]any)
	cf, _ := first["consecutiveFailures"].(float64)
	if cf != 0 {
		t.Errorf("after recovery, expected consecutiveFailures=0, got %v", cf)
	}
}
