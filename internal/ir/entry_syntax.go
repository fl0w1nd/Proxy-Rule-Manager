package ir

import (
	"fmt"
	"strings"
)

// This file is the container-agnostic entry grammar: how a single rule entry
// is recognised, normalized, and validated, regardless of which document
// format (rule-line list, YAML payload, plain list) carried it.
// Container parsers in parse_*.go only unwrap their envelope and delegate
// entry-level work here.

// ruleTypeSpellings maps every accepted rule-type spelling (union across
// mihomo / Surge / Shadowrocket / Quantumult X) to its IR kind. Spellings are
// matched case-insensitively.
var ruleTypeSpellings = map[string]Kind{
	"DOMAIN":          KindDomain,
	"HOST":            KindDomain, // QX
	"DOMAIN-SUFFIX":   KindDomainSuffix,
	"HOST-SUFFIX":     KindDomainSuffix, // QX
	"DOMAIN-KEYWORD":  KindDomainKeyword,
	"HOST-KEYWORD":    KindDomainKeyword, // QX
	"DOMAIN-WILDCARD": KindDomainWildcard,
	"HOST-WILDCARD":   KindDomainWildcard, // QX
	"DOMAIN-REGEX":    KindDomainRegex,
	"GEOSITE":         KindGeosite,

	"IP-CIDR":   KindIPCIDR,
	"IP-CIDR6":  KindIPCIDR, // mihomo alias / Surge v6 spelling
	"IP6-CIDR":  KindIPCIDR, // QX
	"IP-SUFFIX": KindIPSuffix,
	"IP-ASN":    KindIPASN,
	"GEOIP":     KindGeoIP,

	"SRC-IP-CIDR":   KindSrcIPCIDR,
	"SRC-IP":        KindSrcIPCIDR, // Surge
	"SRC-IP-SUFFIX": KindSrcIPSuffix,
	"SRC-IP-ASN":    KindSrcIPASN,
	"SRC-GEOIP":     KindSrcGeoIP,

	"DST-PORT":  KindDstPort,
	"DEST-PORT": KindDstPort, // Surge spelling drift
	"SRC-PORT":  KindSrcPort,
	"IN-PORT":   KindInPort,
	"IN-TYPE":   KindInType,
	"IN-USER":   KindInUser,
	"IN-NAME":   KindInName,

	"PROCESS-NAME":          KindProcessName,
	"PROCESS-NAME-WILDCARD": KindProcessNameWildcard,
	"PROCESS-NAME-REGEX":    KindProcessNameRegex,
	"PROCESS-PATH":          KindProcessPath,
	"PROCESS-PATH-WILDCARD": KindProcessPathWildcard,
	"PROCESS-PATH-REGEX":    KindProcessPathRegex,

	"UID":     KindUID,
	"NETWORK": KindNetwork,
	"DSCP":    KindDSCP,

	"USER-AGENT": KindUserAgent,
	"URL-REGEX":  KindURLRegex,
	"PROTOCOL":   KindProtocol,

	"SUBNET":         KindSubnet,
	"CELLULAR-RADIO": KindCellularRadio,
	"DEVICE-NAME":    KindDeviceName,
	"MAC-ADDRESS":    KindMacAddress,
	"HOSTNAME-TYPE":  KindHostnameType,
	"SCRIPT":         KindScript,
}

// nonEntryTypes are recognised rule types that cannot become standalone IR
// entries (terminal rules, references, sub-rule jumps). They produce a
// diagnostic instead of silently vanishing.
var nonEntryTypes = map[string]string{
	"MATCH":            "terminal rule (MATCH) is not valid inside a rule set",
	"FINAL":            "terminal rule (FINAL) is not valid inside a rule set",
	"RULE-SET":         "nested RULE-SET references cannot be resolved",
	"SUB-RULE":         "SUB-RULE jumps are not representable in a rule set",
	"DOMAIN-SET":       "nested DOMAIN-SET references cannot be resolved",
	"CELLULAR-CARRIER": "CELLULAR-CARRIER is defunct (iOS 16.4+ blocks MCC/MNC)",
}

// commaPayloadKinds contains rule types whose payload may contain commas. The
// whole remainder is the value and no flags are parsed.
var commaPayloadKinds = map[Kind]bool{
	KindDomainRegex:      true,
	KindProcessNameRegex: true,
	KindProcessPathRegex: true,
	KindURLRegex:         true, // Surge regexes may contain commas too
}

// knownLineFlags is the union of per-line parameters seen across dialects.
// Only FlagNoResolve survives into IR; the rest are recognised so they are
// not mistaken for policies, then dropped.
var knownLineFlags = map[string]bool{
	"no-resolve":        true,
	"src":               true,
	"extended-matching": true,
	"pre-matching":      true,
	"dns-failed":        true,
	"requires-resolve":  true,
	"force-remote-dns":  true,
}

