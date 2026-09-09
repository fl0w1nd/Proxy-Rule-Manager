package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
)

// ValidateGeoIPRefs loads geoip provider caches and validates all geoip
// references in the config. Returns per-reference diagnostics (as ConfigError)
// without blocking other rules from working.
func ValidateGeoIPRefs(
	ctx context.Context,
	cfg *config.Config,
	mgr *geoip.Manager,
	logger *slog.Logger,
) []config.ConfigError {
	if mgr == nil {
		return nil
	}

	var errs []config.ConfigError
	caches := loadGeoCaches(ctx, mgr, collectGeoIPProviderNames(cfg), logger, "geoip")

	for i, rule := range cfg.Rules {
		for sourcePath, src := range config.WalkSources(rule.Sources) {
			if src.SourceType() != "geoip" {
				continue
			}
			path := fmt.Sprintf("rules[%d].%s", i, sourcePath)
			if src.GeoIP != "" {
				path += ".geoip"
			}
			ref, err := src.ResolveGeoIPRef()
			if err != nil {
				errs = append(errs, cfg.ErrorAt(path, fmt.Sprintf("invalid geoip ref: %v", err)))
				continue
			}
			cache, ok := caches[ref.Provider]
			if !ok {
				errs = append(errs, cfg.ErrorAt(path,
					fmt.Sprintf("provider %q cache unavailable, cannot validate %q", ref.Provider, ref.FormatRef())))
				continue
			}
			if _, err := geoip.ResolveEntries(cache, ref.List); err != nil {
				errs = append(errs, cfg.ErrorAt(path, err.Error()))
			}
		}
	}

	if cfg.GeoIP != nil {
		for i, prov := range cfg.GeoIP.Providers {
			path := fmt.Sprintf("geoip.providers[%d]", i)
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
