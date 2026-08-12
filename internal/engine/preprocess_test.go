package engine

import (
	"strings"
	"testing"
	"time"
)

func TestPreprocessNormal(t *testing.T) {
	r := NewPreprocessRunner()
	script := `function process(content) { return content.toUpperCase(); }`
	out, err := r.Run(script, "hello world")
	if err != nil || out != "HELLO WORLD" {
		t.Fatalf("Run: err=%v out=%q", err, out)
	}
}

func TestPreprocessNoProcessFunction(t *testing.T) {
	r := NewPreprocessRunner()
	_, err := r.Run("var x = 1;", "hello")
	if err == nil || !strings.Contains(err.Error(), "process") {
		t.Fatalf("expected process function error, got: %v", err)
	}
}

func TestPreprocessTimeout(t *testing.T) {
	r := NewPreprocessRunner()
	r.Configure(100*time.Millisecond, 1024*1024)
	script := `function process(content) { while(true){} return content; }`
	_, err := r.Run(script, "hello")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestPreprocessOutputTooLarge(t *testing.T) {
	r := NewPreprocessRunner()
	r.Configure(5*time.Second, 10)
	script := `function process(content) { return "xxxxxxxxxxxxxxxx"; }`
	_, err := r.Run(script, "hello")
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected too large error, got: %v", err)
	}
}

func TestPreprocessJSThrow(t *testing.T) {
	r := NewPreprocessRunner()
	script := `function process(content) { throw new Error("boom"); }`
	_, err := r.Run(script, "hello")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom error, got: %v", err)
	}
}

func TestPreprocessReturnsEmpty(t *testing.T) {
	r := NewPreprocessRunner()
	script := `function process(content) { return undefined; }`
	_, err := r.Run(script, "hello")
	if err == nil {
		t.Fatal("expected error for undefined return")
	}
}
