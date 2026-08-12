package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
)

// SourceOutcome is the result of fetching + parsing one source.
type SourceOutcome struct {
	Label       string
	Type        string
	Entries     []ir.Entry
	Diagnostics []ir.Diagnostic
	Error       string
}

// CompileResult is the full result of compiling one rule.
type CompileResult struct {
	RuleID       string
	RuleName     string
	Sources      []SourceOutcome
	PreOps       []ir.Entry
	PostOps      []ir.Entry
	Merged       []ir.Entry
	OpsError     string
	Rendered     map[string][]byte // client_id -> rendered content
	RenderErrors map[string]string
}

// CompileRule runs the full pipeline for one rule: fetch sources -> parse ->
// ops -> merge -> render.
func CompileRule(
	ctx context.Context,
	rule config.RuleConfig,
	clients []config.ClientConfig,
	fetcher *Fetcher,
	preprocessor *PreprocessRunner,
	registry *render.Registry,
	geositeProviders map[string]*geosite.ProviderCache,
	refResults map[string][]ir.Entry,
	logger *slog.Logger,
) CompileResult {
	result := CompileResult{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		Rendered:     make(map[string][]byte),
		RenderErrors: make(map[string]string),
	}
	log := logger.With("rule_id", rule.ID, "rule_name", rule.Name)

	// Phase 1: Fetch and parse sources
	outcomes := make([]SourceOutcome, len(rule.Sources))
	var sourcesWG sync.WaitGroup
	for i, src := range rule.Sources {
		sourcesWG.Add(1)
		go func() {
			defer sourcesWG.Done()
			label := src.Label
			if label == "" {
				label = fmt.Sprintf("source[%d]", i)
			}
			outcome := fetchSource(ctx, src, fetcher, preprocessor, rule.Preprocess, geositeProviders, refResults, log)
			outcome.Label = label
			outcomes[i] = outcome
		}()
	}
	sourcesWG.Wait()

	var allEntries [][]ir.Entry
	for _, outcome := range outcomes {
		result.Sources = append(result.Sources, outcome)
		if outcome.Error != "" {
			log.Warn("source failed", "source", outcome.Label, "error", outcome.Error)
			continue
		}
		allEntries = append(allEntries, outcome.Entries)
	}

	// Phase 2: Merge sources
	merged := mergeEntries(allEntries, rule.Merge)
	merged = ir.Dedupe(merged)
	result.PreOps = cloneEntries(merged)

	// Phase 3: Apply ops
	var err error
	merged, err = applyOps(merged, rule.Ops)
	if err != nil {
		result.OpsError = err.Error()
		log.Error("ops failed", "error", err)
	}
	result.PostOps = cloneEntries(merged)
	result.Merged = merged

	// Phase 4: Render for each output client
	clientMap := make(map[string]config.ClientConfig, len(clients))
	for _, c := range clients {
		clientMap[c.ID] = c
	}

	for _, clientID := range rule.Outputs {
		client, ok := clientMap[clientID]
		if !ok {
			result.RenderErrors[clientID] = fmt.Sprintf("unknown client %q", clientID)
			continue
		}
		tmpl, ok := registry.Get(client.Template)
		if !ok {
			result.RenderErrors[clientID] = fmt.Sprintf("unknown template %q", client.Template)
			continue
		}
		rendered, rerr := render.Render(tmpl, merged)
		if rerr != nil {
			result.RenderErrors[clientID] = rerr.Error()
			log.Error("render failed", "client", clientID, "error", rerr)
			continue
		}
		result.Rendered[clientID] = rendered
	}

	return result
}

