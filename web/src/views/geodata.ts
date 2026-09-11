import type { ConfigDocument, GeoKind, HostedGeoKind } from '../api/client';

export type GeoTab = 'geosite' | 'geoip' | 'mmdb' | 'asn';

export const geoTabs: { id: GeoTab; label: string }[] = [
  { id: 'geosite', label: 'Geosite' },
  { id: 'geoip', label: 'GeoIP' },
  { id: 'mmdb', label: 'MMDB' },
  { id: 'asn', label: 'ASN' },
];

export const defaultGeoProviders: Record<GeoKind, string[]> = { geosite: ['v2fly', 'loyalsoldier'], geoip: ['loyalsoldier', 'v2fly'] };
export const defaultHostedGeoProviders: Record<HostedGeoKind, string[]> = { mmdb: ['loyalsoldier'], asn: ['loyalsoldier'] };

export function geoLabel(kind: GeoKind) { return kind === 'geoip' ? 'GeoIP' : 'Geosite'; }
export function hostedGeoLabel(kind: HostedGeoKind) { return kind === 'mmdb' ? 'MMDB' : 'ASN'; }
export function geoTabLabel(tab: GeoTab): string {
  switch (tab) {
    case 'geosite': return 'Geosite';
    case 'geoip': return 'GeoIP';
    case 'mmdb': return 'MMDB';
    case 'asn': return 'ASN';
  }
}

export interface GeoProviderDraft {
  name: string;
  clients: string[];
  host?: boolean;
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
  const providers = (config?.[kind] as { providers?: { name?: string; clients?: string[]; host?: boolean }[] } | undefined)?.providers ?? [];
  return providers.map((provider) => ({
    name: provider.name ?? '',
    clients: [...(provider.clients ?? [])],
    host: provider.host === true,
  }));
}

export interface HostedGeoProviderDraft {
  name: string;
  host?: boolean;
}

export function hostedGeoProviderConfigs(config: ConfigDocument | undefined, kind: HostedGeoKind): HostedGeoProviderDraft[] {
  const providers = (config?.[kind] as { providers?: HostedGeoProviderDraft[] } | undefined)?.providers ?? [];
  return providers
    .map((provider) => ({ ...provider, name: provider.name ?? '', host: provider.host === true }))
    .filter((provider) => provider.name);
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

export function validateHostedGeoProvider(name: string, existing: string[], editing = ''): string {
  if (!name.trim()) return '请选择提供商';
  if (!editing && existing.includes(name)) return '该提供商已添加';
  return '';
}

export function geoPatchValue(providers: GeoProviderDraft[]): { providers: { name: string; clients: string[]; host?: boolean }[] } | null {
  if (!providers.length) return null;
  return {
    providers: providers.map((provider) => {
      const item: { name: string; clients: string[]; host?: boolean } = { name: provider.name, clients: provider.clients };
      if (provider.host) item.host = true;
      return item;
    }),
  };
}

export function hostedGeoPatchValue(providers: HostedGeoProviderDraft[]): { providers: { name: string; host?: boolean }[] } | null {
  if (!providers.length) return null;
  return {
    providers: providers.map((provider) => {
      const item: { name: string; host?: boolean } = { name: provider.name };
      if (provider.host) item.host = true;
      return item;
    }),
  };
}
