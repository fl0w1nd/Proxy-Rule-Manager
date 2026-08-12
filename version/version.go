// Package version exposes build metadata for the CLI and HTTP API.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var sourceVersion string

// These values are overridden with -ldflags in release builds.
var (
	Version = ""
	Commit  = "unknown"
	Date    = "unknown"
)

// Current returns the injected release version or the checked-in version.
func Current() string {
	if Version != "" {
		return Version
	}
	return strings.TrimSpace(sourceVersion)
}
