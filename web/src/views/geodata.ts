import type { ConfigDocument, GeoKind } from '../api/client';

export const defaultGeoProviders: Record<GeoKind, string[]> = { geosite: ['v2fly', 'loyalsoldier'], geoip: ['loyalsoldier', 'v2fly'] };
export function geoLabel(kind: GeoKind) { return kind === 'geoip' ? 'GeoIP' : 'Geosite'; }

export interface GeoProviderDraft {
  name: string;
  clients: string[];
}

export function providerLabel(name: string): string {
  if (name === 'loyalsoldier') return 'Loyalsoldier';
  return name;
}

export function entryTypeLabel(type: string): string {
  switch (type) {
    case 'ipv4': return 'IPv4';
    case 'ipv6': return 'IPv6';
    case 'full': return '完整';
    case 'domain': return '后缀';
    case 'keyword': return '关键词';
    case 'regexp': return '正则';
    default: return type;
  }
}

export function geoProviderConfigs(config?: ConfigDocument, kind: GeoKind = 'geosite'): GeoProviderDraft[] {
  const providers = (config?.[kind] as { providers?: { name?: string; clients?: string[] }[] } | undefined)?.providers ?? [];
  return providers.map((provider) => ({
    name: provider.name ?? '',
    clients: [...(provider.clients ?? [])],
  }));
}

export function validateGeoProvider(
  draft: GeoProviderDraft,
  existing: string[],
  editing: string,
  clientIDs: string[],
): string {
  if (!draft.name.trim()) return '请选择提供商';
  if (!editing && existing.includes(draft.name)) return '该提供商已添加';
  if (!draft.clients.length) return '请选择至少一个输出客户端';
  if (draft.clients.some((id) => !clientIDs.includes(id))) return '输出客户端无效';
  return '';
}

export function geoPatchValue(providers: GeoProviderDraft[]): { providers: GeoProviderDraft[] } | null {
  return providers.length ? { providers } : null;
}
