package render

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

const singboxRuleSetVersion uint8 = 3

func isSingboxCodec(codec string) bool {
	return codec == "singbox" || codec == "singbox_srs"
}

// singboxRule is one rule object, shared by the JSON and SRS codecs. Both
// formats serialize the same conditions and only differ in the container, so
// the same rule object feeds both writers.
type singboxRule struct {
	// Mode is empty for a condition-only rule, and "and" or "or" for a logical
	// rule whose children are held in Subs.
	Mode   string
	Invert bool
	Subs   []singboxRule

	// Fields holds the string conditions (domains, CIDRs, port ranges, process
	// details) keyed by the sing-box field name.
	Fields map[string][]string
	// Ports holds port numbers keyed by the sing-box field name.
	Ports map[string][]uint16
}

// empty reports whether the rule carries no condition at all.
func (r singboxRule) empty() bool {
	return len(r.Fields) == 0 && len(r.Ports) == 0 && len(r.Subs) == 0
}

// renderSingbox renders entries as a sing-box JSON rule-set document.
func renderSingbox(tmpl *Template, entries []ir.Entry) ([]byte, error) {
	rules, err := buildSingboxRules(tmpl, entries)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	doc := map[string]any{
		"version": singboxRuleSetVersion,
		"rules":   singboxRulesJSON(rules),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("singbox marshal: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// singboxRulesJSON converts rule objects into the JSON shape sing-box expects.
func singboxRulesJSON(rules []singboxRule) []map[string]any {
	out := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rule.jsonObject())
	}
	return out
}

// jsonObject writes a rule as a sing-box rule object: a single condition value
// becomes a scalar, repeated values become an array.
func (r singboxRule) jsonObject() map[string]any {
	if r.Mode != "" {
		obj := map[string]any{
			"type":  "logical",
			"mode":  r.Mode,
			"rules": singboxRulesJSON(r.Subs),
		}
		if r.Invert {
			obj["invert"] = true
		}
		return obj
	}
	obj := make(map[string]any, len(r.Fields)+len(r.Ports))
	for field, values := range r.Fields {
		obj[field] = scalarOrList(values)
	}
	for field, values := range r.Ports {
		obj[field] = scalarOrList(values)
	}
	return obj
}

func scalarOrList[T any](values []T) any {
	if len(values) == 1 {
		return values[0]
	}
	return values
}

// buildSingboxRules groups entries into rule objects. Flat entries are grouped
// by field group so conditions sharing OR-semantics land in one object; logical
// entries each become their own rule object.
func buildSingboxRules(tmpl *Template, entries []ir.Entry) ([]singboxRule, error) {
	var rules []singboxRule
	var flatBatch []ir.Entry

	flushFlat := func() error {
		if len(flatBatch) == 0 {
			return nil
		}
		for _, g := range groupByFieldGroup(tmpl, flatBatch) {
			rule, err := buildSingboxRule(tmpl, g)
			if err != nil {
				return err
			}
			if !rule.empty() {
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
			if !rule.empty() {
				rules = append(rules, rule)
			}
			continue
		}
		flatBatch = append(flatBatch, entry)
	}
	if err := flushFlat(); err != nil {
		return nil, err
	}
	return rules, nil
}

// buildLogicalSingboxRule renders an AND/OR/NOT entry as a logical rule object.
// Sub-entries are recursively rendered: logical sub-entries become nested
// logical rules, flat sub-entries become normal rule objects.
func buildLogicalSingboxRule(tmpl *Template, entry ir.Entry) (singboxRule, error) {
	// sing-box accepts only "and" and "or" as logical modes. NOT is a
	// single-child AND whose match result is inverted.
	mode := string(entry.Kind)
	inverted := entry.Kind == ir.KindNot
	if inverted {
		mode = string(ir.KindAnd)
	}
	rule := singboxRule{Mode: mode, Invert: inverted}

	for _, sub := range entry.Sub {
		var (
			subRule singboxRule
			err     error
		)
		if sub.Kind.IsLogical() {
			subRule, err = buildLogicalSingboxRule(tmpl, sub)
		} else {
			subRule, err = buildSingboxRule(tmpl, entryGroup{entries: []ir.Entry{sub}})
		}
		if err != nil {
			return singboxRule{}, err
		}
		if !subRule.empty() {
			rule.Subs = append(rule.Subs, subRule)
		}
	}

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
		result = []entryGroup{{entries: append([]ir.Entry{}, entries...)}}
	}

	return result
}

func buildSingboxRule(tmpl *Template, g entryGroup) (singboxRule, error) {
	rule := singboxRule{Fields: map[string][]string{}, Ports: map[string][]uint16{}}

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
				return singboxRule{}, err
			}
			rule.Ports[field] = append(rule.Ports[field], ports...)
			if len(ranges) > 0 {
				rangeField, ok := resolvePortRangeField(tmpl, e.Kind)
				if !ok {
					return singboxRule{}, fmt.Errorf("template %q has no range field for %s", tmpl.ID, e.Kind)
				}
				rule.Fields[rangeField] = append(rule.Fields[rangeField], ranges...)
			}
			continue
		}

		rule.Fields[field] = append(rule.Fields[field], value)
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
// into port numbers and ranges.
func expandPortsForSingbox(value, rangeSep string) ([]uint16, []string, error) {
	if rangeSep == "" {
		rangeSep = ":"
	}
	parts := strings.Split(value, "/")
	var ports []uint16
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
		if err != nil || n < 0 || n > 65535 {
			return nil, nil, fmt.Errorf("invalid canonical port %q", p)
		}
		ports = append(ports, uint16(n))
	}
	return ports, ranges, nil
}
