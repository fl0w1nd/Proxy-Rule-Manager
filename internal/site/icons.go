package site

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:assets/icons
var iconAssets embed.FS

// fileIconSet is the set of names that resolve to static files.
var fileIconSet = map[string]string{
	"prm":          "prm.svg",
	"mihomo":       "mihomo.svg",
	"singbox":      "singbox.svg",
	"shadowrocket": "shadowrocket.svg",
}

// DefaultIconForClient returns a sensible icon name when the config omits
// the icon field, based on the client id or template name.
func DefaultIconForClient(id string) string {
	s := strings.ToLower(id)
	switch {
	case strings.Contains(s, "shadowrocket"):
		return "shadowrocket"
	case strings.Contains(s, "sing-box"), strings.Contains(s, "singbox"):
		return "singbox"
	case strings.Contains(s, "clash"), strings.Contains(s, "mihomo"), strings.Contains(s, "meta"):
		return "mihomo"
	default:
		return "singbox"
	}
}

// ResolveClientIcon returns the configured icon or a default.
func ResolveClientIcon(configIcon, clientID string) string {
	if configIcon != "" {
		return configIcon
	}
	return DefaultIconForClient(clientID)
}

// validIconName reports whether name is safe to resolve under icons/ and to
// embed in a URL path: a single path segment of common icon-name characters.
func validIconName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == '@'
		if !ok {
			return false
		}
	}
	return true
}

// Utility icons stay as character grids so they can inherit semantic colors.
var pixelGrids = map[string][]string{
	"globe": {
		"....######....",
		"..##########..",
		".###..##..###.",
		".##...##...##.",
		"##....##....##",
		"##....##....##",
		"##############",
		"##....##....##",
		"##....##....##",
		".##...##...##.",
		".###..##..###.",
		"..##########..",
		"....######....",
	},
	"check": {
		"...........##.",
		"..........###.",
		".........###..",
		"##......###...",
		"###....###....",
		".###..###.....",
		"..######......",
		"...####.......",
		"....##........",
	},
	"cross": {
		"##........##",
		"###......###",
		".###....###.",
		"..###..###..",
		"...######...",
		"....####....",
		"....####....",
		"...######...",
		"..###..###..",
		".###....###.",
		"###......###",
		"##........##",
	},
	"up": {
		"....##....",
		"...####...",
		"..######..",
		".########.",
		"....##....",
		"....##....",
		"....##....",
	},
	"down": {
		"....##....",
		"....##....",
		"....##....",
		".########.",
		"..######..",
		"...####...",
		"....##....",
	},
}

func pixelCharColor(ch byte) string {
	switch ch {
	case '#':
		return "currentColor"
	case 'o':
		return "#FF6B1A"
	case 'b':
		return "#4A8DFF"
	case 'g':
		return "#8E8E93"
	case 'w':
		return "#FFFFFF"
	default:
		return ""
	}
}

// pixelIcon renders a file-based <img> for client/brand icons, or an inline
// SVG for utility grid icons (globe, check, etc.). Names outside the builtin
// sets resolve to user-provided files under {staticDir}/icons/ when present.
func pixelIcon(staticDir, name string, px int) template.HTML {
	if px <= 0 {
		return ""
	}

	if filename, ok := fileIconSet[name]; ok {
		return template.HTML(fmt.Sprintf( //nolint:gosec // static icon names
			`<img class="px" width="%d" height="%d" src="static/icons/%s" style="image-rendering:pixelated" alt="" aria-hidden="true">`,
			px, px, filename))
	}

	grid, ok := pixelGrids[name]
	if !ok {
		// User-provided icon on disk (config icon: <name>); the page scripts
		// resolve the same {name}.svg convention for client icons.
		if validIconName(name) {
			if st, err := os.Stat(filepath.Join(staticDir, "icons", name+".svg")); err == nil && !st.IsDir() {
				return template.HTML(fmt.Sprintf(
					`<img class="px" width="%d" height="%d" src="static/icons/%s.svg" style="image-rendering:pixelated" alt="" aria-hidden="true">`,
					px, px, template.HTMLEscapeString(name)))
			}
		}
		return pixelIcon(staticDir, "singbox", px)
	}
	cols, rows := 0, len(grid)
	for _, row := range grid {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 || rows == 0 {
		return ""
	}
	height := px * rows / cols
	if height < 1 {
		height = 1
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="px" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges" aria-hidden="true">`,
		px, height, cols, rows)
	for y, row := range grid {
		x := 0
		for x < len(row) {
			fill := pixelCharColor(row[x])
			if fill == "" {
				x++
				continue
			}
			end := x + 1
			for end < len(row) && pixelCharColor(row[end]) == fill {
				end++
			}
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="1" fill="%s"/>`, x, y, end-x, fill)
			x = end
		}
	}
	b.WriteString("</svg>")
	return template.HTML(b.String()) //nolint:gosec // static developer-controlled grids
}
