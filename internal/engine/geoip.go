package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

func (e *UpdateEngine) loadGeoIP(ctx context.Context, rules []config.RuleConfig, partial bool, result *UpdateResult) map[string]*geoip.ProviderCache {
	cfg := e.currentConfig()
	if partial {
		caches := map[string]*geoip.ProviderCache{}
		if e.GeoIP == nil {
			return caches
		}
		for _, name := range collectGeoIPProviderNames(&config.Config{Rules: rules}) {
			cache, err := e.GeoIP.Read(name)
			if err == nil && cache != nil {
				caches[name] = cache
			}
		}
		return caches
	}
	names := collectGeoIPProviderNames(cfg)
	previous := map[string]*geoip.ProviderCache{}
	if e.GeoIP != nil {
		for _, name := range names {
			previous[name], _ = e.GeoIP.Read(name)
		}
	}
	if len(names) > 0 {
		reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "geoip_refresh", Status: "running", Message: "正在刷新 GeoIP"})
	}
	caches, failed, fetchErrors := refreshGeoIPProviders(ctx, cfg, e.GeoIP, e.Logger)
	for _, name := range names {
		status := state.ProviderUpdated
		if failed[name] {
			status = state.ProviderFailed
			message := fmt.Sprintf("geoip provider %q refresh: %s", name, fetchErrors[name])
			result.addError("geoip_refresh", name, message)
			reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: "geoip_refresh", Subject: name, Status: "failed", Message: message})
		} else if old, current := previous[name], caches[name]; old != nil && current != nil {
			if old.ResolvedVersion == current.ResolvedVersion {
				status = state.ProviderUnchanged
			}
			for _, list := range old.Catalog {
				if _, ok := current.Entries[list]; !ok {
					warning := fmt.Sprintf("GeoIP %s/%s 已从上游移除", name, list)
					result.addWarning(warning)
					reportProgress(ctx, ProgressEvent{Kind: ProgressWarning, Stage: "geoip_refresh", Subject: name, Status: "warning", Message: warning})
				}
			}
		}
		e.State.SetGeoIPUpdate(name, status, time.Now())
	}
	return caches
}

func (e *UpdateEngine) updateGeoIPPublications(ctx context.Context, providers map[string]*geoip.ProviderCache, result *UpdateResult, expectedPaths map[string]struct{}) {
	for _, provider := range e.currentConfig().GeoIP.Providers {
		cache := providers[provider.Name]
		if cache == nil {
			result.addError("geoip_publish", provider.Name, "geoip provider not loaded: "+provider.Name)
			continue
		}
		for index, list := range cache.Catalog {
			if ctx.Err() != nil {
				return
			}
			reference := provider.Name + "/" + list
			reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: "geoip_publish", Status: "running", Current: index + 1, Total: len(cache.Catalog), Subject: reference, Message: fmt.Sprintf("GeoIP %s · %d / %d · %s", provider.Name, index+1, len(cache.Catalog), list)})
			entries, err := geoip.ResolveIR(cache, list)
			if err != nil {
				result.addError("geoip_publish", reference, err.Error())
				continue
			}
			e.publishGeoEntries("geoip", reference, provider.Name, entries, provider.Clients, result, expectedPaths, nil)
		}
	}
}

func (e *UpdateEngine) GeoIPProviderSummaries() []GeoProviderSummary {
	return e.rebuildGeoIPStats().summaries()
}
func (e *UpdateEngine) rebuildGeoIPStats() *geoStats {
	stats := newGeoStats()
	cfg := e.currentConfig()
	if cfg.GeoIP == nil {
		return stats
	}
	for _, provider := range cfg.GeoIP.Providers {
		status, checkedAt, _ := e.State.GeoIPUpdate(provider.Name)
		stats.setMeta(provider.Name, "", status, checkedAt)
		if e.GeoIP == nil {
			continue
		}
		cache, err := e.GeoIP.Read(provider.Name)
		if err != nil || cache == nil {
			continue
		}
		stats.setMeta(provider.Name, cache.ResolvedVersion, status, checkedAt)
		for _, name := range cache.Catalog {
			stats.recordVariant(provider.Name, name, nil, len(cache.Entries[name]))
		}
		targets := config.ExpandSelectedTargets(cfg.Clients, provider.Clients)
		for range countGeoArtifacts(e.DataDir, "geoip", provider.Name, targets) {
			stats.recordFile(provider.Name)
		}
	}
	return stats
}

func refreshGeoIPProviders(ctx context.Context, cfg *config.Config, mgr *geoip.Manager, logger *slog.Logger) (map[string]*geoip.ProviderCache, map[string]bool, map[string]string) {
	return refreshGeoProviders(ctx, cfg, mgr, logger, "geoip", "GeoIP", collectGeoIPProviderNames(cfg))
}

func collectGeoIPProviderNames(cfg *config.Config) []string {
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
			if src.SourceType() != "geoip" {
				continue
			}
			ref, err := src.ResolveGeoIPRef()
			if err == nil {
				addProvider(ref.Provider)
			}
		}
	}
	if cfg.GeoIP != nil {
		for _, p := range cfg.GeoIP.Providers {
			addProvider(p.Name)
		}
	}
	return providers
}
