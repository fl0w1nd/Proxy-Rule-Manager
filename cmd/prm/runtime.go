package main

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultDataDir   = "./data"
	defaultServeHost = "127.0.0.1"
	defaultServePort = 3001
)

var (
	dataDirFlag        = defaultDataDir
	serveHostFlag      = defaultServeHost
	servePortFlag      = defaultServePort
	trustedProxiesFlag []string
)

type serveRuntimeOptions struct {
	DataDir        string
	Host           string
	Port           int
	TrustedProxies []netip.Prefix
	AdminToken     string
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataDirFlag, "data-dir", defaultDataDir, "runtime data directory (env: PRM_DATA_DIR)")
	serveCmd.Flags().StringVar(&serveHostFlag, "host", defaultServeHost, "HTTP listen IP (env: PRM_SERVE_HOST)")
	serveCmd.Flags().IntVar(&servePortFlag, "port", defaultServePort, "HTTP listen port (env: PRM_SERVE_PORT)")
	serveCmd.Flags().StringArrayVar(&trustedProxiesFlag, "trusted-proxy", nil, "trusted reverse proxy IP or CIDR; repeatable (env: PRM_TRUSTED_PROXIES)")
}

func resolveDataDir(cmd *cobra.Command) (string, error) {
	value := dataDirFlag
	if !flagChanged(cmd, "data-dir") {
		if env, ok := os.LookupEnv("PRM_DATA_DIR"); ok {
			value = env
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("data directory is empty")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve data directory %q: %w", value, err)
	}
	return filepath.Clean(abs), nil
}

func resolveServeRuntime(cmd *cobra.Command) (serveRuntimeOptions, error) {
	dataDir, err := resolveDataDir(cmd)
	if err != nil {
		return serveRuntimeOptions{}, err
	}

	host := serveHostFlag
	if !flagChanged(cmd, "host") {
		if env, ok := os.LookupEnv("PRM_SERVE_HOST"); ok {
			host = env
		}
	}
	host = strings.TrimSpace(host)
	if _, err := netip.ParseAddr(host); err != nil {
		return serveRuntimeOptions{}, fmt.Errorf("serve host must be an IP address: %w", err)
	}

	port := servePortFlag
	if !flagChanged(cmd, "port") {
		if env, ok := os.LookupEnv("PRM_SERVE_PORT"); ok {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(env))
			if parseErr != nil {
				return serveRuntimeOptions{}, fmt.Errorf("parse PRM_SERVE_PORT: %w", parseErr)
			}
			port = parsed
		}
	}
	if port < 1 || port > 65535 {
		return serveRuntimeOptions{}, fmt.Errorf("serve port must be between 1 and 65535")
	}

	proxyValues := append([]string(nil), trustedProxiesFlag...)
	if !flagChanged(cmd, "trusted-proxy") {
		if env, ok := os.LookupEnv("PRM_TRUSTED_PROXIES"); ok {
			proxyValues = nil
			if strings.TrimSpace(env) != "" {
				proxyValues = strings.Split(env, ",")
			}
		}
	}
	proxies, err := parseTrustedProxies(proxyValues)
	if err != nil {
		return serveRuntimeOptions{}, err
	}

	adminToken := strings.TrimSpace(os.Getenv("PRM_ADMIN_TOKEN"))
	if adminToken == "" {
		return serveRuntimeOptions{}, fmt.Errorf("PRM_ADMIN_TOKEN environment variable is required")
	}

	return serveRuntimeOptions{
		DataDir: dataDir, Host: host, Port: port,
		TrustedProxies: proxies, AdminToken: adminToken,
	}, nil
}

func parseTrustedProxies(values []string) ([]netip.Prefix, error) {
	proxies := make([]netip.Prefix, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("trusted proxy must not be empty")
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			proxies = append(proxies, prefix)
			continue
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: must be an IP address or CIDR", value)
		}
		proxies = append(proxies, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return proxies, nil
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
		return true
	}
	if flag := cmd.InheritedFlags().Lookup(name); flag != nil && flag.Changed {
		return true
	}
	return false
}
