<p align="center">
  <img src="public/logo.svg" width="100" height="100" alt="Proxy Rule Manager Logo">
</p>

# Proxy Rule Manager

面向代理规则与客户端配置的管理平台，提供规则编辑、客户端管理与公开分发。

## 预览

<p align="center">
  <img src="public/preview1.png" width="90%" alt="Dashboard Preview">
</p>

<p align="center">
  <img src="public/preview2.png" width="90%" alt="Rules Manager Preview">
</p>

<p align="center">
  <img src="public/preview3.png" width="90%" alt="Client Config Preview">
</p>


## 功能

- 规则编排与自动化管理，支持本地/远程数据源混合
- 规则更新记录追溯
- 客户端配置文件编辑与管理
- 公开分享与下载
- 数据源定时从上游同步
- 备份与恢复

## 安装（Docker Compose）

镜像：`ghcr.io/fl0w1nd/Proxy-Rule-Manager`

```yaml
services:
  proxy-rule-manager:
    image: ghcr.io/fl0w1nd/proxy-rule-manager:latest
    container_name: proxy-rule-manager
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - ./data:/app/data
    environment:
      - PORT=3000
      - NODE_ENV=production
      - ADMIN_TOKEN=your-secure-token-here
```

### 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `PORT` | 服务端口 | `3000` |
| `DATA_DIR` | 数据目录 | `./data` |
| `ADMIN_TOKEN` | 管理员令牌（空则无需认证） | 空 |

## 开发者

### 环境要求

- Node.js >= 18
- pnpm >= 10

### 项目结构

```
src/
  app/            # 前端页面
  components/     # UI 组件
  lib/            # 核心逻辑
  server/         # API
```

### 开发

```bash
pnpm install

# 同时启动前端和后端（热重载）
pnpm run dev

# 或分别启动
pnpm run dev:fe   # 前端（3000）
pnpm run dev:be   # 后端（3001）
```

### 构建与运行

```bash
pnpm run build          # 构建前端
pnpm run build:server   # 构建后端
pnpm run start          # 启动生产服务
```

### 测试

```bash
pnpm run test
pnpm run test:watch
```
