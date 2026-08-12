package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
)

func TestGeositeRemovalWarnings(t *testing.T) {
	previous := &geosite.ProviderCache{
		Catalog: []string{"google", "removed"},
		Entries: map[string][]geosite.Entry{
			"google":  {{Type: geosite.EntryDomain, Value: "example.com", Attrs: []string{"cn", "ads"}}},
			"removed": {{Type: geosite.EntryDomain, Value: "removed.example"}},
		},
	}
	current := &geosite.ProviderCache{
		Catalog: []string{"google", "added"},
		Entries: map[string][]geosite.Entry{
			"google": {{Type: geosite.EntryDomain, Value: "example.com", Attrs: []string{"cn"}}},
			"added":  {{Type: geosite.EntryDomain, Value: "added.example"}},
		},
	}
	warnings := geositeRemovalWarnings("v2fly", previous, current)
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"v2fly/google@ads", "v2fly/removed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings missing %q: %v", want, warnings)
		}
	}
	if strings.Contains(joined, "added") {
		t.Fatalf("new list reported as warning: %v", warnings)
	}
}

func TestGeositeFetchResult(t *testing.T) {
	old := &geosite.ProviderCache{ResolvedVersion: "v1"}
	for _, tt := range []struct {
		name    string
		current *geosite.ProviderCache
		failed  bool
		want    string
	}{
		{name: "updated", current: &geosite.ProviderCache{ResolvedVersion: "v2"}, want: state.GeositeUpdated},
		{name: "unchanged", current: &geosite.ProviderCache{ResolvedVersion: "v1"}, want: state.GeositeUnchanged},
		{name: "failed", current: old, failed: true, want: state.GeositeFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := geositeFetchResult(old, tt.current, tt.failed); got != tt.want {
				t.Fatalf("geositeFetchResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeositeTransientFailureEmitsRetryProgress(t *testing.T) {
	attempts := 0
	manager := geosite.NewManager(t.TempDir())
	manager.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("TLS handshake timeout")
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
		}, nil
	})})
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "geo", Name: "Geo", Sources: []config.SourceConfig{{Geosite: "v2fly/google"}}, Outputs: []string{"surge"},
		}},
		Update: config.UpdateConfig{Fetch: config.FetchConfig{Retries: 2, RetryDelay: config.Duration(time.Millisecond)}},
	}
	eng := newTestUpdateEngine(t, cfg)
	eng.Geosite = manager
	var events []ProgressEvent
	ctx := WithProgressReporter(context.Background(), func(event ProgressEvent) { events = append(events, event) })
	result := eng.FullUpdate(ctx)
	if attempts != 2 || len(result.Errors) == 0 || len(result.Warnings) != 0 {
		t.Fatalf("attempts=%d errors=%v warnings=%v", attempts, result.Errors, result.Warnings)
	}
	var retryEvent bool
	for _, event := range events {
		if event.Stage == "geosite_refresh" && event.Status == "retrying" && event.Current == 1 && event.Total == 2 && strings.Contains(event.Message, "TLS handshake timeout") {
			retryEvent = true
		}
	}
	if !retryEvent {
		t.Fatalf("retry progress missing: %+v", events)
	}
}

func TestProgressEventsStayAtRuleLevel(t *testing.T) {
	eng := newTestUpdateEngine(t, &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "Rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"},
		}},
	})
	var events []ProgressEvent
	ctx := WithProgressReporter(context.Background(), func(event ProgressEvent) { events = append(events, event) })
	result := eng.FullUpdate(ctx)
	if len(result.Errors) != 0 {
		t.Fatalf("update errors: %v", result.Errors)
	}
	var sawRunning, sawFinished bool
	for _, event := range events {
		if event.RuleID == "rule" && event.Status == "running" {
			sawRunning = true
		}
		if event.RuleID == "rule" && (event.Status == "updated" || event.Status == "unchanged") {
			sawFinished = true
		}
	}
	if !sawRunning || !sawFinished {
		t.Fatalf("rule progress events = %+v", events)
	}
}

