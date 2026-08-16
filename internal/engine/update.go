package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

// UpdateEngine orchestrates full and partial updates.
type UpdateEngine struct {
	DataDir      string
	Config       *config.Config
	Registry     *render.Registry
	Fetcher      *Fetcher
	Preprocessor *PreprocessRunner
	State        *state.Store
	Geosite      *geosite.Manager
	Logger       *slog.Logger
}

// UpdateResult holds the outcome of an update operation.
type UpdateResult struct {
	StartTime        time.Time
	EndTime          time.Time
	RulesTotal       int
	RulesSucceeded   int
	RulesFailed      int
	Artifacts        int
	EffectiveRuleIDs []string
	ChangedRules     []string // rule IDs with changed artifacts
	Changes          []RuleChange
	Errors           []string
	Warnings         []string
	Issues           []UpdateIssue
}

// RuleChange captures the logical IR diff for one rule.
type RuleChange struct {
	RuleID         string   `json:"rule_id"`
	RuleName       string   `json:"rule_name"`
	Added          int      `json:"added"`
	Removed        int      `json:"removed"`
	AddedSamples   []string `json:"added_samples"`
	RemovedSamples []string `json:"removed_samples"`
}

func (r *UpdateResult) addError(stage, subject, message string) {
	r.Errors = append(r.Errors, message)
	r.Issues = append(r.Issues, UpdateIssue{Stage: stage, Subject: subject, Message: message})
}

func (r *UpdateResult) addWarning(message string) {
	r.Warnings = append(r.Warnings, message)
}

// FullUpdate compiles all rules and writes artifacts.
func (e *UpdateEngine) FullUpdate(ctx context.Context) UpdateResult {
	return e.updateRules(ctx, e.Config.Rules, false)
}

// PartialUpdate compiles only the selected rule IDs plus their dependents.
func (e *UpdateEngine) PartialUpdate(ctx context.Context, ruleIDs []string) UpdateResult {
	known := make(map[string]bool, len(e.Config.Rules))
	for _, rule := range e.Config.Rules {
		known[rule.ID] = true
	}
	var unknown []string
	for _, id := range ruleIDs {
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		return e.finishRejectedUpdate(ctx, fmt.Sprintf("unknown rule IDs: %s", strings.Join(unknown, ", ")))
	}

	affected := CollectAffectedRules(e.Config.Rules, ruleIDs)
	var rules []config.RuleConfig
	for _, r := range e.Config.Rules {
		if _, ok := affected[r.ID]; ok {
			rules = append(rules, r)
		}
	}
	return e.updateRules(ctx, rules, true)
}

