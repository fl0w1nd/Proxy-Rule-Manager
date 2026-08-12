package util

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestIsURLSafe(t *testing.T) {
	bad := []string{
		"https://127.0.0.1/x",
		"https://10.0.0.1/x",
		"https://192.168.1.1/x",
		"https://169.254.1.1/x",
		"https://0.0.0.0/x",
		"https://100.64.0.1/x",
		"https://[::1]/x",
		"https://[fe80::1]/x",
		"https://[fc00::1]/x",
		"https://localhost/x",
		"https://localhost.localdomain/x",
		"https://example.local/x",
		"https://example.internal/x",
		"http://example.com/x", // non-https
		"ftp://example.com/x",  // non-https
		"https:///x",           // empty host
	}
	for _, u := range bad {
		if IsURLSafe(u) {
			t.Errorf("IsURLSafe(%q) = true, want false", u)
		}
	}
	good := []string{
		"https://example.com/x",
		"https://8.8.8.8/x",
		"https://1.1.1.1/x",
		"https://raw.githubusercontent.com/foo/bar",
	}
	for _, u := range good {
		if !IsURLSafe(u) {
			t.Errorf("IsURLSafe(%q) = false, want true", u)
		}
	}
}

func TestIsIPSafe(t *testing.T) {
	bad := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("192.168.1.1"),
		netip.MustParseAddr("172.16.0.1"),
		netip.MustParseAddr("169.254.1.1"),
		netip.MustParseAddr("0.0.0.0"),
		netip.MustParseAddr("100.64.0.1"),
		netip.MustParseAddr("224.0.0.1"),
		netip.MustParseAddr("::1"),
		netip.MustParseAddr("fe80::1"),
		netip.MustParseAddr("fc00::1"),
	}
	for _, ip := range bad {
		if IsIPSafe(ip) {
			t.Errorf("IsIPSafe(%s) = true, want false", ip)
		}
	}
	good := []netip.Addr{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2606:4700:4700::1111"),
	}
	for _, ip := range good {
		if !IsIPSafe(ip) {
			t.Errorf("IsIPSafe(%s) = false, want true", ip)
		}
	}
}

func TestSafeDialContextRejectsPrivateIP(t *testing.T) {
	dialCtx := SafeDialContext(nil, &net.Dialer{})
	_, err := dialCtx(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("expected error for private IP")
	}
	if !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("expected ErrUnsafeURL, got %v", err)
	}
}

func TestSafeDialContextRejectsCGNAT(t *testing.T) {
	dialCtx := SafeDialContext(nil, &net.Dialer{})
	_, err := dialCtx(context.Background(), "tcp", "100.64.0.1:80")
	if err == nil {
		t.Fatal("expected error for CGNAT IP")
	}
	if !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("expected ErrUnsafeURL, got %v", err)
	}
}
