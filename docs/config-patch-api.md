# 配置管理与 Patch API 规范

本文档定义 `/api/v1/config` 系列接口的请求与响应契约。所有接口均位于 `/api/v1` 路径下，要求携带 `Authorization: Bearer <PRM_ADMIN_TOKEN>` 请求头或有效的同源管理会话 Cookie。

## 版本与并发控制模型

配置管理器在进程内维护单调递增的配置版本号 `version`（进程启动时初始值为 `1`）及磁盘配置文件的内容摘要（Digest）：

- **乐观并发控制**：修改类接口（`POST /config/patch`、`POST /config/raw`、`POST /config/backups/{id}/restore`）必须在请求体中携带当前 `version`。若与服务端 `version` 不一致，接口返回 `409 Conflict`（错误码 `config_version_conflict`）。
- **外部修改检测**：提交写入前，服务端校验磁盘文件摘要是否与内存快照一致。若文件在进程外被修改，接口返回 `409 Conflict`（错误码 `config_dirty`），客户端需先调用 `POST /config/reload` 同步磁盘状态。
- **更新互斥**：当后台存在正在执行的规则更新任务时，配置写入请求返回 `409 Conflict`（错误码 `update_in_progress`）。
- **原子持久化与热重载**：校验通过的配置在YAML AST 上完成更新，通过同目录临时文件原子重命名写入磁盘，随后即时切换运行时配置并重建公开规则索引页。

## 接口列表

| 方法与路径 | 说明 |
| --- | --- |
| `GET /api/v1/config` | 获取结构化配置快照与当前 `version` |
| `GET /api/v1/config/raw` | 获取磁盘原始 YAML 文本、文件路径与当前 `version` |
| `POST /api/v1/config/validate` | 校验原始 YAML 文本的语法、语义及模板引用（不写盘） |
| `POST /api/v1/config/raw` | 全量保存原始 YAML 文本并触发热重载 |
| `POST /api/v1/config/patch` | 按事务顺序执行一组结构化 Patch 操作并触发热重载 |
| `GET /api/v1/config/dirty` | 检查磁盘配置文件是否发生进程外修改 |
| `POST /api/v1/config/reload` | 重新加载磁盘配置文件并更新内存快照 |
| `GET /api/v1/config/backups` | 获取自动备份快照列表 |
| `GET /api/v1/config/backups/{id}` | 获取指定备份快照的原始 YAML 文本 |
| `POST /api/v1/config/backups/{id}/restore` | 将配置回滚至指定备份快照并触发热重载 |

## 读取与同步配置

### 获取结构化配置

`GET /api/v1/config` 返回源 YAML 解析后的结构化对象（保留显式声明值，未回填运行时默认值）：

```json
{
  "version": 3,
  "config": {
    "clients": [],
    "rules": []
  }
}
```

### 获取与保存原始 YAML

- `GET /api/v1/config/raw` 返回 `{ "yaml": "...", "path": "config.yaml", "version": 3 }`。
- `POST /api/v1/config/validate` 接收 `{ "yaml": "..." }`，校验通过返回 `{ "valid": true, "errors": [] }`。
- `POST /api/v1/config/raw` 接收 `{ "yaml": "...", "version": 3 }`，校验通过后写入磁盘并返回 `{ "version": 4, "warnings": [] }`。

### 外部修改检查与重载

- `GET /api/v1/config/dirty` 返回 `{ "changed": true | false }`。
- `POST /api/v1/config/reload` 重新加载磁盘上的 `config.yaml`，内容发生变化时递增 `version`，返回 `{ "status": "reloaded", "version": 4, "warnings": [] }`。

## 增量事务更新（`POST /api/v1/config/patch`）

接收 JSON 请求体（上限 `1 MiB`），按 `ops` 数组顺序在源 YAML AST 副本上依次应用变更，全部应用后统一执行完整配置校验与模板检查。未触及的 YAML 节点保留原有注释、字段顺序与标量样式。

### 请求示例

```json
{
  "version": 3,
  "ops": [
    {
      "op": "update_rule",
      "id": "OpenAI",
      "value": {
        "id": "OpenAI",
        "name": "OpenAI",
        "sources": [{ "url": "https://rules.example/OpenAI.list" }],
        "outputs": ["mihomo"]
      }
    }
  ]
}
```

### 支持的 Patch 操作