func (e *UpdateEngine) updateRules(ctx context.Context, rules []config.RuleConfig, partial bool) UpdateResult {
	result := UpdateResult{StartTime: time.Now()}
	log := e.Logger
	expectedPaths := make(map[string]struct{})
	ruleInfos := make(map[string]*ruleSiteInfo)
	gstats := newGeositeStats()
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "prepare", Status: "running", Message: "正在准备更新"})

	sorted, err := TopologicalSort(rules, partial)
	if err != nil {
		result.addError("prepare", "rules", err.Error())
		reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: "prepare", Status: "failed", Message: err.Error()})
		log.Error("topological sort failed", "error", err)
		result.EndTime = time.Now()
		result.RulesTotal = len(rules)
		result.RulesFailed = len(rules)
		for _, rule := range rules {
			e.State.SetRuleCheck(rule.ID, state.RuleFailed, result.EndTime, false)
		}
		e.State.SetLastCheck(result.EndTime)
		if saveErr := e.State.Save(); saveErr != nil {
			result.addError("state", "update.json", fmt.Sprintf("save state: %v", saveErr))
		}
		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "site", Status: "running", Message: "正在刷新规则目录"})
		if siteErr := e.writeSite(&result, ruleInfos, e.rebuildGeositeStats()); siteErr != nil {
			result.addError("site", "public", fmt.Sprintf("rebuild site: %v", siteErr))
			_ = e.State.Save()
		}
		return result
	}

	var geositeProviders map[string]*geosite.ProviderCache
	if partial {
		geositeProviders = e.readCachedGeositeProviders(sorted)
	} else {
		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "geosite_refresh", Status: "running", Message: "正在刷新 Geosite"})
		var geositeResults map[string]string
		var geositeWarnings []string
		var geositeErrors map[string]string
		geositeProviders, geositeResults, geositeWarnings, geositeErrors = e.refreshGeosite(ctx)
		for _, warning := range geositeWarnings {
			result.addWarning(warning)
			reportProgress(ctx, ProgressEvent{Kind: ProgressWarning, Stage: "geosite_refresh", Status: "warning", Message: warning})
		}
		for _, provider := range collectProviderNames(e.Config) {
			message, exists := geositeErrors[provider]
			if !exists {
				continue
			}
			fullMessage := fmt.Sprintf("geosite provider %q refresh: %s", provider, message)
			result.addError("geosite_refresh", provider, fullMessage)
			reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: "geosite_refresh", Status: "failed", Subject: provider, Message: fullMessage})
		}
		geositeCheckedAt := time.Now()
		for _, name := range collectProviderNames(e.Config) {
			version := ""
			if cache := geositeProviders[name]; cache != nil {
				version = cache.ResolvedVersion
			}
			fetchResult := geositeResults[name]
			e.State.SetGeositeUpdate(name, fetchResult, geositeCheckedAt)
			gstats.setMeta(name, version, fetchResult, geositeCheckedAt)
		}
	}

	refResults := make(map[string][]ir.Entry)
	if partial {
		e.preloadReferenceSnapshots(sorted, refResults, &result)
	}
	result.RulesTotal = len(sorted)
	for _, rule := range sorted {
		result.EffectiveRuleIDs = append(result.EffectiveRuleIDs, rule.ID)
	}
	processed := make(map[string]bool, len(sorted))
	succeeded := make(map[string]struct{}, len(sorted))
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "rules", Status: "running", Total: len(sorted), Message: fmt.Sprintf("正在更新规则 · 0 / %d", len(sorted))})

