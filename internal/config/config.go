// Package config defines the YAML configuration model for prm, including
// loader, validator, and environment-variable interpolation.
package config

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geosite"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/internal/util"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from YAML strings like "15s".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return fmt.Errorf("duration must be a string like \"15s\"")
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Size is a byte count that unmarshals from YAML strings like "4MB" or
// plain integer byte counts.
type Size int64

func (s *Size) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err == nil {
		n, err := ParseSize(str)
		if err != nil {
			return err
		}
		*s = Size(n)
		return nil
	}
	// Fall back to plain integer (bytes).
	var n int64
	if err := value.Decode(&n); err != nil {
		return fmt.Errorf("size must be a string like \"4MB\" or an integer byte count")
	}
	*s = Size(n)
	return nil
}

// Config is the root configuration loaded from config.yaml.
type Config struct {
	DataDir string `yaml:"data_dir"`

	Clients []ClientConfig `yaml:"clients"`
	Rules   []RuleConfig   `yaml:"rules"`

	Geosite *GeositeConfig `yaml:"geosite,omitempty"`
	Update  UpdateConfig   `yaml:"update"`
	Serve   ServeConfig    `yaml:"serve"`

	// positions is populated by Load() from the raw YAML node tree;
	// used by Validate() to attach line numbers to errors.
	positions *PositionIndex
}

// ClientConfig defines one output client.
type ClientConfig struct {
	ID       string                `yaml:"id"`
	Name     string                `yaml:"name"`
	Template string                `yaml:"template,omitempty"`
	Icon     string                `yaml:"icon,omitempty"`
	Formats  []ClientFormatConfig  `yaml:"formats,omitempty"`
	Variants []ClientVariantConfig `yaml:"variants,omitempty"`
}

// ClientFormatConfig defines one explicit output format of a client family.
type ClientFormatConfig struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name,omitempty"`
	Template string `yaml:"template"`
}

// ClientVariantConfig defines an explicit IR-level derived output.
type ClientVariantConfig struct {
	ID       string     `yaml:"id"`
	Name     string     `yaml:"name,omitempty"`
	Template string     `yaml:"template,omitempty"`
	Ops      []OpConfig `yaml:"ops"`
}

// RuleConfig defines one rule to compile.
type RuleConfig struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Tags        []string       `yaml:"tags,omitempty"`
	Sources     []SourceConfig `yaml:"sources"`
	Ops         []OpConfig     `yaml:"ops,omitempty"`
	Merge       *MergeConfig   `yaml:"merge,omitempty"`
	Outputs     []string       `yaml:"outputs"`

	Preprocess string `yaml:"preprocess,omitempty"`
}

// SourceConfig defines one rule source.
type SourceConfig struct {
	URL     string `yaml:"url,omitempty"`
	Type    string `yaml:"type,omitempty"`
	Format  string `yaml:"format,omitempty"`
	Ref     string `yaml:"ref,omitempty"` // referenced rule ID
	Content string `yaml:"content,omitempty"`
	File    string `yaml:"file,omitempty"`
	Label   string `yaml:"label,omitempty"`

	// Geosite compact ref: "provider/list" or "provider/list@attr1,attr2"
	// Preferred over separate Provider/List/Attrs fields.
	Geosite  string   `yaml:"geosite,omitempty"`
	Provider string   `yaml:"provider,omitempty"`
	List     string   `yaml:"list,omitempty"`
	Attrs    []string `yaml:"attrs,omitempty"`
}

// SourceType returns the canonical source type.
func (s *SourceConfig) SourceType() string {
	if s.Type != "" {
		return s.Type
	}
	if s.URL != "" {
		return "url"
	}
	if s.Ref != "" {
		return "ref"
	}
	if s.Geosite != "" || s.Provider != "" {
		return "geosite"
	}
	if s.Content != "" || s.File != "" {
		return "local"
	}
	return ""
}

// ResolveGeositeRef returns a parsed GeositeRef from either the compact
// Geosite field or the separate Provider/List/Attrs fields.
func (s *SourceConfig) ResolveGeositeRef() (geosite.GeositeRef, error) {
	if s.Geosite != "" {
		return geosite.ParseRef(s.Geosite)
	}
	ref := geosite.GeositeRef{
		Provider: strings.ToLower(strings.TrimSpace(s.Provider)),
		List:     strings.ToLower(strings.TrimSpace(s.List)),
		Attrs:    geosite.NormalizeAttrs(s.Attrs),
	}
	if err := geosite.ValidateRefSegments(ref); err != nil {
		return geosite.GeositeRef{}, err
	}
	return ref, nil
}

