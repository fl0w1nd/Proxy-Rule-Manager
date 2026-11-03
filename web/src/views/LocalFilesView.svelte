<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, APIRequestError, type LocalFileDetail, type LocalFileItem } from '../api/client';
  import LocalFileEditorDrawer from '../components/LocalFileEditorDrawer.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelTable from '../components/pixel/PixelTable.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import { formatFileSize, referencedRuleLabel, validateLocalFileName } from './localFiles';

  let { onstatechange }: { onstatechange?: (dirty: boolean, busy: boolean) => void } = $props();

  let items = $state<LocalFileItem[]>([]);
  let searchQuery = $state('');
  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let success = $state(false);
  let fileInput = $state<HTMLInputElement>();

  let editorOpen = $state(false);
  let editorName = $state('');
  let editorDirty = $state(false);
  let editorBusy = $state(false);

  let pendingDelete = $state<LocalFileItem | null>(null);
  let deleteOpen = $state(false);
  let overwriteOpen = $state(false);
  let pendingUpload = $state<{ name: string; content: string } | null>(null);

  const filtered = $derived(
    items.filter((item) => !searchQuery || item.name.toLowerCase().includes(searchQuery.toLowerCase()))
  );
  const editingRules = $derived(items.find((item) => item.name === editorName)?.rules ?? []);
  $effect(() => { onstatechange?.(editorDirty, busy || editorBusy); });
  onMount(() => { void load(); });
  onDestroy(() => { onstatechange?.(false, false); });

  async function load() {
    loading = true;
    message = '';
    try {
      const result = await api.listLocalFiles();
      items = result.items ?? [];
    } catch (error) {
      success = false;
      message = `读取本地文件失败：${(error as Error).message}`;
    } finally { loading = false; }
  }

  function formatTime(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleString('zh-CN', { hour12: false });
  }

  function failed(error: unknown) {
    success = false;
    message = (error as Error).message;
    if (error instanceof APIRequestError && error.code === 'file_in_use') {
      const names = referencedRuleLabel(error.details.rules);
      if (names) message = `文件正被规则 ${names} 引用，无法删除`;
    }
  }

  function upsert(detail: LocalFileDetail) {
    const { content: _content, ...item } = detail;
    const index = items.findIndex((entry) => entry.name === detail.name);
    if (index >= 0) items[index] = item;
    else items = [...items, item].sort((a, b) => a.name.localeCompare(b.name));
  }

  function openEditor(name = '') {
    if (busy || editorOpen) return;
    message = '';
    editorName = name;
    editorOpen = true;
  }

  function saved(detail: LocalFileDetail) {
    upsert(detail);
    editorName = detail.name;
  }

  async function remove() {
    const target = pendingDelete;
    pendingDelete = null;
    if (!target || busy) return;
    busy = true;
    message = '';
    try {
      await api.deleteLocalFile(target.name);
      items = items.filter((item) => item.name !== target.name);
      success = true;
      message = `已删除 ${target.name}`;
    } catch (error) { failed(error); }
    finally { busy = false; }
  }

  async function uploaded(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file || busy) return;
    const nameErrorText = validateLocalFileName(file.name);
    if (nameErrorText) {
      success = false;
      message = nameErrorText;
      return;
    }
    let content: string;
    try {
      content = await file.text();
    } catch (error) {
      failed(error);
      return;
    }
    if (items.some((item) => item.name === file.name)) {
      pendingUpload = { name: file.name, content };
      overwriteOpen = true;
      return;
    }
    await writeUploaded(file.name, content, true);
  }

  async function writeUploaded(name: string, content: string, create: boolean) {
    busy = true;
    message = '';
    try {
      const detail = create ? await api.createLocalFile(name, content) : await api.saveLocalFile(name, content);
      upsert(detail);
      success = !detail.diagnostics;
      message = (create ? `已上传 ${detail.name}` : `已覆盖 ${detail.name}`)
        + (detail.diagnostics ? `，其中 ${detail.diagnostics} 行无法解析` : '');
    } catch (error) { failed(error); }
    finally { busy = false; }
  }
</script>

<svelte:window onbeforeunload={event => { if (editorDirty || busy || editorBusy) { event.preventDefault(); event.returnValue = ''; } }} />

