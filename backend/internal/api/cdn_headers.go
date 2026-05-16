package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// applyCDNHeaders writes Cache-Control and CDN-related headers per settings.
// It does NOT set CORS headers — those are handled by corsMiddleware globally.
func (s *Server) applyCDNHeaders(ctx context.Context, w http.ResponseWriter, mime string) {
	w.Header().Set("Content-Type", mime)

	settings, err := s.Store.GetCdnSettings(ctx)
	if err != nil || !settings.Enabled {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}

	w.Header().Set("Cache-Control", buildCacheControl(settings))
	if settings.CloudflareCdnCacheControl != "" {
		w.Header().Set("Cloudflare-CDN-Cache-Control", settings.CloudflareCdnCacheControl)
	}
	for _, h := range settings.CustomHeaders {
		if h.Name == "" || h.Value == "" {
			continue
		}
		w.Header().Set(h.Name, h.Value)
	}
}

// buildCacheControl mirrors buildCacheControlHeader in storage-adapter.ts.
func buildCacheControl(s schema.CdnSettings) string {
	switch s.CacheMode {
	case "no-cache":
		parts := []string{"no-cache"}
		if s.StaleIfErrorSeconds > 0 {
			parts = append(parts, fmt.Sprintf("stale-if-error=%d", s.StaleIfErrorSeconds))
		}
		return strings.Join(parts, ", ")
	case "no-store":
		return "no-store"
	case "custom":
		if s.CustomCacheControl != "" {
			return s.CustomCacheControl
		}
		return "no-cache"
	default:
		return "no-cache"
	}
}

func sanitizeHeaders(headers []schema.CdnCustomHeader) []schema.CdnCustomHeader {
	out := make([]schema.CdnCustomHeader, 0, len(headers))
	for _, h := range headers {
		if h.Name == "" || h.Value == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}
