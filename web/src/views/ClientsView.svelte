<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, APIRequestError, type ConfigSnapshot, type IconItem, type TemplateItem } from '../api/client';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelCard from '../components/pixel/PixelCard.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelIconPicker from '../components/pixel/PixelIconPicker.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import OpsEditor from '../components/forms/OpsEditor.svelte';
  import TemplatesView from './TemplatesView.svelte';
  import { clientIconID, clientIconSrc, clientReferences, defaultClientIcon, validateClient, type ClientConfig } from './clients';
  import { retroScroll } from '../utils/scrollbars';
  import clientIcon from '../assets/icons/nav/clients.svg';

  let { onstatechange }: { onstatechange?: (dirty: boolean, busy: boolean) => void } = $props();
  let snapshot = $state<ConfigSnapshot>();
  let templates = $state<TemplateItem[]>([]);
  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let error = $state(false);
  let tab = $state('clients');
  let templateDirty = $state(false);
  let templateBusy = $state(false);
  let open = $state(false);
  let editing = $state('');
  let draft = $state<ClientConfig>({ id: '', name: '', template: '' });
  let baseline = $state('');
  let discard = $state(false);
  let pendingTab = $state('');
  let deleteOpen = $state(false);
  let deleting = $state<ClientConfig>();
  let refConflictOpen = $state(false);
  let refConflictClient = $state<ClientConfig>();
  let refConflictList = $state<string[]>([]);
  let iconPickerOpen = $state(false);
  let iconPickerLoading = $state(false);
  let iconItems = $state<IconItem[]>([]);
  const clients = $derived((snapshot?.config.clients ?? []) as ClientConfig[]);
  const dirty = $derived(open && JSON.stringify(draft) !== baseline);
  const options = $derived([{ value: '', label: '请选择模板' }, ...templates.map(t => ({ value: t.id, label: `${t.name || t.id} · ${t.id}` }))]);
  const variantOptions = $derived((draft.formats?.length ?? 0) > 1 ? options : [{ value: '', label: '继承客户端格式' }, ...options.slice(1)]);
  const iconPickerItems = $derived(iconItems.map((item) => ({ id: item.id, src: `/static/icons/${encodeURIComponent(item.file)}`, label: item.id })));
  const iconChosen = $derived(!!clientIconID(draft.icon));
  const iconPreviewID = $derived(iconChosen ? clientIconID(draft.icon) : defaultClientIcon(draft.id));
  const iconPreviewSrc = $derived(iconChosen ? clientIconSrc(draft.icon) : `/static/icons/${encodeURIComponent(iconPreviewID)}.svg`);

  function getClientIconSrc(client: ClientConfig): string {
    const icon = client.icon;
    if (icon && clientIconID(icon)) return clientIconSrc(icon);
    const id = defaultClientIcon(client.id);
    return `/static/icons/${encodeURIComponent(id)}.svg`;
  }
  $effect(() => { onstatechange?.(dirty || templateDirty, busy || templateBusy); });
  onMount(() => { void load(); });
  onDestroy(() => { onstatechange?.(false, false); });

  async function load() {
    loading = true;
    try { const [cfg, list] = await Promise.all([api.getConfig(), api.listTemplates()]); snapshot = cfg; templates = list.items; }
    catch (e) { fail(e); }
    finally { loading = false; }
  }
  function fail(e: unknown) {
    error = true;
    message = (e as Error).message;
    if (e instanceof APIRequestError && e.details.errors?.length) message += '：' + e.details.errors.map(i => `${i.path} ${i.message}`).join('；');
    if (e instanceof APIRequestError && e.status === 409) message += '。请关闭编辑器后刷新配置再重试。';
  }
  function edit(client?: ClientConfig) {
    editing = client?.id ?? '';
    draft = client ? JSON.parse(JSON.stringify(client)) : { id: '', name: '', template: '' };
    baseline = JSON.stringify(draft);
    message = ''; open = true;
  }
  function requestClose() {
    if (busy) return false;
    if (dirty) { discard = true; return false; }
    return true;
  }
  function switchTab(next: string) {
    if (busy || templateBusy || next === tab) return;
    if (dirty || templateDirty) { pendingTab = next; discard = true; return; }
    tab = next;
    if (next === 'clients') void load();
  }
  async function openIconPicker() {
    if (busy) return;
    iconItems = [];
    iconPickerOpen = true;
    iconPickerLoading = true;
    try {
      iconItems = (await api.listIcons()).items;
    } catch (e) {
      iconPickerOpen = false;
      fail(e);
    } finally {
      iconPickerLoading = false;
    }
  }
  function applyIcon(id: string) {
    draft.icon = id;
  }
  function clearIcon() {
    delete draft.icon;
  }
  async function save() {
    if (!snapshot || busy) return;
    const validation = validateClient(draft, templates, clients, editing);
    if (validation) { message = validation; error = true; return; }
    busy = true; message = '';
    try {
      const value = JSON.parse(JSON.stringify(draft)) as ClientConfig;
      const result = await api.patchConfig(snapshot.version, [editing ? { op: 'update_client', id: editing, value } : { op: 'add_client', value }]);
      snapshot = { version: result.version, config: { ...snapshot.config, clients: editing ? clients.map(c => c.id === editing ? value : c) : [...clients, value] } };
      editing = value.id; baseline = JSON.stringify(draft);
      error = false; message = result.warnings?.length ? `已保存。${result.warnings.join('；')}` : '客户端已保存';
    } catch (e) { fail(e); }
    finally { busy = false; }
  }
  function askDelete(client: ClientConfig) {
    const refs = clientReferences(snapshot!.config, client.id);
    if (refs.length) {
      error = true;
      refConflictClient = client;
      refConflictList = refs;
      if (refs.length > 3) {
        message = `客户端「${client.name || client.id}」正被 ${refs.length} 项配置（如 ${refs.slice(0, 2).join('、')} 等）引用，无法直接删除`;
      } else {
        message = `客户端正被 ${refs.join('、')} 引用，无法删除`;
      }
      return;
    }
    deleting = client; deleteOpen = true;
  }
  async function remove() {
    if (!snapshot || !deleting || busy) return;
    busy = true;
    try {
      const result = await api.patchConfig(snapshot.version, [{ op: 'remove_client', id: deleting.id }]);
      snapshot = { version: result.version, config: { ...snapshot.config, clients: clients.filter(c => c.id !== deleting?.id) } };
      error = false; message = '客户端已删除';
    } catch (e) { fail(e); }
    finally { busy = false; deleting = undefined; }
  }
