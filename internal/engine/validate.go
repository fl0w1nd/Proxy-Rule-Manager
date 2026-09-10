package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geodata"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
)

// missingProviderCacheMessage is the validation wording for a provider that
// has never been downloaded.
func missingProviderCacheMessage(provider, list string) string {
	scope := fmt.Sprintf("provider %q", provider)
	if list != "" {
		scope = fmt.Sprintf("provider %q for %q", provider, list)
	}
	return scope + " has no local data; run an update to download it"
}

// ValidateGeositeRefs loads geosite provider caches from disk and validates
// all geosite references in the config. Geo data is installed by updates, so
// a provider that was never downloaded is reported instead of fetched.
func ValidateGeositeRefs(
	_ context.Context,
	cfg *config.Config,
	mgr *geosite.Manager,
	logger *slog.Logger,
) []config.ConfigError {
	if mgr == nil {
		return nil
	}

	var errs []config.ConfigError
	caches := readGeoCaches(mgr, collectProviderNames(cfg), logger, "geosite")

	for i, rule := range cfg.Rules {
		for sourcePath, src := range config.WalkSources(rule.Sources) {
			if src.SourceType() != "geosite" {
				continue
			}
			path := fmt.Sprintf("rules[%d].%s", i, sourcePath)
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
				errs = append(errs, cfg.ErrorAt(path, missingProviderCacheMessage(ref.Provider, ref.FormatRef())))
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
				errs = append(errs, cfg.ErrorAt(path, missingProviderCacheMessage(prov.Name, "")))
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

// readGeoCaches loads provider caches from disk. Validation never downloads
// geo data: an update installs it, so a missing cache is reported as such.
func readGeoCaches[E any](
	mgr *geodata.Manager[E],
	names []string,
	logger *slog.Logger,
	kind string,
) map[string]*geodata.Cache[E] {
	caches := make(map[string]*geodata.Cache[E])
	if mgr == nil {
		return caches
	}
	for _, name := range names {
		cache, err := mgr.Read(name)
		if err != nil {
			logger.Warn(kind+" provider cache unreadable", "provider", name, "error", err)
		}
		if cache != nil {
			caches[name] = cache
		}
	}
	return caches
}
