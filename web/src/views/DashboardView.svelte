<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type SystemStatus, type UpdateItem } from '../api/client';
  import PixelCard from '../components/pixel/PixelCard.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';

  interface Props {
    onStartUpdate: (scope: 'all' | 'rules', ruleIds?: string[]) => void;
    onViewUpdates: () => void;
  }

  let { onStartUpdate, onViewUpdates }: Props = $props();

  let status = $state<SystemStatus | null>(null);
  let recentUpdates = $state<UpdateItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  onMount(() => {
    loadData();
  });

  export async function loadData() {
    loading = true;
    error = null;
    try {
      const [s, u] = await Promise.all([
        api.getStatus(),
        api.getUpdates(5),
      ]);
      status = s;
      recentUpdates = u.items || [];
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

<div class="dashboard-root">
  {#if error}
    <div class="dashboard-error">
      <PixelIcon name="warn" size={16} color="var(--red)" />
      <span>读取状态失败：{error}</span>
      <PixelButton size="sm" onclick={loadData}>重试</PixelButton>
    </div>
  {/if}

  <!-- Stats Big Cards -->
  <div class="facts-grid">
    <PixelCard class="fact-card">
      <div class="fact-label">最近全量检查</div>
      <div class="fact-val">{formatTime(status?.last_check)}</div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">当前发布产物</div>
      <div class="fact-val">{status ? status.published_artifacts.toLocaleString() : '—'}</div>
    </PixelCard>
    <PixelCard class="fact-card">
      <div class="fact-label">运行时版本</div>
      <div class="fact-val version-val">{status ? `${status.version} · ${status.go_version}` : '—'}</div>
    </PixelCard>
  </div>

  <!-- Quick Action & Health -->
  <div class="overview-grid">
    <PixelCard title="系统操作">
      <div class="quick-actions">
        <p class="quick-desc">触发全量规则与 Geosite 资源重新拉取、解析、过滤与编译发布。</p>
        <div class="actions-row">
          <PixelButton variant="primary" onclick={() => onStartUpdate('all')}>
            <PixelIcon name="refresh" size={14} color="#fff" />
            立即执行全部更新
          </PixelButton>
          <PixelButton onclick={loadData}>
            刷新看板数据
          </PixelButton>
        </div>
      </div>
    </PixelCard>

    <PixelCard title="最近更新活动">
      {#snippet actions()}
        <PixelButton variant="ghost" size="sm" onclick={onViewUpdates}>
          查看全部日志 &gt;
        </PixelButton>
      {/snippet}

      {#if recentUpdates.length === 0}
        <div class="empty-hint">暂无最近更新记录</div>
      {:else}
        <div class="recent-list">
          {#each recentUpdates as item}
            <div class="recent-item">
              <div class="recent-left">
                <span class="recent-time">{formatTime(item.started_at)}</span>
                <span class="recent-scope">{item.scope === 'all' ? '全部规则' : '指定规则'}</span>
              </div>
              <div class="recent-right">
                <PixelBadge
                  status={item.status === 'completed' ? 'success' : item.status === 'running' ? 'active' : item.status.includes('error') ? 'error' : 'warning'}
                >
                  {item.status === 'completed' ? '完成' : item.status === 'running' ? '运行中' : item.status}
                </PixelBadge>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </PixelCard>
  </div>
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
    background: var(--red-dim);
    border: 2px solid var(--red);
    color: var(--red);
    font-family: "Space Mono", monospace;
    font-size: 12px;
  }

  .facts-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
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

  .version-val {
    font-family: "Space Mono", monospace;
    font-size: 14px;
  }

  .overview-grid {
    display: grid;
    grid-template-columns: 1fr 1.2fr;
    gap: 16px;
  }

  .quick-desc {
    font-size: 13px;
    color: var(--sec);
    line-height: 1.6;
    margin-bottom: 16px;
  }

  .actions-row {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
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
  }
  .recent-item:last-child {
    border-bottom: none;
  }

  .recent-left {
    display: flex;
    align-items: center;
    gap: 12px;
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

  @media (max-width: 900px) {
    .overview-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
