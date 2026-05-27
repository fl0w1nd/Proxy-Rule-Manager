package syncengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/transformer"
)

// Processor combines transformer/fetcher/geosite to produce per-client content.
type Processor struct {
	Store       *store.Store
	Fetcher     *Fetcher
	Transformer *transformer.Engine
	Geosite     *geosite.Manager
}

// ProcessResult contains the per-client outputs and any errors collected.
type ProcessResult struct {
	RuleName    string
	Contents    map[string]string
	ClientOrder []string
	Errors      []string
	// MissingGeositeLists records (provider, list) tuples whose source could
	// not be resolved because the upstream catalog no longer contains them.
	// The engine aggregates these across all rules to produce a single
	// high-signal "geosite-stale:{provider}" failure record per provider.
	MissingGeositeLists []MissingGeositeList
	// Reports carries one TransformReport per client when the rule was run
	// through ProcessRuleReported. The sync pipeline leaves this nil; only
	// admin-side preview populates it.
	Reports map[string]transformer.TransformReport
}

// MissingGeositeList is a (provider, list) tuple that disappeared from the
// upstream catalog during this sync.
type MissingGeositeList struct {
	Provider string
	List     string
}

type cachedRuleContents struct {
	contents map[string]string
	order    []string
}

// RuleContentsCache stores intermediate per-client content for each rule.
type RuleContentsCache struct {
	mu    sync.RWMutex
	store map[string]cachedRuleContents
}

// NewRuleContentsCache constructs an empty cache.
func NewRuleContentsCache() *RuleContentsCache {
	return &RuleContentsCache{store: map[string]cachedRuleContents{}}
}

// Set saves contents for ruleName.
func (c *RuleContentsCache) Set(ruleName string, contents map[string]string, order []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	copied := make(map[string]string, len(contents))
	for k, v := range contents {
		copied[k] = v
	}
	c.store[ruleName] = cachedRuleContents{
		contents: copied,
		order:    append([]string(nil), order...),
	}
}

// Get returns previously stored contents.
func (c *RuleContentsCache) Get(ruleName string) (map[string]string, []string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[ruleName]
	if !ok {
		return nil, nil, false
	}
	contents := make(map[string]string, len(v.contents))
	for k, val := range v.contents {
		contents[k] = val
	}
	return contents, append([]string(nil), v.order...), true
}

// ProcessRule mirrors processRule(rule, transformers, cache, clients) in TS.
// Used by the sync pipeline; per-step diagnostics are not collected. Call
// ProcessRuleReported when you need a TransformReport.
//
// builtinParams maps a built-in transformer name (e.g.
// "builtin:mihomo-to-shadowrocket") to its global parameter blob. Callers
// should pass cfg.BuiltinParams here; nil is equivalent to "no overrides,
// each built-in falls back to its default behaviour".
func (p *Processor) ProcessRule(
	ctx context.Context,
	rule *schema.RuleConfig,
	transformersConfig map[string]schema.ScriptTransformer,
	builtinParams map[string]json.RawMessage,
	cache *RuleContentsCache,
	clients []schema.ClientConfig,
) ProcessResult {
	res, _ := p.processRule(ctx, rule, transformersConfig, builtinParams, cache, clients, false)
	return res
}

// ProcessRuleReported is the preview-only counterpart that records every
// step's input/output line counts and any dropped/modified samples emitted
// by the built-in transformers. Calling pattern is identical to
// ProcessRule; the second return value maps client id → TransformReport.
func (p *Processor) ProcessRuleReported(
	ctx context.Context,
	rule *schema.RuleConfig,
	transformersConfig map[string]schema.ScriptTransformer,
	builtinParams map[string]json.RawMessage,
	cache *RuleContentsCache,
	clients []schema.ClientConfig,
) ProcessResult {
	res, reports := p.processRule(ctx, rule, transformersConfig, builtinParams, cache, clients, true)
	res.Reports = reports
	return res
}

