// Package config defines the YAML configuration model for prm, including
// loader and validator.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/geohost"
	"github.com/fl0w1nd/proxy-rule-manager/internal/geoip"
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
		return fmt.Errorf("line %d: duration must be a string like \"15s\"", value.Line)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: invalid duration %q: %w", value.Line, s, err)
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
			return fmt.Errorf("line %d: %w", value.Line, err)
		}
		*s = Size(n)
		return nil
	}
	// Fall back to plain integer (bytes).
	var n int64
	if err := value.Decode(&n); err != nil {
		return fmt.Errorf("line %d: size must be a string like \"4MB\" or an integer byte count", value.Line)
	}
	*s = Size(n)
	return nil
}

// Config is the root configuration loaded from config.yaml.
type Config struct {
	Clients []ClientConfig `yaml:"clients"`
	Rules   []RuleConfig   `yaml:"rules"`

	Geosite *GeositeConfig   `yaml:"geosite,omitempty"`
	GeoIP   *GeoIPConfig     `yaml:"geoip,omitempty"`
	MMDB    *HostedGeoConfig `yaml:"mmdb,omitempty"`
	ASN     *HostedGeoConfig `yaml:"asn,omitempty"`
	Update  UpdateConfig     `yaml:"update"`

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
	Group      []SourceConfig `yaml:"group,omitempty"`
	Preprocess *string        `yaml:"preprocess,omitempty"`
	Ops        []OpConfig     `yaml:"ops,omitempty"`
	URL        string         `yaml:"url,omitempty"`
	Type       string         `yaml:"type,omitempty"`
	Ref        string         `yaml:"ref,omitempty"` // referenced rule ID
	Content    string         `yaml:"content,omitempty"`
	File       string         `yaml:"file,omitempty"`
	Label      string         `yaml:"label,omitempty"`

	// Geosite compact ref: "provider/list" or "provider/list@attr1,attr2"
	// Preferred over separate Provider/List/Attrs fields.
	Geosite  string   `yaml:"geosite,omitempty"`
	GeoIP    string   `yaml:"geoip,omitempty"`
	Provider string   `yaml:"provider,omitempty"`
	List     string   `yaml:"list,omitempty"`
	Attrs    []string `yaml:"attrs,omitempty"`
}

