import { z } from "zod";

// 代理客户端类型 - 改为动态字符串，不再硬编码
export const ClientTypeSchema = z.string();
export type ClientType = z.infer<typeof ClientTypeSchema>;

// 客户端配置 Schema
export const ClientConfigSchema = z.object({
  id: z.string(), // 客户端标识符（用于内部引用）
  displayName: z.string(), // 显示名称
  pathName: z.string(), // 路径名称（用于文件系统目录和 URL）
  transforms: z.array(z.lazy(() => TransformSchema)).optional(), // 客户端全局转换器
});
export type ClientConfig = z.infer<typeof ClientConfigSchema>;

// 默认客户端配置
export const DEFAULT_CLIENTS: ClientConfig[] = [
  { id: "clash_meta", displayName: "Clash Meta / Stash", pathName: "Clash Meta" },
  { id: "shadowrocket", displayName: "Shadowrocket", pathName: "Shadowrocket" },
];

// 兼容旧代码的显示名称映射（运行时动态生成）
export let CLIENT_DISPLAY_NAMES: Record<string, string> = {
  clash_meta: "Clash Meta / Stash",
  shadowrocket: "Shadowrocket",
};

// 兼容旧代码的路径名称映射（运行时动态生成）
export let CLIENT_PATH_NAMES: Record<string, string> = {
  clash_meta: "Clash Meta",
  shadowrocket: "Shadowrocket",
};

// 更新客户端映射的函数
export function updateClientMappings(clients: ClientConfig[]): void {
  CLIENT_DISPLAY_NAMES = {};
  CLIENT_PATH_NAMES = {};
  for (const client of clients) {
    CLIENT_DISPLAY_NAMES[client.id] = client.displayName;
    CLIENT_PATH_NAMES[client.id] = client.pathName;
  }
}

// 数据来源类型
export const SourceTypeSchema = z.enum(["url", "ref", "local"]);
export type SourceType = z.infer<typeof SourceTypeSchema>;

// 统一的数据来源配置
export const SourceConfigSchema = z.object({
  type: SourceTypeSchema.optional().default("url"),
  // URL 来源
  url: z.string().optional(),
  // 引用其他规则
  ref: z.string().optional(),
  // 本地内容
  content: z.string().optional(),
  // 本地内容引用（外部文件）
  contentRef: z.string().optional(),
  // 来源名称/备注
  name: z.string().optional(),
});
export type SourceConfig = z.infer<typeof SourceConfigSchema>;

// 后处理操作类型（简化版）
export const TransformTypeSchema = z.enum(["use", "replace", "remove_lines"]);
export type TransformType = z.infer<typeof TransformTypeSchema>;

// 后处理操作
export const TransformSchema = z.object({
  // 操作类型
  type: TransformTypeSchema,
  // 目标来源：索引数组或 "all"
  target: z.union([z.array(z.number()), z.literal("all")]).default("all"),
  // use 类型：引用预定义转换器名称
  use: z.string().optional(),
  // replace/remove_lines 类型：正则模式
  pattern: z.string().optional(),
  // replace 类型：替换内容
  replacement: z.string().optional(),
  // 正则标志
  flags: z.string().optional(),
});
export type Transform = z.infer<typeof TransformSchema>;

// 兼容旧版转换器格式
export const LegacyTransformStepSchema = z.union([
  z.object({
    type: z.literal("replace"),
    pattern: z.string(),
    replacement: z.string(),
    flags: z.string().optional(),
  }),
  z.object({
    type: z.literal("remove_lines"),
    pattern: z.string(),
  }),
  z.object({
    type: z.literal("regex_extract"),
    pattern: z.string(),
    template: z.string(),
  }),
  z.object({
    type: z.literal("dedupe"),
  }),
  z.object({
    type: z.literal("sort"),
    order: z.enum(["asc", "desc"]).optional(),
  }),
  z.object({
    type: z.literal("trim"),
  }),
  z.object({
    type: z.literal("normalize_eol"),
  }),
]);

// 旧版转换器引用
export const LegacyTransformerSchema = z.union([
  LegacyTransformStepSchema,
  z.object({ use: z.string() }),
]);
export type Transformer = z.infer<typeof LegacyTransformerSchema>;

// 合并策略
export const MergeStrategySchema = z.enum(["concat", "union", "intersect"]);
export type MergeStrategy = z.infer<typeof MergeStrategySchema>;

// 合并配置
export const MergeConfigSchema = z.object({
  strategy: MergeStrategySchema.optional().default("concat"),
  dedupe: z.boolean().optional().default(false),
});
export type MergeConfig = z.infer<typeof MergeConfigSchema>;

