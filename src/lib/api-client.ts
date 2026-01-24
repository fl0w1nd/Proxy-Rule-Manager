import { RulesConfig, RuleConfig, ClientType, Transform, ClientFileMeta } from "./schema";

// API 客户端 - 用于前端调用后端 API

const API_BASE = "/api";

// 获取存储的 token
function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("admin_token");
}

// 设置 token
export function setToken(token: string): void {
  localStorage.setItem("admin_token", token);
}

// 清除 token
export function clearToken(): void {
  localStorage.removeItem("admin_token");
}

// 通用请求函数
async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };

  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const message =
      typeof error.error === "string"
        ? error.error
        : error?.error?.message || error.message || "Request failed";
    const code =
      typeof error.error === "object" && error.error
        ? error.error.code
        : error.code;
    const err = new Error(message);
    if (code) {
      (err as Error & { code?: string }).code = code;
    }
    throw err;
  }

  return response.json();
}

// 配置 API
export interface ConfigResponse {
  config: RulesConfig;
  rev: number;
}

export async function getConfig(): Promise<ConfigResponse> {
  return apiRequest<ConfigResponse>("/config");
}

export async function getConfigRaw(): Promise<ConfigResponse> {
  return apiRequest<ConfigResponse>("/config?raw=1");
}

export async function saveConfig(config: RulesConfig): Promise<{ success: boolean; rev: number; affectedRules: string[] }> {
  return apiRequest("/config", {
    method: "PUT",
    body: JSON.stringify({ config }),
  });
}

export async function backupDatabase(): Promise<Blob> {
  const token = getToken();
  const headers: HeadersInit = {};
  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }
  const response = await fetch(`${API_BASE}/database/backup`, { headers });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const message =
      typeof error.error === "string"
        ? error.error
        : error?.error?.message || error.message || "Request failed";
    throw new Error(message);
  }
  return response.blob();
}

export async function restoreDatabase(file: File): Promise<{ success: boolean }> {
  const token = getToken();
  const headers: HeadersInit = {};
  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }
  const formData = new FormData();
  formData.append("file", file);
  const response = await fetch(`${API_BASE}/database/restore`, {
    method: "POST",
    headers,
    body: formData,
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const message =
      typeof error.error === "string"
        ? error.error
        : error?.error?.message || error.message || "Request failed";
    throw new Error(message);
  }
  return response.json();
}

export async function exportConfigTemplate(): Promise<Blob> {
  const token = getToken();
  const headers: HeadersInit = {};
  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }
  const response = await fetch(`${API_BASE}/config/template/export`, { headers });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const message =
      typeof error.error === "string"
        ? error.error
        : error?.error?.message || error.message || "Request failed";
    throw new Error(message);
  }
  return response.blob();
}

export async function importConfigTemplate(file: File): Promise<{ success: boolean; rev: number }> {
  const token = getToken();
  const headers: HeadersInit = {};
  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }
  const formData = new FormData();
  formData.append("file", file);
  const response = await fetch(`${API_BASE}/config/template/import`, {
    method: "POST",
    headers,
    body: formData,
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const message =
      typeof error.error === "string"
        ? error.error
        : error?.error?.message || error.message || "Request failed";
    throw new Error(message);
  }
  return response.json();
}

// 状态 API
export interface ChangeRecordSummary {
  id: string;
  timestamp: string;
  ruleName: string;
  client: ClientType;
  changeType: "created" | "updated" | "deleted";
  sizeBytes?: number;
  date: string;
  fileName: string;
}

export interface FailureRecord {
  id: string;
  timestamp: string;
  ruleName: string;
  client?: ClientType;
  source?: string;
  message: string;
  stage: string;
  jobId?: string;
}

export interface ActivityList<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface StatusResponse {
  rulesCount: number;
  ruleFilesCount: number;
  needsInit?: boolean;
  lastSync: {
    lastFullSyncAt: string | null;
    lastPartialSyncAt: string | null;
    lastSuccessfulSyncAt: string | null;
    totalRulesCount: number;
    changedRulesCount: number;
    failedRulesCount: number;
  };
  todayStats: {
    date: string;
    syncCount: number;
    blobWriteCount: number;
    rulesChanged: number;
    totalRulesProcessed: number;
    failedSources: number;
    ruleFilesChanged: number;
    failureRecords: number;
  };
  rules: {
    name: string;
    description?: string;
    clients: ClientType[];
    lastUpdated: string | null;
    hasError: boolean;
  }[];
}

export async function getStatus(): Promise<StatusResponse> {
  return apiRequest<StatusResponse>("/status");
}

