package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const exampleConfig = `# Proxy Rule Manager configuration
# See: https://github.com/fl0w1nd/proxy-rule-manager

# Local: data_dir: ./data
# Docker: data_dir: /data  (the Dockerfile sets WORKDIR /data, so ./data
#         would resolve to /data/data; use /data to match the volume mount)
data_dir: ./data

clients:
  - id: mihomo
    name: Mihomo
    icon: mihomo
    formats:
      - id: mihomo-classical
        name: Classical
        template: mihomo-classical
      - id: mihomo-yaml
        name: YAML
        template: mihomo-yaml

  - id: sing-box
    name: sing-box
    icon: singbox
    template: singbox
    variants:
      - id: sing-box-non-ip
        name: Non-IP
        ops:
          - type: exclude_kinds
            kinds: [ip_cidr]

rules:
  # Local sources support inline content and file paths:
  # - content: |-
  #     DOMAIN,example.com
  # - file: ./rules/custom.list
  - id: Google
    name: Google
    description: Google services
    tags: [google, search]
    sources:
      - url: https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/Google.list
    outputs: [mihomo, sing-box]

  - id: AdBlock
    name: AdBlock
    sources:
      - url: https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/BanAD.list
    ops:
      - type: include_kinds
        kinds: [domain, domain_suffix, domain_keyword]
    outputs: [mihomo]

  - id: GeoBlock
    name: GeoBlock
    sources:
      # Compact geosite ref: provider/list or provider/list@attr1,attr2
      - geosite: v2fly/geolocation-!cn
    outputs: [mihomo, sing-box]

# Geosite auto-publish (optional)
# Declare providers + target clients. All lists and attr variants are
# automatically enumerated and published. No per-list declaration needed.
# Output naming: v2fly/google.list, v2fly/geolocation-!cn@ads.list, etc.
geosite:
  providers:
    - name: v2fly
      clients: [mihomo, sing-box]

update:
  history_retention: 168h
  history_limit: 200
  schedule:
    mode: manual
  fetch:
    timeout: 15s
    max_download: 4MB
    concurrency: 4
    per_host_concurrency: 2
    retries: 2
    retry_delay: 500ms
    user_agent: Proxy-Rule-Manager/2.0

serve:
  # prm serve requires ADMIN_TOKEN in the process environment.
  host: 127.0.0.1
  port: 3001
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate an example config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := cfgFile
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%s already exists, refusing to overwrite", target)
		}
		if err := os.WriteFile(target, []byte(exampleConfig), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		fmt.Printf("Created %s — edit it and run `prm update`.\n", target)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
