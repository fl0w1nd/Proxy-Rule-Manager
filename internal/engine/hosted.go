package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geodata"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geohost"
	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

func hostedGeoDir(dataDir string) string {
	return filepath.Join(dataDir, "geo")
}

func hostedGeoFilePath(dataDir, kind, provider, asset string) (string, error) {
	if err := util.EnsureSafeSegment(kind, "geo kind"); err != nil {
		return "", err
	}
	if err := util.EnsureSafeSegment(provider, "geo provider"); err != nil {
		return "", err
	}
	if err := util.EnsureSafeSegment(asset, "geo asset"); err != nil {
		return "", err
	}
	return util.JoinInside(hostedGeoDir(dataDir), kind, provider, asset)
}

func hostedGeoRelPath(kind, provider, asset string) string {
	return "geo/" + kind + "/" + provider + "/" + asset
}

func collectHostedProviderNames(cfg *config.HostedGeoConfig) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		names = append(names, provider.Name)
	}
	return names
}

func hostedProviderNames(cfg *config.HostedGeoConfig) []string {
	if cfg == nil {
		return nil
	}
	var names []string
	for _, provider := range cfg.Providers {
		if provider.Host {
			names = append(names, provider.Name)
		}
	}
	return names
}

// geoDataKinds are the Geo data kinds an update scope can name directly, in
// publication order.
var geoDataKinds = []string{"geosite", "geoip", geohost.KindMMDB, geohost.KindASN}

// geoKindsForScope maps an update scope to the Geo data kinds it refreshes.
// Refresh, publication and reconciliation all derive from this mapping.
func geoKindsForScope(scope string) map[string]bool {
	kinds := make(map[string]bool, len(geoDataKinds))
	switch scope {
	case "all":
		for _, kind := range geoDataKinds {
			kinds[kind] = true
		}
	case "geosite", "geoip", geohost.KindMMDB, geohost.KindASN:
		kinds[scope] = true
	}
	return kinds
}

// hostedKindsForScope lists the published Geo kinds a scope owns.
func hostedKindsForScope(scope string) []string {
	kinds := geoKindsForScope(scope)
	ordered := make([]string, 0, len(kinds))
	for _, kind := range geoDataKinds {
		if kinds[kind] {
			ordered = append(ordered, kind)
		}
	}
	return ordered
}

func (e *UpdateEngine) refreshHostedGeo(ctx context.Context, kinds map[string]bool, result *UpdateResult) {
	if kinds[geohost.KindMMDB] {
		e.refreshHostedKind(ctx, geohost.KindMMDB, "MMDB", e.MMDB, collectHostedProviderNames(e.currentConfig().MMDB), result)
	}
	if kinds[geohost.KindASN] {
		e.refreshHostedKind(ctx, geohost.KindASN, "ASN", e.ASN, collectHostedProviderNames(e.currentConfig().ASN), result)
	}
}

func (e *UpdateEngine) refreshHostedKind(ctx context.Context, kind, label string, mgr *geohost.Manager, names []string, result *UpdateResult) {
	if len(names) == 0 {
		return
	}
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: kind + "_refresh", Status: "running", Message: "正在刷新 " + label})
	previous := map[string]*geohost.Cache{}
	if mgr != nil {
		for _, name := range names {
			previous[name], _ = mgr.Read(name)
		}
	}
	now := time.Now()
	caches, failed, fetchErrors := refreshGeoProviders(ctx, e.currentConfig(), mgr, e.Logger, kind, label, names)
	for _, name := range names {
		if failed[name] {
			message := fmt.Sprintf("%s provider %q refresh: %s", kind, name, fetchErrors[name])
			result.addError(kind+"_refresh", name, message)
			e.recordHostedGeoUpdate(kind, name, state.ProviderFailed, now, result)
			reportProgress(ctx, ProgressEvent{Kind: ProgressError, Stage: kind + "_refresh", Subject: name, Status: "failed", Message: message})
			continue
		}
		status := state.ProviderUpdated
		if old, current := previous[name], caches[name]; old != nil && current != nil && old.ResolvedVersion == current.ResolvedVersion {
			status = state.ProviderUnchanged
		}
		e.recordHostedGeoUpdate(kind, name, status, now, result)
		reportProgress(ctx, ProgressEvent{Kind: ProgressSuccess, Stage: kind + "_refresh", Subject: name, Status: status, Message: fmt.Sprintf("%s %s · %s", label, name, status)})
	}
}

