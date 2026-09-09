export type GeoKind = 'geosite' | 'geoip';

/**
 * PRM API Client & Type Definitions
 */

export interface SystemStatus {
  last_check: string;
  published_artifacts: number;
  version: string;
  go_version: string;
}

export interface RuleItem {
  id: string;
  name: string;
  entries: number;
  version_at: string;
  last_check?: {
    result: string;
    checked_at: string;
  };
}

export interface GeoProviderItem {
  name: string;
  version?: string;
  lists: number;
  variants: number;
  entries: number;
  files: number;
  result: string;
  checked_at?: string;
  clients: string[];
}

export interface GeoListVariant {
  attr: string;
  entries: number;
}

export interface GeoListOverview {
  name: string;
  entries: number;
  variants?: GeoListVariant[];
}

export interface GeoCatalog {
  provider: string;
  version?: string;
  fetched_at?: string;
  query?: string;
  match?: string;
  lists: GeoListOverview[];
  total: number;
}

export interface GeoEntry {
  type: string;
  value: string;
  attrs?: string[];
}

export interface GeoListDetail {
  provider: string;
  list: string;
  attr?: string;
  query?: string;
  total: number;
  offset: number;
  limit: number;
  items: GeoEntry[];
}

export interface ChangeItem {
  update_id: string;
  finished_at: string;
  origin: string;
  scope: string;
  rule_id: string;
  rule_name: string;
  added: number;
  removed: number;
  added_samples: string[];
  removed_samples: string[];
  added_omitted: number;
  removed_omitted: number;
}

export interface UpdateChange {
  rule_id: string;
  rule_name: string;
  added: number;
  removed: number;
}

export interface UpdateItem {
  id: string;
  origin: 'web' | 'scheduled' | 'cli' | string;
  scope: 'all' | 'rules' | string;
  status: 'running' | 'cancelling' | 'cancelled' | 'completed' | 'completed_with_warnings' | 'completed_with_errors' | 'interrupted' | string;
  started_at: string;
  finished_at?: string;
  rules_total: number;
  rules_succeeded: number;
  rules_failed: number;
  artifacts_processed: number;
  published_artifacts?: number;
  change_count?: number;
  warning_count: number;
  issue_count: number;
  requested_rule_ids?: string[];
  changes?: UpdateChange[];
}

export interface UpdateIssue {
  stage?: string;
  subject?: string;
  message: string;
}

export interface UpdateDetail extends UpdateItem {
  effective_rule_ids?: string[];
  warnings?: string[];
  issues?: UpdateIssue[];
}

export interface UpdateProgressEvent {
  time: string;
  kind?: 'info' | 'success' | 'warning' | 'error';
  message: string;
  current: number;
  total: number;
  rule_id?: string;
}

export interface ConfigDirtyStatus {
  changed: boolean;
}

export type ConfigDocument = Record<string, unknown>;

export interface ConfigSnapshot {
  version: number;
  config: ConfigDocument;
}

export interface ConfigRawSnapshot {
  yaml: string;
  path: string;
  version: number;
}

export interface ConfigMutationResult {
  status?: string;
  version: number;
  warnings: string[];
}

export interface ConfigBackupItem {
  id: string;
  created_at: string;
  size: number;
  added: number;
  removed: number;
}

export interface ConfigBackupLine {
  kind: 'eq' | 'add' | 'del';
  text: string;
}

export interface ConfigBackupDetail extends ConfigBackupItem {
  yaml: string;
  lines: ConfigBackupLine[];
}

export interface ConfigBackupList {
  version: number;
  items: ConfigBackupItem[];
}

export interface IconItem {
  id: string;
  file: string;
}

export interface LocalFileItem {
  name: string;
  size: number;
  lines: number;
  modified_at: string;
}

export interface LocalFileDetail extends LocalFileItem {
  content: string;
}

export interface TemplateItem {
  id: string;
  name: string;
  codec: string;
  extension: string;
  builtin: boolean;
}
export interface TemplateDetail extends TemplateItem { yaml: string; version: string }
export interface IREntry {
  kind: string;
  value: string;
  flags?: string[];
}
export interface TemplatePreview {
  valid: boolean;
  errors: ConfigValidationIssue[];
  output: string;
  extension: string;
  sample: IREntry[];
}

export interface ConfigValidationIssue {
  path: string;
  line?: number;
  message: string;
}

