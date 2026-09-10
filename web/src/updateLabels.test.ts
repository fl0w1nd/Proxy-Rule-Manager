import { describe, expect, it } from 'vitest';
import type { UpdateDetail } from './api/client';
import { finishSummary, scopeText, updateDigest } from './updateLabels';

describe('Geo update labels', () => {
  it.each(['geosite', 'geoip'] as const)('describes %s updates in history and completion messages', (scope) => {
    const detail: UpdateDetail = {
      id: 'geo-update', origin: 'web', scope, status: 'completed', started_at: '',
      rules_total: 0, rules_succeeded: 0, rules_failed: 0, artifacts_processed: 12,
      warning_count: 0, issue_count: 0, requested_rule_ids: [], effective_rule_ids: [],
      warnings: [], issues: [], changes: [],
    };
    const label = scope === 'geoip' ? 'GeoIP 更新' : 'Geosite 更新';
    expect(scopeText(detail)).toBe(label);
    expect(finishSummary(detail)).toBe(`${label} · 成功`);
    expect(updateDigest(detail)).toBe(`${label} · 成功 · 文件 12`);
    expect(finishSummary({ ...detail, status: 'completed_with_errors' })).toBe(`${label} · 失败`);
  });
});
