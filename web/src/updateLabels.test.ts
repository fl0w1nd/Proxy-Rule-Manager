import { describe, expect, it } from 'vitest';
import type { UpdateDetail } from './api/client';
import { finishSummary, scopeText, updateDigest } from './updateLabels';

describe('Geo update labels', () => {
  it.each(['geosite', 'geoip', 'mmdb', 'asn'] as const)('describes %s updates in history and completion messages', (scope) => {
    const detail: UpdateDetail = {
      id: 'geo-update', origin: 'web', scope, status: 'completed', started_at: '',
      rules_total: 0, rules_succeeded: 0, rules_failed: 0, artifacts_processed: 12,
      warning_count: 0, issue_count: 0, requested_rule_ids: [], effective_rule_ids: [],
      warnings: [], issues: [], changes: [],
    };
    const labels = { geosite: 'Geosite 更新', geoip: 'GeoIP 更新', mmdb: 'MMDB 更新', asn: 'ASN 更新' };
    const label = labels[scope];
    expect(scopeText(detail)).toBe(label);
    expect(finishSummary(detail)).toBe(`${label} · 成功`);
    expect(updateDigest(detail)).toBe(`${label} · 成功 · 文件 12`);
    expect(finishSummary({ ...detail, status: 'completed_with_errors' })).toBe(`${label} · 失败`);
  });

  it('names the failing hosted database in a full-update digest', () => {
    const detail: UpdateDetail = {
      id: 'full-update', origin: 'web', scope: 'all', status: 'completed_with_errors', started_at: '',
      rules_total: 2, rules_succeeded: 2, rules_failed: 0, artifacts_processed: 5,
      warning_count: 0, issue_count: 1, requested_rule_ids: [], effective_rule_ids: ['a', 'b'],
      warnings: [], changes: [],
      issues: [{ stage: 'mmdb_refresh', subject: 'loyalsoldier', message: 'refresh failed' }],
    };
    expect(updateDigest(detail)).toBe('规则 2 · 变更 0 · MMDB 更新失败');
  });
});
