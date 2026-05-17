import { RulesConfig, RuleConfig, ClientType, Transform, ClientFileMeta, GeositeProvider, SystemSettings } from "./schema";

// API client for frontend calls into the backend.

const API_BASE = "/api";

// Read the stored token.
function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("admin_token");
}

// Store the token.
export function setToken(token: string): void {
  localStorage.setItem("admin_token", token);
}

// Remove the token.
export function clearToken(): void {
  localStorage.removeItem("admin_token");
}

/** Build fetch with unified error handling and return the raw Response. */
async function apiFetch(
  endpoint: string,
  init: RequestInit = {},
  options: { auth?: boolean } = {}
): Promise<Response> {
  const { auth = true } = options;
  const token = auth ? getToken() : null;
  const headers: Record<string, string> = {};
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...init,
    headers: { ...headers, ...(init.headers as Record<string, string> | undefined) },
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const message =
      typeof error.error === "string"
        ? error.error
        : error?.error?.message || error.message || "Request failed";
    const code =
      typeof error.error === "object" && error.error ? error.error.code : error.code;
    const err = new Error(message);
    if (code) {
      (err as Error & { code?: string }).code = code;
    }
    (err as Error & { status?: number }).status = response.status;
    // When an authenticated request comes back unauthorized or forbidden,
    // broadcast an "auth-expired" event so the AuthProvider can clear the
    // local token and flip the UI back to the login form. We deliberately
    // exclude the auth-disabled (auth=false) requests so that browsing the
    // public site doesn't accidentally invalidate a logged-in session.
    if (
      auth &&
      (response.status === 401 || response.status === 403) &&
      typeof window !== "undefined"
    ) {
      try {
        window.dispatchEvent(new Event("auth-expired"));
      } catch {
        // Ignore: best-effort notification.
      }
    }
    throw err;
  }
  return response;
}

// Generic JSON request helper.
async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const response = await apiFetch(endpoint, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
  });
  const data = await response.json();
  // Some endpoints return HTTP 200 but with { success: false, message: "..." }
  if (data && typeof data === "object" && data.success === false) {
    throw new Error(data.message || data.error || "Operation failed");
  }
  return data as T;
}

// Config API
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

export async function saveConfig(config: RulesConfig, expectedRev?: number): Promise<{ success: boolean; rev: number; affectedRules: string[] }> {
  return apiRequest("/config", {
    method: "PUT",
    body: JSON.stringify({ config, expectedRev }),
  });
}

export async function backupDatabase(): Promise<Blob> {
  const response = await apiFetch("/database/backup");
  return response.blob();
}

export async function restoreDatabase(file: File): Promise<{ success: boolean }> {
  const formData = new FormData();
  formData.append("file", file);
  const response = await apiFetch("/database/restore", { method: "POST", body: formData });
  return response.json();
}

// Status API
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
  geositeRulesCount: number;
  ruleFilesCount: number;
  geositeRuleFilesCount: number;
  needsInit?: boolean;
  lastSync: {
    lastFullSyncAt: string | null;
    lastPartialSyncAt: string | null;
    lastSuccessfulSyncAt: string | null;
    totalRulesCount: number;
    changedRulesCount: number;
    failedRulesCount: number;
    lastSyncDurationMs?: number | null;
  };
  nextSyncAt?: string;
  scheduleMode?: "interval" | "cron";
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
    displayName?: string;
    description?: string;
    clients: ClientType[];
    lastUpdated: string | null;
    hasError: boolean;
    lastFailureAt?: string | null;
    lastFailureError?: string;
  }[];
  geositeRules: PublicGeositeInfo[];
  clients: Pick<ClientConfig, "id" | "displayName">[];
  version?: string;
}

export async function getStatus(): Promise<StatusResponse> {
  return apiRequest<StatusResponse>("/status");
}

export interface PublicRuleInfo {
  name: string;
  displayName?: string;
  description?: string;
  icon?: string;
  tags?: string[];
  clients: ClientType[];
  lastUpdated?: string | null;
  hasError?: boolean;
  lastFailureAt?: string | null;
  lastFailureError?: string;
}

export interface PublicGeositeInfo extends PublicRuleInfo {
  provider: GeositeProvider;
  list: string;
  attrs: string[];
  outputName: string;
  lastUpdated: string | null;
}

export interface PublicStatusResponse {
  rulesCount: number;
  geositeRulesCount: number;
  lastSyncAt: string | null;
  rules: PublicRuleInfo[];
  geositeRules: PublicGeositeInfo[];
  clients: Pick<ClientConfig, "id" | "displayName">[];
  version?: string;
}

export async function getPublicStatus(): Promise<PublicStatusResponse> {
  // Public endpoints omit the admin token so the server returns public data.
  const response = await apiFetch("/status", {}, { auth: false });
  return response.json();
}

