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

# HTTP 服务：提供公开站点和管理 API，也让你能跑 prm serve。
# 管理看板需要 ADMIN_TOKEN 环境变量，否则 serve 拒绝启动。
serve:
  host: 127.0.0.1              # 只允许本机访问；容器或公网部署改用 0.0.0.0
  port: 3001
  # 位于反向代理（Nginx、Caddy 等）后面时，serve 需要知道谁可信，
  # 才会信任其转发协议头，保证管理看板的 HTTPS 判断和 Cookie 设置正确。
  # 127.0.0.1/32 适用于本机反代；172.16.0.0/12 适用于 Docker 容器间反代。
  trusted_proxies: ["127.0.0.1/32", "172.16.0.0/12"]
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
