package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func sampleIndex() *IndexData {
	return &IndexData{
		UpdatedAt: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC),
		Clients: []Client{
			{ID: "Clash Meta", Name: "Clash Meta", Icon: "mihomo", Rules: true, Geosite: true, Options: []ClientOption{{ID: "Clash Meta", Name: "Standard", Ext: ".list", Rules: true, Geosite: true}}},
			{ID: "sing-box", Name: "sing-box", Icon: "singbox", Rules: true, Options: []ClientOption{
				{ID: "sing-box", Name: "Standard", Ext: ".json", Rules: true},
				{ID: "sing-box-non-ip", Name: "Non-IP", Ext: ".json", Rules: true},
			}},
		},
		Rules: []PublicRule{
			{
				ID:         "OpenAi",
				Name:       "OpenAi",
				Tags:       []string{"AI"},
				TagsJoined: "ai",
				Entries:    1234,
				Files: []RuleFile{
					{Client: "Clash Meta", Icon: "mihomo", Path: "rules/Clash Meta/OpenAi.list", Size: 2048},
					{Client: "sing-box", Icon: "singbox", Path: "rules/sing-box/OpenAi.json", Size: 4096},
				},
			},
		},
		Tags: []string{"AI"},
		Geosite: []GeositeCatalog{
			{
				Provider: "v2fly",
				Lists: []GeositeList{
					{Name: "google", Entries: 1000, Variants: []GeositeVariant{{Attr: "cn", Entries: 120}}},
				},
			},
		},
	}
}

func TestWritePublicRendersIndexOnly(t *testing.T) {
	dir := t.TempDir()
	staticDir := filepath.Join(dir, StaticDir)
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateBuiltinAssets(staticDir); err != nil {
		t.Fatalf("UpdateBuiltinAssets: %v", err)
	}
	idx := sampleIndex()
	idx.IconSets = ListIconSets(staticDir)
	if err := WritePublic(dir, idx); err != nil {
		t.Fatalf("WritePublic: %v", err)
	}

	index, err := os.ReadFile(filepath.Join(staticDir, IndexFile))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	for _, want := range []string{
		"规则", "OpenAi", "client-card", "rule-row", "modal-bk",
		"preview-shell", "preview-line", "FILE PREVIEW",
		"GEO_CATALOG", "RULE_META", "ICON_SETS", "v2fly", "复制链接", "打开文件", "更新于",
		`class="table-scroll"`, `class="data-table rule-table"`, `scope="col"`, `scope="row"`,
		"rules/Clash%20Meta/OpenAi.list",
		`data-rules="true" data-geo="true"`,  // Clash Meta: rules + geosite
		`data-rules="true" data-geo="false"`, // sing-box: rules only
		"gprev",                              // geosite preview button
		`class="client-menu"`,                // full-card format/variant menu
		`data-target="sing-box-non-ip"`,      // explicit IR-derived output
		"图标",                                 // icon tab
	} {
		if !strings.Contains(string(index), want) {
			t.Errorf("index missing %q", want)
		}
	}
	// Update internals must not leak into the public page.
	for _, bad := range []string{"耗时", "管理看板", "变更", "失败", "立即更新", "/admin", "/api/v1", `<select class="client-format"`} {
		if strings.Contains(string(index), bad) {
			t.Errorf("index leaks admin content %q", bad)
		}
	}
	assertThemeInitializedBeforeStyles(t, index)
	assertInlineScriptsParse(t, index)

	if _, err := os.Stat(filepath.Join(staticDir, "admin.html")); !os.IsNotExist(err) {
		t.Fatalf("public writer created admin.html: %v", err)
	}
}

func TestWritePublicRendersOptionalAdminLink(t *testing.T) {
	dir := t.TempDir()
	staticDir := filepath.Join(dir, StaticDir)
	if _, err := UpdateBuiltinAssets(staticDir); err != nil {
		t.Fatal(err)
	}
	idx := sampleIndex()
	idx.AdminURL = "/admin"
	if err := WritePublic(dir, idx); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(staticDir, IndexFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `href="/admin"`) || !strings.Contains(string(page), "[ 管理 ]") {
		t.Fatal("service public page is missing the admin link")
	}
}

func assertInlineScriptsParse(t *testing.T, page []byte) {
	t.Helper()
	rest := string(page)
	count := 0
	for {
		start := strings.Index(rest, "<script>")
		if start < 0 {
			break
		}
		rest = rest[start+len("<script>"):]
		end := strings.Index(rest, "</script>")
		if end < 0 {
			t.Fatal("inline script has no closing tag")
		}
		if _, err := goja.Compile("index.html", rest[:end], false); err != nil {
			t.Fatalf("inline script syntax: %v", err)
		}
		count++
		rest = rest[end+len("</script>"):]
	}
	if count == 0 {
		t.Fatal("public page has no inline scripts")
	}
}

