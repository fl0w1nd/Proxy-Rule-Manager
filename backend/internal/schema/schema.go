// Package schema mirrors src/lib/schema.ts so that JSON payloads match the
// frontend's TypeScript types exactly. Every field below intentionally uses the
// same JSON name as the TS Zod schema.
package schema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ValidationError is the sentinel type returned by all ValidateXyz helpers.
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s %q: %s", e.Field, e.Value, e.Message)
}

func validationErr(field, value string, allowed []string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: fmt.Sprintf("must be one of %v", allowed),
	}
}

// ClientConfig describes one proxy client and its global transforms.
type ClientConfig struct {
	ID          string      `json:"id"`
	DisplayName string      `json:"displayName"`
	Transforms  []Transform `json:"transforms,omitempty"`
}

// ClientFileMeta is the metadata for a per-client published configuration file.
type ClientFileMeta struct {
	ID          string  `json:"id"`
	ClientID    string  `json:"clientId"`
	ConfigID    string  `json:"configId"`
	DisplayName string  `json:"displayName"`
	Description *string `json:"description,omitempty"`
	Ext         string  `json:"ext"`
	IsPublic    bool    `json:"isPublic"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// configIDRe mirrors the TS regex /^[a-zA-Z0-9_-]+$/.
var configIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateConfigID checks that s matches the configId allow-list.
// Routes should call this before persisting or using a configId from user input.
func ValidateConfigID(s string) error {
	if !configIDRe.MatchString(s) {
		return &ValidationError{
			Field:   "configId",
			Value:   s,
			Message: "must only contain letters, numbers, hyphens, and underscores",
		}
	}
	return nil
}

// Validate checks that the configId field satisfies the TS schema constraint.
func (m *ClientFileMeta) Validate() error {
	return ValidateConfigID(m.ConfigID)
}

// DefaultClients matches DEFAULT_CLIENTS in TS.
var DefaultClients = []ClientConfig{
	{ID: "clash_meta", DisplayName: "Clash Meta / Stash"},
	{ID: "shadowrocket", DisplayName: "Shadowrocket"},
}

// SourceConfig matches the union of url/ref/local/geosite sources.
type SourceConfig struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
	Ref  string `json:"ref,omitempty"`
	// Content uses *string without omitempty so that nil serialises as JSON null.
	// Routes that delete local-source content must be able to write null, not omit.
	Content       *string  `json:"content"`
	ContentRef    string   `json:"contentRef,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	List          string   `json:"list,omitempty"`
	Attrs         []string `json:"attrs,omitempty"`
	RenderProfile string   `json:"renderProfile,omitempty"`
	Name          string   `json:"name,omitempty"`
}

// SourceType returns the effective type, defaulting to "url".
func (s SourceConfig) SourceType() string {
	if s.Type == "" {
		return "url"
	}
	return s.Type
}

// Transform represents a single post-processing operation.
// Target always serialises (no omitempty) to match the TS default of "all".
type Transform struct {
	Type        string          `json:"type"`
	Target      json.RawMessage `json:"target"`
	Use         string          `json:"use,omitempty"`
	Pattern     string          `json:"pattern,omitempty"`
	Replacement string          `json:"replacement,omitempty"`
	Flags       string          `json:"flags,omitempty"`
}

// TargetIndices resolves the target field, returning the list of indices the
// transform applies to. If target is "all" (or missing), it returns nil to
// signify "all indices".
func (t Transform) TargetIndices() (indices []int, all bool, err error) {
	if len(t.Target) == 0 {
		return nil, true, nil
	}
	trimmed := strings.TrimSpace(string(t.Target))
	if trimmed == "" || trimmed == "null" {
		return nil, true, nil
	}
	if trimmed == "\"all\"" {
		return nil, true, nil
	}
	if trimmed[0] == '[' {
		var arr []int
		if err := json.Unmarshal(t.Target, &arr); err != nil {
			return nil, false, err
		}
		return arr, false, nil
	}
	return nil, false, fmt.Errorf("invalid transform target: %s", trimmed)
}

// MergeConfig describes how multiple sources are merged.
type MergeConfig struct {
	// omitempty intentionally absent: TS schema always emits strategy and dedupe.
	Strategy string `json:"strategy"`
	Dedupe   bool   `json:"dedupe"`
}

// EnsureDefaults sets strategy to "concat" when empty (matching TS default).
func (m *MergeConfig) EnsureDefaults() {
	if m.Strategy == "" {
		m.Strategy = "concat"
	}
}

