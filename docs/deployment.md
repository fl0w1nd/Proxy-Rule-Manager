# 部署指南

prm 有两种部署方式，核心区别在**有没有管理能力**：

| 能力 | 自托管 | GitHub Pages |
| --- | --- | --- |
| 规则站点（公开页、产物下载） | 有 | 有 |
| 管理看板 `/admin`、管理 API `/api/v1` | 有 | 无 |
| 手动触发更新、查看更新历史/变更 | 有 | 无 |
| 定时自动更新 | 有（`update.schedule`） | 有（工作流每日构建） |
| 需要常驻服务器 | 是 | 否 |

- **自托管**：直接运行二进制，或用 Docker Compose 跑容器，管理能力完整。适合个人环境，配合代理客户端使用。
- **GitHub Pages 静态发布**：Actions 定时把规则编译成静态站并发布。只有公开站点，没有 `/admin` 和 API，纯只读。

两种方式都要求先有一份可用的 `config.yaml`。没有的话先执行 `prm init` 并 `prm validate`。

### v0.0.4 运行时参数迁移

`config.yaml` 删除 `data_dir` 和整个 `serve` 区块。对应设置迁移到 `--data-dir` / `PRM_DATA_DIR`、`serve --host` / `PRM_SERVE_HOST`、`serve --port` / `PRM_SERVE_PORT`、`serve --trusted-proxy` / `PRM_TRUSTED_PROXIES`。管理令牌变量统一为 `PRM_ADMIN_TOKEN`。旧字段会由严格配置校验返回 unknown-field 错误。

## 方式一：自托管

### A. 直接运行二进制

1. 从 [Releases 页面](https://github.com/fl0w1nd/Proxy-Rule-Manager/releases) 下载对应平台的压缩包（`prm_<版本>_<系统>_<架构>.tar.gz`），解压出 `prm`。
2. 把 `config.yaml` 和 `data/` 放在同一目录。
3. 首次更新并启动：

   ```bash
   export PRM_DATA_DIR=./data
   export PRM_ADMIN_TOKEN=<强随机令牌>
   ./prm update
   ./prm serve
   ```

`serve` 会按 `update.schedule` 的配置自动执行定时更新。想停止服务直接按 `Ctrl-C`。

### B. Docker Compose

镜像发布在 `ghcr.io/fl0w1nd/prm`，支持 `linux/amd64` 和 `linux/arm64`。建议用固定版本标签，`latest` 只在每次新版本发布时更新。

项目根目录放一份 `docker-compose.yml`，`config.yaml` 放在 `./data/` 下：

```text
项目目录/
├── docker-compose.yml
└── data/
    ├── config.yaml       # 配置文件
    ├── local/            # 本地文件来源
    └── rules/            # 产物（由 prm 生成）
```

```yaml
services:
  prm:
    image: ghcr.io/fl0w1nd/prm:latest
    container_name: prm
    restart: unless-stopped
    ports:
      - "3001:3001"
    environment:
      PRM_ADMIN_TOKEN: ${PRM_ADMIN_TOKEN:?请在 .env 中设置 PRM_ADMIN_TOKEN}
      # PRM_DATA_DIR: /data
      # PRM_SERVE_HOST: 0.0.0.0
      # PRM_SERVE_PORT: 3001
      # PRM_TRUSTED_PROXIES: 127.0.0.1/32,172.16.0.0/12
    volumes:
      - ./data:/data       # 整个目录，包含 config.yaml 和全部数据
```

启动：

```bash
# .env 文件里写一行：PRM_ADMIN_TOKEN=你的强随机令牌
docker compose up -d
docker compose logs -f prm     # 看启动日志
```

容器内 `WORKDIR` 是 `/data`，默认从 `/data/config.yaml` 读取配置。镜像通过 `PRM_DATA_DIR=/data` 固定运行数据目录，产物、缓存和历史都写入该卷。空卷首次启动时会生成纯业务配置。

镜像运行时默认值为 `PRM_SERVE_HOST=0.0.0.0`、`PRM_SERVE_PORT=3001`、`PRM_TRUSTED_PROXIES=127.0.0.1/32,172.16.0.0/12`。Compose 的 `environment` 可覆盖这些值。

只挂目录、不挂单文件，是为了让配置修改能正确生效。单文件挂载依赖文件 inode，宿主机上用编辑器改写文件后容器内不会联动；目录挂载则总是读到最新内容。

修改配置后重新加载：

```bash
docker compose exec prm prm validate
docker compose exec prm prm update
```

升级版本：改 `image:` 里的标签，然后 `docker compose up -d`。

### 反向代理与安全注意事项

prm 自带的 HTTP 服务只做规则提供和管理 API，生产环境通常在它前面放一个反向代理（Nginx、Caddy）负责 TLS 和域名。注意以下几点：

- **监听地址**：本地默认监听 `127.0.0.1`；容器镜像默认监听 `0.0.0.0`。通过 `--host` 或 `PRM_SERVE_HOST` 调整。
- **代理头与 HTTPS 识别**：prm 用 `--trusted-proxy` 或 `PRM_TRUSTED_PROXIES` 声明的网段判断可信转发头。填写反代所在网段或 IP（如 `127.0.0.1/32` 或 `10.0.0.0/8`）。Nginx 侧传递 `X-Forwarded-Proto`：

  ```nginx
  location / {
      proxy_pass http://127.0.0.1:3001;
      proxy_set_header Host $host;
      proxy_set_header X-Forwarded-Proto $scheme;
  }
  ```

- **令牌**：`PRM_ADMIN_TOKEN` 使用足够随机的长字符串，通过环境变量注入。管理接口 `/admin` 和 `/api/v1` 都需要它。
- **可信代理覆盖**：Compose 可设置 `PRM_TRUSTED_PROXIES=127.0.0.1/32,10.0.0.0/8`；每项接受单 IP 或 CIDR。
- **数据持久化**：`./data` 卷保存规则产物、geosite 缓存和更新历史。迁移或备份时整卷复制即可。
- **端口暴露**：只把反代端口暴露到公网，prm 的 `3001` 端口保持内网可达即可。

## 方式二：GitHub Pages 静态发布

把规则站点发布成公开页面，无需常驻服务器。核心流程：Fork 仓库 → 在 `publish` 分支维护配置 → 启用 Actions → 工作流每天自动构建并发布。

完整步骤见 [github-pages.md](github-pages.md)。
