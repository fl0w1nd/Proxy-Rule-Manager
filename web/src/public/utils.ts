import type { PublicClient, PublicClientOption, PublicPageData, PublicView } from './types';

export function readPublicData(): PublicPageData {
  const node = document.getElementById('prm-data');
  if (!node?.textContent) {
    throw new Error('public page data is missing');
  }
  return JSON.parse(node.textContent) as PublicPageData;
}

export function optionUsable(option: PublicClientOption, view: PublicView): boolean {
  return view === 'geosite' ? option.geosite : option.rules;
}

export function clientUsable(client: PublicClient, view: PublicView): boolean {
  return view === 'geosite' ? client.geosite : client.rules;
}

export function selectedOption(
  client: PublicClient,
  selectedID: string | undefined,
  view: PublicView
): PublicClientOption {
  return client.options.find((option) => option.id === selectedID && optionUsable(option, view))
    ?? client.options.find((option) => optionUsable(option, view))
    ?? client.options[0];
}

export function encodedPath(...segments: string[]): string {
  return segments.map((segment) => encodeURIComponent(segment)).join('/');
}

export function geositePath(
  target: PublicClientOption,
  provider: string,
  list: string,
  attr?: string
): string {
  const name = attr ? `${list}@${attr}` : list;
  return `rules/${encodedPath(target.id, 'geosite', provider, name)}${target.ext}`;
}

export function iconPath(setName: string, fileName: string): string {
  return `static/icons/${encodedPath(setName, fileName)}`;
}

export function formatCount(value: number): string {
  return value.toLocaleString('zh-CN');
}

export function formatBytes(value?: number): string {
  if (value === undefined) return '';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(2)} MB`;
}

export function formatUpdatedAt(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toISOString().replace('T', ' ').replace(/\.\d{3}Z$/, ' UTC');
}

export async function copyURL(path: string): Promise<boolean> {
  try {
    const url = new URL(path, location.href).href;
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(url);
      return true;
    }
    const textarea = document.createElement('textarea');
    textarea.value = url;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    textarea.remove();
    return true;
  } catch {
    return false;
  }
}
