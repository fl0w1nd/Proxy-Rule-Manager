package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
)

// ValidateGeositeRefs loads geosite provider caches and validates all geosite
// references in the config. Returns per-reference diagnostics (as ConfigError)
// without blocking other rules from working.
func ValidateGeositeRefs(
	ctx context.Context,
	cfg *config.Config,
	mgr *geosite.Manager,
	logger *slog.Logger,
) []config.ConfigError {
	if mgr == nil {
		return nil
	}

	var errs []config.ConfigError
	caches := loadAllProviderCaches(ctx, cfg, mgr, logger, &errs)

	for i, rule := range cfg.Rules {
		for j, src := range rule.Sources {
			if src.SourceType() != "geosite" {
				continue
			}
			path := fmt.Sprintf("rules[%d].sources[%d]", i, j)
			if src.Geosite != "" {
				path += ".geosite"
			}
			ref, err := src.ResolveGeositeRef()
			if err != nil {
				errs = append(errs, cfg.ErrorAt(path, fmt.Sprintf("invalid geosite ref: %v", err)))
				continue
			}
			cache, ok := caches[ref.Provider]
			if !ok {
				errs = append(errs, cfg.ErrorAt(path,
					fmt.Sprintf("provider %q cache unavailable, cannot validate %q", ref.Provider, ref.FormatRef())))
				continue
			}
			if err := geosite.ValidateRef(cache, ref); err != nil {
				errs = append(errs, cfg.ErrorAt(path, err.Error()))
			}
		}
	}

	if cfg.Geosite != nil {
		for i, prov := range cfg.Geosite.Providers {
			path := fmt.Sprintf("geosite.providers[%d]", i)
			cache, ok := caches[prov.Name]
			if !ok {
				errs = append(errs, cfg.ErrorAt(path,
					fmt.Sprintf("provider %q cache unavailable", prov.Name)))
				continue
			}
			if len(cache.Catalog) == 0 {
				errs = append(errs, cfg.ErrorAt(path,
					fmt.Sprintf("provider %q has empty catalog", prov.Name)))
			}
		}
	}

	return errs
}

func loadAllProviderCaches(
	ctx context.Context,
	cfg *config.Config,
	mgr *geosite.Manager,
	logger *slog.Logger,
	errs *[]config.ConfigError,
) map[string]*geosite.ProviderCache {
	caches := make(map[string]*geosite.ProviderCache)
	providers := collectProviderNames(cfg)
	for _, name := range providers {
		cache, err := mgr.Ensure(ctx, name)
		if err != nil {
			logger.Warn("geosite provider unavailable for validation", "provider", name, "error", err)
			continue
		}
		if cache != nil {
			caches[name] = cache
		}
	}
	return caches
}
