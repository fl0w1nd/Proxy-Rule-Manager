package transformer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// --- mihomo classical → sing-box rule-set source (JSON, configurable mapping) ---
//
// sing-box rule-sets in source format are JSON documents shaped like:
//
//	{
//	  "version": 3,
//	  "rules": [
//	    { "domain": ["a.com"] },
//	    { "domain_suffix": [".cn"] },
//	    { "ip_cidr": ["1.1.1.1/32"] }
//	  ]
//	}
//
// Each rule object in the top-level "rules" array is OR-joined with the
// others; within a single object the listed fields are AND-joined. That
// matters because every line of a mihomo classical payload is a stand-
// alone "match this and apply" rule, so the safe rewrite is to emit one
// rule object per *sing-box field* and let the engine OR them together.
// Multiple values for the same field are collected into a single array so
// the output stays compact and stable across runs.
//
// The transformer is intentionally strict: only mihomo tokens we have an
// explicit sing-box mapping for survive. Logical combinators, GEO* rules,
// inbound-side matchers, regex variants without a sing-box analogue and
// other oddities are dropped with a reason that the preview panel can
// surface. Operators who need a different mapping can edit the mapping
// table from the dashboard; that's the same workflow as the shadowrocket
// built-in.

// SingboxSourceActionMap / Drop name the per-row actions the user can
// pick in the mapping editor. Renaming the leading token is not exposed
// here because the output is structured JSON, not text — the rewrite is
// "this mihomo token contributes its value to *that* sing-box field".
const (
	SingboxSourceActionMap  = "map"
	SingboxSourceActionDrop = "drop"
)

// DefaultSingboxSourceVersion picks the most widely useful rule-set
// schema version. Version 3 (sing-box 1.11.0+) is the floor for
// network_type / network_is_expensive / network_is_constrained and is
// the version sing-box's own docs default to in examples. Operators on
// older binaries can lower it from the UI.
const DefaultSingboxSourceVersion = 3

// MinSingboxSourceVersion / MaxSingboxSourceVersion bound the version
// selector. They match the version history in the sing-box rule-set
// source-format docs at the time of writing; raising the ceiling is a
// one-line change when a new schema lands.
const (
	MinSingboxSourceVersion = 1
	MaxSingboxSourceVersion = 5
)

// singboxFieldKind tells the writer how to serialise a bucket: as a JSON
// string array (the common case) or as a JSON integer array (for the
// port-style fields). Unknown kinds are a config bug — the validator
// rejects mapTo values outside this map at save time.
type singboxFieldKind int

const (
	singboxFieldKindString singboxFieldKind = iota
	singboxFieldKindInt
)

// singboxFieldKinds is the authoritative list of sing-box headless rule
// fields that the built-in knows how to emit. Kept lowercase to match
// the on-disk JSON keys; the UI exposes the same keys in its mapTo
// dropdown so user-edited mappings stay in lockstep.
//
// Notes on intentional omissions:
//   - logical rules ("type":"logical") and the rule_set/geosite/geoip
//     references aren't included because they don't carry plain values
//     mihomo classical can contribute.
//   - "ip_is_private" / "source_ip_is_private" / "network_is_expensive"
//     etc. are booleans, not lists, so they don't fit the bucket model
//     and aren't part of any realistic mihomo classical rewrite.
var singboxFieldKinds = map[string]singboxFieldKind{
	"domain":             singboxFieldKindString,
	"domain_suffix":      singboxFieldKindString,
	"domain_keyword":     singboxFieldKindString,
	"domain_regex":       singboxFieldKindString,
	"ip_cidr":            singboxFieldKindString,
	"source_ip_cidr":     singboxFieldKindString,
	"port":               singboxFieldKindInt,
	"source_port":        singboxFieldKindInt,
	"port_range":         singboxFieldKindString,
	"source_port_range":  singboxFieldKindString,
	"process_name":       singboxFieldKindString,
	"process_path":       singboxFieldKindString,
	"process_path_regex": singboxFieldKindString,
	"package_name":       singboxFieldKindString,
	"package_name_regex": singboxFieldKindString,
	"network":            singboxFieldKindString,
	"network_type":       singboxFieldKindString,
	"wifi_ssid":          singboxFieldKindString,
	"wifi_bssid":         singboxFieldKindString,
}

