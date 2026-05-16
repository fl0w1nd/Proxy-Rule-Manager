package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// TestRecoverStaleSyncState verifies the startup sweep that mops up a prior
// crash. Without this the dashboard would show "正在同步" forever after an
// SIGKILL and every new sync attempt would be rejected with "Another sync
// is already running" until the 5-min TTL on the global lock expires.
func TestRecoverStaleSyncState(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	// Seed: one running job, one already-completed job (should stay), plus
	// a sync:global lock and a rule:* lock from the simulated prior run.
	running, err := s.CreateJob(ctx, "full_sync", nil)
	if err != nil {
		t.Fatalf("create running: %v", err)
	}
	done, err := s.CreateJob(ctx, "full_sync", nil)
	if err != nil {
		t.Fatalf("create done: %v", err)
	}
	if err := s.CompleteJob(ctx, done.JobID, nil, nil); err != nil {
		t.Fatalf("complete done: %v", err)
	}
	if ok, err := s.AcquireLock(ctx, "sync:global"); err != nil || !ok {
		t.Fatalf("acquire global: ok=%v err=%v", ok, err)
	}
	if ok, err := s.AcquireLock(ctx, "rule:Demo"); err != nil || !ok {
		t.Fatalf("acquire rule: ok=%v err=%v", ok, err)
	}

	jobs, locks, err := s.RecoverStaleSyncState(ctx, "test reason")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if jobs != 1 {
		t.Fatalf("expected 1 job recovered, got %d", jobs)
	}
	if locks != 2 {
		t.Fatalf("expected 2 locks released, got %d", locks)
	}

	got, err := s.GetJob(ctx, running.JobID)
	if err != nil || got == nil {
		t.Fatalf("read recovered: %v (nil=%v)", err, got == nil)
	}
	if got.Status != "failed" {
		t.Fatalf("expected status=failed, got %q", got.Status)
	}
	if len(got.FailedRules) == 0 || got.FailedRules[0].Error != "test reason" {
		t.Fatalf("expected failure reason 'test reason', got %+v", got.FailedRules)
	}
	if got.CompletedAt == nil || *got.CompletedAt == "" {
		t.Fatalf("expected completed_at to be populated")
	}

	// Idempotent: a second invocation must be a no-op (otherwise a flaky
	// boot loop would keep overwriting valid recent failures with a stale
	// "server restarted" reason).
	jobs2, locks2, err := s.RecoverStaleSyncState(ctx, "second pass")
	if err != nil {
		t.Fatalf("recover2: %v", err)
	}
	if jobs2 != 0 || locks2 != 0 {
		t.Fatalf("expected idempotent no-op, got jobs=%d locks=%d", jobs2, locks2)
	}

	// Already-completed job is untouched.
	doneAfter, _ := s.GetJob(ctx, done.JobID)
	if doneAfter == nil || doneAfter.Status != "completed" {
		t.Fatalf("completed job must be untouched, got %+v", doneAfter)
	}
}

func newTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	paths := Paths{
		DataDir:       dir,
		RulesDir:      filepath.Join(dir, "Rules"),
		SourcesDir:    filepath.Join(dir, "sources"),
		GeositeDir:    filepath.Join(dir, "geosite"),
		IconSetDir:    filepath.Join(dir, "iconset"),
		ClientFileDir: filepath.Join(dir, "client"),
		WAFDir:        filepath.Join(dir, "waf"),
	}
	s, err := Open(filepath.Join(dir, "db.sqlite"), paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStoreDefaults(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	cfg, err := s.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.Version != 1 {
		t.Fatalf("expected version 1, got %d", cfg.Version)
	}
	clients, err := s.GetClients(ctx)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(clients) != len(schema.DefaultClients) {
		t.Fatalf("expected %d default clients, got %d", len(schema.DefaultClients), len(clients))
	}
}

// TestGetConfigDetectsCorruptedPayload mirrors a scenario where config_json
// becomes invalid (manual edit, hardware glitch, half-restored backup). The
// store must surface ErrConfigCorrupted instead of silently returning a
// default config, otherwise the next PUT /api/config would overwrite the
// damaged-but-recoverable payload.
func TestGetConfigDetectsCorruptedPayload(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	// Force-poison the persisted config_json.
	if _, err := s.DB.ExecContext(ctx, `UPDATE config SET config_json = '{invalid' WHERE id = 1`); err != nil {
		t.Fatalf("inject bad payload: %v", err)
	}
	if _, err := s.GetConfig(ctx); err == nil || !errors.Is(err, ErrConfigCorrupted) {
		t.Fatalf("expected ErrConfigCorrupted, got %v", err)
	}
}

func TestSaveConfigBumpsRev(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	rev0, err := s.GetConfigRev(ctx)
	if err != nil {
		t.Fatalf("GetConfigRev: %v", err)
	}

	cfg := schema.DefaultConfig()
	cfg.Rules = append(cfg.Rules, schema.RuleConfig{
		Name:   "rule_a",
		Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
	})
	rev1, err := s.SaveConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if rev1 <= rev0 {
		t.Fatalf("expected rev > %d, got %d", rev0, rev1)
	}

	loaded, err := s.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].Name != "rule_a" {
		t.Fatalf("expected rule_a to round-trip, got %+v", loaded.Rules)
	}
}

func TestSaveConfigWithExpectedRevDetectsConflict(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	cfg := schema.DefaultConfig()
	rev, err := s.SaveConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("initial SaveConfig: %v", err)
	}
	cfg.Rules = append(cfg.Rules, schema.RuleConfig{Name: "one", Tags: []string{}})
	if _, err := s.SaveConfigWithExpectedRev(ctx, cfg, rev-1); err == nil {
		t.Fatalf("expected stale rev conflict")
	} else if !errors.Is(err, ErrConfigConflict) {
		t.Fatalf("expected ErrConfigConflict, got %v", err)
	}
	if _, err := s.SaveConfigWithExpectedRev(ctx, cfg, rev); err != nil {
		t.Fatalf("SaveConfigWithExpectedRev current rev: %v", err)
	}
}

func TestSaveConfigConflictLeavesLocalSourceUntouched(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	content := "DOMAIN,one.com"
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name: "rule_a",
				Sources: []schema.SourceConfig{
					{Type: "local", Content: &content},
				},
				Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
				Tags:   []string{},
			},
		},
	}
	rev, err := s.SaveConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("initial SaveConfig: %v", err)
	}
	loaded, err := s.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	ref := loaded.Rules[0].Sources[0].ContentRef
	path := filepath.Join(s.SourcesDir, ref)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source before conflict: %v", err)
	}

	next := loaded
	next.Rules[0].Sources[0].Content = ptrString("DOMAIN,two.com")
	if _, err := s.SaveConfigWithExpectedRev(ctx, next, rev-1); err == nil {
		t.Fatalf("expected stale rev conflict")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source after conflict: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("local source changed on conflict: before=%q after=%q", string(before), string(after))
	}
}

func TestValidateConfigPathsRejectsUnsafeClientID(t *testing.T) {
	cfg := schema.RulesConfig{
		Version:      1,
		Transformers: map[string]schema.ScriptTransformer{},
		Rules: []schema.RuleConfig{
			{
				Name: "rule_a",
				Output: schema.OutputConfig{
					Clients: []string{"../oops"},
				},
				Tags: []string{},
			},
		},
	}
	if err := ValidateConfigPaths(cfg); err == nil {
		t.Fatalf("expected validation error for unsafe client id")
	}
}

func ptrString(s string) *string { return &s }

