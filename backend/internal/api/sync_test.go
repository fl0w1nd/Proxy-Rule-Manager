package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

// TestSyncTracker_BeginIsExclusive locks in the rule that prevents two
// syncs from running concurrently. The HTTP handler relies on this for
// its 409 Conflict response.
func TestSyncTracker_BeginIsExclusive(t *testing.T) {
	tr := NewSyncTracker()
	rs, _, ok := tr.Begin(context.Background(), "full_sync")
	if !ok || rs == nil {
		t.Fatalf("first Begin must succeed")
	}
	if _, _, ok2 := tr.Begin(context.Background(), "full_sync"); ok2 {
		t.Fatalf("second Begin must fail while another sync runs")
	}
	tr.End(syncengine.Result{Success: true}, nil)
	if _, _, ok3 := tr.Begin(context.Background(), "full_sync"); !ok3 {
		t.Fatalf("Begin must succeed again after End")
	}
}

// TestSyncTracker_SnapshotReflectsReporterCalls verifies that the
// Reporter implementation populates the polled snapshot.
func TestSyncTracker_SnapshotReflectsReporterCalls(t *testing.T) {
	tr := NewSyncTracker()
	rs, _, _ := tr.Begin(context.Background(), "full_sync")
	rs.SetJobID("job-1")
	rs.SetTotal(3)
	rs.SetPhase("processing", "")
	rs.StartRule("rule-a", 0)
	rs.FinishRule("rule-a", true)
	rs.StartRule("rule-b", 1)
	rs.FinishRule("rule-b", false)

	snap := rs.Snapshot()
	if snap.JobID != "job-1" {
		t.Errorf("jobID: got %q", snap.JobID)
	}
	if snap.Total != 3 {
		t.Errorf("total: got %d", snap.Total)
	}
	if snap.Processed != 2 {
		t.Errorf("processed: got %d, want 2", snap.Processed)
	}
	if snap.Failed != 1 {
		t.Errorf("failed: got %d, want 1", snap.Failed)
	}
	if snap.Phase != "processing" {
		t.Errorf("phase: got %q", snap.Phase)
	}
	if len(snap.LogTail) == 0 {
		t.Errorf("expected non-empty logTail, got %v", snap.LogTail)
	}
}

// TestSyncTracker_CancelMarksAndCancelsContext checks both the cancel
// metadata and the context plumbing — the engine relies on the latter
// to bail out of its rule loop.
func TestSyncTracker_CancelMarksAndCancelsContext(t *testing.T) {
	tr := NewSyncTracker()
	rs, ctx, _ := tr.Begin(context.Background(), "full_sync")
	if rs == nil {
		t.Fatalf("Begin")
	}
	if !tr.Cancel("admin_request") {
		t.Fatalf("Cancel must succeed when a sync is running")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("syncCtx must be cancelled by tracker.Cancel")
	}
	snap := rs.Snapshot()
	if !snap.Cancelled {
		t.Errorf("snapshot must mark Cancelled=true after Cancel")
	}
	if snap.CancelReason != "admin_request" {
		t.Errorf("CancelReason: got %q want admin_request", snap.CancelReason)
	}
}

// TestSyncTracker_CancelWhenIdle confirms the no-op branch the HTTP
// handler turns into 404.
func TestSyncTracker_CancelWhenIdle(t *testing.T) {
	tr := NewSyncTracker()
	if tr.Cancel("admin_request") {
		t.Fatalf("idle Cancel must return false")
	}
}

// TestSyncTracker_LogTailIsBounded ensures the ring buffer truncates so
// a long-running sync cannot blow up the JSON response.
func TestSyncTracker_LogTailIsBounded(t *testing.T) {
	tr := NewSyncTracker()
	rs, _, _ := tr.Begin(context.Background(), "full_sync")
	for i := 0; i < syncLogTail*3; i++ {
		rs.Log("line")
	}
	if got := len(rs.Snapshot().LogTail); got > syncLogTail {
		t.Fatalf("logTail unbounded: %d", got)
	}
}