// OpConfig defines one structured operation on parsed entries.
type OpConfig struct {
	Type    string   `yaml:"type"`
	Kinds   []string `yaml:"kinds,omitempty"`
	Mode    string   `yaml:"mode,omitempty"`
	Pattern string   `yaml:"pattern,omitempty"`
}

// MergeConfig controls how multi-source entries are combined.
type MergeConfig struct {
	Strategy string `yaml:"strategy"`
}

// GeositeConfig configures geosite providers for automatic publication.
// All lists and their attr variants are auto-discovered and published;
// no manual per-list declaration needed.
type GeositeConfig struct {
	Providers []GeositeProvider `yaml:"providers"`
}

// GeositeProvider defines a geosite data source.
// During update, all lists from this provider are enumerated automatically.
// For each list, the full list plus every attr variant is published.
// Output naming: provider/list{ext}, provider/list@attr{ext}
type GeositeProvider struct {
	Name    string   `yaml:"name"`
	Clients []string `yaml:"clients"`
}

// UpdateConfig controls update scheduling and fetch behavior.
type UpdateConfig struct {
	Schedule         ScheduleConfig   `yaml:"schedule"`
	Fetch            FetchConfig      `yaml:"fetch"`
	Preprocess       PreprocessConfig `yaml:"preprocess"`
	HistoryRetention Duration         `yaml:"history_retention,omitempty"`
	HistoryLimit     int              `yaml:"history_limit,omitempty"`
}

// ScheduleConfig controls when automatic updates run.
type ScheduleConfig struct {
	Mode     string   `yaml:"mode"`
	Interval Duration `yaml:"interval,omitempty"`
	Cron     string   `yaml:"cron,omitempty"`
	Timezone string   `yaml:"timezone,omitempty"`
}

// FetchConfig controls URL fetching behavior.
type FetchConfig struct {
	Timeout            Duration `yaml:"timeout"`
	MaxDownload        Size     `yaml:"max_download"`
	Concurrency        int      `yaml:"concurrency"`
	PerHostConcurrency int      `yaml:"per_host_concurrency"`
	Retries            int      `yaml:"retries"`
	RetryDelay         Duration `yaml:"retry_delay"`
	UserAgent          string   `yaml:"user_agent"`
}

// PreprocessConfig controls the JS preprocess sandbox.
type PreprocessConfig struct {
	Timeout   Duration `yaml:"timeout"`
	MaxOutput Size     `yaml:"max_output"`
}

// ServeConfig controls the optional HTTP server.
type ServeConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	// TrustedProxies is the list of IP/CIDR ranges of reverse proxies whose
	// Forwarded/X-Forwarded-Proto headers prm will honor to detect HTTPS
	// behind TLS-terminating proxies. Requests whose TCP peer is not in this
	// list ignore forwarded headers, so a public client cannot fake the
	// scheme. Empty (default) means no proxy is trusted; HTTPS is detected
	// only from a direct TLS connection (r.TLS != nil).
	TrustedProxies []string `yaml:"trusted_proxies,omitempty"`
}

