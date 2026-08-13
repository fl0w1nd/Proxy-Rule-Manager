// Package updates coordinates update executions across HTTP, schedules, and CLI.
package updates

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

const liveJobRetention = 10 * time.Minute

var ErrJobNotRunning = errors.New("update job is not running")

// ErrUpdateInProgress is returned by ReloadConfig when an update is active.
var ErrUpdateInProgress = errors.New("update in progress")

// ConflictError reports the one globally active update.
type ConflictError struct{ CurrentUpdateID string }

func (e *ConflictError) Error() string { return "update already in progress" }

// ValidationError is a stable application-level request validation error.
type ValidationError struct {
	Code    string
	Message string
	Details map[string]any
}

func (e *ValidationError) Error() string { return e.Message }

// Request defines one full or rule-scoped update execution.
type Request struct {
	Scope   string
	RuleIDs []string
}

type runner interface {
	FullUpdate(context.Context) engine.UpdateResult
	PartialUpdate(context.Context, []string) engine.UpdateResult
}

// Manager owns the single active execution and its short-lived event stream.
type Manager struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	current *Job
	runner  runner
	cfg     *config.Config
	state   *state.Store
	logger  *slog.Logger

	stopScheduler func()
}

// Job is the live, cancellable representation of one persisted update record.
type Job struct {
	ID      string
	Request Request
	Origin  string

	mu      sync.Mutex
	status  string
	done    chan struct{}
	notify  chan struct{}
	cancel  context.CancelFunc
	events  []engine.ProgressEvent
	nextSeq int64
}

// NewManager constructs a coordinator and recovers unfinished persisted tasks.
func NewManager(cfg *config.Config, st *state.Store, updateRunner runner, logger *slog.Logger) (*Manager, error) {
	m := &Manager{
		jobs: make(map[string]*Job), runner: updateRunner,
		cfg: cfg, state: st, logger: logger,
	}
	now := time.Now()
	changed := st.MarkInterruptedUpdates(now)
	changed = st.PruneUpdateHistory(time.Duration(cfg.Update.HistoryRetention), cfg.Update.HistoryLimit, now) || changed
	if changed {
		if err := st.Save(); err != nil {
			return nil, fmt.Errorf("save recovered update history: %w", err)
		}
	}
	return m, nil
}

// Start validates and starts one asynchronous update.
func (m *Manager) Start(req Request, origin string) (*Job, error) {
	job, ctx, err := m.prepare(context.Background(), req, origin)
	if err != nil {
		return nil, err
	}
	go m.execute(ctx, job)
	return job, nil
}

// Run validates and executes one update synchronously.
func (m *Manager) Run(ctx context.Context, req Request, origin string) (state.UpdateHistoryRecord, error) {
	job, runCtx, err := m.prepare(ctx, req, origin)
	if err != nil {
		return state.UpdateHistoryRecord{}, err
	}
	m.execute(runCtx, job)
	record, ok := m.state.GetUpdateHistory(job.ID)
	if !ok {
		return state.UpdateHistoryRecord{}, fmt.Errorf("completed update %s missing from history", job.ID)
	}
	return record, nil
}