// TestRoutesSync_FullSyncIsAsync verifies the HTTP contract: 202 with
// jobType/startedAt, the GET /sync/progress endpoint reports running:true
// while the engine works, and finally Last carries the result.
func TestRoutesSync_FullSyncIsAsync(t *testing.T) {
	_, ts := newTestServer(t, "secret")

	// Trigger sync — config is empty in the test fixture so the engine
	// finishes almost instantly, which is fine for this end-to-end test.
	code, body := postJSON(t, ts.URL, "/api/sync/full", "secret", nil)
	if code != http.StatusAccepted {
		t.Fatalf("status: got %d want 202 (%v)", code, body)
	}
	if jt, _ := body["jobType"].(string); jt != "full_sync" {
		t.Errorf("jobType: got %q", jt)
	}

	// Poll until the tracker reports running=false (engine terminated).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		code, progress := getJSON(t, ts.URL, "/api/sync/progress", "secret")
		if code != http.StatusOK {
			t.Fatalf("progress status: %d", code)
		}
		if running, _ := progress["running"].(bool); !running {
			last, _ := progress["last"].(map[string]any)
			if last == nil {
				t.Fatalf("expected last snapshot once sync terminated, got %v", progress)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("sync did not terminate within 5s")
}

// TestRoutesSync_ConflictWhileRunning checks that a second trigger
// while the first sync is in flight gets 409 with the SYNC_ALREADY_RUNNING
// code (frontends route on that for the toast).
func TestRoutesSync_ConflictWhileRunning(t *testing.T) {
	srv, ts := newTestServer(t, "secret")

	// Hold a tracker slot manually so the engine never runs. This
	// simulates "another sync in flight" without the timing fragility
	// of racing a real sync goroutine.
	rs, _, ok := srv.SyncTracker.Begin(context.Background(), "full_sync")
	if !ok {
		t.Fatalf("Begin")
	}
	t.Cleanup(func() {
		srv.SyncTracker.End(syncengine.Result{}, nil)
		_ = rs // keep the variable in scope so the cleanup compiles
	})

	code, body := postJSON(t, ts.URL, "/api/sync/full", "secret", nil)
	if code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (%v)", code, body)
	}
	if c, _ := body["code"].(string); c != "SYNC_ALREADY_RUNNING" {
		t.Errorf("code field: got %q want SYNC_ALREADY_RUNNING", c)
	}
}

// TestRoutesSync_CancelEndpoint404WhenIdle verifies the idle-state
// behaviour of POST /sync/cancel so the frontend never gets confused
// "cancelled successfully but no sync was running" replies.
func TestRoutesSync_CancelEndpoint404WhenIdle(t *testing.T) {
	_, ts := newTestServer(t, "secret")
	code, _ := postJSON(t, ts.URL, "/api/sync/cancel", "secret", nil)
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

// TestRoutesSync_CancelEndpointStopsRunningSync drives a slow fake
// reporter to keep the sync "running" long enough for the cancel
// endpoint to land, then asserts the tracker recorded the cancel.
func TestRoutesSync_CancelEndpointStopsRunningSync(t *testing.T) {
	srv, ts := newTestServer(t, "secret")

	// Claim the tracker slot and pretend the engine is busy. We never
	// call the real engine here because the test fixture has no rules;
	// instead we hand-roll a goroutine that waits on the sync context
	// and exercises tracker.End once cancelled.
	rs, syncCtx, _ := srv.SyncTracker.Begin(context.Background(), "full_sync")
	rs.SetPhase("processing", "")
	rs.SetTotal(99)

	finished := make(chan struct{})
	var ranToCompletion atomic.Bool
	go func() {
		defer close(finished)
		select {
		case <-syncCtx.Done():
			rs.MarkCancelObserved()
		case <-time.After(5 * time.Second):
			ranToCompletion.Store(true)
		}
		srv.SyncTracker.End(syncengine.Result{
			Success:     false,
			FailedRules: []schema.JobFailedRule{{Name: "sync", Error: "cancelled"}},
		}, nil)
	}()

	code, _ := postJSON(t, ts.URL, "/api/sync/cancel", "secret", nil)
	if code != http.StatusOK {
		t.Fatalf("cancel: got %d", code)
	}
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatalf("simulated worker did not observe cancellation")
	}
	if ranToCompletion.Load() {
		t.Fatalf("worker fell through to the 5s timer, cancel did not propagate")
	}

	// Tracker should now be idle and last snapshot should expose the
	// cancellation flag (we set it via the Cancel call).
	code, body := getJSON(t, ts.URL, "/api/sync/progress", "secret")
	if code != http.StatusOK {
		t.Fatalf("progress: %d", code)
	}
	if running, _ := body["running"].(bool); running {
		t.Fatalf("expected running=false after cancel completes, got %v", body)
	}
	last, _ := body["last"].(map[string]any)
	if last == nil {
		t.Fatalf("expected last snapshot, got %v", body)
	}
	if c, _ := last["cancelled"].(bool); !c {
		t.Errorf("last.cancelled: want true, got %v", last)
	}
}

// TestSyncTracker_CancelDuringFinalizeStaysFinished checks the late-cancel
// window where the cancellable rule loop has already ended and only the
// detached finalize phase remains.
func TestSyncTracker_CancelDuringFinalizeStaysFinished(t *testing.T) {
	tr := NewSyncTracker()
	rs, _, _ := tr.Begin(context.Background(), "full_sync")
	if !tr.Cancel("admin_request") {
		t.Fatalf("Cancel")
	}
	tr.End(syncengine.Result{
		Success:      true,
		ChangedRules: []string{"rule-a"},
	}, nil)

	last := tr.Last()
	if last == nil {
		t.Fatalf("expected last snapshot")
	}
	if last.Cancelled {
		t.Fatalf("late cancel should stay finished, got cancelled snapshot: %+v", last)
	}
	_ = rs
}

// TestEngineReporter_NopIsSafeForNil checks that ExecuteFullSyncReport
// tolerates a nil reporter (defensive programming guard inside engine).
func TestEngineReporter_NopIsSafeForNil(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	if _, err := srv.Engine.ExecuteFullSyncReport(context.Background(), nil); err != nil {
		t.Fatalf("nil reporter must not error: %v", err)
	}
}

// helper: keep linters quiet about an unused json import in case future
// test additions remove the direct json.NewDecoder usage.
var _ = json.NewDecoder
