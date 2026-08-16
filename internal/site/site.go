// Package site renders the public static page regenerated on every update:
//
//   - {dataDir}/index.html — public rule index: every rule with per-client
//     download links and content preview, plus the full geosite catalog.
//
// The page is a client-rendered Svelte application that works from any static host.
package site

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

//go:embed site_index.html dist/public.js dist/public.css
var htmlFS embed.FS

// StaticDir is the subdirectory under dataDir for generated pages and assets.
const StaticDir = "static"

// IndexFile is the generated public page.
const IndexFile = "index.html"

// IndexFileTemplate names the embedded public page template.
const IndexFileTemplate = "site_index.html"

// Client describes one configured client family shown on the public page.
type Client struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Icon    string         `json:"icon"`
	Options []ClientOption `json:"options"`
	Rules   bool           `json:"rules"`   // at least one option has rule output
	Geosite bool           `json:"geosite"` // at least one option has geosite output
}

// ClientOption is one concrete official format or IR-derived variant.
type ClientOption struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Ext     string `json:"ext"`
	Rules   bool   `json:"rules"`
	Geosite bool   `json:"geosite"`
}

// RuleFile is one client-specific artifact of a rule.
type RuleFile struct {
	Client string `json:"target_id"` // client ID
	Icon   string `json:"-"`         // pixel icon name
	Path   string `json:"path"`      // relative URL path, e.g. "rules/sing-box/OpenAi.json"
	Size   int64  `json:"size"`
}

// PublicRule is one rule row on the public index.
type PublicRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Tags        []string   `json:"tags"`
	Entries     int        `json:"entries"`
	Files       []RuleFile `json:"files"`
}

// GeositeVariant is one @attr variant of a geosite list.
type GeositeVariant struct {
	Attr    string `json:"attr"`
	Entries int    `json:"entries"`
}

// GeositeList is one published geosite list with its variants.
type GeositeList struct {
	Name     string           `json:"name"`
	Entries  int              `json:"entries"`
	Variants []GeositeVariant `json:"variants"`
}

// GeositeCatalog is the full published catalog of one provider.
type GeositeCatalog struct {
	Provider string        `json:"provider"`
	Lists    []GeositeList `json:"lists"`
}

// IconSet describes one icon collection directory for the gallery.
type IconSet struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Icons []string `json:"icons"`
}

// IndexData is the view model for the public page.
type IndexData struct {
	UpdatedAt time.Time        `json:"updated_at"`
	AdminURL  string           `json:"admin_url,omitempty"`
	Clients   []Client         `json:"clients"`
	Rules     []PublicRule     `json:"rules"`
	Tags      []string         `json:"tags"`
	Geosite   []GeositeCatalog `json:"geosite"`
	IconSets  []IconSet        `json:"icon_sets"`
}

var funcMap = template.FuncMap{
	"publicdata": publicDataJSON,
	"assetver":   publicAssetVersion,
}

// WritePublic renders the public page into dataDir/static/.
// Icons must already be written (via UpdateBuiltinAssets) and IconSets
// populated before calling this.
func WritePublic(dataDir string, index *IndexData) error {
	staticDir := filepath.Join(dataDir, StaticDir)
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		return fmt.Errorf("create static dir: %w", err)
	}
	return WriteIndex(staticDir, staticDir, index)
}

// WriteIndex renders the public index into outputDir. iconStaticDir points to
// the static directory that contains icons/, which may differ from outputDir
// for a standalone export whose layout is index.html + static/icons/.
func WriteIndex(outputDir, iconStaticDir string, index *IndexData) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create index output dir: %w", err)
	}
	if err := writePublicAssets(iconStaticDir); err != nil {
		return err
	}
	return render(outputDir, iconStaticDir, IndexFile, IndexFileTemplate, index)
}

func render(outputDir, _ string, outName, tmplName string, data any) error {
	t, err := template.New(tmplName).Funcs(funcMap).ParseFS(htmlFS, tmplName)
	if err != nil {
		return fmt.Errorf("parse %s template: %w", tmplName, err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return fmt.Errorf("render %s: %w", tmplName, err)
	}
	return util.AtomicWriteFile(filepath.Join(outputDir, outName), buf.Bytes())
}

func writePublicAssets(staticDir string) error {
	assetsDir := filepath.Join(staticDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return fmt.Errorf("create public assets dir: %w", err)
	}
	for _, name := range []string{"public.js", "public.css"} {
		data, err := htmlFS.ReadFile("dist/" + name)
		if err != nil {
			return fmt.Errorf("read embedded public asset %s: %w", name, err)
		}
		if err := util.AtomicWriteFile(filepath.Join(assetsDir, name), data); err != nil {
			return fmt.Errorf("write public asset %s: %w", name, err)
		}
	}
	return nil
}

func publicAssetVersion() string {
	fingerprint := AssetFingerprint()
	if len(fingerprint) > 12 {
		return fingerprint[:12]
	}
	return fingerprint
}

func publicDataJSON(index *IndexData) (template.JS, error) {
	payload := *index
	payload.Clients = make([]Client, len(index.Clients))
	for i, client := range index.Clients {
		payload.Clients[i] = client
		payload.Clients[i].Options = append([]ClientOption{}, client.Options...)
	}
	payload.Tags = append([]string{}, index.Tags...)
	payload.Rules = make([]PublicRule, len(index.Rules))
	for i, rule := range index.Rules {
		payload.Rules[i] = rule
		payload.Rules[i].Tags = append([]string{}, rule.Tags...)
		payload.Rules[i].Files = make([]RuleFile, len(rule.Files))
		for j, file := range rule.Files {
			payload.Rules[i].Files[j] = file
			payload.Rules[i].Files[j].Path = escapePath(file.Path)
		}
	}
	payload.Geosite = make([]GeositeCatalog, len(index.Geosite))
	for i, catalog := range index.Geosite {
		payload.Geosite[i] = catalog
		payload.Geosite[i].Lists = make([]GeositeList, len(catalog.Lists))
		for j, list := range catalog.Lists {
			payload.Geosite[i].Lists[j] = list
			payload.Geosite[i].Lists[j].Variants = append([]GeositeVariant{}, list.Variants...)
		}
	}
	payload.IconSets = make([]IconSet, len(index.IconSets))
	for i, set := range index.IconSets {
		payload.IconSets[i] = set
		payload.IconSets[i].Icons = append([]string{}, set.Icons...)
	}
	data, err := json.Marshal(&payload)
	if err != nil {
		return "", fmt.Errorf("marshal public page data: %w", err)
	}
	return template.JS(data), nil //nolint:gosec // json.Marshal escapes HTML-sensitive characters
}

// escapePath URL-escapes each path segment (client IDs may contain spaces).
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = urlPathEscape(part)
	}
	return strings.Join(parts, "/")
}

func urlPathEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == '~' || c == '@') {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// ListIconSets scans staticDir/icons/ for subdirectories and returns one
// IconSet per subdirectory, each listing the image files it contains.
func ListIconSets(staticDir string) []IconSet {
	iconsDir := filepath.Join(staticDir, "icons")
	entries, err := os.ReadDir(iconsDir)
	if err != nil {
		return nil
	}
	var sets []IconSet
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(iconsDir, e.Name()))
		if err != nil {
			continue
		}
		var icons []string
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".svg") || strings.HasSuffix(lower, ".webp") {
				icons = append(icons, name)
			}
		}
		if len(icons) > 0 {
			sort.Strings(icons)
			sets = append(sets, IconSet{Name: e.Name(), Count: len(icons), Icons: icons})
		}
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Name < sets[j].Name })
	return sets
}
