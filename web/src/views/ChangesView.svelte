<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type ChangeItem } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import { retroScroll } from '../utils/scrollbars';

  let changes = $state<ChangeItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  onMount(() => {
    loadChanges();
  });

  export async function loadChanges() {
    loading = true;
    error = null;
    try {
      const res = await api.getChanges(100);
      changes = res.items || [];
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function formatTime(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleString('zh-CN', { hour12: false });
  }
</script>

<div class="changes-view">
  <div class="changes-toolbar">
    <div class="toolbar-left">
      <span class="count-badge">共 {changes.length} 条变更记录</span>
    </div>
    <div class="toolbar-right">
      <PixelButton size="sm" onclick={loadChanges}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="changes-error">
      <PixelIcon name="warn" size={16} />
      <span>读取变动记录失败：{error}</span>
      <PixelButton size="sm" onclick={loadChanges}>重试</PixelButton>
    </div>
  {/if}

  {#if loading && changes.length === 0}
    <div class="changes-empty">加载变动记录中…</div>
  {:else if changes.length === 0}
    <div class="changes-empty">暂无变更记录</div>
  {:else}
    <div class="changes-list">
      {#each changes as item}
        <details class="change-card">
          <summary class="change-summary">
            <div class="summary-left">
              <span class="summary-toggle">›</span>
              <span class="change-time font-timestamp">{formatTime(item.finished_at)}</span>
              <span class="rule-name font-name">{item.rule_name}</span>
            </div>
            <div class="summary-diff num">
              <span class="diff-add">+{item.added.toLocaleString()}</span>
              <span class="diff-sep">/</span>
              <span class="diff-del">-{item.removed.toLocaleString()}</span>
            </div>
          </summary>

          <div class="change-details">
            <div class="detail-section">
              <div class="section-k">IR 规则条目 Diff</div>
              <div class="pixel-code-block" use:retroScroll role="region" aria-label={`${item.rule_name} IR Diff`}>
                {#each item.removed_samples || [] as line}
                  <div class="del-line">- {line}</div>
                {/each}
                {#if item.removed_omitted > 0}
                  <div class="omit-line">… 另有 {item.removed_omitted.toLocaleString()} 条移除已省略</div>
                {/if}
                {#each item.added_samples || [] as line}
                  <div class="add-line">+ {line}</div>
                {/each}
                {#if item.added_omitted > 0}
                  <div class="omit-line">… 另有 {item.added_omitted.toLocaleString()} 条新增已省略</div>
                {/if}
              </div>
            </div>
          </div>
        </details>
      {/each}
    </div>
  {/if}
</div>

<style>
  .changes-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .changes-toolbar {
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

  .changes-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--status-error);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    color: var(--text);
  }

  .changes-empty {
    padding: 36px 16px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    color: var(--dim);
    text-align: center;
  }

  .changes-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .change-card {
    background: var(--surface);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    box-shadow: var(--highlight-top), var(--shadow-panel);
  }

  .change-summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    cursor: pointer;
    list-style: none;
    transition: background-color 80ms linear;
  }
  .change-summary::-webkit-details-marker {
    display: none;
  }
  .change-summary:hover {
    background: var(--surface-2);
  }

  .change-card[open] .change-summary {
    border-bottom: 1px solid var(--border-vis);
    background: var(--surface-2);
  }

  .summary-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .summary-toggle {
    color: var(--dim);
    transition: transform 80ms linear;
  }
  .change-card[open] .summary-toggle {
    transform: rotate(90deg);
    color: var(--text);
  }

  .summary-diff {
    font-variant-numeric: tabular-nums;
  }
  .diff-add {
    color: var(--diff-add);
  }
  .diff-del {
    color: var(--diff-remove);
  }
  .diff-sep {
    color: var(--dim);
    margin: 0 2px;
  }

  .change-details {
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    background: var(--surface);
  }

  .section-k {
    color: var(--sec);
    margin-bottom: 6px;
  }

  .pixel-code-block {
    background: var(--terminal-bg);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    padding: 10px 12px;
    font: 400 13px/20px var(--font-code);
    color: var(--terminal-text);
    max-height: 240px;
    overflow-y: auto;
    scrollbar-color: var(--bevel-dark) var(--surface-2);
  }

  .add-line {
    color: var(--diff-add);
  }
  .del-line {
    color: var(--diff-remove);
  }
  .omit-line {
    color: var(--terminal-muted);
  }
</style>