ruleLoop:
	for ruleIndex, rule := range sorted {
		select {
		case <-ctx.Done():
			break ruleLoop
		default:
		}

		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "rule", Status: "running", Current: ruleIndex + 1, Total: len(sorted), RuleID: rule.ID, RuleName: rule.Name, Message: fmt.Sprintf("正在更新 %s · %d / %d", rule.Name, ruleIndex+1, len(sorted))})
		outcome := e.compileAndWriteRule(ctx, rule, geositeProviders, refResults)
		processed[rule.ID] = true
		if outcome.info != nil {
			ruleInfos[rule.ID] = outcome.info
		}
		if outcome.failed {
			if ctx.Err() != nil {
				e.State.SetRuleCheck(rule.ID, state.RuleCancelled, time.Now(), false)
			} else {
				result.RulesFailed++
				e.State.SetRuleCheck(rule.ID, state.RuleFailed, time.Now(), false)
			}
		} else if outcome.updated {
			succeeded[rule.ID] = struct{}{}
			result.RulesSucceeded++
			updateResult := state.RuleUnchanged
			if outcome.changed {
				updateResult = state.RuleUpdated
			}
			e.State.SetRuleCheck(rule.ID, updateResult, time.Now(), outcome.changed)
			e.State.SetRuleEntryCount(rule.ID, outcome.info.entries)
		}
		if outcome.changed {
			result.ChangedRules = append(result.ChangedRules, rule.ID)
		}
		if outcome.updated && outcome.info != nil && (outcome.info.added > 0 || outcome.info.removed > 0) {
			result.Changes = append(result.Changes, RuleChange{
				RuleID: rule.ID, RuleName: rule.Name,
				Added: outcome.info.added, Removed: outcome.info.removed,
				AddedSamples:   append([]string(nil), outcome.info.addedSamples...),
				RemovedSamples: append([]string(nil), outcome.info.removedSamples...),
			})
		}
		result.Artifacts += outcome.artifacts
		for _, message := range outcome.errors {
			result.addError("rule", rule.ID, message)
		}
		ruleKind := ProgressSuccess
		ruleStatus := "unchanged"
		ruleMessage := fmt.Sprintf("%s · 已是最新", rule.Name)
		if outcome.failed {
			ruleKind = ProgressError
			ruleStatus = "failed"
			ruleMessage = fmt.Sprintf("%s · 更新失败", rule.Name)
		} else if outcome.changed {
			ruleStatus = "updated"
			ruleMessage = fmt.Sprintf("%s · 已更新", rule.Name)
		}
		reportProgress(ctx, ProgressEvent{Kind: ruleKind, Stage: "rule", Status: ruleStatus, Current: ruleIndex + 1, Total: len(sorted), RuleID: rule.ID, RuleName: rule.Name, Message: ruleMessage})
		if outcome.failed {
			for _, message := range outcome.errors {
				reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: "rule", Status: "failed", RuleID: rule.ID, RuleName: rule.Name, Subject: rule.ID, Message: message})
			}
		}
		for _, path := range outcome.artifactPaths {
			expectedPaths[filepath.Clean(path)] = struct{}{}
		}
	}

	// Handle geosite publications
	if !partial && e.Config.Geosite != nil {
		e.updateGeositePublications(ctx, geositeProviders, &result, expectedPaths, gstats)
	}

	if ctx.Err() != nil {
		result.addError("cancel", "update", "update cancelled")
		cancelledAt := time.Now()
		for _, rule := range sorted {
			if !processed[rule.ID] {
				e.State.SetRuleCheck(rule.ID, state.RuleCancelled, cancelledAt, false)
			}
		}
	}

	if !partial && len(result.Errors) == 0 {
		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "cleanup", Status: "running", Message: "正在清理过期规则文件"})
		if err := ReconcileArtifacts(e.DataDir, expectedPaths); err != nil {
			result.addError("cleanup", "artifacts", fmt.Sprintf("reconcile artifacts: %v", err))
		}
		expectedArtifacts, expectedRules := stateManifest(e.Config)
		if err := e.State.Reconcile(expectedArtifacts, expectedRules); err != nil {
			result.addError("cleanup", "state", fmt.Sprintf("reconcile state: %v", err))
		}
	}
	if partial && ctx.Err() == nil && len(succeeded) > 0 {
		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "cleanup", Status: "running", Message: "正在清理过期规则文件"})
		if err := ReconcileRuleArtifacts(e.DataDir, succeeded, expectedPaths); err != nil {
			result.addError("cleanup", "artifacts", fmt.Sprintf("reconcile partial artifacts: %v", err))
		} else {
			expectedArtifacts, _ := stateManifest(e.Config)
			selectedArtifacts := make(map[string]map[string]struct{}, len(succeeded))
			for ruleID := range succeeded {
				selectedArtifacts[ruleID] = expectedArtifacts[ruleID]
			}
			e.State.ReconcileRuleArtifacts(selectedArtifacts)
		}
	}

	result.EndTime = time.Now()
	e.State.SetLastCheck(result.EndTime)
	if err := e.State.Save(); err != nil {
		result.addError("state", "update.json", fmt.Sprintf("save state: %v", err))
	}

	// Refresh the public catalog from the latest persisted rule state.
	if partial {
		gstats = e.rebuildGeositeStats()
	}
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "site", Status: "running", Message: "正在刷新规则目录"})
	if err := e.writeSite(&result, ruleInfos, gstats); err != nil {
		result.addError("site", "public", fmt.Sprintf("rebuild site: %v", err))
		_ = e.State.Save()
		reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: "site", Status: "failed", Message: fmt.Sprintf("规则目录更新失败：%v", err)})
	} else {
		reportProgress(ctx, ProgressEvent{Kind: ProgressSuccess, Stage: "site", Status: "completed", Message: "规则目录已更新"})
	}

	log.Info("update complete",
		"total", result.RulesTotal,
		"succeeded", result.RulesSucceeded,
		"failed", result.RulesFailed,
		"changed", len(result.ChangedRules),
		"artifacts", result.Artifacts,
	)
	return result
}

