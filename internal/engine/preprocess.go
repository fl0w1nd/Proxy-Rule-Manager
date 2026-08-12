package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

const (
	defaultPreprocessTimeout = 5 * time.Second
	defaultPreprocessMaxOut  = 8 * 1024 * 1024
)

// PreprocessRunner executes the optional source-level JS preprocessing script.
// The script must define `function process(content)` returning transformed text.
type PreprocessRunner struct {
	mu        sync.RWMutex
	timeout   time.Duration
	maxOutput int
}

// NewPreprocessRunner constructs a runner with default limits.
func NewPreprocessRunner() *PreprocessRunner {
	return &PreprocessRunner{
		timeout:   defaultPreprocessTimeout,
		maxOutput: defaultPreprocessMaxOut,
	}
}

// Configure swaps in new limits (values <= 0 restore defaults).
func (r *PreprocessRunner) Configure(timeout time.Duration, maxOutput int) {
	if timeout <= 0 {
		timeout = defaultPreprocessTimeout
	}
	if maxOutput <= 0 {
		maxOutput = defaultPreprocessMaxOut
	}
	r.mu.Lock()
	r.timeout = timeout
	r.maxOutput = maxOutput
	r.mu.Unlock()
}

// Run executes the script against content and returns the transformed text.
func (r *PreprocessRunner) Run(script, content string) (out string, err error) {
	r.mu.RLock()
	timeout, maxOutput := r.timeout, r.maxOutput
	r.mu.RUnlock()

	rt := goja.New()
	timer := time.AfterFunc(timeout, func() {
		rt.Interrupt("preprocess timeout")
	})
	defer timer.Stop()
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("preprocess panicked: %v", rec)
		}
	}()

	if _, err := rt.RunString(script); err != nil {
		return "", fmt.Errorf("preprocess script error: %w", err)
	}
	fn, ok := goja.AssertFunction(rt.Get("process"))
	if !ok {
		return "", fmt.Errorf("preprocess script must define function process(content)")
	}
	res, err := fn(goja.Undefined(), rt.ToValue(content))
	if err != nil {
		return "", fmt.Errorf("preprocess failed: %w", err)
	}
	if res == nil || goja.IsUndefined(res) || goja.IsNull(res) {
		return "", fmt.Errorf("preprocess returned no content")
	}
	s := res.String()
	if len(s) > maxOutput {
		return "", fmt.Errorf("preprocess output too large (%d bytes > %d)", len(s), maxOutput)
	}
	return s, nil
}
