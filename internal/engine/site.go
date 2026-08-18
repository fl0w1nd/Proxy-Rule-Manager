package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/site"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

// diffSampleLimit caps each side of a logical rule diff stored in update
// history. Exact totals are retained separately from these display lines.
const diffSampleLimit = 100

// ruleSiteInfo aggregates everything the generated public page needs for one rule.
type ruleSiteInfo struct {
	id             string
	name           string
	description    string
	tags           []string
	entries        int
	files          []site.RuleFile
	added, removed int
	addedSamples   []string
	removedSamples []string
}

func newRuleSiteInfo(rule config.RuleConfig, cr CompileResult) *ruleSiteInfo {
	return &ruleSiteInfo{
		id:          rule.ID,
		name:        rule.Name,
		description: rule.Description,
		tags:        rule.Tags,
		entries:     len(cr.Merged),
	}
}

// persistedRuleSiteInfo rebuilds page data for a rule not compiled in the
// current run from the
// persisted snapshot and the on-disk artifacts.
func (e *UpdateEngine) persistedRuleSiteInfo(rule config.RuleConfig) *ruleSiteInfo {
	info := &ruleSiteInfo{
		id:          rule.ID,
		name:        rule.Name,
		description: rule.Description,
		tags:        rule.Tags,
	}
	if entries, exists, err := e.State.LoadSnapshotIfExists(rule.ID); err == nil && exists {
		info.entries = len(entries)
	}
	for _, target := range config.ExpandSelectedTargets(e.currentConfig().Clients, rule.Outputs) {
		rel, err := e.ruleFileRelPath(rule.ID, target.ID)
		if err != nil {
			continue
		}
		abs := filepath.Join(e.DataDir, filepath.FromSlash(rel))
		st, err := os.Stat(abs)
		if err != nil {
			continue
		}
		icon := site.ResolveClientIcon(target.Icon, target.ClientID)
		info.files = append(info.files, site.RuleFile{
			Client: target.ID,
			Icon:   icon,
			Path:   rel,
			Size:   st.Size(),
		})
	}
	return info
}

// ruleFileRelPath builds the site-relative artifact URL path for a rule and
// client: rules/<client>/<rule-id><ext>.
func (e *UpdateEngine) ruleFileRelPath(ruleID, clientID string) (string, error) {
	target, ok := config.FindOutputTarget(e.currentConfig().Clients, clientID)
	if !ok {
		return "", os.ErrNotExist
	}
	ext := ".list"
	if tmpl, ok := e.Registry.Get(target.Template); ok {
		ext = tmpl.Extension
	}
	return "rules/" + clientID + "/" + ruleID + ext, nil
}

// siteClients describes every configured output client for the pages, marking
// which views each client actually has content for: rules it is an output of,
// and geosite providers that publish to it.
func (e *UpdateEngine) siteClients() []site.Client {
	ruleClients := make(map[string]bool)
	cfg := e.currentConfig()
	for _, rule := range cfg.Rules {
		for _, target := range config.ExpandSelectedTargets(cfg.Clients, rule.Outputs) {
			ruleClients[target.ID] = true
		}
	}
	geoClients := make(map[string]bool)
	if cfg.Geosite != nil {
		for _, p := range cfg.Geosite.Providers {
			for _, target := range config.ExpandSelectedTargets(cfg.Clients, p.Clients) {
				geoClients[target.ID] = true
			}
		}
	}
	out := make([]site.Client, 0, len(cfg.Clients))
	for _, c := range cfg.Clients {
		name := c.Name
		if name == "" {
			name = c.ID
		}
		client := site.Client{
			ID: c.ID, Name: name, Icon: site.ResolveClientIcon(c.Icon, c.ID),
		}
		for _, target := range config.ExpandClientTargets(c) {
			ext := ".list"
			if tmpl, ok := e.Registry.Get(target.Template); ok {
				ext = tmpl.Extension
			}
			option := site.ClientOption{
				ID: target.ID, Name: target.OptionName, Ext: ext,
				Rules: ruleClients[target.ID], Geosite: geoClients[target.ID],
			}
			client.Options = append(client.Options, option)
			client.Rules = client.Rules || option.Rules
			client.Geosite = client.Geosite || option.Geosite
		}
		out = append(out, client)
	}
	return out
}

// geositeStats accumulates the full published geosite catalog during an update,
// plus per-provider metadata for the admin board.
type geositeStats struct {
	providers map[string]map[string]*geositeListStat
	meta      map[string]*geositeProviderMeta
}

type geositeListStat struct {
	entries  int
	variants map[string]int // attr -> entries
}

type geositeProviderMeta struct {
	version   string
	result    string
	checkedAt time.Time
	files     int
}

// GeositeProviderSummary is the management API view of one configured provider.
type GeositeProviderSummary struct {
	Name      string
	Version   string
	Result    string
	CheckedAt time.Time
	Lists     int
	Variants  int
	Entries   int
	Files     int
}

func newGeositeStats() *geositeStats {
	return &geositeStats{
		providers: make(map[string]map[string]*geositeListStat),
		meta:      make(map[string]*geositeProviderMeta),
	}
}

