package main

import (
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveDataDirPrecedenceAndAbsolutePath(t *testing.T) {
	resetRuntimeFlags(t)
	got, err := resolveDataDir(updateCmd)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(defaultDataDir)
	if got != want {
		t.Fatalf("default data dir = %q, want %q", got, want)
	}

	t.Setenv("PRM_DATA_DIR", "env-data")
	got, err = resolveDataDir(updateCmd)
	if err != nil {
		t.Fatal(err)
	}
	want, _ = filepath.Abs("env-data")
	if got != want {
		t.Fatalf("environment data dir = %q, want %q", got, want)
	}

	dataDirFlag = "flag-data"
	rootCmd.PersistentFlags().Lookup("data-dir").Changed = true
	got, err = resolveDataDir(updateCmd)
	if err != nil {
		t.Fatal(err)
	}
	want, _ = filepath.Abs("flag-data")
	if got != want {
		t.Fatalf("flag data dir = %q, want %q", got, want)
	}
}

func TestResolveServeRuntimeDefaults(t *testing.T) {
	resetRuntimeFlags(t)
	t.Setenv("PRM_ADMIN_TOKEN", "secret")
	got, err := resolveServeRuntime(serveCmd)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != defaultServeHost || got.Port != defaultServePort || len(got.TrustedProxies) != 0 {
		t.Fatalf("default runtime options = %+v", got)
	}
}

func TestResolveServeRuntimeFromEnvironment(t *testing.T) {
	resetRuntimeFlags(t)
	t.Setenv("PRM_DATA_DIR", "runtime-data")
	t.Setenv("PRM_SERVE_HOST", "0.0.0.0")
	t.Setenv("PRM_SERVE_PORT", "8080")
	t.Setenv("PRM_TRUSTED_PROXIES", "127.0.0.1, 172.16.0.0/12")
	t.Setenv("PRM_ADMIN_TOKEN", "secret")

	got, err := resolveServeRuntime(serveCmd)
	if err != nil {
		t.Fatal(err)
	}
	wantProxies := []netip.Prefix{
		netip.PrefixFrom(netip.MustParseAddr("127.0.0.1"), 32),
		netip.MustParsePrefix("172.16.0.0/12"),
	}
	if got.Host != "0.0.0.0" || got.Port != 8080 || got.AdminToken != "secret" || !reflect.DeepEqual(got.TrustedProxies, wantProxies) {
		t.Fatalf("runtime options = %+v", got)
	}
}

func TestResolveServeRuntimeFlagPrecedence(t *testing.T) {
	resetRuntimeFlags(t)
	t.Setenv("PRM_SERVE_HOST", "0.0.0.0")
	t.Setenv("PRM_SERVE_PORT", "8080")
	t.Setenv("PRM_TRUSTED_PROXIES", "10.0.0.0/8")
	t.Setenv("PRM_ADMIN_TOKEN", "secret")
	serveHostFlag, servePortFlag = "127.0.0.2", 9090
	trustedProxiesFlag = []string{"192.168.0.0/16"}
	for _, name := range []string{"host", "port", "trusted-proxy"} {
		serveCmd.Flags().Lookup(name).Changed = true
	}

	got, err := resolveServeRuntime(serveCmd)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "127.0.0.2" || got.Port != 9090 || len(got.TrustedProxies) != 1 || got.TrustedProxies[0] != netip.MustParsePrefix("192.168.0.0/16") {
		t.Fatalf("flag options = %+v", got)
	}
}

func TestResolveServeRuntimeRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		want string
	}{
		{name: "host", env: "PRM_SERVE_HOST=localhost", want: "IP address"},
		{name: "port syntax", env: "PRM_SERVE_PORT=http", want: "parse PRM_SERVE_PORT"},
		{name: "port range", env: "PRM_SERVE_PORT=70000", want: "between 1 and 65535"},
		{name: "trusted proxy", env: "PRM_TRUSTED_PROXIES=invalid", want: "invalid trusted proxy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetRuntimeFlags(t)
			t.Setenv("PRM_ADMIN_TOKEN", "secret")
			parts := strings.SplitN(tc.env, "=", 2)
			t.Setenv(parts[0], parts[1])
			if _, err := resolveServeRuntime(serveCmd); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestResolveServeRuntimeRequiresAdminToken(t *testing.T) {
	resetRuntimeFlags(t)
	unsetEnv(t, "PRM_ADMIN_TOKEN")
	if _, err := resolveServeRuntime(serveCmd); err == nil || !strings.Contains(err.Error(), "PRM_ADMIN_TOKEN") {
		t.Fatalf("missing token error = %v", err)
	}
}

func resetRuntimeFlags(t *testing.T) {
	t.Helper()
	previousDataDir, previousHost, previousPort := dataDirFlag, serveHostFlag, servePortFlag
	previousProxies := append([]string(nil), trustedProxiesFlag...)
	dataDirFlag, serveHostFlag, servePortFlag, trustedProxiesFlag = defaultDataDir, defaultServeHost, defaultServePort, nil
	rootCmd.PersistentFlags().Lookup("data-dir").Changed = false
	for _, name := range []string{"host", "port", "trusted-proxy"} {
		serveCmd.Flags().Lookup(name).Changed = false
	}
	for _, name := range []string{"PRM_DATA_DIR", "PRM_SERVE_HOST", "PRM_SERVE_PORT", "PRM_TRUSTED_PROXIES", "PRM_ADMIN_TOKEN"} {
		unsetEnv(t, name)
	}
	t.Cleanup(func() {
		dataDirFlag, serveHostFlag, servePortFlag, trustedProxiesFlag = previousDataDir, previousHost, previousPort, previousProxies
		rootCmd.PersistentFlags().Lookup("data-dir").Changed = false
		for _, name := range []string{"host", "port", "trusted-proxy"} {
			serveCmd.Flags().Lookup(name).Changed = false
		}
	})
}

func unsetEnv(t *testing.T, name string) {
	t.Helper()
	value, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(name, value)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
