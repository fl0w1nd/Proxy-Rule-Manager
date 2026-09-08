import { describe, expect, it } from 'vitest';
import { formatFileSize, localFileLanguage, referencedRuleLabel, validateLocalFileName } from './localFiles';

describe('local file names', () => {
  it('accepts rule list filenames', () => {
    expect(validateLocalFileName('my-direct.list')).toBeNull();
    expect(validateLocalFileName('notes.yaml')).toBeNull();
    expect(validateLocalFileName('readme.txt')).toBeNull();
  });

  it('rejects path syntax and unknown suffixes', () => {
    expect(validateLocalFileName('../secret.list')).toBe('文件名无效');
    expect(validateLocalFileName('nested/file.list')).toBe('文件名无效');
    expect(validateLocalFileName('.hidden.list')).toBe('文件名无效');
    expect(validateLocalFileName('notes.md')).toBe('文件名需以 .list、.yaml 或 .txt 结尾');
    expect(validateLocalFileName('')).toBe('请输入文件名');
  });
});

describe('local file labels', () => {
  it('formats sizes and referenced rules', () => {
    expect(formatFileSize(512)).toBe('512 B');
    expect(formatFileSize(2048)).toBe('2.0 KB');
    expect(referencedRuleLabel([{ id: 'direct', name: 'Direct' }, { id: 'cn', name: 'CN' }])).toBe('Direct、CN');
    expect(localFileLanguage('rules.yaml')).toBe('yaml');
    expect(localFileLanguage('my-direct.list')).toBe('text');
  });
});