<div class="local-files-view">
  <div class="files-toolbar">
    <div class="toolbar-left">
      <span class="count-badge">
        {#if searchQuery}
          匹配 {filtered.length} / 共 {items.length} 个文件
        {:else}
          共 {items.length} 个文件
        {/if}
      </span>
    </div>
    <div class="toolbar-right">
      <div class="search-wrap">
        <PixelInput placeholder="搜索文件名…" bind:value={searchQuery} spellcheck="false" aria-label="搜索文件" />
      </div>
      <PixelButton size="sm" disabled={busy} onclick={() => fileInput?.click()}>上传</PixelButton>
      <PixelButton size="sm" variant="primary" disabled={busy} onclick={() => openEditor()}>新建文件</PixelButton>
      <PixelButton size="sm" disabled={busy} onclick={load}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
      <input bind:this={fileInput} type="file" accept=".list,.yaml,.txt,text/plain,text/yaml" hidden onchange={uploaded} />
    </div>
  </div>

  {#if message}
    <div class="files-message" class:success role={success ? 'status' : 'alert'}>{message}</div>
  {/if}

  <PixelTable minWidth="880px">
    <thead>
      <tr>
        <th style="width: 28%;">文件名</th>
        <th style="width: 16%;">状态</th>
        <th style="width: 14%;">引用</th>
        <th style="width: 9%;" class="num">大小</th>
        <th style="width: 8%;" class="num">行数</th>
        <th style="width: 13%;">修改时间</th>
        <th style="width: 12%; text-align: right;">操作</th>
      </tr>
    </thead>
    <tbody>
      {#if loading && items.length === 0}
        <tr><td colspan="7" class="table-empty">加载本地文件中…</td></tr>
      {:else if filtered.length === 0}
        <tr><td colspan="7" class="table-empty">{items.length === 0 ? '还没有本地规则文件' : '没有匹配的文件'}</td></tr>
      {:else}
        {#each filtered as item (item.name)}
          <tr class:selected={editorOpen && editorName === item.name}>
            <td class="font-code">{item.name}</td>
            <td>
              {#if item.diagnostics === undefined}
                <span class="text-sec">—</span>
              {:else if item.diagnostics > 0}
                <PixelBadge status="warning">{item.format ? `${item.diagnostics} 行无法解析` : '无法识别格式'}</PixelBadge>
              {:else}
                <PixelBadge status="success">{(item.entries ?? 0).toLocaleString()} 条</PixelBadge>
              {/if}
            </td>
            <td class="text-sec refs" title={referencedRuleLabel(item.rules)}>{item.rules?.length ? referencedRuleLabel(item.rules) : '—'}</td>
            <td class="num">{formatFileSize(item.size)}</td>
            <td class="num">{item.lines.toLocaleString()}</td>
            <td class="text-sec">{formatTime(item.modified_at)}</td>
            <td style="text-align: right;">
              <div class="row-actions">
                <PixelButton size="sm" disabled={busy} onclick={() => openEditor(item.name)}>编辑</PixelButton>
                <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => { pendingDelete = item; deleteOpen = true; }}>删除</PixelButton>
              </div>
            </td>
          </tr>
        {/each}
      {/if}
    </tbody>
  </PixelTable>
</div>

<LocalFileEditorDrawer bind:open={editorOpen} name={editorName} rules={editingRules} onsaved={saved}
  onstatechange={(dirty, busyState) => { editorDirty = dirty; editorBusy = busyState; }} />

<PixelDialog bind:open={deleteOpen} title="删除此文件？" confirmLabel="确认删除" cancelLabel="取消" danger
  oncancel={() => { pendingDelete = null; }} onconfirm={remove}>
  {pendingDelete ? `将删除 ${pendingDelete.name}。` : ''}
</PixelDialog>
<PixelDialog bind:open={overwriteOpen} title="覆盖已有文件？" confirmLabel="覆盖" cancelLabel="取消" danger
  oncancel={() => { pendingUpload = null; }}
  onconfirm={() => { const file = pendingUpload; pendingUpload = null; if (file) void writeUploaded(file.name, file.content, false); }}>
  {pendingUpload ? `${pendingUpload.name} 已存在，上传将替换当前内容。` : ''}
</PixelDialog>
<style>
  .local-files-view { display: flex; flex-direction: column; gap: 16px; }
  .files-toolbar, .toolbar-left, .toolbar-right, .row-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .files-toolbar { justify-content: space-between; gap: 14px; flex-wrap: wrap; }
  .toolbar-right { flex-wrap: wrap; }
  .search-wrap { width: 220px; }
  .files-message {
    padding: 10px 14px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--status-error);
    color: var(--text);
    overflow-wrap: anywhere;
  }
  .files-message.success { background: var(--status-success); }
  .text-sec { color: var(--sec); }
  .table-empty { text-align: center; color: var(--dim); padding: 36px 0; }
  .row-actions { justify-content: flex-end; }
  .refs { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 0; }
</style>
