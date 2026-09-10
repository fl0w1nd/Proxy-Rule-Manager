package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/render"
)

// PreviewReport shows the per-stage pipeline outcome for one rule.
type PreviewReport struct {
	RuleID   string
	RuleName string
	Sources  []SourceOutcome
	PreOps   []ir.Entry
	PostOps  []ir.Entry
	Merged   []ir.Entry
	OpsError string
	OpsDiff  ir.EntryDiff

	// Artifacts are rendered outputs for every configured client target.
	Artifacts      map[string][]byte
	ArtifactErrors map[string]string

	// Rendered output for an explicit format or variant target.
	RenderedTarget string
	RenderedOutput []byte
	RenderError    string
}

// MissingProviderDataError reports a geo provider without a local cache. Geo
// data is installed by updates only, so a preview reports the missing data
// instead of downloading it.
type MissingProviderDataError struct {
	Kind     string // "geosite" or "geoip"
	Provider string
}

func (e *MissingProviderDataError) Error() string {
	return fmt.Sprintf("%s provider %q has no local data; run an update to download it", e.Kind, e.Provider)
}

// Preview compiles a rule without persisting artifacts, returning a
// per-stage report for CLI inspection.
func Preview(
	ctx context.Context,
	cfg *config.Config,
	dataDir string,
	ruleID string,
	targetID string,
	registry *render.Registry,
	fetcher *Fetcher,
	preprocessor *PreprocessRunner,
	geositeMgr *geosite.Manager,
	geoipMgr *geoip.Manager,
	logger *slog.Logger,
) (*PreviewReport, error) {
	var rule *config.RuleConfig
	for i := range cfg.Rules {
		if cfg.Rules[i].ID == ruleID {
			rule = &cfg.Rules[i]
			break
		}
	}
	if rule == nil {
		return nil, &RuleNotFoundError{ID: ruleID}
	}

	selected := collectPreviewDependencies(cfg.Rules, rule.ID)
	sorted, err := TopologicalSort(selected, false)
	if err != nil {
		return nil, fmt.Errorf("resolve preview dependencies: %w", err)
	}
	geositeProviders, geoipProviders, err := previewGeoData(selected, geositeMgr, geoipMgr)
	if err != nil {
		return nil, err
	}
	refResults := make(map[string][]ir.Entry)
	var cr CompileResult
	for _, current := range sorted {
		compileRule := current
		if current.ID != rule.ID {
			compileRule.Outputs = nil
		}
		compiled := CompileRule(
			ctx,
			compileRule,
			cfg.Clients,
			fetcher,
			preprocessor,
			registry,
			geositeProviders,
			geoipProviders,
			refResults,
			config.NewLocalFileResolver(dataDir),
			logger,
		)
		if current.ID == rule.ID {
			cr = compiled
		}
		if compileResultCanBeReferenced(compiled) {
			refResults[current.ID] = compiled.Merged
		}
	}

	report := &PreviewReport{
		RuleID:         rule.ID,
		RuleName:       rule.Name,
		Sources:        cr.Sources,
		PreOps:         cr.PreOps,
		PostOps:        cr.PostOps,
		Merged:         cr.Merged,
		OpsError:       cr.OpsError,
		OpsDiff:        ir.Diff(cr.PreOps, cr.PostOps),
		Artifacts:      cr.Rendered,
		ArtifactErrors: cr.RenderErrors,
	}

	if targetID != "" {
		target, ok := config.FindOutputTarget(cfg.Clients, targetID)
		if !ok {
			report.RenderError = "unknown output target: " + targetID
		} else {
			tmpl, ok := registry.Get(target.Template)
			if !ok {
				report.RenderError = "unknown template: " + target.Template
			} else {
				entries := cloneEntries(cr.Merged)
				entries, err = applyOps(entries, target.Ops)
				if err != nil {
					report.RenderError = err.Error()
					return report, nil
				}
				output, err := render.Render(tmpl, entries)
				if err != nil {
					report.RenderError = err.Error()
				} else {
					report.RenderedTarget = targetID
					report.RenderedOutput = output
				}
			}
		}
	}

	return report, nil
}

// previewGeoData loads the on-disk caches a preview needs. Updates own geo
// data downloads, so a provider without local data is reported instead of
// fetched: a preview reads cache and never waits on the network.
func previewGeoData(
	rules []config.RuleConfig,
	geositeMgr *geosite.Manager,
	geoipMgr *geoip.Manager,
) (map[string]*geosite.ProviderCache, map[string]*geoip.ProviderCache, error) {
	referenced := &config.Config{Rules: rules}

	geositeNames := collectProviderNames(referenced)
	geositeCache := readCachedGeoProviders(geositeNames, geositeMgr)
	for _, name := range geositeNames {
		if geositeCache[name] == nil {
			return nil, nil, &MissingProviderDataError{Kind: "geosite", Provider: name}
		}
	}

	geoipNames := collectGeoIPProviderNames(referenced)
	geoipCache := readCachedGeoProviders(geoipNames, geoipMgr)
	for _, name := range geoipNames {
		if geoipCache[name] == nil {
			return nil, nil, &MissingProviderDataError{Kind: "geoip", Provider: name}
		}
	}

	return geositeCache, geoipCache, nil
}

func collectPreviewDependencies(rules []config.RuleConfig, targetID string) []config.RuleConfig {
	byID := make(map[string]config.RuleConfig, len(rules))
	for _, rule := range rules {
		byID[rule.ID] = rule
	}
	needed := map[string]struct{}{targetID: {}}
	var collect func(string)
	collect = func(id string) {
		rule, ok := byID[id]
		if !ok {
			return
		}
		for dependency := range ExtractDependencies(&rule) {
			if _, ok := needed[dependency]; ok {
				continue
			}
			needed[dependency] = struct{}{}
			collect(dependency)
		}
	}
	collect(targetID)

	selected := make([]config.RuleConfig, 0, len(needed))
	for _, rule := range rules {
		if _, ok := needed[rule.ID]; ok {
			selected = append(selected, rule)
		}
	}
	return selected
}

func compileResultCanBeReferenced(result CompileResult) bool {
	if result.OpsError != "" {
		return false
	}
	for _, source := range result.Sources {
		if source.Error != "" || len(source.Diagnostics) > 0 {
			return false
		}
	}
	return true
}

// RuleNotFoundError indicates the requested rule doesn't exist in config.
type RuleNotFoundError struct {
	ID string
}

func (e *RuleNotFoundError) Error() string {
	return "rule ID not found: " + e.ID
}