</script>

<svelte:window onbeforeunload={event => { if (dirty || templateDirty || busy || templateBusy) { event.preventDefault(); event.returnValue = ''; } }} />
<div class="clients-page">
  <PixelTabs id="clients-tabs" label="客户端管理" items={[{ value: 'clients', label: '客户端' }, { value: 'templates', label: '模板' }]} value={tab} onchange={switchTab} />
  {#if tab === 'templates'}
    <TemplatesView onstatechange={(dirty, busy) => { templateDirty = dirty; templateBusy = busy; }} />
  {:else}
    <div class="toolbar"><span>{clients.length} 个客户端</span><div class="actions"><PixelButton disabled={loading || busy} onclick={load}>刷新</PixelButton><PixelButton variant="primary" disabled={loading || busy || !snapshot} onclick={() => edit()}>新建客户端</PixelButton></div></div>
    {#if message && !open}
      <div class="notice" class:error role="status">
        <div class="notice-inner">
          <span class="notice-msg">{message}</span>
          {#if refConflictList.length > 3}
            <PixelButton size="sm" variant="ghost" onclick={() => { refConflictOpen = true; }}>查看引用清单 ({refConflictList.length})</PixelButton>
          {/if}
        </div>
      </div>
    {/if}
    {#if loading}<div class="notice">读取客户端…</div>
    {:else if clients.length === 0}<div class="notice">暂无客户端</div>
    {:else}<div class="cards">
      {#each clients as client (client.id)}
        <PixelCard class="client-card">
          <div class="client-heading">
            <div class="client-identity">
              <img src={getClientIconSrc(client)} class="client-avatar" width="32" height="32" alt="" />
              <div class="client-meta">
                <h2>{client.name || client.id}</h2>
                <code>{client.id}</code>
              </div>
            </div>
          </div>
          <div class="targets">
            {#if !client.formats?.length}
              <div class="target-row">
                <span class="target-name">默认格式</span>
                <code>{client.template}</code>
              </div>
            {/if}
            {#each client.formats ?? [] as format}
              <div class="target-row">
                <span class="target-name">{format.name || format.id}</span>
                <code>{format.template}</code>
              </div>
            {/each}
            {#each client.variants ?? [] as variant}
              <div class="target-row variant-row">
                <span class="target-name variant-name">
                  <span class="variant-tag">变体</span>
                  {variant.name || variant.id}
                </span>
                <PixelBadge status={variant.ops?.length ? 'info' : 'neutral'}>
                  {variant.ops?.length ?? 0} 项过滤
                </PixelBadge>
              </div>
            {/each}
          </div>
          <div class="card-footer">
            <div class="actions">
              <PixelButton size="sm" disabled={busy} onclick={() => edit(client)}>编辑</PixelButton>
              <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => askDelete(client)}>删除</PixelButton>
            </div>
          </div>
        </PixelCard>
      {/each}
    </div>{/if}
  {/if}
</div>
<PixelDrawer bind:open title={editing ? '编辑客户端' : '新建客户端'} icon={clientIcon} width="760px" onrequestclose={requestClose}>
  <div class="editor-form">
    {#if message}<div class="notice" class:error role="status">{message}</div>{/if}
    <fieldset disabled={busy}>
      <legend>基本属性</legend>
      <div class="fields">
        <label>客户端 ID<PixelInput bind:value={draft.id} disabled={!!editing} placeholder="例如：clash, surge" /></label>
        <label>名称<PixelInput bind:value={draft.name} placeholder="例如：Clash Verge" /></label>
        <div class="full icon-field">
          <span>图标</span>
          <div class="icon-row">
            <img src={iconPreviewSrc} width="32" height="32" alt="" />
            <code>{iconPreviewID}</code>
            {#if !iconChosen}<span class="icon-unset">默认</span>{/if}
            <span class="icon-action"><PixelButton size="sm" disabled={busy} onclick={openIconPicker}>选择</PixelButton></span>
          </div>
        </div>
      </div>
    </fieldset>
    <section>
      <div class="section-head"><h3>输出格式</h3><PixelButton size="sm" disabled={busy} onclick={() => {
        if (!draft.formats?.length) { draft.formats = [{ id: draft.id, name: '', template: draft.template ?? '' }, { id: '', name: '', template: '' }]; delete draft.template; }
        else draft.formats = [...draft.formats, { id: '', name: '', template: '' }];
      }}>添加格式</PixelButton></div>
      {#if !draft.formats?.length}
        <div class="format-single">
          <PixelSelect id="client-template" label="客户端模板" {options} value={draft.template ?? ''} disabled={busy} onchange={v => { draft.template = v; }} />
          <p class="field-hint">当前为单格式模式。若需要同时输出多种格式（如 Classical + YAML），可点击右上角「添加格式」。</p>
        </div>
      {/if}
      {#each draft.formats ?? [] as format, i}
        <fieldset disabled={busy}>
          <div class="item-head">
            <legend>格式 {i + 1}</legend>
            <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => {
              draft.formats = draft.formats?.filter((_, j) => i !== j);
              if (!draft.formats?.length) { delete draft.formats; draft.template = format.template; }
            }}>删除格式</PixelButton>
          </div>
          <div class="fields"><label>格式 ID<PixelInput bind:value={format.id} placeholder="输出文件名 ID" /></label><label>格式名称<PixelInput bind:value={format.name} placeholder="显示名称" /></label></div>
          <PixelSelect id="format-template-{i}" label="格式 {i + 1} 模板" {options} bind:value={format.template} disabled={busy} />
        </fieldset>
      {/each}
    </section>
    <section>
      <div class="section-head"><h3>变体规则</h3><PixelButton size="sm" disabled={busy} onclick={() => { draft.variants = [...(draft.variants ?? []), { id: '', name: '', template: '', ops: [] }]; }}>添加变体</PixelButton></div>
      {#if !draft.variants?.length}
        <p class="empty-hint">暂无变体规则。可添加变体以基于不同规则过滤输出差异化配置（例如 Non-IP 纯域名分流）。</p>
      {/if}
      {#each draft.variants ?? [] as variant, i}
        <fieldset disabled={busy}>
          <div class="item-head">
            <legend>变体 {i + 1}</legend>
            <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => { draft.variants = draft.variants?.filter((_, j) => i !== j); }}>删除变体</PixelButton>
          </div>
          <div class="fields"><label>变体 ID<PixelInput bind:value={variant.id} placeholder="输出变体 ID" /></label><label>变体名称<PixelInput bind:value={variant.name} placeholder="显示名称" /></label></div>
          <PixelSelect id="variant-template-{i}" label="变体 {i + 1} 模板" options={variantOptions} value={variant.template ?? ''} disabled={busy} onchange={v => { variant.template = v; }} />
          <h3>过滤链</h3><OpsEditor bind:value={variant.ops} disabled={busy} />
        </fieldset>
      {/each}
    </section>
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestClose()) open = false; }}>关闭</PixelButton>
    <PixelButton variant="primary" disabled={busy || (!dirty && !!editing)} onclick={save}>{busy ? '保存中…' : '保存客户端'}</PixelButton>
  {/snippet}
</PixelDrawer>
<PixelDialog bind:open={discard} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger oncancel={() => { pendingTab = ''; }} onconfirm={() => {
  open = false;
  if (pendingTab) { tab = pendingTab; pendingTab = ''; templateDirty = false; if (tab === 'clients') void load(); }
}}>当前修改尚未保存，关闭后将丢弃这些修改。</PixelDialog>
<PixelDialog bind:open={deleteOpen} title="删除客户端？" confirmLabel="删除" danger onconfirm={remove}>删除 {deleting?.name || deleting?.id} 的客户端配置及其格式、变体。</PixelDialog>
<PixelIconPicker
  bind:open={iconPickerOpen}
  items={iconPickerItems}
  value={clientIconID(draft.icon)}
  loading={iconPickerLoading}
  onconfirm={applyIcon}
  onclear={clearIcon}
/>
<PixelDialog
  bind:open={refConflictOpen}
  title="客户端引用详情"
  confirmLabel="知道了"
  showCancel={false}
  onconfirm={() => { refConflictOpen = false; }}
>
  <div class="ref-conflict-content">
    <p class="ref-conflict-hint">客户端「{refConflictClient?.name || refConflictClient?.id}」正被以下 <strong>{refConflictList.length}</strong> 项配置引用，请先在对应规则中解除输出关联后再尝试删除：</p>
    <div class="ref-conflict-list" use:retroScroll>
      {#each refConflictList as refItem}
        <div class="ref-conflict-item"><code>{refItem}</code></div>
      {/each}
    </div>
  </div>
</PixelDialog>
<style>
  .clients-page, .editor-form { display: grid; gap: 18px; }
  .editor-form { padding-bottom: 36px; }
  .toolbar, .section-head, .client-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .client-identity { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .client-avatar { width: 32px; height: 32px; object-fit: contain; image-rendering: pixelated; flex-shrink: 0; }
  .client-meta { display: grid; gap: 2px; min-width: 0; }
  .client-meta h2 { margin: 0; }
  .section-head { padding: 0 16px; }
  .toolbar { color: var(--sec); }
  .actions { display: flex; gap: 8px; }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  :global(.client-card) { display: flex; flex-direction: column; }
  :global(.client-card > .pixel-card-body) { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .card-footer { margin-top: auto; padding-top: 14px; border-top: 1px solid var(--border); }
  .card-footer .actions { margin: 0; }
  h2 { font: 400 24px/28px var(--font-display); color: var(--display); margin: 0; }
  h3, legend { font: 400 12px/20px var(--font-ui); color: var(--display); margin: 0; }
  code { font: 12px/20px var(--font-code); color: var(--sec); overflow-wrap: anywhere; }
  .targets { margin: 14px 0 0; border-top: 1px solid var(--border); padding-top: 12px; display: grid; gap: 8px; flex: 1; }
  .target-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
  .target-name { color: var(--sec); font: 12px/20px var(--font-ui); }
  .variant-name { display: flex; align-items: center; gap: 4px; }
  .variant-tag {
    display: inline-block;
    font: 400 10px/14px var(--font-ui);
    padding: 1px 4px;
    border: 1px solid var(--border-vis);
    border-radius: 2px;
    background: var(--surface-3);
    color: var(--dim);
  }
  fieldset { min-width: 0; margin: 12px 0 0; padding: 14px 16px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; display: grid; gap: 12px; }
  .item-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
  .fields { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 20px; }
  label, .icon-field { display: grid; gap: 6px; color: var(--sec); font: 12px/18px var(--font-ui); }
  .full { grid-column: 1 / -1; }
  .icon-row { display: flex; align-items: center; gap: 10px; min-height: 32px; }
  .icon-row img { width: 32px; height: 32px; object-fit: contain; image-rendering: pixelated; }
  .icon-unset { color: var(--dim); }
  .icon-action { margin-left: auto; }
  .notice { padding: 10px 14px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; overflow-wrap: anywhere; }
  .notice.error { background: var(--status-error); }
  .notice-inner { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .notice-msg { flex: 1; min-width: 0; }
  .format-single { margin: 12px 16px 0; display: grid; gap: 8px; }
  .field-hint, .empty-hint { margin: 0; font: 12px/18px var(--font-ui); color: var(--sec); }
  .empty-hint { margin-top: 12px; padding: 12px; background: var(--surface-2); border: 1px dashed var(--border); border-radius: 4px; }
  .ref-conflict-content { display: grid; gap: 12px; }
  .ref-conflict-hint { margin: 0; font: 400 13px/20px var(--font-reading); color: var(--text); }
  .ref-conflict-list {
    max-height: 240px;
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    padding: 8px 12px;
    display: grid;
    gap: 4px;
  }
  .ref-conflict-item { padding: 3px 0; border-bottom: 1px dashed var(--border); }
  .ref-conflict-item:last-child { border-bottom: none; }
  @media(max-width: 600px) { .fields { grid-template-columns: 1fr; } }
</style>