func (e *UpdateEngine) recordHostedGeoUpdate(kind, provider, status string, checkedAt time.Time, result *UpdateResult) {
	if e.State == nil {
		return
	}
	if err := e.State.SetHostedGeoUpdate(kind, provider, status, checkedAt); err != nil {
		result.addError(kind+"_refresh", provider, err.Error())
	}
}

// rawAssetSource is the slice of a Geo provider manager needed to publish
// original database files.
type rawAssetSource interface {
	Source(provider string) (geodata.Source, bool)
	RawPath(provider string) (string, error)
}

type hostedPublication struct {
	kind    string
	manager rawAssetSource
	names   []string
}

func (e *UpdateEngine) publishHostedGeoFiles(ctx context.Context, kinds map[string]bool, result *UpdateResult, expected map[string]struct{}) {
	cfg := e.currentConfig()
	publications := make([]hostedPublication, 0, len(geoDataKinds))
	if e.Geosite != nil {
		publications = append(publications, hostedPublication{"geosite", e.Geosite, hostedGeositeNames(cfg.Geosite)})
	}
	if e.GeoIP != nil {
		publications = append(publications, hostedPublication{"geoip", e.GeoIP, hostedGeoIPNames(cfg.GeoIP)})
	}
	if e.MMDB != nil {
		publications = append(publications, hostedPublication{geohost.KindMMDB, e.MMDB, hostedProviderNames(cfg.MMDB)})
	}
	if e.ASN != nil {
		publications = append(publications, hostedPublication{geohost.KindASN, e.ASN, hostedProviderNames(cfg.ASN)})
	}
	for _, publication := range publications {
		if !kinds[publication.kind] {
			continue
		}
		for _, name := range publication.names {
			publishHostedProvider(ctx, e, publication.kind, name, publication.manager, result, expected)
		}
	}
}

func hostedGeositeNames(cfg *config.GeositeConfig) []string {
	if cfg == nil {
		return nil
	}
	var names []string
	for _, provider := range cfg.Providers {
		if provider.Host {
			names = append(names, provider.Name)
		}
	}
	return names
}

func hostedGeoIPNames(cfg *config.GeoIPConfig) []string {
	if cfg == nil {
		return nil
	}
	var names []string
	for _, provider := range cfg.Providers {
		if provider.Host {
			names = append(names, provider.Name)
		}
	}
	return names
}

func publishHostedProvider(ctx context.Context, e *UpdateEngine, kind, provider string, mgr rawAssetSource, result *UpdateResult, expected map[string]struct{}) {
	if ctx.Err() != nil {
		return
	}
	if mgr == nil {
		result.addError(kind+"_host", provider, fmt.Sprintf("%s provider %q not loaded", kind, provider))
		return
	}
	source, ok := mgr.Source(provider)
	if !ok {
		result.addError(kind+"_host", provider, fmt.Sprintf("unsupported %s provider %q", kind, provider))
		return
	}
	src, err := mgr.RawPath(provider)
	if err != nil {
		result.addError(kind+"_host", provider, err.Error())
		return
	}
	dst, err := hostedGeoFilePath(e.DataDir, kind, provider, source.Asset)
	if err != nil {
		result.addError(kind+"_host", provider, err.Error())
		return
	}
	if _, err := os.Stat(src); err != nil {
		result.addError(kind+"_host", provider, fmt.Sprintf("%s provider %q has no local database; run an update to download it", kind, provider))
		return
	}
	reportProgress(ctx, ProgressEvent{Kind: ProgressInfo, Stage: kind + "_host", Status: "running", Subject: provider, Message: fmt.Sprintf("正在发布 %s %s", kind, source.Asset)})
	if err := publishHostedFile(src, dst); err != nil {
		result.addError(kind+"_host", provider, err.Error())
		return
	}
	expected[filepath.Clean(dst)] = struct{}{}
	result.Artifacts++
}

