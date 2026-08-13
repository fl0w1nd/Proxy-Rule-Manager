# Proxy Rule Manager 配置指南

这份文档讲解 `config.yaml` 怎么配置，配合 [config.template.yaml](../config.template.yaml) 使用：文档讲思路和场景，模板给完整字段参考。

## 心智模型

先记住三句话：

- **clients** 决定输出方向：规则渲染给哪些代理客户端，各自用什么格式。
- **rules** 决定编译内容：一条规则从哪些来源取数，怎么合并、怎么过滤，最终发给哪些客户端。
- **update** 决定更新方式：什么时候自动更新，以及网络请求和预处理限制。
- **运行时参数** 决定数据目录、HTTP 监听地址、端口、可信代理和管理令牌。

## 最小配置

下面这份配置可以直接跑通，也是 `prm init` 生成的内容：

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

逐行说明：

| 配置 | 含义 |
| --- | --- |
| `id: mihomo` | 客户端唯一标识，全文通用引用值 |
| `formats` 里的 `id` | 一个具体产物（`rules/mihomo-classical/` 目录名） |
| `template` | 使用哪个内置模板渲染（见模板列表） |
| `rule.id` | 规则唯一标识，同时作为输出文件名的核心 |
| `sources` | 至少一个来源；这里是一条远程 URL 规则列表 |
| `outputs: [mihomo]` | 这条规则发布给哪些客户端（引用 clients 里的 id） |

其他字段全部有默认值，先不用管。

## 先跑起来

```bash
prm init                          # 生成最小可运行配置（config.yaml）
prm validate                      # 校验配置，报错带 YAML 行号
prm preview Google                # 单独编译一条规则，看各阶段结果
prm update                        # 全量编译，产物写入 data/rules/
PRM_ADMIN_TOKEN=secret prm serve  # 启动站点 + 管理 API
```

`prm preview` 是排查利器：它会分别显示每个来源解析出多少条目、过滤前/后各多少、按类型统计，还能用 `--target` 指定某个格式看渲染结果。

## clients：客户端与格式

三种声明方式，按需选择：

1. **多格式**（上面的 Mihomo）：一个客户端多个产物，每个 `formats` 项是一个完整产物；
2. **单格式**：客户端下面直接写 `template`，产物目录名就是客户端 id；
3. **变体**：在基础产物之外再派生一份。变体在"规则过滤完成之后"额外执行一次 `ops`，适合"全量 + 精简版"的组合：

```yaml
- id: sing-box
  name: sing-box
  template: singbox
  variants:
    - id: sing-box-non-ip
      name: Non-IP
      ops:
        - type: exclude_kinds
          kinds: [ip_cidr]     # 产物里去掉所有 IP 段规则
```

内置模板与产物格式：

| 模板 | 客户端 | 格式 | 产物扩展名 |
| --- | --- | --- | --- |
| `mihomo-classical` | Mihomo | Classical 行列表 | `.list` |
| `mihomo-yaml` | Mihomo | YAML rule provider | `.yaml` |
| `singbox` | sing-box | JSON rule-set | `.json` |
| `surge` | Surge | 行列表 | `.list` |
| `shadowrocket` | Shadowrocket | 行列表 | `.list` |

`icon` 可选值：`prm`、`mihomo`、`singbox`、`shadowrocket`、`surge`；不写则按客户端 id 推断。自定义模板放在 `data/templates/`，同名时覆盖内置模板。

## rules：一条规则 = 一次编译

更新时每条规则按固定顺序处理：**读来源 -> 合并 -> 过滤（ops）-> 渲染**。

### 来源 sources（五选一）

| 类型 | 写法 | 适用场景 |
| --- | --- | --- |
| 远程 URL | `url: https://...` | 社区现成规则集（blackmatrix7、ACL4SSR 等） |
| 内联文本 | `content: \|-\n    DOMAIN,example.com` | 几条零散规则，不想建文件 |
| 本地文件 | `file: custom.list` | 自维护规则，放在 `data/local/` 下 |
| 引用 | `ref: base_rules` | 复用别的规则，如"常见网站"汇总多条规则 |
| geosite | `geosite: v2fly/google@cn` | 直接用域名库列表 |

要点：

- `format` 可显式指定 `classical` 或 `mihomo-yaml`，默认 `auto` 自动识别；
- 本地文件路径永远以 `data/local/` 为根，写相对路径即可；
- 引用别的规则时无需关心顺序，系统会按依赖图先编译被引用的规则；
- 每个来源可加 `label`，出错时用它定位。

### 过滤 ops

| 操作 | 作用 |
| --- | --- |
| `include_kinds` | 只保留指定类型的条目 |
| `exclude_kinds` | 移除指定类型的条目 |
| `filter_values` | 删除"值"匹配 pattern 的条目 |

`filter_values` 的模式：`keyword`（默认，包含即匹配）、`suffix`、`prefix`、`exact`、`regex`。

常用规则类型（kind）一览：