export interface APIErrorDetails {
  errors?: ConfigValidationIssue[];
  current_version?: number;
  config?: ConfigDocument;
  current_update_id?: string;
  reason?: string;
  rules?: { id: string; name: string }[];
  [key: string]: unknown;
}

export interface APIErrorPayload {
  error: {
    code: string;
    message: string;
    details: APIErrorDetails;
  };
}

export class APIRequestError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: APIErrorDetails;
  readonly payload: APIErrorPayload;

  constructor(status: number, payload: APIErrorPayload) {
    super(payload.error.message || `请求失败（${status}）`);
    this.name = 'APIRequestError';
    this.status = status;
    this.code = payload.error.code;
    this.details = payload.error.details;
    this.payload = payload;
  }
}

type ConfigValue = Record<string, unknown>;

export type ConfigPatchOp =
  | { op: 'add_client'; value: ConfigValue }
  | { op: 'update_client'; id: string; value: ConfigValue }
  | { op: 'remove_client'; id: string }
  | { op: 'add_rule'; value: ConfigValue }
  | { op: 'update_rule'; id: string; value: ConfigValue }
  | { op: 'remove_rule'; id: string }
  | { op: 'add_output' | 'remove_output'; rule_id: string; output_id: string }
  | { op: 'batch_add_output' | 'batch_remove_output'; rule_ids: string[]; output_ids: string[] }
  | { op: 'reorder_rules'; order: string[] }
  | { op: 'update_schedule' | 'update_fetch' | 'update_preprocess' | 'update_history'; value: ConfigValue }
  | { op: 'update_geosite' | 'update_geoip'; value: ConfigValue | null };

const API_BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, options);
  if (res.status === 401) {
    location.assign('/admin');
    throw new Error('登录状态已失效');
  }
  if (res.status === 204) {
    return null as unknown as T;
  }
  let payload: any;
  try {
    payload = await res.json();
  } catch {
    payload = {};
  }
  if (!res.ok) {
    const errorPayload: APIErrorPayload = payload?.error
      ? payload
      : {
          error: {
            code: 'request_failed',
            message: `请求失败（${res.status}）`,
            details: {},
          },
        };
    throw new APIRequestError(res.status, errorPayload);
  }
  return payload as T;
}

