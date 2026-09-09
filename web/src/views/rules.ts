import type { FilterOp } from '../components/forms/ops';

export const sourceKinds = [
  { value: 'url', label: '远程 URL' },
  { value: 'file', label: '本地文件' },
  { value: 'content', label: '内联文本' },
  { value: 'ref', label: '引用规则' },
  { value: 'geosite', label: 'Geosite' },
  { value: 'geoip', label: 'GeoIP' },
] as const;

export type SourceKind = (typeof sourceKinds)[number]['value'];

export const mergeStrategies = [
  { value: 'union', label: '并集' },
  { value: 'intersect', label: '交集' },
  { value: 'difference', label: '差集' },
] as const;

export const geositeProviders = [
  { value: 'v2fly', label: 'v2fly' },
  { value: 'loyalsoldier', label: 'loyalsoldier' },
] as const;

export const geoipProviders = [
  { value: 'loyalsoldier', label: 'loyalsoldier' },
  { value: 'v2fly', label: 'v2fly' },
] as const;

export interface RuleSource {
  kind: SourceKind | 'group';
  group?: RuleSource[];
  preprocess?: string;
  ops?: FilterOp[];
  label?: string;
  url?: string;
  format?: string;
  file?: string;
  content?: string;
  ref?: string;
  provider?: string;
  list?: string;
  attrs?: string;
}

export interface RuleConfig extends Record<string, unknown> {
  id: string;
  name: string;
  description?: string;
  tags?: string[];
  sources: RuleSource[];
  ops?: FilterOp[];
  merge?: { strategy: string };
  outputs: string[];
  preprocess?: string;
}

export interface RuleIdentity {
  id: string;
  name?: string;
  sources?: unknown;
}

const unsafeID = /[\\/\x00-\x1f]/;

export function emptySource(kind: SourceKind = 'url'): RuleSource {
  return { kind, ops: [] };
}

export function emptyRule(): RuleConfig {
  return { id: '', name: '', sources: [emptySource()], ops: [], outputs: [], tags: [] };
}

export function safeRuleID(id: string): boolean {
  const value = id.trim();
  return !!value && value === id && value !== '.' && value !== '..' && !unsafeID.test(value);
}

export function readRule(raw: Record<string, unknown>): RuleConfig {
  const sources = Array.isArray(raw.sources) ? raw.sources.map((item) => readSource(item as Record<string, unknown>)) : [];
  const merge = raw.merge && typeof raw.merge === 'object' ? (raw.merge as { strategy?: string }).strategy : '';
  return {
    ...raw,
    id: String(raw.id ?? ''),
    name: String(raw.name ?? ''),
    description: typeof raw.description === 'string' ? raw.description : '',
    tags: Array.isArray(raw.tags) ? raw.tags.map(String) : [],
    sources: sources.length ? sources : [emptySource()],
    ops: Array.isArray(raw.ops) ? (raw.ops as FilterOp[]) : [],
    merge: merge ? { strategy: merge } : undefined,
    outputs: Array.isArray(raw.outputs) ? raw.outputs.map(String) : [],
    preprocess: typeof raw.preprocess === 'string' ? raw.preprocess : '',
  };
}

export function readSource(raw: Record<string, unknown>): RuleSource {
  const kind = sourceKindOf(raw);
  const source: RuleSource = { kind, ops: Array.isArray(raw.ops) ? raw.ops as FilterOp[] : [] };
  if (typeof raw.preprocess === 'string') source.preprocess = raw.preprocess;
  if (kind === 'group') source.group = (raw.group as Record<string, unknown>[]).map(readSource);
  if (typeof raw.label === 'string' && raw.label) source.label = raw.label;
  if (typeof raw.format === 'string' && raw.format && raw.format !== 'auto') source.format = raw.format;
  switch (kind) {
    case 'url':
      source.url = String(raw.url ?? '');
      break;
    case 'file':
      source.file = String(raw.file ?? '');
      break;
    case 'content':
      source.content = String(raw.content ?? '');
      break;
    case 'ref':
      source.ref = String(raw.ref ?? '');
      break;
    case 'geosite': {
      const parsed = typeof raw.geosite === 'string' && raw.geosite
        ? parseGeositeRef(raw.geosite)
        : { provider: String(raw.provider ?? ''), list: String(raw.list ?? ''), attrs: Array.isArray(raw.attrs) ? raw.attrs.map(String) : [] };
      source.provider = parsed.provider;
      source.list = parsed.list;
      source.attrs = parsed.attrs.join(', ');
      break;
    }
    case 'geoip': {
      const parsed = typeof raw.geoip === 'string' && raw.geoip
        ? parseGeoRef(raw.geoip)
        : { provider: String(raw.provider ?? ''), list: String(raw.list ?? '') };
      source.provider = parsed.provider;
      source.list = parsed.list;
      break;
    }
  }
  return source;
}

