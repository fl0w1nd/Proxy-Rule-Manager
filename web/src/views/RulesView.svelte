<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type RuleItem } from '../api/client';
  import PixelTable from '../components/pixel/PixelTable.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';

  interface Props {
    onStartUpdate: (scope: 'rules', ruleIds: string[]) => void;
    activeRuleId?: string | null;
    isUpdating?: boolean;
    currentProcessingRuleId?: string | null;
  }

  let {
    onStartUpdate,
    activeRuleId = null,
    isUpdating = false,
    currentProcessingRuleId = null,
  }: Props = $props();

  let rules = $state<RuleItem[]>([]);
  let searchQuery = $state('');
  let loading = $state(true);
  let error = $state<string | null>(null);

  onMount(() => {
    loadRules();
  });

  export async function loadRules() {
    loading = true;
    error = null;
    try {
      const res = await api.getRules();
      rules = res.items || [];
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  let filteredRules = $derived(
    rules.filter((r) => {
      if (!searchQuery) return true;
      const q = searchQuery.toLowerCase();
      return r.name.toLowerCase().includes(q) || r.id.toLowerCase().includes(q);
    })
  );

  function formatTime(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleString('zh-CN', { hour12: false });
  }

  function getCheckStatusType(res?: string): 'success' | 'warning' | 'error' | 'neutral' | 'active' {
    if (!res) return 'neutral';
    if (res === 'updated') return 'success';
    if (res === 'unchanged') return 'neutral';
    if (res === 'failed') return 'error';
    if (res === 'cancelled') return 'warning';
    if (res === 'updating') return 'active';
    return 'neutral';
  }

  function getCheckStatusLabel(res?: string): string {
    const map: Record<string, string> = {
      updated: '已更新',
      unchanged: '无变化',
      failed: '失败',
      cancelled: '已取消',
      updating: '更新中',
      none: '未检查',
    };
    return (res && map[res]) || res || '未检查';
  }
</script>

<div class="rules-view">
  <div class="rules-header">
    <div class="header-left">
      <h2 class="view-title">规则更新状态</h2>
      <span class="count-badge">{filteredRules.length} / {rules.length}</span>
    </div>
    <div class="header-right">
      <div class="search-wrap">
        <input
          type="text"
          class="pixel-input"
          placeholder="搜索规则名称 / ID…"
          bind:value={searchQuery}
          spellcheck="false"
        />
      </div>
      <PixelButton size="sm" onclick={loadRules}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="rules-error">
      <PixelIcon name="warn" size={16} color="var(--red)" />
      <span>读取规则列表失败：{error}</span>
      <PixelButton size="sm" onclick={loadRules}>重试</PixelButton>
    </div>
  {/if}

  <PixelTable minWidth="760px">
    <thead>
      <tr>
        <th style="width: 32%;">名称 / ID</th>
        <th style="width: 14%;" class="num">条目数量</th>
        <th style="width: 22%;">内容版本时间</th>
        <th style="width: 22%;">上次检查状态</th>
        <th style="width: 10%; text-align: right;">操作</th>
      </tr>
    </thead>
    <tbody>
      {#if loading && rules.length === 0}
        <tr>
          <td colspan="5" class="table-empty">加载规则列表中…</td>
        </tr>
      {:else if filteredRules.length === 0}
        <tr>
          <td colspan="5" class="table-empty">没有匹配的规则</td>
        </tr>
      {:else}
        {#each filteredRules as rule (rule.id)}
          {@const isRuleActive = activeRuleId === rule.id || currentProcessingRuleId === rule.id}
          <tr>
            <td>
              <div class="rule-name">{rule.name}</div>
              <div class="rule-id">{rule.id}</div>
            </td>
            <td class="num font-mono">{rule.entries.toLocaleString()}</td>
            <td class="font-mono text-sec">{formatTime(rule.version_at)}</td>
            <td>
              <div class="status-cell">
                <PixelBadge status={isRuleActive ? 'active' : getCheckStatusType(rule.last_check?.result)} pulse={isRuleActive}>
                  {isRuleActive ? '更新中…' : getCheckStatusLabel(rule.last_check?.result)}
                </PixelBadge>
                {#if rule.last_check?.checked_at}
                  <span class="check-time">{formatTime(rule.last_check.checked_at)}</span>
                {/if}
              </div>
            </td>
            <td style="text-align: right;">
              <PixelButton
                size="sm"
                variant="secondary"
                disabled={isUpdating || isRuleActive}
                title={isUpdating ? '当前有更新任务正在进行中' : '更新此规则'}
                onclick={() => onStartUpdate('rules', [rule.id])}
              >
                更新
              </PixelButton>
            </td>
          </tr>
        {/each}
      {/if}
    </tbody>
  </PixelTable>
</div>

<style>
  .rules-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .rules-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    flex-wrap: wrap;
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

  .header-right {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .pixel-input {
    background: var(--surface);
    border: 2px solid var(--border-vis);
    box-shadow: inset 1px 1px 0 var(--bevel-dark);
    color: var(--display);
    font-family: "Space Mono", monospace;
    font-size: 12px;
    padding: 6px 10px;
    width: 240px;
    outline: none;
    transition: border-color 80ms;
  }
  .pixel-input:focus {
    border-color: var(--orange);
    outline: 2px solid var(--orange);
    outline-offset: 1px;
  }
  .pixel-input::placeholder {
    color: var(--dim);
  }

  .rules-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--red-dim);
    border: 2px solid var(--red);
    color: var(--red);
    font-size: 12px;
  }

  .rule-name {
    font-weight: 700;
    color: var(--display);
    font-size: 13px;
  }

  .rule-id {
    font-family: "Space Mono", monospace;
    font-size: 10px;
    color: var(--dim);
    margin-top: 2px;
  }

  .font-mono {
    font-family: "Space Mono", monospace;
  }
  .text-sec {
    color: var(--sec);
    font-size: 11px;
  }

  .status-cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .check-time {
    font-family: "Space Mono", monospace;
    font-size: 10px;
    color: var(--dim);
  }

  .table-empty {
    text-align: center;
    color: var(--dim);
    padding: 32px 0;
  }
</style>
