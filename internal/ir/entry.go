// Package ir defines the client-agnostic intermediate representation for
// proxy routing rules, plus parsers (upstream formats -> entries) and
// renderers (entries -> client formats).
package ir

import (
	"sort"
	"strings"
)

// Kind identifies one matching dimension. Values are the union across the
// mihomo / Surge / Shadowrocket / sing-box dialects.
type Kind string

const (
	KindDomain         Kind = "domain"
	KindDomainSuffix   Kind = "domain_suffix"
	KindDomainKeyword  Kind = "domain_keyword"
	KindDomainWildcard Kind = "domain_wildcard"
	KindDomainRegex    Kind = "domain_regex"
	KindGeosite        Kind = "geosite"

	KindIPCIDR      Kind = "ip_cidr"
	KindIPSuffix    Kind = "ip_suffix"
	KindIPASN       Kind = "ip_asn"
	KindGeoIP       Kind = "geoip"
	KindSrcIPCIDR   Kind = "src_ip_cidr"
	KindSrcIPSuffix Kind = "src_ip_suffix"
	KindSrcIPASN    Kind = "src_ip_asn"
	KindSrcGeoIP    Kind = "src_geoip"

	KindDstPort Kind = "dst_port"
	KindSrcPort Kind = "src_port"
	KindInPort  Kind = "in_port"
	KindInType  Kind = "in_type"
	KindInUser  Kind = "in_user"
	KindInName  Kind = "in_name"

	KindProcessName         Kind = "process_name"
	KindProcessNameWildcard Kind = "process_name_wildcard"
	KindProcessNameRegex    Kind = "process_name_regex"
	KindProcessPath         Kind = "process_path"
	KindProcessPathWildcard Kind = "process_path_wildcard"
	KindProcessPathRegex    Kind = "process_path_regex"

	KindUID     Kind = "uid"
	KindNetwork Kind = "network"
	KindDSCP    Kind = "dscp"

	KindUserAgent Kind = "user_agent"
	KindURLRegex  Kind = "url_regex"
	KindProtocol  Kind = "protocol"

	KindSubnet        Kind = "subnet"
	KindCellularRadio Kind = "cellular_radio"
	KindDeviceName    Kind = "device_name"
	KindMacAddress    Kind = "mac_address"
	KindHostnameType  Kind = "hostname_type"
	KindScript        Kind = "script"

	// Logical composition kinds; matching data lives in Sub, Value is empty.
	KindAnd Kind = "and"
	KindOr  Kind = "or"
	KindNot Kind = "not"
)

// FlagNoResolve is the only flag preserved in IR; it is meaningful across
// mihomo / Surge / Shadowrocket for the destination-IP kinds.
const FlagNoResolve = "no-resolve"

// AllKinds lists every kind in stable display order.
var AllKinds = []Kind{
	KindDomain, KindDomainSuffix, KindDomainKeyword, KindDomainWildcard,
	KindDomainRegex, KindGeosite,
	KindIPCIDR, KindIPSuffix, KindIPASN, KindGeoIP,
	KindSrcIPCIDR, KindSrcIPSuffix, KindSrcIPASN, KindSrcGeoIP,
	KindDstPort, KindSrcPort, KindInPort, KindInType, KindInUser, KindInName,
	KindProcessName, KindProcessNameWildcard, KindProcessNameRegex,
	KindProcessPath, KindProcessPathWildcard, KindProcessPathRegex,
	KindUID, KindNetwork, KindDSCP,
	KindUserAgent, KindURLRegex, KindProtocol,
	KindSubnet, KindCellularRadio, KindDeviceName, KindMacAddress,
	KindHostnameType, KindScript,
	KindAnd, KindOr, KindNot,
}

var kindSet = func() map[Kind]bool {
	m := make(map[Kind]bool, len(AllKinds))
	for _, k := range AllKinds {
		m[k] = true
	}
	return m
}()

// IsValidKind reports whether k is a known kind.
func IsValidKind(k Kind) bool { return kindSet[k] }

// IsLogical reports whether k is one of the logical composition kinds.
func (k Kind) IsLogical() bool { return k == KindAnd || k == KindOr || k == KindNot }

// Entry is one IR rule entry. For logical kinds Value is empty and Sub holds
// the child entries (NOT has exactly one child).
type Entry struct {
	Kind  Kind     `json:"kind"`
	Value string   `json:"value,omitempty"`
	Flags []string `json:"flags,omitempty"`
	Sub   []Entry  `json:"sub,omitempty"`
}

// HasFlag reports whether the entry carries the given flag.
func (e Entry) HasFlag(flag string) bool {
	for _, f := range e.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// Key returns the canonical identity string used for dedupe / set ops / diff.
// Logical entries serialise recursively so structurally equal trees collide.
func (e Entry) Key() string {
	var b strings.Builder
	e.writeKey(&b)
	return b.String()
}

func (e Entry) writeKey(b *strings.Builder) {
	b.WriteString(string(e.Kind))
	b.WriteByte('\x1f')
	b.WriteString(e.Value)
	if len(e.Flags) > 0 {
		flags := append([]string(nil), e.Flags...)
		sort.Strings(flags)
		b.WriteByte('\x1f')
		b.WriteString(strings.Join(flags, ","))
	}
	if len(e.Sub) > 0 {
		b.WriteByte('(')
		for i, s := range e.Sub {
			if i > 0 {
				b.WriteByte('|')
			}
			s.writeKey(b)
		}
		b.WriteByte(')')
	}
}

// Display returns a compact human-readable one-line form used in previews,
// diffs and drop reports. It intentionally reads like a classical line.
func (e Entry) Display() string {
	var b strings.Builder
	e.writeDisplay(&b)
	return b.String()
}

func (e Entry) writeDisplay(b *strings.Builder) {
	if e.Kind.IsLogical() {
		b.WriteString(strings.ToUpper(string(e.Kind)))
		b.WriteString(",(")
		for i, s := range e.Sub {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('(')
			s.writeDisplay(b)
			b.WriteByte(')')
		}
		b.WriteByte(')')
		return
	}
	b.WriteString(string(e.Kind))
	b.WriteByte(',')
	b.WriteString(e.Value)
	for _, f := range e.Flags {
		b.WriteByte(',')
		b.WriteString(f)
	}
}

// Diagnostic records one input line (or JSON node) that could not be turned
// into an entry. Diagnostics are never silently discarded: they surface in
// preview reports and update events.
type Diagnostic struct {
	Line   int    `json:"line,omitempty"` // 1-based; 0 when not line-oriented
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// RuleSet is the result of parsing one source document.
type RuleSet struct {
	Entries     []Entry      `json:"entries"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// KindCount is one (kind, count) pair, ordered for stable JSON output.
type KindCount struct {
	Kind  Kind `json:"kind"`
	Count int  `json:"count"`
}

// CountKinds aggregates entry counts per kind, ordered by descending count
// then by AllKinds order for ties.
func CountKinds(entries []Entry) []KindCount {
	counts := map[Kind]int{}
	for _, e := range entries {
		counts[e.Kind]++
	}
	out := make([]KindCount, 0, len(counts))
	for _, k := range AllKinds {
		if c, ok := counts[k]; ok {
			out = append(out, KindCount{Kind: k, Count: c})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}
