<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type GeositeProviderItem } from '../api/client';
  import PixelTable from '../components/pixel/PixelTable.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';

  let providers = $state<GeositeProviderItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  onMount(() => {
    loadGeosite();
  });

  export async function loadGeosite() {
    loading = true;
    error = null;
    try {
      const res = await api.getGeositeProviders();
      providers = res.items || [];
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

  function getStatusType(res?: string): 'success' | 'warning' | 'error' | 'neutral' {
    if (res === 'updated' || res === 'unchanged') return 'success';
    if (res === 'failed') return 'error';
    return 'neutral';
  }
</script>

<div class="geosite-view">
  <div class="geosite-header">
    <div class="header-left">
      <h2 class="view-title">Geosite Provider 状态</h2>
      <span class="count-badge">{providers.length} 个 Provider</span>
    </div>
    <div class="header-right">
      <PixelButton size="sm" onclick={loadGeosite}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="geosite-error">
      <PixelIcon name="warn" size={16} color="var(--red)" />
      <span>读取 Geosite 状态失败：{error}</span>
      <PixelButton size="sm" onclick={loadGeosite}>重试</PixelButton>
    </div>
  {/if}

  <PixelTable minWidth="880px">
    <thead>
      <tr>
        <th style="width: 22%;">Provider 名称</th>
        <th style="width: 20%;">检查状态 · 时间</th>
        <th style="width: 18%;">数据版本</th>
        <th style="width: 10%;" class="num">列表数</th>
        <th style="width: 10%;" class="num">变体数</th>
        <th style="width: 10%;" class="num">条目总量</th>
        <th style="width: 10%;" class="num">规则文件</th>
      </tr>
    </thead>
    <tbody>
      {#if loading && providers.length === 0}
        <tr>
          <td colspan="7" class="table-empty">加载 Geosite 状态中…</td>
        </tr>
      {:else if providers.length === 0}
        <tr>
          <td colspan="7" class="table-empty">没有配置 Geosite Provider</td>
        </tr>
      {:else}
        {#each providers as p}
          <tr>
            <td>
              <div class="p-name">
                <PixelIcon name="globe" size={14} color="var(--orange)" />
                <span>{p.name}</span>
              </div>
            </td>
            <td>
              <div class="status-cell">
                <PixelBadge status={getStatusType(p.result)}>
                  {p.result === 'updated' ? '已更新' : p.result === 'unchanged' ? '无变化' : p.result === 'failed' ? '失败' : '未检查'}
                </PixelBadge>
                {#if p.checked_at}
                  <span class="check-time">{formatTime(p.checked_at)}</span>
                {/if}
              </div>
            </td>
            <td class="font-mono text-dim">{p.version || '—'}</td>
            <td class="num font-mono">{p.lists.toLocaleString()}</td>
            <td class="num font-mono">{p.variants.toLocaleString()}</td>
            <td class="num font-mono text-display">{p.entries.toLocaleString()}</td>
            <td class="num font-mono">{p.files.toLocaleString()}</td>
          </tr>
        {/each}
      {/if}
    </tbody>
  </PixelTable>
</div>

<style>
  .geosite-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .geosite-header {
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

  .geosite-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--red-dim);
    border: 2px solid var(--red);
    color: var(--red);
    font-size: 12px;
  }

  .p-name {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 700;
    color: var(--display);
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

  .font-mono {
    font-family: "Space Mono", monospace;
  }
  .text-dim {
    color: var(--dim);
    font-size: 11px;
  }
  .text-display {
    color: var(--display);
    font-weight: 700;
  }

  .table-empty {
    text-align: center;
    color: var(--dim);
    padding: 32px 0;
  }
</style>
