import { z } from "zod";

// 代理客户端类型 - 改为动态字符串，不再硬编码
export const ClientTypeSchema = z.string();
export type ClientType = z.infer<typeof ClientTypeSchema>;

// 默认输出文件后缀，沿用历史的 .list 行为
export const DEFAULT_OUTPUT_EXT = "list";

// 输出后缀允许的字符集：1-16 位的小写字母数字
export const OUTPUT_EXT_REGEX = /^[a-z0-9]{1,16}$/;

// 客户端配置 Schema
export const ClientConfigSchema = z.object({
  id: z.string(), // 客户端标识符（用于内部引用）
  displayName: z.string(), // 显示名称
  outputExt: z
    .string()
    .regex(OUTPUT_EXT_REGEX, "Output ext must be 1-16 lowercase letters or digits")
    .optional(),
  transforms: z.array(z.lazy(() => TransformSchema)).optional(), // 客户端全局转换器
});
export type ClientConfig = z.infer<typeof ClientConfigSchema>;

// 解析客户端输出后缀，空字符串返回默认 list；不做正则校验，仅做归一化
export function resolveOutputExt(ext?: string | null): string {
  if (!ext) return DEFAULT_OUTPUT_EXT;
  const trimmed = ext.trim().replace(/^\./, "").toLowerCase();
  return trimmed || DEFAULT_OUTPUT_EXT;
}

// 客户端配置文件元数据
export const ClientFileMetaSchema = z.object({
  id: z.string(),
  clientId: z.string(),
  configId: z.string().regex(/^[a-zA-Z0-9_-]+$/, "Config ID must only contain letters, numbers, hyphens, and underscores"), // 配置 id，决定客户端目录下的文件名
  displayName: z.string(), // 显示名称
  description: z.string().optional(), // 配置文件描述
  ext: z.string(),
  isPublic: z.boolean().default(false),
  createdAt: z.string(),
  updatedAt: z.string(),
});
export type ClientFileMeta = z.infer<typeof ClientFileMetaSchema>;

// 默认客户端配置
export const DEFAULT_CLIENTS: ClientConfig[] = [
  { id: "clash_meta", displayName: "Clash Meta / Stash" },
  { id: "shadowrocket", displayName: "Shadowrocket" },
];

// 显示名称映射（运行时动态生成）
export let CLIENT_DISPLAY_NAMES: Record<string, string> = {
  clash_meta: "Clash Meta / Stash",
  shadowrocket: "Shadowrocket",
};

// 更新客户端映射的函数
export function updateClientMappings(clients: ClientConfig[]): void {
  CLIENT_DISPLAY_NAMES = {};
  for (const client of clients) {
    CLIENT_DISPLAY_NAMES[client.id] = client.displayName;
  }
}

export const GeositeProviderSchema = z.enum(["v2fly", "loyalsoldier"]);
export type GeositeProvider = z.infer<typeof GeositeProviderSchema>;

export const GeositeRenderProfileSchema = z.enum(["mihomo-classical"]);
export type GeositeRenderProfile = z.infer<typeof GeositeRenderProfileSchema>;

// 数据来源类型
export const SourceTypeSchema = z.enum(["url", "ref", "local", "geosite"]);
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
  // Geosite 来源
  provider: GeositeProviderSchema.optional(),
  list: z.string().optional(),
  attrs: z.array(z.string()).optional(),
  renderProfile: GeositeRenderProfileSchema.optional(),
  // 来源名称/备注
  name: z.string().optional(),
});
export type SourceConfig = z.infer<typeof SourceConfigSchema>;

// 后处理操作类型（简化版）
export const TransformTypeSchema = z.enum(["use", "replace", "remove_lines"]);
export type TransformType = z.infer<typeof TransformTypeSchema>;

// 后处理操作
//
// 内置转换器的参数（如 mihomo→shadowrocket 的映射表）统一放在
// RulesConfig.builtinParams 里：每个 builtin 整个部署只配一次，
// rule / client / override 的 Transform 都只引用名字，不重复配置。
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
  // 后处理转换（新版，支持指定来源）
  transforms: z.array(TransformSchema).optional(),
  // 合并配置
  merge: MergeConfigSchema.optional(),
  // 输出配置
  output: OutputConfigSchema,
  // 标签（用于分类和筛选）
  tags: z.array(z.string()).optional().default([]),
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

