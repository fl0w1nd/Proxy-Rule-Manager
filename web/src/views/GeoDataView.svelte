<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, APIRequestError, conflictHint, type ConfigSnapshot, type GeoKind, type GeoProviderItem } from '../api/client';
  import PixelCard from '../components/pixel/PixelCard.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelCheckbox from '../components/pixel/PixelCheckbox.svelte';
  import GeoCatalogDrawer from './GeoCatalogDrawer.svelte';
  import {
    defaultGeoProviders, geoLabel,
    geoPatchValue,
    geoProviderConfigs,
    providerLabel,
    validateGeoProvider,
    type GeoProviderDraft,
  } from './geodata';
  import geositeIcon from '../assets/icons/nav/geosite.svg';
  import geoipIcon from '../assets/icons/nav/geoip.svg';

  let { kind = 'geosite', onStartUpdate, isUpdating = false, onstatechange }: {
    kind?: GeoKind;
    onStartUpdate: (scope: GeoKind) => void;
    isUpdating?: boolean;
    onstatechange?: (dirty: boolean, busy: boolean) => void;
  } = $props();

  let snapshot = $state<ConfigSnapshot>();
  let providers = $state<GeoProviderItem[]>([]);
  let supported = $state<string[]>([]);
  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let error = $state(false);
  let open = $state(false);
  let editing = $state('');
  let draft = $state<GeoProviderDraft>({ name: '', clients: [] });
  let baseline = $state('');
  let discard = $state(false);
  let deleteOpen = $state(false);
  let deleting = $state<GeoProviderItem>();
  let catalogOpen = $state(false);
  let catalogProvider = $state('');

  const configured = $derived(geoProviderConfigs(snapshot?.config, kind));
  const clients = $derived(((snapshot?.config.clients ?? []) as { id: string; name?: string }[]));
  const dirty = $derived(open && JSON.stringify(draft) !== baseline);
  const remaining = $derived(supported.filter((name) => !configured.some((provider) => provider.name === name)));
  const providerOptions = $derived(remaining.map((name) => ({ value: name, label: providerLabel(name) })));

  $effect(() => { onstatechange?.(dirty, busy); });
  onMount(() => { void loadProviders(); });
  onDestroy(() => { onstatechange?.(false, false); });

  export async function loadProviders() {
    loading = true;
    error = false;
    try {
      const [cfg, res] = await Promise.all([api.getConfig(), api.getGeoProviders(kind)]);
      snapshot = cfg;
      providers = res.items || [];
      supported = res.supported?.length ? res.supported : [...defaultGeoProviders[kind]];
    } catch (e: unknown) {
      fail(e);
    } finally {
      loading = false;
    }
  }

  function fail(e: unknown) {
    error = true;
    message = (e as Error).message;
    if (e instanceof APIRequestError && e.details.errors?.length) {
      message += '：' + e.details.errors.map((item) => `${item.path} ${item.message}`).join('；');
    }
    const hint = conflictHint(e, '请关闭编辑器后刷新配置再重试。');
    if (hint) message += '。' + hint;
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

  function statusLabel(res?: string) {
    if (res === 'updated') return '已更新';
    if (res === 'unchanged') return '无变化';
    if (res === 'failed') return '失败';
    return '未检查';
  }

  function clientName(id: string) {
    return clients.find((client) => client.id === id)?.name || id;
  }

  function clientSummary(item: GeoProviderItem) {
    const names = (item.clients ?? []).map(clientName);
    return names.length ? names.join(' · ') : '—';
  }

  function edit(item?: GeoProviderItem) {
    const current = item ? configured.find((provider) => provider.name === item.name) : undefined;
    editing = current?.name ?? '';
    draft = current
      ? { name: current.name, clients: [...current.clients] }
      : { name: remaining[0] ?? '', clients: [] };
    baseline = JSON.stringify(draft);
    message = '';
    open = true;
  }

  function toggleClient(id: string, checked: boolean) {
    draft.clients = checked ? [...draft.clients, id] : draft.clients.filter((item) => item !== id);
  }

  function requestClose() {
    if (busy) return false;
    if (dirty) {
      discard = true;
      return false;
    }
    return true;
  }

  async function save() {
    if (!snapshot || busy) return;
    const validation = validateGeoProvider(draft, configured.map((item) => item.name), editing, clients.map((item) => item.id));
    if (validation) {
      message = validation;
      error = true;
      return;
    }
    const next = editing
      ? configured.map((item) => item.name === editing ? { name: draft.name, clients: [...draft.clients] } : item)
      : [...configured, { name: draft.name, clients: [...draft.clients] }];
    busy = true;
    message = '';
    try {
      const result = await api.patchConfig(snapshot.version, [{ op: kind === 'geoip' ? 'update_geoip' : 'update_geosite', value: geoPatchValue(next) }]);
      snapshot = { version: result.version, config: { ...snapshot.config, [kind]: geoPatchValue(next) ?? undefined } };
      editing = draft.name;
      baseline = JSON.stringify(draft);
      error = false;
      message = result.warnings?.length ? `已保存。${result.warnings.join('；')}` : '提供商已保存';
      const res = await api.getGeoProviders(kind);
      providers = res.items || [];
    } catch (e) {
      fail(e);
    } finally {
      busy = false;
    }
  }

  function askDelete(item: GeoProviderItem) {
    deleting = item;
    deleteOpen = true;
  }

  async function remove() {
    if (!snapshot || !deleting || busy) return;
    busy = true;
    try {
      const next = configured.filter((item) => item.name !== deleting?.name);
      const value = geoPatchValue(next);
      const result = await api.patchConfig(snapshot.version, [{ op: kind === 'geoip' ? 'update_geoip' : 'update_geosite', value }]);
      snapshot = { version: result.version, config: { ...snapshot.config, [kind]: value ?? undefined } };
      error = false;
      message = '提供商已删除';
      const res = await api.getGeoProviders(kind);
      providers = res.items || [];
    } catch (e) {
      fail(e);
    } finally {
      busy = false;
      deleting = undefined;
    }
  }

  function openCatalog(item: GeoProviderItem) {
    catalogProvider = item.name;
    catalogOpen = true;
  }
</script>

<svelte:window onbeforeunload={(event) => { if (dirty || busy) { event.preventDefault(); event.returnValue = ''; } }} />
<div class="geo-data-view">
  <div class="toolbar">
    <span>共 {providers.length} 个提供商</span>
    <div class="actions">
      <PixelButton disabled={loading || busy} onclick={loadProviders}>刷新</PixelButton>
      <PixelButton disabled={loading || busy || dirty || isUpdating || !snapshot} onclick={() => onStartUpdate(kind)}>更新 {geoLabel(kind)}</PixelButton>
      <PixelButton variant="primary" disabled={loading || busy || !snapshot || remaining.length === 0 || clients.length === 0} onclick={() => edit()}>添加提供商</PixelButton>
    </div>
  </div>

  {#if message && !open}
    <div class="notice" class:error role={error ? 'alert' : 'status'}>{message}</div>
  {/if}

  {#if !loading && clients.length === 0}
    <div class="notice">请先在客户端管理中添加客户端，再配置 {geoLabel(kind)} 输出目标。</div>
  {/if}

  {#if !loading && remaining.length === 0 && clients.length > 0}
    <div class="notice">已配置全部可用提供商。</div>
  {/if}

  {#if loading && providers.length === 0}
    <div class="notice">读取 {geoLabel(kind)} 状态…</div>
  {:else if providers.length === 0}
    <div class="notice">没有配置 {geoLabel(kind)} 提供商</div>
  {:else}
    <div class="cards">
      {#each providers as p (p.name)}
        <PixelCard class="provider-card">
          <div class="card-heading">
            <div class="card-identity">
              <h2>{providerLabel(p.name)}</h2>
              {#if providerLabel(p.name) !== p.name}
                <code>{p.name}</code>
              {/if}
            </div>
            <div class="status-cell">
              <PixelBadge status={getStatusType(p.result)}>{statusLabel(p.result)}</PixelBadge>
              {#if p.checked_at}
                <span class="check-time">{formatTime(p.checked_at)}</span>
              {/if}
            </div>
          </div>
          <div class="card-meta">
            <div class="meta-row">
              <span>输出客户端</span>
              <span>{clientSummary(p)}</span>
            </div>
            <div class="meta-row">
              <span>数据版本</span>
              <code>{p.version || '—'}</code>
            </div>
          </div>
          <div class="stats" class:ip-stats={kind === 'geoip'}>
            <div class="stat"><span>列表</span><strong>{p.lists.toLocaleString()}</strong></div>
            {#if kind === 'geosite'}<div class="stat"><span>变体</span><strong>{p.variants.toLocaleString()}</strong></div>{/if}
            <div class="stat"><span>条目</span><strong>{p.entries.toLocaleString()}</strong></div>
            <div class="stat"><span>文件</span><strong>{p.files.toLocaleString()}</strong></div>
          </div>
          <div class="card-footer">
            <div class="actions">
              <PixelButton size="sm" disabled={busy} onclick={() => openCatalog(p)}>目录</PixelButton>
              <PixelButton size="sm" disabled={busy} onclick={() => edit(p)}>编辑</PixelButton>
              <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => askDelete(p)}>删除</PixelButton>
            </div>
          </div>
        </PixelCard>
      {/each}
    </div>
  {/if}
</div>

<PixelDrawer bind:open title={editing ? `编辑 ${providerLabel(editing)}` : '添加提供商'} icon={kind === 'geoip' ? geoipIcon : geositeIcon} width="520px" onrequestclose={requestClose}>
  <div class="editor-form">
    {#if message && open}
      <div class="notice" class:error role={error ? 'alert' : 'status'}>{message}</div>
    {/if}
    <fieldset disabled={busy}>
      <legend>提供商</legend>
      {#if editing}
        <p class="field-hint">{providerLabel(editing)}</p>
      {:else}
        <PixelSelect id={`${kind}-provider`} label="提供商" options={[{ value: '', label: '请选择提供商' }, ...providerOptions]} bind:value={draft.name} disabled={busy} />
      {/if}
    </fieldset>
    <fieldset disabled={busy}>
      <legend>输出客户端</legend>
      {#if clients.length === 0}
        <p class="field-hint">没有可选择的客户端</p>
      {:else}
        <div class="client-picks">
          {#each clients as client (client.id)}
            <PixelCheckbox checked={draft.clients.includes(client.id)} disabled={busy} onchange={(checked) => toggleClient(client.id, checked)}>
              {client.name || client.id}
            </PixelCheckbox>
          {/each}
        </div>
      {/if}
    </fieldset>
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestClose()) open = false; }}>关闭</PixelButton>
    <PixelButton variant="primary" disabled={busy || (!dirty && !!editing)} onclick={save}>{busy ? '保存中…' : '保存'}</PixelButton>
  {/snippet}
</PixelDrawer>

<GeoCatalogDrawer {kind} bind:open={catalogOpen} provider={catalogProvider} />

<PixelDialog bind:open={discard} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger onconfirm={() => { open = false; }}>
  当前修改尚未保存，关闭后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={deleteOpen} title="删除提供商？" confirmLabel="确认删除" danger onconfirm={remove}>
  删除 {providerLabel(deleting?.name || '')} 后将停止自动发布其全部列表。
</PixelDialog>

<style>
  .geo-data-view { display: grid; gap: 18px; }
  .toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; color: var(--sec); }
  .actions, .client-picks { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .notice { padding: 10px 14px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; overflow-wrap: anywhere; }
  .notice.error { background: var(--status-error); }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  :global(.provider-card) { display: flex; flex-direction: column; }
  :global(.provider-card > .pixel-card-body) { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .card-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
  .card-identity { display: grid; gap: 2px; min-width: 0; }
  h2 { margin: 0; font: 400 24px/28px var(--font-display); color: var(--display); }
  code { font: 12px/20px var(--font-code); color: var(--sec); overflow-wrap: anywhere; }
  .status-cell { display: grid; gap: 2px; justify-items: end; }
  .check-time { color: var(--dim); font-variant-numeric: tabular-nums; }
  .card-meta { margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--border); display: grid; gap: 8px; }
  .meta-row { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; font: 12px/20px var(--font-ui); color: var(--sec); }
  .meta-row span:last-child { color: var(--text); text-align: right; overflow-wrap: anywhere; }
  .stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; margin-top: 14px; }
  .stats.ip-stats { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .stat { display: grid; gap: 2px; }
  .stat span { color: var(--sec); font: 12px/18px var(--font-ui); }
  .stat strong { font: 400 20px/24px var(--font-display); color: var(--display); font-variant-numeric: tabular-nums; }
  .card-footer { margin-top: auto; padding-top: 14px; border-top: 1px solid var(--border); }
  .card-footer .actions { margin: 0; }
  .editor-form { display: grid; gap: 16px; }
  fieldset { min-width: 0; margin: 0; padding: 14px 16px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; display: grid; gap: 12px; }
  legend { font: 400 12px/20px var(--font-ui); color: var(--display); }
  .field-hint { margin: 0; font: 12px/18px var(--font-ui); color: var(--sec); }
  @media (max-width: 520px) { .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
