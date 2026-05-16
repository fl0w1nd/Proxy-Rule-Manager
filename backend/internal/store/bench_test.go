package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

// BenchmarkSaveConfig_10kRules measures bulk-write performance for the
// geosite "import all" scenario where thousands of rules land in a single
// SaveConfig transaction.
func BenchmarkSaveConfig_10kRules(b *testing.B) {
	for _, n := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprintf("rules=%d", n), func(b *testing.B) {
			s := newBenchStore(b)
			cfg := buildGeositeCfg(n)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := s.SaveConfig(context.Background(), cfg); err != nil {
					b.Fatalf("SaveConfig: %v", err)
				}
			}
		})
	}
}

// BenchmarkSaveConfig_ReplaceWithDelta measures the realistic case where
// a rev N+1 differs from rev N by only a handful of rules.
func BenchmarkSaveConfig_DeltaOn10k(b *testing.B) {
	s := newBenchStore(b)
	cfg := buildGeositeCfg(10000)
	if _, err := s.SaveConfig(context.Background(), cfg); err != nil {
		b.Fatalf("warm-up SaveConfig: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cfg.Rules[i%len(cfg.Rules)].Description = fmt.Sprintf("touch %d", i)
		if _, err := s.SaveConfig(context.Background(), cfg); err != nil {
			b.Fatalf("SaveConfig: %v", err)
		}
	}
}

func BenchmarkGetConfig_10k(b *testing.B) {
	s := newBenchStore(b)
	if _, err := s.SaveConfig(context.Background(), buildGeositeCfg(10000)); err != nil {
		b.Fatalf("warm-up: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetConfig(context.Background()); err != nil {
			b.Fatalf("GetConfig: %v", err)
		}
	}
}

func newBenchStore(b *testing.B) *Store {
	b.Helper()
	dir := b.TempDir()
	paths := Paths{
		DataDir:       dir,
		RulesDir:      dir + "/Rules",
		SourcesDir:    dir + "/sources",
		GeositeDir:    dir + "/geosite",
		IconSetDir:    dir + "/iconset",
		ClientFileDir: dir + "/client",
		WAFDir:        dir + "/waf",
	}
	s, err := Open(dir+"/db.sqlite", paths)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

func buildGeositeCfg(n int) schema.RulesConfig {
	cfg := schema.DefaultConfig()
	cfg.Rules = make([]schema.RuleConfig, 0, n)
	for i := 0; i < n; i++ {
		cfg.Rules = append(cfg.Rules, schema.RuleConfig{
			Name: fmt.Sprintf("geosite_v2fly_list_%05d", i),
			Sources: []schema.SourceConfig{{
				Type:     "geosite",
				Provider: "v2fly",
				List:     fmt.Sprintf("list_%05d", i),
			}},
			Output: schema.OutputConfig{Clients: []string{"clash_meta"}},
		})
	}
	return cfg
}
