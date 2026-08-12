package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: ./data
clients:
  - id: clash
    name: Clash
    template: mihomo-classical
rules:
  - id: TestRule
    name: TestRule
    description: Test rule description
    tags: [test, example]
    sources:
      - url: https://example.com/rules.list
    outputs: [clash]
update:
  schedule:
    mode: manual
  fetch:
    retries: 0
serve:
  port: 3001
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if len(cfg.Clients) != 1 {
		t.Errorf("len(Clients) = %d, want 1", len(cfg.Clients))
	}
	if len(cfg.Rules) != 1 {
		t.Errorf("len(Rules) = %d, want 1", len(cfg.Rules))
	}
	if cfg.Rules[0].ID != "TestRule" || cfg.Rules[0].Name != "TestRule" {
		t.Errorf("rule identity = %q / %q", cfg.Rules[0].ID, cfg.Rules[0].Name)
	}
	if cfg.Rules[0].Description != "Test rule description" {
		t.Errorf("Description = %q", cfg.Rules[0].Description)
	}
	if len(cfg.Rules[0].Tags) != 2 || cfg.Rules[0].Tags[0] != "test" || cfg.Rules[0].Tags[1] != "example" {
		t.Errorf("Tags = %v", cfg.Rules[0].Tags)
	}
	if cfg.Serve.Port != 3001 {
		t.Errorf("Serve.Port = %d, want 3001", cfg.Serve.Port)
	}
	if cfg.Update.Fetch.Retries != 0 {
		t.Errorf("explicit Retries = %d, want 0", cfg.Update.Fetch.Retries)
	}
}

func TestLoadExplicitFormatsWithVariant(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
clients:
  - id: sing-box
    name: sing-box
    template: singbox
    variants:
      - id: sing-box-non-ip
        name: Non-IP
        ops:
          - type: exclude_kinds
            kinds: [ip_cidr]
rules: []
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	targets := ExpandOutputTargets(cfg.Clients)
	if len(targets) != 2 || targets[0].ID != "sing-box" || targets[1].ID != "sing-box-non-ip" {
		t.Fatalf("targets=%+v", targets)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
clients:
  - id: test
    name: Test
    template: test-tmpl
rules:
  - id: R
    name: R
    sources:
      - url: https://example.com/r.list
    outputs: [test]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("default DataDir = %q", cfg.DataDir)
	}
	if cfg.Update.Fetch.Concurrency != 4 {
		t.Errorf("default Concurrency = %d", cfg.Update.Fetch.Concurrency)
	}
	if cfg.Update.Fetch.PerHostConcurrency != 2 {
		t.Errorf("default PerHostConcurrency = %d", cfg.Update.Fetch.PerHostConcurrency)
	}
	if cfg.Update.Fetch.RetryDelay != Duration(500*time.Millisecond) {
		t.Errorf("default RetryDelay = %v, want 500ms", cfg.Update.Fetch.RetryDelay)
	}
	if cfg.Update.Fetch.Retries != 2 {
		t.Errorf("default Retries = %d, want 2", cfg.Update.Fetch.Retries)
	}
	if cfg.Update.HistoryRetention != Duration(7*24*time.Hour) {
		t.Fatalf("history retention = %s", time.Duration(cfg.Update.HistoryRetention))
	}
	if cfg.Update.HistoryLimit != 200 {
		t.Fatalf("history limit = %d", cfg.Update.HistoryLimit)
	}
}

func TestLoadRejectsRemovedErrorRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `clients:
  - id: c
    template: t
rules:
  - id: r
    name: r
    sources:
      - content: example.com
    outputs: [c]
update:
  error_retention: 168h
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "error_retention") {
		t.Fatalf("removed field error = %v", err)
	}
}

func TestValidateHistoryBounds(t *testing.T) {
	cfg := Config{Update: UpdateConfig{HistoryRetention: Duration(0), HistoryLimit: 10001}}
	if errs := cfg.Validate(); !containsErrorPath(errs, "update.history_retention") || !containsErrorPath(errs, "update.history_limit") {
		t.Fatalf("history validation errors = %v", ConfigErrors(errs))
	}
}

func TestLoadEnvInterpolation(t *testing.T) {
	t.Setenv("TEST_DATA_DIR", "./env-data")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: ${TEST_DATA_DIR}
clients:
  - id: c
    template: t
rules:
  - id: R
    name: R
    sources:
      - content: example.com
    outputs: [c]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "./env-data" {
		t.Fatalf("data dir: %q", cfg.DataDir)
	}
}

func TestDefaultsBoundPerHostConcurrencyWhenOmitted(t *testing.T) {
	cfg := Config{Update: UpdateConfig{Fetch: FetchConfig{Concurrency: 1}}}
	cfg.Defaults()
	if cfg.Update.Fetch.PerHostConcurrency != 1 {
		t.Fatalf("per-host concurrency = %d, want 1", cfg.Update.Fetch.PerHostConcurrency)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
clients:
  - id: c
    template: t
rules:
  - id: R
    name: R
    sources:
      - content: example.com
    outputs: [c]
serve:
  prot: 3001
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

func TestLoadRejectsArtifactPathTraversal(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `clients:
  - id: c
    template: t
rules:
  - id: ../escape
    name: ../escape
    sources:
      - content: example.com
    outputs: [c]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(cfgPath)
	var configErrs ConfigErrors
	if !errors.As(err, &configErrs) {
		t.Fatalf("error: %v", err)
	}
	for _, configErr := range configErrs {
		if configErr.Path == "rules[0].id" && configErr.Line == 5 {
			return
		}
	}
	t.Fatalf("path error: %v", configErrs)
}

func TestValidateDuplicateClient(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{
			{ID: "a", Template: "t"},
			{ID: "a", Template: "t"},
		},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); !containsErrorPath(errs, "clients[1].id") {
		t.Fatalf("missing duplicate client error: %v", ConfigErrors(errs))
	}
}

func TestValidateNoClients(t *testing.T) {
	cfg := &Config{}
	cfg.Defaults()
	if errs := cfg.Validate(); !containsErrorPath(errs, "clients") {
		t.Fatalf("missing clients error: %v", ConfigErrors(errs))
	}
}

func TestValidateRuleIdentity(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules: []RuleConfig{
			{ID: "first", Name: "Shared Display Name", Sources: []SourceConfig{{Content: "first.example"}}, Outputs: []string{"a"}},
			{ID: "second", Name: "Shared Display Name", Sources: []SourceConfig{{Content: "second.example"}}, Outputs: []string{"a"}},
		},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); len(errs) > 0 {
		t.Fatalf("display names may repeat: %v", ConfigErrors(errs))
	}

	cfg.Rules[1].ID = "first"
	if errs := cfg.Validate(); !containsErrorPath(errs, "rules[1].id") {
		t.Fatalf("missing duplicate rule ID error: %v", ConfigErrors(errs))
	}

	cfg.Rules[1].ID = ""
	cfg.Rules[1].Name = ""
	errs := cfg.Validate()
	if !containsErrorPath(errs, "rules[1].id") || !containsErrorPath(errs, "rules[1].name") {
		t.Fatalf("missing required rule identity errors: %v", ConfigErrors(errs))
	}
}

func TestValidateUnknownOutput(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules: []RuleConfig{
			{
				ID:      "R",
				Name:    "R",
				Sources: []SourceConfig{{URL: "https://example.com/r"}},
				Outputs: []string{"nonexistent"},
			},
		},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); !containsErrorPath(errs, "rules[0].outputs[0]") {
		t.Fatalf("missing output client error: %v", ConfigErrors(errs))
	}
}

