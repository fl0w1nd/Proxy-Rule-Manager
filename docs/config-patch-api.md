# Config Patch API

Config Patch API 为管理界面提供带版本检查的配置事务。所有端点位于 `/api/v1`，沿用管理 API 的 Bearer 令牌或同源会话 Cookie 鉴权。

## 读取配置

`GET /config` 返回源 YAML 对应的结构化数据。`${ENV}` 占位符保持原样，`version` 是进程内配置版本，初始值为 `1`。

```json
{
  "version": 3,
  "config": {
    "clients": [],
    "rules": []
  }
}
```

`GET /config/dirty` 通过内容摘要检查磁盘文件是否发生外部修改：

```json
{ "changed": true }
```

`POST /config/reload` 导入当前磁盘文件并切换运行时配置。内容变化时版本递增。

## 提交 Patch

`POST /config/patch` 接受一个 JSON 对象，请求体上限为 1 MiB：

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
        "sources": [{ "url": "https://${RULE_HOST}/OpenAI.list" }],
        "outputs": ["mihomo"]
      }
    }
  ]
}
```

操作按照数组顺序应用，完成后统一执行环境变量展开、默认值处理和完整配置校验。同一事务可以先解除引用，再删除被引用对象。

| 操作 | 请求字段 |
| --- | --- |
| `add_client` | `value` |
| `update_client` | `id`, `value`；`value.id` 与 `id` 相同 |
| `remove_client` | `id` |
| `add_rule` | `value` |
| `update_rule` | `id`, `value`；`value.id` 与 `id` 相同 |
| `remove_rule` | `id` |
| `add_output`, `remove_output` | `rule_id`, `output_id` |
| `batch_add_output`, `batch_remove_output` | `rule_ids`, `output_ids` |
| `reorder_rules` | `order`；包含完整且唯一的规则 ID 集合 |
| `update_schedule` | `value` |
| `update_fetch` | `value` |
| `update_preprocess` | `value` |
| `update_history` | `value`，包含 `history_retention` 与 `history_limit` |
| `update_geosite` | `value`；`null` 清除 geosite 配置 |

客户端、规则和设置更新采用整对象替换。输出增删采用幂等语义；配置内容保持一致时返回当前版本。

成功响应表示配置已原子写入磁盘并切换到 Server、更新管理器、Engine、Fetcher、Preprocessor 与 Scheduler：

```json
{
  "version": 4,
  "warnings": []
}
```

## 错误

错误使用统一封装：

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

| HTTP 状态 | 场景 |
| --- | --- |
| `200` | Patch 或 reload 已提交；响应包含 `version` 和 `warnings` |
| `409` | `config_version_conflict`、`config_dirty` 或 `update_in_progress` |
| `413` | 请求体超过 1 MiB |
| `415` | Content-Type 不是 `application/json` |
| `422` | 请求结构、Patch 操作或完整配置校验失败 |

版本冲突的 `details` 包含 `current_version` 和当前源配置，客户端可据此刷新编辑状态。

## YAML 保证

Patch 在源 YAML AST 副本上执行。未触及节点保留注释、字段顺序、标量样式和 `${ENV}` 占位符；整对象替换区域使用规范化 YAML。写入使用配置文件同目录的临时文件和原子 rename。

Patch、reload 和更新任务共享配置变更临界区。活跃更新任务会产生 `update_in_progress`，模板引用和新增输出目录在提交前完成预检。

## TypeScript

`web/src/api/client.ts` 导出 `ConfigSnapshot`、`ConfigPatchOp`、`ConfigMutationResult`、`ConfigValidationIssue` 和 `APIRequestError`。调用方式：

```ts
const snapshot = await api.getConfig();
const result = await api.patchConfig(snapshot.version, [
  { op: 'add_output', rule_id: 'OpenAI', output_id: 'mihomo' },
]);
```