// isCommentLine reports whether a line is a comment in any dialect.
func isCommentLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//")
}

// parseRuleLine parses one `TYPE,VALUE[,flags...]` rule line, with or without
// a trailing policy.
func parseRuleLine(line string) (Entry, error) {
	comma := strings.Index(line, ",")
	if comma <= 0 {
		return Entry{}, fmt.Errorf("not a TYPE,VALUE rule line")
	}
	typeName := strings.ToUpper(strings.TrimSpace(line[:comma]))
	rest := strings.TrimSpace(line[comma+1:])

	if reason, bad := nonEntryTypes[typeName]; bad {
		return Entry{}, fmt.Errorf("%s", reason)
	}

	switch typeName {
	case "AND", "OR", "NOT":
		return parseLogicalRule(typeName, rest)
	}

	kind, ok := ruleTypeSpellings[typeName]
	if !ok {
		return Entry{}, fmt.Errorf("unknown rule type %q", typeName)
	}

	if commaPayloadKinds[kind] {
		return buildEntry(kind, rest, nil)
	}

	fields := strings.Split(rest, ",")
	value := strings.TrimSpace(fields[0])
	var flags []string
	srcFlag := false
	for _, f := range fields[1:] {
		f = strings.TrimSpace(f)
		lf := strings.ToLower(f)
		if knownLineFlags[lf] {
			switch lf {
			case "no-resolve":
				flags = append(flags, FlagNoResolve)
			case "src":
				srcFlag = true
			}
			continue
		}
		// Anything else is a policy name (full-config lines) or an unknown
		// parameter; both are irrelevant for rule-set extraction.
	}
	if srcFlag {
		if converted, ok := srcVariant(kind); ok {
			kind = converted
			flags = removeFlag(flags, FlagNoResolve)
		}
	}
	return buildEntry(kind, value, flags)
}

func srcVariant(k Kind) (Kind, bool) {
	switch k {
	case KindIPCIDR:
		return KindSrcIPCIDR, true
	case KindIPSuffix:
		return KindSrcIPSuffix, true
	case KindIPASN:
		return KindSrcIPASN, true
	case KindGeoIP:
		return KindSrcGeoIP, true
	}
	return k, false
}

func removeFlag(flags []string, flag string) []string {
	out := flags[:0]
	for _, f := range flags {
		if f != flag {
			out = append(out, f)
		}
	}
	return out
}

// buildEntry normalises the value per kind and validates it.
func buildEntry(kind Kind, value string, flags []string) (Entry, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Entry{}, fmt.Errorf("empty value for %s", kind)
	}
	switch kind {
	case KindDomain, KindDomainSuffix, KindDomainKeyword, KindDomainWildcard, KindGeosite:
		value = normalizeDomain(value)
	case KindIPCIDR, KindSrcIPCIDR:
		v, err := normalizeCIDR(value)
		if err != nil {
			return Entry{}, err
		}
		value = v
	case KindIPSuffix, KindSrcIPSuffix:
		// Suffix values look like CIDR but the mask means "low bits"; keep the
		// address/bits form verbatim after a light syntax check.
		if !strings.Contains(value, "/") {
			return Entry{}, fmt.Errorf("invalid IP-SUFFIX value %q (expected addr/bits)", value)
		}
	case KindIPASN, KindSrcIPASN:
		v, err := normalizeASN(value)
		if err != nil {
			return Entry{}, err
		}
		value = v
	case KindGeoIP, KindSrcGeoIP:
		value = normalizeGeoIP(value)
	case KindDstPort, KindSrcPort, KindInPort:
		v, err := normalizePorts(value)
		if err != nil {
			return Entry{}, err
		}
		value = v
	case KindNetwork:
		v, err := normalizeNetwork(value)
		if err != nil {
			return Entry{}, err
		}
		value = v
	case KindProtocol:
		value = strings.ToUpper(value)
	}
	return Entry{Kind: kind, Value: value, Flags: flags}, nil
}

