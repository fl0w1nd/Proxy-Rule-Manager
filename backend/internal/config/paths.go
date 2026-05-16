package config

import (
	"os"
	"path/filepath"
)

// Config holds runtime configuration, including filesystem paths.
type Config struct {
	DataDir       string
	DBPath        string
	RulesDir      string
	SourcesDir    string
	GeositeDir    string
	IconSetDir    string
	ClientFileDir string
	WAFDir        string
	Port          int
	AdminToken    string
	OutDir        string
}

// Load reads environment variables and computes derived paths.
func Load() *Config {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		cwd, _ := os.Getwd()
		dataDir = filepath.Join(cwd, "data")
	}

	port := 3000
	if portStr := os.Getenv("PORT"); portStr != "" {
		if parsed, err := parsePort(portStr); err == nil {
			port = parsed
		}
	}

	outDir := os.Getenv("OUT_DIR")
	if outDir == "" {
		cwd, _ := os.Getwd()
		outDir = filepath.Join(cwd, "out")
	}

	return &Config{
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "db.sqlite"),
		RulesDir:      filepath.Join(dataDir, "Rules"),
		SourcesDir:    filepath.Join(dataDir, "sources"),
		GeositeDir:    filepath.Join(dataDir, "geosite"),
		IconSetDir:    filepath.Join(dataDir, "iconset"),
		ClientFileDir: filepath.Join(dataDir, "client"),
		WAFDir:        filepath.Join(dataDir, "waf"),
		Port:          port,
		AdminToken:    os.Getenv("ADMIN_TOKEN"),
		OutDir:        outDir,
	}
}

func parsePort(s string) (int, error) {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errInvalidPort
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 || n > 65535 {
		return 0, errInvalidPort
	}
	return n, nil
}

var errInvalidPort = osErr("invalid port")

type osErr string

func (e osErr) Error() string { return string(e) }
