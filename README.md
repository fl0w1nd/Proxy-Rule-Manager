# Proxy Rule Manager

一个现代化的代理规则管理系统，支持多源聚合、规则转换、客户端适配，专为 Clash Meta / Shadowrocket 等代理客户端设计。

## ✨ 特性

- 🔄 **多源聚合**: 支持 URL、规则引用、本地内容三种数据来源
- 🔀 **智能合并**: 支持 concat、union、intersect 三种合并策略
- ⚡ **规则转换**: 内置正则替换、行删除、脚本转换等多种处理方式
- 🎯 **客户端适配**: 为不同客户端生成差异化规则配置
- 📊 **依赖管理**: 自动拓扑排序，检测循环依赖
- 🔒 **安全防护**: 内置 SSRF 防护，阻止私有网络请求
- ⏱️ **定时同步**: 支持配置自动同步间隔
- 🌙 **暗色主题**: 支持亮色/暗色主题切换

## 🛠️ 技术栈

- **前端**: Next.js 16, React 19, Tailwind CSS, Radix UI
- **后端**: Hono (轻量级 Web 框架)
- **数据验证**: Zod
- **测试**: Vitest

## 📦 快速开始

### 环境要求

- Node.js >= 18
- npm >= 9

### 安装

```bash
# 克隆仓库
git clone https://github.com/fl0w1nd/Proxy-Rule-Manager.git
cd Proxy-Rule-Manager

# 安装依赖
npm install
```

### 开发模式

```bash
# 启动后端 (端口 3001)
npm run start:dev

# 另一个终端启动前端 (端口 3000)
npm run dev
```

访问 http://localhost:3000 即可使用。

### 生产部署

```bash
# 构建前端
npm run build

# 构建后端
npm run build:server

# 启动服务
npm run start
```

## 📂 项目结构

```
src/
├── app/                    # Next.js 页面
├── components/             # React 组件
│   ├── dashboard.tsx       # 仪表盘
│   ├── rule-editor.tsx     # 规则编辑器
│   ├── rules-manager.tsx   # 规则管理
│   └── ...
├── lib/                    # 核心逻辑
│   ├── sync-engine.ts      # 同步引擎
│   ├── sync-engine/        # 同步引擎子模块
│   │   ├── dependency-graph.ts  # 依赖分析
│   │   ├── fetcher.ts           # 数据获取
│   │   └── processor.ts         # 规则处理
│   ├── transformer.ts      # 转换器
│   ├── schema.ts           # 数据模型
│   └── storage-adapter.ts  # 存储适配
└── server/                 # Hono 后端
    ├── index.ts            # 入口
    ├── routes/             # API 路由
    └── errors.ts           # 错误处理
```

## 🔧 配置说明

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | 3000 |
| `DATA_DIR` | 数据目录 | ./data |
| `ADMIN_TOKEN` | 管理员令牌 | (开发模式下可选) |
| `INITIAL_CONFIG_PATH` | 初始配置模板路径 | (可选) |

### 规则配置示例

```json
{
  "name": "YouTube",
  "displayName": "YouTube 视频",
  "description": "YouTube 相关域名规则",
  "sources": [
    {
      "type": "url",
      "url": "https://raw.githubusercontent.com/example/rules/YouTube.list"
    },
    {
      "type": "local",
      "content": "DOMAIN-SUFFIX,youtube.com"
    }
  ],
  "transforms": [
    {
      "type": "remove_lines",
      "target": "all",
      "pattern": "^#"
    }
  ],
  "merge": {
    "strategy": "concat",
    "dedupe": true
  },
  "output": {
    "clients": ["clash_meta", "shadowrocket"]
  }
}
```

## 🧪 测试

```bash
# 运行测试
npm run test

# 监听模式
npm run test:watch
```

当前测试覆盖:
- ✅ 依赖图分析 (循环检测、拓扑排序)
- ✅ 内容转换器 (替换、删除、合并)
- ✅ URL 安全校验 (SSRF 防护)

## 📝 API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/status` | 获取系统状态 |
| GET | `/api/config` | 获取配置 |
| PUT | `/api/config` | 保存配置 |
| POST | `/api/sync/full` | 全量同步 |
| POST | `/api/sync/partial/:name` | 局部同步 |
| GET | `/api/rules` | 获取规则列表 |
| GET | `/api/clients` | 获取客户端列表 |
| GET | `/Rules/:client/:file` | 获取规则文件 |

## 📄 许可证

MIT License

## 🙏 致谢

- [Hono](https://hono.dev/) - 轻量级 Web 框架
- [Next.js](https://nextjs.org/) - React 框架
- [Radix UI](https://www.radix-ui.com/) - 无障碍 UI 组件
