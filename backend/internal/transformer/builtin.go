package transformer

import (
	"encoding/json"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// BuiltinPrefix tags every built-in transformer name. The prefix is treated
// as reserved: the config save route refuses any user-defined transformer
// whose key starts with it, and the dispatcher in executeNewTransform
// resolves the prefix to a native Go runner without going through the JS
// sandbox.
const BuiltinPrefix = "builtin:"

// Names of the built-in transformers. Kept as constants so callers can
// reference them without fearing typos.
const (
	BuiltinMihomoClassicalToYAML          = "builtin:mihomo-classical-to-yaml"
	BuiltinMihomoToShadowrocket           = "builtin:mihomo-to-shadowrocket"
	BuiltinMihomoClassicalToSingboxSource = "builtin:mihomo-classical-to-singbox-source"
)

// BuiltinTransformerMeta carries the human-readable metadata that the UI
// shows when a user picks a built-in from a dropdown. The Script field of
// the ScriptTransformer counterpart is intentionally empty: built-ins are
// implemented in Go, not in the JS sandbox, and the UI uses the empty
// script as a signal to render the lock badge.
type BuiltinTransformerMeta struct {
	Name        string
	Description string
}

// builtinMetas is the source of truth for the built-in registry. Order
// matters because it controls dropdown order in the UI. Descriptions are
// Chinese to match the rest of the dashboard copy.
var builtinMetas = []BuiltinTransformerMeta{
	{
		Name:        BuiltinMihomoClassicalToYAML,
		Description: "将 mihomo classical (.list) 规则渲染为 mihomo `payload:` YAML",
	},
	{
		Name:        BuiltinMihomoToShadowrocket,
		Description: "按映射表把 mihomo classical 规则改写为 Shadowrocket 子集；未配置的类型默认保留",
	},
	{
		Name:        BuiltinMihomoClassicalToSingboxSource,
		Description: "将 mihomo classical 规则按映射表聚合输出为 sing-box rule-set source（headless rule JSON）",
	},
}

// BuiltinTransformers returns the registry as a ScriptTransformer map so
// callers (in particular ProcessRule) can merge it into the user
// transformers map. The returned map is a fresh copy on every call; callers
// may mutate it freely.
func BuiltinTransformers() map[string]schema.ScriptTransformer {
	out := make(map[string]schema.ScriptTransformer, len(builtinMetas))
	for _, m := range builtinMetas {
		out[m.Name] = schema.ScriptTransformer{
			Name:        m.Name,
			Description: m.Description,
			Script:      "", // sentinel: implemented in Go
		}
	}
	return out
}

// MergeBuiltinTransformers returns a new map that contains every entry from
// the user-provided transformers plus the built-in registry. Built-ins
// always win on name collisions — that's a defence-in-depth check: the
// config save route already rejects user names with the reserved prefix,
// but if a stale config slipped through we still want the dispatcher to
// resolve "builtin:foo" to the native implementation.
func MergeBuiltinTransformers(user map[string]schema.ScriptTransformer) map[string]schema.ScriptTransformer {
	out := make(map[string]schema.ScriptTransformer, len(user)+len(builtinMetas))
	for k, v := range user {
		if HasBuiltinPrefix(k) {
			// Drop user override of reserved namespace.
			continue
		}
		out[k] = v
	}
	for k, v := range BuiltinTransformers() {
		out[k] = v
	}
	return out
}

// BuiltinMetas returns a copy of the metadata slice for callers that need
// the ordered view (e.g. the API endpoint that surfaces built-ins to the
// frontend).
func BuiltinMetas() []BuiltinTransformerMeta {
	out := make([]BuiltinTransformerMeta, len(builtinMetas))
	copy(out, builtinMetas)
	return out
}

// IsBuiltinName reports whether the given name belongs to the built-in
// registry. False is returned for any name that has the prefix but is not
// an actual implementation, so the route validator and the dispatcher
// share a single source of truth.
func IsBuiltinName(name string) bool {
	switch name {
	case BuiltinMihomoClassicalToYAML,
		BuiltinMihomoToShadowrocket,
		BuiltinMihomoClassicalToSingboxSource:
		return true
	}
	return false
}

// HasBuiltinPrefix reports whether name starts with the reserved prefix.
// Used by the config save route to reject user-defined transformers whose
// key collides with the reserved namespace, regardless of whether the
// exact name is currently a built-in.
func HasBuiltinPrefix(name string) bool {
	return strings.HasPrefix(name, BuiltinPrefix)
}

// BuiltinResult bundles the outputs of a built-in transformer in a single
// struct so callers don't juggle four-tuple returns. Output is the new
// content; Dropped/Modified samples are capped at MaxReportSamples while
// DroppedTotal/ModifiedTotal record the full pre-cap counts.
type BuiltinResult struct {
	Output        string
	Dropped       []DroppedLine
	Modified      []ModifiedLine
	DroppedTotal  int
	ModifiedTotal int
}

// RunBuiltin dispatches to the named built-in. params carries the optional
// transform-specific configuration (raw JSON); each runner decodes its own
// shape and falls back to defaults on empty/invalid input. The second
// return value is false when the name is not a recognised built-in;
// callers should treat that as "fall through to the user-script branch"
// rather than a hard error so an unknown builtin: name in a stale config
// behaves the same way as a missing user-defined transformer (no-op).
func RunBuiltin(name string, params json.RawMessage, content string) (BuiltinResult, bool) {
	switch name {
	case BuiltinMihomoClassicalToYAML:
		return runMihomoClassicalToYAML(content), true
	case BuiltinMihomoToShadowrocket:
		return runMihomoToShadowrocket(params, content), true
	case BuiltinMihomoClassicalToSingboxSource:
		return runMihomoClassicalToSingboxSource(params, content), true
	}
	return BuiltinResult{Output: content}, false
}

// splitClassicalLines normalises CR/LF, splits, and pairs every line with
// its 1-indexed line number so diagnostics can pinpoint the source line
// even when comments/blanks are skipped.
func splitClassicalLines(content string) []classicalLine {
	if content == "" {
		return nil
	}
	normalised := strings.ReplaceAll(content, "\r\n", "\n")
	normalised = strings.ReplaceAll(normalised, "\r", "\n")
	parts := strings.Split(normalised, "\n")
	out := make([]classicalLine, 0, len(parts))
	for i, p := range parts {
		out = append(out, classicalLine{lineNo: i + 1, raw: p})
	}
	return out
}

type classicalLine struct {
	lineNo int
	raw    string
}

// classifyClassicalLine returns the leading token (type) of a classical
// rule line and a trimmed copy of the rest. Comments and blank lines are
// signalled by an empty type. Token comparison is case-sensitive — mihomo
// and shadowrocket both reject lowercase variants, so this matches actual
// loader behaviour.
func classifyClassicalLine(raw string) (typ, rest string, isComment, isBlank bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false, true
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return "", "", true, false
	}
	idx := strings.IndexByte(trimmed, ',')
	if idx < 0 {
		// Some rule types (FINAL, MATCH) appear without a comma when they
		// only carry a policy that's been omitted; treat the whole line as
		// the type token.
		return trimmed, "", false, false
	}
	return strings.TrimSpace(trimmed[:idx]), strings.TrimSpace(trimmed[idx+1:]), false, false
}

