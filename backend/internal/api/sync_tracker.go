package api

import (
	"context"
	"sync"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

// syncLogTail caps the in-memory log ring kept per running sync. Polled
// snapshots return at most this many lines, which keeps the wire payload
// small while still giving the UI enough context to debug a stuck job.
const syncLogTail = 40

// SyncTracker is a process-wide singleton that tracks the currently
// running full-sync job (if any) plus a short-lived snapshot of the most
// recent completion. It implements the dual role of "progress observer
// for the engine" and "state source for the HTTP polling endpoint" so
// the dashboard can render a determinate progress bar without polling
// the database.
//
// Only one sync may be active at a time, mirroring the DB-level global
// sync lock. Begin returns nil when another sync is already in flight,
// which routes_sync.go surfaces as 409 Conflict.
type SyncTracker struct {
	mu     sync.RWMutex
	active *RunningSync
	last   *LastSyncSnapshot
}

// NewSyncTracker constructs an empty tracker.
func NewSyncTracker() *SyncTracker {
	return &SyncTracker{}
}

// RunningSync represents one in-flight sync. It implements
// syncengine.Reporter so the engine can mutate progress fields directly.
// All public methods are safe for concurrent use; the engine writes,
// the HTTP polling endpoint reads.
type RunningSync struct {
	mu sync.RWMutex

	jobType     string
	trigger     string // "manual" or "scheduled"
	jobID       string
	startedAt   time.Time
	phase       string
	phaseDetail string
	currentRule string
	total       int
	processed   int
	failed      int
	logTail     []string

	cancel      context.CancelFunc
	cancelAt    *time.Time
	cancelledBy string
	cancelSeen  bool
}

// LastSyncSnapshot is the small payload the tracker exposes after a sync
// finishes so the dashboard can show a "just completed" toast on its
// next poll, without re-querying the jobs table.
type LastSyncSnapshot struct {
	JobID        string    `json:"jobId"`
	JobType      string    `json:"jobType"`
	Trigger      string    `json:"trigger,omitempty"`
	StartedAt    time.Time `json:"startedAt"`
	FinishedAt   time.Time `json:"finishedAt"`
	Success      bool      `json:"success"`
	Cancelled    bool      `json:"cancelled"`
	ChangedCount int       `json:"changedCount"`
	FailedCount  int       `json:"failedCount"`
	DurationMs   int64     `json:"durationMs"`
	// Error is set when the engine returned a non-nil error (lock
	// failure, DB error, etc.). Distinct from FailedCount which counts
	// per-rule failures.
	Error string `json:"error,omitempty"`
}

// Begin atomically claims the singleton slot for a new sync. It returns
// the RunningSync to wire as a Reporter, plus a child context whose
// cancellation is bound to RunningSync.Cancel(). Callers MUST invoke
// End once the sync goroutine returns, regardless of outcome.
//
// Returns (nil, parentCtx, false) when another sync is already active.
func (t *SyncTracker) Begin(parent context.Context, jobType string) (*RunningSync, context.Context, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active != nil {
		return nil, parent, false
	}
	ctx, cancel := context.WithCancel(parent)
	rs := &RunningSync{
		jobType:   jobType,
		startedAt: time.Now().UTC(),
		phase:     "starting",
		cancel:    cancel,
	}
	t.active = rs
	return rs, ctx, true
}

// End records the terminal state of a sync and clears the active slot.
// If a sync errored before Begin's child context was used, pass err to
// surface it in the snapshot.
func (t *SyncTracker) End(res syncengine.Result, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active == nil {
		return
	}
	now := time.Now().UTC()
	t.active.mu.RLock()
	jobID := t.active.jobID
	if jobID == "" {
		jobID = res.JobID
	}
	jobType := t.active.jobType
	trigger := t.active.trigger
	started := t.active.startedAt
	cancelled := t.active.cancelSeen
	t.active.mu.RUnlock()

	snap := &LastSyncSnapshot{
		JobID:        jobID,
		JobType:      jobType,
		Trigger:      trigger,
		StartedAt:    started,
		FinishedAt:   now,
		Success:      err == nil && res.Success,
		Cancelled:    cancelled,
		ChangedCount: len(res.ChangedRules),
		FailedCount:  len(res.FailedRules),
		DurationMs:   now.Sub(started).Milliseconds(),
	}
	if err != nil {
		snap.Error = err.Error()
	}
	t.last = snap
	t.active = nil
}

// Active returns the currently running sync, or nil when idle. The
// returned pointer remains valid even after End — but its state is
// frozen, so callers must take a snapshot via RunningSync.Snapshot
// rather than reading its fields.
func (t *SyncTracker) Active() *RunningSync {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active
}

// Last returns the most recent finished snapshot, or nil when no sync
// has finished since process start.
func (t *SyncTracker) Last() *LastSyncSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.last
}

// Cancel asks the currently running sync to stop. Returns false when no
// sync is active. The cancellation is best-effort: the engine reacts at
// the next per-rule checkpoint, which is usually under a second.
func (t *SyncTracker) Cancel(reason string) bool {
	t.mu.RLock()
	rs := t.active
	t.mu.RUnlock()
	if rs == nil {
		return false
	}
	rs.requestCancel(reason)
	return true
}

