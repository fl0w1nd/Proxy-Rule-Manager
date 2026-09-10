<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, APIRequestError, type LocalFileDetail, type LocalFileItem } from '../api/client';
  import CodeEditor from '../components/CodeEditor.svelte';
  import PixelTable from '../components/pixel/PixelTable.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import rulesIcon from '../assets/icons/nav/rules.svg';
  import { formatFileSize, localFileLanguage, referencedRuleLabel, validateLocalFileName } from './localFiles';

  let { onstatechange }: { onstatechange?: (dirty: boolean, busy: boolean) => void } = $props();

  let items = $state<LocalFileItem[]>([]);
  let searchQuery = $state('');
  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let success = $state(false);
  let fileInput = $state<HTMLInputElement>();

  let editorOpen = $state(false);
  let creating = $state(false);
  let draftName = $state('');
  let draftContent = $state('');
  let savedContent = $state('');
  let nameError = $state('');
  let editorLoading = $state(false);
  let editorRequestId = 0;

  let pendingDelete = $state<LocalFileItem | null>(null);
  let deleteOpen = $state(false);
  let discardOpen = $state(false);
  let overwriteOpen = $state(false);
  let pendingUpload = $state<{ name: string; content: string } | null>(null);

  const filtered = $derived(
    items.filter((item) => !searchQuery || item.name.toLowerCase().includes(searchQuery.toLowerCase()))
  );
  const editorLanguage = $derived(localFileLanguage(draftName));
  const dirty = $derived(editorOpen && (creating ? draftName.trim() !== '' || draftContent !== '' : draftContent !== savedContent));
  $effect(() => { onstatechange?.(dirty, busy || editorLoading); });
  onMount(() => { void load(); });
  onDestroy(() => { editorRequestId++; onstatechange?.(false, false); });

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
    const item: LocalFileItem = { name: detail.name, size: detail.size, lines: detail.lines, modified_at: detail.modified_at };
    const index = items.findIndex((entry) => entry.name === detail.name);
    if (index >= 0) items[index] = item;
    else items = [...items, item].sort((a, b) => a.name.localeCompare(b.name));
  }

  function requestClose() {
    if (busy) return false;
    if (dirty) {
      discardOpen = true;
      return false;
    }
    return true;
  }

  function closeEditor() {
    editorRequestId++;
    editorOpen = false;
    creating = false;
    draftName = '';
    draftContent = '';
    savedContent = '';
    nameError = '';
    editorLoading = false;
  }

  function openCreate() {
    if (busy || editorLoading || dirty) return;
    creating = true;
    draftName = '';
    draftContent = '';
    savedContent = '';
    nameError = '';
    message = '';
    editorOpen = true;
  }

  async function openEdit(item: LocalFileItem) {
    if (busy || editorLoading || dirty) return;
    const requestId = ++editorRequestId;
    creating = false;
    draftName = item.name;
    nameError = '';
    message = '';
    editorLoading = true;
    editorOpen = true;
    try {
      const detail = await api.getLocalFile(item.name);
      if (requestId !== editorRequestId) return;
      draftContent = detail.content;
      savedContent = detail.content;
    } catch (error) {
      if (requestId !== editorRequestId) return;
      editorOpen = false;
      failed(error);
    } finally {
      if (requestId === editorRequestId) editorLoading = false;
    }
  }

  async function save() {
    if (busy || editorLoading) return;
    const name = creating ? draftName.trim() : draftName;
    if (creating) {
      nameError = validateLocalFileName(draftName) ?? '';
      if (nameError) return;
    }
    busy = true;
    message = '';
    try {
      const detail = creating
        ? await api.createLocalFile(name, draftContent)
        : await api.saveLocalFile(name, draftContent);
      upsert(detail);
      creating = false;
      draftName = detail.name;
      savedContent = detail.content;
      success = true;
      message = `已保存 ${detail.name}`;
    } catch (error) {
      failed(error);
      if (error instanceof APIRequestError && error.code === 'invalid_filename') {
        nameError = '文件名不能包含路径，且需以 .list、.yaml 或 .txt 结尾';
      }
    } finally { busy = false; }
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
      if (editorOpen && draftName === target.name) closeEditor();
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
      success = true;
      message = create ? `已上传 ${detail.name}` : `已覆盖 ${detail.name}`;
    } catch (error) { failed(error); }
    finally { busy = false; }
  }
</script>

