package render

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

// renderSingbox renders entries as a sing-box JSON rule-set document.
// Logical entries (AND/OR/NOT) are emitted as {"type":"logical","mode":...}
// rule objects; flat entries are grouped by field group as usual.
func renderSingbox(tmpl *Template, entries []ir.Entry) ([]byte, error) {
	const ruleSetVersion = 3

	var rules []map[string]any
	var flatBatch []ir.Entry

	// flushFlat processes accumulated flat entries through groupByFieldGroup
	// and appends the resulting rule objects to rules.
	flushFlat := func() error {
		if len(flatBatch) == 0 {
			return nil
		}
		groups := groupByFieldGroup(tmpl, flatBatch)
		for _, g := range groups {
			rule, err := buildSingboxRule(tmpl, g)
			if err != nil {
				return err
			}
			if len(rule) > 0 {
				rules = append(rules, rule)
			}
		}
		flatBatch = nil
		return nil
	}

	for _, entry := range entries {
		if entry.Kind.IsLogical() {
			if err := flushFlat(); err != nil {
				return nil, err
			}
			rule, err := buildLogicalSingboxRule(tmpl, entry)
			if err != nil {
				return nil, err
			}
			if len(rule) > 0 {
				rules = append(rules, rule)
			}
			continue
		}
		flatBatch = append(flatBatch, entry)
	}
	if err := flushFlat(); err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	doc := map[string]any{
		"version": ruleSetVersion,
		"rules":   rules,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("singbox marshal: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// buildLogicalSingboxRule renders an AND/OR/NOT entry as a sing-box logical
// rule object. Sub-entries are recursively rendered: logical sub-entries
// become nested logical rules, flat sub-entries become normal rule objects.
func buildLogicalSingboxRule(tmpl *Template, entry ir.Entry) (map[string]any, error) {
	rule := map[string]any{
		"type": "logical",
		"mode": string(entry.Kind), // "and", "or", "not"
	}

	var subRules []map[string]any
	for _, sub := range entry.Sub {
		if sub.Kind.IsLogical() {
			subRule, err := buildLogicalSingboxRule(tmpl, sub)
			if err != nil {
				return nil, err
			}
			if len(subRule) > 0 {
				subRules = append(subRules, subRule)
			}
		} else {
			subRule, err := buildSingboxRule(tmpl, entryGroup{entries: []ir.Entry{sub}})
			if err != nil {
				return nil, err
			}
			if len(subRule) > 0 {
				subRules = append(subRules, subRule)
			}
		}
	}

	if len(subRules) == 0 {
		return nil, nil
	}
	rule["rules"] = subRules
	return rule, nil
}

type entryGroup struct {
	groupName string
	entries   []ir.Entry
}

// groupByFieldGroup groups entries by the field group they belong to.
// Entries sharing a field group are combined into one rule object.
func groupByFieldGroup(tmpl *Template, entries []ir.Entry) []entryGroup {
	fieldToGroup := map[string]string{}
	for _, fg := range tmpl.FieldGroups {
		for _, f := range fg.Fields {
			fieldToGroup[f] = fg.Name
		}
	}

	groupMap := map[string]*entryGroup{}
	var order []string
	var ungrouped []ir.Entry

	for _, e := range entries {
		field, ok := resolveFieldName(tmpl, e)
		if !ok {
			continue
		}
		gname, inGroup := fieldToGroup[field]
		if !inGroup {
			ungrouped = append(ungrouped, e)
			continue
		}
		if g, ok := groupMap[gname]; ok {
			g.entries = append(g.entries, e)
		} else {
			groupMap[gname] = &entryGroup{groupName: gname, entries: []ir.Entry{e}}
			order = append(order, gname)
		}
	}

	var result []entryGroup
	for _, name := range order {
		result = append(result, *groupMap[name])
	}

	// Ungrouped entries each become their own rule object.
	for _, e := range ungrouped {
		result = append(result, entryGroup{entries: []ir.Entry{e}})
	}

	if len(result) == 0 && len(entries) > 0 {
		// If no field groups defined, put all entries in one rule.
		var flat []ir.Entry
		flat = append(flat, entries...)
		if len(flat) > 0 {
			result = []entryGroup{{entries: flat}}
		}
	}

	return result
}

func buildSingboxRule(tmpl *Template, g entryGroup) (map[string]any, error) {
	rule := map[string]any{}
	fieldValues := map[string][]any{}

	for _, e := range g.entries {
		field, ok := resolveFieldName(tmpl, e)
		if !ok {
			continue
		}

		value := applyTransform(tmpl, e)
		hint := tmpl.Hints[string(e.Kind)]

		if hint.LeadingDot {
			value = ensureLeadingDot(value)
		}

		if e.Kind == ir.KindDstPort || e.Kind == ir.KindSrcPort {
			ports, ranges, err := expandPortsForSingbox(value, hint.PortRangeSep)
			if err != nil {
				return nil, err
			}
			fieldValues[field] = append(fieldValues[field], ports...)
			if len(ranges) > 0 {
				rangeField, ok := resolvePortRangeField(tmpl, e.Kind)
				if !ok {
					return nil, fmt.Errorf("template %q has no range field for %s", tmpl.ID, e.Kind)
				}
				for _, rv := range ranges {
					fieldValues[rangeField] = append(fieldValues[rangeField], rv)
				}
			}
			continue
		}

		fieldValues[field] = append(fieldValues[field], value)
	}

	for field, vals := range fieldValues {
		if len(vals) == 1 {
			rule[field] = vals[0]
		} else {
			rule[field] = vals
		}
	}

	return rule, nil
}

func resolvePortRangeField(tmpl *Template, kind ir.Kind) (string, bool) {
	mapping, ok := tmpl.KindMap[string(kind)+"_range"]
	if !ok {
		return "", false
	}
	if mapping.FieldName != "" {
		return mapping.FieldName, true
	}
	return mapping.TypeName, mapping.TypeName != ""
}

// expandPortsForSingbox converts an IR canonical port value (e.g. "80/443/8000-9000")
// into singbox-appropriate values.
func expandPortsForSingbox(value, rangeSep string) ([]any, []string, error) {
	if rangeSep == "" {
		rangeSep = ":"
	}
	parts := strings.Split(value, "/")
	var ports []any
	var ranges []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "-") {
			ranges = append(ranges, strings.ReplaceAll(p, "-", rangeSep))
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid canonical port %q", p)
		}
		ports = append(ports, n)
	}
	return ports, ranges, nil
}