func TestClientCRUD(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	if err := s.AddClient(ctx, schema.ClientConfig{ID: "surge", DisplayName: "Surge"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if err := s.UpdateClient(ctx, "surge", schema.ClientConfig{ID: "surge_pro", DisplayName: "Surge Pro"}); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}
	clients, err := s.GetClients(ctx)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	found := false
	for _, c := range clients {
		if c.ID == "surge_pro" && c.DisplayName == "Surge Pro" {
			found = true
		}
	}
	if !found {
		t.Fatalf("renamed client missing: %+v", clients)
	}
	if err := s.DeleteClient(ctx, "surge_pro"); err != nil {
		t.Fatalf("DeleteClient: %v", err)
	}
}

func TestLockRoundTrip(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()
	ok, _, err := s.AcquireGlobalSyncLock(ctx)
	if err != nil || !ok {
		t.Fatalf("AcquireGlobalSyncLock: ok=%v err=%v", ok, err)
	}
	if err := s.ReleaseGlobalSyncLock(ctx); err != nil {
		t.Fatalf("ReleaseGlobalSyncLock: %v", err)
	}
}

// TestActivityGeositeFiltering locks in the asymmetric geosite policy:
//
//   - per-list `geosite_*` records (high volume) are hidden from BOTH change
//     and failure feeds, regardless of which side accidentally writes them;
//   - provider-level `geosite:*` failure records are visible (sparse, high
//     signal) so admins can spot a whole-provider outage in the activity log.
//
// This also pins the SQL-LIKE escaping fix: `geosite_%` would otherwise match
// `geosite:foo` because `_` is a LIKE wildcard, accidentally hiding provider
// failures along with the per-list noise.
func TestActivityGeositeFiltering(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	date := now[:10]

	if err := s.RecordRuleFileChanges(ctx, []ChangeRecordInput{
		{ID: "c-rule", Timestamp: now, RuleName: "ad_block", Client: "clash_meta", ChangeType: "updated"},
		{ID: "c-list", Timestamp: now, RuleName: "geosite_v2fly_cn", Client: "clash_meta", ChangeType: "updated"},
		{ID: "c-prov", Timestamp: now, RuleName: "geosite:loyalsoldier", Client: "clash_meta", ChangeType: "updated"},
	}); err != nil {
		t.Fatalf("RecordRuleFileChanges: %v", err)
	}
	if err := s.RecordFailureRecords(ctx, []schema.FailureRecord{
		{ID: "f-rule", Timestamp: now, RuleName: "ad_block", Message: "boom", Stage: "fetch"},
		{ID: "f-list", Timestamp: now, RuleName: "geosite_v2fly_cn", Message: "list err", Stage: "write_artifact"},
		{ID: "f-prov", Timestamp: now, RuleName: "geosite:loyalsoldier", Message: "404", Stage: "fetch_geosite"},
	}); err != nil {
		t.Fatalf("RecordFailureRecords: %v", err)
	}

	changes, err := s.ListChangeRecords(ctx, "", 1, 50, "", 30)
	if err != nil {
		t.Fatalf("ListChangeRecords: %v", err)
	}
	gotChange := map[string]bool{}
	for _, r := range changes.Items {
		gotChange[r.RuleName] = true
	}
	if !gotChange["ad_block"] {
		t.Errorf("change feed must contain regular rule, got %v", gotChange)
	}
	if gotChange["geosite_v2fly_cn"] || gotChange["geosite:loyalsoldier"] {
		t.Errorf("change feed must hide all geosite forms, got %v", gotChange)
	}

	failures, err := s.ListFailureRecords(ctx, "", 1, 50, "", 30)
	if err != nil {
		t.Fatalf("ListFailureRecords: %v", err)
	}
	gotFail := map[string]bool{}
	for _, r := range failures.Items {
		gotFail[r.RuleName] = true
	}
	if !gotFail["ad_block"] {
		t.Errorf("failure feed must contain regular rule, got %v", gotFail)
	}
	if gotFail["geosite_v2fly_cn"] {
		t.Errorf("failure feed must hide per-list geosite_*, got %v", gotFail)
	}
	if !gotFail["geosite:loyalsoldier"] {
		t.Errorf("failure feed MUST surface provider-level geosite:*, got %v", gotFail)
	}

	failCount, err := s.CountFailureRecords(ctx, date)
	if err != nil {
		t.Fatalf("CountFailureRecords: %v", err)
	}
	if failCount != 2 {
		t.Errorf("expected today failure count = 2 (regular + provider), got %d", failCount)
	}
	changeCount, err := s.CountChangeRecords(ctx, date)
	if err != nil {
		t.Fatalf("CountChangeRecords: %v", err)
	}
	if changeCount != 1 {
		t.Errorf("expected today change count = 1 (only ad_block), got %d", changeCount)
	}

	dates, err := s.ListActivityDates(ctx)
	if err != nil {
		t.Fatalf("ListActivityDates: %v", err)
	}
	found := false
	for _, d := range dates {
		if d == date {
			found = true
		}
	}
	if !found {
		t.Errorf("today should be present in activity dates, got %v", dates)
	}
}

// TestListFailingSourcesRanking verifies the dashboard "本周失败源 Top N"
// aggregation: results ordered by count desc / last-time desc, per-list
// geosite noise filtered out, provider-level geosite outages kept, and the
// last-error/stage of the most recent row attached to each entry.
func TestListFailingSourcesRanking(t *testing.T) {
	s := newTempStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	iso := func(offset time.Duration) string {
		return now.Add(offset).Format("2006-01-02T15:04:05.000Z")
	}

	if err := s.RecordFailureRecords(ctx, []schema.FailureRecord{
		{ID: "1", Timestamp: iso(-5 * time.Minute), RuleName: "ad_block", Message: "old", Stage: "fetch"},
		{ID: "2", Timestamp: iso(-3 * time.Minute), RuleName: "ad_block", Message: "newer", Stage: "fetch"},
		{ID: "3", Timestamp: iso(-1 * time.Minute), RuleName: "ad_block", Message: "newest", Stage: "fetch"},

		{ID: "4", Timestamp: iso(-2 * time.Minute), RuleName: "geosite:loyalsoldier", Message: "404", Stage: "fetch_geosite"},
		{ID: "5", Timestamp: iso(-30 * time.Second), RuleName: "geosite:loyalsoldier", Message: "still 404", Stage: "fetch_geosite"},

		{ID: "6", Timestamp: iso(-10 * time.Second), RuleName: "tracker_block", Message: "tls", Stage: "fetch"},

		{ID: "7", Timestamp: iso(-2 * time.Minute), RuleName: "geosite_v2fly_cn", Message: "noise", Stage: "write_artifact"},
		{ID: "8", Timestamp: iso(-1 * time.Minute), RuleName: "geosite_v2fly_cn", Message: "more noise", Stage: "write_artifact"},
	}); err != nil {
		t.Fatalf("RecordFailureRecords: %v", err)
	}

	rows, err := s.ListFailingSources(ctx, 7, 5)
	if err != nil {
		t.Fatalf("ListFailingSources: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (per-list geosite noise filtered), got %d (%+v)", len(rows), rows)
	}
	if rows[0].RuleName != "ad_block" || rows[0].Count != 3 {
		t.Errorf("rank 1 should be ad_block (3 fails), got %+v", rows[0])
	}
	if rows[0].LastMessage != "newest" {
		t.Errorf("ad_block last message should be the most recent (\"newest\"), got %q", rows[0].LastMessage)
	}
	if rows[1].RuleName != "geosite:loyalsoldier" || rows[1].Count != 2 {
		t.Errorf("rank 2 should be geosite:loyalsoldier (2 fails), got %+v", rows[1])
	}
	if rows[2].RuleName != "tracker_block" || rows[2].Count != 1 {
		t.Errorf("rank 3 should be tracker_block (1 fail), got %+v", rows[2])
	}
	for _, r := range rows {
		if r.RuleName == "geosite_v2fly_cn" {
			t.Errorf("per-list geosite must not appear: %+v", r)
		}
	}
}