// runMihomoClassicalToYAML emits a mihomo `payload` YAML document where
// every retained rule line is reproduced verbatim as a single-quoted YAML
// string. Comments and blanks are not propagated — yaml rule-provider
// readers don't expect them and we already decided to ship zero managed
// header. The output is terminated by a trailing newline so yamllint /
// git diff / cat all stay happy.
func runMihomoClassicalToYAML(content string) BuiltinResult {
	lines := splitClassicalLines(content)
	if len(lines) == 0 {
		return BuiltinResult{Output: "payload: []\n"}
	}
	var (
		dropped      []DroppedLine
		droppedTotal int
		payload      []string
	)
	for _, line := range lines {
		typ, _, isComment, isBlank := classifyClassicalLine(line.raw)
		if isBlank {
			continue
		}
		if isComment {
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: "comment removed by yaml renderer",
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
		payload = append(payload, strings.TrimSpace(line.raw))
	}

	var sb strings.Builder
	if len(payload) == 0 {
		sb.WriteString("payload: []\n")
	} else {
		sb.WriteString("payload:\n")
		for _, item := range payload {
			sb.WriteString("  - '")
			sb.WriteString(yamlSingleQuoteEscape(item))
			sb.WriteString("'\n")
		}
	}
	return BuiltinResult{
		Output:       sb.String(),
		Dropped:      dropped,
		DroppedTotal: droppedTotal,
	}
}

// yamlSingleQuoteEscape escapes single quotes by doubling them, per the
// YAML 1.1/1.2 single-quoted scalar grammar. We deliberately pick
// single-quoted strings so we don't have to escape backslashes (rare but
// possible in DOMAIN-KEYWORD values).
func yamlSingleQuoteEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// --- mihomo → shadowrocket (configurable mapping) ---

// ShadowrocketMappingAction enumerates the per-type actions a user can pick
// in the frontend mapping editor. They are kept as plain strings so the
// json.RawMessage payload remains stable when round-tripped through the
// config save endpoint.
const (
	ShadowrocketActionKeep   = "keep"
	ShadowrocketActionRename = "rename"
	ShadowrocketActionDrop   = "drop"
)

// shadowrocketMapping is a single mapping table row. Type is the leading
// classical token (case-sensitive); Action picks the runtime behaviour.
// RenameTo is required when Action="rename" and is otherwise ignored.
// Reason is surfaced in the preview report for both rename and drop rows,
// so the operator can see *why* a line was dropped without inspecting the
// mapping table.
type shadowrocketMapping struct {
	Type     string `json:"type"`
	Action   string `json:"action"`
	RenameTo string `json:"renameTo,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// shadowrocketParams is the decoded shape of Transform.Params for the
// builtin:mihomo-to-shadowrocket transformer. UnknownAction controls what
// happens to a classical rule type that isn't in Rules; defaults to
// "keep" so unrecognised types (e.g. a brand-new DOMAIN-NEW token from a
// future mihomo release) pass through unchanged.
type shadowrocketParams struct {
	Rules         []shadowrocketMapping `json:"rules"`
	UnknownAction string                `json:"unknownAction,omitempty"`
}

// DefaultShadowrocketMapping returns the curated default table that ships
// with the binary. The frontend uses the same list (mirrored in
// src/lib/schema.ts) to seed its editor when the user first picks the
// builtin in a transform; saving with an empty params keeps the defaults.
//
// Order matters: the list is intentionally grouped (passthroughs first,
// renames next, drops last) so the UI renders something self-explanatory.
func DefaultShadowrocketMapping() []shadowrocketMapping {
	return []shadowrocketMapping{
		// Compatible passthroughs.
		{Type: "DOMAIN", Action: ShadowrocketActionKeep},
		{Type: "DOMAIN-SUFFIX", Action: ShadowrocketActionKeep},
		{Type: "DOMAIN-KEYWORD", Action: ShadowrocketActionKeep},
		{Type: "IP-CIDR", Action: ShadowrocketActionKeep},
		{Type: "IP-CIDR6", Action: ShadowrocketActionKeep},
		{Type: "GEOIP", Action: ShadowrocketActionKeep},
		{Type: "SRC-IP-CIDR", Action: ShadowrocketActionKeep},
		{Type: "DST-PORT", Action: ShadowrocketActionKeep},
		{Type: "SRC-PORT", Action: ShadowrocketActionKeep},
		{Type: "IN-PORT", Action: ShadowrocketActionKeep},
		{Type: "PROTOCOL", Action: ShadowrocketActionKeep},
		{Type: "NETWORK", Action: ShadowrocketActionKeep},
		{Type: "USER-AGENT", Action: ShadowrocketActionKeep},
		{Type: "URL-REGEX", Action: ShadowrocketActionKeep},
		{Type: "FINAL", Action: ShadowrocketActionKeep},
		// Renames.
		{Type: "MATCH", Action: ShadowrocketActionRename, RenameTo: "FINAL", Reason: "Shadowrocket 用 FINAL 替代 MATCH"},
		// Drops with reasons.
		{Type: "PROCESS-NAME", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持 PROCESS-NAME"},
		{Type: "PROCESS-PATH", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持 PROCESS-PATH"},
		{Type: "IP-ASN", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持 IP-ASN"},
		{Type: "DOMAIN-REGEX", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 无 DOMAIN-REGEX 等价规则（URL-REGEX 语义不同）"},
		{Type: "RULE-SET", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持内联 RULE-SET 引用"},
		{Type: "SUB-RULE", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持 SUB-RULE"},
		{Type: "AND", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持逻辑组合规则 AND"},
		{Type: "OR", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持逻辑组合规则 OR"},
		{Type: "NOT", Action: ShadowrocketActionDrop, Reason: "Shadowrocket 不支持逻辑组合规则 NOT"},
	}
}

// decodeShadowrocketParams parses the user-supplied params. An empty /
// invalid blob falls back to the curated defaults so the dropdown
// continues to do something sensible even when the operator forgets to
// configure params. The errs from json.Unmarshal are swallowed because
// the schema layer already enforces validity; if it ever fails here it
// means a stale config was hand-edited, and silently falling back is
// preferable to crashing the preview/sync pipeline.
func decodeShadowrocketParams(raw json.RawMessage) shadowrocketParams {
	out := shadowrocketParams{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if len(out.Rules) == 0 {
		out.Rules = DefaultShadowrocketMapping()
	}
	if out.UnknownAction == "" {
		out.UnknownAction = ShadowrocketActionKeep
	}
	return out
}

// runMihomoToShadowrocket rewrites a mihomo classical rule list according
// to the resolved mapping table. Comments are always dropped (consistent
// with the zero-header policy) and unknown rule types fall back to
// params.UnknownAction (default: keep). Per-line decisions land in the
// returned report so the preview panel can show the operator exactly which
// rules were rewritten and why.
func runMihomoToShadowrocket(rawParams json.RawMessage, content string) BuiltinResult {
	params := decodeShadowrocketParams(rawParams)
	mapping := make(map[string]shadowrocketMapping, len(params.Rules))
	for _, r := range params.Rules {
		if r.Type == "" {
			continue
		}
		mapping[r.Type] = r
	}

	lines := splitClassicalLines(content)
	if len(lines) == 0 {
		return BuiltinResult{}
	}
	var (
		dropped       []DroppedLine
		droppedTotal  int
		modified      []ModifiedLine
		modifiedTotal int
		out           []string
	)
	for _, line := range lines {
		typ, _, isComment, isBlank := classifyClassicalLine(line.raw)
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
			rule = shadowrocketMapping{Type: typ, Action: params.UnknownAction, Reason: "未在映射表中配置"}
		}
		switch rule.Action {
		case ShadowrocketActionDrop:
			reason := rule.Reason
			if reason == "" {
				reason = "类型 " + typ + " 已被映射表标记为丢弃"
			}
			dropped = AppendDropped(dropped, &droppedTotal, DroppedLine{
				LineNo: line.lineNo,
				Text:   line.raw,
				Reason: reason,
			})
		case ShadowrocketActionRename:
			to := strings.TrimSpace(rule.RenameTo)
			if to == "" || to == typ {
				// Misconfigured rename row → fall back to keep so the line
				// doesn't silently vanish; surface a synthetic dropped
				// entry instead would be too aggressive for a typo.
				out = append(out, strings.TrimSpace(line.raw))
				continue
			}
			rewritten := renameLeadingToken(strings.TrimSpace(line.raw), typ, to)
			reason := rule.Reason
			if reason == "" {
				reason = typ + " 重命名为 " + to
			}
			modified = AppendModified(modified, &modifiedTotal, ModifiedLine{
				LineNo: line.lineNo,
				From:   strings.TrimSpace(line.raw),
				To:     rewritten,
				Reason: reason,
			})
			out = append(out, rewritten)
		case ShadowrocketActionKeep, "":
			out = append(out, strings.TrimSpace(line.raw))
		default:
			// Unknown action token → treat as keep so a stale enum value
			// from a future release doesn't silently delete user rules.
			out = append(out, strings.TrimSpace(line.raw))
		}
	}
	output := strings.Join(out, "\n")
	if output != "" {
		output += "\n"
	}
	return BuiltinResult{
		Output:        output,
		Dropped:       dropped,
		Modified:      modified,
		DroppedTotal:  droppedTotal,
		ModifiedTotal: modifiedTotal,
	}
}

// renameLeadingToken replaces the first comma-delimited token of line if it
// matches from. The rest of the line (policy, modifiers) is preserved
// verbatim. Whitespace inside the line stays put.
func renameLeadingToken(line, from, to string) string {
	idx := strings.IndexByte(line, ',')
	if idx < 0 {
		if strings.TrimSpace(line) == from {
			return to
		}
		return line
	}
	if strings.TrimSpace(line[:idx]) == from {
		return to + line[idx:]
	}
	return line
}