func TestFailedRuleReturnsStructuredIssue(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "broken", Name: "Broken", Sources: []config.SourceConfig{{File: filepath.Join(t.TempDir(), "missing.list")}}, Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	result := eng.FullUpdate(context.Background())
	if len(result.Errors) == 0 {
		t.Fatal("failed update returned no errors")
	}
	if len(result.Issues) != 1 || result.Issues[0].Stage != "rule" || result.Issues[0].Subject != "broken" || !strings.Contains(result.Issues[0].Message, "missing.list") {
		t.Fatalf("issues = %+v", result.Issues)
	}
}

func TestCancelledUpdateReturnsCancellationResult(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "Rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := eng.FullUpdate(ctx)
	if len(result.Errors) != 1 || result.Errors[0] != "update cancelled" {
		t.Fatalf("cancelled result = %+v", result)
	}
	if result.RulesFailed != 0 || result.RulesSucceeded != 0 {
		t.Fatalf("cancelled counts = %+v", result)
	}
}

func TestFullUpdateWritesReferencesAndDetectsUnchangedArtifacts(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{
				ID:      "base",
				Name:    "Base Rule",
				Sources: []config.SourceConfig{{Content: "DOMAIN-SUFFIX,Example.COM\nDOMAIN-SUFFIX,example.com\nIP-CIDR,10.1.2.3/8,no-resolve"}},
				Outputs: []string{"surge"},
			},
			{
				ID:      "derived",
				Name:    "Derived Rule",
				Sources: []config.SourceConfig{{Ref: "base"}, {Content: "+.Extra.COM"}},
				Outputs: []string{"surge"},
			},
		},
	}
	eng := newTestUpdateEngine(t, cfg)

	first := eng.FullUpdate(context.Background())
	if first.RulesSucceeded != 2 || first.RulesFailed != 0 || first.Artifacts != 2 || len(first.ChangedRules) != 2 || len(first.Errors) != 0 {
		t.Fatalf("first update: %+v", first)
	}
	if len(first.Changes) != 2 || first.Changes[0].Added == 0 || len(first.Changes[0].Files) != 1 {
		t.Fatalf("first update changes: %+v", first.Changes)
	}
	assertRuleResult(t, eng, "base", state.RuleUpdated)
	assertRuleResult(t, eng, "derived", state.RuleUpdated)
	_, _, firstVersion, ok := eng.State.RuleUpdate("base")
	if !ok || firstVersion.IsZero() {
		t.Fatal("first content version was not recorded")
	}
	base := readArtifact(t, dataDir, "surge", "base.list")
	if strings.Count(base, "DOMAIN-SUFFIX,example.com") != 1 || !strings.Contains(base, "IP-CIDR,10.0.0.0/8,no-resolve") {
		t.Fatalf("base artifact:\n%s", base)
	}
	derived := readArtifact(t, dataDir, "surge", "derived.list")
	if !strings.Contains(derived, "DOMAIN-SUFFIX,example.com") || !strings.Contains(derived, "DOMAIN-SUFFIX,extra.com") {
		t.Fatalf("derived artifact:\n%s", derived)
	}

	second := eng.FullUpdate(context.Background())
	if second.RulesSucceeded != 2 || second.RulesFailed != 0 || second.Artifacts != 2 || len(second.ChangedRules) != 0 || len(second.Errors) != 0 {
		t.Fatalf("second update: %+v", second)
	}
	assertRuleResult(t, eng, "base", state.RuleUnchanged)
	assertRuleResult(t, eng, "derived", state.RuleUnchanged)
	_, _, unchangedVersion, _ := eng.State.RuleUpdate("base")
	if !unchangedVersion.Equal(firstVersion) {
		t.Fatalf("unchanged check advanced version: %v -> %v", firstVersion, unchangedVersion)
	}
	if lastCheck, ok := eng.State.LastCheck(); !ok || lastCheck.IsZero() {
		t.Fatalf("last check was not persisted: %v, %t", lastCheck, ok)
	}

	basePath := filepath.Join(dataDir, "rules", "surge", "base.list")
	if err := os.Remove(basePath); err != nil {
		t.Fatal(err)
	}
	third := eng.FullUpdate(context.Background())
	if len(third.Errors) != 0 || len(third.ChangedRules) != 0 {
		t.Fatalf("missing artifact recovery: %+v", third)
	}
	if len(third.Changes) != 1 || len(third.Changes[0].Files) != 1 || third.Changes[0].Files[0].Change != "updated" || third.Changes[0].Added != 0 || third.Changes[0].Removed != 0 {
		t.Fatalf("missing artifact recovery change: %+v", third.Changes)
	}
	readArtifact(t, dataDir, "surge", "base.list")
	_, _, recoveredVersion, _ := eng.State.RuleUpdate("base")
	if !recoveredVersion.Equal(firstVersion) {
		t.Fatalf("same-content recovery advanced version: %v -> %v", firstVersion, recoveredVersion)
	}
}

