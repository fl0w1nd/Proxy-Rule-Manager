package ir

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// normalizeDomain lowercases and trims a domain-ish value (mihomo lowercases
// domain payloads; matching is case-insensitive in every dialect).
func normalizeDomain(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

// normalizeCIDR canonicalises an IPv4/IPv6 CIDR. Bare addresses gain /32 or
// /128 (Surge Mac 6.0+ and sing-box both accept bare addresses upstream).
func normalizeCIDR(v string) (string, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return "", fmt.Errorf("empty CIDR")
	}
	if !strings.Contains(s, "/") {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return "", fmt.Errorf("invalid IP address %q", v)
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		return netip.PrefixFrom(addr.Unmap(), bits).String(), nil
	}
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %q", v)
	}
	return p.Masked().String(), nil
}

// normalizeASN strips an optional "AS" prefix and validates numeric form.
func normalizeASN(v string) (string, error) {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(strings.ToUpper(s), "AS")
	if s == "" {
		return "", fmt.Errorf("empty ASN")
	}
	if _, err := strconv.ParseUint(s, 10, 32); err != nil {
		return "", fmt.Errorf("invalid ASN %q", v)
	}
	return s, nil
}

// normalizeGeoIP uppercases a country code (conventional display form; all
// dialects match case-insensitively).
func normalizeGeoIP(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

// PortRange is one inclusive port span; Lo == Hi for a single port.
type PortRange struct {
	Lo int
	Hi int
}

// parsePorts accepts the union port syntax and returns canonical ranges:
//   - single: "80"
//   - dash range: "8000-9000" (mihomo/Surge)
//   - multi-segment: "114-514/810-1919" (mihomo)
//   - comparison: ">=50000", ">50000", "<=1024", "<1024" (Surge 5.8.4+)
func parsePorts(v string) ([]PortRange, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil, fmt.Errorf("empty port value")
	}
	segs := strings.Split(s, "/")
	if len(segs) > 28 {
		return nil, fmt.Errorf("too many port segments (max 28)")
	}
	out := make([]PortRange, 0, len(segs))
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			return nil, fmt.Errorf("empty port segment in %q", v)
		}
		r, err := parsePortSegment(seg)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parsePortSegment(seg string) (PortRange, error) {
	parseOne := func(s string) (int, error) {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 0 || n > 65535 {
			return 0, fmt.Errorf("invalid port %q", s)
		}
		return n, nil
	}
	switch {
	case strings.HasPrefix(seg, ">="):
		n, err := parseOne(seg[2:])
		if err != nil {
			return PortRange{}, err
		}
		return PortRange{Lo: n, Hi: 65535}, nil
	case strings.HasPrefix(seg, "<="):
		n, err := parseOne(seg[2:])
		if err != nil {
			return PortRange{}, err
		}
		return PortRange{Lo: 0, Hi: n}, nil
	case strings.HasPrefix(seg, ">"):
		n, err := parseOne(seg[1:])
		if err != nil || n >= 65535 {
			return PortRange{}, fmt.Errorf("invalid port segment %q", seg)
		}
		return PortRange{Lo: n + 1, Hi: 65535}, nil
	case strings.HasPrefix(seg, "<"):
		n, err := parseOne(seg[1:])
		if err != nil || n <= 0 {
			return PortRange{}, fmt.Errorf("invalid port segment %q", seg)
		}
		return PortRange{Lo: 0, Hi: n - 1}, nil
	case strings.Contains(seg, "-"):
		parts := strings.SplitN(seg, "-", 2)
		lo, err := parseOne(parts[0])
		if err != nil {
			return PortRange{}, err
		}
		hi, err := parseOne(parts[1])
		if err != nil {
			return PortRange{}, err
		}
		if lo > hi {
			return PortRange{}, fmt.Errorf("inverted port range %q", seg)
		}
		return PortRange{Lo: lo, Hi: hi}, nil
	default:
		n, err := parseOne(seg)
		if err != nil {
			return PortRange{}, err
		}
		return PortRange{Lo: n, Hi: n}, nil
	}
}

// formatPorts renders ranges back to the canonical IR value: segments joined
// by "/", each "N" or "N-M".
func formatPorts(ranges []PortRange) string {
	segs := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Lo == r.Hi {
			segs = append(segs, strconv.Itoa(r.Lo))
		} else {
			segs = append(segs, fmt.Sprintf("%d-%d", r.Lo, r.Hi))
		}
	}
	return strings.Join(segs, "/")
}

// normalizePorts parses then re-formats, giving the canonical value.
func normalizePorts(v string) (string, error) {
	ranges, err := parsePorts(v)
	if err != nil {
		return "", err
	}
	return formatPorts(ranges), nil
}

// normalizeNetwork validates tcp/udp (the only values mihomo/sing-box allow).
func normalizeNetwork(v string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(v))
	if s != "tcp" && s != "udp" {
		return "", fmt.Errorf("invalid network %q (must be tcp or udp)", v)
	}
	return s, nil
}
