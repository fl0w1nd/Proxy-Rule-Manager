package transformer

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestScriptRunner_TimeoutDoesNotPoisonPool runs alternating fast/slow
// scripts in sequence so the slow runs leave the pool in whatever state
// they happen to leave it; fast runs after must still succeed.
func TestScriptRunner_TimeoutDoesNotPoisonPool(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      30 * time.Millisecond,
		MaxOutputLen: 1 << 20,
	})

	fast := `function transform(c) { return c.toUpperCase(); }`
	slow := `function transform(c) { while (true) {} return c; }`

	for i := 0; i < 50; i++ {
		if _, err := r.Execute(slow, "x"); err == nil {
			t.Fatalf("iter %d: slow script should have errored on timeout", i)
		}
		out, err := r.Execute(fast, "hello")
		if err != nil || out != "HELLO" {
			t.Fatalf("iter %d: fast script after slow returned %q err=%v", i, out, err)
		}
	}
}

// TestScriptRunner_ConcurrentFastScriptsStable runs many parallel fast
// scripts to make sure the pool serves clean runtimes without poisoning
// from concurrent timer firings.
func TestScriptRunner_ConcurrentFastScriptsStable(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      time.Second,
		MaxOutputLen: 1 << 20,
	})
	fast := `function transform(c) { return c.toUpperCase(); }`
	var wg sync.WaitGroup
	errors := make(chan error, 500)
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := r.Execute(fast, "hello")
			if err != nil || out != "HELLO" {
				errors <- newErr("fast got=" + out + " err=" + errStr(err))
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

// TestScriptRunner_NoPanicUnderTimeoutStorm fires many slow scripts in
// parallel to ensure the timer-and-pool handshake never deadlocks or
// panics, even though each individual script will return a timeout error.
func TestScriptRunner_NoPanicUnderTimeoutStorm(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      20 * time.Millisecond,
		MaxOutputLen: 1 << 20,
	})
	slow := `function transform(c) { while (true) {} return c; }`
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Execute(slow, "x")
		}()
	}
	wg.Wait()
}

func errStr(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

func TestScriptRunner_LongOutputCapped(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      time.Second,
		MaxOutputLen: 100,
	})
	out, err := r.Execute(`function transform(c) { return c.repeat(10000); }`, "abc")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(out) != 100 {
		t.Fatalf("expected 100 byte cap, got %d", len(out))
	}
}

func TestScriptRunner_RecoversFromPriorTimeout(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      30 * time.Millisecond,
		MaxOutputLen: 1 << 20,
	})
	for i := 0; i < 50; i++ {
		_, _ = r.Execute(`function transform(c) { while (true) {} return c; }`, "x")
	}
	// Now a healthy script should run cleanly even though the pool may
	// have been pressured with interrupted runtimes.
	out, err := r.Execute(`function transform(c) { return c + "!"; }`, "ok")
	if err != nil {
		t.Fatalf("execute after timeout barrage: %v", err)
	}
	if !strings.HasSuffix(out, "!") {
		t.Fatalf("got %q after recovery", out)
	}
}

// TestScriptRunner_ModuleLevelContentAccess verifies that scripts can reference
// `content` at module level (outside any function body), matching the TS
// `new Function("content", script)` semantics.
func TestScriptRunner_ModuleLevelContentAccess(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      time.Second,
		MaxOutputLen: 1 << 20,
	})

	// content is used at module level to pre-compute lines, then transform
	// filters them. This would fail with the old wrapper that only put
	// content inside the transform() parameter scope.
	script := `
const lines = content.split("\n");
function transform(c) {
    return lines.filter(function(l) { return l !== "" && l[0] !== "#"; }).join("\n");
}`
	input := "# comment\nDOMAIN,example.com\n# another\nDOMAIN,test.com"
	out, err := r.Execute(script, input)
	if err != nil {
		t.Fatalf("module-level content access failed: %v", err)
	}
	if strings.Contains(out, "#") {
		t.Errorf("expected comments stripped, got: %q", out)
	}
	if !strings.Contains(out, "DOMAIN,example.com") {
		t.Errorf("expected DOMAIN lines preserved, got: %q", out)
	}
}

// TestScriptRunner_ConsoleLogDoesNotThrow verifies that calling all the
// supported console.* methods inside a script does not throw.
func TestScriptRunner_ConsoleLogDoesNotThrow(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      time.Second,
		MaxOutputLen: 1 << 20,
	})

	script := `
function transform(c) {
    console.log("log");
    console.error("error");
    console.warn("warn");
    console.info("info");
    console.debug("debug");
    console.trace("trace");
    console.group("group");
    console.groupEnd();
    return c;
}`
	out, err := r.Execute(script, "hello")
	if err != nil {
		t.Fatalf("console calls should not throw, got: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected passthrough, got: %q", out)
	}
}