func TestFailedCheckPreservesContentVersion(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID:      "rule",
			Name:    "rule",
			Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}},
			Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	if result := eng.FullUpdate(context.Background()); len(result.Errors) != 0 {
		t.Fatalf("initial update: %+v", result)
	}
	_, _, versionBefore, ok := eng.State.RuleUpdate("rule")
	if !ok || versionBefore.IsZero() {
		t.Fatal("initial version is missing")
	}

	cfg.Rules[0].Sources[0].Content = "IP-CIDR,999.1.1.1/33"
	if result := eng.FullUpdate(context.Background()); result.RulesFailed != 1 {
		t.Fatalf("failed check: %+v", result)
	}
	result, _, versionAfter, ok := eng.State.RuleUpdate("rule")
	if !ok || result != state.RuleFailed || !versionAfter.Equal(versionBefore) {
		t.Fatalf("failed check changed version: result=%q before=%v after=%v", result, versionBefore, versionAfter)
	}
}

func TestSuccessfulCheckInitializesMissingVersionFromExistingArtifact(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID:      "rule",
			Name:    "rule",
			Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}},
			Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	content := "DOMAIN,example.com\n"
	path := filepath.Join(dataDir, "rules", "surge", "rule.list")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	eng.State.SetArtifactHash("rule", "surge", util.SHA256Hex(content))

	result := eng.FullUpdate(context.Background())
	if len(result.Errors) != 0 || len(result.ChangedRules) != 0 {
		t.Fatalf("baseline check: %+v", result)
	}
	checkResult, _, versionAt, ok := eng.State.RuleUpdate("rule")
	if !ok || checkResult != state.RuleUnchanged || versionAt.IsZero() {
		t.Fatalf("baseline state: result=%q version=%v ok=%t", checkResult, versionAt, ok)
	}
}

func TestFullUpdateRemovesArtifactsAndStateForDeletedRules(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID:      "obsolete",
			Name:    "obsolete",
			Sources: []config.SourceConfig{{Content: "DOMAIN,obsolete.example"}},
			Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	if result := eng.FullUpdate(context.Background()); len(result.Errors) != 0 {
		t.Fatalf("initial update: %+v", result)
	}
	artifactPath := filepath.Join(dataDir, "rules", "surge", "obsolete.list")
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatal(err)
	}

	cfg.Rules = nil
	if result := eng.FullUpdate(context.Background()); len(result.Errors) != 0 {
		t.Fatalf("reconciliation update: %+v", result)
	}
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("obsolete artifact remains: %v", err)
	}
	if hash := eng.State.GetArtifactHash("obsolete", "surge"); hash != "" {
		t.Fatalf("obsolete hash remains: %q", hash)
	}
	if _, exists, err := eng.State.LoadSnapshotIfExists("obsolete"); err != nil || exists {
		t.Fatalf("obsolete snapshot remains: exists=%t err=%v", exists, err)
	}
}