// 客户端输出配置
export const ClientOutputConfigSchema = z.object({
  enabled: z.boolean().default(true),
  useGlobalTransforms: z.boolean().optional().default(true), // 是否使用客户端全局转换器
  transforms: z.array(TransformSchema).optional().default([]), // 规则级别的额外转换器
});
export type ClientOutputConfig = z.infer<typeof ClientOutputConfigSchema>;

// 输出配置
export const OutputConfigSchema = z.object({
  clients: z.array(ClientTypeSchema),
  client_overrides: z.record(z.string(), ClientOutputConfigSchema).optional(),
});
export type OutputConfig = z.infer<typeof OutputConfigSchema>;

// 规则配置
export const RuleConfigSchema = z.object({
  // 规则 ID（用于生成 URL 路径）
  name: z.string(),
  // 显示名称（用于界面显示，可选，默认使用 name）
  displayName: z.string().optional(),
  // 规则描述
  description: z.string().optional(),
  // 图标
  icon: z.string().optional(),
  // 数据来源（混合模式）
  sources: z.array(SourceConfigSchema).optional(),
  // 兼容旧版：聚合规则
  compose_from: z.array(z.string()).optional(),
  // 后处理转换（新版，支持指定来源）
  transforms: z.array(TransformSchema).optional(),
  // 兼容旧版后处理
  post_transforms: z.array(LegacyTransformerSchema).optional(),
  // 合并配置
  merge: MergeConfigSchema.optional(),
  // 输出配置
  output: OutputConfigSchema,
});
export type RuleConfig = z.infer<typeof RuleConfigSchema>;

// 预定义转换器（JS 脚本模式）
export const ScriptTransformerSchema = z.object({
  name: z.string(),
  description: z.string().optional(),
  // JS 脚本内容
  script: z.string(),
  // 创建时间
  createdAt: z.string().optional(),
  // 更新时间
  updatedAt: z.string().optional(),
});
export type ScriptTransformer = z.infer<typeof ScriptTransformerSchema>;

// 预定义转换器配置
export const TransformersConfigSchema = z.record(z.string(), ScriptTransformerSchema);
export type TransformersConfig = z.infer<typeof TransformersConfigSchema>;

// 完整的编排配置文件
export const RulesConfigSchema = z.object({
  version: z.number().default(1),
  transformers: TransformersConfigSchema.optional().default({}),
  rules: z.array(RuleConfigSchema),
});
export type RulesConfig = z.infer<typeof RulesConfigSchema>;

// 产物元数据
export const ArtifactMetaSchema = z.object({
  ruleName: z.string(),
  client: ClientTypeSchema,
  lastHash: z.string(),
  lastUpdatedAt: z.string(),
  blobPath: z.string(),
  blobUrl: z.string().optional(),
  sizeBytes: z.number().optional(),
});
export type ArtifactMeta = z.infer<typeof ArtifactMetaSchema>;

// 任务状态
export const JobStatusSchema = z.enum(["pending", "running", "completed", "failed"]);
export type JobStatus = z.infer<typeof JobStatusSchema>;

// 任务记录
export const JobRecordSchema = z.object({
  jobId: z.string(),
  type: z.enum(["full_sync", "partial_sync"]),
  status: JobStatusSchema,
  startedAt: z.string(),
  completedAt: z.string().optional(),
  affectedRules: z.array(z.string()).optional(),
  changedRules: z.array(z.string()).optional(),
  failedRules: z.array(z.object({
    name: z.string(),
    error: z.string(),
  })).optional(),
  logs: z.array(z.string()).optional(),
});
export type JobRecord = z.infer<typeof JobRecordSchema>;

// 统计数据
export const DailyStatsSchema = z.object({
  date: z.string(),
  syncCount: z.number().default(0),
  blobWriteCount: z.number().default(0),
  rulesChanged: z.number().default(0),
  totalRulesProcessed: z.number().default(0),
  failedSources: z.number().default(0),
});
export type DailyStats = z.infer<typeof DailyStatsSchema>;

// 定时同步配置（强制开启，不可禁用）
export const SyncScheduleSchema = z.object({
  // 同步间隔（小时），最小 1 小时，默认 24 小时
  intervalHours: z.number().min(1).default(24),
  // 上次定时同步时间
  lastScheduledSyncAt: z.string().optional(),
  // 下次同步时间
  nextSyncAt: z.string().optional(),
});
export type SyncSchedule = z.infer<typeof SyncScheduleSchema>;

// 默认定时同步配置
export const DEFAULT_SYNC_SCHEDULE: SyncSchedule = {
  intervalHours: 24,
};

// 默认的空配置
export const DEFAULT_CONFIG: RulesConfig = {
  version: 1,
  transformers: {},
  rules: [],
};

// 验证函数
export function validateConfig(config: unknown): RulesConfig {
  return RulesConfigSchema.parse(config);
}

// 验证单个规则
export function validateRule(rule: unknown): RuleConfig {
  return RuleConfigSchema.parse(rule);
}
