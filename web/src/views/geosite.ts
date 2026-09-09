import type { ConfigDocument } from '../api/client';

export const defaultGeositeProviders = ['v2fly', 'loyalsoldier'] as const;

export interface GeositeProviderDraft {
  name: string;
  clients: string[];
}

export function providerLabel(name: string): string {
  if (name === 'loyalsoldier') return 'Loyalsoldier';
  return name;
}

export function entryTypeLabel(type: string): string {
  switch (type) {
    case 'full': return '完整';
    case 'domain': return '后缀';
    case 'keyword': return '关键词';
    case 'regexp': return '正则';
    default: return type;
  }
}

export function geositeProviderConfigs(config?: ConfigDocument): GeositeProviderDraft[] {
  const providers = (config?.geosite as { providers?: { name?: string; clients?: string[] }[] } | undefined)?.providers ?? [];
  return providers.map((provider) => ({
    name: provider.name ?? '',
    clients: [...(provider.clients ?? [])],
  }));
}

export function validateGeositeProvider(
  draft: GeositeProviderDraft,
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

export function geositePatchValue(providers: GeositeProviderDraft[]): { providers: GeositeProviderDraft[] } | null {
  return providers.length ? { providers } : null;
}