func TestFullUpdateLocalizesParseFailureAndBlocksFailedReferences(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{
				ID:      "healthy",
				Name:    "healthy",
				Sources: []config.SourceConfig{{Content: "DOMAIN,healthy.example"}},
				Outputs: []string{"surge"},
			},
			{
				ID:      "broken",
				Name:    "broken",
				Sources: []config.SourceConfig{{Label: "upstream", Content: "DOMAIN,partial.example\nIP-CIDR,999.1.1.1/33"}},
				Outputs: []string{"surge"},
			},
			{
				ID:      "dependent",
				Name:    "dependent",
				Sources: []config.SourceConfig{{Ref: "broken"}},
				Outputs: []string{"surge"},
			},
			{
				ID:      "transitive",
				Name:    "transitive",
				Sources: []config.SourceConfig{{Ref: "dependent"}},
				Outputs: []string{"surge"},
			},
		},
	}
	eng := newTestUpdateEngine(t, cfg)

	result := eng.FullUpdate(context.Background())
	if result.RulesSucceeded != 1 || result.RulesFailed != 3 || result.Artifacts != 1 {
		t.Fatalf("update counts: %+v", result)
	}
	errors := strings.Join(result.Errors, "\n")
	if !strings.Contains(errors, `rule "broken" (broken) source "upstream" line 2`) ||
		!strings.Contains(errors, `rule "dependent" (dependent) source "source[0]": ref "broken" not available`) ||
		!strings.Contains(errors, `rule "transitive" (transitive) source "source[0]": ref "dependent" not available`) {
		t.Fatalf("localized errors:\n%s", errors)
	}
	readArtifact(t, dataDir, "surge", "healthy.list")
	for _, name := range []string{"broken.list", "dependent.list", "transitive.list"} {
		if _, err := os.Stat(filepath.Join(dataDir, "rules", "surge", name)); !os.IsNotExist(err) {
			t.Fatalf("failed rule artifact %s exists or stat returned %v", name, err)
		}
	}
}

func TestPartialUpdateLoadsDependencySnapshot(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{
				ID:      "base",
				Name:    "base",
				Sources: []config.SourceConfig{{Content: "DOMAIN,base.example"}},
				Outputs: []string{"surge"},
			},
			{
				ID:      "derived",
				Name:    "derived",
				Sources: []config.SourceConfig{{Ref: "base"}, {Content: "DOMAIN,derived.example"}},
				Outputs: []string{"surge"},
			},
		},
	}
	eng := newTestUpdateEngine(t, cfg)
	if result := eng.FullUpdate(context.Background()); len(result.Errors) != 0 {
		t.Fatalf("initial full update: %+v", result)
	}
	baseResult, baseCheckedAt, baseVersionAt, ok := eng.State.RuleUpdate("base")
	if !ok {
		t.Fatal("base update record is missing")
	}

	// A partial update of derived excludes base from compilation and reads its
	// last successful IR snapshot for the ref source.
	result := eng.PartialUpdate(context.Background(), []string{"derived"})
	if result.RulesTotal != 1 || result.RulesSucceeded != 1 || result.RulesFailed != 0 || len(result.Errors) != 0 {
		t.Fatalf("partial update: %+v", result)
	}
	resultAfter, checkedAtAfter, versionAtAfter, ok := eng.State.RuleUpdate("base")
	if !ok || resultAfter != baseResult || !checkedAtAfter.Equal(baseCheckedAt) || !versionAtAfter.Equal(baseVersionAt) {
		t.Fatalf("base update record changed: %q checked=%v version=%v", resultAfter, checkedAtAfter, versionAtAfter)
	}
	assertRuleResult(t, eng, "derived", state.RuleUnchanged)
	artifact := readArtifact(t, dataDir, "surge", "derived.list")
	if !strings.Contains(artifact, "DOMAIN,base.example") || !strings.Contains(artifact, "DOMAIN,derived.example") {
		t.Fatalf("derived artifact:\n%s", artifact)
	}
}

