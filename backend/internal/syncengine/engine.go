package syncengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/diff"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// finalizeTimeout caps the detached cleanup writes (CompleteJob, ReleaseLock,
// RecordFailureRecords, ...) so a stuck DB doesn't pin the sync goroutine
// forever. 30s is comfortably more than any normal SQLite write needs.
const finalizeTimeout = 30 * time.Second

// finalizeCtx returns a fresh context that is NOT bound to the caller's
// request context. Terminal sync persistence must succeed even when the
// HTTP client has already disconnected; otherwise the job would stay in
// 'running' status forever and the global sync lock would hold until the
// next 5-minute TTL sweep. Callers MUST invoke the cancel func.
func finalizeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), finalizeTimeout)
}

// logFinalizeErr surfaces a persistence error to docker logs instead of
// silently dropping it. We deliberately keep these as `log.Printf` rather
// than returning the error: by the time we hit a cleanup write, the caller
// has already decided the sync is finished and there's no useful way to
// propagate the failure beyond the operator's logs.
func logFinalizeErr(op, jobID string, err error) {
	if err == nil {
		return
	}
	if jobID == "" {
		log.Printf("[sync] persist %s failed: %v", op, err)
		return
	}
	log.Printf("[sync] persist %s failed (job=%s): %v", op, jobID, err)
}

// logRuleFailures emits one log line per failed rule so operators see the
// actual cause (DNS error, HTTP status, transformer panic, ...) in
// `docker logs` instead of having to query the failure_records table.
// We truncate each message to keep one log line per failure.
func logRuleFailures(trigger string, jobID string, failed []schema.JobFailedRule) {
	const maxLen = 400
	for _, f := range failed {
		msg := f.Error
		if len(msg) > maxLen {
			msg = msg[:maxLen] + "...(truncated)"
		}
		log.Printf("[sync] %s rule failed (job=%s) %s: %s", trigger, jobID, f.Name, msg)
	}
}

// Engine orchestrates full / partial sync.
type Engine struct {
	Store       *store.Store
	Fetcher     *Fetcher
	Transformer *transformer.Engine
	Geosite     *geosite.Manager
	Processor   *Processor
	RulesDir    string
}

// NewEngine wires together the underlying pieces.
func NewEngine(st *store.Store, mgr *geosite.Manager, rulesDir string) *Engine {
	tEngine := transformer.NewEngine()
	fetcher := NewFetcher()
	return &Engine{
		Store:       st,
		Fetcher:     fetcher,
		Transformer: tEngine,
		Geosite:     mgr,
		Processor: &Processor{
			Store:       st,
			Fetcher:     fetcher,
			Transformer: tEngine,
			Geosite:     mgr,
		},
		RulesDir: rulesDir,
	}
}

// Result mirrors SyncExecutionResult.
type Result struct {
	Success      bool                   `json:"success"`
	ChangedRules []string               `json:"changedRules"`
	FailedRules  []schema.JobFailedRule `json:"failedRules"`
	JobID        string                 `json:"jobId"`
}

// ExecuteFullSync re-syncs every rule. Equivalent to
// ExecuteFullSyncReport(ctx, NopReporter{}); kept as the no-reporter entry
// point so existing callers (tests, partial-sync internals) don't need to
// know about the Reporter interface.
func (e *Engine) ExecuteFullSync(ctx context.Context) (Result, error) {
	return e.ExecuteFullSyncReport(ctx, NopReporter{})
}