func (e *UpdateEngine) finishRejectedUpdate(ctx context.Context, message string) UpdateResult {
	now := time.Now()
	result := UpdateResult{StartTime: now, EndTime: now}
	result.addError("prepare", "rules", message)
	reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: "prepare", Status: "failed", Message: message})
	e.State.SetLastCheck(now)
	_ = e.State.Save()
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "site", Status: "running", Message: "正在刷新规则目录"})
	if err := e.writeSite(&result, map[string]*ruleSiteInfo{}, e.rebuildGeositeStats()); err != nil {
		result.addError("site", "public", fmt.Sprintf("rebuild site: %v", err))
		_ = e.State.Save()
	}
	return result
}

func (e *UpdateEngine) preloadReferenceSnapshots(rules []config.RuleConfig, refResults map[string][]ir.Entry, result *UpdateResult) {
	selected := make(map[string]bool, len(rules))
	for _, rule := range rules {
		selected[rule.ID] = true
	}
	for _, rule := range rules {
		for dep := range ExtractDependencies(&rule) {
			if selected[dep] {
				continue
			}
			entries, exists, err := e.State.LoadSnapshotIfExists(dep)
			if err != nil {
				result.addError("prepare", dep, fmt.Sprintf("load ref snapshot %q: %v", dep, err))
				continue
			}
			if exists {
				refResults[dep] = entries
			}
		}
	}
}

// ruleOutcome aggregates the result of compiling and writing one rule.
type ruleOutcome struct {
	updated       bool
	failed        bool
	changed       bool
	artifacts     int
	artifactPaths []string
	errors        []string
	info          *ruleSiteInfo
}

