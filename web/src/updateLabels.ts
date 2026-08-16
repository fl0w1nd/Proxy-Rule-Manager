import type { UpdateDetail, UpdateItem } from './api/client';

export function originText(v?: string): string {
  const map: Record<string, string> = { web: '手动', scheduled: '定时', cli: '命令行' };
  return (v && map[v]) || v || '—';
}

export function getStatusType(st: string): 'success' | 'warning' | 'error' | 'active' | 'neutral' {
  if (st === 'completed') return 'success';
  if (st === 'running' || st === 'cancelling') return 'active';
  if (st === 'completed_with_warnings' || st === 'cancelled') return 'warning';
  if (st === 'completed_with_errors' || st === 'interrupted') return 'error';
  return 'neutral';
}

export function getStatusLabel(st: string): string {
  const map: Record<string, string> = {
    completed: '成功',
    running: '进行中',
    cancelling: '取消中',
    cancelled: '已取消',
    completed_with_warnings: '警告',
    completed_with_errors: '失败',
    interrupted: '已中断',
  };
  return map[st] || st;
}

export function scopeText(item: Pick<UpdateItem, 'scope' | 'requested_rule_ids'>): string {
  if (item.scope === 'all') return '全部更新';
  const n = (item.requested_rule_ids || []).length;
  return n > 0 ? `指定 ${n} 条` : '指定规则';
}

export function displayTime(item: Pick<UpdateItem, 'status' | 'started_at' | 'finished_at'>): string | undefined {
  if (item.status === 'running' || item.status === 'cancelling' || !item.finished_at) {
    return item.started_at;
  }
  return item.finished_at;
}

export function formatTime(iso?: string): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString('zh-CN', { hour12: false });
}

export function changeCount(item: UpdateItem | UpdateDetail): number {
  if (item.changes && item.changes.length > 0) return item.changes.length;
  return item.change_count ?? 0;
}

export function finishSummary(detail: UpdateDetail): string {
  const label = getStatusLabel(detail.status);
  const failed = detail.rules_failed || 0;
  const changed = changeCount(detail);
  if (failed > 0) return `${label} · ${failed} 条`;
  if (changed > 0) return `${label} · 变更 ${changed}`;
  return label;
}

// A prepare-stage issue in a full update means the update aborted before the
// Geosite refresh ran (topological sort failure or rejected update), so the
// digest must not claim Geosite succeeded.
function hasGeositeBlocker(detail: UpdateDetail): boolean {
  return (detail.issues || []).some((issue) => issue.stage === 'prepare');
}

function hasGeositeIssue(detail: UpdateDetail): boolean {
  return (detail.issues || []).some((issue) => (issue.stage || '').startsWith('geosite'));
}

export function updateDigest(detail: UpdateDetail): string {
  const checked = detail.effective_rule_ids?.length || detail.rules_total || 0;
  const changed = changeCount(detail);
  const requested = detail.requested_rule_ids || [];
  const failed = detail.rules_failed || 0;
  const parts: string[] = [];
  if (detail.scope === 'all') {
    parts.push(`规则 ${checked}`, `变更 ${changed}`);
    if (hasGeositeIssue(detail)) {
      parts.push('Geosite 更新失败');
    } else if (hasGeositeBlocker(detail)) {
      parts.push('Geosite 未更新');
    } else {
      parts.push('Geosite 已更新');
    }
  } else {
    parts.push(`指定 ${requested.length}`, `含依赖 ${checked}`, `变更 ${changed}`);
  }
  if (failed > 0) {
    parts.push(`失败 ${failed}`);
  }
  return parts.join(' · ');
}
