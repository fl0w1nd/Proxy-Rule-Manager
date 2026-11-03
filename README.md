# Proxy Rule Manager

<p align="center">
  <img src="web/src/assets/icons/brand/prm.svg" width="96" alt="prm logo">
</p>

<p align="center"><em>多源代理规则编译与分发工具，将上游规则集统一编译为各客户端专用格式。</em></p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26.10-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go version"></a>
  <a href="https://github.com/fl0w1nd/Proxy-Rule-Manager/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/fl0w1nd/Proxy-Rule-Manager/ci.yml?style=flat-square&branch=main&label=ci" alt="CI"></a>
  <a href="https://github.com/fl0w1nd/Proxy-Rule-Manager/releases"><img src="https://img.shields.io/github/v/release/fl0w1nd/Proxy-Rule-Manager?style=flat-square&sort=semver&label=release" alt="Release"></a>
  <a href="https://github.com/fl0w1nd/Proxy-Rule-Manager/stargazers"><img src="https://img.shields.io/github/stars/fl0w1nd/Proxy-Rule-Manager?style=flat-square&label=stars" alt="Stars"></a>
  <a href="https://github.com/fl0w1nd/Proxy-Rule-Manager/commits/main"><img src="https://img.shields.io/github/last-commit/fl0w1nd/Proxy-Rule-Manager?style=flat-square&label=last%20commit" alt="Last commit"></a>
</p>

Proxy Rule Manager（`prm`）通过声明式配置拉取、解析、合并与过滤多个上游代理规则源，并将其渲染为 Mihomo、sing-box、Surge、Shadowrocket 等客户端的规则格式。`prm` 既可作为命令行构建工具单次执行，也可作为附带 Web 管理界面的常驻 HTTP 服务运行，或导出为无后端的纯静态规则站点。

## 核心特性

- **多源输入与组合**：支持远程 URL、本地文件、内联规则、规则间引用（`ref`）以及 Geosite / GeoIP 分类库；支持分组局部处理与并集、交集、差集合并策略。
- **结构化过滤与预处理**：将经典行规则与 Mihomo YAML 统一解析为中间表示（IR，涵盖 40 余种规则类型及逻辑组合），支持按规则类型或匹配模式（关键词、前后缀、精确匹配、正则表达式）过滤，并提供解析前 JavaScript 预处理能力。
- **多格式与变体输出**：内置 Mihomo（Classical `.list` / YAML `.yaml`）、sing-box（Source `.json` / Binary `.srs`）、Surge、Shadowrocket 模板，支持为同一客户端派生附加过滤变体（如 Non-IP 规则集）及自定义模板覆盖。
- **Geo 数据库同步与托管**：支持将 `v2fly`、`loyalsoldier` 的 Geosite 与 GeoIP 分类批量编译至目标客户端，并可托管原始 `.dat`、`Country.mmdb` 与 `GeoLite2-ASN.mmdb` 文件。
- **可视化管理与静态发布**：内置 Web 公开索引页与 `/admin` 管理控制台（支持规则在线编辑、调试预览、变更对比、配置快照回滚与定时调度），同时支持一键导出静态站点部署至 GitHub Pages。

## 工作原理

每条规则（`rule`）对应一个独立的编译单元。执行更新时，`prm` 按依赖拓扑顺序执行以下流水线：

```mermaid
flowchart LR
    SRC[上游来源<br/>URL / 本地 / 引用 / Geo] --> PARSE[预处理与解析]
    PARSE --> MERGE[集合合并与去重]
    MERGE --> OPS[规则级与变体级过滤]
    OPS --> RENDER[模板渲染]
    RENDER --> OUT[输出产物<br/>data/rules/]
```

最小配置示例（`config.yaml`）：

```yaml
clients:
  - id: mihomo
    name: Mihomo
    formats:
      - id: mihomo-classical
        name: Classical
        template: mihomo-classical

rules:
  - id: OpenAI
    name: OpenAI
    sources:
      - url: https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/OpenAI/OpenAI.list
    ops:
      - type: include_kinds
        kinds: [domain, domain_suffix, ip_cidr]
    outputs: [mihomo]
```

## 快速开始