// publishHostedFile writes the cached original asset to dst. The cache and the
// published copy live in the same data directory, so a hard link keeps a single
// copy on disk; file systems without hard links fall back to a plain copy.
func publishHostedFile(src, dst string) error {
	if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	if err := linkHostedFile(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return util.AtomicWriteStream(dst, in)
}

func linkHostedFile(src, dst string) error {
	temp := dst + ".link-tmp"
	_ = os.Remove(temp)
	if err := os.Link(src, temp); err != nil {
		return err
	}
	if err := os.Rename(temp, dst); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func (e *UpdateEngine) hostedGeoFiles() []site.GeoFile {
	cfg := e.currentConfig()
	files := make([]site.GeoFile, 0)
	for _, name := range hostedGeositeNames(cfg.Geosite) {
		files = append(files, hostedGeoFile(e, "geosite", name, e.Geosite)...)
	}
	for _, name := range hostedGeoIPNames(cfg.GeoIP) {
		files = append(files, hostedGeoFile(e, "geoip", name, e.GeoIP)...)
	}
	for _, name := range hostedProviderNames(cfg.MMDB) {
		files = append(files, hostedGeoFile(e, geohost.KindMMDB, name, e.MMDB)...)
	}
	for _, name := range hostedProviderNames(cfg.ASN) {
		files = append(files, hostedGeoFile(e, geohost.KindASN, name, e.ASN)...)
	}
	return files
}

func hostedGeoFile[E any](e *UpdateEngine, kind, provider string, mgr *geodata.Manager[E]) []site.GeoFile {
	if mgr == nil {
		return nil
	}
	source, ok := mgr.Source(provider)
	if !ok {
		return nil
	}
	path, err := hostedGeoFilePath(e.DataDir, kind, provider, source.Asset)
	if err != nil {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	file := site.GeoFile{
		Kind: kind, Provider: provider, Name: source.Asset, Path: hostedGeoRelPath(kind, provider, source.Asset), Size: info.Size(),
	}
	if cache, err := mgr.Read(provider); err == nil && cache != nil {
		file.Version = cache.ResolvedVersion
	}
	return []site.GeoFile{file}
}

func (e *UpdateEngine) HostedProviderSummaries(kind string) []GeoProviderSummary {
	cfg := e.currentConfig()
	var names []string
	var mgr *geohost.Manager
	switch kind {
	case geohost.KindMMDB:
		names = collectHostedProviderNames(cfg.MMDB)
		mgr = e.MMDB
	case geohost.KindASN:
		names = collectHostedProviderNames(cfg.ASN)
		mgr = e.ASN
	default:
		return nil
	}
	out := make([]GeoProviderSummary, 0, len(names))
	for _, name := range names {
		summary := GeoProviderSummary{Name: name}
		if mgr != nil {
			cache, err := mgr.Read(name)
			var (
				recResult   string
				recChecked  time.Time
				hasRecorded bool
			)
			if e.State != nil {
				recResult, recChecked, hasRecorded = e.State.HostedGeoUpdate(kind, name)
			}
			if err == nil && cache != nil {
				summary.Version = cache.ResolvedVersion
				if hasRecorded {
					summary.Result = recResult
					summary.CheckedAt = recChecked
				} else {
					summary.Result = state.ProviderUpdated
					if checkedAt, parseErr := time.Parse(time.RFC3339, cache.FetchedAt); parseErr == nil {
						summary.CheckedAt = checkedAt
					}
				}
			} else if hasRecorded {
				summary.Result = recResult
				summary.CheckedAt = recChecked
			}
			if source, ok := mgr.Source(name); ok {
				if path, err := hostedGeoFilePath(e.DataDir, kind, name, source.Asset); err == nil {
					if _, err := os.Stat(path); err == nil {
						summary.Files = 1
					}
				}
			}
		}
		out = append(out, summary)
	}
	return out
}

func ReconcileHostedGeo(dataDir string, kinds []string, expected map[string]struct{}) error {
	for _, kind := range kinds {
		if err := reconcileArtifacts(filepath.Join(hostedGeoDir(dataDir), kind), expected); err != nil {
			return err
		}
	}
	return nil
}

func countHostedGeo(dataDir string) (int, error) {
	root := hostedGeoDir(dataDir)
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return count, err
}