<svelte:window onbeforeunload={event => { if (dirty || busy) { event.preventDefault(); event.returnValue = ''; } }} />

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
      <PixelButton size="sm" variant="primary" disabled={busy} onclick={openCreate}>新建文件</PixelButton>
      <PixelButton size="sm" disabled={busy} onclick={load}>
        <PixelIcon name="refresh" size={12} />
        刷新
      </PixelButton>
      <input bind:this={fileInput} type="file" accept=".list,.yaml,.txt,text/plain,text/yaml" hidden onchange={uploaded} />
    </div>
  </div>

  {#if message && !editorOpen}
    <div class="files-message" class:success role={success ? 'status' : 'alert'}>{message}</div>
  {/if}

  <PixelTable minWidth="720px">
    <thead>
      <tr>
        <th style="width: 36%;">文件名</th>
        <th style="width: 14%;" class="num">大小</th>
        <th style="width: 14%;" class="num">行数</th>
        <th style="width: 22%;">修改时间</th>
        <th style="width: 14%; text-align: right;">操作</th>
      </tr>
    </thead>
    <tbody>
      {#if loading && items.length === 0}
        <tr><td colspan="5" class="table-empty">加载本地文件中…</td></tr>
      {:else if filtered.length === 0}
        <tr><td colspan="5" class="table-empty">{items.length === 0 ? '还没有本地规则文件' : '没有匹配的文件'}</td></tr>
      {:else}
        {#each filtered as item (item.name)}
          <tr class:selected={editorOpen && draftName === item.name}>
            <td class="font-code">{item.name}</td>
            <td class="num">{formatFileSize(item.size)}</td>
            <td class="num">{item.lines.toLocaleString()}</td>
            <td class="text-sec">{formatTime(item.modified_at)}</td>
            <td style="text-align: right;">
              <div class="row-actions">
                <PixelButton size="sm" disabled={busy} onclick={() => openEdit(item)}>编辑</PixelButton>
                <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => { pendingDelete = item; deleteOpen = true; }}>删除</PixelButton>
              </div>
            </td>
          </tr>
        {/each}
      {/if}
    </tbody>
  </PixelTable>
</div>

<PixelDrawer
  bind:open={editorOpen}
  title={creating ? '新建本地文件' : draftName || '本地文件'}
  icon={rulesIcon}
  width="760px"
  scrollable={false}
  onrequestclose={requestClose}
  onclose={closeEditor}
>
  <div class="drawer-editor-form">
    {#if editorLoading}
      <p class="editor-status" role="status">正在读取文件…</p>
    {:else}
      {#if creating}
        <label class="field" for="local-file-name">
          <span>文件名</span>
          <PixelInput id="local-file-name" error={!!nameError} bind:value={draftName}
            placeholder="my-direct.list" spellcheck="false" oninput={() => { nameError = ''; }} />
          {#if nameError}<span class="field-error">{nameError}</span>{/if}
        </label>
      {/if}
      {#if editorOpen}
        <div class="editor-field">
          <CodeEditor fill filename={draftName} language={editorLanguage} label="文件内容" value={draftContent} readonly={busy} onchange={(value) => { draftContent = value; }} />
        </div>
      {/if}
    {/if}
    {#if message && editorOpen}
      <div class="files-message editor-message" class:success role={success ? 'status' : 'alert'}>{message}</div>
    {/if}
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestClose()) closeEditor(); }}>取消</PixelButton>
    <PixelButton variant="primary" disabled={busy || editorLoading || (!creating && !dirty)} onclick={save}>
      {busy ? '保存中…' : '保存'}
    </PixelButton>
  {/snippet}
</PixelDrawer>

<PixelDialog bind:open={deleteOpen} title="删除此文件？" confirmLabel="确认删除" cancelLabel="取消" danger
  oncancel={() => { pendingDelete = null; }} onconfirm={remove}>
  {pendingDelete ? `将删除 ${pendingDelete.name}。` : ''}
</PixelDialog>
<PixelDialog bind:open={discardOpen} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger
  onconfirm={closeEditor}>
  当前修改尚未保存，关闭后将丢弃这些修改。
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
  .editor-message { flex-shrink: 0; margin: 0; max-height: 120px; overflow-y: auto; }
  .text-sec { color: var(--sec); }
  .table-empty { text-align: center; color: var(--dim); padding: 36px 0; }
  .row-actions { justify-content: flex-end; }
  .drawer-editor-form {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    height: 100%;
    gap: 12px;
  }
  .editor-status { margin: 0; color: var(--sec); }
  .field { display: flex; flex-direction: column; gap: 6px; flex-shrink: 0; }
  .field span { color: var(--display); font: 400 12px/20px var(--font-ui); }
  .field-error { color: var(--error-border); }
  .editor-field {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
</style>
