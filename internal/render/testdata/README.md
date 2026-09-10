# SRS fixtures

`*.srs` here are sing-box binary rule-sets written by sing-box's own writer
(`github.com/sagernet/sing-box/common/srs`, version 3) — sing-box v1.13.19.
`TestRenderSingboxSRSMatchesSingboxFixtures` renders the same conditions from IR
entries and compares the decompressed payloads, so the hand-written encoder in
`codec_singbox_srs.go` stays byte-compatible with the format sing-box loads.

The comparison is on the decompressed payload and the 4-byte container header,
not on the raw file: zlib output is not stable across Go releases.

`conditions.srs` mirrors one flat entry list after field-group merging (the
domain group collapses into one rule, every other kind stands alone);
`logical.srs` mirrors AND/NOT/OR entries, where NOT is `and` plus `invert`
because sing-box has no `not` mode.

## Regenerating

Copy the program below into a scratch module that can reach sing-box, then run
it with this directory as the argument. Anything that changes the fixtures has
to keep the entry order in the Go test in sync.

```
module srsgen

go 1.24.7

require github.com/sagernet/sing-box v1.13.19
```

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
)

func def(rule option.DefaultHeadlessRule) option.HeadlessRule {
	return option.HeadlessRule{Type: "default", DefaultOptions: rule}
}

func logical(mode string, invert bool, rules ...option.HeadlessRule) option.HeadlessRule {
	return option.HeadlessRule{
		Type:           "logical",
		LogicalOptions: option.LogicalHeadlessRule{Mode: mode, Invert: invert, Rules: rules},
	}
}

func write(dir, name string, rules []option.HeadlessRule) {
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := srs.Write(f, option.PlainRuleSet{Rules: rules}, 3); err != nil {
		panic(err)
	}
}

func main() {
	dir := os.Args[1]
	write(dir, "srs-v3-conditions.srs", []option.HeadlessRule{
		def(option.DefaultHeadlessRule{
			Domain:        []string{"google.com", "example.com"},
			DomainSuffix:  []string{"youtube.com", "googlevideo.com"},
			DomainKeyword: []string{"ads", "tracker"},
			DomainRegex:   []string{`^ad[0-9]+\.example\.com$`, `^track[0-9]+\.example\.com$`},
			IPCIDR:        []string{"10.0.0.0/8", "192.168.0.0/16", "2001:db8::/32"},
		}),
		def(option.DefaultHeadlessRule{SourceIPCIDR: []string{"172.16.0.0/12"}}),
		def(option.DefaultHeadlessRule{Port: []uint16{80, 443}, PortRange: []string{"1000:2000"}}),
		def(option.DefaultHeadlessRule{SourcePort: []uint16{1024}, SourcePortRange: []string{"2000:3000"}}),
		def(option.DefaultHeadlessRule{Network: []string{"tcp"}}),
		def(option.DefaultHeadlessRule{Network: []string{"udp"}}),
		def(option.DefaultHeadlessRule{ProcessName: []string{"curl"}}),
		def(option.DefaultHeadlessRule{ProcessPath: []string{"/usr/bin/curl"}}),
		def(option.DefaultHeadlessRule{ProcessPathRegex: []string{`^/opt/.*`}}),
	})
	write(dir, "srs-v3-logical.srs", []option.HeadlessRule{
		logical("and", false,
			def(option.DefaultHeadlessRule{DomainSuffix: []string{"youtube.com"}}),
			def(option.DefaultHeadlessRule{IPCIDR: []string{"10.0.0.0/8"}}),
		),
		logical("and", true,
			def(option.DefaultHeadlessRule{Domain: []string{"ads.example.com"}}),
		),
		logical("or", false,
			def(option.DefaultHeadlessRule{Network: []string{"udp"}}),
			logical("and", false,
				def(option.DefaultHeadlessRule{Domain: []string{"nested.example.com"}}),
				def(option.DefaultHeadlessRule{IPCIDR: []string{"2001:db8::/32"}}),
			),
		),
	})
	fmt.Println("fixtures written to", dir)
}
```