export interface GeositeProviderStatus {
  provider: GeositeProvider;
  ready: boolean;
  fetchedAt: string | null;
  resolvedVersion: string | null;
  catalogCount: number;
}

export interface GeositeCatalogItem {
  name: string;
  imported: boolean;
  ruleName: string | null;
  clients: string[];
  attrs: string[];
  entryCount: number;
}

export async function getGeositeProviders(): Promise<{ providers: GeositeProviderStatus[] }> {
  return apiRequest("/geosite/providers");
}

export async function refreshGeositeProvider(provider: GeositeProvider): Promise<{
  success: boolean;
  provider: GeositeProvider;
  resolvedVersion: string;
  fetchedAt: string;
  catalogCount: number;
}> {
  return apiRequest(`/geosite/providers/${encodeURIComponent(provider)}/refresh`, {
    method: "POST",
  });
}

export interface GeositeStaleImport {
  name: string;
  ruleName: string;
  clients: string[];
}

export async function getGeositeCatalog(provider: GeositeProvider): Promise<{
  provider: GeositeProvider;
  resolvedVersion: string;
  fetchedAt: string;
  catalog: GeositeCatalogItem[];
  staleImports?: GeositeStaleImport[];
}> {
  return apiRequest(`/geosite/catalog?provider=${encodeURIComponent(provider)}`);
}

export async function lookupGeositeDomain(
  provider: GeositeProvider,
  domain: string
): Promise<{ matches: string[] }> {
  const params = new URLSearchParams({
    provider,
    domain,
  });
  return apiRequest(`/geosite/domain-lookup?${params.toString()}`);
}

export async function importAllGeositeRules(
  provider: GeositeProvider,
  clientId: string
): Promise<{
  success: boolean;
  created: number;
  updated: number;
  skipped: number;
  total: number;
  ruleNames: string[];
}> {
  return apiRequest("/geosite/import-all", {
    method: "POST",
    body: JSON.stringify({ provider, clientId }),
  });
}

export async function importSelectedGeositeRules(
  provider: GeositeProvider,
  clientId: string,
  lists: Array<string | { list: string; attrs?: string[] }>
): Promise<{
  success: boolean;
  created: number;
  updated: number;
  skipped: number;
  total: number;
  ruleNames: string[];
}> {
  return apiRequest("/geosite/import-selected", {
    method: "POST",
    body: JSON.stringify({ provider, clientId, lists }),
  });
}

export async function previewGeosite(
  provider: GeositeProvider,
  list: string,
  clientId: string,
  attrs: string[] = [],
  limit?: number
): Promise<{
  content: string;
  totalEntries: number;
  totalLines: number;
  truncated: boolean;
}> {
  const params = new URLSearchParams({
    provider,
    list,
    client: clientId,
  });
  if (attrs.length > 0) {
    params.set("attrs", attrs.join(","));
  }
  if (limit !== undefined && limit > 0) {
    params.set("limit", String(limit));
  }
  return apiRequest(`/geosite/preview?${params.toString()}`);
}

export async function getChangeRecords(
  date?: string,
  page: number = 1,
  pageSize: number = 20,
  client?: string,
  days?: number
): Promise<ActivityList<ChangeRecordSummary>> {
  const params = new URLSearchParams();
  if (date) params.set("date", date);
  if (client) params.set("client", client);
  if (days) params.set("days", String(days));
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
  client?: string,
  days?: number
): Promise<ActivityList<FailureRecord>> {
  const params = new URLSearchParams();
  if (date) params.set("date", date);
  if (client) params.set("client", client);
  if (days) params.set("days", String(days));
  params.set("page", String(page));
  params.set("pageSize", String(pageSize));
  return apiRequest<ActivityList<FailureRecord>>(`/activity/failures?${params.toString()}`);
}

export interface FailingSource {
  ruleName: string;
  count: number;
  lastTimestamp: string;
  lastMessage: string;
  lastStage?: string;
}

export async function getFailingSources(
  days = 7,
  limit = 5,
): Promise<{ sources: FailingSource[] }> {
  return apiRequest(`/activity/failing-sources?days=${days}&limit=${limit}`);
}

export async function getActivityDates(): Promise<{ dates: string[] }> {
  return apiRequest<{ dates: string[] }>("/activity/dates");
}

export async function clearActivityRecords(): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>("/activity/clear", { method: "POST" });
}

// Sync API
export interface SyncResult {
  success: boolean;
  changedRules: string[];
  failedRules: { name: string; error: string }[];
  jobId: string;
}

// SyncStartAck matches the 202 Accepted body from POST /api/sync/full.
// The call returns immediately and progress is polled through getSyncProgress.
export interface SyncStartAck {
  status: "started";
  jobType: "full_sync";
  startedAt: string;
}

export async function executeFullSync(): Promise<SyncStartAck> {
  return apiRequest<SyncStartAck>("/sync/full", { method: "POST" });
}