// ExecuteFullSyncReport runs a full sync and pipes coarse-grained
// progress events through reporter. Pass NopReporter{} when no observer
// is needed. The reporter is invoked synchronously from the engine
// goroutine; it MUST be cheap and non-blocking.
func (e *Engine) ExecuteFullSyncReport(ctx context.Context, reporter Reporter) (Result, error) {
	if reporter == nil {
		reporter = NopReporter{}
	}
	// Track wall-clock duration so the dashboard can show the last sync duration.
	// We start before the lock attempt because the dashboard should reflect
	// user-visible latency, including time spent waiting on a lock holder.
	start := time.Now()

	reporter.SetPhase("acquire_lock", "")
	acquired, reason, err := e.Store.AcquireGlobalSyncLock(ctx)
	if err != nil {
		log.Printf("[sync] full sync: acquire lock failed: %v", err)
		return Result{}, err
	}
	if !acquired {
		log.Printf("[sync] full sync: skipped (%s)", reason)
		reporter.Log("skipped: " + reason)
		return Result{
			Success:     false,
			FailedRules: []schema.JobFailedRule{{Name: "sync", Error: reason}},
		}, nil
	}
	// Use a detached context so the lock is always released even if the
	// caller's context cancels (e.g. HTTP client disconnected mid-sync).
	// Otherwise the lock would linger until its 5-minute TTL expires and
	// every retry in the meantime would fail with "Another sync is already
	// running".
	defer func() {
		rctx, rcancel := finalizeCtx()
		defer rcancel()
		logFinalizeErr("release lock", "", e.Store.ReleaseGlobalSyncLock(rctx))
	}()

	reporter.SetPhase("loading_config", "")
	cfg, err := e.Store.GetConfig(ctx)
	if err != nil {
		log.Printf("[sync] full sync: load config failed: %v", err)
		return Result{}, err
	}
	clients, err := e.Store.GetClients(ctx)
	if err != nil {
		log.Printf("[sync] full sync: load clients failed: %v", err)
		return Result{}, err
	}

	job, err := e.Store.CreateJob(ctx, "full_sync", nil)
	if err != nil {
		log.Printf("[sync] full sync: create job failed: %v", err)
		return Result{}, err
	}
	reporter.SetJobID(job.JobID)
	reporter.Log(fmt.Sprintf("started (rules=%d clients=%d)", len(cfg.Rules), len(clients)))
	log.Printf("[sync] full sync started (job=%s rules=%d clients=%d)", job.JobID, len(cfg.Rules), len(clients))

	var (
		changedRules    []string
		failedRules     []schema.JobFailedRule
		ruleFileChanges []store.ChangeRecordInput
		failureRecords  []schema.FailureRecord
		pendingArts     []schema.ArtifactMeta
		blobWriteCount  int64
	)

	if providers := collectGeositeProviders(cfg.Rules); len(providers) > 0 {
		reporter.SetPhase("refreshing_geosite", fmt.Sprintf("%d providers", len(providers)))
		reporter.Log(fmt.Sprintf("refreshing %d geosite providers", len(providers)))
		failedProviders := refreshGeositeProviders(ctx, e.Geosite, providers)
		if len(failedProviders) > 0 {
			nowISO := util.NowISO()
			for _, f := range failedProviders {
				failedRules = append(failedRules, schema.JobFailedRule{
					Name:  "geosite:" + f.Provider,
					Error: f.Error,
				})
				// Surface provider-level outages in the activity log so
				// admins can react. Use "geosite:{provider}" as ruleName so
				// the failure-record reader keeps it (per-list noise stays
				// hidden behind the "geosite_" prefix filter).
				failureRecords = append(failureRecords, schema.FailureRecord{
					ID:        uuid.New().String(),
					Timestamp: nowISO,
					RuleName:  "geosite:" + f.Provider,
					Message:   f.Error,
					Stage:     "fetch_geosite",
					JobID:     job.JobID,
				})
			}
			fctx, fcancel := finalizeCtx()
			defer fcancel()
			logFinalizeErr("record failures", job.JobID, e.Store.RecordFailureRecords(fctx, failureRecords))
			logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, changedRules, failedRules))
			durMs := time.Since(start).Milliseconds()
			logFinalizeErr("update last sync info", job.JobID, e.Store.UpdateLastSyncInfo(fctx, schema.LastSyncInfo{
				LastFullSyncAt:     &nowISO,
				TotalRulesCount:    int64(len(cfg.Rules)),
				ChangedRulesCount:  0,
				FailedRulesCount:   int64(len(failedRules)),
				LastSyncDurationMs: &durMs,
			}, map[string]bool{
				"lastFullSyncAt":     true,
				"totalRulesCount":    true,
				"changedRulesCount":  true,
				"failedRulesCount":   true,
				"lastSyncDurationMs": true,
			}))
			logRuleFailures("full sync", job.JobID, failedRules)
			log.Printf("[sync] full sync aborted: geosite providers unavailable (job=%s failed=%d duration=%dms)",
				job.JobID, len(failedRules), durMs)
			return Result{
				Success:     false,
				FailedRules: failedRules,
				JobID:       job.JobID,
			}, nil
		}
	}

	sorted, err := TopologicalSort(cfg.Rules, false)
	if err != nil {
		fctx, fcancel := finalizeCtx()
		defer fcancel()
		failed := []schema.JobFailedRule{{Name: "sync", Error: err.Error()}}
		logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, changedRules, failed))
		log.Printf("[sync] full sync aborted: dependency sort failed (job=%s): %v", job.JobID, err)
		return Result{
			Success:     false,
			FailedRules: failed,
			JobID:       job.JobID,
		}, nil
	}

	reporter.SetTotal(len(sorted))
	reporter.SetPhase("processing", "")
	cache := NewRuleContentsCache()
	missingByProvider := map[string]map[string]struct{}{}
	var pendingAttempts []store.ArtifactAttempt
	extLookup := extByClient(clients)
	for i := range sorted {
		// Honour cancellation before each rule so a cancel issued by the
		// HTTP layer takes effect within at most one rule's runtime
		// (fetcher/transformer also respect ctx so an in-flight rule
		// aborts even faster). We emit a synthetic failure for "sync"
		// to make the cancel visible in the activity log.
		if err := ctx.Err(); err != nil {
			if obs, ok := reporter.(interface{ MarkCancelObserved() }); ok {
				obs.MarkCancelObserved()
			}
			reporter.Log("cancelled by client; stopping rule loop")
			failedRules = append(failedRules, schema.JobFailedRule{
				Name:  "sync",
				Error: "cancelled: " + err.Error(),
			})
			break
		}
		rule := &sorted[i]
		trackActivity := !schema.IsGeositeRule(rule)
		reporter.StartRule(rule.Name, i)
		res := e.Processor.ProcessRule(ctx, rule, cfg.Transformers, cfg.BuiltinParams, cache, clients)
		for _, m := range res.MissingGeositeLists {
			if m.Provider == "" || m.List == "" {
				continue
			}
			if _, ok := missingByProvider[m.Provider]; !ok {
				missingByProvider[m.Provider] = map[string]struct{}{}
			}
			missingByProvider[m.Provider][m.List] = struct{}{}
		}
		if len(res.Errors) > 0 {
			if trackActivity {
				failureRecords = append(failureRecords, buildFailureRecords(rule.Name, res.Errors, job.JobID)...)
			}
			failedRules = append(failedRules, schema.JobFailedRule{Name: rule.Name, Error: joinErrors(res.Errors)})
			pendingAttempts = append(pendingAttempts,
				attemptsForFailedRule(rule, res.Errors)...)
			reporter.FinishRule(rule.Name, false)
			continue
		}
		cache.Set(rule.Name, res.Contents, res.ClientOrder)
		ruleOk := true
		for client, content := range res.Contents {
			art, err := e.flushArtifact(ctx, rule, client, lookupExt(extLookup, client), content, trackActivity)
			if err != nil {
				ruleOk = false
				if trackActivity {
					failureRecords = append(failureRecords, schema.FailureRecord{
						ID:        uuid.New().String(),
						Timestamp: util.NowISO(),
						RuleName:  rule.Name,
						Client:    client,
						Message:   err.Error(),
						Stage:     "write_artifact",
						JobID:     job.JobID,
					})
				}
				failedRules = append(failedRules, schema.JobFailedRule{Name: rule.Name, Error: err.Error()})
				pendingAttempts = append(pendingAttempts, store.ArtifactAttempt{
					RuleName:    rule.Name,
					Client:      client,
					AttemptedAt: util.NowISO(),
					Status:      "failed",
					Error:       err.Error(),
				})
				continue
			}
			if art.Meta != nil {
				pendingArts = append(pendingArts, *art.Meta)
				if art.Wrote {
					blobWriteCount++
				}
				if art.Change != nil {
					ruleFileChanges = append(ruleFileChanges, *art.Change)
					addUnique(&changedRules, rule.Name)
				}
			}
		}
		reporter.FinishRule(rule.Name, ruleOk)
	}

	// Switch to a detached context for ALL terminal persistence so a
	// cancelled request (HTTP client disconnect, curl timeout, ...) still
	// leaves a consistent job + activity record on disk.
	reporter.SetPhase("finalizing", "")
	fctx, fcancel := finalizeCtx()
	defer fcancel()

	if err := e.Store.SaveArtifactMetas(fctx, pendingArts); err != nil {
		failedRules = append(failedRules, schema.JobFailedRule{Name: "sync", Error: "save artifact metadata: " + err.Error()})
		logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, changedRules, failedRules))
		log.Printf("[sync] full sync: save artifact metadata failed (job=%s): %v", job.JobID, err)
		return Result{Success: false, ChangedRules: changedRules, FailedRules: failedRules, JobID: job.JobID}, nil
	}
	logFinalizeErr("record artifact attempts", job.JobID, e.Store.RecordArtifactAttempts(fctx, pendingAttempts))

	failureRecords = append(failureRecords, buildGeositeStaleRecords(missingByProvider, job.JobID)...)

	today := time.Now().UTC().Format("2006-01-02")
	logFinalizeErr("increment daily stats", job.JobID, e.Store.IncrementDailyStats(fctx, today, schema.DailyStats{
		SyncCount:           1,
		BlobWriteCount:      blobWriteCount,
		RulesChanged:        int64(len(changedRules)),
		TotalRulesProcessed: int64(len(sorted)),
		FailedSources:       int64(len(failureRecords)),
	}))

	nowISO := util.NowISO()
	durMs := time.Since(start).Milliseconds()
	info := schema.LastSyncInfo{
		LastFullSyncAt:     &nowISO,
		TotalRulesCount:    int64(len(sorted)),
		ChangedRulesCount:  int64(len(changedRules)),
		FailedRulesCount:   int64(len(failedRules)),
		LastSyncDurationMs: &durMs,
	}
	present := map[string]bool{
		"lastFullSyncAt":     true,
		"totalRulesCount":    true,
		"changedRulesCount":  true,
		"failedRulesCount":   true,
		"lastSyncDurationMs": true,
	}
	if len(failedRules) == 0 {
		info.LastSuccessfulSyncAt = &nowISO
		present["lastSuccessfulSyncAt"] = true
	}
	logFinalizeErr("update last sync info", job.JobID, e.Store.UpdateLastSyncInfo(fctx, info, present))

	logFinalizeErr("record rule file changes", job.JobID, e.Store.RecordRuleFileChanges(fctx, ruleFileChanges))
	logFinalizeErr("record failures", job.JobID, e.Store.RecordFailureRecords(fctx, failureRecords))
	if err := e.Store.CompleteJob(fctx, job.JobID, changedRules, failedRules); err != nil {
		log.Printf("[sync] full sync: complete job persist failed (job=%s): %v", job.JobID, err)
		return Result{}, err
	}

	logRuleFailures("full sync", job.JobID, failedRules)
	log.Printf("[sync] full sync finished (job=%s rules=%d changed=%d failed=%d duration=%dms)",
		job.JobID, len(sorted), len(changedRules), len(failedRules), durMs)
	reporter.Log(fmt.Sprintf("finished: changed=%d failed=%d duration=%dms",
		len(changedRules), len(failedRules), durMs))
	reporter.SetPhase("done", "")

	return Result{
		Success:      len(failedRules) == 0,
		ChangedRules: changedRules,
		FailedRules:  failedRules,
		JobID:        job.JobID,
	}, nil
}

