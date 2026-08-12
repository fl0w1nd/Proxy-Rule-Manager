package engine

import (
	"context"
	"time"
)

const (
	ProgressInfo    = "info"
	ProgressSuccess = "success"
	ProgressWarning = "warning"
	ProgressError   = "error"
)

// ProgressEvent is one user-facing update event. Sequence is assigned by the
// update manager so SSE clients can resume after a connection interruption.
type ProgressEvent struct {
	Sequence int64     `json:"sequence,omitempty"`
	Time     time.Time `json:"time"`
	Kind     string    `json:"kind"`
	Stage    string    `json:"stage,omitempty"`
	Status   string    `json:"status,omitempty"`
	Current  int       `json:"current,omitempty"`
	Total    int       `json:"total,omitempty"`
	RuleID   string    `json:"rule_id,omitempty"`
	RuleName string    `json:"rule_name,omitempty"`
	Subject  string    `json:"subject,omitempty"`
	Message  string    `json:"message"`
}

// UpdateIssue is one actionable failure from an update operation.
type UpdateIssue struct {
	Stage   string `json:"stage,omitempty"`
	Subject string `json:"subject,omitempty"`
	Message string `json:"message"`
}

type progressReporter func(ProgressEvent)

type progressReporterKey struct{}

// WithProgressReporter installs a reporter for one update execution.
func WithProgressReporter(ctx context.Context, reporter func(ProgressEvent)) context.Context {
	return context.WithValue(ctx, progressReporterKey{}, progressReporter(reporter))
}

func reportProgress(ctx context.Context, event ProgressEvent) {
	reporter, _ := ctx.Value(progressReporterKey{}).(progressReporter)
	if reporter == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	reporter(event)
}