// EffectiveStrategy returns the strategy or the default "concat".
func (m *MergeConfig) EffectiveStrategy() string {
	if m == nil || m.Strategy == "" {
		return "concat"
	}
	return m.Strategy
}

// EffectiveDedupe returns dedupe or false default.
func (m *MergeConfig) EffectiveDedupe() bool {
	if m == nil {
		return false
	}
	return m.Dedupe
}

// ClientOutputOverride is a per-rule per-client override.
//
// Both Enabled and UseGlobalTransforms are plain bool (always present in JSON)
// with a TS-matching default of true. Because Go's zero value for bool is false,
// we use a custom UnmarshalJSON that applies the true default for absent keys, and
// an explicit EnsureDefaults() for values constructed in Go code.
// Transforms always serialises (no omitempty) to match the TS default of [].
type ClientOutputOverride struct {
	Enabled             bool        `json:"enabled"`
	UseGlobalTransforms bool        `json:"useGlobalTransforms"`
	Transforms          []Transform `json:"transforms"`
}

// UnmarshalJSON applies TS defaults: enabled=true and useGlobalTransforms=true
// when the respective keys are absent from the JSON object.
func (c *ClientOutputOverride) UnmarshalJSON(data []byte) error {
	type raw struct {
		Enabled             *bool       `json:"enabled"`
		UseGlobalTransforms *bool       `json:"useGlobalTransforms"`
		Transforms          []Transform `json:"transforms"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if r.Enabled != nil {
		c.Enabled = *r.Enabled
	} else {
		c.Enabled = true
	}
	if r.UseGlobalTransforms != nil {
		c.UseGlobalTransforms = *r.UseGlobalTransforms
	} else {
		c.UseGlobalTransforms = true
	}
	if r.Transforms != nil {
		c.Transforms = r.Transforms
	} else {
		c.Transforms = []Transform{}
	}
	return nil
}

// EnsureDefaults initialises Enabled and UseGlobalTransforms to true,
// and Transforms to an empty slice (matching TS default of []).
// Call this when constructing an override in Go code rather than decoding from JSON.
func (c *ClientOutputOverride) EnsureDefaults() {
	c.Enabled = true
	c.UseGlobalTransforms = true
	if c.Transforms == nil {
		c.Transforms = []Transform{}
	}
}

// IsEnabled reports whether the override is enabled; default true.
func (c *ClientOutputOverride) IsEnabled() bool {
	if c == nil {
		return true
	}
	return c.Enabled
}

// ShouldUseGlobalTransforms reports the effective flag; default true.
func (c *ClientOutputOverride) ShouldUseGlobalTransforms() bool {
	if c == nil {
		return true
	}
	return c.UseGlobalTransforms
}

// OutputConfig matches OutputConfigSchema.
type OutputConfig struct {
	Clients         []string                        `json:"clients"`
	ClientOverrides map[string]ClientOutputOverride `json:"client_overrides,omitempty"`
}

// RuleConfig matches RuleConfigSchema.
type RuleConfig struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName,omitempty"`
	Description string         `json:"description,omitempty"`
	Icon        string         `json:"icon,omitempty"`
	Sources     []SourceConfig `json:"sources,omitempty"`
	Transforms  []Transform    `json:"transforms,omitempty"`
	Merge       *MergeConfig   `json:"merge,omitempty"`
	Output      OutputConfig   `json:"output"`
	// omitempty intentionally absent: TS schema always emits tags (default []).
	Tags []string `json:"tags"`
}

// EnsureDefaults normalises nil slices and nested structs to their TS defaults.
func (r *RuleConfig) EnsureDefaults() {
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if r.Merge != nil {
		r.Merge.EnsureDefaults()
	}
}

// ScriptTransformer is a predefined JS-script transformer.
type ScriptTransformer struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Script      string `json:"script"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// RulesConfig is the full orchestration document persisted under config.config_json.
type RulesConfig struct {
	Version      int                          `json:"version"`
	Transformers map[string]ScriptTransformer `json:"transformers"`
	Rules        []RuleConfig                 `json:"rules"`
}

// EnsureDefaults populates defaults (version=1, non-nil maps/slices).
func (r *RulesConfig) EnsureDefaults() {
	if r.Version == 0 {
		r.Version = 1
	}
	if r.Transformers == nil {
		r.Transformers = map[string]ScriptTransformer{}
	}
	if r.Rules == nil {
		r.Rules = []RuleConfig{}
	}
	for i := range r.Rules {
		r.Rules[i].EnsureDefaults()
		ensureTransformDefaults(r.Rules[i].Transforms)
	}
}

// ensureTransformDefaults fills nil Target fields with the JSON literal "all".
func ensureTransformDefaults(transforms []Transform) {
	for i := range transforms {
		if len(transforms[i].Target) == 0 {
			transforms[i].Target = json.RawMessage(`"all"`)
		}
	}
}

// DefaultConfig returns the empty default.
func DefaultConfig() RulesConfig {
	return RulesConfig{Version: 1, Transformers: map[string]ScriptTransformer{}, Rules: []RuleConfig{}}
}

// ArtifactMeta represents a produced rule artifact for a (rule, client) pair.
type ArtifactMeta struct {
	RuleName      string `json:"ruleName"`
	Client        string `json:"client"`
	LastHash      string `json:"lastHash"`
	LastUpdatedAt string `json:"lastUpdatedAt"`
	BlobPath      string `json:"blobPath"`
	BlobURL       string `json:"blobUrl,omitempty"`
	SizeBytes     *int64 `json:"sizeBytes,omitempty"`
	// LastAttemptedAt records the timestamp of the most recent sync attempt
	// regardless of outcome; empty for legacy rows that were never updated by
	// the post-B1 engine. LastAttemptStatus is "success", "failed", or "".
	LastAttemptedAt   string `json:"lastAttemptedAt,omitempty"`
	LastAttemptStatus string `json:"lastAttemptStatus,omitempty"`
	LastAttemptError  string `json:"lastAttemptError,omitempty"`
	// ConsecutiveFailures counts the number of failed sync attempts since
	// the most recent successful publish for this (rule, client) pair. Reset
	// to 0 on any successful sync. Used by the dashboard to flag rules whose
	// upstream has been broken for too long (vs. a single transient blip).
	ConsecutiveFailures int `json:"consecutiveFailures"`
}

// JobFailedRule mirrors the failedRules entry inside a job.
type JobFailedRule struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// JobRecord matches JobRecordSchema.
type JobRecord struct {
	JobID         string          `json:"jobId"`
	Type          string          `json:"type"`
	Status        string          `json:"status"`
	StartedAt     string          `json:"startedAt"`
	CompletedAt   *string         `json:"completedAt,omitempty"`
	AffectedRules []string        `json:"affectedRules,omitempty"`
	ChangedRules  []string        `json:"changedRules,omitempty"`
	FailedRules   []JobFailedRule `json:"failedRules,omitempty"`
	Logs          []string        `json:"logs,omitempty"`
}

// DailyStats matches DailyStatsSchema.
type DailyStats struct {
	Date                string `json:"date"`
	SyncCount           int64  `json:"syncCount"`
	BlobWriteCount      int64  `json:"blobWriteCount"`
	RulesChanged        int64  `json:"rulesChanged"`
	TotalRulesProcessed int64  `json:"totalRulesProcessed"`
	FailedSources       int64  `json:"failedSources"`
}

// LastSyncInfo matches LastSyncInfo in storage-adapter.ts.
//
// LastSyncDurationMs is the wall-clock duration of the most recent full
// sync, in milliseconds. Pointer so older backups (and partial-only stores)
// JSON-decode as nil instead of falsely reporting "0 ms".
type LastSyncInfo struct {
	LastFullSyncAt       *string `json:"lastFullSyncAt"`
	LastPartialSyncAt    *string `json:"lastPartialSyncAt"`
	LastSuccessfulSyncAt *string `json:"lastSuccessfulSyncAt"`
	TotalRulesCount      int64   `json:"totalRulesCount"`
	ChangedRulesCount    int64   `json:"changedRulesCount"`
	FailedRulesCount     int64   `json:"failedRulesCount"`
	LastSyncDurationMs   *int64  `json:"lastSyncDurationMs,omitempty"`
}

// DefaultLastSyncInfo returns the zero value used at first boot.
func DefaultLastSyncInfo() LastSyncInfo {
	return LastSyncInfo{}
}

// SyncSchedule matches SyncScheduleSchema.
type SyncSchedule struct {
	Mode                string  `json:"mode"`
	IntervalHours       int     `json:"intervalHours"`
	CronExpression      string  `json:"cronExpression,omitempty"`
	LastScheduledSyncAt *string `json:"lastScheduledSyncAt,omitempty"`
	NextSyncAt          *string `json:"nextSyncAt,omitempty"`
}

// DefaultSyncSchedule mirrors DEFAULT_SYNC_SCHEDULE.
func DefaultSyncSchedule() SyncSchedule {
	return SyncSchedule{
		Mode:           "interval",
		IntervalHours:  24,
		CronExpression: "0 0 * * *",
	}
}

// CdnCustomHeader matches { name, value }.
type CdnCustomHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CdnSettings matches CdnSettingsSchema.
type CdnSettings struct {
	Enabled                   bool              `json:"enabled"`
	CacheMode                 string            `json:"cacheMode"`
	StaleIfErrorSeconds       int64             `json:"staleIfErrorSeconds"`
	CustomCacheControl        string            `json:"customCacheControl,omitempty"`
	CloudflareCdnCacheControl string            `json:"cloudflareCdnCacheControl,omitempty"`
	CustomHeaders             []CdnCustomHeader `json:"customHeaders"`
}

// DefaultCdnSettings mirrors DEFAULT_CDN_SETTINGS.
func DefaultCdnSettings() CdnSettings {
	return CdnSettings{
		Enabled:             false,
		CacheMode:           "no-cache",
		StaleIfErrorSeconds: 604800,
		CustomHeaders:       []CdnCustomHeader{},
	}
}

// SystemSettings groups the runtime knobs an admin can tune from the UI.
// All values are positive integers; zero is treated as "use built-in default"
// at apply time so that older backups (without the field) keep working.
type SystemSettings struct {
	Fetch       FetchSettings       `json:"fetch"`
	Transformer TransformerSettings `json:"transformer"`
	RateLimit   RateLimitSettings   `json:"rateLimit"`
	Sync        SyncSettings        `json:"sync"`
}

// FetchSettings controls the URL fetcher used by the sync engine.
type FetchSettings struct {
	TimeoutSeconds     int    `json:"timeoutSeconds"`
	MaxDownloadMB      int    `json:"maxDownloadMB"`
	PerHostConcurrency int    `json:"perHostConcurrency"`
	UserAgent          string `json:"userAgent,omitempty"`
}

// TransformerSettings controls the JS transformer sandbox.
type TransformerSettings struct {
	TimeoutMs   int `json:"timeoutMs"`
	MaxOutputMB int `json:"maxOutputMB"`
}

// RateLimitSettings controls the admin-auth rate limiter (login attempts).
type RateLimitSettings struct {
	BaseDelaySeconds  int `json:"baseDelaySeconds"`
	MaxBlockSeconds   int `json:"maxBlockSeconds"`
	PermanentBanLimit int `json:"permanentBanLimit"`
	RecordMaxAgeHours int `json:"recordMaxAgeHours"`
}

// SyncSettings controls how the dashboard interprets sync-attempt history.
// FailureThreshold is the consecutive-failure count at which a rule starts
// rendering the "更新失败" badge. Set low to catch outages early; set high
// to tolerate noisy upstreams that occasionally 5xx.
type SyncSettings struct {
	FailureThreshold int `json:"failureThreshold"`
}

// DefaultSystemSettings returns the values the codebase historically hard-coded.
// Any new field added here must keep parity with the corresponding constant
// elsewhere in the backend (fetcher.go / js.go / auth.go).
func DefaultSystemSettings() SystemSettings {
	return SystemSettings{
		Fetch: FetchSettings{
			TimeoutSeconds:     15,
			MaxDownloadMB:      4,
			PerHostConcurrency: 4,
			UserAgent:          "Proxy-Rule-Manager/1.0",
		},
		Transformer: TransformerSettings{
			TimeoutMs:   5000,
			MaxOutputMB: 8,
		},
		RateLimit: RateLimitSettings{
			BaseDelaySeconds:  5,
			MaxBlockSeconds:   3600,
			PermanentBanLimit: 10,
			RecordMaxAgeHours: 24,
		},
		Sync: SyncSettings{
			FailureThreshold: 3,
		},
	}
}

// MergeDefaults rewrites zero-valued fields with the built-in default.
// Called both on read (so old envelopes with missing keys round-trip) and
// on write (so users can clear a single field by sending 0).
func (s *SystemSettings) MergeDefaults() {
	d := DefaultSystemSettings()
	if s.Fetch.TimeoutSeconds <= 0 {
		s.Fetch.TimeoutSeconds = d.Fetch.TimeoutSeconds
	}
	if s.Fetch.MaxDownloadMB <= 0 {
		s.Fetch.MaxDownloadMB = d.Fetch.MaxDownloadMB
	}
	if s.Fetch.PerHostConcurrency <= 0 {
		s.Fetch.PerHostConcurrency = d.Fetch.PerHostConcurrency
	}
	if s.Fetch.UserAgent == "" {
		s.Fetch.UserAgent = d.Fetch.UserAgent
	}
	if s.Transformer.TimeoutMs <= 0 {
		s.Transformer.TimeoutMs = d.Transformer.TimeoutMs
	}
	if s.Transformer.MaxOutputMB <= 0 {
		s.Transformer.MaxOutputMB = d.Transformer.MaxOutputMB
	}
	if s.RateLimit.BaseDelaySeconds <= 0 {
		s.RateLimit.BaseDelaySeconds = d.RateLimit.BaseDelaySeconds
	}
	if s.RateLimit.MaxBlockSeconds <= 0 {
		s.RateLimit.MaxBlockSeconds = d.RateLimit.MaxBlockSeconds
	}
	if s.RateLimit.PermanentBanLimit <= 0 {
		s.RateLimit.PermanentBanLimit = d.RateLimit.PermanentBanLimit
	}
	if s.RateLimit.RecordMaxAgeHours <= 0 {
		s.RateLimit.RecordMaxAgeHours = d.RateLimit.RecordMaxAgeHours
	}
	if s.Sync.FailureThreshold <= 0 {
		s.Sync.FailureThreshold = d.Sync.FailureThreshold
	}
}

// Validate enforces sane upper bounds so an admin can't accidentally lock
// themselves out (e.g. a 100ms script timeout) or DoS the backend (e.g. a
// 1 GB transformer output cap). Mirrors the limits exposed in the UI.
func (s SystemSettings) Validate() error {
	check := func(field string, v, lo, hi int, suffix string) error {
		if v < lo || v > hi {
			return validationErr(field, fmt.Sprint(v), []string{fmt.Sprintf("%d..%d%s", lo, hi, suffix)})
		}
		return nil
	}
	if err := check("fetch.timeoutSeconds", s.Fetch.TimeoutSeconds, 1, 600, "s"); err != nil {
		return err
	}
	if err := check("fetch.maxDownloadMB", s.Fetch.MaxDownloadMB, 1, 256, "MB"); err != nil {
		return err
	}
	if err := check("fetch.perHostConcurrency", s.Fetch.PerHostConcurrency, 1, 64, ""); err != nil {
		return err
	}
	if l := len(s.Fetch.UserAgent); l < 1 || l > 200 {
		return validationErr("fetch.userAgent", fmt.Sprintf("%d chars", l), []string{"1..200 chars"})
	}
	if err := check("transformer.timeoutMs", s.Transformer.TimeoutMs, 100, 60000, "ms"); err != nil {
		return err
	}
	if err := check("transformer.maxOutputMB", s.Transformer.MaxOutputMB, 1, 256, "MB"); err != nil {
		return err
	}
	if err := check("rateLimit.baseDelaySeconds", s.RateLimit.BaseDelaySeconds, 1, 600, "s"); err != nil {
		return err
	}
	if err := check("rateLimit.maxBlockSeconds", s.RateLimit.MaxBlockSeconds, 60, 86400, "s"); err != nil {
		return err
	}
	if err := check("rateLimit.permanentBanLimit", s.RateLimit.PermanentBanLimit, 1, 1000, ""); err != nil {
		return err
	}
	if err := check("rateLimit.recordMaxAgeHours", s.RateLimit.RecordMaxAgeHours, 1, 720, "h"); err != nil {
		return err
	}
	if err := check("sync.failureThreshold", s.Sync.FailureThreshold, 1, 50, ""); err != nil {
		return err
	}
	return nil
}

// BanRecord matches the IP ban data shape.
type BanRecord struct {
	IP        string  `json:"ip"`
	Reason    string  `json:"reason"`
	BannedAt  string  `json:"bannedAt"`
	ExpiresAt *string `json:"expiresAt"`
	FailCount int64   `json:"failCount"`
}

// ChangeRecordSummary matches the activity change summary shape.
type ChangeRecordSummary struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	RuleName   string `json:"ruleName"`
	Client     string `json:"client"`
	ChangeType string `json:"changeType"`
	SizeBytes  *int64 `json:"sizeBytes,omitempty"`
	Date       string `json:"date"`
	FileName   string `json:"fileName"`
}

// FailureRecord matches the activity failure record shape.
type FailureRecord struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	RuleName  string `json:"ruleName"`
	Client    string `json:"client,omitempty"`
	Source    string `json:"source,omitempty"`
	Message   string `json:"message"`
	Stage     string `json:"stage"`
	JobID     string `json:"jobId,omitempty"`
}

// ActivityList is the generic paginated wrapper.
type ActivityList[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// ---------------------------------------------------------------------------
// Enum validator helpers – route handlers should call these on incoming values.
// Each returns a *ValidationError (implements error) on failure, nil on success.
// ---------------------------------------------------------------------------

var allowedSourceTypes = []string{"url", "ref", "local", "geosite"}

// ValidateSourceType checks that s is a valid SourceConfig.type value.
func ValidateSourceType(s string) error {
	for _, v := range allowedSourceTypes {
		if s == v {
			return nil
		}
	}
	return validationErr("sourceType", s, allowedSourceTypes)
}

var allowedGeositeProviders = []string{"v2fly", "loyalsoldier"}

// ValidateGeositeProvider checks that s is a valid geosite provider.
func ValidateGeositeProvider(s string) error {
	for _, v := range allowedGeositeProviders {
		if s == v {
			return nil
		}
	}
	return validationErr("provider", s, allowedGeositeProviders)
}

var allowedGeositeRenderProfiles = []string{"mihomo-classical"}

// ValidateGeositeRenderProfile checks that s is a valid geosite renderProfile.
func ValidateGeositeRenderProfile(s string) error {
	for _, v := range allowedGeositeRenderProfiles {
		if s == v {
			return nil
		}
	}
	return validationErr("renderProfile", s, allowedGeositeRenderProfiles)
}

var allowedTransformTypes = []string{"use", "replace", "remove_lines"}

// ValidateTransformType checks that s is a valid Transform.type value.
func ValidateTransformType(s string) error {
	for _, v := range allowedTransformTypes {
		if s == v {
			return nil
		}
	}
	return validationErr("transformType", s, allowedTransformTypes)
}

var allowedMergeStrategies = []string{"concat", "union", "intersect"}

// ValidateMergeStrategy checks that s is a valid MergeConfig.strategy value.
func ValidateMergeStrategy(s string) error {
	for _, v := range allowedMergeStrategies {
		if s == v {
			return nil
		}
	}
	return validationErr("mergeStrategy", s, allowedMergeStrategies)
}

var allowedJobTypes = []string{"full_sync", "partial_sync"}

// ValidateJobType checks that s is a valid JobRecord.type value.
// reserved: no current route call site; kept for future job-filter endpoints.
func ValidateJobType(s string) error {
	for _, v := range allowedJobTypes {
		if s == v {
			return nil
		}
	}
	return validationErr("jobType", s, allowedJobTypes)
}

var allowedJobStatuses = []string{"pending", "running", "completed", "failed"}

// ValidateJobStatus checks that s is a valid JobRecord.status value.
// reserved: no current route call site; kept for future job-filter endpoints.
func ValidateJobStatus(s string) error {
	for _, v := range allowedJobStatuses {
		if s == v {
			return nil
		}
	}
	return validationErr("jobStatus", s, allowedJobStatuses)
}

// allowedCacheModes matches TS: "no-cache" | "no-store" | "custom".
// Note: "transparent" is NOT a valid value.
var allowedCacheModes = []string{"no-cache", "no-store", "custom"}

// ValidateCacheMode checks that s is a valid CdnSettings.cacheMode value.
func ValidateCacheMode(s string) error {
	for _, v := range allowedCacheModes {
		if s == v {
			return nil
		}
	}
	return validationErr("cacheMode", s, allowedCacheModes)
}

var allowedChangeTypes = []string{"created", "updated", "deleted"}

// ValidateChangeType checks that s is a valid ChangeRecordSummary.changeType value.
// reserved: no current route call site; kept for future activity-filter endpoints.
func ValidateChangeType(s string) error {
	for _, v := range allowedChangeTypes {
		if s == v {
			return nil
		}
	}
	return validationErr("changeType", s, allowedChangeTypes)
}