func TestPartialUpdateExpandsDependentsInExecutionOrder(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "base", Name: "base", Sources: []config.SourceConfig{{Content: "DOMAIN,base.example"}}, Outputs: []string{"surge"}},
			{ID: "child", Name: "child", Sources: []config.SourceConfig{{Ref: "base"}}, Outputs: []string{"surge"}},
			{ID: "grandchild", Name: "grandchild", Sources: []config.SourceConfig{{Ref: "child"}}, Outputs: []string{"surge"}},
		},
	}
	eng := newTestUpdateEngine(t, cfg)
	result := eng.PartialUpdate(context.Background(), []string{"base"})
	if !reflect.DeepEqual(result.EffectiveRuleIDs, []string{"base", "child", "grandchild"}) || result.RulesSucceeded != 3 {
		t.Fatalf("partial result = %+v", result)
	}
}

func TestPartialGeositeUsesCacheAndPreservesProviderState(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules:   []config.RuleConfig{{ID: "geo", Name: "geo", Sources: []config.SourceConfig{{Geosite: "v2fly/google"}}, Outputs: []string{"surge"}}},
		Geosite: &config.GeositeConfig{Providers: []config.GeositeProvider{{Name: "v2fly", Clients: []string{"surge"}}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	cacheDir := filepath.Join(dataDir, "geosite")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cache := &geosite.ProviderCache{Provider: "v2fly", ResolvedVersion: "cached-v1", FetchedAt: time.Now().Format(time.RFC3339), Catalog: []string{"google"}, Entries: map[string][]geosite.Entry{"google": {{Type: geosite.EntryDomain, Value: "google.example"}}}}
	payload, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "v2fly.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := geosite.NewManager(cacheDir)
	requests := 0
	manager.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("network should stay idle")
	})})
	eng.Geosite = manager
	checkedAt := time.Now().Add(-time.Hour).UTC()
	eng.State.SetGeositeUpdate("v2fly", state.GeositeUpdated, checkedAt)
	result := eng.PartialUpdate(context.Background(), []string{"geo"})
	if len(result.Errors) != 0 || result.RulesSucceeded != 1 || requests != 0 {
		t.Fatalf("result=%+v requests=%d", result, requests)
	}
	providerResult, providerCheckedAt, ok := eng.State.GeositeUpdate("v2fly")
	if !ok || providerResult != state.GeositeUpdated || !providerCheckedAt.Equal(checkedAt.Truncate(time.Millisecond)) {
		t.Fatalf("provider state=%q %v %t", providerResult, providerCheckedAt, ok)
	}
	if content := readArtifact(t, dataDir, "surge", "geo.list"); !strings.Contains(content, "DOMAIN-SUFFIX,google.example") {
		t.Fatalf("artifact=%s", content)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "rules", "surge", "geosite", "v2fly", "google.list")); !os.IsNotExist(err) {
		t.Fatalf("partial update published geosite catalog: %v", err)
	}
}

