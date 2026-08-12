// Package site renders the public static page regenerated on every update:
//
//   - {data_dir}/index.html — public rule index: every rule with per-client
//     download links and content preview, plus the full geosite catalog.
//
// The page is self-contained and works from file:// or any static host.
package site

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

//go:embed site_index.html site_admin.html site_admin_app.html
var htmlFS embed.FS

// StaticDir is the subdirectory under dataDir for generated pages and assets.
const StaticDir = "static"

// IndexFile is the generated public page.
const IndexFile = "index.html"

// IndexFileTemplate names the embedded public page template.
const IndexFileTemplate = "site_index.html"

// Client describes one output client/format for link building.
type Client struct {
	ID      string
	Name    string
	Icon    string
	Ext     string // template extension, e.g. ".list"
	Rules   bool   // at least one rule outputs this client
	Geosite bool   // at least one geosite provider publishes to this client
}

// RuleFile is one client-specific artifact of a rule.
type RuleFile struct {
	Client string // client ID
	Icon   string // pixel icon name
	Path   string // relative URL path, e.g. "rules/sing-box/OpenAi.json"
	Size   int64
}

// PublicRule is one rule row on the public index.
type PublicRule struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	TagsJoined  string
	Entries     int
	Files       []RuleFile
}

// GeositeVariant is one @attr variant of a geosite list.
type GeositeVariant struct {
	Attr    string
	Entries int
}

// GeositeList is one published geosite list with its variants.
type GeositeList struct {
	Name     string
	Entries  int
	Variants []GeositeVariant
}

// GeositeCatalog is the full published catalog of one provider.
type GeositeCatalog struct {
	Provider string
	Lists    []GeositeList
}

// IconSet describes one icon collection directory for the gallery.
type IconSet struct {
	Name  string
	Count int
	Icons []string
}

// IndexData is the view model for the public page.
type IndexData struct {
	UpdatedAt time.Time
	Clients   []Client
	Rules     []PublicRule
	Tags      []string
	Geosite   []GeositeCatalog
	IconSets  []IconSet
}

var funcMap = template.FuncMap{
	"commas":   commas,
	"human":    humanBytes,
	"tfmt":     formatTime,
	"lower":    strings.ToLower,
	"escpath":  escapePath,
	"catalog":  catalogJSON,
	"clients":  clientsJSON,
	"rulemeta": ruleMetaJSON,
	"iconsets": iconSetsJSON,
}

// WritePublic renders the public page into dataDir/static/.
// Icons must already be written (via UpdateBuiltinAssets) and IconSets
// populated before calling this.
func WritePublic(dataDir string, index *IndexData) error {
	staticDir := filepath.Join(dataDir, StaticDir)
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		return fmt.Errorf("create static dir: %w", err)
	}
	return render(staticDir, IndexFile, IndexFileTemplate, index)
}

// AdminPage returns the backend-served management application shell. The
// established visual stylesheet is kept with the old template while the body
// is a fixed API client with no generated business data.
func AdminPage() ([]byte, error) {
	styled, err := htmlFS.ReadFile("site_admin.html")
	if err != nil {
		return nil, err
	}
	app, err := htmlFS.ReadFile("site_admin_app.html")
	if err != nil {
		return nil, err
	}
	headEnd := bytes.Index(styled, []byte("</head>"))
	if headEnd < 0 {
		return nil, fmt.Errorf("admin stylesheet document has no head")
	}
	page := append([]byte(nil), styled[:headEnd+len("</head>")]...)
	page = append(page, app...)
	return page, nil
}

func render(dataDir, outName, tmplName string, data any) error {
	// The icon helper resolves user-provided icon files relative to the
	// generated site directory, so it is bound per render call.
	funcs := maps.Clone(funcMap)
	funcs["icon"] = func(name string, px int) template.HTML {
		return pixelIcon(dataDir, name, px)
	}
	t, err := template.New(tmplName).Funcs(funcs).ParseFS(htmlFS, tmplName)
	if err != nil {
		return fmt.Errorf("parse %s template: %w", tmplName, err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return fmt.Errorf("render %s: %w", tmplName, err)
	}
	return util.AtomicWriteFile(filepath.Join(dataDir, outName), buf.Bytes())
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

// catalogJSON serialises the geosite catalog as a compact JS array:
// [["provider",[["list",entries,[["attr",entries],...]],...]],...]
// List and attr names are validated safe segments (lowercase alnum/dash),
// so the output is safe to embed in a script context.
func catalogJSON(cats []GeositeCatalog) template.JS {
	compact := make([]any, 0, len(cats))
	for _, c := range cats {
		lists := make([]any, 0, len(c.Lists))
		for _, l := range c.Lists {
			variants := make([]any, 0, len(l.Variants))
			for _, v := range l.Variants {
				variants = append(variants, []any{v.Attr, v.Entries})
			}
			lists = append(lists, []any{l.Name, l.Entries, variants})
		}
		compact = append(compact, []any{c.Provider, lists})
	}
	data, err := json.Marshal(compact)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(data) //nolint:gosec // names are validated safe segments
}

// clientsJSON serialises the client list for the page scripts:
// [{"id":"...","name":"...","icon":"cat","ext":".list"},...]
func clientsJSON(clients []Client) template.JS {
	type clientJSON struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Icon    string `json:"icon"`
		Ext     string `json:"ext"`
		Rules   bool   `json:"rules"`
		Geosite bool   `json:"geosite"`
	}
	out := make([]clientJSON, 0, len(clients))
	for _, c := range clients {
		out = append(out, clientJSON(c))
	}
	data, err := json.Marshal(out)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(data) //nolint:gosec // json.Marshal escapes HTML-sensitive chars
}

// ruleMetaJSON serialises per-rule metadata for the detail modal:
// [{"i":"openai","n":"OpenAI","d":"...","t":["AI"],"e":1234,"f":{"Clash Meta":["rules/...",123],...}},...]
// File paths are escaped per segment before embedding.
func ruleMetaJSON(rules []PublicRule) template.JS {
	type ruleMeta struct {
		ID          string            `json:"i"`
		Name        string            `json:"n"`
		Description string            `json:"d,omitempty"`
		Tags        []string          `json:"t,omitempty"`
		Entries     int               `json:"e"`
		Files       map[string][2]any `json:"f"`
	}
	out := make([]ruleMeta, 0, len(rules))
	for _, r := range rules {
		m := ruleMeta{ID: r.ID, Name: r.Name, Description: r.Description, Tags: r.Tags, Entries: r.Entries}
		m.Files = make(map[string][2]any, len(r.Files))
		for _, f := range r.Files {
			m.Files[f.Client] = [2]any{escapePath(f.Path), f.Size}
		}
		out = append(out, m)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(data) //nolint:gosec // json.Marshal escapes HTML-sensitive chars
}

// iconSetsJSON serialises the icon set list for the page scripts.
func iconSetsJSON(sets []IconSet) template.JS {
	type iconSetJSON struct {
		Name  string   `json:"name"`
		Count int      `json:"count"`
		Icons []string `json:"icons"`
	}
	out := make([]iconSetJSON, 0, len(sets))
	for _, s := range sets {
		out = append(out, iconSetJSON(s))
	}
	data, err := json.Marshal(out)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(data) //nolint:gosec
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

func commas(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	v := float64(b) / float64(div)
	if v >= 100 {
		return fmt.Sprintf("%.0f %cB", v, "KMGTPE"[exp])
	}
	return fmt.Sprintf("%.1f %cB", v, "KMGTPE"[exp])
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05") + " UTC"
}
