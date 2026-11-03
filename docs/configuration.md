# 配置指南

本文档说明 `config.yaml` 的结构与配置方法。完整的字段示例与默认值注释可参考仓库根目录下的 [config.template.yaml](../config.template.yaml)。

## 配置结构概览

`config.yaml` 由四个核心部分组成：

| 顶层字段 | 职责 | 是否必填 |
| --- | --- | --- |
| `clients` | 定义目标代理客户端、输出格式模板及派生变体 | 是 |
| `rules` | 定义规则编译单元：上游来源、预处理、合并策略、过滤操作与目标客户端 | 是 |
| `geosite` / `geoip` / `mmdb` / `asn` | 配置 Geo 数据库的批量客户端发布与原始数据库文件托管 | 否 |
| `update` | 配置自动更新计划、网络抓取并发与超时、预处理资源限制及历史记录保留策略 | 否 |

通用语法约定：

- **时长（Duration）**：采用 Go 标准时长格式，如 `500ms`、`15s`、`30m`、`168h`。
- **体积（Size）**：支持带单位字符串（如 `50MB`、`8MB`）或纯数字字节数。
- **校验与调试**：修改配置后可通过 `prm validate` 校验语法与引用（报错精确到 YAML 行号与字段路径），或使用 `prm preview <rule-id>` 查看单条规则在各阶段的编译结果。

## 最小配置示例

执行 `prm init` 会生成以下最小可运行配置：

```yaml
clients:
  - id: mihomo
    name: Mihomo
    formats:
      - id: mihomo-classical
        name: Classical
        template: mihomo-classical

rules:
  - id: Google
    name: Google
    sources:
      - url: https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/Google.list
    outputs: [mihomo]
```

| 字段 | 说明 |
| --- | --- |
| `clients[].id` | 客户端唯一标识，供 `rules[].outputs` 及 Geo 自动发布配置引用 |
| `clients[].formats[].id` | 具体输出目标的唯一标识，对应产物子目录名（如 `data/rules/mihomo-classical/`） |
| `clients[].formats[].template` | 渲染该产物所使用的模板名称 |
| `rules[].id` | 规则唯一标识（不可含路径分隔符），同时作为产物文件名与 `ref` 引用标识 |
| `rules[].sources` | 规则输入源列表（至少包含一个来源） |
| `rules[].outputs` | 该规则需要输出的目标客户端 `id` 列表 |

## 客户端与输出（`clients`）

每个客户端可通过以下三种方式定义输出产物：

1. **单格式客户端**：直接在客户端下指定 `template`，产物目录名即客户端 `id`。
2. **多格式客户端**：通过 `formats` 列表声明多个格式（如同时输出 Classical 与 YAML 格式），每个格式拥有独立的 `id` 与 `template`。
3. **派生变体（`variants`）**：在规则完成合并与规则级过滤后，额外执行一次变体专属的 `ops` 过滤并渲染为独立产物，常用于生成“全量规则 + 去除 IP 规则（Non-IP）”等组合。

```yaml
clients:
  - id: sing-box
    name: sing-box
    icon: singbox
    formats:
      - id: singbox
        name: Source
        template: singbox
      - id: singbox-binary
        name: Binary
        template: singbox-binary
    variants:
      - id: sing-box-non-ip
        name: Non-IP
        template: singbox
        ops:
          - type: exclude_kinds
            kinds: [ip_cidr]
```

### 内置模板

| 模板标识 | 适用客户端 | 输出格式 | 产物扩展名 |
| --- | --- | --- | --- |
| `mihomo-classical` | Mihomo | Classical 行列表 | `.list` |
| `mihomo-yaml` | Mihomo | YAML Rule Provider | `.yaml` |
| `singbox` | sing-box | Source Rule-Set（JSON） | `.json` |
| `singbox-binary` | sing-box | Binary Rule-Set（SRS v3） | `.srs` |
| `surge` | Surge | Classical 行列表 | `.list` |
| `shadowrocket` | Shadowrocket | Classical 行列表 | `.list` |

- **自定义模板**：将自定义模板文件放入 `<data-dir>/templates/` 目录即可引用；若与内置模板同名，将覆盖内置模板。
- **客户端图标（`icon`）**：内置支持 `mihomo`、`singbox`、`shadowrocket`、`surge`，省略时按客户端 `id` 自动匹配，也可引用 `<data-dir>/static/icons/` 下的自定义图标。

## 编译规则（`rules`）