func fetchSource(
	ctx context.Context,
	src config.SourceConfig,
	fetcher *Fetcher,
	preprocessor *PreprocessRunner,
	preprocessScript string,
	geositeProviders map[string]*geosite.ProviderCache,
	refResults map[string][]ir.Entry,
	log *slog.Logger,
) SourceOutcome {
	srcType := src.SourceType()
	outcome := SourceOutcome{Type: srcType}

	switch srcType {
	case "url":
		res := fetcher.Fetch(ctx, src.URL)
		if res.Error != "" {
			outcome.Error = res.Error
			return outcome
		}
		content := res.Content
		if preprocessScript != "" {
			var err error
			content, err = preprocessor.Run(preprocessScript, content)
			if err != nil {
				outcome.Error = err.Error()
				return outcome
			}
		}
		rs, _, err := ir.Parse(content, src.Format)
		if err != nil {
			outcome.Error = err.Error()
			return outcome
		}
		outcome.Entries = rs.Entries
		outcome.Diagnostics = rs.Diagnostics

	case "local":
		content := src.Content
		format := src.Format
		if src.File != "" {
			data, err := os.ReadFile(src.File)
			if err != nil {
				outcome.Error = fmt.Sprintf("read local source %q: %v", src.File, err)
				return outcome
			}
			content = string(data)
			format = ""
		}
		if preprocessScript != "" {
			var err error
			content, err = preprocessor.Run(preprocessScript, content)
			if err != nil {
				outcome.Error = err.Error()
				return outcome
			}
		}
		rs, _, err := ir.Parse(content, format)
		if err != nil {
			outcome.Error = err.Error()
			return outcome
		}
		outcome.Entries = rs.Entries
		outcome.Diagnostics = rs.Diagnostics

	case "ref":
		entries, ok := refResults[src.Ref]
		if !ok {
			outcome.Error = fmt.Sprintf("ref %q not available (not yet compiled or failed)", src.Ref)
			return outcome
		}
		outcome.Entries = cloneEntries(entries)

	case "geosite":
		ref, err := src.ResolveGeositeRef()
		if err != nil {
			outcome.Error = fmt.Sprintf("invalid geosite ref: %v", err)
			return outcome
		}
		cache, ok := geositeProviders[ref.Provider]
		if !ok {
			outcome.Error = fmt.Sprintf("geosite provider %q not loaded", ref.Provider)
			return outcome
		}
		geositeEntries, err := geosite.ResolveEntries(cache, ref.List, ref.Attrs)
		if err != nil {
			outcome.Error = err.Error()
			return outcome
		}
		for _, ge := range geositeEntries {
			kind := geositeTypeToIRKind(ge.Type)
			if kind != "" {
				outcome.Entries = append(outcome.Entries, ir.Entry{Kind: kind, Value: ge.Value})
			} else {
				log.Warn("unknown geosite entry type", "type", ge.Type, "value", ge.Value)
			}
		}

	default:
		outcome.Error = fmt.Sprintf("unknown source type %q", srcType)
	}

	return outcome
}

func geositeTypeToIRKind(t geosite.EntryType) ir.Kind {
	switch t {
	case geosite.EntryDomain:
		return ir.KindDomainSuffix
	case geosite.EntryFull:
		return ir.KindDomain
	case geosite.EntryRegexp:
		return ir.KindDomainRegex
	case geosite.EntryKeyword:
		return ir.KindDomainKeyword
	default:
		return ""
	}
}

func mergeEntries(groups [][]ir.Entry, mergeCfg *config.MergeConfig) []ir.Entry {
	if len(groups) == 0 {
		return nil
	}
	if len(groups) == 1 {
		return groups[0]
	}

	strategy := "union"
	if mergeCfg != nil && mergeCfg.Strategy != "" {
		strategy = mergeCfg.Strategy
	}

	result := groups[0]
	for _, g := range groups[1:] {
		switch strategy {
		case "union":
			result = ir.Union(result, g)
		case "intersect":
			result = ir.Intersect(result, g)
		case "difference":
			result = ir.Difference(result, g)
		default:
			result = ir.Union(result, g)
		}
	}
	return result
}

func applyOps(entries []ir.Entry, ops []config.OpConfig) ([]ir.Entry, error) {
	for _, op := range ops {
		switch op.Type {
		case "include_kinds":
			kinds := make([]ir.Kind, len(op.Kinds))
			for i, k := range op.Kinds {
				kinds[i] = ir.Kind(k)
			}
			entries = ir.FilterKinds(entries, kinds, ir.FilterKeep)
		case "exclude_kinds":
			kinds := make([]ir.Kind, len(op.Kinds))
			for i, k := range op.Kinds {
				kinds[i] = ir.Kind(k)
			}
			entries = ir.FilterKinds(entries, kinds, ir.FilterRemove)
		case "filter_values":
			mode := ir.ValueMatchMode(op.Mode)
			if mode == "" {
				mode = ir.MatchKeyword
			}
			var err error
			entries, err = ir.FilterValues(entries, mode, op.Pattern, ir.FilterRemove)
			if err != nil {
				return entries, fmt.Errorf("filter_values: %w", err)
			}
		default:
			return entries, fmt.Errorf("unknown op type %q", op.Type)
		}
	}
	return entries, nil
}

func cloneEntries(entries []ir.Entry) []ir.Entry {
	if entries == nil {
		return nil
	}
	out := make([]ir.Entry, len(entries))
	copy(out, entries)
	return out
}