// Defaults fills in zero values with sensible defaults.
func (c *Config) Defaults() {
	shouldDefault := func(path string) bool {
		return c.positions == nil || !c.positions.Has(path)
	}
	if c.DataDir == "" && shouldDefault("data_dir") {
		c.DataDir = "./data"
	}
	if c.Update.Schedule.Mode == "" && shouldDefault("update.schedule.mode") {
		c.Update.Schedule.Mode = "manual"
	}
	if c.Update.Schedule.Timezone == "" && shouldDefault("update.schedule.timezone") {
		c.Update.Schedule.Timezone = "UTC"
	}
	if c.Update.Fetch.Timeout == 0 && shouldDefault("update.fetch.timeout") {
		c.Update.Fetch.Timeout = Duration(15 * time.Second)
	}
	if c.Update.Fetch.MaxDownload == 0 && shouldDefault("update.fetch.max_download") {
		c.Update.Fetch.MaxDownload = Size(4 * 1024 * 1024)
	}
	if c.Update.Fetch.Concurrency == 0 && shouldDefault("update.fetch.concurrency") {
		c.Update.Fetch.Concurrency = 4
	}
	if c.Update.Fetch.PerHostConcurrency == 0 && shouldDefault("update.fetch.per_host_concurrency") {
		c.Update.Fetch.PerHostConcurrency = 2
		if c.Update.Fetch.Concurrency > 0 && c.Update.Fetch.Concurrency < c.Update.Fetch.PerHostConcurrency {
			c.Update.Fetch.PerHostConcurrency = c.Update.Fetch.Concurrency
		}
	}
	if c.Update.Fetch.Retries == 0 && shouldDefault("update.fetch.retries") {
		c.Update.Fetch.Retries = 2
	}
	if c.Update.Fetch.RetryDelay == 0 && shouldDefault("update.fetch.retry_delay") {
		c.Update.Fetch.RetryDelay = Duration(500 * time.Millisecond)
	}
	if c.Update.Fetch.UserAgent == "" && shouldDefault("update.fetch.user_agent") {
		c.Update.Fetch.UserAgent = "Proxy-Rule-Manager/2.0"
	}
	if c.Update.Preprocess.Timeout == 0 && shouldDefault("update.preprocess.timeout") {
		c.Update.Preprocess.Timeout = Duration(5 * time.Second)
	}
	if c.Update.Preprocess.MaxOutput == 0 && shouldDefault("update.preprocess.max_output") {
		c.Update.Preprocess.MaxOutput = Size(8 * 1024 * 1024)
	}
	if c.Update.HistoryRetention == 0 && shouldDefault("update.history_retention") {
		c.Update.HistoryRetention = Duration(7 * 24 * time.Hour)
	}
	if c.Update.HistoryLimit == 0 && shouldDefault("update.history_limit") {
		c.Update.HistoryLimit = 200
	}
	if c.Serve.Port == 0 && shouldDefault("serve.port") {
		c.Serve.Port = 3001
	}
	if c.Serve.Host == "" && shouldDefault("serve.host") {
		c.Serve.Host = "127.0.0.1"
	}
}

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// interpolateEnv replaces ${VAR} placeholders with os.Getenv(VAR).
func interpolateEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := envPattern.FindStringSubmatch(match)[1]
		if v, ok := os.LookupEnv(varName); ok {
			return v
		}
		return match
	})
}

// Load reads and parses a YAML config file, applying environment variable
// interpolation and defaults. The returned Config retains YAML source
// positions so that Validate() can report line-precise errors.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	expanded := interpolateEnv(string(data))

	// Parse into Node tree to capture source positions.
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(expanded), &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(strings.NewReader(expanded))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	cfg.positions = BuildPositionIndex(&doc)
	cfg.Defaults()

	if errs := cfg.Validate(); len(errs) > 0 {
		return nil, ConfigErrors(errs)
	}
	return &cfg, nil
}