// compileAndWriteRule runs the full pipeline for one rule (fetch → parse →
// ops → merge → render) and writes artifacts with hash-based dedup. Errors
// are localised: a failed source or render does not block other rules.
func (e *UpdateEngine) compileAndWriteRule(
	ctx context.Context,
	rule config.RuleConfig,
	geositeProviders map[string]*geosite.ProviderCache,
	refResults map[string][]ir.Entry,
) ruleOutcome {
	var outcome ruleOutcome
	log := e.Logger.With("rule_id", rule.ID, "rule_name", rule.Name)

	cr := CompileRule(ctx, rule, e.Config.Clients, e.Fetcher, e.Preprocessor, e.Registry, geositeProviders, refResults, config.NewLocalFileResolver(e.DataDir), e.Logger)

	info := newRuleSiteInfo(rule, cr)

	hasError := false
	for _, src := range cr.Sources {
		if src.Error != "" {
			hasError = true
			outcome.errors = append(outcome.errors, fmt.Sprintf("rule %q (%s) source %q: %s", rule.Name, rule.ID, src.Label, src.Error))
		}
		for _, diag := range src.Diagnostics {
			hasError = true
			location := ""
			if diag.Line > 0 {
				location = fmt.Sprintf(" line %d", diag.Line)
			}
			outcome.errors = append(outcome.errors, fmt.Sprintf(
				"rule %q (%s) source %q%s (%s): %s",
				rule.Name, rule.ID, src.Label, location, diag.Text, diag.Reason,
			))
		}
	}
	if cr.OpsError != "" {
		hasError = true
		outcome.errors = append(outcome.errors, fmt.Sprintf("rule %q (%s) ops: %s", rule.Name, rule.ID, cr.OpsError))
	}
	for client, rerr := range cr.RenderErrors {
		hasError = true
		outcome.errors = append(outcome.errors, fmt.Sprintf("rule %q (%s) render %q: %s", rule.Name, rule.ID, client, rerr))
	}

	if hasError {
		outcome.failed = true
		outcome.info = info
		return outcome
	}

	// Compute the client-independent IR delta before publishing any artifacts.
	// A missing snapshot is the empty baseline for a newly introduced rule.
	old, _, err := e.State.LoadSnapshotIfExists(rule.ID)
	if err != nil {
		outcome.errors = append(outcome.errors, fmt.Sprintf("load snapshot for rule %q (%s): %v", rule.Name, rule.ID, err))
		outcome.failed = true
		outcome.info = info
		return outcome
	}
	diff := ir.Diff(old, cr.Merged)
	info.added, info.removed = diff.AddedCount, diff.RemovedCount
	for _, g := range diff.Groups {
		info.addedSamples = appendSamples(info.addedSamples, g.Added)
		info.removedSamples = appendSamples(info.removedSamples, g.Removed)
	}

	refResults[rule.ID] = cr.Merged

	writeFailed := false
	for clientID, content := range cr.Rendered {
		target, ok := config.FindOutputTarget(e.Config.Clients, clientID)
		if !ok {
			continue
		}
		tmpl, _ := e.Registry.Get(target.Template)
		ext := ".list"
		if tmpl != nil {
			ext = tmpl.Extension
		}
		artifactPath, err := ArtifactPath(e.DataDir, clientID, rule.ID+ext)
		if err != nil {
			outcome.errors = append(outcome.errors, fmt.Sprintf("resolve artifact path for rule %q (%s) client %q: %v", rule.Name, rule.ID, clientID, err))
			writeFailed = true
			continue
		}
		if len(content) == 0 {
			existed := util.FileExists(artifactPath)
			if err := os.Remove(artifactPath); err != nil && !os.IsNotExist(err) {
				outcome.errors = append(outcome.errors, fmt.Sprintf("remove empty artifact %s: %v", artifactPath, err))
				writeFailed = true
				continue
			}
			e.State.DeleteArtifactHash(rule.ID, clientID)
			if existed {
				outcome.changed = true
			}
			continue
		}

		file := site.RuleFile{
			Client: clientID,
			Icon:   site.ResolveClientIcon(target.Icon, target.ClientID),
			Path:   "rules/" + clientID + "/" + rule.ID + ext,
			Size:   int64(len(content)),
		}

		contentHash := util.SHA256Hex(string(content))
		storedHash := e.State.GetArtifactHash(rule.ID, clientID)
		if storedHash == contentHash && util.FileExists(artifactPath) {
			outcome.artifacts++
			outcome.artifactPaths = append(outcome.artifactPaths, artifactPath)
			info.files = append(info.files, file)
			continue
		}

		if err := util.AtomicWriteFile(artifactPath, content); err != nil {
			outcome.errors = append(outcome.errors, fmt.Sprintf("write %s: %v", artifactPath, err))
			writeFailed = true
			continue
		}

		e.State.SetArtifactHash(rule.ID, clientID, contentHash)
		outcome.artifacts++
		outcome.artifactPaths = append(outcome.artifactPaths, artifactPath)
		if storedHash != contentHash {
			outcome.changed = true
		}
		info.files = append(info.files, file)
	}
	if writeFailed {
		outcome.failed = true
		outcome.info = info
		return outcome
	}

	if err := e.State.SaveSnapshot(rule.ID, cr.Merged); err != nil {
		outcome.errors = append(outcome.errors, fmt.Sprintf("save snapshot for rule %q (%s): %v", rule.Name, rule.ID, err))
		outcome.failed = true
		outcome.info = info
		return outcome
	}

	// Stable file ordering following rule.Outputs.
	orderedTargets := config.ExpandSelectedTargets(e.Config.Clients, rule.Outputs)
	order := make(map[string]int, len(orderedTargets))
	for i, target := range orderedTargets {
		order[target.ID] = i
	}
	sort.SliceStable(info.files, func(i, j int) bool {
		return order[info.files[i].Client] < order[info.files[j].Client]
	})

	outcome.info = info
	outcome.updated = true
	log.Info("rule compiled",
		"entries", len(cr.Merged),
		"clients", len(cr.Rendered),
		"changed", outcome.changed,
	)
	return outcome
}