// ExecutePartialSync runs a single rule and its affected downstream rules under the global sync lock.
func (e *Engine) ExecutePartialSync(ctx context.Context, ruleName string) (Result, error) {
	return e.executeSelective(ctx, []string{ruleName}, lockModeGlobal, NopReporter{})
}

// ExecuteBatchPartialSync runs multiple rules with a global lock.
func (e *Engine) ExecuteBatchPartialSync(ctx context.Context, ruleNames []string) (Result, error) {
	return e.executeSelective(ctx, ruleNames, lockModeGlobal, NopReporter{})
}

// ExecuteBatchPartialSyncReport runs multiple rules under the global sync lock
// and pipes coarse-grained progress events through reporter. Pass NopReporter{}
// when no observer is needed. The reporter is invoked synchronously from the
// engine goroutine; it MUST be cheap and non-blocking. Mirrors
// ExecuteFullSyncReport for the partial path so the HTTP layer can wire the
// SyncTracker pill while a batch sync runs.
func (e *Engine) ExecuteBatchPartialSyncReport(ctx context.Context, ruleNames []string, reporter Reporter) (Result, error) {
	if reporter == nil {
		reporter = NopReporter{}
	}
	return e.executeSelective(ctx, ruleNames, lockModeGlobal, reporter)
}

type lockMode int

const (
	lockModeRule lockMode = iota
	lockModeGlobal
)