// TestScriptRunner_Base64 covers btoa/atob round-trips and lenient decoding
// of URL-safe and unpadded variants (which browsers also tolerate).
func TestScriptRunner_Base64(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{Timeout: time.Second, MaxOutputLen: 1 << 20})
	script := `
function transform(c) {
    const enc = btoa("hello");                  // aGVsbG8=
    if (atob(enc) !== "hello") throw new Error("std roundtrip failed: " + atob(enc));
    if (atob("aGVsbG8") !== "hello") throw new Error("unpadded failed");
    if (atob("aGVsbG8=") !== "hello") throw new Error("padded failed");
    return c + ":" + enc;
}`
	out, err := r.Execute(script, "x")
	if err != nil {
		t.Fatalf("base64 helpers failed: %v", err)
	}
	if !strings.HasSuffix(out, ":aGVsbG8=") {
		t.Errorf("unexpected btoa output, got %q", out)
	}
}

// TestScriptRunner_URL exercises the URL / URLSearchParams globals provided
// by goja_nodejs/url so we know require + url.Enable wiring is alive.
func TestScriptRunner_URL(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{Timeout: time.Second, MaxOutputLen: 1 << 20})
	script := `
function transform(c) {
    const u = new URL("https://example.com/a/b?x=1&y=2#frag");
    if (u.host !== "example.com") throw new Error("host: " + u.host);
    if (u.pathname !== "/a/b") throw new Error("pathname: " + u.pathname);
    const sp = new URLSearchParams("a=1&b=2&a=3");
    const all = sp.getAll("a").join(",");
    if (all !== "1,3") throw new Error("getAll: " + all);
    return u.host + "?" + sp.toString();
}`
	out, err := r.Execute(script, "x")
	if err != nil {
		t.Fatalf("URL/URLSearchParams failed: %v", err)
	}
	if !strings.Contains(out, "example.com") {
		t.Errorf("unexpected URL test output: %q", out)
	}
}

// TestScriptRunner_TextCoders verifies the primary use case the user called
// out: computing UTF-8 byte length from a string, plus a decode round-trip.
func TestScriptRunner_TextCoders(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{Timeout: time.Second, MaxOutputLen: 1 << 20})
	script := `
function transform(c) {
    const enc = new TextEncoder();
    const bytes = enc.encode("héllo");          // 6 bytes in UTF-8
    if (bytes.length !== 6) throw new Error("byte length: " + bytes.length);
    const dec = new TextDecoder();
    if (dec.decode(bytes) !== "héllo") throw new Error("decode mismatch: " + dec.decode(bytes));
    return String(bytes.length);
}`
	out, err := r.Execute(script, "x")
	if err != nil {
		t.Fatalf("TextEncoder/Decoder failed: %v", err)
	}
	if out != "6" {
		t.Errorf("expected 6, got %q", out)
	}
}

// TestScriptRunner_Helpers covers each member of the `helpers` global plus
// the immutability guarantee — user scripts must not be able to swap or
// mutate it for the next pooled run.
func TestScriptRunner_Helpers(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{Timeout: time.Second, MaxOutputLen: 1 << 20})
	script := `
function transform(c) {
    const lines = helpers.splitLines("a\nb\r\na\n# x\n  // y\n; z\n\n");
    const filtered = lines.filter(function (l) { return l.length > 0 && !helpers.isComment(l); });
    const joined = helpers.joinLines(helpers.dedupe(filtered));
    return joined;
}`
	out, err := r.Execute(script, "x")
	if err != nil {
		t.Fatalf("helpers script failed: %v", err)
	}
	want := "a\nb"
	if out != want {
		t.Errorf("unexpected helpers output\n got: %q\nwant: %q", out, want)
	}

	// Try to mutate / replace; immutability should survive between runs
	// in the same pooled runtime.
	_, _ = r.Execute(`function transform(c) {
    try { helpers.dedupe = function () { return ["pwned"]; }; } catch (e) {}
    try { helpers = {}; } catch (e) {}
    return c;
}`, "x")
	out2, err := r.Execute(`function transform(c) {
    return helpers.dedupe([1,1,2,3,3]).join(",");
}`, "x")
	if err != nil {
		t.Fatalf("helpers post-mutation: %v", err)
	}
	if out2 != "1,2,3" {
		t.Errorf("helpers should be immutable, got: %q", out2)
	}
}

// TestScriptRunner_ModuleLevelContentCaching verifies that when the same
// module-level script runs twice (cache hit), both runs produce correct results.
func TestScriptRunner_ModuleLevelContentCaching(t *testing.T) {
	r := NewScriptRunner(ScriptOptions{
		Timeout:      time.Second,
		MaxOutputLen: 1 << 20,
	})

	script := `
const prefix = content.split(",")[0];
function transform(c) { return prefix + ":" + c; }`

	out1, err := r.Execute(script, "DOMAIN,a.com")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	out2, err := r.Execute(script, "IP,1.2.3.4")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	// Each call should use ITS OWN content, not the previous call's.
	if !strings.HasPrefix(out1, "DOMAIN:") {
		t.Errorf("call 1: expected DOMAIN: prefix, got %q", out1)
	}
	if !strings.HasPrefix(out2, "IP:") {
		t.Errorf("call 2: expected IP: prefix, got %q", out2)
	}
}

type strErr string

func (e strErr) Error() string { return string(e) }
func newErr(s string) error    { return strErr(s) }