export async function getChangeRecords(
  date?: string,
  page: number = 1,
  pageSize: number = 20,
  client?: string
): Promise<ActivityList<ChangeRecordSummary>> {
  const params = new URLSearchParams();
  if (date) params.set("date", date);
  if (client) params.set("client", client);
  params.set("page", String(page));
  params.set("pageSize", String(pageSize));
  return apiRequest<ActivityList<ChangeRecordSummary>>(`/activity/changes?${params.toString()}`);
}

export async function getChangeDiff(
  date: string,
  fileName: string
): Promise<{ diff: string }> {
  return apiRequest<{ diff: string }>(`/activity/changes/${encodeURIComponent(date)}/${encodeURIComponent(fileName)}`);
}

export async function getFailureRecords(
  date?: string,
  page: number = 1,
  pageSize: number = 20,
  client?: string
): Promise<ActivityList<FailureRecord>> {
  const params = new URLSearchParams();
  if (date) params.set("date", date);
  if (client) params.set("client", client);
  params.set("page", String(page));
  params.set("pageSize", String(pageSize));
  return apiRequest<ActivityList<FailureRecord>>(`/activity/failures?${params.toString()}`);
}

export async function getActivityDates(): Promise<{ dates: string[] }> {
  return apiRequest<{ dates: string[] }>("/activity/dates");
}

export async function clearActivityRecords(): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>("/activity/clear", { method: "POST" });
}

// 同步 API
export interface SyncResult {
  success: boolean;
  changedRules: string[];
  failedRules: { name: string; error: string }[];
  jobId: string;
}

export async function executeFullSync(): Promise<SyncResult> {
  return apiRequest<SyncResult>("/sync/full", { method: "POST" });
}

// 定时同步配置
export interface SyncSchedule {
  mode: "interval" | "cron";
  intervalHours: number;
  cronExpression?: string;
  lastScheduledSyncAt?: string;
  nextSyncAt?: string;
}

export async function getSyncSchedule(): Promise<{ schedule: SyncSchedule }> {
  return apiRequest<{ schedule: SyncSchedule }>("/sync/schedule");
}

export async function updateSyncSchedule(payload: {
  mode: "interval" | "cron";
  intervalHours?: number;
  cronExpression?: string;
}): Promise<{ success: boolean; schedule: SyncSchedule }> {
  return apiRequest<{ success: boolean; schedule: SyncSchedule }>("/sync/schedule", {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function refreshRule(ruleName: string): Promise<SyncResult> {
  return apiRequest<SyncResult>(`/rules/${encodeURIComponent(ruleName)}/refresh`, {
    method: "POST",
  });
}

// 删除规则
export interface DeleteRuleResult {
  success: boolean;
  deletedRule: string;
  deletedClients: ClientType[];
}

export async function deleteRule(ruleName: string): Promise<DeleteRuleResult> {
  return apiRequest<DeleteRuleResult>(`/rules/${encodeURIComponent(ruleName)}`, {
    method: "DELETE",
  });
}

// 预览 API
export interface PreviewResponse {
  contents: Record<ClientType, string>;
  diagnostics: {
    sourceResults: { url: string; success: boolean; error?: string; size?: number }[];
    truncated: boolean;
    totalLines: number;
  };
}

export async function previewRule(
  ruleName?: string,
  rule?: RuleConfig,
  limitLines?: number
): Promise<PreviewResponse> {
  return apiRequest<PreviewResponse>("/preview", {
    method: "POST",
    body: JSON.stringify({ ruleName, rule, limitLines }),
  });
}

// 验证 token（也检查是否需要认证）
export async function verifyToken(token: string): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE}/status`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });
    return response.ok;
  } catch {
    return false;
  }
}

// 检查后端是否需要认证
// 如果不需要认证（未设置 ADMIN_TOKEN），直接返回 true
export async function checkAuthRequired(): Promise<{ required: boolean; authenticated: boolean }> {
  try {
    const response = await fetch(`${API_BASE}/auth/required`);
    if (!response.ok) {
      return { required: true, authenticated: false };
    }
    const data = await response.json().catch(() => ({ required: true }));
    const required = !!data.required;
    return { required, authenticated: !required };
  } catch {
    return { required: true, authenticated: false };
  }
}

// 检查是否已初始化
export interface InitStatusResponse {
  initialized: boolean;
  rulesCount: number;
}

export async function checkInitStatus(): Promise<InitStatusResponse> {
  return apiRequest<InitStatusResponse>("/init");
}

// 执行初始化
export interface InitResult {
  success: boolean;
  message: string;
  rulesCount?: number;
}

export async function executeInit(): Promise<InitResult> {
  return apiRequest<InitResult>("/init", { method: "POST" });
}

// --- Rule Rename ---
export interface RenameResult {
  success: boolean;
  oldName: string;
  newName: string;
  renamedFiles: string[];
}

export async function renameRule(oldName: string, newName: string): Promise<RenameResult> {
  return apiRequest<RenameResult>(`/rules/${encodeURIComponent(oldName)}`, {
    method: "PUT",
    body: JSON.stringify({ newName }),
  });
}

// --- Client Management ---
export interface ClientConfig {
  id: string;
  displayName: string;
  pathName: string;
  transforms?: Transform[]; // 客户端全局转换器
}

export async function getClients(): Promise<{ clients: ClientConfig[] }> {
  return apiRequest<{ clients: ClientConfig[] }>("/clients");
}

export async function addClient(client: ClientConfig): Promise<{ success: boolean; client: ClientConfig }> {
  return apiRequest("/clients", {
    method: "POST",
    body: JSON.stringify(client),
  });
}

export async function updateClient(
  clientId: string,
  updates: Partial<ClientConfig>
): Promise<{ success: boolean; renamedPath?: { from: string; to: string } }> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}`, {
    method: "PUT",
    body: JSON.stringify(updates),
  });
}