func (p *Processor) processRule(
	ctx context.Context,
	rule *schema.RuleConfig,
	transformersConfig map[string]schema.ScriptTransformer,
	builtinParams map[string]json.RawMessage,
	cache *RuleContentsCache,
	clients []schema.ClientConfig,
	withReport bool,
) (ProcessResult, map[string]transformer.TransformReport) {
	// Merge the built-in registry before running transforms so any user
	// config that still references "builtin:…" via a stale dropdown
	// resolves correctly. The dispatcher in pipeline.go also short-circuits
	// builtins by name, so this merge is defence-in-depth.
	transformersConfig = transformer.MergeBuiltinTransformers(transformersConfig)
	pipelineCtx := transformer.PipelineCtx{Transformers: transformersConfig, BuiltinParams: builtinParams}
	result := ProcessResult{RuleName: rule.Name, Contents: map[string]string{}}
	var reports map[string]transformer.TransformReport
	if withReport {
		reports = make(map[string]transformer.TransformReport)
	}
	staticContents := make(map[int]string)

	for i, src := range rule.Sources {
		switch src.SourceType() {
		case "url":
			if src.URL == "" {
				continue
			}
			res := p.Fetcher.Fetch(ctx, src.URL)
			if res.Error != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("Source %s: %s", src.URL, res.Error))
				continue
			}
			staticContents[i] = res.Content
		case "local":
			if src.Content != nil {
				staticContents[i] = *src.Content
			} else if src.ContentRef != "" {
				content, err := p.Store.ReadLocalSource(ctx, src.ContentRef)
				if err != nil {
					// Surface the failure so the operator can see why the
					// rule has no content instead of the generic "no
					// sources fetched successfully" downstream.
					result.Errors = append(result.Errors,
						fmt.Sprintf("Local source %d (%s): %s", i, src.ContentRef, err.Error()))
					continue
				}
				staticContents[i] = content
			} else {
				// Both Content and ContentRef are empty: explicit bug in
				// the rule config; report instead of silently producing
				// an empty source.
				result.Errors = append(result.Errors,
					fmt.Sprintf("Local source %d: missing content or contentRef", i))
				continue
			}
		case "geosite":
			if p.Geosite == nil || src.Provider == "" || src.List == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("Geosite %s/%s: invalid source", src.Provider, src.List))
				continue
			}
			cacheData, err := p.Geosite.Ensure(ctx, src.Provider)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Geosite %s/%s: %s", src.Provider, src.List, err.Error()))
				continue
			}
			entries, err := geosite.ResolveEntries(cacheData, src.List, src.Attrs)
			if err != nil {
				if errors.Is(err, geosite.ErrListNotFound) {
					result.MissingGeositeLists = append(result.MissingGeositeLists, MissingGeositeList{
						Provider: src.Provider,
						List:     src.List,
					})
				}
				result.Errors = append(result.Errors, fmt.Sprintf("Geosite %s/%s: %s", src.Provider, src.List, err.Error()))
				continue
			}
			rendered, err := geosite.RenderEntries(entries, src.RenderProfile)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Geosite %s/%s: %s", src.Provider, src.List, err.Error()))
				continue
			}
			staticContents[i] = rendered
		}
	}

	for _, client := range rule.Output.Clients {
		if len(rule.Sources) == 0 {
			result.Errors = append(result.Errors, "Rule has no sources")
			return result, reports
		}
		sourceContents := make([]string, 0, len(rule.Sources))
		for i, src := range rule.Sources {
			switch src.SourceType() {
			case "ref":
				if src.Ref == "" {
					continue
				}
				refContents, refOrder, ok := cache.Get(src.Ref)
				if !ok {
					result.Errors = append(result.Errors, fmt.Sprintf(`Ref rule "%s" not found in cache`, src.Ref))
					continue
				}
				content, ok := refContents[client]
				if !ok {
					order := refOrder
					if len(order) == 0 {
						order = make([]string, 0, len(refContents))
						for k := range refContents {
							order = append(order, k)
						}
						sort.Strings(order)
					}
					for _, name := range order {
						if v, exists := refContents[name]; exists {
							content = v
							ok = true
							break
						}
					}
				}
				if !ok || content == "" {
					result.Errors = append(result.Errors, fmt.Sprintf(`Ref "%s" has no content for client %s`, src.Ref, client))
					continue
				}
				sourceContents = append(sourceContents, content)
			default:
				if content, ok := staticContents[i]; ok {
					sourceContents = append(sourceContents, content)
				}
			}
		}
		if len(sourceContents) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("No sources fetched successfully for client %s", client))
			continue
		}

		clientReport := transformer.TransformReport{}

		// --- Stage 1: rule.transforms (per-source) ---
		processed := sourceContents
		if len(rule.Transforms) > 0 {
			var (
				err   error
				steps []transformer.StepReport
			)
			if withReport {
				processed, steps, err = p.Transformer.ApplyNewTransformsReported(sourceContents, rule.Transforms, pipelineCtx, transformer.StageRule)
				clientReport.Steps = append(clientReport.Steps, steps...)
			} else {
				processed, err = p.Transformer.ApplyNewTransforms(sourceContents, rule.Transforms, pipelineCtx)
			}
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Rule %s client %s: %s", rule.Name, client, err.Error()))
				continue
			}
		}

		// --- Stage 2: merge (always 1 logical step, multi-input → single) ---
		strategy := rule.Merge.EffectiveStrategy()
		dedupe := rule.Merge.EffectiveDedupe()
		var mergeDropped []transformer.DroppedLine
		var mergeDroppedTotal int
		var baseContent string
		if withReport {
			baseContent, mergeDropped, mergeDroppedTotal = transformer.MergeContentsReported(processed, strategy, dedupe)
		} else {
			baseContent = transformer.MergeContents(processed, strategy, dedupe)
		}
		if withReport {
			inputLines := 0
			for _, c := range processed {
				inputLines += transformer.CountSignificantLines(c)
			}
			label := "merge " + strategy
			if dedupe {
				label += " +dedupe"
			}
			mergeStep := transformer.StepReport{
				Stage:        transformer.StageMerge,
				Index:        0,
				Kind:         transformer.KindMerge,
				Label:        label,
				InputLines:   inputLines,
				OutputLines:  transformer.CountSignificantLines(baseContent),
				Dropped:      mergeDropped,
				DroppedTotal: mergeDroppedTotal,
			}
			clientReport.Steps = append(clientReport.Steps, mergeStep)
		}

		override, hasOverride := rule.Output.ClientOverrides[client]
		if hasOverride && !override.IsEnabled() {
			result.Contents[client] = baseContent
			if withReport {
				clientReport.FinalStats = transformer.ComputeFinalStats(baseContent)
				reports[client] = clientReport
			}
			continue
		}

		useGlobal := true
		if hasOverride {
			useGlobal = override.ShouldUseGlobalTransforms()
		}
		clientTransformFailed := false

		// --- Stage 3: client.transforms (single content track) ---
		if useGlobal {
			for _, cConfig := range clients {
				if cConfig.ID == client && len(cConfig.Transforms) > 0 {
					var (
						transformed []string
						err         error
						steps       []transformer.StepReport
					)
					if withReport {
						transformed, steps, err = p.Transformer.ApplyNewTransformsReported([]string{baseContent}, cConfig.Transforms, pipelineCtx, transformer.StageClient)
						clientReport.Steps = append(clientReport.Steps, steps...)
					} else {
						transformed, err = p.Transformer.ApplyNewTransforms([]string{baseContent}, cConfig.Transforms, pipelineCtx)
					}
					if err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("Rule %s client %s: %s", rule.Name, client, err.Error()))
						clientTransformFailed = true
						break
					}
					if len(transformed) > 0 {
						baseContent = transformed[0]
					}
					break
				}
			}
		}
		if clientTransformFailed {
			continue
		}

		// --- Stage 4: override.transforms ---
		if hasOverride && len(override.Transforms) > 0 {
			var (
				transformed []string
				err         error
				steps       []transformer.StepReport
			)
			if withReport {
				transformed, steps, err = p.Transformer.ApplyNewTransformsReported([]string{baseContent}, override.Transforms, pipelineCtx, transformer.StageOverride)
				clientReport.Steps = append(clientReport.Steps, steps...)
			} else {
				transformed, err = p.Transformer.ApplyNewTransforms([]string{baseContent}, override.Transforms, pipelineCtx)
			}
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Rule %s client %s: %s", rule.Name, client, err.Error()))
				continue
			}
			if len(transformed) > 0 {
				baseContent = transformed[0]
			}
		}

		result.Contents[client] = baseContent
		result.ClientOrder = append(result.ClientOrder, client)
		if withReport {
			clientReport.FinalStats = transformer.ComputeFinalStats(baseContent)
			reports[client] = clientReport
		}
	}
	return result, reports
}