func (e *Engine) executeSelective(ctx context.Context, seedNames []string, mode lockMode, reporter Reporter) (Result, error) {
	if reporter == nil {
		reporter = NopReporter{}
	}
	uniqueSeeds := uniqueSlice(seedNames)
	primary := "sync"
	if len(uniqueSeeds) > 0 {
		primary = uniqueSeeds[0]
	}

	reporter.SetPhase("acquire_lock", "")
	var (
		acquired bool
		reason   string
		err      error
	)
	switch mode {
	case lockModeGlobal:
		acquired, reason, err = e.Store.AcquireGlobalSyncLock(ctx)
	case lockModeRule:
		acquired, reason, err = e.Store.AcquireRuleLock(ctx, primary)
	}
	if err != nil {
		log.Printf("[sync] partial sync: acquire lock failed (seeds=%v): %v", uniqueSeeds, err)
		return Result{}, err
	}
	if !acquired {
		log.Printf("[sync] partial sync: skipped (%s seeds=%v)", reason, uniqueSeeds)
		reporter.Log("skipped: " + reason)
		return Result{
			Success:     false,
			FailedRules: []schema.JobFailedRule{{Name: primary, Error: reason}},
		}, nil
	}
	// Detached cleanup ctx, see ExecuteFullSync for rationale.
	defer func() {
		rctx, rcancel := finalizeCtx()
		defer rcancel()
		if mode == lockModeGlobal {
			logFinalizeErr("release global lock", "", e.Store.ReleaseGlobalSyncLock(rctx))
		} else {
			logFinalizeErr("release rule lock", "", e.Store.ReleaseRuleLock(rctx, primary))
		}
	}()

	reporter.SetPhase("loading_config", "")
	cfg, err := e.Store.GetConfig(ctx)
	if err != nil {
		return Result{}, err
	}
	clients, err := e.Store.GetClients(ctx)
	if err != nil {
		return Result{}, err
	}

	ruleByName := map[string]struct{}{}
	for i := range cfg.Rules {
		ruleByName[cfg.Rules[i].Name] = struct{}{}
	}
	var missingSeeds []string
	for _, seed := range uniqueSeeds {
		if _, ok := ruleByName[seed]; !ok {
			missingSeeds = append(missingSeeds, seed)
		}
	}
	if len(missingSeeds) > 0 {
		job, err := e.Store.CreateJob(ctx, "partial_sync", uniqueSeeds)
		if err != nil {
			log.Printf("[sync] partial sync: create job failed: %v", err)
			return Result{}, err
		}
		failedRules := make([]schema.JobFailedRule, 0, len(missingSeeds))
		for _, name := range missingSeeds {
			failedRules = append(failedRules, schema.JobFailedRule{
				Name:  name,
				Error: "rule not found",
			})
		}
		fctx, fcancel := finalizeCtx()
		defer fcancel()
		logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, nil, failedRules))
		log.Printf("[sync] partial sync: unknown seeds (job=%s missing=%v)", job.JobID, missingSeeds)
		return Result{
			Success:     false,
			FailedRules: failedRules,
			JobID:       job.JobID,
		}, nil
	}

	affected := CollectAffectedRules(cfg.Rules, uniqueSeeds)
	var subset []schema.RuleConfig
	for _, r := range cfg.Rules {
		if _, ok := affected[r.Name]; ok {
			subset = append(subset, r)
		}
	}
	sorted, err := TopologicalSort(subset, true)
	if err != nil {
		return Result{}, err
	}

	affectedList := make([]string, 0, len(affected))
	for k := range affected {
		affectedList = append(affectedList, k)
	}
	job, err := e.Store.CreateJob(ctx, "partial_sync", affectedList)
	if err != nil {
		log.Printf("[sync] partial sync: create job failed: %v", err)
		return Result{}, err
	}
	reporter.SetJobID(job.JobID)
	reporter.Log(fmt.Sprintf("started (seeds=%d affected=%d)", len(uniqueSeeds), len(affectedList)))
	log.Printf("[sync] partial sync started (job=%s seeds=%v affected=%d)", job.JobID, uniqueSeeds, len(affectedList))

	// Refresh upstream geosite caches that the affected rules touch. This
	// mirrors the full-sync behaviour so a partial sync that targets a
	// geosite rule still picks up upstream catalog changes (in particular,
	// list removals which would otherwise stay invisible until the next
	// full sync). Provider refresh failures used to be silently ignored,
	// so a partial sync that depended on a broken provider would falsely
	// report success while serving stale data. We now turn each failed
	// provider into a JobFailedRule + FailureRecord and short-circuit.
	if providers := collectGeositeProviders(subset); len(providers) > 0 {
		reporter.SetPhase("refreshing_geosite", fmt.Sprintf("%d providers", len(providers)))
		reporter.Log(fmt.Sprintf("refreshing %d geosite providers", len(providers)))
		failedProviders := refreshGeositeProviders(ctx, e.Geosite, providers)
		if len(failedProviders) > 0 {
			nowISO := util.NowISO()
			var (
				failedRules    []schema.JobFailedRule
				failureRecords []schema.FailureRecord
			)
			for _, f := range failedProviders {
				failedRules = append(failedRules, schema.JobFailedRule{
					Name:  "geosite:" + f.Provider,
					Error: f.Error,
				})
				failureRecords = append(failureRecords, schema.FailureRecord{
					ID:        uuid.New().String(),
					Timestamp: nowISO,
					RuleName:  "geosite:" + f.Provider,
					Message:   f.Error,
					Stage:     "fetch_geosite",
					JobID:     job.JobID,
				})
			}
			fctx, fcancel := finalizeCtx()
			defer fcancel()
			logFinalizeErr("record failures", job.JobID, e.Store.RecordFailureRecords(fctx, failureRecords))
			logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, nil, failedRules))
			logRuleFailures("partial sync", job.JobID, failedRules)
			log.Printf("[sync] partial sync aborted: geosite providers unavailable (job=%s failed=%d)",
				job.JobID, len(failedRules))
			return Result{
				Success:     false,
				FailedRules: failedRules,
				JobID:       job.JobID,
			}, nil
		}
	}

	cache := NewRuleContentsCache()
	cfgRuleByName := map[string]*schema.RuleConfig{}
	for i := range cfg.Rules {
		cfgRuleByName[cfg.Rules[i].Name] = &cfg.Rules[i]
	}
	processed := map[string]struct{}{}
	allDeps := map[string]struct{}{}
	var collect func(string)
	collect = func(name string) {
		if _, ok := processed[name]; ok {
			return
		}
		processed[name] = struct{}{}
		rule, ok := cfgRuleByName[name]
		if !ok {
			return
		}
		for dep := range ExtractDependencies(rule) {
			if _, ok := affected[dep]; ok {
				continue
			}
			allDeps[dep] = struct{}{}
			collect(dep)
		}
	}
	for _, r := range sorted {
		collect(r.Name)
	}
	// Pre-resolve the dependency warm-up subset so we can include it in
	// the total progress count BEFORE the main loop begins. Without this
	// the pill's "X/Y" jumps backward when deps run first.
	var sortedDeps []schema.RuleConfig
	if len(allDeps) > 0 {
		var depRules []schema.RuleConfig
		for _, r := range cfg.Rules {
			if _, ok := allDeps[r.Name]; ok {
				depRules = append(depRules, r)
			}
		}
		var depSortErr error
		sortedDeps, depSortErr = TopologicalSort(depRules, true)
		if depSortErr != nil {
			failures := []schema.JobFailedRule{{Name: "sync", Error: depSortErr.Error()}}
			fctx, fcancel := finalizeCtx()
			defer fcancel()
			logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, nil, failures))
			log.Printf("[sync] partial sync: dependency sort failed (job=%s): %v", job.JobID, depSortErr)
			return Result{
				Success:     false,
				FailedRules: failures,
				JobID:       job.JobID,
			}, nil
		}
	}

	reporter.SetTotal(len(sortedDeps) + len(sorted))
	reporter.SetPhase("processing", "")

	// observeCancel records that the engine has acknowledged a cancel request
	// before bailing out of either rule loop. SyncTracker.End reads cancelSeen
	// to decide whether to render a "cancelled" toast vs a generic failure.
	observeCancel := func() {
		if obs, ok := reporter.(interface{ MarkCancelObserved() }); ok {
			obs.MarkCancelObserved()
		}
	}

	var (
		changedRules    []string
		failedRules     []schema.JobFailedRule
		ruleFileChanges []store.ChangeRecordInput
		failureRecords  []schema.FailureRecord
		pendingArts     []schema.ArtifactMeta
		blobWriteCount  int64
	)
	missingByProvider := map[string]map[string]struct{}{}
	var pendingAttempts []store.ArtifactAttempt
	extLookup := extByClient(clients)

	cancelled := false
	for i := range sortedDeps {
		// Honour cancellation before each dependency so a cancel triggered
		// during the warm-up phase actually takes effect within one rule.
		if err := ctx.Err(); err != nil {
			observeCancel()
			reporter.Log("cancelled by client; stopping dependency loop")
			failedRules = append(failedRules, schema.JobFailedRule{
				Name:  "sync",
				Error: "cancelled: " + err.Error(),
			})
			cancelled = true
			break
		}
		reporter.StartRule(sortedDeps[i].Name, i)
		result := e.Processor.ProcessRule(ctx, &sortedDeps[i], cfg.Transformers, cfg.BuiltinParams, cache, clients)
		if len(result.Errors) > 0 {
			reporter.FinishRule(sortedDeps[i].Name, false)
			failures := []schema.JobFailedRule{{Name: sortedDeps[i].Name, Error: joinErrors(result.Errors)}}
			fctx, fcancel := finalizeCtx()
			defer fcancel()
			logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, nil, failures))
			logRuleFailures("partial sync", job.JobID, failures)
			return Result{
				Success:     false,
				FailedRules: failures,
				JobID:       job.JobID,
			}, nil
		}
		if len(result.Contents) > 0 {
			cache.Set(sortedDeps[i].Name, result.Contents, result.ClientOrder)
		}
		reporter.FinishRule(sortedDeps[i].Name, true)
	}

	for i := range sorted {
		if cancelled {
			break
		}
		// Mirror the per-rule cancel check from ExecuteFullSyncReport. Without
		// this, a cancel requested mid-batch would still let ProcessRule march
		// through the remaining rules — each failing fast on ctx — and inflate
		// the failure count instead of cleanly aborting.
		if err := ctx.Err(); err != nil {
			observeCancel()
			reporter.Log("cancelled by client; stopping rule loop")
			failedRules = append(failedRules, schema.JobFailedRule{
				Name:  "sync",
				Error: "cancelled: " + err.Error(),
			})
			cancelled = true
			break
		}
		rule := &sorted[i]
		trackActivity := !schema.IsGeositeRule(rule)
		reporter.StartRule(rule.Name, len(sortedDeps)+i)
		res := e.Processor.ProcessRule(ctx, rule, cfg.Transformers, cfg.BuiltinParams, cache, clients)
		for _, m := range res.MissingGeositeLists {
			if m.Provider == "" || m.List == "" {
				continue
			}
			if _, ok := missingByProvider[m.Provider]; !ok {
				missingByProvider[m.Provider] = map[string]struct{}{}
			}
			missingByProvider[m.Provider][m.List] = struct{}{}
		}
		if len(res.Errors) > 0 {
			if trackActivity {
				failureRecords = append(failureRecords, buildFailureRecords(rule.Name, res.Errors, job.JobID)...)
			}
			failedRules = append(failedRules, schema.JobFailedRule{Name: rule.Name, Error: joinErrors(res.Errors)})
			pendingAttempts = append(pendingAttempts,
				attemptsForFailedRule(rule, res.Errors)...)
			reporter.FinishRule(rule.Name, false)
			continue
		}
		cache.Set(rule.Name, res.Contents, res.ClientOrder)
		ruleOk := true
		for client, content := range res.Contents {
			art, err := e.flushArtifact(ctx, rule, client, lookupExt(extLookup, client), content, trackActivity)
			if err != nil {
				ruleOk = false
				if trackActivity {
					failureRecords = append(failureRecords, schema.FailureRecord{
						ID:        uuid.New().String(),
						Timestamp: util.NowISO(),
						RuleName:  rule.Name,
						Client:    client,
						Message:   err.Error(),
						Stage:     "write_artifact",
						JobID:     job.JobID,
					})
				}
				failedRules = append(failedRules, schema.JobFailedRule{Name: rule.Name, Error: err.Error()})
				pendingAttempts = append(pendingAttempts, store.ArtifactAttempt{
					RuleName:    rule.Name,
					Client:      client,
					AttemptedAt: util.NowISO(),
					Status:      "failed",
					Error:       err.Error(),
				})
				continue
			}
			if art.Meta != nil {
				pendingArts = append(pendingArts, *art.Meta)
				if art.Wrote {
					blobWriteCount++
				}
				if art.Change != nil {
					ruleFileChanges = append(ruleFileChanges, *art.Change)
					addUnique(&changedRules, rule.Name)
				}
			}
		}
		reporter.FinishRule(rule.Name, ruleOk)
	}

	reporter.SetPhase("finalizing", "")
	fctx, fcancel := finalizeCtx()
	defer fcancel()

	if err := e.Store.SaveArtifactMetas(fctx, pendingArts); err != nil {
		failedRules = append(failedRules, schema.JobFailedRule{Name: "sync", Error: "save artifact metadata: " + err.Error()})
		logFinalizeErr("complete job", job.JobID, e.Store.CompleteJob(fctx, job.JobID, changedRules, failedRules))
		log.Printf("[sync] partial sync: save artifact metadata failed (job=%s): %v", job.JobID, err)
		return Result{Success: false, ChangedRules: changedRules, FailedRules: failedRules, JobID: job.JobID}, nil
	}
	logFinalizeErr("record artifact attempts", job.JobID, e.Store.RecordArtifactAttempts(fctx, pendingAttempts))
	failureRecords = append(failureRecords, buildGeositeStaleRecords(missingByProvider, job.JobID)...)
	today := time.Now().UTC().Format("2006-01-02")
	logFinalizeErr("increment daily stats", job.JobID, e.Store.IncrementDailyStats(fctx, today, schema.DailyStats{
		BlobWriteCount:      blobWriteCount,
		RulesChanged:        int64(len(changedRules)),
		TotalRulesProcessed: int64(len(sorted)),
		FailedSources:       int64(len(failureRecords)),
	}))
	nowISO := util.NowISO()
	logFinalizeErr("update last sync info", job.JobID, e.Store.UpdateLastSyncInfo(fctx, schema.LastSyncInfo{LastPartialSyncAt: &nowISO}, map[string]bool{"lastPartialSyncAt": true}))

	logFinalizeErr("record rule file changes", job.JobID, e.Store.RecordRuleFileChanges(fctx, ruleFileChanges))
	logFinalizeErr("record failures", job.JobID, e.Store.RecordFailureRecords(fctx, failureRecords))
	if err := e.Store.CompleteJob(fctx, job.JobID, changedRules, failedRules); err != nil {
		log.Printf("[sync] partial sync: complete job persist failed (job=%s): %v", job.JobID, err)
		return Result{}, err
	}

	logRuleFailures("partial sync", job.JobID, failedRules)
	log.Printf("[sync] partial sync finished (job=%s rules=%d changed=%d failed=%d)",
		job.JobID, len(sorted), len(changedRules), len(failedRules))
	reporter.Log(fmt.Sprintf("finished: rules=%d changed=%d failed=%d",
		len(sorted), len(changedRules), len(failedRules)))
	reporter.SetPhase("done", "")

	return Result{
		Success:      len(failedRules) == 0,
		ChangedRules: changedRules,
		FailedRules:  failedRules,
		JobID:        job.JobID,
	}, nil
}

