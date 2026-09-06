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
  <div class="geosite-toolbar">
    <div class="toolbar-left">
      <span class="count-badge">共 {providers.length} 个 Provider 源</span>
    </div>
    <div class="toolbar-right">
      <PixelButton size="sm" onclick={loadGeosite}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </div>
  </div>

  {#if error}
    <div class="geosite-error">
      <PixelIcon name="warn" size={16} />
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
                <span class="font-name">{p.name}</span>
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
            <td class="text-dim">{p.version || '—'}</td>
            <td class="num">{p.lists.toLocaleString()}</td>
            <td class="num">{p.variants.toLocaleString()}</td>
            <td class="num text-display">{p.entries.toLocaleString()}</td>
            <td class="num">{p.files.toLocaleString()}</td>
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

  .geosite-toolbar {
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

  .geosite-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--status-error);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    color: var(--text);
  }

  .p-name {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--display);
  }

  .status-cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .check-time {
    color: var(--dim);
    font-variant-numeric: tabular-nums;
  }

  .text-dim {
    color: var(--dim);
  }
  .text-display {
    color: var(--display);
  }

  .table-empty {
    text-align: center;
    color: var(--dim);
    padding: 36px 0;
  }
</style>
