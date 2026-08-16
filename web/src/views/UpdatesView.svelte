<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type UpdateItem, type UpdateDetail } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';

  let updates = $state<UpdateItem[]>([]);
  let expandedDetails = $state<Record<string, UpdateDetail>>({});
  let loadingDetails = $state<Record<string, boolean>>({});
  let loading = $state(true);
  let error = $state<string | null>(null);

  onMount(() => {
    loadUpdates();
  });

  export async function loadUpdates() {
    loading = true;
    error = null;
    try {
      const res = await api.getUpdates(100);
      updates = res.items || [];
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function toggleExpand(id: string) {
    if (expandedDetails[id]) {
      const next = { ...expandedDetails };
      delete next[id];
      expandedDetails = next;
      return;
    }

    loadingDetails = { ...loadingDetails, [id]: true };
    try {
      const detail = await api.getUpdateDetail(id);
      expandedDetails = { ...expandedDetails, [id]: detail };
    } catch (e) {
      console.error('Failed to load update detail', e);
    } finally {
      const nextLoading = { ...loadingDetails };
      delete nextLoading[id];
      loadingDetails = nextLoading;
    }
  }

  function formatTime(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleString('zh-CN', { hour12: false });
  }

  function originText(v?: string) {
    const map: Record<string, string> = { web: '管理页', scheduled: '定时调度', cli: '命令行' };
    return (v && map[v]) || v || '—';
  }

  function getStatusType(st: string): 'success' | 'warning' | 'error' | 'active' | 'neutral' {
    if (st === 'completed') return 'success';
    if (st === 'running' || st === 'cancelling') return 'active';
    if (st === 'completed_with_warnings' || st === 'cancelled') return 'warning';
    if (st === 'completed_with_errors' || st === 'interrupted') return 'error';
    return 'neutral';
  }

  function getStatusLabel(st: string): string {
    const map: Record<string, string> = {
      completed: '完成',
      running: '运行中',
      cancelling: '取消中',
      cancelled: '已取消',
      completed_with_warnings: '带警告完成',
      completed_with_errors: '异常结束',
      interrupted: '已中断',
    };
    return map[st] || st;
  }
</script>

<div class="updates-view">
  <div class="updates-header">
    <div class="header-left">
      <h2 class="view-title">更新历史记录</h2>
      <span class="count-badge">{updates.length} 次更新</span>
    </div>
    <div class="header-right">
      <PixelButton size="sm" onclick={loadUpdates}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="updates-error">
      <PixelIcon name="warn" size={16} color="var(--red)" />
      <span>读取更新记录失败：{error}</span>
      <PixelButton size="sm" onclick={loadUpdates}>重试</PixelButton>
    </div>
  {/if}

  {#if loading && updates.length === 0}
    <div class="updates-empty">加载更新记录中…</div>
  {:else if updates.length === 0}
    <div class="updates-empty">暂无更新记录</div>
  {:else}
    <div class="updates-list">
      <div class="list-head">
        <span></span>
        <span>更新时间</span>
        <span>状态</span>
        <span>来源</span>
        <span>请求范围</span>
        <span class="text-right">成功 / 总计</span>
        <span class="text-right">产物数</span>
      </div>

      {#each updates as item (item.id)}
        {@const isExpanded = !!expandedDetails[item.id]}
        {@const detail = expandedDetails[item.id]}
        <div class="update-card {isExpanded ? 'open' : ''}">
          <button
            type="button"
            class="update-summary"
            onclick={() => toggleExpand(item.id)}
            aria-expanded={isExpanded}
          >
            <span class="toggle-icon">{isExpanded ? '▼' : '▶'}</span>
            <span class="col-time font-mono">{formatTime(item.started_at)}</span>
            <span>
              <PixelBadge status={getStatusType(item.status)} pulse={item.status === 'running'}>
                {getStatusLabel(item.status)}
              </PixelBadge>
            </span>
            <span class="col-origin">{originText(item.origin)}</span>
            <span class="col-scope">{item.scope === 'all' ? '全部规则' : `${(item.requested_rule_ids || []).length} 条指定规则`}</span>
            <span class="col-num text-right font-mono">{item.rules_succeeded} / {item.rules_total}</span>
            <span class="col-num text-right font-mono">{item.artifacts_processed}</span>
          </button>

          {#if isExpanded}
            <div class="update-detail-panel">
              {#if loadingDetails[item.id]}
                <div class="loading-hint">正在读取详细明细…</div>
              {:else if detail}
                {#if detail.effective_rule_ids && detail.effective_rule_ids.length > 0}
                  <div class="detail-block">
                    <div class="block-k">更新生效规则 ({detail.effective_rule_ids.length})</div>
                    <div class="tag-chips">
                      {#each detail.effective_rule_ids as rid}
                        <span class="rule-chip">{rid}</span>
                      {/each}
                    </div>
                  </div>
                {/if}

                {#if detail.warnings && detail.warnings.length > 0}
                  <div class="detail-block">
                    <div class="block-k text-orange">任务警告消息 ({detail.warnings.length})</div>
                    {#each detail.warnings as w}
                      <div class="warn-msg">! {w}</div>
                    {/each}
                  </div>
                {/if}

                {#if detail.issues && detail.issues.length > 0}
                  <div class="detail-block">
                    <div class="block-k text-red">异常与错误 ({detail.issues.length})</div>
                    {#each detail.issues as issue}
                      <div class="error-msg">
                        <span class="issue-stage">[{[issue.stage, issue.subject].filter(Boolean).join(' · ') || '异常'}]</span>
                        <span class="issue-text">{issue.message}</span>
                      </div>
                    {/each}
                  </div>
                {/if}
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .updates-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .updates-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .view-title {
    font-family: "Doto", "Space Mono", monospace;
    font-size: 16px;
    font-weight: 800;
    color: var(--display);
    letter-spacing: 0.04em;
    text-shadow: 1px 0 currentColor;
  }

  .count-badge {
    font-family: "Space Mono", monospace;
    font-size: 11px;
    color: var(--dim);
    background: var(--surface-2);
    border: 1px solid var(--border-vis);
    padding: 2px 6px;
  }

  .updates-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--red-dim);
    border: 2px solid var(--red);
    color: var(--red);
    font-size: 12px;
  }

  .updates-empty {
    text-align: center;
    color: var(--dim);
    padding: 48px 0;
    font-family: "Space Mono", monospace;
    font-size: 13px;
    border: 2px dashed var(--border-vis);
    background: var(--surface);
  }

  .updates-list {
    display: flex;
    flex-direction: column;
    border: 2px solid var(--border-vis);
    background: var(--surface);
    box-shadow: 3px 3px 0 var(--shadow);
    overflow-x: auto;
    min-width: 860px;
  }

  .list-head {
    display: grid;
    grid-template-columns: 24px minmax(180px, 1.3fr) 110px 80px minmax(120px, 1fr) 100px 80px;
    gap: 10px;
    align-items: center;
    padding: 10px 14px;
    background: var(--surface-2);
    border-bottom: 2px solid var(--border-vis);
    color: var(--sec);
    font-family: "Space Mono", monospace;
    font-size: 11px;
    font-weight: 700;
  }

  .update-card {
    border-bottom: 1px dashed var(--border);
  }
  .update-card:last-child {
    border-bottom: none;
  }

  .update-summary {
    display: grid;
    grid-template-columns: 24px minmax(180px, 1.3fr) 110px 80px minmax(120px, 1fr) 100px 80px;
    gap: 10px;
    align-items: center;
    padding: 11px 14px;
    width: 100%;
    background: transparent;
    border: none;
    text-align: left;
    font-family: inherit;
    color: inherit;
    cursor: pointer;
    transition: background 80ms steps(2, end);
  }
  .update-summary:hover {
    background: var(--surface-2);
  }

  .toggle-icon {
    font-size: 10px;
    color: var(--dim);
  }
  .update-card.open .toggle-icon {
    color: var(--orange);
  }

  .col-time {
    font-size: 12px;
    font-weight: 700;
    color: var(--display);
  }
  .col-origin {
    font-size: 11px;
    color: var(--sec);
  }
  .col-scope {
    font-size: 11px;
    color: var(--dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .text-right {
    text-align: right;
  }
  .font-mono {
    font-family: "Space Mono", monospace;
  }

  .update-detail-panel {
    padding: 14px 18px;
    background: var(--surface-2);
    border-top: 1px dashed var(--border-vis);
    border-left: 4px solid var(--orange);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .detail-block {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .block-k {
    font-family: "Space Mono", monospace;
    font-size: 11px;
    font-weight: 700;
    color: var(--sec);
  }
  .text-orange {
    color: var(--orange);
  }
  .text-red {
    color: var(--red);
  }

  .tag-chips {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .rule-chip {
    font-family: "Space Mono", monospace;
    font-size: 10px;
    padding: 2px 6px;
    background: var(--bg);
    border: 1px solid var(--border-vis);
    color: var(--text);
  }

  .warn-msg, .error-msg {
    padding: 6px 10px;
    font-family: "Space Mono", monospace;
    font-size: 11px;
    background: var(--bg);
    border-left: 3px solid var(--orange);
  }
  .error-msg {
    border-left-color: var(--red);
    display: flex;
    gap: 8px;
  }
  .issue-stage {
    color: var(--red);
    font-weight: 700;
  }

  .loading-hint {
    color: var(--dim);
    font-size: 11px;
  }
</style>
