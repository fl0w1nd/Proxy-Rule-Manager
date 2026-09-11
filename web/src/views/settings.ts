import type { ConfigDocument, ConfigPatchOp } from '../api/client';
import { normalizeDuration } from '../utils/duration';
import { joinQuantity, splitQuantity, type DurationUnit, type QuantityKind, type SizeUnit } from '../utils/quantity';
import { normalizeSize } from '../utils/size';

type Values = Record<string, string | number>;
export interface RuntimeSettings {
  schedule: Values;
  fetch: Values;
  preprocess: Values;
  history: Values;
}

export interface SettingField {
  key: string;
  label: string;
  kind?: QuantityKind | 'number';
  units?: readonly DurationUnit[] | readonly SizeUnit[];
  min?: number;
  max?: number;
  hint?: string;
  tooltip?: string;
}

export const settingTabs = [
  { value: 'runtime', label: '运行设置' },
  { value: 'config', label: '配置文件' },
  { value: 'backup', label: '备份与恢复' },
];

export const scheduleIntervalUnits = ['m', 'h'] as const;

export const settingGroups: { key: 'fetch' | 'preprocess' | 'history'; title: string; fields: SettingField[] }[] = [
  { key: 'fetch', title: '抓取设置', fields: [
    { key: 'timeout', label: '抓取超时', kind: 'duration', units: ['ms', 's', 'm'] },
    { key: 'max_download', label: '下载大小上限', kind: 'size', units: ['KB', 'MB', 'GB'], tooltip: '单个来源允许下载的最大体积。' },
    { key: 'concurrency', label: '全局并发', kind: 'number', min: 1, max: 64, tooltip: '同时抓取的来源数量上限。' },
    { key: 'per_host_concurrency', label: '单主机并发', kind: 'number', min: 1, max: 64, tooltip: '同一主机同时下载的上限，不能超过全局并发。' },
    { key: 'retries', label: '重试次数', kind: 'number', min: 0, max: 10 },
    { key: 'retry_delay', label: '重试间隔', kind: 'duration', units: ['ms', 's'], tooltip: '失败后的基础等待时间，后续重试按倍数递增。' },
    { key: 'user_agent', label: 'User-Agent', tooltip: '抓取远程规则时发送的请求标识。' },
  ] },
  { key: 'preprocess', title: '预处理', fields: [
    { key: 'timeout', label: '预处理超时', kind: 'duration', units: ['ms', 's', 'm'], tooltip: '单条规则执行 JS 预处理的时限。' },
    { key: 'max_output', label: '输出大小上限', kind: 'size', units: ['KB', 'MB', 'GB'], tooltip: '预处理脚本写出结果的体积上限。' },
  ] },
  { key: 'history', title: '历史保留', fields: [
    { key: 'history_retention', label: '保留时长', kind: 'duration', units: ['h', 'd'], tooltip: '超过此时长的更新记录会被清除。' },
    { key: 'history_limit', label: '记录上限', kind: 'number', min: 1, max: 10000, tooltip: '最多保留的更新条数，与保留时长同时生效。' },
  ] },
];

const cronToken = /^(?:\*|\d+)(?:-\d+)?(?:\/\d+)?$/;

export function readSettings(config: ConfigDocument): RuntimeSettings {
  const update = (config.update ?? {}) as Record<string, unknown>;
  const fetch = (update.fetch ?? {}) as Values;
  return {
    schedule: { mode: 'manual', timezone: 'UTC', interval: '30m', cron: '0 3 * * *', ...(update.schedule as Values) },
    fetch: { timeout: '15s', max_download: '50MB', concurrency: 4, per_host_concurrency: Math.min(Number(fetch.concurrency ?? 4), 2), retries: 2, retry_delay: '500ms', user_agent: 'Proxy-Rule-Manager/2.0', ...fetch },
    preprocess: { timeout: '5s', max_output: '8MB', ...(update.preprocess as Values) },
    history: { history_retention: (update.history_retention ?? '168h') as string, history_limit: (update.history_limit ?? 200) as number },
  };
}

function changedFields(group: keyof RuntimeSettings, baseline: RuntimeSettings, form: RuntimeSettings): string[] {
  return Object.keys(form[group]).filter((key) => {
    if (group === 'schedule' && key === 'cron' && form.schedule.mode !== 'cron') return false;
    if (group === 'schedule' && key === 'interval' && form.schedule.mode !== 'interval') return false;
    return String(form[group][key]) !== String(baseline[group][key]);
  });
}

export function settingsChanged(baseline: RuntimeSettings, form: RuntimeSettings): boolean {
  return (Object.keys(form) as (keyof RuntimeSettings)[]).some((group) => changedFields(group, baseline, form).length > 0);
}