// parseLogicalRule parses `AND,((r1),(r2))[,POLICY]` bodies. rest starts at
// the first character after "AND,".
func parseLogicalRule(typeName, rest string) (Entry, error) {
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "(") {
		return Entry{}, fmt.Errorf("logical rule %s missing parenthesised group", typeName)
	}
	group, _, err := matchParen(rest)
	if err != nil {
		return Entry{}, fmt.Errorf("logical rule %s: %v", typeName, err)
	}
	// Anything after the outer group is `,POLICY` in full-config lines; ignore.
	subs, err := splitTopLevelGroups(group)
	if err != nil {
		return Entry{}, fmt.Errorf("logical rule %s: %v", typeName, err)
	}
	if len(subs) == 0 {
		return Entry{}, fmt.Errorf("logical rule %s has no sub-rules", typeName)
	}
	kind := Kind(strings.ToLower(typeName))
	if kind == KindNot && len(subs) != 1 {
		return Entry{}, fmt.Errorf("NOT accepts exactly one sub-rule, got %d", len(subs))
	}
	entry := Entry{Kind: kind}
	for _, sub := range subs {
		child, err := parseRuleLine(strings.TrimSpace(sub))
		if err != nil {
			return Entry{}, fmt.Errorf("sub-rule %q: %v", sub, err)
		}
		entry.Sub = append(entry.Sub, child)
	}
	return entry, nil
}

// matchParen returns the content of the first balanced (...) group in s
// (which must start with '(') and the remainder after the closing paren.
func matchParen(s string) (inner, remainder string, err error) {
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], nil
			}
			if depth < 0 {
				return "", "", fmt.Errorf("unbalanced parentheses")
			}
		}
	}
	return "", "", fmt.Errorf("unbalanced parentheses")
}

// splitTopLevelGroups splits "(a),(b),(c)" into ["a","b","c"], tolerating
// spaces around commas (Surge's official examples include them).
func splitTopLevelGroups(s string) ([]string, error) {
	var out []string
	rest := strings.TrimSpace(s)
	for rest != "" {
		if !strings.HasPrefix(rest, "(") {
			return nil, fmt.Errorf("expected '(' at %q", rest)
		}
		inner, rem, err := matchParen(rest)
		if err != nil {
			return nil, err
		}
		out = append(out, inner)
		rem = strings.TrimSpace(rem)
		if rem == "" {
			break
		}
		if !strings.HasPrefix(rem, ",") {
			return nil, fmt.Errorf("expected ',' between groups at %q", rem)
		}
		rest = strings.TrimSpace(rem[1:])
	}
	return out, nil
}

// parsePlainItem classifies a single bare item (domain pattern or IP/CIDR),
// covering the plain-list dialects: `+.x`, `.x`, `*.x`, bare domains, CIDRs.
func parsePlainItem(item string) (Entry, error) {
	s := strings.TrimSpace(item)
	if s == "" {
		return Entry{}, fmt.Errorf("empty item")
	}
	// IP / CIDR first: they never contain wildcard characters.
	if looksLikeIP(s) {
		v, err := normalizeCIDR(s)
		if err != nil {
			return Entry{}, err
		}
		return Entry{Kind: KindIPCIDR, Value: v}, nil
	}
	switch {
	case strings.HasPrefix(s, "+."):
		// Clash `+.x`: any-level subdomain AND the apex itself == domain_suffix.
		v := normalizeDomain(s[2:])
		if v == "" {
			return Entry{}, fmt.Errorf("empty domain after +. prefix")
		}
		return Entry{Kind: KindDomainSuffix, Value: v}, nil
	case strings.HasPrefix(s, "."):
		// Surge domain-set `.x` (suffix incl. self) and Clash `.x` (subdomains
		// only) diverge; we take the suffix-inclusive reading, which is what
		// list authors almost always intend.
		v := normalizeDomain(s[1:])
		if v == "" {
			return Entry{}, fmt.Errorf("empty domain after leading dot")
		}
		return Entry{Kind: KindDomainSuffix, Value: v}, nil
	case strings.ContainsAny(s, "*?"):
		if s == "*" {
			return Entry{}, fmt.Errorf("bare * matches only dot-less hostnames; not representable")
		}
		return Entry{Kind: KindDomainWildcard, Value: normalizeDomain(s)}, nil
	default:
		v := normalizeDomain(s)
		if !isPlausibleDomain(v) {
			return Entry{}, fmt.Errorf("not a domain or IP: %q", item)
		}
		return Entry{Kind: KindDomain, Value: v}, nil
	}
}

// looksLikeIP is a cheap pre-check before the strict netip parse.
func looksLikeIP(s string) bool {
	base := s
	if i := strings.Index(s, "/"); i >= 0 {
		base = s[:i]
	}
	if strings.Contains(base, ":") {
		return true // only IPv6 among our inputs contains colons
	}
	// IPv4: digits and dots only.
	dots := 0
	for _, r := range base {
		switch {
		case r == '.':
			dots++
		case r < '0' || r > '9':
			return false
		}
	}
	return dots == 3
}

// isPlausibleDomain rejects obvious garbage without being a full RFC check.
func isPlausibleDomain(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' && r < 0x80 {
				return false
			}
		}
	}
	return true
}
