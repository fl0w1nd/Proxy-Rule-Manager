<p align="center">
  <img src="public/logo.svg" width="100" height="100" alt="Proxy Rule Manager Logo">
</p>

# Proxy Rule Manager

面向代理规则与客户端配置的管理平台，提供规则编辑、客户端管理与公开分发。

## 功能

- 规则编辑与管理
- 客户端管理
- 配置文件编辑与管理
- 公开分享与下载
- 同步
- 备份与恢复
- 管理员认证

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
      # 可选：自定义初始化模板路径
      # - INITIAL_CONFIG_PATH=/app/templates/initial-config.json
```

### 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `PORT` | 服务端口 | `3000` |
| `DATA_DIR` | 数据目录 | `./data` |
| `ADMIN_TOKEN` | 管理员令牌（空则无需认证） | 空 |
| `INITIAL_CONFIG_PATH` | 初始化模板路径（可选） | 空 |

## 开发者

### 环境要求

- Node.js >= 18
- npm >= 9

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
npm install

# 同时启动前端和后端（热重载）
npm run dev

# 或分别启动
npm run dev:fe   # 前端（3000）
npm run dev:be   # 后端（3001）
```

### 构建与运行

```bash
npm run build          # 构建前端
npm run build:server   # 构建后端
npm run start          # 启动生产服务
```

### 测试

```bash
npm run test
npm run test:watch
```
