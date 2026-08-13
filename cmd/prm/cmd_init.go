package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const exampleConfig = `# Proxy Rule Manager 最小可用配置
# 这份配置可以直接运行：cp 到 config.yaml 后执行 prm update。
# 完整字段与讲解见 config.template.yaml 或 docs/configuration.md。

# 数据目录：规则产物、缓存、更新历史都写在这里。
data_dir: ./data

# 输出客户端：规则渲染给哪些代理客户端、用什么格式。
clients:
  - id: mihomo
    name: Mihomo
    formats:
      - id: mihomo-classical
        name: Classical
        template: mihomo-classical

# 规则：一条规则 = 一次编译单元（读来源 -> 合并 -> 过滤 -> 渲染）。
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