// SyncLastSnapshot comes from SyncTracker.last and captures the final sync result.
// success/failedCount/changedCount feed the toast, and cancelled distinguishes
// an explicit user cancel.
export interface SyncLastSnapshot {
  jobId: string;
  jobType: string;
  startedAt: string;
  finishedAt: string;
  success: boolean;
  cancelled: boolean;
  changedCount: number;
  failedCount: number;
  durationMs: number;
  error?: string;
}

// SyncProgress matches GET /api/sync/progress.
// When running=false, most fields are empty, while last can still carry the
// most recent completed snapshot.
export interface SyncProgress {
  running: boolean;
  jobId?: string;
  jobType?: string;
  startedAt?: string;
  phase?: string;
  phaseDetail?: string;
  currentRule?: string;
  total: number;
  processed: number;
  failed: number;
  elapsedMs?: number;
  logTail?: string[];
  cancelled?: boolean;
  cancelReason?: string;
  last?: SyncLastSnapshot;
}

export async function getSyncProgress(): Promise<SyncProgress> {
  return apiRequest<SyncProgress>("/sync/progress");
}

export async function cancelSync(): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>("/sync/cancel", { method: "POST" });
}

export async function refreshRules(ruleNames: string[]): Promise<SyncResult> {
  return apiRequest<SyncResult>("/sync/partial/batch", {
    method: "POST",
    body: JSON.stringify({ ruleNames }),
  });
}

// Scheduled sync configuration
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

// Delete rule
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

export interface BatchDeleteResult {
  success: boolean;
  deleted: string[];
  notFound: string[];
  blocked: { name: string; dependents: string[] }[];
}

export async function batchDeleteRules(ruleNames: string[]): Promise<BatchDeleteResult> {
  return apiRequest<BatchDeleteResult>("/rules/batch-delete", {
    method: "POST",
    body: JSON.stringify({ ruleNames }),
  });
}

// Preview API
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

// Validate the token and check whether authentication is required.
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

// Check whether the backend requires authentication.
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

// Check whether initialization has completed.
export interface InitStatusResponse {
  initialized: boolean;
  rulesCount: number;
}

export async function checkInitStatus(): Promise<InitStatusResponse> {
  return apiRequest<InitStatusResponse>("/init");
}

// Run initialization.
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
  transforms?: Transform[]; // Global per-client transformers.
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
): Promise<{ success: boolean }> {
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

// --- CDN Settings ---
export interface CdnSettings {
  enabled: boolean;
  cacheMode: "no-cache" | "no-store" | "custom";
  staleIfErrorSeconds: number;
  customCacheControl?: string;
  cloudflareCdnCacheControl?: string;
  customHeaders: { name: string; value: string }[];
}

export async function getCdnSettings(): Promise<{ settings: CdnSettings }> {
  return apiRequest<{ settings: CdnSettings }>("/cdn-settings");
}

export async function updateCdnSettings(
  settings: Partial<CdnSettings>
): Promise<{ success: boolean; settings: CdnSettings }> {
  return apiRequest("/cdn-settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

// --- System Settings ---
export interface DiskUsageBucket {
  key: "rules" | "geosite" | "sources" | "iconset" | "client" | "db";
  path: string;
  bytes: number;
}

export interface DiskUsageResponse {
  total: number;
  buckets: DiskUsageBucket[];
}

export async function getDiskUsage(): Promise<DiskUsageResponse> {
  return apiRequest<DiskUsageResponse>("/system/disk-usage");
}

export interface SystemSettingsResponse {
  settings: SystemSettings;
  defaults: SystemSettings;
}

export async function getSystemSettings(): Promise<SystemSettingsResponse> {
  return apiRequest<SystemSettingsResponse>("/system-settings");
}

export async function updateSystemSettings(
  settings: SystemSettings,
): Promise<{ success: boolean; settings: SystemSettings }> {
  return apiRequest("/system-settings", {
    method: "PUT",
    body: JSON.stringify({ settings }),
  });
}

// --- IconSet Management ---
export interface IconMeta {
  id: string;
  name: string;
  url: string;
  size: number;
  createdAt: string;
}

export async function listIcons(): Promise<{ icons: IconMeta[] }> {
  return apiRequest<{ icons: IconMeta[] }>("/iconset");
}

export async function uploadIcons(
  files: FileList | File[]
): Promise<{ success: boolean; uploaded: IconMeta[]; renamed: { original: string; renamed: string }[] }> {
  const formData = new FormData();
  for (const file of files) {
    formData.append("files", file);
  }
  const response = await apiFetch("/iconset/upload", { method: "POST", body: formData });
  return response.json();
}

export async function renameIcon(id: string, newName: string): Promise<{ success: boolean; icon: IconMeta }> {
  return apiRequest(`/iconset/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify({ newName }),
  });
}

export async function deleteIcon(id: string): Promise<{ success: boolean }> {
  return apiRequest(`/iconset/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}
