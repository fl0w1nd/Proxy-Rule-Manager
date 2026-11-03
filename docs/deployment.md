# 部署指南

`prm` 支持**自托管常驻服务**与 **GitHub Pages 静态发布**两种部署模式：

| 维度 | 自托管服务（`prm serve`） | GitHub Pages 静态发布（`prm build`） |
| --- | --- | --- |
| 公开规则索引页与产物下载 | 支持 | 支持 |
| Web 管理控制台（`/admin`）与 REST API（`/api/v1`） | 支持 | 不支持 |
| 在线编辑、实时调试预览、配置快照回滚 | 支持 | 不支持 |
| 自动更新机制 | 内置调度器（`interval` / `cron`） | GitHub Actions 定时工作流 |
| 基础设施要求 | 常驻进程或容器环境 | 仅需 GitHub 仓库 |

本文档说明自托管服务的部署与运维配置；无服务器静态部署流程见 [GitHub Pages 静态发布指南](github-pages.md)。

## 二进制直接部署

1. 从 [Releases](https://github.com/fl0w1nd/Proxy-Rule-Manager/releases) 下载对应操作系统与架构的压缩包（`prm_<version>_<os>_<arch>.tar.gz`）并解压。
2. 准备配置文件与数据目录：
   ```bash
   ./prm init
   ./prm validate
   ```
3. 执行首次规则编译并启动 HTTP 服务：
   ```bash
   export PRM_DATA_DIR=./data
   export PRM_ADMIN_TOKEN=<设置高强度随机令牌>

   ./prm update
   ./prm serve
   ```

`prm serve` 默认监听 `127.0.0.1:3001`，并根据 `config.yaml` 中的 `update.schedule` 配置自动运行后台定时任务。

## Docker Compose 部署

官方容器镜像发布于 `ghcr.io/fl0w1nd/prm`，支持 `linux/amd64` 与 `linux/arm64` 架构。

### 目录结构

容器内工作目录为 `/data`，默认读取 `/data/config.yaml`，并将运行数据固定写入 `/data`。建议将宿主机的 `./data` 目录整体挂载至 `/data`（避免单文件挂载在原子重命名保存配置时因 inode 变更导致宿主机与容器文件脱节）：

```text
project/
├── docker-compose.yml
├── .env
└── data/
    ├── config.yaml       # 配置文件（若为空目录，首次启动时会自动生成默认配置）
    ├── local/            # 本地规则文件
    └── rules/            # 编译产物
```

### `docker-compose.yml`

```yaml
services:
  prm:
    image: ghcr.io/fl0w1nd/prm:latest
    container_name: prm
    restart: unless-stopped
    ports:
      - "127.0.0.1:3001:3001"
    environment:
      PRM_ADMIN_TOKEN: ${PRM_ADMIN_TOKEN:?请在 .env 中设置 PRM_ADMIN_TOKEN}
      # 镜像内置默认环境变量（按需覆盖）：
      # PRM_DATA_DIR: /data
      # PRM_SERVE_HOST: 0.0.0.0
      # PRM_SERVE_PORT: 3001
      # PRM_TRUSTED_PROXIES: 127.0.0.1/32,172.16.0.0/12
    volumes:
      - ./data:/data
```

启动与日常运维命令：

```bash
# 启动服务
docker compose up -d

# 查看运行日志
docker compose logs -f prm

# 命令行校验配置或手动触发更新
docker compose exec prm prm validate
docker compose exec prm prm update
```

## HTTP 路由与服务接口

| 路径 | 鉴权要求 | 说明 |
| --- | --- | --- |
| `/`、`/index.html` | 公开 | 规则索引页（支持客户端切换、标签过滤、产物链接复制与 Geo 目录检索） |
| `/rules/*` | 公开 | 编译生成的各客户端规则产物 |
| `/geo/*` | 公开 | 启用 `host: true` 时发布的原始 `.dat` 与 `.mmdb` 数据库文件 |
| `/static/icons/*`、`/static/assets/*` | 公开 | 客户端图标与公开页前端静态资源 |
| `/admin` | 需鉴权 | Web 管理控制台 |
| `/api/v1/*` | 需鉴权 | 管理与控制 REST API（配置读写见 [Config Patch API](config-patch-api.md)） |

鉴权与并发控制说明：

- **身份验证**：`/api/v1` 接受 `Authorization: Bearer <PRM_ADMIN_TOKEN>` 请求头，或通过 `/admin` 登录建立的同源会话 Cookie（`HttpOnly`、`SameSite=Strict`）。
- **更新触发**：`POST /api/v1/updates` 通过请求体 `{"scope": "..."}` 触发更新，支持 `all`（全量）、`rules`（配合 `rule_ids` 数组更新指定规则及其下游依赖）、`geosite`、`geoip`、`mmdb`、`asn`。同一时间仅允许运行一个更新任务，并发触发将返回 `409 Conflict`。

## 数据目录结构

运行时数据目录（默认 `./data`）包含全部编译产物、缓存与状态文件，同一时刻通过 `.state/prm.lock` 排他锁防止多进程并发写入：

```text
data/
├── rules/              # 渲染后的客户端规则产物（公开于 /rules/）
│   ├── mihomo-classical/
│   │   ├── OpenAI.list
│   │   └── geosite/v2fly/google.list
│   └── sing-box-non-ip/
│       └── OpenAI.json
├── geo/                # 托管的原始 Geo 数据库（公开于 /geo/）
│   ├── geosite/v2fly/dlc.dat
│   ├── geoip/v2fly/geoip.dat
│   ├── mmdb/loyalsoldier/Country.mmdb
│   └── asn/loyalsoldier/GeoLite2-ASN.mmdb
├── local/              # 本地规则源（对应 sources 中的 file 相对路径）
├── templates/          # 自定义模板（同名时覆盖内置模板）
├── static/
│   ├── assets/         # 公开页前端静态资源（随版本自动同步）
│   └── icons/          # 内置与自定义客户端图标
├── geosite/            # Geosite 解析缓存与上游原始文件（raw/）
├── geoip/              # GeoIP 解析缓存与上游原始文件（raw/）
├── mmdb/               # MMDB 数据库缓存
├── asn/                # ASN 数据库缓存
└── .state/             # 运行状态快照、更新历史记录与配置自动备份（backups/）
```

备份或迁移时，直接完整备份 `config.yaml` 与 `data/` 目录即可。

## 反向代理配置

生产环境建议将 `prm` 置于 Nginx 或 Caddy 等反向代理之后以提供 HTTPS 接入。

当通过 HTTPS 反向代理访问 `/admin` 时，`prm` 需要识别请求的真实协议以设置 `Secure` Cookie 并执行同源校验。请确保：

1. 通过 `--trusted-proxy` 或环境变量 `PRM_TRUSTED_PROXIES` 声明反向代理所在的 IP 或网段（例如 `127.0.0.1/32` 或容器网段 `172.16.0.0/12`）。
2. 在反向代理中转发 `Host` 与 `X-Forwarded-Proto` 请求头。

Nginx 配置示例：

```nginx
server {
    listen 443 ssl http2;
    server_name rules.example.com;

    # SSL 证书配置省略

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
