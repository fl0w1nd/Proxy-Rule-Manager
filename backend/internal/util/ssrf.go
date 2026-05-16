package util

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// blockedHostnamePatterns mirrors src/lib/sync-engine/fetcher.ts.
//
// This list is the first line of defense (catches obvious junk before DNS
// resolution). The authoritative check happens in IsIPSafe / SafeDialContext
// against the resolved address, so DNS rebinding and "looks public, resolves
// private" hostnames are still blocked even if they pass this regex.
var blockedHostnamePatterns = func() []*regexp.Regexp {
	src := []string{
		`^127\.`,
		`^10\.`,
		`^172\.(1[6-9]|2[0-9]|3[0-1])\.`,
		`^192\.168\.`,
		`^169\.254\.`,
		`^0\.`,
		`^100\.(6[4-9]|[7-9][0-9]|1[0-1][0-9]|12[0-7])\.`,
		`^198\.18\.`,
		`^198\.51\.100\.`,
		`^203\.0\.113\.`,
		`^224\.`,
		`^240\.`,
		`^255\.`,
		`^\[.*\]$`,
		`^::1$`,
		`(?i)^fe80:`,
		`(?i)^fc00:`,
		`(?i)^fd[0-9a-f]{2}:`,
		`(?i)^localhost$`,
		`(?i)^localhost\.`,
		`(?i)\.localhost$`,
		`(?i)\.local$`,
		`(?i)\.internal$`,
		`^\d+$`,
		`(?i)^0x[0-9a-f]+$`,
	}
	out := make([]*regexp.Regexp, 0, len(src))
	for _, p := range src {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}()

// ErrUnsafeURL is returned by SafeDialContext / re-check helpers when the
// destination is rejected by the SSRF guard.
var ErrUnsafeURL = errors.New("unsafe URL")

// IsURLSafe reports whether the URL is allowed to be fetched: HTTPS scheme,
// hostname not on the deny-list. This is a cheap pre-flight check; callers
// MUST still use a SSRF-aware dialer so the resolved IPs are validated.
func IsURLSafe(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return false
	}
	for _, re := range blockedHostnamePatterns {
		if re.MatchString(hostname) {
			return false
		}
	}
	return true
}

// IsIPSafe returns true when the IP is a routable public address. It rejects
// loopback, private, link-local, multicast, unspecified, broadcast, CG-NAT
// (100.64/10), 4-in-6 representations of those, and other reserved ranges.
func IsIPSafe(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	// Normalize IPv4-in-IPv6 so checks below apply uniformly.
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsPrivate() {
		return false
	}
	if addr.Is4() {
		b := addr.As4()
		// 0.0.0.0/8 "this network".
		if b[0] == 0 {
			return false
		}
		// 100.64.0.0/10 CG-NAT.
		if b[0] == 100 && b[1] >= 64 && b[1] <= 127 {
			return false
		}
		// 198.18.0.0/15 benchmarking.
		if b[0] == 198 && (b[1] == 18 || b[1] == 19) {
			return false
		}
		// 192.0.0.0/24 IETF protocol assignments.
		if b[0] == 192 && b[1] == 0 && b[2] == 0 {
			return false
		}
		// Documentation ranges (TEST-NET-{1,2,3}).
		if (b[0] == 192 && b[1] == 0 && b[2] == 2) ||
			(b[0] == 198 && b[1] == 51 && b[2] == 100) ||
			(b[0] == 203 && b[1] == 0 && b[2] == 113) {
			return false
		}
		// 224.0.0.0/4 multicast, 240.0.0.0/4 reserved, 255.255.255.255.
		if b[0] >= 224 {
			return false
		}
	} else {
		// IPv6 documentation prefix 2001:db8::/32.
		b := addr.As16()
		if b[0] == 0x20 && b[1] == 0x01 && b[2] == 0x0d && b[3] == 0xb8 {
			return false
		}
	}
	return true
}

// SafeDialContext returns a DialContext function that resolves the requested
// host and rejects any candidate IP that fails IsIPSafe. It is the canonical
// way to plug SSRF protection into a net/http.Transport.
//
// Behaviour:
//   - The resolver is invoked for every dial (so DNS rebinding attempts are
//     re-checked).
//   - Every returned address must be safe; if any one is not, the dial fails.
//   - We dial each safe IP in order until one connects.
func SafeDialContext(resolver *net.Resolver, base *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	if base == nil {
		base = &net.Dialer{}
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// Literal IP: validate directly.
		if ip, err := netip.ParseAddr(host); err == nil {
			if !IsIPSafe(ip) {
				return nil, fmt.Errorf("%w: %s", ErrUnsafeURL, host)
			}
			return base.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		ips, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: no addresses for %s", ErrUnsafeURL, host)
		}
		// Refuse the dial if ANY resolved IP is unsafe. This is stricter
		// than "first safe wins" because a hostile resolver could return
		// a public IP first and a private one later for the same name.
		for _, ip := range ips {
			if !IsIPSafe(ip) {
				return nil, fmt.Errorf("%w: %s resolved to %s", ErrUnsafeURL, host, ip)
			}
		}
		var lastErr error
		for _, ip := range ips {
			conn, derr := base.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if derr == nil {
				return conn, nil
			}
			lastErr = derr
		}
		return nil, lastErr
	}
}