// 内置转换器的参数表：键为内置转换器名（如 "builtin:mihomo-to-shadowrocket"），
// 值为该转换器自定义解码的 JSON 对象（前端不做强 schema，
// 由后端 validateBuiltinParams 校验）。
export const BuiltinParamsConfigSchema = z.record(z.string(), z.unknown());
export type BuiltinParamsConfig = z.infer<typeof BuiltinParamsConfigSchema>;

// 完整的编排配置文件
export const RulesConfigSchema = z.object({
  version: z.number().default(1),
  transformers: TransformersConfigSchema.optional().default({}),
  builtinParams: BuiltinParamsConfigSchema.optional().default({}),
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
  // 同步模式：interval 或 cron
  mode: z.enum(["interval", "cron"]).default("interval"),
  // 同步间隔（小时），最小 1 小时，默认 24 小时
  intervalHours: z.number().min(1).default(24),
  // cron 表达式（支持 5/6 段）
  cronExpression: z.string().optional(),
  // 上次定时同步时间
  lastScheduledSyncAt: z.string().optional(),
  // 下次同步时间
  nextSyncAt: z.string().optional(),
});
export type SyncSchedule = z.infer<typeof SyncScheduleSchema>;

// 默认定时同步配置
export const DEFAULT_SYNC_SCHEDULE: SyncSchedule = {
  mode: "interval",
  intervalHours: 24,
  cronExpression: "0 0 * * *",
};

// CDN 缓存设置（用于 Cloudflare 等 CDN）
export const CdnSettingsSchema = z.object({
  // 是否启用自定义响应头
  enabled: z.boolean().default(false),
  // 缓存模式
  // "no-cache": 每次请求都验证源站，但源站不可用时使用旧缓存（推荐）
  // "no-store": 完全不缓存
  // "custom": 自定义 Cache-Control
  cacheMode: z.enum(["no-cache", "no-store", "custom"]).default("no-cache"),
  // stale-if-error 时长（秒），源站不可用时缓存兜底时间
  staleIfErrorSeconds: z.number().min(0).default(604800), // 默认 7 天
  // 自定义 Cache-Control 值（仅当 cacheMode 为 custom 时使用）
  customCacheControl: z.string().optional(),
  // Cloudflare CDN 专用头（可选）
  cloudflareCdnCacheControl: z.string().optional(),
  // 额外的自定义响应头
  customHeaders: z.array(z.object({
    name: z.string(),
    value: z.string(),
  })).optional().default([]),
});
export type CdnSettings = z.infer<typeof CdnSettingsSchema>;

// 默认 CDN 设置
export const DEFAULT_CDN_SETTINGS: CdnSettings = {
  enabled: false,
  cacheMode: "no-cache",
  staleIfErrorSeconds: 604800,
  customCacheControl: undefined,
  cloudflareCdnCacheControl: undefined,
  customHeaders: [],
};

// 系统级运行时参数
// 这些参数原本写死在后端常量里，现在通过 /api/system-settings 暴露，并随
// 数据库备份一起归档。所有字段都是正整数；零值由后端 MergeDefaults 回填。
export const SystemSettingsSchema = z.object({
  fetch: z.object({
    timeoutSeconds: z.number().int().min(1).max(600).default(15),
    maxDownloadMB: z.number().int().min(1).max(256).default(4),
    perHostConcurrency: z.number().int().min(1).max(64).default(4),
    userAgent: z.string().min(1).max(200).default("Proxy-Rule-Manager/1.0"),
  }),
  transformer: z.object({
    timeoutMs: z.number().int().min(100).max(60000).default(5000),
    maxOutputMB: z.number().int().min(1).max(256).default(8),
  }),
  rateLimit: z.object({
    baseDelaySeconds: z.number().int().min(1).max(600).default(5),
    maxBlockSeconds: z.number().int().min(60).max(86400).default(3600),
    permanentBanLimit: z.number().int().min(1).max(1000).default(10),
    recordMaxAgeHours: z.number().int().min(1).max(720).default(24),
  }),
  sync: z.object({
    // 连续失败多少次后规则在面板上显示「更新失败」徽标
    failureThreshold: z.number().int().min(1).max(50).default(3),
  }),
});
export type SystemSettings = z.infer<typeof SystemSettingsSchema>;