func (m *Manager) prepare(parent context.Context, req Request, origin string) (*Job, context.Context, error) {
	normalized, err := m.normalize(req)
	if err != nil {
		return nil, nil, err
	}
	if origin == "" {
		origin = "web"
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil {
		return nil, nil, &ConflictError{CurrentUpdateID: m.current.ID}
	}

	now := time.Now()
	id := fmt.Sprintf("%d", now.UnixNano())
	ctx, cancel := context.WithCancel(parent)
	job := &Job{
		ID: id, Request: normalized, Origin: origin, status: "running",
		done: make(chan struct{}), notify: make(chan struct{}, 1), cancel: cancel,
	}
	record := state.UpdateHistoryRecord{
		ID: id, Origin: origin, Scope: normalized.Scope,
		RequestedRuleIDs: append([]string(nil), normalized.RuleIDs...),
		Status:           "running", StartedAt: util.FormatISO(now),
	}
	m.state.PutUpdateHistory(record, time.Duration(m.cfg.Update.HistoryRetention), m.cfg.Update.HistoryLimit, now)
	if err := m.state.Save(); err != nil {
		m.state.DeleteUpdateHistory(id)
		cancel()
		return nil, nil, fmt.Errorf("save running update: %w", err)
	}
	m.jobs[id] = job
	m.current = job
	return job, ctx, nil
}

func (m *Manager) normalize(req Request) (Request, error) {
	switch req.Scope {
	case "all":
		if len(req.RuleIDs) > 0 {
			return Request{}, &ValidationError{Code: "invalid_update_scope", Message: "全部更新不能指定规则", Details: map[string]any{}}
		}
		return Request{Scope: "all", RuleIDs: []string{}}, nil
	case "rules":
		if len(req.RuleIDs) == 0 {
			return Request{}, &ValidationError{Code: "invalid_rule_ids", Message: "规则更新至少需要一个规则 ID", Details: map[string]any{}}
		}
	default:
		return Request{}, &ValidationError{Code: "invalid_update_scope", Message: "更新范围必须是 all 或 rules", Details: map[string]any{}}
	}

	requested := make(map[string]struct{}, len(req.RuleIDs))
	for _, id := range req.RuleIDs {
		requested[id] = struct{}{}
	}
	known := make(map[string]struct{}, len(m.cfg.Rules))
	for _, rule := range m.cfg.Rules {
		known[rule.ID] = struct{}{}
	}
	var unknown []string
	for id := range requested {
		if _, ok := known[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Request{}, &ValidationError{
			Code: "invalid_rule_ids", Message: "请求包含未知规则",
			Details: map[string]any{"rule_ids": unknown},
		}
	}
	ordered := make([]string, 0, len(requested))
	for _, rule := range m.cfg.Rules {
		if _, ok := requested[rule.ID]; ok {
			ordered = append(ordered, rule.ID)
		}
	}
	return Request{Scope: "rules", RuleIDs: ordered}, nil
}

func (m *Manager) execute(ctx context.Context, job *Job) {
	ctx = engine.WithProgressReporter(ctx, job.addEvent)
	var result engine.UpdateResult
	if job.Request.Scope == "rules" {
		result = m.runner.PartialUpdate(ctx, job.Request.RuleIDs)
	} else {
		result = m.runner.FullUpdate(ctx)
	}

	status := "completed"
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		status = "cancelled"
	case len(result.Errors) > 0:
		status = "completed_with_errors"
	case len(result.Warnings) > 0:
		status = "completed_with_warnings"
	}
	finishedAt := result.EndTime
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	published, countErr := engine.CountArtifacts(m.cfg.DataDir)
	if countErr != nil {
		message := fmt.Sprintf("count published artifacts: %v", countErr)
		result.Errors = append(result.Errors, message)
		result.Issues = append(result.Issues, engine.UpdateIssue{Stage: "state", Subject: "artifacts", Message: message})
		status = "completed_with_errors"
	}

	record := state.UpdateHistoryRecord{
		ID: job.ID, Origin: job.Origin, Scope: job.Request.Scope,
		RequestedRuleIDs: append([]string(nil), job.Request.RuleIDs...),
		EffectiveRuleIDs: append([]string(nil), result.EffectiveRuleIDs...),
		Status:           status, StartedAt: util.FormatISO(result.StartTime), FinishedAt: util.FormatISO(finishedAt),
		RulesTotal: result.RulesTotal, RulesSucceeded: result.RulesSucceeded, RulesFailed: result.RulesFailed,
		ArtifactsProcessed: result.Artifacts, PublishedArtifacts: published,
		Warnings: append([]string(nil), result.Warnings...),
	}
	if result.StartTime.IsZero() {
		if started, ok := m.state.GetUpdateHistory(job.ID); ok {
			record.StartedAt = started.StartedAt
		}
	}
	for _, issue := range result.Issues {
		record.Issues = append(record.Issues, state.UpdateIssueRecord{Stage: issue.Stage, Subject: issue.Subject, Message: issue.Message})
	}
	for _, change := range result.Changes {
		stored := state.RuleChangeRecord{
			RuleID: change.RuleID, RuleName: change.RuleName,
			Added: change.Added, Removed: change.Removed,
			AddedSamples:   append([]string(nil), change.AddedSamples...),
			RemovedSamples: append([]string(nil), change.RemovedSamples...),
		}
		for _, file := range change.Files {
			stored.Files = append(stored.Files, state.ArtifactChangeRecord{ClientID: file.ClientID, Path: file.Path, Change: file.Change})
		}
		record.Changes = append(record.Changes, stored)
	}
	ruleOrder := make(map[string]int, len(m.cfg.Rules))
	for i, rule := range m.cfg.Rules {
		ruleOrder[rule.ID] = i
	}
	sort.SliceStable(record.Changes, func(i, j int) bool {
		return ruleOrder[record.Changes[i].RuleID] < ruleOrder[record.Changes[j].RuleID]
	})
	m.state.PutUpdateHistory(record, time.Duration(m.cfg.Update.HistoryRetention), m.cfg.Update.HistoryLimit, finishedAt)
	if err := m.state.Save(); err != nil && m.logger != nil {
		m.logger.Error("update history save failed", "update_id", job.ID, "error", err)
	}

	job.mu.Lock()
	job.status = status
	job.cancel = nil
	job.mu.Unlock()
	m.mu.Lock()
	if m.current == job {
		m.current = nil
	}
	m.mu.Unlock()
	close(job.done)
	time.AfterFunc(liveJobRetention, func() {
		m.mu.Lock()
		delete(m.jobs, job.ID)
		m.mu.Unlock()
	})
}

// Current returns the active live job.
func (m *Manager) Current() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// Job returns a live task while its event buffer is retained.
func (m *Manager) Job(id string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id]
}

