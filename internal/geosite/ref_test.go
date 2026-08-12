package geosite

import (
	"testing"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		input   string
		want    GeositeRef
		wantErr bool
	}{
		{
			input: "v2fly/geolocation-!cn",
			want:  GeositeRef{Provider: "v2fly", List: "geolocation-!cn"},
		},
		{
			input: "v2fly/geolocation-!cn@ads",
			want:  GeositeRef{Provider: "v2fly", List: "geolocation-!cn", Attrs: []string{"ads"}},
		},
		{
			input: "loyalsoldier/google@cn,!ads",
			want:  GeositeRef{Provider: "loyalsoldier", List: "google", Attrs: []string{"!ads", "cn"}},
		},
		{
			input: "  V2Fly / Google  ",
			want:  GeositeRef{Provider: "v2fly", List: "google"},
		},
		{input: "", wantErr: true},
		{input: "noslash", wantErr: true},
		{input: "/noprefix", wantErr: true},
		{input: "provider/", wantErr: true},
	}

	for _, tt := range tests {
		ref, err := ParseRef(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRef(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRef(%q) = error %v", tt.input, err)
			continue
		}
		if ref.Provider != tt.want.Provider {
			t.Errorf("ParseRef(%q).Provider = %q, want %q", tt.input, ref.Provider, tt.want.Provider)
		}
		if ref.List != tt.want.List {
			t.Errorf("ParseRef(%q).List = %q, want %q", tt.input, ref.List, tt.want.List)
		}
		if len(ref.Attrs) != len(tt.want.Attrs) {
			t.Errorf("ParseRef(%q).Attrs = %v, want %v", tt.input, ref.Attrs, tt.want.Attrs)
		} else {
			for i := range ref.Attrs {
				if ref.Attrs[i] != tt.want.Attrs[i] {
					t.Errorf("ParseRef(%q).Attrs[%d] = %q, want %q", tt.input, i, ref.Attrs[i], tt.want.Attrs[i])
				}
			}
		}
	}
}

func TestFormatRef(t *testing.T) {
	tests := []struct {
		ref  GeositeRef
		want string
	}{
		{GeositeRef{Provider: "v2fly", List: "geolocation-!cn"}, "v2fly/geolocation-!cn"},
		{GeositeRef{Provider: "v2fly", List: "geolocation-!cn", Attrs: []string{"ads"}}, "v2fly/geolocation-!cn@ads"},
		{GeositeRef{Provider: "p", List: "l", Attrs: []string{"a", "b"}}, "p/l@a,b"},
	}
	for _, tt := range tests {
		got := tt.ref.FormatRef()
		if got != tt.want {
			t.Errorf("FormatRef(%+v) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestValidateRef(t *testing.T) {
	cache := &ProviderCache{
		Provider: "v2fly",
		Entries: map[string][]Entry{
			"google": {
				{Type: EntryDomain, Value: "google.com", Attrs: []string{"cn"}},
				{Type: EntryFull, Value: "www.google.com", Attrs: []string{"cn", "ads"}},
			},
			"geolocation-!cn": {
				{Type: EntryDomain, Value: "example.com", Attrs: []string{"ads"}},
			},
		},
	}

	tests := []struct {
		ref     GeositeRef
		wantErr bool
	}{
		{GeositeRef{Provider: "v2fly", List: "google"}, false},
		{GeositeRef{Provider: "v2fly", List: "google", Attrs: []string{"cn"}}, false},
		{GeositeRef{Provider: "v2fly", List: "google", Attrs: []string{"nonexist"}}, true},
		{GeositeRef{Provider: "v2fly", List: "nonexist"}, true},
		{GeositeRef{Provider: "v2fly", List: "geolocation-!cn", Attrs: []string{"ads"}}, false},
	}

	for _, tt := range tests {
		err := ValidateRef(cache, tt.ref)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateRef(%+v) expected error", tt.ref)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateRef(%+v) = %v", tt.ref, err)
		}
	}
}
