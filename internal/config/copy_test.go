package config

import "testing"

func TestConfigDeepCopyIsIndependent(t *testing.T) {
	original := &Config{
		Clients: []ClientConfig{{
			ID: "client", Formats: []ClientFormatConfig{{ID: "format", Template: "template"}},
			Variants: []ClientVariantConfig{{ID: "variant", Ops: []OpConfig{{Type: "include_kinds", Kinds: []string{"domain"}}}}},
		}},
		Rules: []RuleConfig{{
			ID: "rule", Tags: []string{"tag"}, Sources: []SourceConfig{{Content: "example", Attrs: []string{"ads"}}},
			Ops: []OpConfig{{Type: "exclude_kinds", Kinds: []string{"ip_cidr"}}}, Merge: &MergeConfig{Strategy: "union"}, Outputs: []string{"client"},
		}},
		Geosite:   &GeositeConfig{Providers: []GeositeProvider{{Name: "v2fly", Clients: []string{"client"}}}},
		positions: &PositionIndex{entries: map[string]Position{"rules[0].id": {Line: 4, Column: 5}}},
	}

	clone := original.DeepCopy()
	clone.Clients[0].Formats[0].ID = "changed"
	clone.Clients[0].Variants[0].Ops[0].Kinds[0] = "changed"
	clone.Rules[0].Tags[0] = "changed"
	clone.Rules[0].Sources[0].Attrs[0] = "changed"
	clone.Rules[0].Ops[0].Kinds[0] = "changed"
	clone.Rules[0].Merge.Strategy = "difference"
	clone.Rules[0].Outputs[0] = "changed"
	clone.Geosite.Providers[0].Clients[0] = "changed"
	clone.positions.entries["rules[0].id"] = Position{Line: 9}

	if original.Clients[0].Formats[0].ID != "format" || original.Clients[0].Variants[0].Ops[0].Kinds[0] != "domain" {
		t.Fatal("client copy shares nested data")
	}
	if original.Rules[0].Tags[0] != "tag" || original.Rules[0].Sources[0].Attrs[0] != "ads" || original.Rules[0].Merge.Strategy != "union" {
		t.Fatal("rule copy shares nested data")
	}
	if original.Rules[0].Outputs[0] != "client" || original.Geosite.Providers[0].Clients[0] != "client" {
		t.Fatal("output or geosite copy shares nested data")
	}
	if original.positions.Lookup("rules[0].id").Line != 4 {
		t.Fatal("position copy shares map data")
	}
}