单条规则的编译按固定顺序执行：**读取来源与预处理 -> 解析为中间表示（IR） -> 集合合并 -> 规则级过滤（`ops`） -> 客户端模板渲染**。

### 数据源（`sources`）

每项数据源支持设置可选的 `label`（用于日志与诊断定位），输入内容格式（经典行列表或 Mihomo YAML）由解析器自动识别。支持以下六种来源类型：

| 来源类型 | 配置写法 | 说明 |
| --- | --- | --- |
| 远程 URL | `url: https://...` | 拉取远程规则列表 |
| 内联文本 | `content: \|-\n  DOMAIN,example.com` | 直接在配置文件中内联声明规则条目 |
| 本地文件 | `file: custom.list` | 读取 `<data-dir>/local/` 下的相对路径文件 |
| 规则引用 | `ref: base_rules` | 复用另一条已声明规则的编译后 IR 结果（自动按依赖拓扑排序，禁止循环引用） |
| Geosite | `geosite: v2fly/google@cn` | 提取指定提供商的域名列表（格式为 `provider/list` 或 `provider/list@attr1,attr2`） |
| GeoIP | `geoip: loyalsoldier/cn` | 提取指定提供商的 IP 分类并展开为 IPv4 / IPv6 `ip_cidr` 条目 |

### 来源分组与局部处理（`group`）

`sources` 支持单个来源与分组（`group`，支持单层嵌套）混排。单个来源或分组均可独立配置局部 `preprocess` 与 `ops`：

```yaml
rules:
  - id: CustomService
    name: Custom Service
    preprocess: |-
      function process(content) { return content.trim(); }
    sources:
      - group:
          - url: https://example.com/a.list
          - url: https://example.com/b.list
        ops:
          - type: include_kinds
            kinds: [domain, domain_suffix]
      - content: DOMAIN,custom.example
        preprocess: ""
    merge:
      strategy: union
    outputs: [mihomo]
```

执行与继承规则：

- **预处理继承**：规则级的 `preprocess` 作为默认脚本；来源或分组省略 `preprocess` 时继承规则级脚本，指定非空字符串时覆盖默认脚本，显式设为空字符串 `""` 时禁用预处理。
- **分组求值顺序**：组内各成员分别执行预处理与解析，合并去重（并集）后执行组上的 `ops`；随后所有顶层来源与分组按规则级 `merge.strategy` 合并，最后执行规则级 `ops`。

### 合并策略（`merge`）

当规则包含多个顶层来源或分组时，通过 `merge.strategy` 指定集合运算方式（移动或增减顺序会影响差集结果），合并后自动去重：

| 策略 | 说明 |
| --- | --- |
| `union` | **并集（默认）**：保留所有来源的全部条目 |
| `intersect` | **交集**：仅保留在所有顶层来源中同时存在的条目 |
| `difference` | **差集**：以第一个顶层来源为基准，剔除后续来源中出现的条目 |

### 过滤操作（`ops`）

`ops` 按数组声明顺序依次执行，可用于规则级、来源/分组级以及客户端变体（`variants`）：

| 操作类型 (`type`) | 参数 | 说明 |
| --- | --- | --- |
| `include_kinds` | `kinds: [...]` | 仅保留指定类型的规则条目 |
| `exclude_kinds` | `kinds: [...]` | 移除指定类型的规则条目 |
| `filter_values` | `mode`, `pattern` | 移除规则值匹配 `pattern` 的条目；`mode` 支持 `keyword`（默认）、`suffix`、`prefix`、`exact`、`regex` |

常用规则类型（`kinds`）：

| 类型标识 | 含义 |
| --- | --- |
| `domain` | 完整域名匹配 |
| `domain_suffix` | 域名后缀匹配 |
| `domain_keyword` | 域名关键词匹配 |
| `domain_regex` | 域名正则表达式匹配 |
| `ip_cidr` / `ip_asn` / `geoip` | IP 网段（IPv4/IPv6 CIDR） / 自治系统号（ASN） / GeoIP 国家代码 |
| `process_name` / `process_path` | 进程名称 / 进程完整路径 |
| `user_agent` / `url_regex` | HTTP User-Agent / URL 正则表达式 |

### JavaScript 预处理（`preprocess`）

当上游文本并非标准规则格式（如 hosts、dnsmasq 配置或自定义格式）时，可在解析前通过 JavaScript 脚本改写原始文本。脚本需定义 `process(content)` 入口函数并返回处理后的字符串：