1. **获取二进制程序**
   从 [Releases](https://github.com/fl0w1nd/Proxy-Rule-Manager/releases) 下载对应平台的可执行文件，或在仓库根目录通过 `mise run build` 编译生成 `bin/prm`。
2. **初始化配置文件**
   ```bash
   prm init
   ```
   该命令在当前目录生成最小可运行的 `config.yaml`。
3. **校验并编译规则**
   ```bash
   prm validate
   prm update
   ```
   编译完成后，生成的规则文件位于 `./data/rules/`。
4. **启动 HTTP 服务（可选）**
   ```bash
   PRM_ADMIN_TOKEN=your-secret-token prm serve
   ```
   访问 `http://127.0.0.1:3001/` 查看公开规则索引页，访问 `http://127.0.0.1:3001/admin` 进入管理控制台。

## 命令行参考

### 子命令

| 命令 | 说明 |
| --- | --- |
| `prm init` | 生成最小示例配置 `config.yaml` |
| `prm validate` | 校验配置文件语法、模板引用及本地缓存的 Geosite / GeoIP 引用 |
| `prm update [rule-ids...]` | 执行全量更新，或仅编译指定规则及其下游依赖规则 |
| `prm update --geosite` | 更新 Geosite 数据库及自动发布产物 |
| `prm update --geoip` | 更新 GeoIP、MMDB、ASN 数据库及自动发布产物 |
| `prm preview <rule-id> [--target <id>]` | 预览单条规则在各编译阶段的条目统计，可通过 `--target` 查看指定格式的渲染内容 |
| `prm build [--output <dir>]` | 执行全量更新并将静态规则站点导出至目标目录（默认 `dist/`） |
| `prm serve` | 启动常驻 HTTP 服务、定时更新调度器与管理 API |

### 参数与环境变量

配置优先级为：**CLI 参数 > 环境变量 > 默认值**。

| 参数 | 环境变量 | 默认值 | 适用范围 | 说明 |
| --- | --- | --- | --- | --- |
| `-c, --config` | — | `config.yaml` | 全局 | 配置文件路径 |
| `--data-dir` | `PRM_DATA_DIR` | `./data` | 全局 | 运行时数据与产物目录 |
| `--host` | `PRM_SERVE_HOST` | `127.0.0.1` | `serve` | HTTP 监听 IP 地址 |
| `--port` | `PRM_SERVE_PORT` | `3001` | `serve` | HTTP 监听端口 |
| `--trusted-proxy` | `PRM_TRUSTED_PROXIES` | 空 | `serve` | 可信反向代理 IP 或 CIDR（CLI 可重复指定，环境变量用逗号分隔） |
| — | `PRM_ADMIN_TOKEN` | — | `serve` | 管理控制台与 API 鉴权令牌（非开发模式下必填） |
| — | `PRM_DEV` | 关闭 | `serve` | 设为 `1` 或 `true` 时启用开发模式，免除管理鉴权（仅限本地开发） |

## 部署与运行模式

`prm` 支持两种交付模式：

- **自托管服务（二进制 / Docker Compose）**：运行 `prm serve`，提供公开规则站（`/`、`/rules/`、`/geo/`）、Web 管理控制台（`/admin`）、REST API（`/api/v1`）以及内置定时调度（`interval` / `cron`）。详见 [部署指南](docs/deployment.md)。
- **无服务静态站点（GitHub Pages）**：通过 `prm build` 将公开索引页与全部产物导出为纯静态目录，结合 GitHub Actions 定时编译发布，无需常驻服务器。详见 [GitHub Pages 部署指南](docs/github-pages.md)。

## 开发指南

项目使用 [mise](https://mise.jdx.dev) 统一管理 Go、Node.js、pnpm、golangci-lint、protoc 等工具链版本（定义于 `mise.toml` 与 `mise.lock`）。

```bash
mise install      # 安装全部开发工具链
mise run dev      # 启动本地开发环境（Go 后端热重载 :3001 + Vite 管理前端 :5173/admin/）
mise run build    # 构建前端资源并编译 bin/prm
mise run ci       # 运行完整本地质量检查（版本一致性、代码检查、Go 竞态测试、前端测试）
```

常用开发任务：

| 任务 | 说明 |
| --- | --- |
| `mise run dev [-c <file>] [--data-dir <dir>]` | 启动前后端热重载开发服务（默认读取或初始化 `config.dev.yaml`，自动启用 `PRM_DEV=1`） |
| `mise run test` / `mise run test:race` | 运行 Go 单元测试（含覆盖率 / 竞态检测） |
| `mise run test:web` | 运行前端类型检查与组件测试 |
| `mise run lint` | 运行 `gofmt`、`go vet` 与 `golangci-lint` |
| `mise run proto` | 重新生成 Geosite 与 GeoIP 的 Protobuf 代码 |
| `mise run docker:build` / `docker:run` | 构建本地容器镜像 / 挂载 `./data` 启动测试容器 |
| `mise run clean` / `mise run distclean` | 清理构建产物（`bin/`、`tmp/`） / 额外清理运行数据（`data/`） |

## 文档索引

- [配置指南 (`docs/configuration.md`)](docs/configuration.md)：`config.yaml` 各模块结构、数据源类型、过滤操作、Geo 自动发布与更新策略详解。
- [配置模板 (`config.template.yaml`)](config.template.yaml)：带完整字段注释与默认值的参考配置模板。
- [部署指南 (`docs/deployment.md`)](docs/deployment.md)：二进制与 Docker Compose 部署、数据目录结构及反向代理配置。
- [GitHub Pages 静态发布 (`docs/github-pages.md`)](docs/github-pages.md)：基于 GitHub Actions 的无服务器静态规则站构建与发布流程。
- [Config Patch API (`docs/config-patch-api.md`)](docs/config-patch-api.md)：管理端配置快照、版本控制、增量 Patch 事务与回滚接口规范。