func TestAdminPageIsFixedAPIApplication(t *testing.T) {
	admin, err := AdminPage()
	if err != nil {
		t.Fatal(err)
	}
	html := string(admin)
	for _, want := range []string{"管理看板", "更新日志", "var API = '/api/v1'", "request('/status')", "scope:'rules'", "textContent", "replaceChildren", "EventSource", "sessionStorage", "update-list-head", "规则更新明细"} {
		if !strings.Contains(html, want) {
			t.Errorf("admin missing %q", want)
		}
	}
	for _, status := range []string{"completed_with_warnings:'警告'", "completed_with_errors:'异常'"} {
		if !strings.Contains(html, status) {
			t.Errorf("admin missing status copy %q", status)
		}
	}
	for _, removed := range []string{"{{", "location.reload", "location.href = '/admin", "/api/update", "ErrorHistory", "index === 0 && expandedUpdates.size === 0", "error-items", "update-facts"} {
		if strings.Contains(html, removed) {
			t.Errorf("admin contains removed pattern %q", removed)
		}
	}
	if strings.Contains(html, "updateFinal") || strings.Contains(html, "update-final") {
		t.Error("admin contains redundant update footer status")
	}
	for _, countID := range []string{"rulesCount", "changesCount", "updatesCount", "geositeCount"} {
		if !strings.Contains(html, `id="`+countID+`" hidden`) {
			t.Errorf("admin count %s is visible before its data loads", countID)
		}
	}
	if !strings.Contains(html, "badge.hidden = false") {
		t.Error("admin does not reveal counts after loading")
	}
	assertThemeInitializedBeforeStyles(t, admin)
}

func assertThemeInitializedBeforeStyles(t *testing.T, page []byte) {
	t.Helper()
	html := string(page)
	themeInit := strings.Index(html, "localStorage.getItem('prm-theme')")
	styles := strings.Index(html, "<style>")
	if themeInit < 0 || styles < 0 || themeInit > styles {
		t.Error("saved theme must be restored before styles are evaluated")
	}
}

func TestEscapePath(t *testing.T) {
	if got := escapePath("rules/Clash Meta/OpenAi.list"); got != "rules/Clash%20Meta/OpenAi.list" {
		t.Errorf("escapePath = %q", got)
	}
}