type artifactFlush struct {
	Meta   *schema.ArtifactMeta
	Wrote  bool
	Change *store.ChangeRecordInput
}

// flushArtifact writes the rule output for a single (rule, client) and returns
// metadata + change record (if any).
//
// `ext` is the client-resolved output extension. Empty falls back to
// schema.DefaultOutputExt via resolveExt in ruledisk.go.
//
// As of the "zero header" rewrite, the on-disk artifact contains the exact
// post-transform content with no managed header injected. The hash and diff
// inputs are therefore the raw content directly. For backward compatibility
// with artifacts written by older versions (which prepended a "# 规则数量：…"
// block), we still pass `previousContent` through StripManagedRuleHeader
// before comparison/diff so the first re-sync after upgrade produces a clean
// change record and rewrites the file to its header-less form.
func (e *Engine) flushArtifact(ctx context.Context, rule *schema.RuleConfig, client, ext, content string, trackActivity bool) (artifactFlush, error) {
	existing, err := e.Store.GetArtifactMeta(ctx, rule.Name, client)
	if err != nil {
		return artifactFlush{}, err
	}
	hash := util.SHA256Hex(content)
	syncedAt := util.NowISO()

	var previousContent string
	previousFetched := false
	if existing != nil {
		prev, err := ReadForRule(e.RulesDir, rule, client, ext)
		if err == nil {
			previousContent = prev
			previousFetched = true
			if prev != "" {
				// Strip any legacy managed header so we can fairly compare
				// the on-disk semantic payload against the freshly produced
				// content. After a single resync the on-disk file no longer
				// has a header and Strip becomes a no-op.
				previousSource := transformer.StripManagedRuleHeader(prev)
				if previousSource == content {
					if prev == content {
						// Bytes identical — just bump LastUpdatedAt so the
						// "stale" badge reflects "last confirmed up-to-date"
						// rather than "last time bytes actually changed".
						copyMeta := *existing
						copyMeta.LastHash = hash
						copyMeta.LastUpdatedAt = syncedAt
						copyMeta.ConsecutiveFailures = 0
						return artifactFlush{Meta: &copyMeta}, nil
					}
					// Same semantic content but disk still carries a legacy
					// header. Rewrite to the clean form, but do NOT record an
					// activity change — semantically nothing changed.
					upload, uerr := UploadForRule(e.RulesDir, rule, client, ext, content)
					if uerr != nil {
						return artifactFlush{}, uerr
					}
					size := int64(len(content))
					meta := *existing
					meta.LastHash = hash
					meta.LastUpdatedAt = syncedAt
					meta.BlobPath = upload.Path
					meta.BlobURL = upload.URL
					meta.SizeBytes = &size
					return artifactFlush{Meta: &meta, Wrote: true}, nil
				}
			}
		}
	}
	if !previousFetched {
		prev, _ := ReadForRule(e.RulesDir, rule, client, ext)
		previousContent = prev
	}

	upload, err := UploadForRule(e.RulesDir, rule, client, ext, content)
	if err != nil {
		return artifactFlush{}, err
	}
	size := int64(len(content))
	meta := schema.ArtifactMeta{
		RuleName:      rule.Name,
		Client:        client,
		LastHash:      hash,
		LastUpdatedAt: syncedAt,
		BlobPath:      upload.Path,
		BlobURL:       upload.URL,
		SizeBytes:     &size,
	}

	var change *store.ChangeRecordInput
	if trackActivity {
		changeType := diff.Created
		if existing != nil {
			changeType = diff.Updated
		}
		// Strip any legacy header so the diff against the new (clean)
		// content does not surface the soon-to-vanish "# 规则数量：…" block
		// as a meaningful change.
		previousSource := transformer.StripManagedRuleHeader(previousContent)
		body := diff.CreateActivityDiff(
			changeType,
			previousSource,
			content,
			3,
		)
		// Only record a change when there is an actual diff to show.
		// This avoids 0-byte empty-diff records when both the on-disk
		// content and the freshly produced content are empty.
		if body != "" {
			change = &store.ChangeRecordInput{
				ID:         uuid.New().String(),
				Timestamp:  meta.LastUpdatedAt,
				RuleName:   rule.Name,
				Client:     client,
				ChangeType: string(changeType),
				SizeBytes:  &size,
				Diff:       body,
			}
		}
	}
	return artifactFlush{Meta: &meta, Wrote: true, Change: change}, nil
}

