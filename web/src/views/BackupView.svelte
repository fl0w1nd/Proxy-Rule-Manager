<script lang="ts">
  import { onMount } from 'svelte';
  import { api, APIRequestError, type ConfigBackupDetail, type ConfigBackupItem } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import { retroScroll } from '../utils/scrollbars';

  let { onstatechange, onimport }: { onstatechange?: (dirty: boolean, busy: boolean) => void; onimport?: (yaml: string) => void } = $props();
  let items = $state<ConfigBackupItem[]>([]);
  let version = $state(0);
  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let success = $state(false);
  let fileInput = $state<HTMLInputElement>();
  let pending = $state<ConfigBackupItem | null>(null);
  let confirmOpen = $state(false);
  let openID = $state('');
  let detail = $state<ConfigBackupDetail | null>(null);
  let detailLoading = $state(false);
  $effect(() => { onstatechange?.(false, busy); });

  async function load() {
    loading = true;
    message = '';
    openID = '';
    detail = null;
    try {
      const result = await api.listConfigBackups();
      items = result.items;
      version = result.version;
    } catch (error) {
      success = false;
      message = `读取快照失败：${(error as Error).message}`;
    } finally { loading = false; }
  }
  onMount(() => { void load(); });

  function formatTime(iso: string) {
    return new Date(iso).toLocaleString('zh-CN', { hour12: false });
  }
  function failed(error: unknown) {
    success = false;
    message = (error as Error).message;
    if (error instanceof APIRequestError) {
      if (error.code === 'config_version_conflict') message += '。请刷新快照列表后重试。';
      if (error.code === 'config_dirty') message += '。请先重新加载外部修改。';
    }
  }
  async function download() {
    if (busy) return;
    busy = true;
    message = '';
    try {
      const snapshot = await api.getConfigRaw();
      const blob = new Blob([snapshot.yaml], { type: 'text/yaml' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = snapshot.path.split(/[/\\]/).pop() || 'config.yaml';
      link.click();
      URL.revokeObjectURL(url);
    } catch (error) { failed(error); }
    finally { busy = false; }
  }
  async function imported(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file || busy) return;
    try {
      onimport?.(await file.text());
    } catch (error) { failed(error); }
  }
  async function toggle(item: ConfigBackupItem) {
    if (openID === item.id) {
      openID = '';
      detail = null;
      return;
    }
    openID = item.id;
    detail = null;
    detailLoading = true;
    try {
      detail = await api.getConfigBackup(item.id);
    } catch (error) {
      failed(error);
      openID = '';
    } finally { detailLoading = false; }
  }
  async function restore() {
    const target = pending;
    pending = null;
    if (!target || busy || version < 1) return;
    busy = true;
    message = '';
    try {
      const result = await api.restoreConfigBackup(target.id, version);
      version = result.version;
      success = true;
      message = ['已回滚到所选快照', ...(result.warnings ?? [])].join('；');
    } catch (error) {
      failed(error);
      busy = false;
      return;
    }
    try {
      const listed = await api.listConfigBackups();
      items = listed.items;
      version = listed.version;
      openID = '';
      detail = null;
    } catch { /* restore already applied */ }
    finally { busy = false; }
  }
</script>

<section class="backup-view" aria-label="备份与恢复">
  <section aria-labelledby="backup-current-title">
    <h2 id="backup-current-title">当前配置</h2>
    <p>下载当前正在使用的配置，或导入文件到配置文件页校验后再保存。</p>
    <div class="actions">
      <PixelButton disabled={busy} onclick={download}>下载当前配置</PixelButton>
      <PixelButton disabled={busy} onclick={() => fileInput?.click()}>导入配置文件</PixelButton>
      <input bind:this={fileInput} type="file" accept=".yaml,.yml,.txt,text/yaml,text/plain" hidden onchange={imported} />
    </div>
  </section>
  <section aria-labelledby="backup-history-title">
    <header>
      <div>
        <h2 id="backup-history-title">历史快照</h2>
        <p>每次成功保存前会留下一份。+/− 是相对当前配置将增加 / 删除的行。</p>
      </div>
      <PixelButton size="sm" disabled={busy} onclick={load}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
    </header>
    {#if loading && items.length === 0 && !message}
      <div class="status" role="status">正在读取快照…</div>
    {:else if items.length === 0}
      <div class="status" role="status">暂无快照</div>
    {:else}
      <div class="backup-list">
        {#each items as item (item.id)}
          <article class="backup-card" class:open={openID === item.id}>
            <div class="backup-summary">
              <span class="font-timestamp">{formatTime(item.created_at)}</span>
              <span class="summary-diff num">
                {#if item.added === 0 && item.removed === 0}
                  无差异
                {:else}
                  <span class="diff-add">+{item.added}</span>
                  <span class="diff-sep">/</span>
                  <span class="diff-del">-{item.removed}</span>
                {/if}
              </span>
              <PixelButton size="sm" disabled={busy} onclick={() => toggle(item)}>{openID === item.id ? '收起' : '查看差异'}</PixelButton>
            </div>
            {#if openID === item.id}
              <div class="backup-detail">
                {#if detailLoading || !detail || detail.id !== item.id}
                  <div class="status" role="status">正在读取差异…</div>
                {:else if detail.added === 0 && detail.removed === 0}
                  <p>与当前配置相同。</p>
                {:else}
                  <div class="pixel-code-block" use:retroScroll role="region" aria-label="相对当前配置的差异">
                    {#each detail.lines as line}
                      <div class:add-line={line.kind === 'add'} class:del-line={line.kind === 'del'}>
                        {line.kind === 'add' ? '+' : '-'} {line.text}
                      </div>
                    {/each}
                  </div>
                {/if}
                {#if detail && detail.id === item.id}
                  <div class="detail-actions">
                    <PixelButton size="sm" disabled={busy} onclick={() => { if (detail) onimport?.(detail.yaml); }}>导入此快照</PixelButton>
                    <PixelButton size="sm" variant="danger" disabled={busy || (detail.added === 0 && detail.removed === 0)}
                      onclick={() => { pending = item; confirmOpen = true; }}>回滚</PixelButton>
                  </div>
                {/if}
              </div>
            {/if}
          </article>
        {/each}
      </div>
    {/if}
  </section>
  {#if message}<p class="message" class:success role={success ? 'status' : 'alert'}>{message}</p>{/if}
</section>
<PixelDialog bind:open={confirmOpen} title="回滚到此快照？" confirmLabel="确认回滚" cancelLabel="取消" danger
  oncancel={() => { pending = null; }} onconfirm={restore}>
  {pending ? `将用 ${formatTime(pending.created_at)} 的快照替换当前配置，系统会立即热重载。` : ''}
</PixelDialog>

<style>
  .backup-view { display: flex; flex-direction: column; gap: 20px; }
  section > section {
    padding: 20px 24px 24px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    box-shadow: inset 0 1px 0 var(--bevel-light), var(--shadow-panel);
  }
  header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
  h2 { margin: 0 0 8px; font: 400 12px/20px var(--font-ui); color: var(--display); }
  header h2 { margin-bottom: 6px; }
  p { margin: 0 0 16px; color: var(--sec); font: 400 12px/20px var(--font-ui); }
  header p { margin: 0; }
  .actions { display: flex; flex-wrap: wrap; gap: 10px; }
  .status, .message { margin: 0; padding: 12px 16px; border: 1px solid var(--border-vis); border-radius: 4px; background: var(--status-error); color: var(--text); font: 400 12px/20px var(--font-ui); overflow-wrap: anywhere; }
  .status { background: var(--surface-2); }
  .success { background: var(--status-success); }
  .backup-list { display: flex; flex-direction: column; gap: 10px; }
  .backup-card {
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    box-shadow: var(--highlight-top), var(--shadow-panel);
  }
  .backup-summary {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    flex-wrap: wrap;
  }
  .backup-card.open .backup-summary { border-bottom: 1px solid var(--border-vis); background: var(--surface-2); }
  .font-timestamp { flex: 1; min-width: 0; }
  .summary-diff { font-variant-numeric: tabular-nums; }
  .diff-add { color: var(--diff-add); }
  .diff-del { color: var(--diff-remove); }
  .diff-sep { color: var(--dim); margin: 0 2px; }
  .backup-detail { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  .backup-detail p { margin: 0; }
  .pixel-code-block {
    background: var(--terminal-bg);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    padding: 10px 12px;
    font: 400 13px/20px var(--font-code);
    color: var(--terminal-text);
    max-height: 280px;
    overflow: auto;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .add-line { color: var(--diff-add); }
  .del-line { color: var(--diff-remove); }
  .detail-actions { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 10px; }
</style>