| 操作 (`op`) | 必填参数 | 语义说明 |
| --- | --- | --- |
| `add_client` | `value` | 新增客户端定义 |
| `update_client` | `id`, `value` | 整对象替换指定客户端（`value.id` 须与 `id` 一致） |
| `remove_client` | `id` | 删除指定客户端 |
| `add_rule` | `value` | 新增规则定义 |
| `update_rule` | `id`, `value` | 整对象替换指定规则（`value.id` 须与 `id` 一致） |
| `remove_rule` | `id` | 删除指定规则 |
| `add_output` / `remove_output` | `rule_id`, `output_id` | 为单条规则添加或移除输出客户端（幂等） |
| `batch_add_output` / `batch_remove_output` | `rule_ids`, `output_ids` | 批量为多条规则添加或移除输出客户端（幂等） |
| `reorder_rules` | `order` | 调整规则排列顺序（`order` 须包含全量且唯一的规则 ID） |
| `update_schedule` | `value` | 整对象替换 `update.schedule` |
| `update_fetch` | `value` | 整对象替换 `update.fetch` |
| `update_preprocess` | `value` | 整对象替换 `update.preprocess` |
| `update_history` | `value` | 更新 `update.history_retention` 与 `update.history_limit` |
| `update_geosite` | `value` | 整对象替换 `geosite`（传 `null` 清除该配置块） |
| `update_geoip` | `value` | 整对象替换 `geoip`（传 `null` 清除该配置块） |
| `update_mmdb` | `value` | 整对象替换 `mmdb`（传 `null` 清除该配置块） |
| `update_asn` | `value` | 整对象替换 `asn`（传 `null` 清除该配置块） |

> 注：对象更新类操作（`update_*`）采用**整对象替换**而非 JSON Merge Patch；`value` 中省略的可选字段将从 YAML 节点中移除，并在运行时回退至系统默认值。

### 成功响应

```json
{
  "version": 4,
  "warnings": []
}
```

## 自动备份与回滚

每次配置写入（Patch、Raw 保存或回滚）使文件内容发生变更前，系统自动将变更前的源 YAML 备份至 `<data-dir>/.state/backups/config-YYYYMMDD-HHmmss.yaml`（若与最新备份字节完全一致则跳过；目录最多保留最近 20 份快照，超出部分按时间滚动清理）。

- `GET /api/v1/config/backups`：返回当前 `version` 及按时间倒序排列的快照元数据列表（`id`、`created_at`、`size`）。
- `GET /api/v1/config/backups/{id}`：返回 `{ "id": "...", "yaml": "..." }`。
- `POST /api/v1/config/backups/{id}/restore`：请求体为 `{ "version": 3 }`，校验快照有效性后替换当前配置并触发热重载。

## 错误响应契约

所有错误均返回统一的 JSON 结构：

```json
{
  "error": {
    "code": "config_invalid",
    "message": "配置校验失败",
    "details": {
      "errors": [
        { "path": "rules[0].outputs[0]", "line": 12, "message": "unknown output" }
      ]
    }
  }
}
```

| HTTP 状态码 | 错误码 (`error.code`) | 触发条件 |
| --- | --- | --- |
| `404 Not Found` | `backup_not_found` | 指定的备份快照不存在或文件名非法 |
| `409 Conflict` | `config_version_conflict` | 请求携带的 `version` 与服务端当前版本不一致（`details` 附带 `current_version` 与最新 `config`） |
| `409 Conflict` | `config_dirty` | 磁盘配置文件已被外部进程修改，需先执行 `reload` |
| `409 Conflict` | `update_in_progress` | 存在正在运行的规则更新任务 |
| `413 Payload Too Large` | `request_too_large` | 请求体超过 `1 MiB` 上限 |
| `415 Unsupported Media Type` | `unsupported_media_type` | `Content-Type` 不是 `application/json` |
| `422 Unprocessable Entity` | `invalid_request` / `patch_invalid` / `config_invalid` | JSON 结构错误、Patch 参数不合法或变更后配置未通过校验 |

## 前端 SDK 调用示例

`web/src/api/client.ts` 提供类型化的配置管理封装（导出 `ConfigSnapshot`、`ConfigPatchOp`、`ConfigMutationResult`、`ConfigValidationIssue`、`ConfigBackupList` 与 `APIRequestError`）：

```ts
const snapshot = await api.getConfig();
const result = await api.patchConfig(snapshot.version, [
  { op: 'add_output', rule_id: 'OpenAI', output_id: 'mihomo' },
]);
```