export function sourceKindOf(raw: Record<string, unknown>): RuleSource['kind'] {
  if (Array.isArray(raw.group)) return 'group';
  const type = String(raw.type ?? '');
  if (type === 'url' || type === 'ref' || type === 'geosite' || type === 'geoip') return type;
  if (type === 'local') return raw.file ? 'file' : 'content';
  if (raw.url) return 'url';
  if (raw.ref) return 'ref';
  if (raw.geoip) return 'geoip';
  if (raw.geosite || raw.provider || raw.list) return 'geosite';
  if (raw.file) return 'file';
  if (raw.content) return 'content';
  return 'url';
}

export function parseGeositeRef(value: string): { provider: string; list: string; attrs: string[] } {
  const trimmed = value.trim();
  const slash = trimmed.indexOf('/');
  if (slash <= 0) return { provider: '', list: trimmed, attrs: [] };
  const rest = trimmed.slice(slash + 1);
  const at = rest.indexOf('@');
  if (at < 0) return { provider: trimmed.slice(0, slash), list: rest, attrs: [] };
  return {
    provider: trimmed.slice(0, slash),
    list: rest.slice(0, at),
    attrs: rest.slice(at + 1).split(',').map((item) => item.trim()).filter(Boolean),
  };
}

export function parseGeoRef(value: string): { provider: string; list: string } {
  const [provider = '', list = ''] = value.trim().split('/');
  return { provider, list };
}

export function serializeRule(draft: RuleConfig): Record<string, unknown> {
  const value: Record<string, unknown> = { ...draft };
  value.id = draft.id.trim();
  value.name = draft.name.trim();
  value.sources = draft.sources.map(serializeSource);
  value.outputs = [...draft.outputs];
  const tags = (draft.tags ?? []).map((tag) => tag.trim()).filter(Boolean);
  if (tags.length) value.tags = tags;
  else delete value.tags;
  const description = draft.description?.trim();
  if (description) value.description = description;
  else delete value.description;
  const ops = (draft.ops ?? []).filter((op) => op.type);
  if (ops.length) value.ops = ops;
  else delete value.ops;
  const strategy = draft.merge?.strategy?.trim();
  if (strategy && strategy !== 'union') value.merge = { strategy };
  else delete value.merge;
  const preprocess = draft.preprocess?.trim();
  if (preprocess) value.preprocess = preprocess;
  else delete value.preprocess;
  return value;
}

export function serializeSource(source: RuleSource): Record<string, unknown> {
  const value: Record<string, unknown> = {};
  if (source.label?.trim()) value.label = source.label.trim();
  if (source.kind === 'group') value.group = (source.group ?? []).map(serializeSource);
  switch (source.kind) {
    case 'url':
      value.url = source.url?.trim() ?? '';
      if (source.format) value.format = source.format;
      break;
    case 'file':
      value.file = source.file?.trim() ?? '';
      if (source.format) value.format = source.format;
      break;
    case 'content':
      value.content = source.content ?? '';
      if (source.format) value.format = source.format;
      break;
    case 'ref':
      value.ref = source.ref?.trim() ?? '';
      break;
    case 'geosite': {
      const provider = source.provider?.trim() ?? '';
      const list = source.list?.trim() ?? '';
      const attrs = splitCSV(source.attrs);
      value.geosite = attrs.length ? `${provider}/${list}@${attrs.join(',')}` : `${provider}/${list}`;
      break;
    }
    case 'geoip':
      value.geoip = `${source.provider?.trim() ?? ''}/${source.list?.trim() ?? ''}`;
      break;
  }
  if (source.preprocess !== undefined) value.preprocess = source.preprocess;
  if (source.ops?.length) value.ops = source.ops;
  return value;
}

export function splitCSV(value?: string): string[] {
  return (value ?? '').split(/[,，]/).map((item) => item.trim()).filter(Boolean);
}

export function sourceRefs(raw: { sources?: unknown }): string[] {
  const sources = Array.isArray(raw.sources) ? raw.sources : [];
  const refs: string[] = [];
  for (const item of sources) {
    if (!item || typeof item !== 'object') continue;
    const source = item as Record<string, unknown>;
    if (Array.isArray(source.group)) refs.push(...sourceRefs({ sources: source.group }));
    if (sourceKindOf(source) === 'ref' && typeof source.ref === 'string' && source.ref) refs.push(source.ref);
  }
  return refs;
}