export const api = {
  listTemplates(): Promise<{ items: TemplateItem[] }> { return request('/templates'); },
  getTemplate(id: string): Promise<TemplateDetail> { return request(`/templates/${encodeURIComponent(id)}`); },
  validateTemplate(yaml: string, sample?: IREntry[]): Promise<TemplatePreview> {
    return request('/templates/validate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ yaml, sample }) });
  },
  saveTemplate(yaml: string, id?: string, version?: string): Promise<TemplateDetail> {
    return request(id ? `/templates/${encodeURIComponent(id)}` : '/templates', {
      method: id ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ yaml, version }),
    });
  },
  getStatus(): Promise<SystemStatus> {
    return request<SystemStatus>('/status');
  },

  getRules(): Promise<{ items: RuleItem[] }> {
    return request<{ items: RuleItem[] }>('/rules');
  },

  getGeoProviders(kind: GeoKind = 'geosite'): Promise<{ items: GeoProviderItem[]; supported: string[] }> {
    return request<{ items: GeoProviderItem[]; supported: string[] }>(`/${kind}/providers`);
  },

  getGeoCatalog(provider: string, query = '', match: 'name' | 'content' = 'name', kind: GeoKind = 'geosite'): Promise<GeoCatalog> {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    if (match) params.set('match', match);
    const qs = params.toString();
    return request<GeoCatalog>(`/${kind}/providers/${encodeURIComponent(provider)}/catalog${qs ? `?${qs}` : ''}`);
  },

  getGeoList(provider: string, list: string, opts?: { attr?: string; q?: string; offset?: number; limit?: number }, kind: GeoKind = 'geosite'): Promise<GeoListDetail> {
    const params = new URLSearchParams();
    if (opts?.attr) params.set('attr', opts.attr);
    if (opts?.q) params.set('q', opts.q);
    if (opts?.offset) params.set('offset', String(opts.offset));
    if (opts?.limit) params.set('limit', String(opts.limit));
    const qs = params.toString();
    return request<GeoListDetail>(`/${kind}/providers/${encodeURIComponent(provider)}/lists/${encodeURIComponent(list)}${qs ? `?${qs}` : ''}`);
  },

  getChanges(limit = 100): Promise<{ items: ChangeItem[] }> {
    return request<{ items: ChangeItem[] }>(`/changes?limit=${limit}`);
  },

  getUpdates(limit = 100): Promise<{ items: UpdateItem[] }> {
    return request<{ items: UpdateItem[] }>(`/updates?limit=${limit}`);
  },

  getCurrentUpdate(): Promise<UpdateItem | null> {
    return request<UpdateItem | null>('/updates/current');
  },

  getUpdateDetail(id: string): Promise<UpdateDetail> {
    return request<UpdateDetail>(`/updates/${encodeURIComponent(id)}`);
  },

  startUpdate(payload: { scope: 'all' | 'rules'; rule_ids?: string[] }): Promise<UpdateItem> {
    return request<UpdateItem>('/updates', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
  },

  cancelUpdate(id: string): Promise<void> {
    return request<void>(`/updates/${encodeURIComponent(id)}/cancel`, {
      method: 'POST',
    });
  },

  checkConfigDirty(): Promise<ConfigDirtyStatus> {
    return request<ConfigDirtyStatus>('/config/dirty');
  },

  getConfig(): Promise<ConfigSnapshot> {
    return request<ConfigSnapshot>('/config');
  },

  getConfigRaw(): Promise<ConfigRawSnapshot> {
    return request('/config/raw');
  },

  validateConfig(yaml: string): Promise<{ valid: boolean; errors: ConfigValidationIssue[] }> {
    return request('/config/validate', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ yaml }),
    });
  },

  saveConfigRaw(yaml: string, version: number): Promise<ConfigMutationResult> {
    return request('/config/raw', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ yaml, version }),
    });
  },

  patchConfig(version: number, ops: ConfigPatchOp[]): Promise<ConfigMutationResult> {
    return request<ConfigMutationResult>('/config/patch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version, ops }),
    });
  },

  reloadConfig(): Promise<ConfigMutationResult> {
    return request<ConfigMutationResult>('/config/reload', {
      method: 'POST',
    });
  },

  listConfigBackups(): Promise<ConfigBackupList> {
    return request<ConfigBackupList>('/config/backups');
  },

  getConfigBackup(id: string): Promise<ConfigBackupDetail> {
    return request<ConfigBackupDetail>(`/config/backups/${encodeURIComponent(id)}`);
  },

  restoreConfigBackup(id: string, version: number): Promise<ConfigMutationResult> {
    return request<ConfigMutationResult>(`/config/backups/${encodeURIComponent(id)}/restore`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version }),
    });
  },

  listIcons(): Promise<{ items: IconItem[] }> {
    return request<{ items: IconItem[] }>('/icons');
  },

  listLocalFiles(): Promise<{ items: LocalFileItem[] }> {
    return request<{ items: LocalFileItem[] }>('/local-files');
  },

  getLocalFile(name: string): Promise<LocalFileDetail> {
    return request<LocalFileDetail>(`/local-files/${encodeURIComponent(name)}`);
  },

  createLocalFile(name: string, content: string): Promise<LocalFileDetail> {
    return request<LocalFileDetail>('/local-files', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, content }),
    });
  },

  saveLocalFile(name: string, content: string): Promise<LocalFileDetail> {
    return request<LocalFileDetail>(`/local-files/${encodeURIComponent(name)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content }),
    });
  },

  deleteLocalFile(name: string): Promise<{ name: string }> {
    return request<{ name: string }>(`/local-files/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    });
  },

  subscribeUpdateEvents(
    id: string,
    onProgress: (event: UpdateProgressEvent) => void,
    onComplete: (detail: UpdateDetail) => void,
    onError: (err: Event) => void
  ): EventSource {
    const es = new EventSource(`${API_BASE}/updates/${encodeURIComponent(id)}/events`);
    es.addEventListener('progress', (ev) => {
      try {
        onProgress(JSON.parse(ev.data));
      } catch (e) {
        console.error('Failed to parse progress event', e);
      }
    });
    es.addEventListener('complete', (ev) => {
      try {
        onComplete(JSON.parse(ev.data));
      } catch (e) {
        console.error('Failed to parse complete event', e);
      }
    });
    es.onerror = onError;
    return es;
  },
};