// PreviewResult is returned from PreviewRule. Reports carries the per-step
// transform diagnostics surfaced exclusively in the admin preview panel —
// the field is always nil for any code path that comes from the sync
// pipeline.
type PreviewResult struct {
	Contents    map[string]string                      `json:"contents"`
	Diagnostics PreviewDiag                            `json:"diagnostics"`
	Reports     map[string]transformer.TransformReport `json:"reports,omitempty"`
}

// PreviewDiag mirrors the TS diagnostics shape.
type PreviewDiag struct {
	SourceResults []PreviewSource `json:"sourceResults"`
	Truncated     bool            `json:"truncated"`
	TotalLines    int             `json:"totalLines"`
}

// PreviewSource is a single source diagnostic.
type PreviewSource struct {
	URL     string `json:"url"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Size    int    `json:"size,omitempty"`
}

// PreviewRule executes a rule without persisting anything. builtinParams
// (typically the live RulesConfig.BuiltinParams map) supplies global
// configuration for any built-in transformers referenced by the rule.
func (e *Engine) PreviewRule(ctx context.Context, rule *schema.RuleConfig, transformers map[string]schema.ScriptTransformer, builtinParams map[string]json.RawMessage, limitLines int) (PreviewResult, error) {
	clients, err := e.Store.GetClients(ctx)
	if err != nil {
		return PreviewResult{}, err
	}
	cache := NewRuleContentsCache()

	depNames := map[string]struct{}{}
	for _, src := range rule.Sources {
		if src.SourceType() == "ref" && src.Ref != "" {
			depNames[src.Ref] = struct{}{}
		}
	}
	if len(depNames) > 0 {
		cfg, err := e.Store.GetConfig(ctx)
		if err != nil {
			return PreviewResult{}, err
		}
		processed := map[string]struct{}{}
		queue := make([]string, 0, len(depNames))
		for name := range depNames {
			queue = append(queue, name)
		}
		for len(queue) > 0 {
			name := queue[0]
			queue = queue[1:]
			if _, ok := processed[name]; ok {
				continue
			}
			processed[name] = struct{}{}
			for i := range cfg.Rules {
				if cfg.Rules[i].Name == name {
					for dep := range ExtractDependencies(&cfg.Rules[i]) {
						if _, ok := processed[dep]; !ok {
							queue = append(queue, dep)
						}
					}
				}
			}
		}
		var depRules []schema.RuleConfig
		for _, r := range cfg.Rules {
			if _, ok := processed[r.Name]; ok {
				depRules = append(depRules, r)
			}
		}
		sortedDeps, err := TopologicalSort(depRules, true)
		if err != nil {
			return PreviewResult{}, err
		}
		for i := range sortedDeps {
			res := e.Processor.ProcessRule(ctx, &sortedDeps[i], transformers, builtinParams, cache, clients)
			if len(res.Errors) > 0 {
				return PreviewResult{}, errors.New(joinErrors(res.Errors))
			}
			if len(res.Contents) > 0 {
				cache.Set(sortedDeps[i].Name, res.Contents, res.ClientOrder)
			}
		}
	}

	var diagnostics PreviewDiag
	for _, src := range rule.Sources {
		switch src.SourceType() {
		case "url":
			if src.URL == "" {
				continue
			}
			res := e.Fetcher.Fetch(ctx, src.URL)
			diag := PreviewSource{URL: src.URL, Success: res.Error == "", Size: len(res.Content)}
			if res.Error != "" {
				diag.Error = res.Error
			}
			diagnostics.SourceResults = append(diagnostics.SourceResults, diag)
		case "ref":
			if src.Ref == "" {
				continue
			}
			refContents, refOrder, _ := cache.Get(src.Ref)
			refSize := 0
			if len(refOrder) > 0 {
				if v, ok := refContents[refOrder[0]]; ok {
					refSize = len(v)
				}
			} else {
				for _, v := range refContents {
					refSize = len(v)
					break
				}
			}
			diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
				URL: "ref:" + src.Ref, Success: true, Size: refSize,
			})
		case "local":
			// Hydration may have left src.Content nil if reading the
			// local-source row failed; surface that explicitly here so
			// preview reflects the same diagnostic the sync pipeline will
			// produce, instead of silently reporting size=0 success.
			label := "local"
			if src.ContentRef != "" {
				label = "local:" + src.ContentRef
			}
			if src.Content != nil {
				diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
					URL: label, Success: true, Size: len(*src.Content),
				})
			} else if src.ContentRef == "" {
				diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
					URL: label, Success: false, Error: "missing content or contentRef",
				})
			} else {
				if _, err := e.Store.ReadLocalSource(ctx, src.ContentRef); err != nil {
					diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
						URL: label, Success: false, Error: err.Error(),
					})
				} else {
					// Reachable only if hydrate failed for a transient reason
					// and the second read succeeded; mark as success but with
					// zero size so the UI shows something consistent.
					diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
						URL: label, Success: true, Size: 0,
					})
				}
			}
		case "geosite":
			cacheData, err := e.Geosite.Ensure(ctx, src.Provider)
			if err != nil {
				diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
					URL: "geosite:" + src.Provider + "/" + src.List, Success: false, Error: err.Error(),
				})
				continue
			}
			entries, err := geosite.ResolveEntries(cacheData, src.List, src.Attrs)
			if err != nil {
				diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
					URL: "geosite:" + src.Provider + "/" + src.List, Success: false, Error: err.Error(),
				})
				continue
			}
			rendered, _ := geosite.RenderEntries(entries, src.RenderProfile)
			diagnostics.SourceResults = append(diagnostics.SourceResults, PreviewSource{
				URL: "geosite:" + src.Provider + "/" + src.List, Success: true, Size: len(rendered),
			})
		}
	}

	res := e.Processor.ProcessRuleReported(ctx, rule, transformers, builtinParams, cache, clients)
	if len(res.Errors) > 0 {
		return PreviewResult{}, errors.New(joinErrors(res.Errors))
	}
	contents := res.Contents
	totalLines := 0
	for client, content := range contents {
		lines := splitLines(content)
		if l := len(lines); l > totalLines {
			totalLines = l
		}
		if limitLines > 0 && len(lines) > limitLines {
			diagnostics.Truncated = true
			contents[client] = joinLines(lines[:limitLines]) + "\n# ... (truncated)"
		}
	}
	diagnostics.TotalLines = totalLines
	return PreviewResult{
		Contents:    contents,
		Diagnostics: diagnostics,
		Reports:     res.Reports,
	}, nil
}

// ---- helpers ----

// extByClient builds a quick id -> resolved output extension lookup so the
// flushArtifact loop can stay independent of the underlying slice ordering.
// Returns the schema default for any client missing from the slice (defensive
// against stale references coming out of a rule's Output.Clients).
func extByClient(clients []schema.ClientConfig) map[string]string {
	out := make(map[string]string, len(clients))
	for _, c := range clients {
		out[c.ID] = c.ResolvedOutputExt()
	}
	return out
}

// lookupExt returns the ext for client id, falling back to the schema default.
func lookupExt(m map[string]string, clientID string) string {
	if e, ok := m[clientID]; ok && e != "" {
		return e
	}
	return schema.DefaultOutputExt
}

type providerFail struct {
	Provider string
	Error    string
}

func refreshGeositeProviders(ctx context.Context, mgr *geosite.Manager, providers []string) []providerFail {
	if mgr == nil || len(providers) == 0 {
		return nil
	}
	var (
		mu     sync.Mutex
		failed []providerFail
		wg     sync.WaitGroup
	)
	wg.Add(len(providers))
	for _, p := range providers {
		provider := p
		go func() {
			defer wg.Done()
			if _, err := mgr.Refresh(ctx, provider); err != nil {
				mu.Lock()
				failed = append(failed, providerFail{Provider: provider, Error: err.Error()})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return failed
}

func collectGeositeProviders(rules []schema.RuleConfig) []string {
	set := map[string]struct{}{}
	for i := range rules {
		src := schema.PrimaryGeositeSource(&rules[i])
		if src != nil && src.Provider != "" {
			set[src.Provider] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

var (
	failureSourceRe  = regexp.MustCompile(`^Source (.+?):`)
	failureRefRe     = regexp.MustCompile(`^Ref "(.+?)"`)
	failureRefRuleRe = regexp.MustCompile(`^Ref rule "(.+?)"`)
	failureDepRe     = regexp.MustCompile(`^Dependency rule "(.+?)"`)
	failureClientRe  = regexp.MustCompile(`client ([^ )]+)`)
)

func buildFailureRecords(ruleName string, errors []string, jobID string) []schema.FailureRecord {
	timestamp := util.NowISO()
	out := make([]schema.FailureRecord, 0, len(errors))
	for _, message := range errors {
		rec := schema.FailureRecord{
			ID:        uuid.New().String(),
			Timestamp: timestamp,
			RuleName:  ruleName,
			Message:   message,
			Stage:     "process_rule",
			JobID:     jobID,
		}
		if m := failureSourceRe.FindStringSubmatch(message); len(m) == 2 {
			rec.Source = m[1]
		}
		if m := failureRefRe.FindStringSubmatch(message); len(m) == 2 {
			rec.Source = "ref:" + m[1]
		}
		if m := failureRefRuleRe.FindStringSubmatch(message); len(m) == 2 {
			rec.Source = "ref:" + m[1]
		}
		if m := failureDepRe.FindStringSubmatch(message); len(m) == 2 {
			rec.Source = "dependency:" + m[1]
		}
		if m := failureClientRe.FindStringSubmatch(message); len(m) == 2 {
			rec.Client = m[1]
		}
		out = append(out, rec)
	}
	return out
}

// attemptsForFailedRule produces one ArtifactAttempt per output client for a
// rule that failed during ProcessRule (i.e. before flushArtifact). This keeps
// the artifacts table's last_attempted_at honest even when we never touched
// the blob, so dashboards can flag rules that have been silently stuck.
func attemptsForFailedRule(rule *schema.RuleConfig, errs []string) []store.ArtifactAttempt {
	if rule == nil || len(rule.Output.Clients) == 0 {
		return nil
	}
	now := util.NowISO()
	msg := joinErrors(errs)
	out := make([]store.ArtifactAttempt, 0, len(rule.Output.Clients))
	for _, client := range rule.Output.Clients {
		out = append(out, store.ArtifactAttempt{
			RuleName:    rule.Name,
			Client:      client,
			AttemptedAt: now,
			Status:      "failed",
			Error:       msg,
		})
	}
	return out
}

// buildGeositeStaleRecords emits one aggregated failure record per provider
// summarising every geosite list that vanished from the upstream catalog
// during this sync. Geosite rule failures are otherwise swallowed by design
// (see trackActivity in the main loop) to avoid flooding the activity feed
// with thousands of per-list rows; this gives the admin a single high-signal
// row instead.
func buildGeositeStaleRecords(missing map[string]map[string]struct{}, jobID string) []schema.FailureRecord {
	if len(missing) == 0 {
		return nil
	}
	timestamp := util.NowISO()
	providers := make([]string, 0, len(missing))
	for p := range missing {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	out := make([]schema.FailureRecord, 0, len(providers))
	for _, provider := range providers {
		listSet := missing[provider]
		if len(listSet) == 0 {
			continue
		}
		lists := make([]string, 0, len(listSet))
		for l := range listSet {
			lists = append(lists, l)
		}
		sort.Strings(lists)
		out = append(out, schema.FailureRecord{
			ID:        uuid.New().String(),
			Timestamp: timestamp,
			RuleName:  "geosite-stale:" + provider,
			Source:    fmt.Sprintf("%d list(s)", len(lists)),
			Message:   fmt.Sprintf("Lists missing in upstream catalog: %s", strings.Join(lists, ", ")),
			Stage:     "geosite_list_missing",
			JobID:     jobID,
		})
	}
	return out
}

func uniqueSlice(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func addUnique(out *[]string, v string) {
	for _, e := range *out {
		if e == v {
			return
		}
	}
	*out = append(*out, v)
}

func joinErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	result := errors[0]
	for i := 1; i < len(errors); i++ {
		result += "; " + errors[i]
	}
	return result
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			out = append(out, content[start:i])
			start = i + 1
		}
	}
	out = append(out, content[start:])
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}