// singboxFieldMinVersion records the minimum rule-set source-format
// version a given headless-rule field appears in. Fields without an
// entry default to version 1 (the schema's floor).
//
// Sources, cross-referenced with sing-box source-format.md "Version
// History" and the per-field "added in sing-box X.Y.Z" notes in
// headless-rule.md:
//
//   - rule-set v2 (sing-box 1.10.0): process_path_regex
//   - rule-set v3 (sing-box 1.11.0): network_type, wifi_ssid, wifi_bssid
//     (network_type is called out in the changelog; the Wi-Fi state
//     matchers are gated on the same Wi-Fi state plumbing introduced
//     in 1.11)
//   - rule-set v5 (sing-box 1.14.0): package_name_regex
//
// Bumping a field here ripples through:
//   - decodeSingboxSourceParams (runtime drops cross-version rows)
//   - validateSingboxSourceParams (save-time rejection of incompatible
//     version + field combos)
//   - the SINGBOX_SOURCE_FIELD_MIN_VERSION map in src/lib/schema.ts
//     (frontend disables incompatible options in the dropdown)
var singboxFieldMinVersion = map[string]int{
	"process_path_regex": 2,
	"network_type":       3,
	"wifi_ssid":          3,
	"wifi_bssid":         3,
	"package_name_regex": 5,
}

// SingboxFieldMinVersion returns the minimum rule-set version the named
// sing-box field appears in. Unknown / version-1 fields return 1.
// Exposed for validators and any caller that needs to gate UI choices
// on the active schema version.
func SingboxFieldMinVersion(name string) int {
	if v, ok := singboxFieldMinVersion[name]; ok {
		return v
	}
	return 1
}

// singboxFieldOrder pins the order in which the writer emits rule
// objects. It follows the grouping the sing-box docs use in their own
// headless-rule example (domains → ips → ports → process → app →
// network metadata) so a human reading the generated JSON sees the
// rules clustered the same way the docs do.
var singboxFieldOrder = []string{
	"domain", "domain_suffix", "domain_keyword", "domain_regex",
	"source_ip_cidr", "ip_cidr",
	"source_port", "source_port_range", "port", "port_range",
	"process_name", "process_path", "process_path_regex",
	"package_name", "package_name_regex",
	"network", "network_type", "wifi_ssid", "wifi_bssid",
}

// SingboxSourceFields exposes the supported sing-box field names in the
// canonical emit order. The frontend uses this list to populate the
// "map to" dropdown so the table editor only ever offers fields the
// runner will accept.
func SingboxSourceFields() []string {
	out := make([]string, len(singboxFieldOrder))
	copy(out, singboxFieldOrder)
	return out
}

// IsSingboxSourceField reports whether name is a known sing-box headless
// rule field. Exposed for the validator so it can reject typoed mapTo
// values before they're persisted.
func IsSingboxSourceField(name string) bool {
	_, ok := singboxFieldKinds[name]
	return ok
}