func (e *UpdateEngine) refreshGeosite(ctx context.Context) (map[string]*geosite.ProviderCache, map[string]string, []string, map[string]string) {
	previous := make(map[string]*geosite.ProviderCache)
	if e.Geosite != nil {
		for _, name := range collectProviderNames(e.Config) {
			cache, _ := e.Geosite.Read(name)
			previous[name] = cache
		}
	}
	caches, failed, fetchErrors := refreshGeositeProviders(ctx, e.Config, e.Geosite, e.Logger)
	results := make(map[string]string)
	for _, name := range collectProviderNames(e.Config) {
		results[name] = geositeFetchResult(previous[name], caches[name], failed[name])
	}
	var warnings []string
	for name, current := range caches {
		if failed[name] || previous[name] == nil || current == nil {
			continue
		}
		warnings = append(warnings, geositeRemovalWarnings(name, previous[name], current)...)
	}
	return caches, results, warnings, fetchErrors
}

// readCachedGeositeProviders loads only providers referenced by a partial rule
// update. Partial updates deliberately reuse on-disk cache data.
func (e *UpdateEngine) readCachedGeositeProviders(rules []config.RuleConfig) map[string]*geosite.ProviderCache {
	providers := make(map[string]*geosite.ProviderCache)
	if e.Geosite == nil {
		return providers
	}
	names := make(map[string]struct{})
	for i := range rules {
		for _, source := range rules[i].Sources {
			if source.SourceType() != "geosite" {
				continue
			}
			ref, err := source.ResolveGeositeRef()
			if err == nil {
				names[ref.Provider] = struct{}{}
			}
		}
	}
	for name := range names {
		cache, err := e.Geosite.Read(name)
		if err == nil && cache != nil {
			providers[name] = cache
		}
	}
	return providers
}

func geositeFetchResult(previous, current *geosite.ProviderCache, failed bool) string {
	if failed {
		return state.GeositeFailed
	}
	if previous != nil && current != nil && previous.ResolvedVersion == current.ResolvedVersion {
		return state.GeositeUnchanged
	}
	return state.GeositeUpdated
}

// refreshGeositeProviders fetches the latest provider data. A failed fetch
// falls back to the current cache so published rules remain available.
func refreshGeositeProviders(ctx context.Context, cfg *config.Config, mgr *geosite.Manager, logger *slog.Logger) (map[string]*geosite.ProviderCache, map[string]bool, map[string]string) {
	caches := make(map[string]*geosite.ProviderCache)
	failed := make(map[string]bool)
	fetchErrors := make(map[string]string)
	providers := collectProviderNames(cfg)
	if mgr == nil {
		for _, name := range providers {
			failed[name] = true
			fetchErrors[name] = "provider manager unavailable"
		}
		return caches, failed, fetchErrors
	}
	for _, name := range providers {
		cache, err := mgr.RefreshWithRetry(
			ctx,
			name,
			cfg.Update.Fetch.Retries,
			time.Duration(cfg.Update.Fetch.RetryDelay),
			func(attempt, total int, delay time.Duration, retryErr error) {
				message := fmt.Sprintf("Geosite %s 刷新失败 · %v · 正在重试 %d / %d", name, retryErr, attempt, total)
				reportProgress(ctx, ProgressEvent{
					Kind: ProgressWarning, Stage: "geosite_refresh", Status: "retrying",
					Current: attempt, Total: total, Subject: name, Message: message,
				})
				logger.Warn("geosite provider refresh retrying",
					"provider", name, "attempt", attempt, "retries", total,
					"delay", delay, "error", retryErr,
				)
			},
		)
		if err != nil || cache == nil {
			failed[name] = true
			if err != nil {
				fetchErrors[name] = err.Error()
			} else {
				fetchErrors[name] = "refresh returned no data"
			}
			logger.Warn("geosite provider refresh failed", "provider", name, "error", err)
			cached, readErr := mgr.Read(name)
			if readErr != nil {
				logger.Warn("geosite provider cache read failed", "provider", name, "error", readErr)
				fetchErrors[name] += "; cache read: " + readErr.Error()
			}
			if cached != nil {
				cache = cached
			}
		}
		if cache != nil {
			caches[name] = cache
		}
		if cache == nil {
			failed[name] = true
		}
	}
	return caches, failed, fetchErrors
}

