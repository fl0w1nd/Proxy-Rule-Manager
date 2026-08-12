package config

import (
	"strings"
	"testing"
)

func TestExpandOutputTargetsIncludesBuiltinFormatsAndIRVariants(t *testing.T) {
	clients := []ClientConfig{
		{ID: "mihomo", Name: "Mihomo", Formats: []ClientFormatConfig{
			{ID: "mihomo-classical", Name: "Classical", Template: "mihomo-classical"},
			{ID: "mihomo-yaml", Name: "YAML", Template: "mihomo-yaml"},
		}},
		{
			ID: "sing-box", Name: "sing-box", Template: "singbox",
			Variants: []ClientVariantConfig{{
				ID: "sing-box-non-ip", Name: "Non-IP",
				Ops: []OpConfig{{Type: "exclude_kinds", Kinds: []string{"ip_cidr"}}},
			}},
		},
	}

	targets := ExpandOutputTargets(clients)
	want := []struct{ id, template, option string }{
		{"mihomo-classical", "mihomo-classical", "Classical"},
		{"mihomo-yaml", "mihomo-yaml", "YAML"},
		{"sing-box", "singbox", "Standard"},
		{"sing-box-non-ip", "singbox", "Non-IP"},
	}
	if len(targets) != len(want) {
		t.Fatalf("targets=%+v", targets)
	}
	for i := range want {
		if targets[i].ID != want[i].id || targets[i].Template != want[i].template || targets[i].OptionName != want[i].option {
			t.Fatalf("target[%d]=%+v, want=%+v", i, targets[i], want[i])
		}
	}
	if len(targets[3].Ops) != 1 || targets[3].Ops[0].Type != "exclude_kinds" {
		t.Fatalf("variant ops=%+v", targets[3].Ops)
	}
}

func TestValidateExplicitFormatAndVariantOps(t *testing.T) {
	cfg := &Config{Clients: []ClientConfig{{
		ID: "sing-box", Template: "singbox",
		Variants: []ClientVariantConfig{{
			ID:  "sing-box-filtered",
			Ops: []OpConfig{{Type: "filter_values", Mode: "regex", Pattern: "(["}},
		}},
	}}}
	cfg.Defaults()
	errs := cfg.Validate()
	if !containsErrorPath(errs, "clients[0].variants[0].ops[0].pattern") {
		t.Fatalf("variant validation errors=%v", ConfigErrors(errs))
	}
}

func TestValidateMultiFormatVariantRequiresTemplate(t *testing.T) {
	cfg := &Config{Clients: []ClientConfig{{
		ID: "mihomo",
		Formats: []ClientFormatConfig{
			{ID: "mihomo-classical", Template: "mihomo-classical"},
			{ID: "mihomo-yaml", Template: "mihomo-yaml"},
		},
		Variants: []ClientVariantConfig{{
			ID:  "mihomo-filtered",
			Ops: []OpConfig{{Type: "exclude_kinds", Kinds: []string{"ip_cidr"}}},
		}},
	}}}
	cfg.Defaults()
	errs := cfg.Validate()
	if !containsErrorPath(errs, "clients[0].variants[0].template") {
		t.Fatalf("variant validation errors=%v", ConfigErrors(errs))
	}
}

func TestValidateOutputIDCannotMatchAnotherClientFamily(t *testing.T) {
	cfg := &Config{Clients: []ClientConfig{
		{ID: "alpha", Formats: []ClientFormatConfig{{ID: "mihomo", Template: "mihomo-classical"}}},
		{ID: "mihomo", Formats: []ClientFormatConfig{{ID: "mihomo-classical", Template: "mihomo-classical"}}},
	}}
	cfg.Defaults()
	err := ConfigErrors(cfg.Validate()).Error()
	if !strings.Contains(err, `output id "mihomo" conflicts with client id`) {
		t.Fatalf("collision validation errors=%s", err)
	}
}