// Setting operations replace a whole group. Overlay edits on the source so
// omitted fields stay absent instead of being filled with form defaults.
export function buildSettingsPatch(config: ConfigDocument, baseline: RuntimeSettings, form: RuntimeSettings): ConfigPatchOp[] {
  const update = (config.update ?? {}) as Record<string, unknown>;
  const ops: ConfigPatchOp[] = [];
  for (const key of ['schedule', 'fetch', 'preprocess', 'history'] as const) {
    const changed = changedFields(key, baseline, form);
    if (!changed.length) continue;
    const value: Record<string, unknown> = key === 'history' ? { ...baseline.history } : { ...(update[key] as Values) };
    for (const field of changed) {
      let input: string | number = form[key][field];
      const definition = settingGroups.find((group) => group.key === key)?.fields.find((item) => item.key === field);
      if (definition?.kind === 'duration' || (key === 'schedule' && field === 'interval')) input = normalizeDuration(String(input));
      if (definition?.kind === 'size') input = normalizeSize(String(input));
      if (definition?.kind === 'number') input = Number(input);
      if (field === 'user_agent') input = String(input).trim();
      value[field] = input;
    }
    if (key === 'schedule' && changed.includes('mode')) {
      if (form.schedule.mode === 'interval') value.interval = normalizeDuration(String(form.schedule.interval));
      else delete value.interval;
      if (form.schedule.mode === 'cron') value.cron = String(form.schedule.cron).trim();
      else delete value.cron;
    }
    ops.push({ op: `update_${key}`, value });
  }
  return ops;
}

export function validateSettings(form: RuntimeSettings, baseline: RuntimeSettings): Record<string, string> {
  const errors: Record<string, string> = {};
  const dirty = {
    schedule: changedFields('schedule', baseline, form),
    fetch: changedFields('fetch', baseline, form),
    preprocess: changedFields('preprocess', baseline, form),
    history: changedFields('history', baseline, form),
  };
  const submitted = (group: keyof typeof dirty, key: string) => dirty[group].includes(key)
    || (group === 'schedule' && dirty.schedule.includes('mode') && (
      (key === 'interval' && form.schedule.mode === 'interval')
      || (key === 'cron' && form.schedule.mode === 'cron')
    ));

  function quantity(path: string, kind: QuantityKind, input: string | number, units: readonly string[]) {
    const parsed = splitQuantity(kind, String(input), units);
    const n = Number(parsed.amount);
    if (!parsed.amount.trim() || !Number.isFinite(n) || n <= 0) {
      errors[path] = '请输入大于 0 的数字';
      return;
    }
    try {
      const joined = joinQuantity(kind, parsed.amount, parsed.unit);
      if (kind === 'duration') normalizeDuration(joined);
      else normalizeSize(joined);
    } catch (error) { errors[path] = (error as Error).message; }
  }

  if (submitted('schedule', 'timezone') && !isTimezone(String(form.schedule.timezone))) {
    errors['schedule.timezone'] = '请输入有效时区，例如 UTC 或 Asia/Shanghai';
  }
  if (submitted('schedule', 'interval')) quantity('schedule.interval', 'duration', form.schedule.interval, scheduleIntervalUnits);
  if (submitted('schedule', 'cron') && !isCron(String(form.schedule.cron))) {
    errors['schedule.cron'] = '请输入五段 Cron，例如 0 3 * * *';
  }
  for (const group of settingGroups) {
    for (const field of group.fields) {
      if (!submitted(group.key, field.key)) continue;
      const path = `${group.key}.${field.key}`;
      const value = String(form[group.key][field.key]).trim();
      if (field.kind === 'duration' || field.kind === 'size') quantity(path, field.kind, value, field.units ?? []);
      if (field.kind === 'number' && (!value || !Number.isInteger(Number(value)) || Number(value) < field.min! || Number(value) > field.max!)) {
        errors[path] = `请输入 ${field.min} 至 ${field.max} 的整数`;
      }
      if (field.key === 'user_agent' && !value) errors[path] = '请输入 User-Agent';
    }
  }
  if ((dirty.fetch.includes('concurrency') || dirty.fetch.includes('per_host_concurrency'))
    && Number(form.fetch.per_host_concurrency) > Number(form.fetch.concurrency)) {
    errors['fetch.per_host_concurrency'] = '单主机并发不能超过全局并发';
  }
  return errors;
}

function isTimezone(value: string): boolean {
  const timezone = value.trim();
  if (!timezone) return false;
  if (timezone === 'Local') return true;
  try {
    Intl.DateTimeFormat(undefined, { timeZone: timezone });
    return true;
  } catch {
    return false;
  }
}

function isCron(value: string): boolean {
  const fields = value.trim().split(/\s+/);
  return fields.length === 5 && fields.every((field) => field.split(',').every((part) => cronToken.test(part)));
}
