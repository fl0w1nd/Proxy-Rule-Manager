package render

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/fl0w1nd/proxy-rule-manager/templates"
)

// The fixtures in testdata are written by sing-box's own rule-set writer
// (common/srs.Write, version 3) for the same conditions rendered here, which
// keeps this hand-written encoder pinned to the format users actually load.
// testdata/README.md holds the generator.
func TestRenderSingboxSRSMatchesSingboxFixtures(t *testing.T) {
	registry := NewRegistry()
	if err := registry.LoadEmbedded(templates.FS); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	tmpl, ok := registry.Get("singbox-binary")
	if !ok {
		t.Fatal("template singbox-binary not loaded")
	}

	tests := []struct {
		name    string
		fixture string
		entries []ir.Entry
	}{
		{
			name:    "conditions",
			fixture: "srs-v3-conditions.srs",
			// One flat list: the domain field group merges into a single rule,
			// every other kind becomes its own rule.
			entries: []ir.Entry{
				{Kind: ir.KindDomain, Value: "google.com"},
				{Kind: ir.KindDomain, Value: "example.com"},
				{Kind: ir.KindDomainSuffix, Value: "youtube.com"},
				{Kind: ir.KindDomainSuffix, Value: "googlevideo.com"},
				{Kind: ir.KindDomainKeyword, Value: "ads"},
				{Kind: ir.KindDomainKeyword, Value: "tracker"},
				{Kind: ir.KindDomainRegex, Value: `^ad[0-9]+\.example\.com$`},
				{Kind: ir.KindDomainRegex, Value: `^track[0-9]+\.example\.com$`},
				{Kind: ir.KindIPCIDR, Value: "10.0.0.0/8"},
				{Kind: ir.KindIPCIDR, Value: "192.168.0.0/16"},
				{Kind: ir.KindIPCIDR, Value: "2001:db8::/32"},
				{Kind: ir.KindSrcIPCIDR, Value: "172.16.0.0/12"},
				{Kind: ir.KindDstPort, Value: "80/443/1000-2000"},
				{Kind: ir.KindSrcPort, Value: "1024/2000-3000"},
				{Kind: ir.KindNetwork, Value: "tcp"},
				{Kind: ir.KindNetwork, Value: "udp"},
				{Kind: ir.KindProcessName, Value: "curl"},
				{Kind: ir.KindProcessPath, Value: "/usr/bin/curl"},
				{Kind: ir.KindProcessPathRegex, Value: `^/opt/.*`},
			},
		},
		{
			name:    "logical",
			fixture: "srs-v3-logical.srs",
			entries: []ir.Entry{
				{Kind: ir.KindAnd, Sub: []ir.Entry{
					{Kind: ir.KindDomainSuffix, Value: "youtube.com"},
					{Kind: ir.KindIPCIDR, Value: "10.0.0.0/8"},
				}},
				{Kind: ir.KindNot, Sub: []ir.Entry{
					{Kind: ir.KindDomain, Value: "ads.example.com"},
				}},
				{Kind: ir.KindOr, Sub: []ir.Entry{
					{Kind: ir.KindNetwork, Value: "udp"},
					{Kind: ir.KindAnd, Sub: []ir.Entry{
						{Kind: ir.KindDomain, Value: "nested.example.com"},
						{Kind: ir.KindIPCIDR, Value: "2001:db8::/32"},
					}},
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := Render(tmpl, test.entries)
			if err != nil {
				t.Fatal(err)
			}
			fixture, err := os.ReadFile(filepath.Join("testdata", test.fixture))
			if err != nil {
				t.Fatal(err)
			}

			gotPayload, err := srsPayload(out)
			if err != nil {
				t.Fatalf("rendered rule-set: %v", err)
			}
			wantPayload, err := srsPayload(fixture)
			if err != nil {
				t.Fatalf("fixture %s: %v", test.fixture, err)
			}
			// zlib output differs between Go releases, so the comparison is on
			// the decompressed payload; the container is checked separately.
			if !bytes.Equal(gotPayload, wantPayload) {
				t.Fatalf("payload differs from sing-box at byte %d:\n got % x\nwant % x",
					firstDifference(gotPayload, wantPayload), gotPayload, wantPayload)
			}
			if !bytes.HasPrefix(out, fixture[:4]) {
				t.Fatalf("container header = % x, want % x", out[:4], fixture[:4])
			}
		})
	}
}

// srsPayload validates the SRS container header and returns the decompressed
// rule payload.
func srsPayload(raw []byte) ([]byte, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("truncated rule-set: %d bytes", len(raw))
	}
	if !bytes.Equal(raw[:3], srsMagic[:]) {
		return nil, fmt.Errorf("bad magic % x", raw[:3])
	}
	if raw[3] != singboxRuleSetVersion {
		return nil, fmt.Errorf("version = %d, want %d", raw[3], singboxRuleSetVersion)
	}
	reader, err := zlib.NewReader(bytes.NewReader(raw[4:]))
	if err != nil {
		return nil, err
	}
	payload, err := io.ReadAll(reader)
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func firstDifference(got, want []byte) int {
	for i := range got {
		if i >= len(want) || got[i] != want[i] {
			return i
		}
	}
	return len(want)
}

func TestRenderSingboxSRSRejectsUnencodableField(t *testing.T) {
	tmpl := &Template{
		ID:        "custom-srs",
		Codec:     "singbox_srs",
		Extension: ".srs",
		KindMap:   map[string]KindMapping{"domain": {FieldName: "domain_typo"}},
	}
	_, err := Render(tmpl, []ir.Entry{{Kind: ir.KindDomain, Value: "example.com"}})
	if err == nil {
		t.Fatal("expected an error for a field the SRS format cannot encode")
	}
}