func TestGeositePublicationWritesFullListAndVariants(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Geosite: &config.GeositeConfig{Providers: []config.GeositeProvider{{Name: "v2fly", Clients: []string{"surge"}}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	cache := &geosite.ProviderCache{Provider: "v2fly", Catalog: []string{"google"}, Entries: map[string][]geosite.Entry{"google": {
		{Type: geosite.EntryDomain, Value: "google.example", Attrs: []string{"cn"}},
		{Type: geosite.EntryFull, Value: "full.example"},
	}}}
	result := UpdateResult{}
	expected := make(map[string]struct{})
	stats := newGeositeStats()
	eng.updateGeositePublications(context.Background(), map[string]*geosite.ProviderCache{"v2fly": cache}, &result, expected, stats)
	if len(result.Errors) != 0 || result.Artifacts != 2 || len(expected) != 2 {
		t.Fatalf("result=%+v expected=%v", result, expected)
	}
	for _, name := range []string{"google.list", "google@cn.list"} {
		path := filepath.Join(cfg.DataDir, "rules", "surge", "geosite", "v2fly", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("published %s: %v", name, err)
		}
	}
}

func TestPartialGeositeMissingCacheCreatesRuleIssue(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules:   []config.RuleConfig{{ID: "geo", Name: "geo", Sources: []config.SourceConfig{{Geosite: "v2fly/google"}}, Outputs: []string{"surge"}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	eng.Geosite = geosite.NewManager(filepath.Join(cfg.DataDir, "geosite"))
	result := eng.PartialUpdate(context.Background(), []string{"geo"})
	if result.RulesFailed != 1 || len(result.Issues) != 1 || result.Issues[0].Subject != "geo" || !strings.Contains(result.Issues[0].Message, `geosite provider "v2fly" not loaded`) {
		t.Fatalf("result=%+v", result)
	}
}

func TestRuleChangeCapturesMultipleClientsAndCapsSamples(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("DOMAIN,item-%02d.example", i))
	}
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}, {ID: "shadowrocket", Template: "shadowrocket"}},
		Rules:   []config.RuleConfig{{ID: "many", Name: "Many", Sources: []config.SourceConfig{{Content: strings.Join(lines, "\n")}}, Outputs: []string{"surge", "shadowrocket"}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	result := eng.FullUpdate(context.Background())
	if len(result.Changes) != 1 || len(result.Changes[0].Files) != 2 || result.Changes[0].Added != 20 || len(result.Changes[0].AddedSamples) != 15 {
		t.Fatalf("changes=%+v", result.Changes)
	}
}

func TestRuleChangeRecordsPureRenderingChange(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(), Clients: []config.ClientConfig{{ID: "client", Template: "surge"}},
		Rules: []config.RuleConfig{{ID: "ports", Name: "Ports", Sources: []config.SourceConfig{{Content: "DEST-PORT,443"}}, Outputs: []string{"client"}}},
	}
	eng := newTestUpdateEngine(t, cfg)
	if result := eng.FullUpdate(context.Background()); len(result.Errors) != 0 {
		t.Fatalf("initial=%+v", result)
	}
	cfg.Clients[0].Template = "shadowrocket"
	result := eng.FullUpdate(context.Background())
	if len(result.Changes) != 1 || result.Changes[0].Added != 0 || result.Changes[0].Removed != 0 || len(result.Changes[0].Files) != 1 {
		t.Fatalf("format change=%+v", result.Changes)
	}
}

func TestFullUpdateWritesExplicitFormatsAndVariantDirectories(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{
			{
				ID: "mihomo", Formats: []config.ClientFormatConfig{
					{ID: "mihomo-classical", Name: "Classical", Template: "mihomo-classical"},
					{ID: "mihomo-yaml", Name: "YAML", Template: "mihomo-yaml"},
				},
			},
			{
				ID: "sing-box", Template: "singbox",
				Variants: []config.ClientVariantConfig{{
					ID: "sing-box-non-ip", Name: "Non-IP",
					Ops: []config.OpConfig{{Type: "exclude_kinds", Kinds: []string{"ip_cidr"}}},
				}},
			},
		},
		Rules: []config.RuleConfig{{
			ID: "explicit", Name: "Explicit",
			Sources: []config.SourceConfig{{Content: "DOMAIN,example.com\nIP-CIDR,192.0.2.0/24"}},
			Outputs: []string{"mihomo", "sing-box"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	result := eng.FullUpdate(context.Background())
	if len(result.Errors) != 0 || result.Artifacts != 4 {
		t.Fatalf("result=%+v", result)
	}
	classical := readArtifact(t, dataDir, "mihomo-classical", "explicit.list")
	if !strings.Contains(classical, "DOMAIN,example.com") {
		t.Fatalf("classical=%s", classical)
	}
	yamlOutput := readArtifact(t, dataDir, "mihomo-yaml", "explicit.yaml")
	if !strings.Contains(yamlOutput, "payload:") || !strings.Contains(yamlOutput, "DOMAIN,example.com") {
		t.Fatalf("yaml=%s", yamlOutput)
	}
	variant := readArtifact(t, dataDir, "sing-box-non-ip", "explicit.json")
	if strings.Contains(variant, "192.0.2.0/24") || !strings.Contains(variant, "example.com") {
		t.Fatalf("variant=%s", variant)
	}
}

func TestPartialUpdateRejectsUnknownRuleIDs(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
	}
	eng := newTestUpdateEngine(t, cfg)
	result := eng.PartialUpdate(context.Background(), []string{"typo"})
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "unknown rule IDs: typo") {
		t.Fatalf("partial update result: %+v", result)
	}
}

func TestPartialUpdateRemovesObsoleteArtifactsForUpdatedRules(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{
			ID: "mihomo",
			Formats: []config.ClientFormatConfig{
				{ID: "mihomo-classical", Name: "Classical", Template: "mihomo-classical"},
				{ID: "mihomo-yaml", Name: "YAML", Template: "mihomo-yaml"},
			},
		}},
		Rules: []config.RuleConfig{{
			ID: "rule", Name: "rule", Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}}, Outputs: []string{"mihomo"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	removedDir := filepath.Join(dataDir, "rules", "removed-target")
	if err := os.MkdirAll(removedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	obsoletePath := filepath.Join(removedDir, "rule.list")
	otherRulePath := filepath.Join(removedDir, "other.list")
	for path, content := range map[string]string{
		obsoletePath:  "DOMAIN,old.example\n",
		otherRulePath: "DOMAIN,other.example\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	eng.State.SetArtifactHash("rule", "removed-target", "obsolete")
	eng.State.SetArtifactHash("other", "removed-target", "preserved")

	result := eng.PartialUpdate(context.Background(), []string{"rule"})
	if len(result.Errors) != 0 || result.Artifacts != 2 {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(obsoletePath); !os.IsNotExist(err) {
		t.Fatalf("obsolete artifact remains: %v", err)
	}
	if _, err := os.Stat(otherRulePath); err != nil {
		t.Fatalf("other rule artifact: %v", err)
	}
	if hash := eng.State.GetArtifactHash("rule", "removed-target"); hash != "" {
		t.Fatalf("obsolete hash remains: %q", hash)
	}
	if hash := eng.State.GetArtifactHash("other", "removed-target"); hash != "preserved" {
		t.Fatalf("other rule hash=%q", hash)
	}
}

func TestFullUpdateRemovesVariantArtifactWhenRenderedOutputIsEmpty(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{
			ID: "singbox", Template: "singbox",
			Variants: []config.ClientVariantConfig{{
				ID: "singbox-non-ip", Ops: []config.OpConfig{{Type: "exclude_kinds", Kinds: []string{"ip_cidr"}}},
			}},
		}},
		Rules: []config.RuleConfig{{
			ID:      "empty",
			Name:    "empty",
			Sources: []config.SourceConfig{{Content: "IP-CIDR,192.0.2.0/24"}},
			Outputs: []string{"singbox"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	artifactPath := filepath.Join(dataDir, "rules", "singbox-non-ip", "empty.json")
	if err := os.WriteFile(artifactPath, []byte("{\"rules\":null,\"version\":3}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng.State.SetArtifactHash("empty", "singbox-non-ip", "stale")

	result := eng.FullUpdate(context.Background())
	if result.RulesSucceeded != 1 || result.RulesFailed != 0 || result.Artifacts != 1 || len(result.Errors) != 0 {
		t.Fatalf("update result: %+v", result)
	}
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("empty artifact remains: %v", err)
	}
	if hash := eng.State.GetArtifactHash("empty", "singbox-non-ip"); hash != "" {
		t.Fatalf("stale hash remains: %q", hash)
	}
	if len(result.Changes) != 1 || len(result.Changes[0].Files) != 2 {
		t.Fatalf("deleted artifact change = %+v", result.Changes)
	}
	var deleted bool
	for _, file := range result.Changes[0].Files {
		deleted = deleted || file.ClientID == "singbox-non-ip" && file.Change == "deleted"
	}
	if !deleted {
		t.Fatalf("variant deletion missing: %+v", result.Changes[0].Files)
	}
	assertRuleResult(t, eng, "empty", state.RuleUpdated)
}

func TestFullUpdateCountsArtifactWriteFailure(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir: dataDir,
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{{
			ID:      "blocked",
			Name:    "blocked",
			Sources: []config.SourceConfig{{Content: "DOMAIN,example.com"}},
			Outputs: []string{"surge"},
		}},
	}
	eng := newTestUpdateEngine(t, cfg)
	artifactPath := filepath.Join(dataDir, "rules", "surge", "blocked.list")
	clientPath := filepath.Dir(artifactPath)
	if err := os.Remove(clientPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clientPath, []byte("blocks artifact directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := eng.FullUpdate(context.Background())
	if result.RulesSucceeded != 0 || result.RulesFailed != 1 || result.Artifacts != 0 || len(result.Errors) != 1 {
		t.Fatalf("update result: %+v", result)
	}
	if !strings.Contains(result.Errors[0], "write "+artifactPath) {
		t.Fatalf("write error: %v", result.Errors)
	}
	assertRuleResult(t, eng, "blocked", state.RuleFailed)
}

func TestCancelledUpdateRecordsEveryPendingRule(t *testing.T) {
	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "first", Name: "first", Sources: []config.SourceConfig{{Content: "DOMAIN,first.example"}}, Outputs: []string{"surge"}},
			{ID: "second", Name: "second", Sources: []config.SourceConfig{{Content: "DOMAIN,second.example"}}, Outputs: []string{"surge"}},
		},
	}
	eng := newTestUpdateEngine(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := eng.FullUpdate(ctx)
	if len(result.Errors) != 1 || result.Errors[0] != "update cancelled" {
		t.Fatalf("cancelled update: %+v", result)
	}
	assertRuleResult(t, eng, "first", state.RuleCancelled)
	assertRuleResult(t, eng, "second", state.RuleCancelled)
}

func TestCancelledUpdateKeepsCompletedRuleResult(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		select {
		case requestStarted <- struct{}{}:
		default:
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	cfg := &config.Config{
		DataDir: t.TempDir(),
		Clients: []config.ClientConfig{{ID: "surge", Template: "surge"}},
		Rules: []config.RuleConfig{
			{ID: "completed", Name: "completed", Sources: []config.SourceConfig{{Content: "DOMAIN,completed.example"}}, Outputs: []string{"surge"}},
			{ID: "pending", Name: "pending", Sources: []config.SourceConfig{{URL: "https://example.com/rules.list"}}, Outputs: []string{"surge"}},
		},
	}
	eng := newTestUpdateEngine(t, cfg)
	eng.Fetcher.Client = &http.Client{Transport: transport}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan UpdateResult, 1)
	go func() { done <- eng.FullUpdate(ctx) }()

	<-requestStarted
	cancel()
	result := <-done
	if len(result.Errors) == 0 {
		t.Fatalf("cancelled update has no result error: %+v", result)
	}
	assertRuleResult(t, eng, "completed", state.RuleUpdated)
	assertRuleResult(t, eng, "pending", state.RuleCancelled)
}

func newTestUpdateEngine(t *testing.T, cfg *config.Config) *UpdateEngine {
	t.Helper()
	registry := testRegistry(t)
	store, err := state.Open(cfg.DataDir)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	targets := config.ExpandOutputTargets(cfg.Clients)
	clientIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		clientIDs = append(clientIDs, target.ID)
	}
	if err := EnsureArtifactDirs(cfg.DataDir, clientIDs); err != nil {
		t.Fatalf("create artifact dirs: %v", err)
	}
	return &UpdateEngine{
		Config:       cfg,
		Registry:     registry,
		Fetcher:      NewFetcher(),
		Preprocessor: NewPreprocessRunner(),
		State:        store,
		Logger:       testLogger(),
	}
}

func readArtifact(t *testing.T, dataDir, client, name string) string {
	t.Helper()
	path := filepath.Join(dataDir, "rules", client, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact %s: %v", path, err)
	}
	return string(data)
}

func assertRuleResult(t *testing.T, eng *UpdateEngine, ruleID, want string) {
	t.Helper()
	result, checkedAt, _, ok := eng.State.RuleUpdate(ruleID)
	if !ok || result != want || checkedAt.IsZero() {
		t.Fatalf("rule %q result = %q, %v, %t; want %q", ruleID, result, checkedAt, ok, want)
	}
}
