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

## 方式一：自托管

### A. 直接运行二进制

1. 从 [Releases 页面](https://github.com/fl0w1nd/Proxy-Rule-Manager/releases) 下载对应平台的压缩包（`prm_<版本>_<系统>_<架构>.tar.gz`），解压出 `prm`。
2. 把 `config.yaml` 和 `data/` 放在同一目录，`data_dir: ./data`。
3. 首次更新并启动：

   ```bash
   export ADMIN_TOKEN=<强随机令牌>
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
    image: ghcr.io/fl0w1nd/prm:0.0.1
    container_name: prm
    restart: unless-stopped
    ports:
      - "3001:3001"
    environment:
      ADMIN_TOKEN: ${ADMIN_TOKEN:?请在 .env 中设置 ADMIN_TOKEN}
      TRUSTED_PROXY_CIDR: '["127.0.0.1/32", "172.16.0.0/12"]'
    volumes:
      - ./data:/data       # 整个目录，包含 config.yaml 和全部数据
```

启动：

```bash
# .env 文件里写一行：ADMIN_TOKEN=你的强随机令牌
docker compose up -d
docker compose logs -f prm     # 看启动日志
```

容器内 `WORKDIR` 是 `/data`，默认从 `/data/config.yaml` 读取配置。把 `config.yaml` 放进挂载的 `data/` 目录后，它就落在 `/data/config.yaml`，无需单独挂载文件。`data_dir` 保持 `/data`，产物、缓存、历史都写在这个卷里。

只挂目录、不挂单文件，是为了让配置修改能正确生效。单文件挂载依赖文件 inode，宿主机上用编辑器改写文件后容器内不会联动；目录挂载则总是读到最新内容。

修改配置后重新加载：

```bash
docker compose exec prm prm validate
docker compose exec prm prm update
```

升级版本：改 `image:` 里的标签，然后 `docker compose up -d`。

### 反向代理与安全注意事项

prm 自带的 HTTP 服务只做规则提供和管理 API，生产环境通常在它前面放一个反向代理（Nginx、Caddy）负责 TLS 和域名。注意以下几点：

- **监听地址**：`serve.host` 默认 `127.0.0.1`。容器或远程反代部署必须改成 `0.0.0.0`，否则反代连不上。只在本机用则保持 `127.0.0.1`。
- **代理头与 HTTPS 识别**：prm 用 `serve.trusted_proxies` 判断哪些请求可以携带转发头。反代后面如果不配置，cookie 的 `Secure` 标记和同源校验会判断错误；只写反代所在网段或 IP（如 `["127.0.0.1"]` 或 `["10.0.0.0/8"]`），不要写 `0.0.0.0/0`，否则公网客户端可伪造转发头。Nginx 侧记得传递 `X-Forwarded-Proto`：

  ```nginx
  location / {
      proxy_pass http://127.0.0.1:3001;
      proxy_set_header Host $host;
      proxy_set_header X-Forwarded-Proto $scheme;
  }
  ```

- **令牌**：`ADMIN_TOKEN` 必须是足够随机的长字符串，只通过环境变量注入，不要写进配置文件或提交到仓库。管理接口 `/admin` 和 `/api/v1` 都需要它。
- **反代网段也可用环境变量**：config 的字符串支持 `${VAR}` 插值（解析前整体替换），compose 场景不必把 IP 写死在 config 里。

  默认推荐覆盖最常见反代场景的一组值（本机反代 + Docker 容器反代）：

  ```yaml
  # docker-compose.yml
  environment:
    TRUSTED_PROXY_CIDR: '["127.0.0.1/32", "172.16.0.0/12"]'
  ```
  ```yaml
  # config.yaml
  serve:
    trusted_proxies: ${TRUSTED_PROXY_CIDR}
  ```

  变量未设置时占位符原样保留，校验会因地址无法解析而报错，正好提示你漏配了。
- **数据持久化**：`./data` 卷保存规则产物、geosite 缓存和更新历史。迁移或备份时整卷复制即可。
- **端口暴露**：只把反代端口暴露到公网，prm 的 `3001` 端口保持内网可达即可。

## 方式二：GitHub Pages 静态发布

把规则站点发布成公开页面，无需常驻服务器。核心流程：Fork 仓库 → 在 `publish` 分支维护配置 → 启用 Actions → 工作流每天自动构建并发布。

完整步骤见 [github-pages.md](github-pages.md)。
