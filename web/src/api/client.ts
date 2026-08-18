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

export interface GeositeProviderItem {
  name: string;
  version?: string;
  lists: number;
  variants: number;
  entries: number;
  files: number;
  result: string;
  checked_at?: string;
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

export interface ConfigMutationResult {
  status?: string;
  version: number;
  warnings: string[];
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
  | { op: 'update_geosite'; value: ConfigValue | null };

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
  getStatus(): Promise<SystemStatus> {
    return request<SystemStatus>('/status');
  },

  getRules(): Promise<{ items: RuleItem[] }> {
    return request<{ items: RuleItem[] }>('/rules');
  },

  getGeositeProviders(): Promise<{ items: GeositeProviderItem[] }> {
    return request<{ items: GeositeProviderItem[] }>('/geosite/providers');
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
