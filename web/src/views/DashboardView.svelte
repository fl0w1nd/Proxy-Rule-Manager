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
  <div class="dashboard-header">
    <div class="header-left">
      <h2 class="view-title">概览</h2>
    </div>
    <div class="header-right">
      <PixelButton size="sm" onclick={loadData}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="dashboard-error">
      <PixelIcon name="warn" size={16} color="var(--red)" />
      <span>读取状态失败：{error}</span>
      <PixelButton size="sm" onclick={loadData}>重试</PixelButton>
    </div>
  {/if}

  <div class="facts-grid">
    <PixelCard class="fact-card">
      <div class="fact-label">上次更新</div>
      <div class="fact-val">
        {#if latestUpdate}
          {formatTime(displayTime(latestUpdate))}
        {:else}
          {formatTime(status?.last_check)}
        {/if}
      </div>
      <div class="fact-sub">
        {#if latestUpdate}
          <PixelBadge status={getStatusType(latestUpdate.status)} pulse={latestUpdate.status === 'running'}>
            {getStatusLabel(latestUpdate.status)}
          </PixelBadge>
        {:else if !loading}
          <span class="fact-muted">暂无记录</span>
        {/if}
      </div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">规则</div>
      <div class="fact-val">{ruleCount === null ? '—' : ruleCount.toLocaleString()}</div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">Geosite 源</div>
      <div class="fact-val">{geositeCount === null ? '—' : geositeCount.toLocaleString()}</div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">规则文件</div>
      <div class="fact-val">{status ? status.published_artifacts.toLocaleString() : '—'}</div>
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
              <span class="recent-time">{formatTime(displayTime(item))}</span>
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

  .dashboard-header {
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

  .dashboard-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--red-dim);
    border: 2px solid var(--red);
    color: var(--red);
    font-family: "Space Mono", monospace;
    font-size: 12px;
  }

  .facts-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
  }

  .fact-label {
    font-family: "Space Mono", monospace;
    font-size: 12px;
    font-weight: 700;
    color: var(--sec);
    margin-bottom: 8px;
    letter-spacing: 0.05em;
  }

  .fact-val {
    font-family: "Doto", "Space Mono", monospace;
    font-size: 20px;
    font-weight: 800;
    color: var(--display);
    letter-spacing: 0.04em;
  }

  .fact-sub {
    margin-top: 10px;
    min-height: 22px;
  }

  .fact-muted {
    font-size: 11px;
    color: var(--dim);
  }

  .empty-hint {
    color: var(--dim);
    text-align: center;
    padding: 24px 0;
    font-size: 12px;
  }

  .recent-list {
    display: flex;
    flex-direction: column;
  }

  .recent-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 9px 0;
    border-bottom: 1px dashed var(--border);
    gap: 12px;
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

  .recent-time {
    font-family: "Space Mono", monospace;
    font-size: 12px;
    font-weight: 700;
    color: var(--display);
  }

  .recent-scope {
    font-size: 11px;
    color: var(--dim);
  }

  .recent-changed {
    font-size: 11px;
    color: var(--sec);
  }
</style>
