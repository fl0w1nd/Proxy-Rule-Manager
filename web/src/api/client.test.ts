import { describe, expect, it } from 'vitest';
import { APIRequestError, conflictHint } from './client';

function conflict(code: string): APIRequestError {
  return new APIRequestError(409, { error: { code, message: '冲突', details: {} } });
}

describe('conflictHint', () => {
  it('tells the operator to wait when an update holds the configuration', () => {
    expect(conflictHint(conflict('update_in_progress'), '兜底')).toContain('更新任务正在执行');
  });

  it('tells the operator to reload when the file changed on disk', () => {
    expect(conflictHint(conflict('config_dirty'), '兜底')).toContain('重新加载');
  });

  it('tells the operator to refresh on a stale version', () => {
    expect(conflictHint(conflict('config_version_conflict'), '兜底')).toContain('刷新页面');
  });

  it('adds nothing when the server message is already actionable', () => {
    expect(conflictHint(conflict('geo_data_missing'), '兜底')).toBe('');
  });

  it('falls back for unknown conflict codes', () => {
    expect(conflictHint(conflict('template_save_failed'), '兜底')).toBe('兜底');
  });

  it('ignores errors that are not conflicts', () => {
    const notConflict = new APIRequestError(422, { error: { code: 'config_invalid', message: '无效', details: {} } });
    expect(conflictHint(notConflict, '兜底')).toBe('');
    expect(conflictHint(new Error('boom'), '兜底')).toBe('');
  });
});