// Validate checks the config for structural correctness. Returns a list of
// errors, each carrying the YAML path and line number of the offending node.
// An empty return means the config is valid.
func (c *Config) Validate() []ConfigError {
	var errs []ConfigError
	pos := c.positions

	addErr := func(path, msg string) {
		p := Position{}
		if pos != nil {
			p = pos.Lookup(path)
		}
		errs = append(errs, ConfigError{Path: path, Line: p.Line, Message: msg})
	}
	if c.DataDir == "" {
		addErr("data_dir", "must not be empty")
	}

	if len(c.Clients) == 0 {
		addErr("clients", "at least one client is required")
	}
	clientIDs := map[string]bool{}
	for i, cl := range c.Clients {
		base := fmt.Sprintf("clients[%d]", i)
		if cl.ID == "" {
			addErr(base+".id", "required")
			continue
		}
		if err := util.EnsureSafeSegment(cl.ID, "client id"); err != nil {
			addErr(base+".id", err.Error())
		}
		if len(cl.Formats) == 0 && cl.Template == "" {
			addErr(base, "formats or template is required")
		}
		if len(cl.Formats) > 0 && cl.Template != "" {
			addErr(base, "formats and template are mutually exclusive")
		}
		if clientIDs[cl.ID] {
			addErr(base+".id", fmt.Sprintf("duplicate client id %q", cl.ID))
		}
		clientIDs[cl.ID] = true

		outputIDs := map[string]bool{}
		if len(cl.Formats) == 0 {
			outputIDs[cl.ID] = true
		}
		for j, format := range cl.Formats {
			formatPath := fmt.Sprintf("%s.formats[%d]", base, j)
			validateOutputID(formatPath, format.ID, "format", outputIDs, addErr)
			if format.Template == "" {
				addErr(formatPath+".template", "required")
			}
		}
		for j, variant := range cl.Variants {
			variantPath := fmt.Sprintf("%s.variants[%d]", base, j)
			validateOutputID(variantPath, variant.ID, "variant", outputIDs, addErr)
			if variant.Template == "" && cl.Template == "" {
				addErr(variantPath+".template", "required for a multi-format client")
			}
			if len(variant.Ops) == 0 {
				addErr(variantPath+".ops", "at least one operation is required")
			}
			validateOps(variantPath+".ops", variant.Ops, addErr)
		}
	}
	targetIDs := map[string]bool{}
	for _, target := range ExpandOutputTargets(c.Clients) {
		if targetIDs[target.ID] {
			addErr("clients", fmt.Sprintf("duplicate expanded output id %q", target.ID))
		}
		targetIDs[target.ID] = true
		if clientIDs[target.ID] && target.ID != target.ClientID {
			addErr("clients", fmt.Sprintf("output id %q conflicts with client id", target.ID))
		}
	}

	ruleIDs := map[string]bool{}
	for i, r := range c.Rules {
		base := fmt.Sprintf("rules[%d]", i)
		if r.ID == "" {
			addErr(base+".id", "required")
		} else {
			if err := util.EnsureSafeSegment(r.ID, "rule id"); err != nil {
				addErr(base+".id", err.Error())
			}
			if ruleIDs[r.ID] {
				addErr(base+".id", fmt.Sprintf("duplicate rule id %q", r.ID))
			}
			ruleIDs[r.ID] = true
		}
		if strings.TrimSpace(r.Name) == "" {
			addErr(base+".name", "required")
		}
		tagNames := map[string]bool{}
		for j, tag := range r.Tags {
			tagPath := fmt.Sprintf("%s.tags[%d]", base, j)
			if strings.TrimSpace(tag) == "" {
				addErr(tagPath, "must not be empty")
				continue
			}
			if tagNames[tag] {
				addErr(tagPath, fmt.Sprintf("duplicate tag %q", tag))
			}
			tagNames[tag] = true
		}

		if len(r.Sources) == 0 {
			addErr(base+".sources", "at least one source is required")
		}
		for j, s := range r.Sources {
			sp := fmt.Sprintf("%s.sources[%d]", base, j)
			sourceType := s.SourceType()
			switch sourceType {
			case "url", "ref", "geosite", "local":
			case "":
				addErr(sp, "no recognized source type (need url, ref, geosite, or content)")
				continue
			default:
				addErr(sp+".type", fmt.Sprintf("unknown source type %q", sourceType))
				continue
			}

			selectors := 0
			if s.URL != "" {
				selectors++
			}
			if s.Ref != "" {
				selectors++
			}
			if s.Geosite != "" || s.Provider != "" || s.List != "" || len(s.Attrs) > 0 {
				selectors++
			}
			if s.Content != "" {
				selectors++
			}
			if s.File != "" {
				selectors++
			}
			if selectors > 1 {
				addErr(sp, "source must configure exactly one of url, ref, geosite, content, or file")
			}
			if s.Format != "" {
				if sourceType != "url" && sourceType != "local" {
					addErr(sp+".format", "format is only valid for url or local sources")
				} else if !ir.IsValidSourceFormat(s.Format) {
					addErr(sp+".format", fmt.Sprintf("unknown source format %q", s.Format))
				}
			}

			switch sourceType {
			case "url":
				if s.URL == "" {
					addErr(sp+".url", "required for url source")
				}
			case "ref":
				if s.Ref == "" {
					addErr(sp+".ref", "required for ref source")
				}
			case "local":
				if s.Content == "" && s.File == "" {
					addErr(sp, "local source requires content or file")
				}
			case "geosite":
				if s.Geosite != "" {
					if ref, err := geosite.ParseRef(s.Geosite); err != nil {
						addErr(sp+".geosite", err.Error())
					} else if !isSupportedGeositeProvider(ref.Provider) {
						addErr(sp+".geosite", fmt.Sprintf("unsupported geosite provider %q", ref.Provider))
					}
				} else if s.Provider == "" || s.List == "" {
					addErr(sp, "geosite source requires geosite ref or provider+list")
				} else {
					if !isSupportedGeositeProvider(s.Provider) {
						addErr(sp+".provider", fmt.Sprintf("unsupported geosite provider %q", s.Provider))
					}
					if err := util.EnsureSafeSegment(s.List, "geosite list"); err != nil {
						addErr(sp+".list", err.Error())
					}
					for k, attr := range s.Attrs {
						if err := util.EnsureSafeSegment(attr, "geosite attr"); err != nil {
							addErr(fmt.Sprintf("%s.attrs[%d]", sp, k), err.Error())
						}
					}
				}
			}
		}
		for k, out := range r.Outputs {
			if !clientIDs[out] {
				addErr(fmt.Sprintf("%s.outputs[%d]", base, k), fmt.Sprintf("unknown client %q", out))
			}
		}
		validateOps(base+".ops", r.Ops, addErr)
		if r.Merge != nil {
			switch r.Merge.Strategy {
			case "union", "intersect", "difference":
			default:
				addErr(base+".merge.strategy", fmt.Sprintf("unknown strategy %q", r.Merge.Strategy))
			}
		}
	}

	for i, r := range c.Rules {
		for j, s := range r.Sources {
			if s.SourceType() == "ref" && s.Ref != "" && !ruleIDs[s.Ref] {
				addErr(fmt.Sprintf("rules[%d].sources[%d].ref", i, j), fmt.Sprintf("unknown rule ID %q", s.Ref))
			}
		}
	}
	if cycle := DetectCircularDependency(c.Rules); cycle != nil {
		addErr("rules", fmt.Sprintf("circular dependency detected: %s", strings.Join(cycle, " -> ")))
	}

	switch c.Update.Schedule.Mode {
	case "manual", "interval", "cron":
	default:
		addErr("update.schedule.mode", fmt.Sprintf("must be manual, interval, or cron; got %q", c.Update.Schedule.Mode))
	}
	if c.Update.Schedule.Mode == "interval" {
		if time.Duration(c.Update.Schedule.Interval) <= 0 {
			addErr("update.schedule.interval", "must be a positive duration")
		}
	}
	if c.Update.Schedule.Mode == "cron" {
		if strings.TrimSpace(c.Update.Schedule.Cron) == "" {
			addErr("update.schedule.cron", "required for cron schedule")
		} else if _, err := cron.ParseStandard(c.Update.Schedule.Cron); err != nil {
			addErr("update.schedule.cron", err.Error())
		}
	}
	if _, err := time.LoadLocation(c.Update.Schedule.Timezone); err != nil {
		addErr("update.schedule.timezone", err.Error())
	}
	if time.Duration(c.Update.Fetch.Timeout) <= 0 {
		addErr("update.fetch.timeout", "must be a positive duration")
	}
	if int64(c.Update.Fetch.MaxDownload) <= 0 {
		addErr("update.fetch.max_download", "must be a positive size")
	}
	if c.Update.Fetch.Concurrency <= 0 || c.Update.Fetch.Concurrency > 64 {
		addErr("update.fetch.concurrency", "must be between 1 and 64")
	}
	if c.Update.Fetch.PerHostConcurrency <= 0 || c.Update.Fetch.PerHostConcurrency > c.Update.Fetch.Concurrency {
		addErr("update.fetch.per_host_concurrency", "must be between 1 and update.fetch.concurrency")
	}
	if c.Update.Fetch.Retries < 0 || c.Update.Fetch.Retries > 10 {
		addErr("update.fetch.retries", "must be between 0 and 10")
	}
	if time.Duration(c.Update.Fetch.RetryDelay) <= 0 {
		addErr("update.fetch.retry_delay", "must be a positive duration")
	}
	if time.Duration(c.Update.Preprocess.Timeout) <= 0 {
		addErr("update.preprocess.timeout", "must be a positive duration")
	}
	if int64(c.Update.Preprocess.MaxOutput) <= 0 {
		addErr("update.preprocess.max_output", "must be a positive size")
	}
	if time.Duration(c.Update.HistoryRetention) <= 0 {
		addErr("update.history_retention", "must be a positive duration")
	}
	if c.Update.HistoryLimit < 1 || c.Update.HistoryLimit > 10000 {
		addErr("update.history_limit", "must be between 1 and 10000")
	}
	if _, err := netip.ParseAddr(c.Serve.Host); err != nil {
		addErr("serve.host", fmt.Sprintf("must be an IP address: %v", err))
	}
	if c.Serve.Port < 1 || c.Serve.Port > 65535 {
		addErr("serve.port", "must be between 1 and 65535")
	}
	for i, p := range c.Serve.TrustedProxies {
		tp := strings.TrimSpace(p)
		if tp == "" {
			addErr(fmt.Sprintf("serve.trusted_proxies[%d]", i), "must not be empty")
			continue
		}
		if _, err := netip.ParsePrefix(tp); err != nil {
			if _, err2 := netip.ParseAddr(tp); err2 != nil {
				addErr(fmt.Sprintf("serve.trusted_proxies[%d]", i), fmt.Sprintf("invalid IP or CIDR: %v", err))
			}
		}
	}

	if c.Geosite != nil {
		providerNames := map[string]bool{}
		for i, p := range c.Geosite.Providers {
			base := fmt.Sprintf("geosite.providers[%d]", i)
			if p.Name == "" {
				addErr(base+".name", "required")
				continue
			}
			if !isSupportedGeositeProvider(p.Name) {
				addErr(base+".name", fmt.Sprintf("unsupported geosite provider %q", p.Name))
			}
			if err := util.EnsureSafeSegment(p.Name, "geosite provider"); err != nil {
				addErr(base+".name", err.Error())
			}
			if providerNames[p.Name] {
				addErr(base+".name", fmt.Sprintf("duplicate provider name %q", p.Name))
			}
			providerNames[p.Name] = true
			if len(p.Clients) == 0 {
				addErr(base+".clients", "at least one client is required")
			}
			for k, cl := range p.Clients {
				if !clientIDs[cl] {
					addErr(fmt.Sprintf("%s.clients[%d]", base, k), fmt.Sprintf("unknown client %q", cl))
				}
			}
		}
	}

	return errs
}