| kind | 含义 |
| --- | --- |
| `domain` | 精确域名 |
| `domain_suffix` | 域名及所有子域 |
| `domain_keyword` | 域名含关键词 |
| `domain_regex` | 域名正则 |
| `ip_cidr` / `ip_asn` / `geoip` | IP 段 / ASN / 地理 IP 库 |
| `process_name` / `process_path` | 按进程名 / 路径匹配 |
| `user_agent` / `url_regex` | 按 UA / URL 匹配 |

### 合并策略

多个来源合并时：`union` 并集（默认）、`intersect` 只留交集、`difference` 用第一个来源减去其余来源。

### 预处理 preprocess（可选）

来源数据不符合期望时，可在解析前用一段 JavaScript 重写内容。脚本必须定义 `process(content)` 并返回转换后的字符串。例：把 DNS 配置格式转成规则行。

```yaml
preprocess: |-
  function process(content) {
    return content.split(/\r?\n/).map(function(line) {
      var match = line.match(/^server=\/([^/]+)\/114\.114\.114\.114$/);
      return match ? "DOMAIN-SUFFIX," + match[1] : line;
    }).join("\n");
  }
```

预处理只作用于该规则的 `url`、`content`、`file` 来源；`ref` 和 `geosite` 来源不经过它。

### outputs

`outputs` 引用 `clients` 里声明的 id。客户端会自动展开：一个多格式客户端会为这条规则生成多个产物文件。

## geosite 域名库

geosite 有内置下载器，支持两个源：`v2fly`、`loyalsoldier`。两种用法：

1. 作为规则来源（见上表），写法 `provider/list` 或 `provider/list@attr`，如 `v2fly/google@cn` 取 google 列表的 cn 属性变体；
2. 自动发布：把某个 provider 的全部列表和属性变体同步发布到目标客户端，不用逐个声明规则：

```yaml
geosite:
  providers:
    - name: v2fly
      clients: [mihomo, sing-box]
```

产物路径形如 `rules/<客户端>/geosite/v2fly/google.list`。

## update：调度与限制

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `history_retention` | 168h | 更新历史最长保留时间 |
| `history_limit` | 200 | 更新历史最大条数（1..10000） |
| `schedule.mode` | manual | `manual` / `interval` / `cron`；自动调度只在 `serve` 运行时生效 |
| `schedule.interval` | - | interval 模式的执行间隔，如 `30m` |
| `schedule.cron` | - | 标准五段 cron 表达式，如 `"0 3 * * *"`，配合 `timezone` |
| `fetch.timeout` | 15s | 单次下载超时 |
| `fetch.max_download` | 4MB | 单个来源最大下载量 |
| `fetch.concurrency` | 4 | 全局并发（1..64） |
| `fetch.per_host_concurrency` | 2 | 同主机并发（1..concurrency） |
| `fetch.retries` | 2 | 失败重试（0..10） |
| `fetch.retry_delay` | 500ms | 重试基础间隔 |
| `fetch.user_agent` | Proxy-Rule-Manager/2.0 | 请求 UA |
| `preprocess.timeout` | 5s | 单次 JS 预处理超时 |
| `preprocess.max_output` | 8MB | 预处理结果体积上限 |

## 运行时参数

文件系统路径与 HTTP 进程参数通过 CLI flag 或 `PRM_*` 环境变量设置，优先级为 CLI flag > 环境变量 > 默认值。

| 用途 | CLI flag | 环境变量 | 默认值 |
| --- | --- | --- | --- |
| 数据目录 | `--data-dir` | `PRM_DATA_DIR` | `./data` |
| 监听地址 | `serve --host` | `PRM_SERVE_HOST` | `127.0.0.1` |
| 监听端口 | `serve --port` | `PRM_SERVE_PORT` | `3001` |
| 可信代理 | 重复 `serve --trusted-proxy` | `PRM_TRUSTED_PROXIES`（CSV） | 空列表 |
| 管理令牌 | — | `PRM_ADMIN_TOKEN` | 必填 |

可信代理接受单 IP 或 CIDR。管理 API 的写操作要求 Bearer 令牌或同源会话 Cookie，Cookie 为 HttpOnly + SameSite=Strict。

## 环境变量插值

任何字符串字段都能用 `${ENV_NAME}` 插入环境变量值，适合放访问令牌或内网地址：

```yaml
sources:
  - url: ${PRIVATE_RULES_URL}    # 需要导出 PRIVATE_RULES_URL
```

## 常见问题

- **报了错但不知道在哪**：`prm validate` 的错误都带 YAML 行号和配置路径，直接看行号。
- **本地文件读不到**：文件必须放在 `data/local/` 下，配置里写相对路径。
- **引用不生效**：确认被引用的规则 id 存在且拼写一致；引用关系会自动按依赖排序。
- **某条规则编译失败影响全局吗**：`update` 会继续编译其余规则，失败规则在结果里单独列出。
- **模板被我改坏了怎么办**：`data/templates/` 里的同名自定义模板覆盖内置模板，删掉即可恢复内置。
