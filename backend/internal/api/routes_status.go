package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/syncengine"
)

func (s *Server) registerStatusRoutes(r chi.Router) {
	r.Get("/status", s.handleStatus)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip := s.IP(r)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		blocked, retryAfter, _, err := s.RateLimiter.IsBlocked(ctx, s.Store, ip)
		if err != nil {
			s.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if blocked {
			if retryAfter <= 0 {
				retryAfter = 60
			}
			w.Header().Set("Retry-After", itoa(retryAfter))
			s.ErrorWith(w, http.StatusTooManyRequests, map[string]any{
				"error":      "Too many failed attempts",
				"retryAfter": retryAfter,
			})
			return
		}
		if !s.VerifyAdmin(authHeader) {
			_ = s.RateLimiter.RecordFailure(ctx, s.Store, ip)
			s.Error(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		s.RateLimiter.Clear(ip)
	}

	auth := s.CheckAuth(authHeader)
	isAdmin := auth == AuthAdmin

	cfg, err := s.Store.GetConfigRaw(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	clients, err := s.Store.GetClients(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	arts, err := s.Store.GetAllArtifactMetas(ctx)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	artsByRule := map[string][]schema.ArtifactMeta{}
	for _, a := range arts {
		artsByRule[a.RuleName] = append(artsByRule[a.RuleName], a)
	}

	type publicRule struct {
		Name                string   `json:"name"`
		DisplayName         string   `json:"displayName,omitempty"`
		Description         string   `json:"description,omitempty"`
		Icon                string   `json:"icon,omitempty"`
		Tags                []string `json:"tags"`
		Clients             []string `json:"clients"`
		LastUpdated         *string  `json:"lastUpdated"`
		HasError            bool     `json:"hasError"`
		LastFailureAt       *string  `json:"lastFailureAt,omitempty"`
		LastFailureError    string   `json:"lastFailureError,omitempty"`
		ConsecutiveFailures int      `json:"consecutiveFailures"`
	}
	type publicGeosite struct {
		Name                string   `json:"name"`
		DisplayName         string   `json:"displayName,omitempty"`
		Description         string   `json:"description,omitempty"`
		Icon                string   `json:"icon,omitempty"`
		Tags                []string `json:"tags"`
		Clients             []string `json:"clients"`
		Provider            string   `json:"provider"`
		List                string   `json:"list"`
		Attrs               []string `json:"attrs"`
		OutputName          string   `json:"outputName"`
		LastUpdated         *string  `json:"lastUpdated"`
		HasError            bool     `json:"hasError,omitempty"`
		LastFailureAt       *string  `json:"lastFailureAt,omitempty"`
		LastFailureError    string   `json:"lastFailureError,omitempty"`
		ConsecutiveFailures int      `json:"consecutiveFailures"`
	}

	rulesStatus := []publicRule{}
	geositeStatus := []publicGeosite{}
	var (
		rulesCount   int
		geositeCount int
		ruleFiles    int
		geositeFiles int
	)
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		metas := artsByRule[rule.Name]
		lastUpdated, lastFailureAt, lastFailureMsg, hasError, consecutiveFailures := summarizeArtifacts(metas)
		if schema.IsGeositeRule(rule) {
			source := schema.PrimaryGeositeSource(rule)
			geositeStatus = append(geositeStatus, publicGeosite{
				Name:                choose(source.List, rule.Name),
				DisplayName:         rule.DisplayName,
				Description:         rule.Description,
				Icon:                rule.Icon,
				Tags:                defaultStrings(rule.Tags),
				Clients:             rule.Output.Clients,
				Provider:            choose(source.Provider, "v2fly"),
				List:                choose(source.List, rule.Name),
				Attrs:               defaultStrings(source.Attrs),
				OutputName:          schema.GeositeOutputName(source),
				LastUpdated:         lastUpdated,
				HasError:            hasError,
				LastFailureAt:       lastFailureAt,
				LastFailureError:    lastFailureMsg,
				ConsecutiveFailures: consecutiveFailures,
			})
			geositeCount++
			geositeFiles += len(rule.Output.Clients)
		} else {
			rulesStatus = append(rulesStatus, publicRule{
				Name:                rule.Name,
				DisplayName:         rule.DisplayName,
				Description:         rule.Description,
				Icon:                rule.Icon,
				Tags:                defaultStrings(rule.Tags),
				Clients:             rule.Output.Clients,
				LastUpdated:         lastUpdated,
				HasError:            hasError,
				LastFailureAt:       lastFailureAt,
				LastFailureError:    lastFailureMsg,
				ConsecutiveFailures: consecutiveFailures,
			})
			rulesCount++
			ruleFiles += len(rule.Output.Clients)
		}
	}

	clientsList := make([]map[string]string, 0, len(clients))
	for _, c := range clients {
		clientsList = append(clientsList, map[string]string{"id": c.ID, "displayName": c.DisplayName})
	}

	// Resolve the consecutive-failure threshold once per response so both the
	// admin and public payloads can pre-compute the "更新失败" tag client-side
	// without an extra round-trip to /api/system-settings.
	sysSettings, sysErr := s.Store.GetSystemSettings(ctx)
	if sysErr != nil {
		log.Printf("[status] failed to load system settings: %v; using defaults", sysErr)
	}
	failureThreshold := sysSettings.Sync.FailureThreshold
	if failureThreshold <= 0 {
		failureThreshold = schema.DefaultSystemSettings().Sync.FailureThreshold
	}

	if !isAdmin {
		// Public consumers must not see free-form failure text. Source URLs
		// occasionally carry tokens or private hostnames that processor.go
		// (line ~108) splices into the error message verbatim; redact the
		// whole string and keep only the boolean + timestamp signal.
		for i := range rulesStatus {
			rulesStatus[i].LastFailureError = ""
		}
		for i := range geositeStatus {
			geositeStatus[i].LastFailureError = ""
		}
		lastSync, _ := s.Store.GetLastSyncInfo(ctx)
		var lastSyncAt *string
		if lastSync.LastSuccessfulSyncAt != nil {
			lastSyncAt = lastSync.LastSuccessfulSyncAt
		} else if lastSync.LastFullSyncAt != nil {
			lastSyncAt = lastSync.LastFullSyncAt
		}
		s.JSON(w, http.StatusOK, map[string]any{
			"rulesCount":        rulesCount,
			"geositeRulesCount": geositeCount,
			"lastSyncAt":        lastSyncAt,
			"rules":             rulesStatus,
			"geositeRules":      geositeStatus,
			"clients":           clientsList,
			"version":           Version,
			"failureThreshold":  failureThreshold,
		})
		return
	}

	lastSync, _ := s.Store.GetLastSyncInfo(ctx)
	today := time.Now().UTC().Format("2006-01-02")
	todayStats, _ := s.Store.GetDailyStats(ctx, today)
	changeCount, _ := s.Store.CountChangeRecords(ctx, today)
	failureCount, _ := s.Store.CountFailureRecords(ctx, today)

	// Compute the next scheduled sync from the current schedule + the most
	// recent successful sync. Anchoring on lastSuccessfulSyncAt (rather than
	// "now") matches the scheduler's own logic — a fresh install with no
	// successes yet falls back to "now" so the UI shows "in N hours" rather
	// than a stale time travelled from the past.
	schedule, _ := s.Store.GetSyncSchedule(ctx)
	nextSyncAt := syncengine.ComputeNextSyncAt(schedule, lastSync.LastSuccessfulSyncAt)

	s.JSON(w, http.StatusOK, map[string]any{
		"rulesCount":            rulesCount,
		"geositeRulesCount":     geositeCount,
		"ruleFilesCount":        ruleFiles,
		"geositeRuleFilesCount": geositeFiles,
		"lastSync":              lastSync,
		"nextSyncAt":            nextSyncAt,
		"scheduleMode":          schedule.Mode,
		"needsInit":             len(cfg.Rules) == 0,
		"todayStats": map[string]any{
			"date":                todayStats.Date,
			"syncCount":           todayStats.SyncCount,
			"blobWriteCount":      todayStats.BlobWriteCount,
			"rulesChanged":        todayStats.RulesChanged,
			"totalRulesProcessed": todayStats.TotalRulesProcessed,
			"failedSources":       todayStats.FailedSources,
			"ruleFilesChanged":    changeCount,
			"failureRecords":      failureCount,
		},
		"rules":            rulesStatus,
		"geositeRules":     geositeStatus,
		"clients":          clientsList,
		"version":          Version,
		"failureThreshold": failureThreshold,
	})
}

// summarizeArtifacts collapses per-(rule,client) attempt rows into the pieces
// the UI cares about: latest successful publish time, latest failed attempt
// time + message, a flag for "any client currently in failed state", and the
// maximum consecutive-failure count across clients (used by the dashboard to
// render the "更新失败" badge when it exceeds the configured threshold).
// Empty strings on legacy rows are treated as no-data.
func summarizeArtifacts(metas []schema.ArtifactMeta) (lastUpdated *string, lastFailureAt *string, lastFailureMsg string, hasError bool, maxConsecutiveFailures int) {
	if len(metas) == 0 {
		return nil, nil, "", false, 0
	}
	var latestSuccess string
	var latestFailure string
	for i := range metas {
		m := &metas[i]
		if m.LastUpdatedAt != "" && m.LastUpdatedAt > latestSuccess {
			latestSuccess = m.LastUpdatedAt
		}
		if m.ConsecutiveFailures > maxConsecutiveFailures {
			maxConsecutiveFailures = m.ConsecutiveFailures
		}
		if m.LastAttemptStatus == "failed" {
			hasError = true
			ts := m.LastAttemptedAt
			if ts == "" {
				ts = m.LastUpdatedAt
			}
			if ts > latestFailure {
				latestFailure = ts
				lastFailureMsg = m.LastAttemptError
			}
		}
	}
	if latestSuccess != "" {
		lastUpdated = &latestSuccess
	}
	if latestFailure != "" {
		lastFailureAt = &latestFailure
	}
	return
}

func choose(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func defaultStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func itoa(n int) string {
	return formatInt(int64(n))
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ensure context import used
var _ = context.Background
