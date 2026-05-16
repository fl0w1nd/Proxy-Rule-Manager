package syncengine

import (
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func TestComputeNextSyncAt_Interval(t *testing.T) {
	base := "2025-06-01T12:00:00Z"
	sched := schema.SyncSchedule{Mode: "interval", IntervalHours: 6}
	got := ComputeNextSyncAt(sched, &base)
	want := "2025-06-01T18:00:00Z"
	if !sameInstant(got, want) {
		t.Fatalf("interval=6h: want %s got %s", want, got)
	}
}

func TestComputeNextSyncAt_Cron(t *testing.T) {
	base := "2025-06-01T12:00:00Z"
	sched := schema.SyncSchedule{Mode: "cron", CronExpression: "0 0 * * *"}
	got := ComputeNextSyncAt(sched, &base)
	want := "2025-06-02T00:00:00Z"
	if !sameInstant(got, want) {
		t.Fatalf("daily cron: want %s got %s", want, got)
	}
}

func TestComputeNextSyncAt_CronDefaultsWhenEmpty(t *testing.T) {
	base := "2025-06-01T12:00:00Z"
	sched := schema.SyncSchedule{Mode: "cron", CronExpression: ""}
	got := ComputeNextSyncAt(sched, &base)
	want := "2025-06-02T00:00:00Z"
	if !sameInstant(got, want) {
		t.Fatalf("empty cron defaults: want %s got %s", want, got)
	}
}

func TestComputeNextSyncAt_DefaultIntervalWhenInvalid(t *testing.T) {
	base := "2025-06-01T10:00:00Z"
	sched := schema.SyncSchedule{Mode: "interval", IntervalHours: 0}
	got := ComputeNextSyncAt(sched, &base)
	want := "2025-06-02T10:00:00Z" // falls back to 24h
	if !sameInstant(got, want) {
		t.Fatalf("invalid interval falls back to 24h: want %s got %s", want, got)
	}
}

func TestComputeNextSyncAt_NoBaseUsesNow(t *testing.T) {
	sched := schema.SyncSchedule{Mode: "interval", IntervalHours: 1}
	before := time.Now().UTC()
	gotStr := ComputeNextSyncAt(sched, nil)
	got, err := time.Parse(time.RFC3339Nano, gotStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	delta := got.Sub(before)
	if delta < 59*time.Minute || delta > 61*time.Minute {
		t.Fatalf("expected ~1h ahead, got %v", delta)
	}
}

func TestValidateCronExpression(t *testing.T) {
	good := []string{"0 0 * * *", "*/15 * * * *", "0 6,18 * * 1-5", "@daily"}
	for _, expr := range good {
		if err := ValidateCronExpression(expr); err != nil {
			t.Errorf("expected %q to be valid, got %v", expr, err)
		}
	}
	bad := []string{"invalid", "60 * * * *", "* * * *", ""}
	for _, expr := range bad {
		if err := ValidateCronExpression(expr); err == nil {
			t.Errorf("expected %q to be invalid", expr)
		}
	}
}

func sameInstant(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	if errA != nil || errB != nil {
		return false
	}
	return ta.Equal(tb)
}