// SourceType returns the canonical source type.
func (s *SourceConfig) SourceType() string {
	if s.Group != nil {
		return "group"
	}
	if s.Type != "" {
		return s.Type
	}
	if s.URL != "" {
		return "url"
	}
	if s.Ref != "" {
		return "ref"
	}
	if s.GeoIP != "" {
		return "geoip"
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

// ResolveGeoIPRef returns a provider/list reference.
func (s *SourceConfig) ResolveGeoIPRef() (geoip.Ref, error) {
	if len(s.Attrs) > 0 {
		return geoip.Ref{}, fmt.Errorf("geoip sources do not support attributes")
	}
	if s.GeoIP != "" {
		return geoip.ParseRef(s.GeoIP)
	}
	return geoip.ParseRef(s.Provider + "/" + s.List)
}

// GeoIPConfig publishes all IP lists to the selected clients.
type GeoIPConfig struct {
	Providers []GeoIPProvider `yaml:"providers"`
}
type GeoIPProvider struct {
	Name    string   `yaml:"name"`
	Clients []string `yaml:"clients"`
	Host    bool     `yaml:"host,omitempty"`
}

// HostedGeoConfig downloads original geo database files for the public site.
type HostedGeoConfig struct {
	Providers []HostedGeoProvider `yaml:"providers"`
}

// HostedGeoProvider names one upstream database published as a raw file.
type HostedGeoProvider struct {
	Name string `yaml:"name"`
	Host bool   `yaml:"host,omitempty"`
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
	Host    bool     `yaml:"host,omitempty"`
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

// Defaults fills in zero values with sensible defaults.
func (c *Config) Defaults() {
	shouldDefault := func(path string) bool {
		return c.positions == nil || !c.positions.Has(path)
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
		c.Update.Fetch.MaxDownload = Size(50 * 1024 * 1024)
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
}

// Load reads and parses a YAML config file and applies defaults. The returned
// Config retains YAML source positions so that Validate() can report
// line-precise errors.
func Load(path, dataDir string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg, _, err := decodeDocument(data, dataDir)
	return cfg, err
}

// decodeDocument parses raw YAML into the runtime configuration and retains
// the document used for source-preserving edits.
func decodeDocument(data []byte, dataDir string) (*Config, *yaml.Node, error) {
	var source yaml.Node
	if err := yaml.Unmarshal(data, &source); err != nil {
		return nil, nil, documentError(err, nil)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, nil, documentError(err, &source)
	}
	if len(source.Content) == 0 || source.Content[0].Kind != yaml.MappingNode {
		return nil, nil, ConfigErrors{{Path: "config", Line: 1, Message: "configuration must be a YAML mapping"}}
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, nil, documentError(err, nil)
		}
		return nil, nil, ConfigErrors{{Path: "config", Line: extra.Line, Message: "must contain exactly one YAML document"}}
	}
	cfg.positions = BuildPositionIndex(&source)
	cfg.Defaults()

	if errs := cfg.Validate(dataDir); len(errs) > 0 {
		return nil, nil, ConfigErrors(errs)
	}
	return &cfg, &source, nil
}

// Validate checks the config for structural correctness. Returns a list of
// errors, each carrying the YAML path and line number of the offending node.
// An empty return means the config is valid.
func (c *Config) Validate(dataDir string) []ConfigError {
	var errs []ConfigError
	pos := c.positions

	addErr := func(path, msg string) {
		p := Position{}
		if pos != nil {
			p = pos.Lookup(path)
		}
		errs = append(errs, ConfigError{Path: path, Line: p.Line, Message: msg})
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
		for path, s := range WalkSources(r.Sources) {
			sp := base + "." + path
			validateOps(sp+".ops", s.Ops, addErr)
			if s.Group != nil {
				if strings.Contains(path, ".group[") {
					addErr(sp, "source groups cannot be nested")
				}
				if len(s.Group) == 0 {
					addErr(sp+".group", "at least one source is required")
				}
				if s.URL != "" || s.File != "" || s.Content != "" || s.Ref != "" || s.Geosite != "" || s.GeoIP != "" || s.Provider != "" || s.List != "" || len(s.Attrs) > 0 || s.Type != "" {
					addErr(sp, "group cannot configure a source selector or type")
				}
				for k, child := range s.Group {
					if child.Preprocess != nil || len(child.Ops) > 0 {
						addErr(fmt.Sprintf("%s.group[%d]", sp, k), "configure preprocessing and ops on the group")
					}
				}
				continue
			}
			if s.Preprocess != nil && *s.Preprocess != "" && s.SourceType() != "url" && s.SourceType() != "local" {
				addErr(sp+".preprocess", "preprocessing requires a url or local source")
			}
			sourceType := s.SourceType()
			switch sourceType {
			case "url", "ref", "geosite", "geoip", "local":
			case "":
				addErr(sp, "no recognized source type (need url, ref, geosite, geoip, or content)")
				continue
			default:
				addErr(sp+".type", fmt.Sprintf("unknown source type %q", sourceType))
				continue
			}

			selectors := 0
			if s.GeoIP != "" {
				selectors++
			}
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
				addErr(sp, "source must configure exactly one of url, ref, geosite, geoip, content, or file")
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
				if s.File != "" {
					if _, err := NewLocalFileResolver(dataDir)(s.File); err != nil {
						addErr(sp+".file", err.Error())
					}
				}
			case "geoip":
				if _, err := s.ResolveGeoIPRef(); err != nil {
					addErr(sp+".geoip", err.Error())
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
		for path, s := range WalkSources(r.Sources) {
			if s.SourceType() == "ref" && s.Ref != "" && !ruleIDs[s.Ref] {
				addErr(fmt.Sprintf("rules[%d].%s.ref", i, path), fmt.Sprintf("unknown rule ID %q", s.Ref))
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
	validateHostedGeoProviders("mmdb", c.MMDB, isSupportedMMDBProvider, addErr)
	validateHostedGeoProviders("asn", c.ASN, isSupportedASNProvider, addErr)
	if c.GeoIP != nil {
		providerNames := map[string]bool{}
		for i, p := range c.GeoIP.Providers {
			base := fmt.Sprintf("geoip.providers[%d]", i)
			if p.Name == "" {
				addErr(base+".name", "required")
				continue
			}
			if !isSupportedGeoIPProvider(p.Name) {
				addErr(base+".name", fmt.Sprintf("unsupported geoip provider %q", p.Name))
			}
			if err := util.EnsureSafeSegment(p.Name, "geoip provider"); err != nil {
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

func isSupportedGeoIPProvider(name string) bool {
	for _, supported := range geoip.SupportedProviders {
		if name == supported {
			return true
		}
	}
	return false
}

func isSupportedMMDBProvider(name string) bool {
	return geohost.Supports(geohost.KindMMDB, name)
}

func isSupportedASNProvider(name string) bool {
	return geohost.Supports(geohost.KindASN, name)
}

func validateHostedGeoProviders(kind string, cfg *HostedGeoConfig, supported func(string) bool, addErr func(string, string)) {
	if cfg == nil {
		return
	}
	providerNames := map[string]bool{}
	for i, p := range cfg.Providers {
		base := fmt.Sprintf("%s.providers[%d]", kind, i)
		if p.Name == "" {
			addErr(base+".name", "required")
			continue
		}
		if !supported(p.Name) {
			addErr(base+".name", fmt.Sprintf("unsupported %s provider %q", kind, p.Name))
		}
		if err := util.EnsureSafeSegment(p.Name, kind+" provider"); err != nil {
			addErr(base+".name", err.Error())
		}
		if providerNames[p.Name] {
			addErr(base+".name", fmt.Sprintf("duplicate provider name %q", p.Name))
		}
		providerNames[p.Name] = true
	}
}
