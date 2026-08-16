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
				ID:      "OpenAi",
				Name:    "OpenAi",
				Tags:    []string{"AI"},
				Entries: 1234,
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
		`id="public-app"`, `id="prm-data"`, `type="application/json"`,
		"static/assets/public.css?v=", "static/assets/public.js?v=", "OpenAi", "v2fly",
		"rules/Clash%20Meta/OpenAi.list",
		`"id":"sing-box-non-ip"`,
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
	assertPublicData(t, index, "")
	for _, name := range []string{"public.js", "public.css"} {
		if info, err := os.Stat(filepath.Join(staticDir, "assets", name)); err != nil || info.IsDir() {
			t.Errorf("public asset %s: info=%v err=%v", name, info, err)
		}
	}

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
	assertPublicData(t, page, "/admin")
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

func assertThemeInitializedBeforeStyles(t *testing.T, page []byte) {
	t.Helper()
	html := string(page)
	themeInit := strings.Index(html, "localStorage.getItem('prm-theme')")
	styles := strings.Index(html, "public.css")
	if themeInit < 0 || styles < 0 || themeInit > styles {
		t.Error("saved theme must be restored before styles are evaluated")
	}
}

func TestEscapePath(t *testing.T) {
	if got := escapePath("rules/Clash Meta/OpenAi.list"); got != "rules/Clash%20Meta/OpenAi.list" {
		t.Errorf("escapePath = %q", got)
	}
}

func TestPublicDataJSONEscapesHTML(t *testing.T) {
	idx := sampleIndex()
	idx.Rules[0].Description = `</script><script>alert("x")</script>`
	got, err := publicDataJSON(idx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "</script>") || !strings.Contains(string(got), `\u003c/script\u003e`) {
		t.Fatalf("public data HTML escaping = %s", got)
	}
}

func TestPublicDataJSONUsesArraysForEmptyCollections(t *testing.T) {
	idx := &IndexData{UpdatedAt: time.Now(), Clients: []Client{{ID: "empty"}}, Rules: []PublicRule{{ID: "empty"}}, Geosite: []GeositeCatalog{{Provider: "empty", Lists: []GeositeList{{Name: "empty"}}}}, IconSets: []IconSet{{Name: "empty"}}}
	got, err := publicDataJSON(idx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"options":[]`, `"tags":[]`, `"files":[]`, `"variants":[]`, `"icons":[]`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("public data missing stable array %s: %s", want, got)
		}
	}
}

func assertPublicData(t *testing.T, page []byte, adminURL string) {
	t.Helper()
	html := string(page)
	startMarker := `<script id="prm-data" type="application/json">`
	start := strings.Index(html, startMarker)
	if start < 0 {
		t.Fatal("public data script missing")
	}
	start += len(startMarker)
	end := strings.Index(html[start:], "</script>")
	if end < 0 {
		t.Fatal("public data script has no closing tag")
	}
	var payload IndexData
	if err := json.Unmarshal([]byte(html[start:start+end]), &payload); err != nil {
		t.Fatalf("decode public data: %v", err)
	}
	if payload.AdminURL != adminURL || len(payload.Rules) != 1 || payload.Rules[0].ID != "OpenAi" {
		t.Fatalf("public data mismatch: admin=%q rules=%+v", payload.AdminURL, payload.Rules)
	}
	if got := payload.Rules[0].Files[0].Path; got != "rules/Clash%20Meta/OpenAi.list" {
		t.Fatalf("escaped rule path = %q", got)
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
