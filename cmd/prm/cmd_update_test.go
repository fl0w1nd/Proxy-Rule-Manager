package main

import (
	"testing"
	"time"

	"github.com/fl0w1nd/proxy-rule-manager/internal/state"
)

func resetUpdateFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"geosite", "geoip"} {
		flag := updateCmd.Flags().Lookup(name)
		value, changed := flag.Value.String(), flag.Changed
		_ = flag.Value.Set("false")
		flag.Changed = false
		t.Cleanup(func() {
			_ = flag.Value.Set(value)
			flag.Changed = changed
		})
	}
}

func TestUpdateCommandGeoScopes(t *testing.T) {
	for _, scope := range []string{"geosite", "geoip"} {
		t.Run(scope, func(t *testing.T) {
			resetRuntimeFlags(t)
			resetUpdateFlags(t)
			root := t.TempDir()
			withBuildCommandPaths(t, writeBuildTestConfig(t, root, "content: DOMAIN,example.com"), "")
			if err := updateCmd.Flags().Set(scope, "true"); err != nil {
				t.Fatal(err)
			}
			if err := updateCmd.RunE(updateCmd, nil); err != nil {
				t.Fatal(err)
			}
			st, err := state.Open(dataDirFlag)
			if err != nil {
				t.Fatal(err)
			}
			records := st.ListUpdateHistory(24*time.Hour, 10, time.Now())
			if len(records) != 1 || records[0].Scope != scope || records[0].Status != "completed" || records[0].RulesTotal != 0 {
				t.Fatalf("history=%+v", records)
			}
		})
	}
}

func TestUpdateCommandRejectsMixedScopes(t *testing.T) {
	for _, scope := range []string{"geosite", "geoip"} {
		t.Run(scope, func(t *testing.T) {
			resetUpdateFlags(t)
			if err := updateCmd.Flags().Set(scope, "true"); err != nil {
				t.Fatal(err)
			}
			if err := updateCmd.Args(updateCmd, []string{"rule"}); err == nil {
				t.Fatal("expected rule ID validation error")
			}
		})
	}
	t.Run("both geo flags", func(t *testing.T) {
		resetUpdateFlags(t)
		_ = updateCmd.Flags().Set("geosite", "true")
		_ = updateCmd.Flags().Set("geoip", "true")
		if err := updateCmd.ValidateFlagGroups(); err == nil {
			t.Fatal("expected mutually exclusive flag error")
		}
	})
}
