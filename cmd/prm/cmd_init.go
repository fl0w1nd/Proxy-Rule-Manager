package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const exampleConfig = `# Proxy Rule Manager 最小配置示例
# 完整字段参考见 config.template.yaml，详细配置指南见 docs/configuration.md。
# 数据目录与 HTTP 服务参数通过 CLI 参数或 PRM_* 环境变量设置。

# 目标代理客户端与输出格式
clients:
  - id: mihomo
    name: Mihomo
    formats:
      - id: mihomo-classical
        name: Classical
        template: mihomo-classical

# 规则编译单元
rules:
  - id: Google
    name: Google
    sources:
      - url: https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/Google.list
    outputs: [mihomo]

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
