<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type UpdateItem, type UpdateDetail } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import {
    changeCount,
    displayTime,
    formatTime,
    getStatusLabel,
    getStatusType,
    originText,
    scopeText,
    updateDigest,
  } from '../updateLabels';

  const CHANGE_PREVIEW_LIMIT = 20;
  const REQUESTED_LIST_LIMIT = 5;

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

  function requestedIds(detail: UpdateDetail): string[] {
    return detail.requested_rule_ids || [];
  }

  function previewChanges(detail: UpdateDetail) {
    const all = detail.changes || [];
    return {
      shown: all.slice(0, CHANGE_PREVIEW_LIMIT),
      hidden: Math.max(0, all.length - CHANGE_PREVIEW_LIMIT),
    };
  }
</script>

<div class="updates-view">
  <div class="updates-header">
    <div class="header-left">
      <h2 class="view-title">更新日志</h2>
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
        <span>时间</span>
        <span>状态</span>
        <span>来源</span>
        <span>范围</span>
        <span class="text-right">变更</span>
        <span class="text-right">文件</span>
      </div>

      {#each updates as item (item.id)}
        {@const isExpanded = !!expandedDetails[item.id] || !!loadingDetails[item.id]}
        {@const detail = expandedDetails[item.id]}
        <div class="update-card {isExpanded ? 'open' : ''}">
          <button
            type="button"
            class="update-summary"
            onclick={() => toggleExpand(item.id)}
            aria-expanded={isExpanded}
          >
            <span class="toggle-icon">{isExpanded ? '▼' : '▶'}</span>
            <span class="col-time font-mono">{formatTime(displayTime(item))}</span>
            <span>
              <PixelBadge status={getStatusType(item.status)} pulse={item.status === 'running'}>
                {getStatusLabel(item.status)}
              </PixelBadge>
            </span>
            <span class="col-origin">{originText(item.origin)}</span>
            <span class="col-scope">{scopeText(item)}</span>
            <span class="col-num text-right font-mono">{changeCount(item)}</span>
            <span class="col-num text-right font-mono">{(item.artifacts_processed ?? 0).toLocaleString()}</span>
          </button>

          {#if isExpanded}
            <div class="update-detail-panel">
              {#if loadingDetails[item.id]}
                <div class="loading-hint">加载详情…</div>
              {:else if detail}
                <div class="digest">{updateDigest(detail)}</div>

                {#if detail.scope !== 'all'}
                  {@const requested = requestedIds(detail)}
                  {#if requested.length > 0 && requested.length <= REQUESTED_LIST_LIMIT}
                    <div class="detail-block">
                      <div class="block-k">指定规则</div>
                      <div class="tag-chips">
                        {#each requested as rid}
                          <span class="rule-chip">{rid}</span>
                        {/each}
                      </div>
                    </div>
                  {/if}
                {/if}

                {#if (detail.changes || []).length > 0}
                  {@const preview = previewChanges(detail)}
                  <div class="detail-block">
                    <div class="block-k">变更 ({(detail.changes || []).length})</div>
                    {#each preview.shown as ch}
                      <div class="change-row">
                        <span class="change-name">{ch.rule_name}</span>
                        <span class="change-diff font-mono">
                          <span class="diff-add">+{ch.added.toLocaleString()}</span>
                          <span class="diff-sep">/</span>
                          <span class="diff-del">-{ch.removed.toLocaleString()}</span>
                        </span>
                      </div>
                    {/each}
                    {#if preview.hidden > 0}
                      <div class="more-hint">其余 {preview.hidden} 条见变更对比</div>
                    {/if}
                  </div>
                {/if}

                {#if detail.warnings && detail.warnings.length > 0}
                  <div class="detail-block">
                    <div class="block-k text-orange">警告 ({detail.warnings.length})</div>
                    {#each detail.warnings as w}
                      <div class="warn-msg">! {w}</div>
                    {/each}
                  </div>
                {/if}

                {#if detail.issues && detail.issues.length > 0}
                  <div class="detail-block">
                    <div class="block-k text-red">失败 ({detail.issues.length})</div>
                    {#each detail.issues as issue}
                      <div class="error-msg">
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
    grid-template-columns: 24px minmax(170px, 1.3fr) 90px 70px minmax(110px, 1fr) 80px 90px;
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
    grid-template-columns: 24px minmax(170px, 1.3fr) 90px 70px minmax(110px, 1fr) 80px 90px;
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

  .digest {
    font-size: 13px;
    line-height: 1.6;
    color: var(--display);
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

  .change-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 10px;
    background: var(--bg);
    border-left: 3px solid var(--border-vis);
  }
  .change-name {
    font-size: 12px;
    color: var(--display);
  }
  .change-diff {
    font-size: 11px;
    white-space: nowrap;
  }
  .diff-add {
    color: var(--green);
  }
  .diff-sep {
    color: var(--dim);
    margin: 0 4px;
  }
  .diff-del {
    color: var(--red);
  }
  .more-hint {
    font-size: 11px;
    color: var(--dim);
    padding: 2px 2px 0;
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
  }

  .loading-hint {
    color: var(--dim);
    font-size: 11px;
  }
</style>
