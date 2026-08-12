package updates

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

type resultRunner struct{ result engine.UpdateResult }

func (r resultRunner) FullUpdate(context.Context) engine.UpdateResult              { return r.result }
func (r resultRunner) PartialUpdate(context.Context, []string) engine.UpdateResult { return r.result }

type blockingRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingRunner) FullUpdate(ctx context.Context) engine.UpdateResult {
	close(r.started)
	start := time.Now()
	<-ctx.Done()
	if r.release != nil {
		<-r.release
	}
	return engine.UpdateResult{StartTime: start, EndTime: time.Now(), Errors: []string{"update cancelled"}}
}
func (r *blockingRunner) PartialUpdate(ctx context.Context, _ []string) engine.UpdateResult {
	return r.FullUpdate(ctx)
}

func managerFixture(t *testing.T, runner runner) (*Manager, *state.Store, *config.Config) {
	t.Helper()
	cfg := &config.Config{DataDir: t.TempDir(), Rules: []config.RuleConfig{{ID: "base", Name: "Base"}, {ID: "child", Name: "Child"}}}
	cfg.Defaults()
	st, err := state.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(cfg, st, runner, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return m, st, cfg
}

func TestNormalizeRuleIDsDeduplicatesAndUsesConfigOrder(t *testing.T) {
	m, _, _ := managerFixture(t, resultRunner{})
	got, err := m.normalize(Request{Scope: "rules", RuleIDs: []string{"child", "base", "child"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.RuleIDs, []string{"base", "child"}) {
		t.Fatalf("rule IDs=%v", got.RuleIDs)
	}
	_, err = m.normalize(Request{Scope: "rules", RuleIDs: []string{"z", "a"}})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Code != "invalid_rule_ids" || !reflect.DeepEqual(validation.Details["rule_ids"], []string{"a", "z"}) {
		t.Fatalf("validation=%#v err=%v", validation, err)
	}
}

func TestRunPersistsEveryOriginAndCompleteResult(t *testing.T) {
	result := engine.UpdateResult{
		StartTime: time.Now().Add(-time.Second), EndTime: time.Now(), EffectiveRuleIDs: []string{"base", "child"}, RulesTotal: 2, RulesSucceeded: 1, RulesFailed: 1, Artifacts: 3,
		Errors: []string{"failed"}, Warnings: []string{"warning"}, Issues: []engine.UpdateIssue{{Stage: "rule", Subject: "child", Message: "failed"}},
		Changes: []engine.RuleChange{
			{RuleID: "child", RuleName: "Child", Files: []engine.ArtifactChange{{ClientID: "surge", Path: "rules/surge/child.list", Change: "updated"}}},
			{RuleID: "base", RuleName: "Base", Files: []engine.ArtifactChange{{ClientID: "surge", Path: "rules/surge/base.list", Change: "updated"}}, Added: 2, Removed: 1, AddedSamples: []string{"one", "two"}, RemovedSamples: []string{"old"}},
		},
	}
	m, st, cfg := managerFixture(t, resultRunner{result: result})
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "rules", "surge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "rules", "surge", "base.list"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"web", "scheduled", "cli"} {
		record, err := m.Run(context.Background(), Request{Scope: "rules", RuleIDs: []string{"base"}}, origin)
		if err != nil {
			t.Fatal(err)
		}
		if record.Origin != origin || record.Status != "completed_with_errors" || record.PublishedArtifacts != 1 || len(record.Changes) != 2 || record.Changes[0].RuleID != "base" || record.Changes[1].RuleID != "child" || len(record.Issues) != 1 || !reflect.DeepEqual(record.EffectiveRuleIDs, []string{"base", "child"}) {
			t.Fatalf("record=%+v", record)
		}
	}
	history := st.ListUpdateHistory(time.Duration(cfg.Update.HistoryRetention), cfg.Update.HistoryLimit, time.Now())
	if len(history) != 3 {
		t.Fatalf("history=%+v", history)
	}
}

func TestConflictAndCancellationPersistLifecycle(t *testing.T) {
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	m, st, _ := managerFixture(t, runner)
	job, err := m.Start(Request{Scope: "all"}, "web")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	running, ok := st.GetUpdateHistory(job.ID)
	if !ok || running.Status != "running" {
		t.Fatalf("running=%+v ok=%t", running, ok)
	}
	_, err = m.Start(Request{Scope: "all"}, "scheduled")
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.CurrentUpdateID != job.ID {
		t.Fatalf("conflict=%#v err=%v", conflict, err)
	}
	if err := m.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	cancelling, _ := st.GetUpdateHistory(job.ID)
	if cancelling.Status != "cancelling" {
		t.Fatalf("cancelling=%+v", cancelling)
	}
	close(runner.release)
	select {
	case <-job.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancel timeout")
	}
	completed, _ := st.GetUpdateHistory(job.ID)
	if completed.Status != "cancelled" || completed.FinishedAt == "" {
		t.Fatalf("completed=%+v", completed)
	}
}

func TestNewManagerRecoversInterruptedTasks(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir()}
	cfg.Defaults()
	st, err := state.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	st.PutUpdateHistory(state.UpdateHistoryRecord{ID: "old", Status: "running", StartedAt: now.Add(-time.Hour).Format(time.RFC3339)}, time.Duration(cfg.Update.HistoryRetention), cfg.Update.HistoryLimit, now)
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(cfg, st, resultRunner{}, nil); err != nil {
		t.Fatal(err)
	}
	record, _ := st.GetUpdateHistory("old")
	if record.Status != "interrupted" || record.FinishedAt == "" {
		t.Fatalf("record=%+v", record)
	}
}

func TestJobEventsReplayAfterSequence(t *testing.T) {
	job := &Job{notify: make(chan struct{}, 1)}
	job.addEvent(engine.ProgressEvent{Message: "first"})
	job.addEvent(engine.ProgressEvent{Message: "second"})
	job.addEvent(engine.ProgressEvent{Message: "third"})
	events := job.EventsAfter(1)
	if len(events) != 2 || events[0].Sequence != 2 || events[0].Message != "second" || events[1].Sequence != 3 {
		t.Fatalf("events=%+v", events)
	}
}
