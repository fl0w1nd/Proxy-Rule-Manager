package util

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestEnsureSafeSegment(t *testing.T) {
	good := []string{"clash_meta", "rule_a", "abc-123.list"}
	for _, s := range good {
		if err := EnsureSafeSegment(s, "x"); err != nil {
			t.Errorf("expected %q safe, got %v", s, err)
		}
	}
	bad := []string{"", ".", "..", "a/b", "a\\b", "/etc/passwd", "..\\evil"}
	for _, s := range bad {
		if err := EnsureSafeSegment(s, "x"); err == nil {
			t.Errorf("expected %q unsafe", s)
		}
	}
}

func TestJoinInside(t *testing.T) {
	root := t.TempDir()
	abs, err := JoinInside(root, "a", "b.txt")
	if err != nil {
		t.Fatalf("JoinInside: %v", err)
	}
	if abs != filepath.Join(root, "a", "b.txt") {
		t.Fatalf("unexpected join: %s", abs)
	}
	if _, err := JoinInside(root, "..", "escape"); err == nil {
		t.Fatalf("expected directory traversal to error")
	}
}

func TestAtomicWriteFileAndEnsureDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "out.txt")
	if err := EnsureDir(filepath.Dir(target)); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if err := AtomicWriteFile(target, []byte("hello")); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content: %q", data)
	}
}

func TestAtomicWriteFileConcurrentSameTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	const writers = 32
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			content := strings.Repeat(string(rune('a'+i%26)), 1024)
			errs <- AtomicWriteFile(target, []byte(content))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("AtomicWriteFile concurrent error: %v", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".out.txt.") {
			t.Fatalf("temp file leaked: %s", entry.Name())
		}
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read final target: %v", err)
	}
	if len(data) != 1024 {
		t.Fatalf("final content length = %d", len(data))
	}
}

func TestIsURLSafe(t *testing.T) {
	safe := []string{
		"https://raw.githubusercontent.com/foo/bar/rules.list",
		"https://example.com/list.txt",
	}
	for _, u := range safe {
		if !IsURLSafe(u) {
			t.Errorf("expected %q safe", u)
		}
	}
	unsafe := []string{
		"http://example.com/x",
		"https://127.0.0.1/x",
		"https://localhost/x",
		"https://10.0.0.1/x",
		"https://192.168.0.1/x",
		"https://[::1]/x",
		"https://api.internal/x",
		"https://abc.local/x",
		"https://169.254.0.1/x",
		"ftp://example.com/x",
		"https://2130706433/x",
	}
	for _, u := range unsafe {
		if IsURLSafe(u) {
			t.Errorf("expected %q unsafe", u)
		}
	}
}

// TestIsIPSafe covers the resolved-IP guard used by SafeDialContext. It is
// independent of DNS so we can test arbitrary IPs deterministically.
func TestIsIPSafe(t *testing.T) {
	unsafe := []string{
		"127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.0.1", "169.254.0.1",
		"0.0.0.0", "100.64.0.1", "198.18.0.1", "198.51.100.1", "203.0.113.5",
		"224.0.0.1", "255.255.255.255",
		"::1", "fe80::1", "fc00::1", "::",
		"2001:db8::1",
		"::ffff:127.0.0.1", // IPv4-mapped loopback
	}
	for _, raw := range unsafe {
		ip, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if IsIPSafe(ip) {
			t.Errorf("expected %s to be unsafe", raw)
		}
	}
	safe := []string{
		"8.8.8.8", "1.1.1.1", "13.225.0.1",
		"2606:4700::1",
	}
	for _, raw := range safe {
		ip, _ := netip.ParseAddr(raw)
		if !IsIPSafe(ip) {
			t.Errorf("expected %s to be safe", raw)
		}
	}
}

func TestClientIP(t *testing.T) {
	got := ClientIP(func(name string) string {
		if name == "x-forwarded-for" {
			return "203.0.113.5, 10.0.0.1"
		}
		return ""
	})
	if got != "203.0.113.5" {
		t.Fatalf("expected first X-Forwarded-For entry, got %q", got)
	}
	got = ClientIP(func(name string) string {
		if name == "x-real-ip" {
			return "198.51.100.7"
		}
		return ""
	})
	if got != "198.51.100.7" {
		t.Fatalf("expected X-Real-IP fallback, got %q", got)
	}
	got = ClientIP(func(name string) string {
		if name == "cf-connecting-ip" {
			return "192.0.2.42"
		}
		return ""
	})
	if got != "192.0.2.42" {
		t.Fatalf("expected CF-Connecting-IP fallback, got %q", got)
	}
	if ClientIP(func(string) string { return "" }) != "unknown" {
		t.Fatalf("expected 'unknown' fallback")
	}
}

func TestSHA256Hex(t *testing.T) {
	h := SHA256Hex("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if h != want {
		t.Fatalf("SHA256Hex hello: got %s want %s", h, want)
	}
}
