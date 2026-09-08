export const localFileExtensions = ['.list', '.yaml', '.txt'] as const;

const unsafeName = /[/\\]/;

export function validateLocalFileName(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) return '请输入文件名';
  if (unsafeName.test(trimmed) || trimmed === '.' || trimmed === '..' || trimmed.startsWith('.')) {
    return '文件名无效';
  }
  const dot = trimmed.lastIndexOf('.');
  if (dot <= 0) return '文件名需以 .list、.yaml 或 .txt 结尾';
  const ext = trimmed.slice(dot).toLowerCase();
  if (!(localFileExtensions as readonly string[]).includes(ext)) {
    return '文件名需以 .list、.yaml 或 .txt 结尾';
  }
  if (trimmed.length > 255) return '文件名过长';
  return null;
}

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}

export function localFileLanguage(name: string): 'yaml' | 'text' {
  return name.trim().toLowerCase().endsWith('.yaml') ? 'yaml' : 'text';
}

export function referencedRuleLabel(rules: { id: string; name: string }[] | undefined): string {
  if (!rules?.length) return '';
  return rules.map((rule) => rule.name || rule.id).join('、');
}