```yaml
preprocess: |-
  function process(content) {
    return content.split(/\r?\n/).map(function(line) {
      var match = line.match(/^server=\/([^/]+)\//);
      return match ? "DOMAIN-SUFFIX," + match[1] : line;
    }).join("\n");
  }
```

> 注：预处理仅作用于文本类来源（`url`、`content`、`file`）；`ref`、`geosite` 与 `geoip` 来源直接产出结构化条目，不经过文本预处理。

## Geo 数据库发布与托管

除在单条规则的 `sources` 中按需引用外，`prm` 还支持将上游 Geo 数据库的全量分类批量编译发布至指定客户端，或在公开站点直接托管原始数据库文件（供客户端 `geox-url` 等特性直接下载）。

Geo 数据库仅在执行更新任务（如 `prm update`、定时调度或后台触发更新）时联网拉取；`prm validate` 与 `prm preview` 仅读取本地缓存。

### Geosite 与 GeoIP（`geosite` / `geoip`）

支持 `v2fly` 与 `loyalsoldier` 两个上游提供商：

```yaml
geosite:
  providers:
    - name: v2fly
      clients: [mihomo, sing-box]
      host: true

geoip:
  providers:
    - name: loyalsoldier
      clients: [mihomo, sing-box]
      host: true
    - name: v2fly
      clients: [mihomo]
```

- **`clients`**：自动遍历该提供商的所有分类（Geosite 含 `@attr` 属性变体），按客户端所含格式与变体批量渲染输出至 `data/rules/<输出目标 ID>/geosite/<provider>/` 与 `data/rules/<输出目标 ID>/geoip/<provider>/`。
- **`host: true`**：将上游原始数据库文件发布至公开目录 `data/geo/geosite/<provider>/`（如 `dlc.dat`）或 `data/geo/geoip/<provider>/geoip.dat`，对应 HTTP 路径 `/geo/...`。

### MMDB 与 ASN 原始库托管（`mmdb` / `asn`）

用于托管 Mihomo 等客户端所需的 MaxMind 格式原始数据库，当前支持提供商 `loyalsoldier`（随全量更新、`prm update --geoip` 或通过管理 API 单独刷新时下载）：

```yaml
mmdb:
  providers:
    - name: loyalsoldier
      host: true

asn:
  providers:
    - name: loyalsoldier
      host: true
```

启用 `host: true` 后，文件分别发布于：

- `/geo/mmdb/loyalsoldier/Country.mmdb`
- `/geo/asn/loyalsoldier/GeoLite2-ASN.mmdb`

## 更新调度与资源限制（`update`）

`update` 字段控制历史保留策略、后台自动更新计划以及网络抓取与沙箱执行限制：

| 字段路径 | 默认值 | 说明 |
| --- | --- | --- |
| `history_retention` | `168h` | 更新历史记录的最长保留时间 |
| `history_limit` | `200` | 更新历史记录保留条数上限（范围 `1..10000`） |
| `schedule.mode` | `manual` | 调度模式：`manual`（仅手动触发）、`interval`（固定间隔）、`cron`（定时表达式，仅在 `prm serve` 运行时生效） |
| `schedule.interval` | — | `interval` 模式下的执行周期（如 `30m`、`6h`） |
| `schedule.cron` | — | `cron` 模式下的五段式 Cron 表达式（如 `"0 3 * * *"`） |
| `schedule.timezone` | `UTC` | Cron 表达式对应的时区（如 `Asia/Shanghai`、`UTC`） |
| `fetch.timeout` | `15s` | 远程下载的响应与传输停滞（stall）超时时间 |
| `fetch.max_download` | `50MB` | 单个远程资源的体积上限 |
| `fetch.concurrency` | `4` | 全局最大并发下载数（范围 `1..64`） |
| `fetch.per_host_concurrency` | `2` | 单一主机最大并发下载数（范围 `1..concurrency`） |
| `fetch.retries` | `2` | 下载失败重试次数（范围 `0..10`） |
| `fetch.retry_delay` | `500ms` | 重试退避基础间隔 |
| `fetch.user_agent` | `Proxy-Rule-Manager/2.0` | HTTP 请求 `User-Agent` |
| `preprocess.timeout` | `5s` | 单次 JavaScript 预处理脚本执行超时时间 |
| `preprocess.max_output` | `8MB` | 单次 JavaScript 预处理输出文本体积上限 |
