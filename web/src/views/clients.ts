import type { ConfigDocument, TemplateItem } from '../api/client';

import type { FilterOp } from '../components/forms/ops';
export interface ClientFormat { id: string; name?: string; template: string }
export interface ClientVariant { id: string; name?: string; template?: string; ops: FilterOp[] }
export interface ClientConfig extends Record<string, unknown> {
  id: string; name: string; icon?: string; template?: string;
  formats?: ClientFormat[]; variants?: ClientVariant[];
}

export function clientIconID(icon?: string): string {
  if (!icon) return '';
  const prefix = '/static/icons/';
  const path = icon.startsWith(prefix) ? icon.slice(prefix.length) : icon;
  if (path.includes('/')) return '';
  return path.replace(/\.svg$/i, '');
}

export function clientIconSrc(icon?: string): string {
  if (!icon) return '';
  if (icon.startsWith('/static/icons/')) return icon;
  const id = clientIconID(icon);
  return id ? `/static/icons/${encodeURIComponent(id)}.svg` : '';
}

export function defaultClientIcon(id: string): string {
  const s = id.toLowerCase();
  if (s.includes('shadowrocket')) return 'shadowrocket';
  if (s.includes('surge')) return 'surge';
  if (s.includes('sing-box') || s.includes('singbox')) return 'singbox';
  if (s.includes('clash') || s.includes('mihomo') || s.includes('meta')) return 'mihomo';
  return 'client';
}

export function clientReferences(config: ConfigDocument, id: string): string[] {
  const rules = (config.rules ?? []) as { id: string; name?: string; outputs?: string[] }[];
  const providers = (config.geosite as { providers?: { name: string; clients?: string[] }[] } | undefined)?.providers ?? [];
  const ipProviders = (config.geoip as { providers?: { name: string; clients?: string[] }[] } | undefined)?.providers ?? [];
  return [
    ...rules.filter(r => r.outputs?.includes(id)).map(r => `规则 ${r.name || r.id}`),
    ...ipProviders.filter(p => p.clients?.includes(id)).map(p => `GeoIP ${p.name}`),
    ...providers.filter(p => p.clients?.includes(id)).map(p => `Geosite ${p.name}`),
  ];
}
export function validateClient(client: ClientConfig, templates: TemplateItem[], clients: ClientConfig[], editing: string): string {
  const safe = (id: string) => !!id.trim() && id === id.trim() && id !== '.' && id !== '..' && !/[\\/\x00-\x1f]/.test(id);
  if (!safe(client.id)) return '客户端 ID 不能为空或包含路径';
  if (clients.some(c => c.id === client.id && c.id !== editing)) return '客户端 ID 已存在';
  const formats = client.formats ?? [];
  const variants = client.variants ?? [];
  const known = (id?: string) => templates.some(t => t.id === id);
  if (!formats.length && !known(client.template)) return '请选择客户端模板';
  const used = new Set<string>();
  for (const other of clients.filter(c => c.id !== editing)) {
    used.add(other.id);
    for (const f of [...(other.formats ?? []), ...(other.variants ?? [])]) used.add(f.id);
  }
  if (!formats.length) used.add(client.id);
  for (const f of [...formats, ...variants]) {
    if (!safe(f.id)) return '格式与变体 ID 不能为空或包含路径';
    if (used.has(f.id)) return `输出 ID ${f.id} 已存在`;
    used.add(f.id);
  }
  if (formats.some(f => !known(f.template))) return '请选择格式模板';
  for (const v of variants) {
    if (v.template ? !known(v.template) : formats.length > 1) return '请选择变体模板';
    for (const op of v.ops ?? []) {
      if (op.type === 'filter_values' && !op.pattern) return '请输入过滤匹配值';
      if (op.type !== 'filter_values' && !op.kinds?.length) return '请选择过滤规则类型';
    }
  }
  return '';
}