export function ruleTopology(rules: RuleIdentity[], id: string): { upstream: RuleIdentity[]; downstream: RuleIdentity[] } {
  const byID = new Map(rules.map((rule) => [rule.id, rule]));
  const upstream = sourceRefs(byID.get(id) ?? {}).map((ref) => byID.get(ref) ?? { id: ref });
  const downstream = rules.filter((rule) => rule.id !== id && sourceRefs(rule).includes(id));
  return { upstream, downstream };
}

export function dependentIDs(rules: RuleIdentity[], id: string): Set<string> {
  const dependents = new Map<string, string[]>();
  for (const rule of rules) {
    for (const ref of sourceRefs(rule)) {
      const list = dependents.get(ref) ?? [];
      list.push(rule.id);
      dependents.set(ref, list);
    }
  }
  const seen = new Set<string>();
  const queue = [id];
  for (let i = 0; i < queue.length; i++) {
    for (const next of dependents.get(queue[i]) ?? []) {
      if (seen.has(next)) continue;
      seen.add(next);
      queue.push(next);
    }
  }
  seen.delete(id);
  return seen;
}

export function refChoices(rules: RuleIdentity[], selfID: string): { value: string; label: string }[] {
  const blocked = dependentIDs(rules, selfID);
  blocked.add(selfID);
  return rules
    .filter((rule) => rule.id && !blocked.has(rule.id))
    .map((rule) => ({ value: rule.id, label: `${rule.name || rule.id} · ${rule.id}` }));
}

export function ruleReferences(rules: RuleIdentity[], id: string): string[] {
  return rules.filter((rule) => sourceRefs(rule).includes(id)).map((rule) => rule.name || rule.id);
}

export function moveRuleOrder(order: string[], fromID: string, toID: string): string[] {
  const next = [...order];
  const from = next.indexOf(fromID);
  const to = next.indexOf(toID);
  if (from < 0 || to < 0 || from === to) return order;
  next.splice(from, 1);
  next.splice(to, 0, fromID);
  return next;
}

export function validateRule(
  rule: RuleConfig,
  rules: RuleIdentity[],
  clients: { id: string; name?: string }[],
  files: { name: string }[],
  editing: string,
): string {
  if (!safeRuleID(rule.id)) return '规则 ID 不能为空或包含路径';
  if (rules.some((item) => item.id === rule.id && item.id !== editing)) return '规则 ID 已存在';
  if (!rule.name.trim()) return '请输入规则名称';
  if (!rule.sources.length) return '请添加至少一个来源';
  const knownFiles = new Set(files.map((file) => file.name));
  const refs = new Set(refChoices(rules, editing || rule.id).map((item) => item.value));
  const clientIDs = new Set(clients.map((client) => client.id));
  for (const [index, source] of rule.sources.flatMap(source => source.group ? [source, ...source.group] : [source]).entries()) {
    if (source.kind === 'group' && !source.group?.length) return '请为来源组添加至少一个来源';
    for (const op of source.ops ?? []) {
      if (op.type === 'filter_values' && !op.pattern) return '请输入来源过滤匹配值';
      if (op.type !== 'filter_values' && !op.kinds?.length) return '请选择来源过滤规则类型';
    }
    const label = source.label?.trim() || `来源 ${index + 1}`;
    switch (source.kind) {
      case 'url':
        if (!source.url?.trim()) return `请输入${label}的 URL`;
        break;
      case 'file':
        if (!source.file?.trim()) return `请选择${label}的本地文件`;
        if (!knownFiles.has(source.file.trim())) return `${label} 引用了不存在的本地文件`;
        break;
      case 'content':
        if (!source.content?.trim()) return `请输入${label}的内联文本`;
        break;
      case 'ref':
        if (!source.ref?.trim()) return `请选择${label}引用的规则`;
        if (source.ref === rule.id || source.ref === editing) return `${label} 不能引用自身`;
        if (!refs.has(source.ref)) return `${label} 的引用会形成循环或指向未知规则`;
        break;
      case 'geosite':
        if (!source.provider?.trim() || !source.list?.trim()) return `请填写${label} 的 Geosite 提供商和列表`;
        break;
      case 'geoip':
        if (!source.provider?.trim() || !source.list?.trim()) return `请填写${label} 的 GeoIP 提供商和列表`;
        break;
    }
  }
  for (const id of rule.outputs) {
    if (!clientIDs.has(id)) return `输出客户端 ${id} 不存在`;
  }
  for (const op of rule.ops ?? []) {
    if (op.type === 'filter_values' && !op.pattern) return '请输入过滤匹配值';
    if (op.type !== 'filter_values' && !op.kinds?.length) return '请选择过滤规则类型';
  }
  const strategy = rule.merge?.strategy;
  if (strategy && !mergeStrategies.some((item) => item.value === strategy)) return '请选择有效的合并策略';
  return '';
}