// Cancel requests cancellation of one live task.
func (m *Manager) Cancel(id string) error {
	job := m.Job(id)
	if job == nil {
		return ErrJobNotRunning
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status != "running" && job.status != "cancelling" {
		return ErrJobNotRunning
	}
	job.status = "cancelling"
	if job.cancel != nil {
		job.cancel()
	}
	if record, ok := m.state.GetUpdateHistory(id); ok {
		record.Status = "cancelling"
		m.state.PutUpdateHistory(record, time.Duration(m.cfg.Update.HistoryRetention), m.cfg.Update.HistoryLimit, time.Now())
		_ = m.state.Save()
	}
	return nil
}

// Stop cancels the active task and waits for it to finish.
func (m *Manager) Stop(ctx context.Context) error {
	job := m.Current()
	if job == nil {
		return nil
	}
	if err := m.Cancel(job.ID); err != nil && !errors.Is(err, ErrJobNotRunning) {
		return err
	}
	select {
	case <-job.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartScheduler starts the update scheduler based on the current config.
// Call once at startup after NewManager.
func (m *Manager) StartScheduler() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopScheduler = m.startSchedulerInternal()
}

// StopScheduler stops the update scheduler if running. Safe to call once at
// shutdown; idempotent.
func (m *Manager) StopScheduler() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopScheduler != nil {
		m.stopScheduler()
		m.stopScheduler = nil
	}
}

// ReloadConfig atomically swaps the manager's config and rebuilds the
// scheduler. Returns ErrUpdateInProgress if an update is active.
func (m *Manager) ReloadConfig(newCfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil {
		return ErrUpdateInProgress
	}
	if m.stopScheduler != nil {
		m.stopScheduler()
	}
	m.cfg = newCfg
	m.stopScheduler = m.startSchedulerInternal()
	return nil
}

// startSchedulerInternal creates the scheduler for the current config. The
// returned stop function halts the goroutine; calling it more than once is
// undefined — callers track lifecycle via stopScheduler.
func (m *Manager) startSchedulerInternal() func() {
	switch m.cfg.Update.Schedule.Mode {
	case "interval":
		dur := time.Duration(m.cfg.Update.Schedule.Interval)
		if dur <= 0 {
			return func() {}
		}
		ticker := time.NewTicker(dur)
		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					m.startScheduledUpdate()
				}
			}
		}()
		return func() {
			close(done)
			ticker.Stop()
		}

	case "cron":
		loc, err := time.LoadLocation(m.cfg.Update.Schedule.Timezone)
		if err != nil {
			loc = time.UTC
		}
		c := cron.New(cron.WithLocation(loc))
		_, err = c.AddFunc(m.cfg.Update.Schedule.Cron, func() {
			m.startScheduledUpdate()
		})
		if err != nil {
			if m.logger != nil {
				m.logger.Error("invalid cron expression", "cron", m.cfg.Update.Schedule.Cron, "error", err)
			}
			return func() {}
		}
		c.Start()
		return func() {
			c.Stop()
		}

	default:
		return func() {}
	}
}

func (m *Manager) startScheduledUpdate() {
	job, err := m.Start(Request{Scope: "all"}, "scheduled")
	var conflict *ConflictError
	if errors.As(err, &conflict) {
		if m.logger != nil {
			m.logger.Info("scheduled update skipped", "current_update_id", conflict.CurrentUpdateID)
		}
		return
	}
	if err != nil {
		if m.logger != nil {
			m.logger.Error("scheduled update failed to start", "error", err)
		}
		return
	}
	if m.logger != nil {
		m.logger.Info("scheduled update started", "job_id", job.ID)
	}
}

// Snapshot returns immutable live task metadata.
func (j *Job) Snapshot() (string, Request, string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status, Request{Scope: j.Request.Scope, RuleIDs: append([]string(nil), j.Request.RuleIDs...)}, j.Origin
}

// Done closes when the execution reaches a terminal state.
func (j *Job) Done() <-chan struct{} { return j.done }

// Notify signals that progress events may be available.
func (j *Job) Notify() <-chan struct{} { return j.notify }

// EventsAfter returns progress events following a sequence number.
func (j *Job) EventsAfter(sequence int64) []engine.ProgressEvent {
	j.mu.Lock()
	defer j.mu.Unlock()
	if sequence >= j.nextSeq {
		return nil
	}
	start := 0
	for start < len(j.events) && j.events[start].Sequence <= sequence {
		start++
	}
	return append([]engine.ProgressEvent(nil), j.events[start:]...)
}

func (j *Job) addEvent(event engine.ProgressEvent) {
	j.mu.Lock()
	j.nextSeq++
	event.Sequence = j.nextSeq
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	j.events = append(j.events, event)
	j.mu.Unlock()
	select {
	case j.notify <- struct{}{}:
	default:
	}
}
