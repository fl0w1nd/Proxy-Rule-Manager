<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type ChangeItem } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';

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
  <div class="changes-header">
    <div class="header-left">
      <h2 class="view-title">规则变动对比</h2>
      <span class="count-badge">{changes.length} 条记录</span>
    </div>
    <div class="header-right">
      <PixelButton size="sm" onclick={loadChanges}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="changes-error">
      <PixelIcon name="warn" size={16} color="var(--red)" />
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
              <span class="change-time">{formatTime(item.finished_at)}</span>
              <span class="rule-name">{item.rule_name}</span>
            </div>
            <div class="summary-diff font-mono">
              <span class="diff-add">+{item.added.toLocaleString()}</span>
              <span class="diff-sep">/</span>
              <span class="diff-del">-{item.removed.toLocaleString()}</span>
            </div>
          </summary>

          <div class="change-details">
            <div class="detail-section">
              <div class="section-k">IR 规则条目 Diff</div>
              <div class="pixel-code-block" role="region" aria-label={`${item.rule_name} IR Diff`}>
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

  .changes-header {
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

  .changes-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--red-dim);
    border: 2px solid var(--red);
    color: var(--red);
    font-size: 12px;
  }

  .changes-empty {
    text-align: center;
    color: var(--dim);
    padding: 48px 0;
    font-family: "Space Mono", monospace;
    font-size: 13px;
    border: 2px dashed var(--border-vis);
    background: var(--surface);
  }

  .changes-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .change-card {
    background: var(--surface);
    border: 2px solid var(--border-vis);
    box-shadow:
      inset 1px 1px 0 var(--bevel-light),
      inset -1px -1px 0 var(--bevel-dark),
      2px 2px 0 var(--shadow);
  }

  .change-summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    cursor: pointer;
    list-style: none;
    transition: background 80ms steps(2, end);
  }
  .change-summary::-webkit-details-marker {
    display: none;
  }
  .change-summary:hover {
    background: var(--surface-2);
  }

  .change-card[open] .change-summary {
    border-bottom: 2px dashed var(--border-vis);
    background: var(--surface-2);
  }

  .summary-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .summary-toggle {
    font-family: "Space Mono", monospace;
    font-size: 14px;
    font-weight: 700;
    color: var(--dim);
    transition: transform 80ms steps(2, end);
  }
  .change-card[open] .summary-toggle {
    transform: rotate(90deg);
    color: var(--orange);
  }

  .change-time {
    font-family: "Space Mono", monospace;
    font-size: 12px;
    color: var(--sec);
  }

  .rule-name {
    font-weight: 700;
    color: var(--display);
    font-size: 13px;
  }

  .font-mono {
    font-family: "Space Mono", monospace;
  }

  .summary-diff {
    font-size: 12px;
    font-weight: 700;
  }
  .diff-add {
    color: var(--green);
  }
  .diff-del {
    color: var(--orange);
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
    background: var(--surface-2);
    border-left: 4px solid var(--orange);
  }

  .section-k {
    font-family: "Space Mono", monospace;
    font-size: 11px;
    font-weight: 700;
    color: var(--sec);
    margin-bottom: 6px;
  }

  .pixel-code-block {
    background: var(--terminal-bg);
    border: 2px solid var(--border-vis);
    box-shadow: inset 1px 1px 0 rgba(0, 0, 0, 0.6);
    padding: 10px 12px;
    font-family: "Space Mono", monospace;
    font-size: 11px;
    line-height: 1.6;
    color: var(--terminal-text-dim, #cbd2bf);
    max-height: 240px;
    overflow-y: auto;
    scrollbar-color: var(--border-vis) var(--bg);
  }

  .add-line {
    color: var(--green);
  }
  .del-line {
    color: var(--orange);
  }
  .omit-line {
    color: var(--dim);
    font-style: italic;
  }
</style>