export const DEFAULT_SYSTEM_SETTINGS: SystemSettings = {
  fetch: {
    timeoutSeconds: 15,
    maxDownloadMB: 4,
    perHostConcurrency: 4,
    userAgent: "Proxy-Rule-Manager/1.0",
  },
  transformer: {
    timeoutMs: 5000,
    maxOutputMB: 8,
  },
  rateLimit: {
    baseDelaySeconds: 5,
    maxBlockSeconds: 3600,
    permanentBanLimit: 10,
    recordMaxAgeHours: 24,
  },
  sync: {
    failureThreshold: 3,
  },
};

// 默认的空配置
export const DEFAULT_CONFIG: RulesConfig = {
  version: 1,
  transformers: {},
  builtinParams: {},
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

// ----- Built-in transformer registry (mirrors backend/transformer/builtin.go) -----

// 内置转换器统一前缀，前端用于识别只读项 + 渲染锁图标
export const BUILTIN_TRANSFORMER_PREFIX = "builtin:";

export function isBuiltinTransformerName(name: string): boolean {
  return name.startsWith(BUILTIN_TRANSFORMER_PREFIX);
}

// 内置转换器元数据（GET /api/config 透传过来的列表）
export const BuiltinTransformerSchema = z.object({
  name: z.string(),
  description: z.string().optional().default(""),
});
export type BuiltinTransformer = z.infer<typeof BuiltinTransformerSchema>;

// ----- mihomo → shadowrocket 映射表（params 形态，对应 backend builtin.go） -----

export const SHADOWROCKET_ACTIONS = ["keep", "rename", "drop"] as const;
export type ShadowrocketAction = (typeof SHADOWROCKET_ACTIONS)[number];

export const ShadowrocketMappingSchema = z.object({
  type: z.string(),
  action: z.enum(SHADOWROCKET_ACTIONS),
  renameTo: z.string().optional(),
  reason: z.string().optional(),
});
export type ShadowrocketMapping = z.infer<typeof ShadowrocketMappingSchema>;

export const ShadowrocketParamsSchema = z.object({
  rules: z.array(ShadowrocketMappingSchema).default([]),
  unknownAction: z.enum(SHADOWROCKET_ACTIONS).optional().default("keep"),
});
export type ShadowrocketParams = z.infer<typeof ShadowrocketParamsSchema>;

// Default table mirrors backend DefaultShadowrocketMapping; used to seed
// the editor on first interaction and as a "restore defaults" target.
export const DEFAULT_SHADOWROCKET_MAPPING: ShadowrocketMapping[] = [
  { type: "DOMAIN", action: "keep" },
  { type: "DOMAIN-SUFFIX", action: "keep" },
  { type: "DOMAIN-KEYWORD", action: "keep" },
  { type: "IP-CIDR", action: "keep" },
  { type: "IP-CIDR6", action: "keep" },
  { type: "GEOIP", action: "keep" },
  { type: "SRC-IP-CIDR", action: "keep" },
  { type: "DST-PORT", action: "keep" },
  { type: "SRC-PORT", action: "keep" },
  { type: "IN-PORT", action: "keep" },
  { type: "PROTOCOL", action: "keep" },
  { type: "NETWORK", action: "keep" },
  { type: "USER-AGENT", action: "keep" },
  { type: "URL-REGEX", action: "keep" },
  { type: "FINAL", action: "keep" },
  { type: "MATCH", action: "rename", renameTo: "FINAL", reason: "Shadowrocket 用 FINAL 替代 MATCH" },
  { type: "PROCESS-NAME", action: "drop", reason: "Shadowrocket 不支持 PROCESS-NAME" },
  { type: "PROCESS-PATH", action: "drop", reason: "Shadowrocket 不支持 PROCESS-PATH" },
  { type: "IP-ASN", action: "drop", reason: "Shadowrocket 不支持 IP-ASN" },
  { type: "DOMAIN-REGEX", action: "drop", reason: "Shadowrocket 无 DOMAIN-REGEX 等价规则（URL-REGEX 语义不同）" },
  { type: "RULE-SET", action: "drop", reason: "Shadowrocket 不支持内联 RULE-SET 引用" },
  { type: "SUB-RULE", action: "drop", reason: "Shadowrocket 不支持 SUB-RULE" },
  { type: "AND", action: "drop", reason: "Shadowrocket 不支持逻辑组合规则 AND" },
  { type: "OR", action: "drop", reason: "Shadowrocket 不支持逻辑组合规则 OR" },
  { type: "NOT", action: "drop", reason: "Shadowrocket 不支持逻辑组合规则 NOT" },
];

// ----- mihomo classical → sing-box rule-set source (params, mirrors backend builtin_singbox.go) -----

export const SINGBOX_SOURCE_ACTIONS = ["map", "drop"] as const;
export type SingboxSourceAction = (typeof SINGBOX_SOURCE_ACTIONS)[number];

// SINGBOX_SOURCE_FIELDS enumerates the sing-box headless-rule fields the
// backend runner knows how to emit. Kept in lockstep with
// SingboxSourceFields() in backend/internal/transformer/builtin_singbox.go;
// adding a new field here without a matching backend entry would silently
// produce invalid JSON, so any change needs to land in both places.
export const SINGBOX_SOURCE_FIELDS = [
  "domain",
  "domain_suffix",
  "domain_keyword",
  "domain_regex",
  "source_ip_cidr",
  "ip_cidr",
  "source_port",
  "source_port_range",
  "port",
  "port_range",
  "process_name",
  "process_path",
  "process_path_regex",
  "package_name",
  "package_name_regex",
  "network",
  "network_type",
  "wifi_ssid",
  "wifi_bssid",
] as const;
export type SingboxSourceField = (typeof SINGBOX_SOURCE_FIELDS)[number];

// SINGBOX_SOURCE_FIELD_MIN_VERSION mirrors singboxFieldMinVersion in
// backend/internal/transformer/builtin_singbox.go: each entry is the
// minimum rule-set source-format version that introduces the field.
// Fields absent from this table are available since version 1 (the
// floor), so the UI doesn't need to special-case them.
//
// Keep this in lockstep with the backend table. Adding a new field
// without an entry here would silently let the dropdown offer it on
// version 1, which the backend validator will then reject — a poor UX.
export const SINGBOX_SOURCE_FIELD_MIN_VERSION: Partial<Record<SingboxSourceField, number>> = {
  process_path_regex: 2,
  network_type: 3,
  wifi_ssid: 3,
  wifi_bssid: 3,
  package_name_regex: 5,
};

export function singboxFieldMinVersion(field: string): number {
  return SINGBOX_SOURCE_FIELD_MIN_VERSION[field as SingboxSourceField] ?? 1;
}

// sing-box rule-set schema versions accepted by the runner. The ceiling
// matches the latest version documented in source-format.md; raising it
// requires bumping MaxSingboxSourceVersion in the backend as well.
export const SINGBOX_SOURCE_VERSIONS = [1, 2, 3, 4, 5] as const;
export type SingboxSourceVersion = (typeof SINGBOX_SOURCE_VERSIONS)[number];
export const DEFAULT_SINGBOX_SOURCE_VERSION: SingboxSourceVersion = 3;

export const SingboxSourceMappingSchema = z.object({
  type: z.string(),
  action: z.enum(SINGBOX_SOURCE_ACTIONS),
  // mapTo is unconstrained here so the editor can re-render a stale row
  // pointing at a removed field; the dropdown still limits new picks to
  // SINGBOX_SOURCE_FIELDS and the backend rejects unknown values at
  // save time.
  mapTo: z.string().optional(),
  reason: z.string().optional(),
});
export type SingboxSourceMapping = z.infer<typeof SingboxSourceMappingSchema>;

export const SingboxSourceParamsSchema = z.object({
  version: z.number().int().optional(),
  rules: z.array(SingboxSourceMappingSchema).default([]),
});
export type SingboxSourceParams = z.infer<typeof SingboxSourceParamsSchema>;

// Default mapping mirrors backend DefaultSingboxSourceMapping. The UI
// seeds the editor with this on first interaction; clearing the params
// blob server-side falls back to the same list at runtime.
export const DEFAULT_SINGBOX_SOURCE_MAPPING: SingboxSourceMapping[] = [
  // maps: mihomo token → sing-box headless rule field
  { type: "DOMAIN", action: "map", mapTo: "domain" },
  { type: "DOMAIN-SUFFIX", action: "map", mapTo: "domain_suffix" },
  { type: "DOMAIN-KEYWORD", action: "map", mapTo: "domain_keyword" },
  { type: "DOMAIN-REGEX", action: "map", mapTo: "domain_regex" },
  { type: "IP-CIDR", action: "map", mapTo: "ip_cidr" },
  { type: "IP-CIDR6", action: "map", mapTo: "ip_cidr" },
  { type: "IP-SUFFIX", action: "map", mapTo: "ip_cidr" },
  { type: "SRC-IP-CIDR", action: "map", mapTo: "source_ip_cidr" },
  { type: "SRC-IP-SUFFIX", action: "map", mapTo: "source_ip_cidr" },
  { type: "DST-PORT", action: "map", mapTo: "port" },
  { type: "SRC-PORT", action: "map", mapTo: "source_port" },
  { type: "PROCESS-NAME", action: "map", mapTo: "process_name" },
  { type: "PROCESS-PATH", action: "map", mapTo: "process_path" },
  { type: "PROCESS-PATH-REGEX", action: "map", mapTo: "process_path_regex" },
  { type: "NETWORK", action: "map", mapTo: "network" },
  // drops with explanatory reasons
  { type: "GEOIP", action: "drop", reason: "sing-box rule-set 不内联 GEOIP，请改用独立 rule-set 引用 geoip-cn 之类" },
  { type: "GEOSITE", action: "drop", reason: "sing-box rule-set 不内联 GEOSITE，请改用独立 rule-set" },
  { type: "SRC-GEOIP", action: "drop", reason: "sing-box rule-set 不支持 SRC-GEOIP" },
  { type: "IP-ASN", action: "drop", reason: "sing-box rule-set 不支持 IP-ASN" },
  { type: "SRC-IP-ASN", action: "drop", reason: "sing-box rule-set 不支持 SRC-IP-ASN" },
  { type: "DOMAIN-WILDCARD", action: "drop", reason: "sing-box rule-set 无 domain_wildcard 字段；如必须迁移请改写为 domain_regex" },
  { type: "PROCESS-NAME-REGEX", action: "drop", reason: "sing-box rule-set 无 process_name_regex 字段" },
  { type: "PROCESS-NAME-WILDCARD", action: "drop", reason: "sing-box rule-set 无 process_name_wildcard 字段" },
  { type: "PROCESS-PATH-WILDCARD", action: "drop", reason: "sing-box rule-set 无 process_path_wildcard 字段" },
  { type: "IN-PORT", action: "drop", reason: "sing-box rule-set 不携带入站匹配（应在 route rule 上配置 inbound）" },
  { type: "IN-TYPE", action: "drop", reason: "sing-box rule-set 不携带入站类型" },
  { type: "IN-USER", action: "drop", reason: "sing-box rule-set 不携带入站用户" },
  { type: "IN-NAME", action: "drop", reason: "sing-box rule-set 不携带入站名" },
  { type: "UID", action: "drop", reason: "sing-box rule-set 无 UID 等价字段（如需匹配请改用 user/user_id）" },
  { type: "DSCP", action: "drop", reason: "sing-box rule-set 无 DSCP 字段" },
  { type: "PROTOCOL", action: "drop", reason: "sing-box rule-set 无 PROTOCOL 字段" },
  { type: "USER-AGENT", action: "drop", reason: "sing-box rule-set 无 USER-AGENT 字段" },
  { type: "URL-REGEX", action: "drop", reason: "sing-box rule-set 无 URL-REGEX 字段" },
  { type: "RULE-SET", action: "drop", reason: "sing-box rule-set 不支持嵌套引用其它规则集" },
  { type: "SUB-RULE", action: "drop", reason: "sing-box rule-set 不支持 SUB-RULE" },
  { type: "AND", action: "drop", reason: "sing-box rule-set 单条 rule 内字段已是 AND；不接受嵌套 AND 表达式" },
  { type: "OR", action: "drop", reason: "sing-box rule-set 顶层 rules 已是 OR；不接受嵌套 OR 表达式" },
  { type: "NOT", action: "drop", reason: "sing-box headless rule 用 invert:true 实现取反，且整条规则只能整体取反" },
  { type: "MATCH", action: "drop", reason: "sing-box rule-set 无 MATCH/FINAL 概念，最终归属在 route rule 配置" },
  { type: "FINAL", action: "drop", reason: "sing-box rule-set 无 MATCH/FINAL 概念" },
];

// ----- Preview report types (mirrors backend/transformer/report.go) -----

// 单条 transform step 中被丢弃的源行
export const DroppedLineSchema = z.object({
  lineNo: z.number().int().nonnegative(),
  text: z.string(),
  reason: z.string(),
  // 后端把单条样本字节截断到 MaxSampleBytes（2KB）时置 true；前端用它打截断标。
  truncated: z.boolean().optional().default(false),
});
export type DroppedLine = z.infer<typeof DroppedLineSchema>;

// 单条 transform step 中被改写的源行（如 MATCH → FINAL）
export const ModifiedLineSchema = z.object({
  lineNo: z.number().int().nonnegative(),
  from: z.string(),
  to: z.string(),
  reason: z.string().optional().default(""),
  truncated: z.boolean().optional().default(false),
});
export type ModifiedLine = z.infer<typeof ModifiedLineSchema>;

// 单条 transform step 中被新增的输出行
// （没有任何源行与之对应，例如 JS 脚本合成的规则）。
export const AddedLineSchema = z.object({
  lineNo: z.number().int().nonnegative(),
  text: z.string(),
  reason: z.string(),
  truncated: z.boolean().optional().default(false),
});
export type AddedLine = z.infer<typeof AddedLineSchema>;

// 转换流水线中单个 step 的可视化记录
export const StepReportSchema = z.object({
  stage: z.string(),
  index: z.number().int().nonnegative(),
  sourceIndex: z.number().int().nonnegative().optional().default(0),
  kind: z.string(),
  label: z.string(),
  inputLines: z.number().int().nonnegative(),
  outputLines: z.number().int().nonnegative(),
  dropped: z.array(DroppedLineSchema).optional().default([]),
  modified: z.array(ModifiedLineSchema).optional().default([]),
  added: z.array(AddedLineSchema).optional().default([]),
  droppedTotal: z.number().int().nonnegative().optional().default(0),
  modifiedTotal: z.number().int().nonnegative().optional().default(0),
  addedTotal: z.number().int().nonnegative().optional().default(0),
});
export type StepReport = z.infer<typeof StepReportSchema>;

// 最终内容的统计快照（顶部统计卡）。
// format 标识后端识别出的内容格式，决定 UI 端 PayloadCount 应当如何被标注：
//   - "classical"        经典文本规则列表（没有 payloadCount）
//   - "yaml_payload"     mihomo 的 `payload:` YAML 文档（payloadCount = payload 长度）
//   - "singbox_source"   sing-box rule-set source JSON（payloadCount = 顶层 rules 长度）
// 缺省值（旧后端不返回该字段）按 "classical" 处理。
export const FinalStatsSchema = z.object({
  totalLines: z.number().int().nonnegative(),
  byType: z.record(z.string(), z.number().int().nonnegative()).default({}),
  payloadCount: z.number().int().nonnegative().optional(),
  format: z.enum(["classical", "yaml_payload", "singbox_source"]).optional(),
});
export type FinalStats = z.infer<typeof FinalStatsSchema>;

// 单个 client 的完整转换报告
export const TransformReportSchema = z.object({
  steps: z.array(StepReportSchema).default([]),
  finalStats: FinalStatsSchema,
});
export type TransformReport = z.infer<typeof TransformReportSchema>;

// 阶段常量，与后端保持一致，方便 UI 配色
export const TRANSFORM_STAGE = {
  rule: "rule",
  merge: "merge",
  client: "client",
  override: "override",
} as const;
export type TransformStage = (typeof TRANSFORM_STAGE)[keyof typeof TRANSFORM_STAGE];

// step 类型常量
export const TRANSFORM_STEP_KIND = {
  use: "use",
  useBuiltin: "use_builtin",
  replace: "replace",
  removeLines: "remove_lines",
  merge: "merge",
} as const;
export type TransformStepKind = (typeof TRANSFORM_STEP_KIND)[keyof typeof TRANSFORM_STEP_KIND];