func geositeRemovalWarnings(provider string, previous, current *geosite.ProviderCache) []string {
	currentLists := make(map[string]geosite.CatalogSummary)
	for _, summary := range geosite.CatalogSummaries(current) {
		currentLists[summary.Name] = summary
	}
	var warnings []string
	for _, oldSummary := range geosite.CatalogSummaries(previous) {
		newSummary, exists := currentLists[oldSummary.Name]
		if !exists {
			warnings = append(warnings, fmt.Sprintf("Geosite %s/%s 已从上游移除", provider, oldSummary.Name))
			continue
		}
		currentAttrs := make(map[string]bool, len(newSummary.Attrs))
		for _, attr := range newSummary.Attrs {
			currentAttrs[attr] = true
		}
		for _, attr := range oldSummary.Attrs {
			if !currentAttrs[attr] {
				warnings = append(warnings, fmt.Sprintf("Geosite %s/%s@%s 已从上游移除", provider, oldSummary.Name, attr))
			}
		}
	}
	sort.Strings(warnings)
	return warnings
}

func collectProviderNames(cfg *config.Config) []string {
	seen := map[string]bool{}
	var providers []string

	addProvider := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			providers = append(providers, name)
		}
	}

	for _, rule := range cfg.Rules {
		for _, src := range rule.Sources {
			if src.SourceType() != "geosite" {
				continue
			}
			ref, err := src.ResolveGeositeRef()
			if err == nil {
				addProvider(ref.Provider)
			}
		}
	}
	if cfg.Geosite != nil {
		for _, p := range cfg.Geosite.Providers {
			addProvider(p.Name)
		}
	}
	return providers
}

// updateGeositePublications auto-enumerates all lists from each configured
// provider and publishes:
//   - provider/list{ext}           (full list, all entries)
//   - provider/list@attr{ext}      (per-attr variant, one file per unique attr)
func (e *UpdateEngine) updateGeositePublications(
	ctx context.Context,
	providers map[string]*geosite.ProviderCache,
	result *UpdateResult,
	expectedPaths map[string]struct{},
	gstats *geositeStats,
) {
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "geosite_publish", Status: "running", Message: "正在更新 Geosite 规则文件"})
	for _, prov := range e.Config.Geosite.Providers {
		cache, ok := providers[prov.Name]
		if !ok {
			result.addError("geosite_publish", prov.Name, fmt.Sprintf("geosite provider %q not loaded", prov.Name))
			continue
		}

		summaries := geosite.CatalogSummaries(cache)
		e.Logger.Info("geosite auto-publish", "provider", prov.Name, "lists", len(summaries))
		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "geosite_publish", Status: "running", Total: len(summaries), Subject: prov.Name, Message: fmt.Sprintf("正在更新 Geosite %s · 0 / %d", prov.Name, len(summaries))})

		for summaryIndex, summary := range summaries {
			select {
			case <-ctx.Done():
				return
			default:
			}
			reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "geosite_publish", Status: "running", Current: summaryIndex + 1, Total: len(summaries), Subject: prov.Name + "/" + summary.Name, Message: fmt.Sprintf("Geosite %s · %d / %d · %s", prov.Name, summaryIndex+1, len(summaries), summary.Name)})

			// Full list (no attr filter)
			fullRef := geosite.GeositeRef{Provider: prov.Name, List: summary.Name}
			e.publishGeositeVariant(cache, fullRef, prov.Clients, result, expectedPaths, gstats)

			// Per-attr variants
			for _, attr := range summary.Attrs {
				attrRef := geosite.GeositeRef{Provider: prov.Name, List: summary.Name, Attrs: []string{attr}}
				e.publishGeositeVariant(cache, attrRef, prov.Clients, result, expectedPaths, gstats)
			}
		}
		reportProgress(ctx, ProgressEvent{Kind: ProgressSuccess, Stage: "geosite_publish", Status: "completed", Current: len(summaries), Total: len(summaries), Subject: prov.Name, Message: fmt.Sprintf("Geosite %s · 已更新", prov.Name)})
	}
}

