package syncengine

import (
	"context"
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
func (p *Processor) ProcessRule(
	ctx context.Context,
	rule *schema.RuleConfig,
	transformersConfig map[string]schema.ScriptTransformer,
	cache *RuleContentsCache,
	clients []schema.ClientConfig,
) ProcessResult {
	result := ProcessResult{RuleName: rule.Name, Contents: map[string]string{}}
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
			return result
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

		processed := sourceContents
		if len(rule.Transforms) > 0 {
			var err error
			processed, err = p.Transformer.ApplyNewTransforms(sourceContents, rule.Transforms, transformersConfig)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Rule %s client %s: %s", rule.Name, client, err.Error()))
				continue
			}
		}

		strategy := rule.Merge.EffectiveStrategy()
		dedupe := rule.Merge.EffectiveDedupe()
		baseContent := transformer.MergeContents(processed, strategy, dedupe)

		override, hasOverride := rule.Output.ClientOverrides[client]
		if hasOverride && !override.IsEnabled() {
			result.Contents[client] = baseContent
			continue
		}

		useGlobal := true
		if hasOverride {
			useGlobal = override.ShouldUseGlobalTransforms()
		}
		clientTransformFailed := false
		if useGlobal {
			for _, cConfig := range clients {
				if cConfig.ID == client && len(cConfig.Transforms) > 0 {
					transformed, err := p.Transformer.ApplyNewTransforms([]string{baseContent}, cConfig.Transforms, transformersConfig)
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
		if hasOverride && len(override.Transforms) > 0 {
			transformed, err := p.Transformer.ApplyNewTransforms([]string{baseContent}, override.Transforms, transformersConfig)
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
	}
	return result
}
