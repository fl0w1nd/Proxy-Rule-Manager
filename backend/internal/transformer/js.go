package transformer

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	jsurl "github.com/dop251/goja_nodejs/url"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// ScriptOptions tunes the runtime behavior.
type ScriptOptions struct {
	Timeout      time.Duration
	MaxOutputLen int
}

// DefaultScriptOptions matches the plan: 5s timeout, 8 MB cap.
func DefaultScriptOptions() ScriptOptions {
	return ScriptOptions{
		Timeout:      5 * time.Second,
		MaxOutputLen: 8 * 1024 * 1024,
	}
}

// ScriptRunner executes user transformer scripts in a sandboxed goja runtime.
//
// optsMu guards opts so that admin-driven Configure() calls are observable on
// the very next Execute without restarting the server. The compiled-program
// cache is unaffected because it keys on script SHA, not on options.
type ScriptRunner struct {
	optsMu      sync.RWMutex
	opts        ScriptOptions
	programs    sync.Map
	runtimePool sync.Pool
}

// NewScriptRunner constructs a script runner with the given options.
func NewScriptRunner(opts ScriptOptions) *ScriptRunner {
	opts = normaliseOpts(opts)
	return &ScriptRunner{
		opts: opts,
		runtimePool: sync.Pool{
			New: func() any {
				return buildRuntime()
			},
		},
	}
}

// buildRuntime constructs a goja runtime with the standard sandbox surface:
// a console shim, atob/btoa, URL/URLSearchParams, TextEncoder/TextDecoder,
// and a frozen `helpers` global with rule-set utilities. The runtime is
// reused across script executions via runtimePool, so all state set here
// must be idempotent or immutable per script run.
func buildRuntime() *goja.Runtime {
	rt := goja.New()

	// Required by goja_nodejs core modules (url, etc.) — needs a require
	// registry attached before module Enable() helpers can resolve "url".
	new(require.Registry).Enable(rt)
	jsurl.Enable(rt) // exposes URL + URLSearchParams as globals

	installConsole(rt)
	installBase64(rt)
	installTextCoders(rt)
	installHelpers(rt)

	return rt
}

// installConsole exposes console.{log,error,warn,info,debug,trace,group,groupEnd}.
// All methods route through the same logger; group/groupEnd accept arguments
// without indenting (we don't model a stack, just mirror the call so user
// scripts don't throw).
func installConsole(rt *goja.Runtime) {
	console := rt.NewObject()
	logFn := func(call goja.FunctionCall) goja.Value {
		var parts []string
		for _, arg := range call.Arguments {
			parts = append(parts, arg.String())
		}
		log.Printf("[js-transform] %s", strings.Join(parts, " "))
		return goja.Undefined()
	}
	for _, name := range []string{
		"log", "error", "warn", "info",
		"debug", "trace", "group", "groupEnd",
	} {
		_ = console.Set(name, logFn)
	}
	_ = rt.Set("console", console)
}

// installBase64 adds the WHATWG-style atob/btoa pair. We treat strings as
// raw byte sequences (latin-1) to match the browser semantics closely; this
// is how rule sets typically embed base64 fragments.
func installBase64(rt *goja.Runtime) {
	_ = rt.Set("btoa", func(s string) (string, error) {
		// btoa requires every char to be in the 0-0xFF range. Reject
		// out-of-range chars rather than silently mangling them.
		for _, r := range s {
			if r > 0xFF {
				return "", fmt.Errorf("btoa: argument contains non-Latin1 character U+%04X", r)
			}
		}
		buf := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			buf[i] = s[i]
		}
		return base64.StdEncoding.EncodeToString(buf), nil
	})
	_ = rt.Set("atob", func(s string) (string, error) {
		// Be liberal: accept both standard and URL-safe encodings,
		// padded or not, like browsers do.
		s = strings.TrimSpace(s)
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			if d, e2 := base64.RawStdEncoding.DecodeString(s); e2 == nil {
				decoded = d
			} else if d, e2 := base64.URLEncoding.DecodeString(s); e2 == nil {
				decoded = d
			} else if d, e2 := base64.RawURLEncoding.DecodeString(s); e2 == nil {
				decoded = d
			} else {
				return "", fmt.Errorf("atob: invalid base64 input: %w", err)
			}
		}
		return string(decoded), nil
	})
}

