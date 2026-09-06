<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type SystemStatus, type UpdateItem } from '../api/client';
  import PixelCard from '../components/pixel/PixelCard.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import {
    changeCount,
    displayTime,
    formatTime,
    getStatusLabel,
    getStatusType,
    scopeText,
  } from '../updateLabels';

  interface Props {
    onViewUpdates: () => void;
  }

  let { onViewUpdates }: Props = $props();

  let status = $state<SystemStatus | null>(null);
  let recentUpdates = $state<UpdateItem[]>([]);
  let ruleCount = $state<number | null>(null);
  let geositeCount = $state<number | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  const latestUpdate = $derived(recentUpdates[0] ?? null);

  onMount(() => {
    loadData();
  });

  export async function loadData() {
    loading = true;
    error = null;
    try {
      const [s, u, rules, geosite] = await Promise.all([
        api.getStatus(),
        api.getUpdates(5),
        api.getRules(),
        api.getGeositeProviders(),
      ]);
      status = s;
      recentUpdates = u.items || [];
      ruleCount = (rules.items || []).length;
      geositeCount = (geosite.items || []).length;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="dashboard-root">
  {#if error}
    <div class="dashboard-error">
      <PixelIcon name="warn" size={16} />
      <span>读取状态失败：{error}</span>
      <PixelButton size="sm" onclick={loadData}>重试</PixelButton>
    </div>
  {/if}

  <div class="facts-grid">
    <PixelCard class="fact-card">
      <div class="fact-label">规则</div>
      <div class="fact-val font-metric">{ruleCount === null ? '—' : ruleCount.toLocaleString()}</div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">Geosite 源</div>
      <div class="fact-val font-metric">{geositeCount === null ? '—' : geositeCount.toLocaleString()}</div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">规则文件</div>
      <div class="fact-val font-metric">{status ? status.published_artifacts.toLocaleString() : '—'}</div>
    </PixelCard>
  </div>

  <PixelCard title="最近更新">
    {#snippet actions()}
      <PixelButton variant="ghost" size="sm" onclick={onViewUpdates}>
        查看全部 &gt;
      </PixelButton>
    {/snippet}

    {#if recentUpdates.length === 0}
      <div class="empty-hint">暂无最近更新记录</div>
    {:else}
      <div class="recent-list">
        {#each recentUpdates as item}
          <div class="recent-item">
            <div class="recent-left">
              <span class="recent-time font-timestamp">{formatTime(displayTime(item))}</span>
              <span class="recent-scope">{scopeText(item)}</span>
              <span class="recent-changed">变更 {changeCount(item)}</span>
            </div>
            <div class="recent-right">
              <PixelBadge
                status={getStatusType(item.status)}
                pulse={item.status === 'running'}
              >
                {getStatusLabel(item.status)}
              </PixelBadge>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </PixelCard>
</div>

<style>
  .dashboard-root {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .dashboard-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--status-error);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    color: var(--text);
  }

  .facts-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 16px;
  }

  @media (max-width: 768px) {
    .facts-grid {
      grid-template-columns: 1fr;
    }
  }

  .fact-label {
    color: var(--sec);
    margin-bottom: 8px;
  }

  .fact-val {
    color: var(--display);
    line-height: 1;
  }

  .empty-hint {
    color: var(--dim);
    text-align: center;
    padding: 24px 0;
  }

  .recent-list {
    display: flex;
    flex-direction: column;
  }

  .recent-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--border);
    gap: 14px;
  }
  .recent-item:last-child {
    border-bottom: none;
  }

  .recent-left {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
    flex-wrap: wrap;
  }

  .recent-scope {
    color: var(--dim);
  }

  .recent-changed {
    color: var(--sec);
  }
</style>