// requestCancel records the cancel request and triggers the context
// cancellation. Safe to call multiple times — only the first request
// is recorded.
func (rs *RunningSync) requestCancel(reason string) {
	rs.mu.Lock()
	if rs.cancelAt == nil {
		now := time.Now().UTC()
		rs.cancelAt = &now
		rs.cancelledBy = reason
		rs.logLocked("cancel requested: " + reason)
	}
	cancel := rs.cancel
	rs.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// MarkCancelObserved records that the engine has reacted to the cancel
// request and stopped the cancellable part of the sync.
func (rs *RunningSync) MarkCancelObserved() {
	rs.mu.Lock()
	rs.cancelSeen = true
	rs.logLocked("cancel observed")
	rs.mu.Unlock()
}

// SyncProgress is the JSON shape returned by GET /api/sync/progress.
// The struct is flat on purpose so the React polling code can render
// without ceremony.
type SyncProgress struct {
	Running      bool      `json:"running"`
	JobID        string    `json:"jobId,omitempty"`
	JobType      string    `json:"jobType,omitempty"`
	Trigger      string    `json:"trigger,omitempty"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
	Phase        string    `json:"phase,omitempty"`
	PhaseDetail  string    `json:"phaseDetail,omitempty"`
	CurrentRule  string    `json:"currentRule,omitempty"`
	Total        int       `json:"total"`
	Processed    int       `json:"processed"`
	Failed       int       `json:"failed"`
	ElapsedMs    int64     `json:"elapsedMs,omitempty"`
	LogTail      []string  `json:"logTail,omitempty"`
	Cancelled    bool      `json:"cancelled,omitempty"`
	CancelReason string    `json:"cancelReason,omitempty"`

	// Last carries the most recent completed snapshot so the dashboard
	// can show a "just finished" toast on the first poll after End,
	// without a separate endpoint.
	Last *LastSyncSnapshot `json:"last,omitempty"`
}

// Snapshot returns a self-consistent view of the current state. Safe to
// call concurrently with engine-side updates.
func (rs *RunningSync) Snapshot() SyncProgress {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := SyncProgress{
		Running:     true,
		JobID:       rs.jobID,
		JobType:     rs.jobType,
		Trigger:     rs.trigger,
		StartedAt:   rs.startedAt,
		Phase:       rs.phase,
		PhaseDetail: rs.phaseDetail,
		CurrentRule: rs.currentRule,
		Total:       rs.total,
		Processed:   rs.processed,
		Failed:      rs.failed,
		ElapsedMs:   time.Since(rs.startedAt).Milliseconds(),
		Cancelled:   rs.cancelAt != nil,
	}
	if rs.cancelledBy != "" {
		out.CancelReason = rs.cancelledBy
	}
	if len(rs.logTail) > 0 {
		out.LogTail = append([]string(nil), rs.logTail...)
	}
	return out
}

// --- Reporter implementation -------------------------------------------------

// SetJobID implements syncengine.Reporter.
func (rs *RunningSync) SetJobID(id string) {
	rs.mu.Lock()
	rs.jobID = id
	rs.mu.Unlock()
}

// SetTrigger records whether this sync was "manual" or "scheduled".
func (rs *RunningSync) SetTrigger(trigger string) {
	rs.mu.Lock()
	rs.trigger = trigger
	rs.mu.Unlock()
}

// SetTotal implements syncengine.Reporter.
func (rs *RunningSync) SetTotal(n int) {
	rs.mu.Lock()
	rs.total = n
	rs.mu.Unlock()
}

// SetPhase implements syncengine.Reporter.
func (rs *RunningSync) SetPhase(phase, detail string) {
	rs.mu.Lock()
	rs.phase = phase
	rs.phaseDetail = detail
	rs.logLocked("phase: " + phase)
	rs.mu.Unlock()
}

// StartRule implements syncengine.Reporter.
func (rs *RunningSync) StartRule(name string, index int) {
	rs.mu.Lock()
	rs.currentRule = name
	rs.logLocked(name)
	rs.mu.Unlock()
}

// FinishRule implements syncengine.Reporter.
func (rs *RunningSync) FinishRule(name string, ok bool) {
	rs.mu.Lock()
	rs.processed++
	if !ok {
		rs.failed++
		rs.logLocked("failed: " + name)
	}
	if rs.currentRule == name {
		rs.currentRule = ""
	}
	rs.mu.Unlock()
}

// Log implements syncengine.Reporter.
func (rs *RunningSync) Log(line string) {
	rs.mu.Lock()
	rs.logLocked(line)
	rs.mu.Unlock()
}

// logLocked appends to the log ring; caller must hold rs.mu.
func (rs *RunningSync) logLocked(line string) {
	stamped := time.Now().UTC().Format("15:04:05") + " " + line
	rs.logTail = append(rs.logTail, stamped)
	if overflow := len(rs.logTail) - syncLogTail; overflow > 0 {
		rs.logTail = rs.logTail[overflow:]
	}
}

// IdleProgress returns the "nothing running" payload, optionally
// embedding the last finished snapshot.
func (t *SyncTracker) IdleProgress() SyncProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return SyncProgress{Running: false, Last: t.last}
}

// Compile-time assertion that RunningSync satisfies the engine reporter
// interface. If the interface ever grows a method this catches it.
var _ syncengine.Reporter = (*RunningSync)(nil)