func TestValidateRuleTags(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules: []RuleConfig{{
			ID:      "R",
			Name:    "R",
			Tags:    []string{"media", "", "media"},
			Sources: []SourceConfig{{URL: "https://example.com/r"}},
			Outputs: []string{"a"},
		}},
	}
	cfg.Defaults()
	errs := cfg.Validate()
	if !containsErrorPath(errs, "rules[0].tags[1]") || !containsErrorPath(errs, "rules[0].tags[2]") {
		t.Fatalf("tag errors: %v", ConfigErrors(errs))
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"4MB", 4 * 1024 * 1024},
		{"1KB", 1024},
		{"512B", 512},
		{"1GB", 1024 * 1024 * 1024},
	}
	for _, tt := range tests {
		got, err := ParseSize(tt.input)
		if err != nil {
			t.Errorf("ParseSize(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
	if _, err := ParseSize("1MBgarbage"); err == nil {
		t.Fatal("size with trailing data was accepted")
	}
}

func TestResolveGeositeRef(t *testing.T) {
	src := SourceConfig{Geosite: "v2fly/geolocation-!cn@ads"}
	ref, err := src.ResolveGeositeRef()
	if err != nil {
		t.Fatalf("ResolveGeositeRef: %v", err)
	}
	if ref.Provider != "v2fly" || ref.List != "geolocation-!cn" || len(ref.Attrs) != 1 || ref.Attrs[0] != "ads" {
		t.Errorf("unexpected ref: %+v", ref)
	}

	src2 := SourceConfig{Provider: "v2fly", List: "google", Attrs: []string{"cn"}}
	ref2, err := src2.ResolveGeositeRef()
	if err != nil {
		t.Fatalf("ResolveGeositeRef: %v", err)
	}
	if ref2.Provider != "v2fly" || ref2.List != "google" || len(ref2.Attrs) != 1 || ref2.Attrs[0] != "cn" {
		t.Errorf("unexpected ref: %+v", ref2)
	}
}

func TestValidateGeositeCompactRef(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules: []RuleConfig{
			{
				ID:      "R",
				Name:    "R",
				Sources: []SourceConfig{{Geosite: "v2fly/google"}},
				Outputs: []string{"a"},
			},
		},
		Update: UpdateConfig{Schedule: ScheduleConfig{Mode: "manual"}},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); len(errs) > 0 {
		t.Errorf("expected valid config, got: %v", ConfigErrors(errs))
	}
}

func TestValidateGeositeInvalidRef(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules: []RuleConfig{
			{
				ID:      "R",
				Name:    "R",
				Sources: []SourceConfig{{Geosite: "invalid-no-slash"}},
				Outputs: []string{"a"},
			},
		},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); !containsErrorPath(errs, "rules[0].sources[0].geosite") {
		t.Fatalf("missing geosite ref error: %v", ConfigErrors(errs))
	}
}

func TestValidateGeositeProviderWithClients(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules:   []RuleConfig{{ID: "R", Name: "R", Sources: []SourceConfig{{URL: "https://x.com/r"}}, Outputs: []string{"a"}}},
		Geosite: &GeositeConfig{
			Providers: []GeositeProvider{{Name: "v2fly", Clients: []string{"a"}}},
		},
		Update: UpdateConfig{Schedule: ScheduleConfig{Mode: "manual"}},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); len(errs) > 0 {
		t.Errorf("expected valid config, got: %v", ConfigErrors(errs))
	}
}

func TestValidateGeositeProviderMissingClients(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules:   []RuleConfig{{ID: "R", Name: "R", Sources: []SourceConfig{{URL: "https://x.com/r"}}, Outputs: []string{"a"}}},
		Geosite: &GeositeConfig{
			Providers: []GeositeProvider{{Name: "v2fly"}},
		},
		Update: UpdateConfig{Schedule: ScheduleConfig{Mode: "manual"}},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); !containsErrorPath(errs, "geosite.providers[0].clients") {
		t.Fatalf("missing provider clients error: %v", ConfigErrors(errs))
	}
}

func TestValidateGeositeProviderUnknownClient(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "t"}},
		Rules:   []RuleConfig{{ID: "R", Name: "R", Sources: []SourceConfig{{URL: "https://x.com/r"}}, Outputs: []string{"a"}}},
		Geosite: &GeositeConfig{
			Providers: []GeositeProvider{{Name: "v2fly", Clients: []string{"nonexistent"}}},
		},
		Update: UpdateConfig{Schedule: ScheduleConfig{Mode: "manual"}},
	}
	cfg.Defaults()
	if errs := cfg.Validate(); !containsErrorPath(errs, "geosite.providers[0].clients[0]") {
		t.Fatalf("missing provider client error: %v", ConfigErrors(errs))
	}
}

func TestValidateRuleProcessingBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		source   SourceConfig
		ops      []OpConfig
		outputs  []string
		wantPath string
	}{
		{
			name:     "unknown source type",
			source:   SourceConfig{Type: "database"},
			outputs:  []string{"a"},
			wantPath: "rules[0].sources[0].type",
		},
		{
			name:     "conflicting source selectors",
			source:   SourceConfig{URL: "https://example.com/rules", Content: "example.com"},
			outputs:  []string{"a"},
			wantPath: "rules[0].sources[0]",
		},
		{
			name:     "unknown operation kind",
			source:   SourceConfig{Content: "example.com"},
			ops:      []OpConfig{{Type: "include_kinds", Kinds: []string{"domain_typo"}}},
			outputs:  []string{"a"},
			wantPath: "rules[0].ops[0].kinds[0]",
		},
		{
			name:     "unknown filter mode",
			source:   SourceConfig{Content: "example.com"},
			ops:      []OpConfig{{Type: "filter_values", Mode: "glob", Pattern: "*.com"}},
			outputs:  []string{"a"},
			wantPath: "rules[0].ops[0].mode",
		},
		{
			name:     "invalid filter regex",
			source:   SourceConfig{Content: "example.com"},
			ops:      []OpConfig{{Type: "filter_values", Mode: "regex", Pattern: "(["}},
			outputs:  []string{"a"},
			wantPath: "rules[0].ops[0].pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Clients: []ClientConfig{{ID: "a", Template: "surge"}},
				Rules: []RuleConfig{{
					ID:      "rule",
					Name:    "rule",
					Sources: []SourceConfig{tt.source},
					Ops:     tt.ops,
					Outputs: tt.outputs,
				}},
			}
			cfg.Defaults()
			errs := cfg.Validate()
			for _, err := range errs {
				if err.Path == tt.wantPath {
					return
				}
			}
			t.Fatalf("missing error at %s; got %v", tt.wantPath, ConfigErrors(errs))
		})
	}
}

func TestValidateUnknownRuleReference(t *testing.T) {
	cfg := &Config{
		Clients: []ClientConfig{{ID: "a", Template: "surge"}},
		Rules: []RuleConfig{{
			ID:      "derived",
			Name:    "derived",
			Sources: []SourceConfig{{Ref: "missing"}},
			Outputs: []string{"a"},
		}},
	}
	cfg.Defaults()
	for _, err := range cfg.Validate() {
		if err.Path == "rules[0].sources[0].ref" {
			return
		}
	}
	t.Fatal("expected unknown rule reference error")
}

func TestValidateFetchBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantPath    string
		wantLine    int
		wantMessage string
	}{
		{"invalid timeout", "    timeout: 0s", "update.fetch.timeout", 12, "must be a positive duration"},
		{"invalid maximum size", "    max_download: 0B", "update.fetch.max_download", 12, "must be a positive size"},
		{"invalid global concurrency", "    concurrency: 65", "update.fetch.concurrency", 12, "must be between 1 and 64"},
		{"host limit exceeds global", "    concurrency: 2\n    per_host_concurrency: 5", "update.fetch.per_host_concurrency", 13, "must be between 1 and update.fetch.concurrency"},
		{"invalid retries", "    retries: 11", "update.fetch.retries", 12, "must be between 0 and 10"},
		{"invalid retry delay", "    retry_delay: 0s", "update.fetch.retry_delay", 12, "must be a positive duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			content := fmt.Sprintf(`clients:
  - id: a
    template: surge
rules:
  - id: rule
    name: rule
    sources:
      - content: example.com
    outputs: [a]
update:
  fetch:
%s
`, tt.yaml)
			if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(cfgPath)
			var configErrs ConfigErrors
			if !errors.As(err, &configErrs) {
				t.Fatalf("error: %v", err)
			}
			for _, configErr := range configErrs {
				if configErr.Path == tt.wantPath {
					if configErr.Line != tt.wantLine || configErr.Message != tt.wantMessage {
						t.Fatalf("error: %+v", configErr)
					}
					return
				}
			}
			t.Fatalf("missing error at %s: %v", tt.wantPath, configErrs)
		})
	}
}

func TestLoadRejectsLegacySyncField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("sync:\n  schedule:\n    mode: manual\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "field sync not found") {
		t.Fatalf("legacy field error = %v", err)
	}
}

func containsErrorPath(errs []ConfigError, path string) bool {
	for _, err := range errs {
		if err.Path == path {
			return true
		}
	}
	return false
}