// setMeta records provider version and the latest fetch result.
func (s *geositeStats) setMeta(provider, version, result string, checkedAt time.Time) {
	s.meta[provider] = &geositeProviderMeta{version: version, result: result, checkedAt: checkedAt}
}

func (s *geositeStats) recordFile(provider string) {
	m, ok := s.meta[provider]
	if !ok {
		m = &geositeProviderMeta{}
		s.meta[provider] = m
	}
	m.files++
}

// recordVariant records one published variant. An empty attrs slice is the
// full list; otherwise the joined attrs key identifies the variant.
func (s *geositeStats) recordVariant(provider, list string, attrs []string, entries int) {
	lists, ok := s.providers[provider]
	if !ok {
		lists = make(map[string]*geositeListStat)
		s.providers[provider] = lists
	}
	st, ok := lists[list]
	if !ok {
		st = &geositeListStat{variants: make(map[string]int)}
		lists[list] = st
	}
	if len(attrs) == 0 {
		st.entries = entries
		return
	}
	st.variants[strings.Join(attrs, ",")] = entries
}

func (s *geositeStats) catalog() []site.GeositeCatalog {
	out := make([]site.GeositeCatalog, 0, len(s.providers))
	for provider, lists := range s.providers {
		cat := site.GeositeCatalog{Provider: provider}
		for name, st := range lists {
			gl := site.GeositeList{Name: name, Entries: st.entries}
			for attr, entries := range st.variants {
				gl.Variants = append(gl.Variants, site.GeositeVariant{Attr: attr, Entries: entries})
			}
			sort.Slice(gl.Variants, func(i, j int) bool { return gl.Variants[i].Attr < gl.Variants[j].Attr })
			cat.Lists = append(cat.Lists, gl)
		}
		sort.Slice(cat.Lists, func(i, j int) bool { return cat.Lists[i].Name < cat.Lists[j].Name })
		out = append(out, cat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

// summaries builds per-provider API summaries, merging publication stats with cache metadata.
func (s *geositeStats) summaries() []GeositeProviderSummary {
	names := make(map[string]bool)
	for name := range s.providers {
		names[name] = true
	}
	for name := range s.meta {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	out := make([]GeositeProviderSummary, 0, len(sorted))
	for _, name := range sorted {
		p := GeositeProviderSummary{Name: name}
		if m, ok := s.meta[name]; ok {
			p.Version = m.version
			p.Result = m.result
			p.CheckedAt = m.checkedAt
			p.Files = m.files
		}
		for _, l := range s.providers[name] {
			p.Lists++
			p.Variants += len(l.variants)
			p.Entries += l.entries
		}
		out = append(out, p)
	}
	return out
}

// EnsureSite guarantees that the generated public site matches
// the running binary: builtin icons are reconciled with the embedded set
// (user-modified icons are preserved), and when pages are missing or the
// embedded asset fingerprint changed, the public page is re-rendered from
// persisted state. Called at serve startup so a fresh deployment serves a
// complete public site before the first update.
func (e *UpdateEngine) EnsureSite() error {
	staticDir := filepath.Join(e.DataDir, site.StaticDir)
	res, err := site.UpdateBuiltinAssets(staticDir)
	if err != nil {
		return fmt.Errorf("update builtin assets: %w", err)
	}
	for _, name := range res.Skipped {
		e.Logger.Info("builtin icon preserved (user-modified)", "icon", name)
	}
	if err := removeLegacyAdmin(staticDir); err != nil {
		return err
	}
	st, statErr := os.Stat(filepath.Join(staticDir, site.IndexFile))
	pageExists := statErr == nil && !st.IsDir()
	if pageExists && !res.FingerprintChanged {
		return nil
	}
	if err := e.RebuildSite(); err != nil {
		return fmt.Errorf("rebuild site: %w", err)
	}
	reason := "pages missing"
	if res.FingerprintChanged {
		reason = "asset fingerprint changed"
	}
	e.Logger.Info("site regenerated",
		"reason", reason,
		"path", filepath.Join(staticDir, site.IndexFile))
	return nil
}

// RebuildSite re-renders the public page from persisted state without running a
// update: per-rule data comes from snapshots and on-disk artifacts, geosite
// stats from the on-disk provider caches. Rules with data on disk report
// "ok"; rules never updated report "stale".
func (e *UpdateEngine) RebuildSite() error {
	staticDir := filepath.Join(e.DataDir, site.StaticDir)

	updatedAt := time.Now()
	if t, ok := e.State.LastCheck(); ok {
		updatedAt = t
	}

	idx := e.publicIndexData(updatedAt, staticDir, "/admin", nil, nil)
	if err := removeLegacyAdmin(staticDir); err != nil {
		return err
	}
	return site.WritePublic(e.DataDir, idx)
}

// publicIndexData assembles the shared public-page model. infos contains
// results from the current update when available; persisted state fills the
// remaining rules. adminURL selects the service or standalone-static view.
func (e *UpdateEngine) publicIndexData(
	updatedAt time.Time,
	staticDir string,
	adminURL string,
	infos map[string]*ruleSiteInfo,
	gstats *geositeStats,
) *site.IndexData {
	idx := &site.IndexData{
		UpdatedAt: updatedAt,
		AdminURL:  adminURL,
		Clients:   e.siteClients(),
		IconSets:  site.ListIconSets(staticDir),
	}
	tagSet := make(map[string]bool)
	for _, rule := range e.currentConfig().Rules {
		info := infos[rule.ID]
		if info == nil {
			info = e.persistedRuleSiteInfo(rule)
		}
		idx.Rules = append(idx.Rules, site.PublicRule{
			ID:          info.id,
			Name:        info.name,
			Description: info.description,
			Tags:        rule.Tags,
			Entries:     info.entries,
			Files:       info.files,
		})
		for _, tag := range rule.Tags {
			if !tagSet[tag] {
				tagSet[tag] = true
				idx.Tags = append(idx.Tags, tag)
			}
		}
	}
	sort.Strings(idx.Tags)
	if gstats == nil {
		gstats = e.rebuildGeositeStats()
	}
	idx.Geosite = gstats.catalog()
	return idx
}

// GeositeProviderSummaries reconstructs current provider summaries for the API.
func (e *UpdateEngine) GeositeProviderSummaries() []GeositeProviderSummary {
	return e.rebuildGeositeStats().summaries()
}

// rebuildGeositeStats reconstructs the published geosite catalog from the
// on-disk provider caches, mirroring what an update would publish, and counts
// the artifacts present on disk per provider.
func (e *UpdateEngine) rebuildGeositeStats() *geositeStats {
	gstats := newGeositeStats()
	cfg := e.currentConfig()
	if cfg.Geosite == nil || e.Geosite == nil {
		return gstats
	}
	for _, prov := range cfg.Geosite.Providers {
		cache, err := e.Geosite.Read(prov.Name)
		result, checkedAt, recorded := e.State.GeositeUpdate(prov.Name)
		if err != nil || cache == nil {
			if !recorded {
				result = state.GeositeUnchanged
			}
			gstats.setMeta(prov.Name, "", result, checkedAt)
			continue
		}
		if !recorded {
			result = state.GeositeUpdated
			checkedAt, _ = time.Parse(time.RFC3339, cache.FetchedAt)
		}
		gstats.setMeta(prov.Name, cache.ResolvedVersion, result, checkedAt)
		for _, summary := range geosite.CatalogSummaries(cache) {
			fullRef := geosite.GeositeRef{Provider: prov.Name, List: summary.Name}
			if irEntries, err := resolveGeositeIR(cache, fullRef); err == nil && len(irEntries) > 0 {
				gstats.recordVariant(prov.Name, summary.Name, nil, len(irEntries))
			}
			for _, attr := range summary.Attrs {
				attrRef := geosite.GeositeRef{Provider: prov.Name, List: summary.Name, Attrs: []string{attr}}
				if irEntries, err := resolveGeositeIR(cache, attrRef); err == nil && len(irEntries) > 0 {
					gstats.recordVariant(prov.Name, summary.Name, []string{attr}, len(irEntries))
				}
			}
		}
		targets := config.ExpandSelectedTargets(cfg.Clients, prov.Clients)
		for range countGeositeArtifacts(e.DataDir, prov.Name, targets) {
			gstats.recordFile(prov.Name)
		}
	}
	return gstats
}

// countGeositeArtifacts counts published geosite artifact files on disk:
// dataDir/rules/<client>/geosite/<provider>/*.
func countGeositeArtifacts(dataDir, provider string, targets []config.OutputTarget) int {
	n := 0
	for _, target := range targets {
		entries, err := os.ReadDir(filepath.Join(dataDir, "rules", target.ID, "geosite", provider))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				n++
			}
		}
	}
	return n
}

// writeSite assembles and writes the public index.
func (e *UpdateEngine) writeSite(result *UpdateResult, infos map[string]*ruleSiteInfo, gstats *geositeStats) error {
	now := time.Now()

	staticDir := filepath.Join(e.DataDir, site.StaticDir)
	if _, err := site.UpdateBuiltinAssets(staticDir); err != nil {
		e.Logger.Warn("write builtin icons failed", "error", err)
		return fmt.Errorf("update builtin assets: %w", err)
	}
	idx := e.publicIndexData(now, staticDir, "/admin", infos, gstats)
	if err := removeLegacyAdmin(staticDir); err != nil {
		return err
	}
	if err := site.WritePublic(e.DataDir, idx); err != nil {
		e.Logger.Warn("site generation failed", "error", err)
		return err
	}
	e.Logger.Info("site generated", "path", filepath.Join(e.DataDir, site.StaticDir, site.IndexFile))
	return nil
}

func removeLegacyAdmin(staticDir string) error {
	path := filepath.Join(staticDir, "admin.html")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy managed admin page: %w", err)
	}
	return nil
}

// appendSamples appends display lines up to the per-rule sample limit.
func appendSamples(dst, src []string) []string {
	for _, s := range src {
		if len(dst) >= diffSampleLimit {
			break
		}
		dst = append(dst, s)
	}
	return dst
}