// singboxSourceMapping is one row of the user-editable mapping table.
// Type is the mihomo classical leading token (case-sensitive); Action
// picks the runtime behaviour. MapTo is required when Action="map" and
// must reference a known sing-box field. Reason surfaces in the preview
// report so the operator can see why a line was dropped or rewritten.
type singboxSourceMapping struct {
	Type   string `json:"type"`
	Action string `json:"action"`
	MapTo  string `json:"mapTo,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// singboxSourceParams is the decoded shape of Transform.Params for the
// builtin:mihomo-classical-to-singbox-source transformer. Version pins
// the rule-set schema version emitted in the JSON header; an empty/zero
// value falls back to DefaultSingboxSourceVersion.
type singboxSourceParams struct {
	Version int                    `json:"version,omitempty"`
	Rules   []singboxSourceMapping `json:"rules"`
}

// DefaultSingboxSourceMapping returns the curated default table that
// ships with the binary. The frontend mirrors this list (in schema.ts)
// to seed its editor on first interaction; saving with empty Rules
// keeps the defaults.
//
// Grouping mirrors the shadowrocket default: passthroughs (map) first,
// then drops with self-explanatory reasons. Within each group, entries
// follow the sing-box field order so the table reads top-to-bottom in
// the same shape as the output JSON.
func DefaultSingboxSourceMapping() []singboxSourceMapping {
	return []singboxSourceMapping{
		// --- maps: mihomo token → sing-box headless rule field ---
		{Type: "DOMAIN", Action: SingboxSourceActionMap, MapTo: "domain"},
		{Type: "DOMAIN-SUFFIX", Action: SingboxSourceActionMap, MapTo: "domain_suffix"},
		{Type: "DOMAIN-KEYWORD", Action: SingboxSourceActionMap, MapTo: "domain_keyword"},
		{Type: "DOMAIN-REGEX", Action: SingboxSourceActionMap, MapTo: "domain_regex"},
		// mihomo IP-CIDR / IP-CIDR6 both land in sing-box ip_cidr (it
		// accepts both address families). IP-SUFFIX is mihomo's
		// CIDR-by-suffix shorthand and is syntactically identical.
		{Type: "IP-CIDR", Action: SingboxSourceActionMap, MapTo: "ip_cidr"},
		{Type: "IP-CIDR6", Action: SingboxSourceActionMap, MapTo: "ip_cidr"},
		{Type: "IP-SUFFIX", Action: SingboxSourceActionMap, MapTo: "ip_cidr"},
		{Type: "SRC-IP-CIDR", Action: SingboxSourceActionMap, MapTo: "source_ip_cidr"},
		{Type: "SRC-IP-SUFFIX", Action: SingboxSourceActionMap, MapTo: "source_ip_cidr"},
		{Type: "DST-PORT", Action: SingboxSourceActionMap, MapTo: "port"},
		{Type: "SRC-PORT", Action: SingboxSourceActionMap, MapTo: "source_port"},
		{Type: "PROCESS-NAME", Action: SingboxSourceActionMap, MapTo: "process_name"},
		{Type: "PROCESS-PATH", Action: SingboxSourceActionMap, MapTo: "process_path"},
		{Type: "PROCESS-PATH-REGEX", Action: SingboxSourceActionMap, MapTo: "process_path_regex"},
		{Type: "NETWORK", Action: SingboxSourceActionMap, MapTo: "network"},
		// --- drops with explanatory reasons ---
		{Type: "GEOIP", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不内联 GEOIP，请改用独立 rule-set 引用 geoip-cn 之类"},
		{Type: "GEOSITE", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不内联 GEOSITE，请改用独立 rule-set"},
		{Type: "SRC-GEOIP", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不支持 SRC-GEOIP"},
		{Type: "IP-ASN", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不支持 IP-ASN"},
		{Type: "SRC-IP-ASN", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不支持 SRC-IP-ASN"},
		{Type: "DOMAIN-WILDCARD", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 domain_wildcard 字段；如必须迁移请改写为 domain_regex"},
		{Type: "PROCESS-NAME-REGEX", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 process_name_regex 字段"},
		{Type: "PROCESS-NAME-WILDCARD", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 process_name_wildcard 字段"},
		{Type: "PROCESS-PATH-WILDCARD", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 process_path_wildcard 字段"},
		{Type: "IN-PORT", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不携带入站匹配（应在 route rule 上配置 inbound）"},
		{Type: "IN-TYPE", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不携带入站类型"},
		{Type: "IN-USER", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不携带入站用户"},
		{Type: "IN-NAME", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不携带入站名"},
		{Type: "UID", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 UID 等价字段（如需匹配请改用 user/user_id）"},
		{Type: "DSCP", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 DSCP 字段"},
		{Type: "PROTOCOL", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 PROTOCOL 字段"},
		{Type: "USER-AGENT", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 USER-AGENT 字段"},
		{Type: "URL-REGEX", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 URL-REGEX 字段"},
		{Type: "RULE-SET", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不支持嵌套引用其它规则集"},
		{Type: "SUB-RULE", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 不支持 SUB-RULE"},
		{Type: "AND", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 单条 rule 内字段已是 AND；不接受嵌套 AND 表达式"},
		{Type: "OR", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 顶层 rules 已是 OR；不接受嵌套 OR 表达式"},
		{Type: "NOT", Action: SingboxSourceActionDrop, Reason: "sing-box headless rule 用 invert:true 实现取反，且整条规则只能整体取反"},
		{Type: "MATCH", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 MATCH/FINAL 概念，最终归属在 route rule 配置"},
		{Type: "FINAL", Action: SingboxSourceActionDrop, Reason: "sing-box rule-set 无 MATCH/FINAL 概念"},
	}
}

// decodeSingboxSourceParams parses the persisted params blob and applies
// safe fallbacks. The decoder distinguishes three cases for the `rules`
// field so an explicit operator intent is never silently overridden:
//
//  1. raw is empty / missing       → fall back to DefaultSingboxSourceMapping
//  2. raw has no "rules" key at all → fall back to DefaultSingboxSourceMapping
//  3. raw contains "rules": [...]   → honour the supplied list verbatim,
//     even if the list is empty (the
//     "drop everything" user intent)
//
// json.Unmarshal collapses cases (2) and (3) for the empty-array side
// (both leave out.Rules nil after the call when the key is absent and
// a non-nil empty slice when the key is present and []), so we read
// out.Rules == nil as the unambiguous signal for "operator never set
// this field".
func decodeSingboxSourceParams(raw json.RawMessage) singboxSourceParams {
	out := singboxSourceParams{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if out.Rules == nil {
		out.Rules = DefaultSingboxSourceMapping()
	}
	if out.Version < MinSingboxSourceVersion || out.Version > MaxSingboxSourceVersion {
		out.Version = DefaultSingboxSourceVersion
	}
	return out
}

// singboxBucket accumulates the values destined for one sing-box field
// across the input. We keep the two storage types side-by-side rather
// than `any` so the writer doesn't have to type-assert on every value
// and so the per-kind dedupe stays trivially correct.
type singboxBucket struct {
	strings    []string
	stringSeen map[string]struct{}
	ints       []int
	intSeen    map[int]struct{}
}

func (b *singboxBucket) appendString(v string) bool {
	if b.stringSeen == nil {
		b.stringSeen = make(map[string]struct{})
	}
	if _, dup := b.stringSeen[v]; dup {
		return false
	}
	b.stringSeen[v] = struct{}{}
	b.strings = append(b.strings, v)
	return true
}

func (b *singboxBucket) appendInt(v int) bool {
	if b.intSeen == nil {
		b.intSeen = make(map[int]struct{})
	}
	if _, dup := b.intSeen[v]; dup {
		return false
	}
	b.intSeen[v] = struct{}{}
	b.ints = append(b.ints, v)
	return true
}

// extractClassicalValue isolates the value cell from the residue left
// by classifyClassicalLine. A mihomo classical rule may carry a
// trailing policy / modifier (`,DIRECT,no-resolve`); the sing-box
// headless rule has no policy slot and the rule-set's hosting
// `route.rules[]` entry drives the equivalent of `no-resolve`, so we
// discard everything past the value column. How we *find* that column
// is type-dependent:
//
//   - Non-regex types reserve the comma as the column separator, so
//     the value cannot contain one. We take everything up to the
//     first comma and call it done.
//
//   - Regex types (DOMAIN-REGEX, PROCESS-NAME-REGEX, PROCESS-PATH-REGEX,
//     etc.) carry a regex literal whose value column may legitimately
//     contain commas — `{1,3}` repetition quantifiers and bare
//     alternations being the obvious examples. Naively splitting on
//     the first comma would corrupt the regex (turning `^a{1,3}\.com$`
//     into the meaningless `^a{1`). Instead we split on every comma,
//     pop trailing known modifiers (e.g. `no-resolve`) from the right,
//     then pop one trailing policy column iff the segment looks
//     policy-shaped (no regex metacharacters). Anything left is the
//     value, rejoined with the commas the regex originally used.
//     The heuristic is intentionally conservative: when we're unsure
//     whether a comma-segment is part of the regex or a stray policy,
//     we keep it as part of the regex — corrupting a regex is far
//     worse than emitting a slightly-too-long value that sing-box's
//     regex engine will then reject explicitly.
func extractClassicalValue(typ, rest string) string {
	rest = strings.TrimSpace(rest)
	if !isRegexRuleType(typ) {
		if idx := strings.IndexByte(rest, ','); idx >= 0 {
			return strings.TrimSpace(rest[:idx])
		}
		return rest
	}
	parts := strings.Split(rest, ",")
	// Pop trailing known modifiers (zero or more — mihomo only allows
	// one in practice but we don't lose anything by looping).
	for len(parts) > 1 && isKnownClassicalModifier(strings.TrimSpace(parts[len(parts)-1])) {
		parts = parts[:len(parts)-1]
	}
	// Pop one trailing policy iff the last segment is plausibly a
	// policy name. A segment containing regex metacharacters can't be
	// a policy (those characters never appear in outbound / group
	// names), so we treat it as part of the regex value.
	if len(parts) > 1 {
		last := strings.TrimSpace(parts[len(parts)-1])
		if looksLikePolicy(last) {
			parts = parts[:len(parts)-1]
		}
	}
	return strings.TrimSpace(strings.Join(parts, ","))
}

// isRegexRuleType reports whether the mihomo token names a regex-style
// matcher whose value column may legitimately contain commas. Today
// the convention is a trailing `-REGEX` suffix (DOMAIN-REGEX,
// PROCESS-NAME-REGEX, PROCESS-PATH-REGEX); any future *-REGEX token
// will be covered automatically.
func isRegexRuleType(typ string) bool {
	return strings.HasSuffix(typ, "-REGEX")
}

// isKnownClassicalModifier names the closed set of suffix tokens
// mihomo allows after a classical rule's value/policy columns. The
// list is intentionally short and explicit so a typoed policy name
// (e.g. "direct" with the wrong case) doesn't get silently swallowed
// as a modifier — the explicit allow-list means anything we don't
// recognise stays in the row and is then handled by the policy-shape
// heuristic.
func isKnownClassicalModifier(s string) bool {
	switch s {
	case "no-resolve", "src":
		return true
	}
	return false
}

// looksLikePolicy returns true when s could plausibly be a policy
// name (a mihomo outbound or group identifier). Policies are
// conventional identifiers: they don't contain regex metacharacters
// or backslashes, so the presence of any such character is a strong
// signal that the segment is part of a regex value that happened to
// contain a comma. The check is byte-wise rather than rune-wise
// because every blocklist character is ASCII; CJK policy names (which
// mihomo allows) survive unchanged.
func looksLikePolicy(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{', '}', '(', ')', '[', ']', '?', '+', '*', '|', '\\', '^', '$':
			return false
		}
	}
	return true
}

// runMihomoClassicalToSingboxSource rewrites a mihomo classical rule
// list as a sing-box rule-set source document. Each retained rule is
// recorded as a ModifiedLine (so the preview can show "DOMAIN,a.com →
// domain: \"a.com\""); dropped rules land in the Dropped track with the
// mapping table's reason or a default placeholder.
func runMihomoClassicalToSingboxSource(rawParams json.RawMessage, content string) BuiltinResult {
	params := decodeSingboxSourceParams(rawParams)
	mapping := make(map[string]singboxSourceMapping, len(params.Rules))
	for _, r := range params.Rules {
		if r.Type == "" {
			continue
		}
		mapping[r.Type] = r
	}

	buckets := make(map[string]*singboxBucket)

	var (
		dropped       []DroppedLine
		droppedTotal  int
		modified      []ModifiedLine
		modifiedTotal int
	)

	lines := splitClassicalLines(content)
	for _, line := range lines {
		typ, rest, isComment, isBlank := classifyClassicalLine(line.raw)
		if isBlank {
			continue
		}
		if isComment {
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: "comment removed",
			})
			continue
		}
		if typ == "" {
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: "missing rule type",
			})
			continue
		}

		rule, known := mapping[typ]
		if !known {
			// Unknown types are dropped by default: emitting them would
			// have to invent a sing-box field, and the rule-set parser
			// rejects unknown keys outright. The reason is explicit so
			// the operator knows why the line vanished.
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: "类型 " + typ + " 未在映射表中配置（默认按 drop 处理）",
			})
			continue
		}

		switch rule.Action {
		case SingboxSourceActionDrop:
			reason := rule.Reason
			if reason == "" {
				reason = "类型 " + typ + " 已被映射表标记为丢弃"
			}
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: reason,
			})
		case SingboxSourceActionMap:
			field := strings.TrimSpace(rule.MapTo)
			kind, ok := singboxFieldKinds[field]
			if !ok {
				dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
					LineNo: line.lineNo,
					Text:   line.raw,
					Reason: "映射目标 " + field + " 不是已知的 sing-box headless rule 字段",
				})
				continue
			}
			// Defence-in-depth: even if the validator missed it (e.g.
			// a config saved before this check existed), drop rows
			// that target a field newer than the selected schema
			// version so the produced rule-set still compiles on the
			// declared sing-box release.
			if minVer := SingboxFieldMinVersion(field); minVer > params.Version {
				dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
					LineNo: line.lineNo,
					Text:   line.raw,
					Reason: fmt.Sprintf("字段 %s 要求 sing-box rule-set version ≥ %d，当前 version=%d", field, minVer, params.Version),
				})
				continue
			}
			value := extractClassicalValue(typ, rest)
			if value == "" {
				dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
					LineNo: line.lineNo,
					Text:   line.raw,
					Reason: "缺少匹配值",
				})
				continue
			}

			b, exists := buckets[field]
			if !exists {
				b = &singboxBucket{}
				buckets[field] = b
			}

			var rewritten string
			switch kind {
			case singboxFieldKindString:
				if !b.appendString(value) {
					// Duplicate values are folded into the same bucket
					// without emitting a Dropped record — the user
					// asked for "this token maps to that field" and we
					// did exactly that; the dedupe is just hygiene on
					// the output JSON. Still record a ModifiedLine so
					// the preview can show the rewrite intent.
				}
				rewritten = `"` + field + `": ` + jsonStringLiteral(value)
			case singboxFieldKindInt:
				n, err := strconv.Atoi(value)
				if err != nil {
					dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
						LineNo: line.lineNo,
						Text:   line.raw,
						Reason: "无法将 " + value + " 解析为整数（" + field + " 字段要求 int）",
					})
					continue
				}
				if n < 0 || n > 65535 {
					dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
						LineNo: line.lineNo,
						Text:   line.raw,
						Reason: "端口 " + value + " 越界（合法范围 0-65535）",
					})
					continue
				}
				b.appendInt(n)
				rewritten = `"` + field + `": ` + strconv.Itoa(n)
			}

			reason := rule.Reason
			if reason == "" {
				reason = typ + " → sing-box " + field
			}
			modified = AppendModified(modified, &modifiedTotal, ModifiedLine{
				LineNo: line.lineNo,
				From:   strings.TrimSpace(line.raw),
				To:     rewritten,
				Reason: reason,
			})
		default:
			// Unknown action enum (a config typo from a future schema)
			// → drop with a synthetic reason so the operator can see
			// the bad row.
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: "映射动作 " + rule.Action + " 不支持，按 drop 处理",
			})
		}
	}

	output := writeSingboxSourceJSON(params.Version, buckets)
	return BuiltinResult{
		Output:        output,
		Dropped:       dropped,
		Modified:      modified,
		DroppedTotal:  droppedTotal,
		ModifiedTotal: modifiedTotal,
	}
}

// writeSingboxSourceJSON renders the accumulated buckets as the final
// sing-box source-format document. We hand-roll the encoder rather than
// reach for json.MarshalIndent because:
//
//   - we want a deterministic field order (singboxFieldOrder), which a
//     map-backed marshal can't promise across Go versions;
//   - we want a stable two-space indent regardless of how Go's encoder
//     evolves;
//   - the document is small enough (one map + a flat list of single-key
//     objects) that hand-writing it stays readable.
//
// String values are deduplicated within their field; integer values
// likewise. Order within an array is *insertion order* — i.e. the order
// the matching mihomo lines appeared — so a downstream diff stays
// minimal when the input only changes in one spot.
func writeSingboxSourceJSON(version int, buckets map[string]*singboxBucket) string {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("  \"version\": ")
	sb.WriteString(strconv.Itoa(version))
	sb.WriteString(",\n")

	emitted := make([]string, 0, len(buckets))
	for _, field := range singboxFieldOrder {
		b, ok := buckets[field]
		if !ok {
			continue
		}
		switch singboxFieldKinds[field] {
		case singboxFieldKindString:
			if len(b.strings) > 0 {
				emitted = append(emitted, field)
			}
		case singboxFieldKindInt:
			if len(b.ints) > 0 {
				emitted = append(emitted, field)
			}
		}
	}

	sb.WriteString("  \"rules\": [")
	if len(emitted) == 0 {
		sb.WriteString("]\n")
		sb.WriteString("}\n")
		return sb.String()
	}
	sb.WriteByte('\n')
	for i, field := range emitted {
		sb.WriteString("    { \"")
		sb.WriteString(field)
		sb.WriteString("\": ")
		switch singboxFieldKinds[field] {
		case singboxFieldKindString:
			sb.WriteString(writeStringArray(buckets[field].strings))
		case singboxFieldKindInt:
			sb.WriteString(writeIntArray(buckets[field].ints))
		}
		sb.WriteString(" }")
		if i < len(emitted)-1 {
			sb.WriteByte(',')
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("  ]\n")
	sb.WriteString("}\n")
	return sb.String()
}

// writeStringArray renders a JSON array of strings inline. Each value
// runs through jsonStringLiteral so embedded quotes / backslashes /
// control characters survive without breaking the document.
func writeStringArray(values []string) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(jsonStringLiteral(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// writeIntArray renders a JSON array of ints inline. The values are
// expected to be in 0..65535 (the runner pre-validates that); we don't
// sort because insertion order is part of the contract.
func writeIntArray(values []int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// jsonStringLiteral escapes s as a JSON string literal. We rely on
// encoding/json to handle the escape table (it knows about \uXXXX for
// control codes) and then return its output verbatim. This is a tiny
// allocation per value but keeps us out of the business of maintaining
// an escape table by hand.
func jsonStringLiteral(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal on a string never returns an error in practice
		// (UTF-8 inputs always succeed), but if it ever did we fall
		// back to a literal empty string rather than panic the sync
		// pipeline.
		return `""`
	}
	return string(b)
}

// SortedSingboxFields is a small convenience for tests / debug dumps
// that need a name-sorted view of the supported fields. The runtime
// emits in singboxFieldOrder (which is intentionally not alphabetical
// — it groups by category).
func SortedSingboxFields() []string {
	out := SingboxSourceFields()
	sort.Strings(out)
	return out
}