func TestCommas(t *testing.T) {
	cases := map[int]string{0: "0", 12: "12", 999: "999", 1000: "1,000", 1234567: "1,234,567"}
	for in, want := range cases {
		if got := commas(in); got != want {
			t.Errorf("commas(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCatalogJSON(t *testing.T) {
	got := string(catalogJSON(sampleIndex().Geosite))
	if !strings.Contains(got, `"v2fly"`) || !strings.Contains(got, `"google"`) || !strings.Contains(got, `"cn"`) {
		t.Errorf("catalog JSON unexpected: %s", got)
	}
}

func TestRuleMetaJSON(t *testing.T) {
	got := string(ruleMetaJSON(sampleIndex().Rules))
	for _, want := range []string{`"i":"OpenAi"`, `"n":"OpenAi"`, `rules/Clash%20Meta/OpenAi.list`, `"e":1234`} {
		if !strings.Contains(got, want) {
			t.Errorf("rulemeta JSON missing %q: %s", want, got)
		}
	}
}

func TestPixelIconFallback(t *testing.T) {
	if got := pixelIcon(t.TempDir(), "nonexistent", 16); !strings.Contains(string(got), "singbox.svg") {
		t.Error("fallback icon should reference singbox.svg")
	}
}

func TestPixelIconUserFile(t *testing.T) {
	staticDir := t.TempDir()
	iconsDir := filepath.Join(staticDir, "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(iconsDir, "brand.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := string(pixelIcon(staticDir, "brand", 28))
	if !strings.Contains(got, "static/icons/brand.svg") {
		t.Errorf("user icon should reference brand.svg, got %q", got)
	}
	// Names that fail validation never reach the filesystem and fall back.
	got = string(pixelIcon(staticDir, "../secret", 16))
	if !strings.Contains(got, "singbox.svg") {
		t.Errorf("traversal name should fall back to singbox, got %q", got)
	}
}

func TestUpdateBuiltinAssets(t *testing.T) {
	dir := t.TempDir()
	staticDir := filepath.Join(dir, StaticDir)
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := UpdateBuiltinAssets(staticDir)
	if err != nil {
		t.Fatalf("UpdateBuiltinAssets: %v", err)
	}
	if !res.Changed || !res.FingerprintChanged {
		t.Fatalf("first run should change assets and fingerprint: %+v", res)
	}
	iconsDir := filepath.Join(staticDir, "icons")
	for _, name := range []string{"mihomo.svg", "singbox.svg", "shadowrocket.svg", "surge.svg", "prm.svg"} {
		if _, err := os.Stat(filepath.Join(iconsDir, name)); err != nil {
			t.Errorf("icon %s not written: %v", name, err)
		}
	}
	qureDir := filepath.Join(iconsDir, "QureColor")
	if entries, err := os.ReadDir(qureDir); err == nil {
		if len(entries) < 300 {
			t.Errorf("QureColor has only %d icons, expected 300+", len(entries))
		}
	}

	// Second run: nothing to do.
	res, err = UpdateBuiltinAssets(staticDir)
	if err != nil {
		t.Fatalf("UpdateBuiltinAssets (2nd): %v", err)
	}
	if res.Changed || res.FingerprintChanged || len(res.Written) > 0 || len(res.Deleted) > 0 {
		t.Errorf("second run should be a no-op: %+v", res)
	}
}

func TestUpdateBuiltinAssetsPreservesUserModifications(t *testing.T) {
	staticDir := t.TempDir()
	if _, err := UpdateBuiltinAssets(staticDir); err != nil {
		t.Fatalf("UpdateBuiltinAssets: %v", err)
	}
	target := filepath.Join(staticDir, "icons", "singbox.svg")
	custom := []byte("<svg>user version</svg>")
	if err := os.WriteFile(target, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := UpdateBuiltinAssets(staticDir)
	if err != nil {
		t.Fatalf("UpdateBuiltinAssets (2nd): %v", err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != string(custom) {
		t.Error("user-modified builtin icon was overwritten")
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "singbox.svg" {
		t.Errorf("expected singbox.svg in Skipped: %+v", res)
	}

	// A user file outside the builtin set is never touched.
	userFile := filepath.Join(staticDir, "icons", "brand.svg")
	if err := os.WriteFile(userFile, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateBuiltinAssets(staticDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Error("user icon file missing after update")
	}
}

func TestUpdateBuiltinAssetsRemovesOrphans(t *testing.T) {
	staticDir := t.TempDir()
	if _, err := UpdateBuiltinAssets(staticDir); err != nil {
		t.Fatalf("UpdateBuiltinAssets: %v", err)
	}

	// Simulate an asset dropped from the binary: manifest still records it.
	manifestPath := assetManifestPath(staticDir)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m assetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(staticDir, "icons", "old.png")
	orphanContent := []byte("old")
	if err := os.WriteFile(orphan, orphanContent, 0o644); err != nil {
		t.Fatal(err)
	}
	m.Files["old.png"] = hashBytes(orphanContent)
	data, _ = json.Marshal(m)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := UpdateBuiltinAssets(staticDir)
	if err != nil {
		t.Fatalf("UpdateBuiltinAssets (2nd): %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("untouched orphan should be removed")
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != "old.png" {
		t.Errorf("expected old.png in Deleted: %+v", res)
	}

	// A user-modified orphan survives.
	if err := os.WriteFile(orphan, []byte("user edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Files["old.png"] = hashBytes([]byte("old"))
	data, _ = json.Marshal(m)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateBuiltinAssets(staticDir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(orphan); string(data) != "user edit" {
		t.Error("user-modified orphan should be preserved")
	}
}

func TestResolveClientIcon(t *testing.T) {
	if got := ResolveClientIcon("custom", "whatever"); got != "custom" {
		t.Errorf("explicit icon: got %q", got)
	}
	if got := ResolveClientIcon("", "Clash Meta"); got != "mihomo" {
		t.Errorf("default for Clash Meta: got %q", got)
	}
	if got := ResolveClientIcon("", "sing-box"); got != "singbox" {
		t.Errorf("default for sing-box: got %q", got)
	}
	if got := ResolveClientIcon("", "Shadowrocket"); got != "shadowrocket" {
		t.Errorf("default for Shadowrocket: got %q", got)
	}
	if got := ResolveClientIcon("", "Surge"); got != "surge" {
		t.Errorf("default for Surge: got %q", got)
	}
}
