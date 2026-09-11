package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ErrTransferStalled indicates no data was received for the configured duration.
var ErrTransferStalled = errors.New("transfer stalled")

// ActivityController enforces two-phase network activity deadlines:
// 1. Initial response header arrival within timeout.
// 2. Continuous body reading without stalling for longer than timeout.
type ActivityController struct {
	ctx      context.Context
	cancel   context.CancelFunc
	timer    *time.Timer
	timeout  time.Duration
	timedOut atomic.Bool
	closerMu sync.Mutex
	closer   io.Closer
}

// NewActivityController starts tracking activity for a network operation.
func NewActivityController(parent context.Context, timeout time.Duration) *ActivityController {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithCancel(parent)
	ac := &ActivityController{
		ctx:     ctx,
		cancel:  cancel,
		timeout: timeout,
	}
	ac.timer = time.AfterFunc(timeout, func() {
		ac.timedOut.Store(true)
		ac.cancel()
		ac.closerMu.Lock()
		if ac.closer != nil {
			_ = ac.closer.Close()
		}
		ac.closerMu.Unlock()
	})
	return ac
}

// Context returns the context to associate with the HTTP request.
func (ac *ActivityController) Context() context.Context {
	return ac.ctx
}

// Reset resets the inactivity timer. Should be called after receiving headers.
func (ac *ActivityController) Reset() {
	if ac.timer != nil {
		ac.timer.Reset(ac.timeout)
	}
}

// TimedOut reports whether the inactivity timer fired.
func (ac *ActivityController) TimedOut() bool {
	return ac.timedOut.Load()
}

// Reader wraps an io.Reader so each read that returns data resets the timer.
// If r implements io.Closer, it will be closed automatically when the inactivity
// timer fires to unblock pending reads.
func (ac *ActivityController) Reader(r io.Reader) io.Reader {
	if c, ok := r.(io.Closer); ok {
		ac.closerMu.Lock()
		ac.closer = c
		ac.closerMu.Unlock()
	}
	return &activityReader{
		r:  r,
		ac: ac,
	}
}

// WrapErr returns a descriptive timeout error if the inactivity timer fired,
// or original err otherwise.
func (ac *ActivityController) WrapErr(err error) error {
	if err == nil {
		return nil
	}
	if ac.timedOut.Load() {
		return fmt.Errorf("%w: no data received for %s: %w", ErrTransferStalled, ac.timeout, context.DeadlineExceeded)
	}
	return err
}

// Close stops the timer and releases resources.
func (ac *ActivityController) Close() {
	if ac.timer != nil {
		ac.timer.Stop()
	}
	ac.cancel()
}

type activityReader struct {
	r  io.Reader
	ac *ActivityController
}

func (ar *activityReader) Read(p []byte) (int, error) {
	n, err := ar.r.Read(p)
	if n > 0 && ar.ac.timer != nil {
		ar.ac.Reset()
	}
	return n, err
}
