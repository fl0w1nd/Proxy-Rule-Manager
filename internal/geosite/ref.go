package geosite

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

// GeositeRef is a parsed compact geosite reference like "v2fly/geolocation-!cn@ads".
type GeositeRef struct {
	Provider string
	List     string
	Attrs    []string
}

// ParseRef parses a compact geosite reference string.
// Format: "provider/list" or "provider/list@attr1,attr2"
// Examples:
//   - "v2fly/geolocation-!cn" -> {Provider: "v2fly", List: "geolocation-!cn"}
//   - "v2fly/geolocation-!cn@ads" -> {Provider: "v2fly", List: "geolocation-!cn", Attrs: ["ads"]}
//   - "loyalsoldier/google@cn,!ads" -> {Provider: "loyalsoldier", List: "google", Attrs: ["cn", "!ads"]}
func ParseRef(ref string) (GeositeRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return GeositeRef{}, fmt.Errorf("empty geosite reference")
	}

	slash := strings.IndexByte(ref, '/')
	if slash <= 0 {
		return GeositeRef{}, fmt.Errorf("geosite reference %q must contain provider/list", ref)
	}

	provider := ref[:slash]
	rest := ref[slash+1:]
	if rest == "" {
		return GeositeRef{}, fmt.Errorf("geosite reference %q has empty list name", ref)
	}

	var list string
	var attrs []string

	if at := strings.IndexByte(rest, '@'); at >= 0 {
		list = rest[:at]
		attrStr := rest[at+1:]
		if attrStr != "" {
			for _, a := range strings.Split(attrStr, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					attrs = append(attrs, a)
				}
			}
		}
	} else {
		list = rest
	}

	if list == "" {
		return GeositeRef{}, fmt.Errorf("geosite reference %q has empty list name", ref)
	}

	parsed := GeositeRef{
		Provider: strings.ToLower(strings.TrimSpace(provider)),
		List:     strings.ToLower(strings.TrimSpace(list)),
		Attrs:    NormalizeAttrs(attrs),
	}
	if err := ValidateRefSegments(parsed); err != nil {
		return GeositeRef{}, err
	}
	return parsed, nil
}

// ValidateRefSegments checks that a geosite ref is safe for artifact paths.
func ValidateRefSegments(ref GeositeRef) error {
	if err := util.EnsureSafeSegment(ref.Provider, "geosite provider"); err != nil {
		return err
	}
	if err := util.EnsureSafeSegment(ref.List, "geosite list"); err != nil {
		return err
	}
	for _, attr := range ref.Attrs {
		if err := util.EnsureSafeSegment(attr, "geosite attr"); err != nil {
			return err
		}
	}
	return nil
}

// FormatRef returns the canonical string representation of a GeositeRef.
func (r GeositeRef) FormatRef() string {
	base := r.Provider + "/" + r.List
	if len(r.Attrs) > 0 {
		return base + "@" + strings.Join(r.Attrs, ",")
	}
	return base
}

// ArtifactName returns a filesystem-safe name for geosite artifacts.
// Uses "/" as directory separator (provider is a subdirectory).
// Example: "v2fly/geolocation-!cn@ads"
func (r GeositeRef) ArtifactName() string {
	return r.FormatRef()
}

// ValidateRef checks whether a geosite reference points to a valid list and
// valid attrs in the given provider cache. Returns nil if valid.
func ValidateRef(cache *ProviderCache, ref GeositeRef) error {
	if cache == nil {
		return fmt.Errorf("provider %q cache not available", ref.Provider)
	}

	normalized := normalizeName(ref.List)
	if _, ok := cache.Entries[normalized]; !ok {
		return fmt.Errorf("geosite list %q not found in provider %q", ref.List, ref.Provider)
	}

	if len(ref.Attrs) == 0 {
		return nil
	}

	// Collect all known attrs for this list
	knownAttrs := collectListAttrs(cache, normalized)

	for _, attr := range ref.Attrs {
		if !knownAttrs[attr] {
			return fmt.Errorf("geosite attr %q not found in list %q of provider %q (known: %s)",
				attr, ref.List, ref.Provider, formatKnownAttrs(knownAttrs))
		}
	}
	return nil
}

// ListAvailableAttrs returns all known attrs for a list.
func ListAvailableAttrs(cache *ProviderCache, list string) []string {
	if cache == nil {
		return nil
	}
	known := collectListAttrs(cache, normalizeName(list))
	out := make([]string, 0, len(known))
	for k := range known {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectListAttrs(cache *ProviderCache, normalizedList string) map[string]bool {
	entries := cache.Entries[normalizedList]
	attrs := map[string]bool{}
	for _, e := range entries {
		for _, a := range e.Attrs {
			attrs[a] = true
		}
	}
	return attrs
}

func formatKnownAttrs(attrs map[string]bool) string {
	if len(attrs) == 0 {
		return "none"
	}
	list := make([]string, 0, len(attrs))
	for k := range attrs {
		list = append(list, k)
	}
	sort.Strings(list)
	if len(list) > 10 {
		return strings.Join(list[:10], ", ") + fmt.Sprintf(" ... (%d total)", len(list))
	}
	return strings.Join(list, ", ")
}