func validateOutputID(base, id, kind string, seen map[string]bool, addErr func(string, string)) {
	if id == "" {
		addErr(base+".id", "required")
		return
	}
	if err := util.EnsureSafeSegment(id, kind+" id"); err != nil {
		addErr(base+".id", err.Error())
	}
	if seen[id] {
		addErr(base+".id", fmt.Sprintf("duplicate output id %q", id))
	}
	seen[id] = true
}

func validateOps(base string, ops []OpConfig, addErr func(string, string)) {
	for j, op := range ops {
		opPath := fmt.Sprintf("%s[%d]", base, j)
		switch op.Type {
		case "include_kinds", "exclude_kinds":
			if len(op.Kinds) == 0 {
				addErr(opPath+".kinds", fmt.Sprintf("%s requires at least one kind", op.Type))
			}
			for k, kind := range op.Kinds {
				if !ir.IsValidKind(ir.Kind(kind)) {
					addErr(fmt.Sprintf("%s.kinds[%d]", opPath, k), fmt.Sprintf("unknown rule kind %q", kind))
				}
			}
		case "filter_values":
			if op.Pattern == "" {
				addErr(opPath+".pattern", "required for filter_values")
			}
			switch op.Mode {
			case "", "keyword", "suffix", "prefix", "exact", "regex":
			default:
				addErr(opPath+".mode", fmt.Sprintf("unknown filter mode %q", op.Mode))
			}
			if op.Mode == "regex" {
				if _, err := regexp.Compile(op.Pattern); err != nil {
					addErr(opPath+".pattern", fmt.Sprintf("invalid regex: %v", err))
				}
			}
		case "":
			addErr(opPath+".type", "required")
		default:
			addErr(opPath+".type", fmt.Sprintf("unknown op type %q", op.Type))
		}
	}
}

func isSupportedGeositeProvider(name string) bool {
	for _, supported := range geosite.SupportedProviders {
		if name == supported {
			return true
		}
	}
	return false
}

// ParseSize parses a human-readable size string like "4MB" into bytes.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	// Ordered longest-suffix-first to avoid "B" matching before "KB".
	suffixes := []struct {
		s string
		m int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.s) {
			numStr := strings.TrimSpace(strings.TrimSuffix(s, sf.s))
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
				return 0, fmt.Errorf("invalid size %q", s)
			}
			value := n * float64(sf.m)
			if value > math.MaxInt64 {
				return 0, fmt.Errorf("size %q is too large", s)
			}
			return int64(value), nil
		}
	}
	return 0, fmt.Errorf("invalid size %q (use B, KB, MB, or GB suffix)", s)
}
