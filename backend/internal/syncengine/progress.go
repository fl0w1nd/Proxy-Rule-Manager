package syncengine

// Reporter is the progress-notification surface the engine uses while a
// full sync runs. Implementations live in api/sync_tracker.go for the
// HTTP layer and in tests; engine code only depends on this interface so
// the engine package has no awareness of HTTP transport or in-memory
// state holders.
//
// All methods MUST be safe for concurrent calls because rule processing
// will eventually be parallelised; today the engine processes rules
// sequentially but reporters should not assume that.
type Reporter interface {
	// SetJobID records the persistent job_id assigned by store.CreateJob.
	// Reporters use it to expose the id to clients (so they can correlate
	// progress with the activity log) and to dedupe last-result toasts.
	SetJobID(id string)

	// SetTotal records the number of rules the engine is going to
	// process. The engine calls this exactly once, after dependency
	// sorting, so UIs can render a determinate progress bar.
	SetTotal(n int)

	// SetPhase records the current high-level phase. Known phases:
	//
	//   "acquire_lock"        - waiting on the global sync lock
	//   "loading_config"      - reading config/clients out of SQLite
	//   "refreshing_geosite"  - pulling geosite providers
	//   "processing"          - main per-rule loop
	//   "finalizing"          - terminal DB persistence
	//   "done"                - sync has finished; result is stable
	//
	// Detail is an optional human-readable hint for the operator.
	SetPhase(phase, detail string)

	// StartRule is called immediately before ProcessRule for the given
	// rule. Index is zero-based.
	StartRule(name string, index int)

	// FinishRule is called immediately after ProcessRule returns,
	// regardless of whether artifact persistence will succeed.
	FinishRule(name string, ok bool)

	// Log appends a single short status line (no newlines). Reporters
	// are free to drop entries beyond a fixed tail.
	Log(line string)
}

// NopReporter is the safe default for call sites that don't need
// progress observation (scheduled sync without tracker, partial syncs,
// tests). All methods are no-ops and the zero value is ready to use.
type NopReporter struct{}

// SetJobID implements Reporter.
func (NopReporter) SetJobID(string) {}

// SetTotal implements Reporter.
func (NopReporter) SetTotal(int) {}

// SetPhase implements Reporter.
func (NopReporter) SetPhase(string, string) {}

// StartRule implements Reporter.
func (NopReporter) StartRule(string, int) {}

// FinishRule implements Reporter.
func (NopReporter) FinishRule(string, bool) {}

// Log implements Reporter.
func (NopReporter) Log(string) {}