// installTextCoders provides minimal TextEncoder/TextDecoder for the most
// common use case: getting the UTF-8 byte length of a string and round-
// tripping bytes <-> strings. We return a Uint8Array when available, and
// fall back to a plain numeric array if the runtime lacks typed-array
// support (which goja does ship with by default — fallback is defensive).
func installTextCoders(rt *goja.Runtime) {
	encoderCtor := func(call goja.ConstructorCall) *goja.Object {
		obj := rt.NewObject()
		_ = obj.DefineDataProperty("encoding", rt.ToValue("utf-8"),
			goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
		_ = obj.Set("encode", func(input string) goja.Value {
			bytes := []byte(input)
			if u8 := rt.Get("Uint8Array"); u8 != nil {
				if ctor, ok := goja.AssertConstructor(u8); ok {
					ab := rt.NewArrayBuffer(bytes)
					if v, err := ctor(nil, rt.ToValue(ab)); err == nil {
						return v
					}
				}
			}
			// Fallback: plain JS array of byte values.
			out := make([]int, len(bytes))
			for i, b := range bytes {
				out[i] = int(b)
			}
			return rt.ToValue(out)
		})
		return obj
	}
	_ = rt.Set("TextEncoder", encoderCtor)

	decoderCtor := func(call goja.ConstructorCall) *goja.Object {
		obj := rt.NewObject()
		_ = obj.DefineDataProperty("encoding", rt.ToValue("utf-8"),
			goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
		_ = obj.Set("decode", func(input goja.Value) string {
			if input == nil || goja.IsUndefined(input) || goja.IsNull(input) {
				return ""
			}
			// ArrayBuffer fast-path.
			if ab, ok := input.Export().(goja.ArrayBuffer); ok {
				return string(ab.Bytes())
			}
			// Generic array-like (Uint8Array, plain array, ...).
			o := input.ToObject(rt)
			lenVal := o.Get("length")
			if lenVal == nil {
				return ""
			}
			n := int(lenVal.ToInteger())
			bytes := make([]byte, n)
			for i := 0; i < n; i++ {
				if v := o.Get(strconv.Itoa(i)); v != nil {
					bytes[i] = byte(v.ToInteger())
				}
			}
			return string(bytes)
		})
		return obj
	}
	_ = rt.Set("TextDecoder", decoderCtor)
}

// helpersScript installs a frozen `helpers` global with utilities frequently
// rewritten by rule-set authors. Implemented entirely in JS so we don't
// touch the Go-JS marshaling path on the hot loop.
const helpersScript = `(function () {
  const h = Object.freeze({
    dedupe: function (arr) { return Array.from(new Set(arr)); },
    splitLines: function (s) { return String(s).split(/\r?\n/); },
    joinLines: function (a) { return Array.from(a).join("\n"); },
    isComment: function (s) {
      const t = String(s).replace(/^[ \t]+/, "");
      return t.startsWith("#") || t.startsWith("//") || t.startsWith(";");
    },
  });
  return h;
})()`

func installHelpers(rt *goja.Runtime) {
	v, err := rt.RunString(helpersScript)
	if err != nil {
		log.Printf("[js-transform] failed to install helpers: %v", err)
		return
	}
	// Make `helpers` non-writable + non-configurable so user scripts
	// can't replace it for the lifetime of the pooled runtime.
	if err := rt.GlobalObject().DefineDataProperty(
		"helpers", v,
		goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE,
	); err != nil {
		log.Printf("[js-transform] failed to bind helpers global: %v", err)
	}
}

// normaliseOpts fills in zero values with the historical defaults.
func normaliseOpts(opts ScriptOptions) ScriptOptions {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.MaxOutputLen <= 0 {
		opts.MaxOutputLen = 8 * 1024 * 1024
	}
	return opts
}

// Configure swaps the runtime options atomically. The next Execute observes
// the new timeout and output cap; in-flight executions complete with the
// values they snapshotted at start.
func (r *ScriptRunner) Configure(opts ScriptOptions) {
	opts = normaliseOpts(opts)
	r.optsMu.Lock()
	r.opts = opts
	r.optsMu.Unlock()
}

func (r *ScriptRunner) currentOpts() ScriptOptions {
	r.optsMu.RLock()
	defer r.optsMu.RUnlock()
	return r.opts
}

// Execute wraps the script in `(function(content){ … ; return transform(content); })`
// and calls the resulting function with the given content, matching exactly the
// TS `new Function("content", script + "\nreturn transform(content);")` semantics.
// This means module-level code (e.g. `const lines = content.split("\n")`) has
// access to `content` as a parameter throughout the entire script body.
func (r *ScriptRunner) Execute(script, content string) (string, error) {
	if script == "" {
		return content, nil
	}
	hash := util.SHA256Hex(script)
	var program *goja.Program
	if v, ok := r.programs.Load(hash); ok {
		program = v.(*goja.Program)
	} else {
		// Wrap the entire script so `content` is a parameter available
		// at every level of the script body, including module-level code.
		// Running this program expression returns the wrapper function,
		// which we then call with the actual content value.
		wrapped := "(function(content){" + script + "\n;return transform(content);})"
		compiled, err := goja.Compile(hash, wrapped, true)
		if err != nil {
			return content, fmt.Errorf("compile script: %w", err)
		}
		program = compiled
		r.programs.Store(hash, program)
	}

	rt := r.runtimePool.Get().(*goja.Runtime)
	opts := r.currentOpts()
	// Race-safe timeout. The timer fires in its own goroutine and the
	// caller's defer races with it across three observable steps (read
	// finished, write timeoutFn, call Interrupt). The naive approach of
	// checking timeoutFn from the defer is wrong because the timer might
	// have observed finished=false, then we set finished=true, then we
	// read timeoutFn=false, then the timer sets timeoutFn=true and
	// finally calls Interrupt — poisoning the runtime AFTER we put it
	// back. We close `timerDone` from the timer goroutine to guarantee
	// the defer sees the timer's writes before deciding.
	var (
		finished  atomic.Bool
		timeoutFn atomic.Bool
		timerDone = make(chan struct{})
	)
	timer := time.AfterFunc(opts.Timeout, func() {
		defer close(timerDone)
		if finished.Load() {
			return
		}
		timeoutFn.Store(true)
		rt.Interrupt("transformer script timeout")
	})

	defer func() {
		finished.Store(true)
		if !timer.Stop() {
			// Timer fired (or is firing): wait for the goroutine to
			// publish its writes so the timeoutFn read below is
			// authoritative.
			<-timerDone
		}
		if timeoutFn.Load() {
			return // runtime was interrupted; let the pool re-create.
		}
		rt.ClearInterrupt()
		r.runtimePool.Put(rt)
	}()

	rt.ClearInterrupt()

	// Evaluate the wrapper expression — this returns the function object
	// without executing any user code yet. No globals are written.
	fnVal, err := rt.RunProgram(program)
	if err != nil {
		return content, fmt.Errorf("load program: %w", err)
	}

	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		return content, errors.New("transform() not defined")
	}

	value, err := fn(goja.Undefined(), rt.ToValue(content))
	if err != nil {
		return content, fmt.Errorf("run script: %w", err)
	}
	out := value.Export()
	str, ok := out.(string)
	if !ok {
		return content, nil
	}
	if opts.MaxOutputLen > 0 && len(str) > opts.MaxOutputLen {
		str = str[:opts.MaxOutputLen]
	}
	return str, nil
}
