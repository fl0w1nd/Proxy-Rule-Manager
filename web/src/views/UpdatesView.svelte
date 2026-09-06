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
  <div class="updates-toolbar">
    <div class="toolbar-left">
      <span class="count-badge">共 {updates.length} 次更新记录</span>
    </div>
    <div class="toolbar-right">
      <PixelButton size="sm" onclick={loadUpdates}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="updates-error">
      <PixelIcon name="warn" size={16} />
      <span>读取更新记录失败：{error}</span>
      <PixelButton size="sm" onclick={loadUpdates}>重试</PixelButton>
    </div>
  {/if}

  {#if loading && updates.length === 0}
    <div class="updates-empty">加载更新记录中…</div>
  {:else if updates.length === 0}
    <div class="updates-empty">暂无更新记录</div>
  {:else}
    <div class="updates-table-wrap">
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
              <span class="col-time font-timestamp">{formatTime(displayTime(item))}</span>
              <span>
                <PixelBadge status={getStatusType(item.status)} pulse={item.status === 'running'}>
                  {getStatusLabel(item.status)}
                </PixelBadge>
              </span>
              <span class="col-origin">{originText(item.origin)}</span>
              <span class="col-scope">{scopeText(item)}</span>
              <span class="col-num text-right num">{changeCount(item)}</span>
              <span class="col-num text-right num">{(item.artifacts_processed ?? 0).toLocaleString()}</span>
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
                          <span class="change-diff num">
                            <span class="diff-add">+{ch.added.toLocaleString()}</span>
                            <span class="diff-sep">/</span>
                            <span class="diff-del">-{ch.removed.toLocaleString()}</span>
                          </span>
                        </div>
                      {/each}
                      {#if preview.hidden > 0}
                        <div class="more-hint">其余 {preview.hidden} 条见 Diff</div>
                      {/if}
                    </div>
                  {/if}

                {#if detail.warnings && detail.warnings.length > 0}
                  <div class="detail-block">
                    <div class="block-k text-warn">警告 ({detail.warnings.length})</div>
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
    </div>
  {/if}
</div>

<style>
  .updates-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .updates-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    flex-wrap: wrap;
  }

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .updates-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--status-error);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    color: var(--text);
  }

  .updates-empty {
    padding: 36px 16px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    color: var(--dim);
    text-align: center;
  }

  .updates-table-wrap {
    width: 100%;
    overflow-x: auto;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface);
    scrollbar-color: var(--bevel-dark) var(--surface-2);
    scrollbar-width: thin;
  }

  .updates-list {
    display: flex;
    flex-direction: column;
    min-width: 860px;
  }

  .list-head {
    display: grid;
    grid-template-columns: 24px minmax(170px, 1.3fr) 90px 70px minmax(110px, 1fr) 80px 90px;
    gap: 10px;
    align-items: center;
    padding: 10px 14px;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-vis);
    color: var(--sec);
  }

  .update-card {
    border-bottom: 1px solid var(--border);
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
    transition: background-color 80ms linear;
  }
  .update-summary:hover {
    background: var(--surface-2);
  }
  .update-card.open .update-summary {
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-vis);
  }

  .toggle-icon {
    color: var(--dim);
  }
  .update-card.open .toggle-icon {
    color: var(--text);
  }

  .col-origin {
    color: var(--sec);
  }
  .col-scope {
    color: var(--dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .text-right {
    text-align: right;
  }

  .update-detail-panel {
    padding: 14px 18px;
    background: var(--surface-2);
    border-top: 1px solid var(--border-vis);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .digest {
    color: var(--display);
    font: 400 14px/22px var(--font-reading);
  }

  .detail-block {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .block-k {
    color: var(--sec);
  }
  .text-warn {
    color: var(--text);
  }
  .text-red {
    color: var(--text);
  }

  .tag-chips {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .rule-chip {
    padding: 0 6px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--sec);
  }

  .change-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 10px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 3px;
  }
  .change-name {
    color: var(--display);
    font: 400 20px/24px var(--font-display);
  }
  .change-diff {
    white-space: nowrap;
  }
  .diff-add {
    color: var(--diff-add);
  }
  .diff-sep {
    color: var(--dim);
    margin: 0 4px;
  }
  .diff-del {
    color: var(--diff-remove);
  }
  .more-hint {
    color: var(--dim);
    padding: 2px 2px 0;
  }

  .warn-msg {
    padding: 6px 10px;
    background: var(--status-warning);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    color: var(--text);
  }
  .error-msg {
    padding: 6px 10px;
    background: var(--status-error);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    color: var(--text);
  }

  .loading-hint {
    color: var(--dim);
  }
</style>