export async function deleteClient(clientId: string): Promise<{ success: boolean; deletedClient: string }> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}`, {
    method: "DELETE",
  });
}

// --- Client File Management ---
export interface ClientFileDetail {
  file: ClientFileMeta;
  content: string;
}

export async function listClientFiles(clientId: string): Promise<{ files: ClientFileMeta[] }> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}/files`);
}

export async function createClientFile(
  clientId: string,
  input: { configId: string; displayName: string; description?: string; ext: string; isPublic: boolean; content: string }
): Promise<{ success: boolean; file: ClientFileMeta }> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}/files`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getClientFile(
  clientId: string,
  fileId: string
): Promise<ClientFileDetail> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}/files/${encodeURIComponent(fileId)}`);
}

export async function updateClientFile(
  clientId: string,
  fileId: string,
  updates: Partial<{ configId: string; displayName: string; description?: string; ext: string; isPublic: boolean; content: string }>
): Promise<{ success: boolean; file: ClientFileMeta }> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}/files/${encodeURIComponent(fileId)}`, {
    method: "PUT",
    body: JSON.stringify(updates),
  });
}

export async function deleteClientFile(
  clientId: string,
  fileId: string
): Promise<{ success: boolean; deletedFile: string }> {
  return apiRequest(`/clients/${encodeURIComponent(clientId)}/files/${encodeURIComponent(fileId)}`, {
    method: "DELETE",
  });
}

export async function getPublicClientFiles(): Promise<{ files: ClientFileMeta[] }> {
  return apiRequest("/client-files/public");
}

// --- WAF Management ---
export interface BanRecord {
  ip: string;
  reason: string;
  bannedAt: string;
  expiresAt: string | null;
  failCount: number;
}

export interface WafStats {
  bans: {
    total: number;
    permanent: number;
    temporary: number;
  };
  temporary: {
    totalTracked: number;
    currentlyBlocked: number;
  };
}

export interface FailureInfo {
  ip: string;
  failCount: number;
  lastFailedAt: string;
  blockDuration: number;
  isBlocked: boolean;
  blockedUntil: string | null;
}

export async function getWafBans(): Promise<{ bans: BanRecord[] }> {
  return apiRequest<{ bans: BanRecord[] }>("/waf/bans");
}

export async function addWafBan(
  ip: string,
  reason?: string,
  permanent?: boolean,
  durationSeconds?: number
): Promise<{ success: boolean; message: string }> {
  return apiRequest("/waf/bans", {
    method: "POST",
    body: JSON.stringify({ ip, reason, permanent, durationSeconds }),
  });
}

export async function removeWafBan(ip: string): Promise<{ success: boolean; message: string }> {
  return apiRequest(`/waf/bans/${encodeURIComponent(ip)}`, {
    method: "DELETE",
  });
}

export async function getWafStats(): Promise<WafStats> {
  return apiRequest<WafStats>("/waf/stats");
}

export async function getWafFailures(): Promise<{ failures: FailureInfo[] }> {
  return apiRequest<{ failures: FailureInfo[] }>("/waf/failures");
}

export async function cleanupWafBans(): Promise<{ success: boolean; message: string }> {
  return apiRequest("/waf/cleanup", { method: "POST" });
}

export async function getMyIp(): Promise<{ ip: string }> {
  return apiRequest<{ ip: string }>("/waf/my-ip");
}