// resolveGeositeIR resolves a geosite ref against a provider cache into
// deduplicated IR entries. Shared by publishing and by site rebuilds that
// recount published variants from the on-disk cache.
func resolveGeositeIR(cache *geosite.ProviderCache, ref geosite.GeositeRef) ([]ir.Entry, error) {
	entries, err := geosite.ResolveEntries(cache, ref.List, ref.Attrs)
	if err != nil {
		return nil, err
	}
	irEntries := make([]ir.Entry, 0, len(entries))
	for _, ge := range entries {
		if kind := geositeTypeToIRKind(ge.Type); kind != "" {
			irEntries = append(irEntries, ir.Entry{Kind: kind, Value: ge.Value})
		}
	}
	return ir.Dedupe(irEntries), nil
}

func (e *UpdateEngine) publishGeositeVariant(
	cache *geosite.ProviderCache,
	ref geosite.GeositeRef,
	clientIDs []string,
	result *UpdateResult,
	expectedPaths map[string]struct{},
	gstats *geositeStats,
) {
	irEntries, err := resolveGeositeIR(cache, ref)
	if err != nil {
		result.addError("geosite_publish", ref.FormatRef(), fmt.Sprintf("geosite resolve %s: %v", ref.FormatRef(), err))
		return
	}
	if len(irEntries) == 0 {
		return
	}
	gstats.recordVariant(ref.Provider, ref.List, ref.Attrs, len(irEntries))

	artifactName := "geosite/" + ref.ArtifactName()

	for _, target := range config.ExpandSelectedTargets(e.Config.Clients, clientIDs) {
		tmpl, ok := e.Registry.Get(target.Template)
		if !ok {
			result.addError("geosite_publish", target.ID, fmt.Sprintf("geosite template %q not found for output %q", target.Template, target.ID))
			continue
		}
		targetEntries := cloneEntries(irEntries)
		targetEntries, rerr := applyOps(targetEntries, target.Ops)
		if rerr != nil {
			result.addError("geosite_publish", ref.FormatRef(), fmt.Sprintf("geosite variant ops %s for %s: %v", ref.FormatRef(), target.ID, rerr))
			continue
		}
		rendered, rerr := render.Render(tmpl, targetEntries)
		if rerr != nil {
			result.addError("geosite_publish", ref.FormatRef(), fmt.Sprintf("geosite render %s for %s: %v", ref.FormatRef(), target.ID, rerr))
			continue
		}
		// data/rules/{client}/geosite/{provider}/{list}{ext} or {list}@{attr}{ext}
		artifactPath, err := ArtifactPath(e.DataDir, target.ID, artifactName+tmpl.Extension)
		if err != nil {
			result.addError("geosite_publish", ref.FormatRef(), fmt.Sprintf("resolve geosite artifact path %s: %v", ref.FormatRef(), err))
			continue
		}
		if len(rendered) == 0 {
			if err := os.Remove(artifactPath); err != nil && !os.IsNotExist(err) {
				result.addError("geosite_publish", ref.FormatRef(), fmt.Sprintf("remove empty geosite artifact: %v", err))
			}
			continue
		}
		if err := util.AtomicWriteFile(artifactPath, rendered); err != nil {
			result.addError("geosite_publish", ref.FormatRef(), fmt.Sprintf("write geosite artifact: %v", err))
			continue
		}
		expectedPaths[filepath.Clean(artifactPath)] = struct{}{}
		result.Artifacts++
		gstats.recordFile(ref.Provider)
	}
}

func stateManifest(cfg *config.Config) (map[string]map[string]struct{}, map[string]struct{}) {
	artifacts := make(map[string]map[string]struct{}, len(cfg.Rules))
	rules := make(map[string]struct{}, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		rules[rule.ID] = struct{}{}
		targets := config.ExpandSelectedTargets(cfg.Clients, rule.Outputs)
		clients := make(map[string]struct{}, len(targets))
		for _, target := range targets {
			clients[target.ID] = struct{}{}
		}
		artifacts[rule.ID] = clients
	}
	return artifacts, rules
}
