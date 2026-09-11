<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    api, APIRequestError, conflictHint,
    type ConfigSnapshot, type GeoKind, type GeoProviderItem, type HostedGeoKind, type UpdateScope,
  } from '../api/client';
  import PixelCard from '../components/pixel/PixelCard.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelCheckbox from '../components/pixel/PixelCheckbox.svelte';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import GeoCatalogDrawer from './GeoCatalogDrawer.svelte';
  import {
    defaultGeoProviders, defaultHostedGeoProviders,
    geoPatchValue,
    geoProviderConfigs,
    geoTabs, geoTabLabel,
    hostedGeoLabel,
    hostedGeoPatchValue,
    hostedGeoProviderConfigs,
    providerLabel,
    validateGeoProvider,
    validateHostedGeoProvider,
    type GeoProviderDraft,
    type HostedGeoProviderDraft,
    type GeoTab,
  } from './geodata';
  import geositeIcon from '../assets/icons/nav/geosite.svg';

  let {
    onStartUpdate,
    isUpdating = false,
    onstatechange,
  }: {
    onStartUpdate: (scope: UpdateScope) => void;
    isUpdating?: boolean;
    onstatechange?: (dirty: boolean, busy: boolean) => void;
  } = $props();

  let activeTab = $state<GeoTab>('geosite');
  let snapshot = $state<ConfigSnapshot>();
  let providers = $state<Record<GeoKind, GeoProviderItem[]>>({ geosite: [], geoip: [] });
  let supported = $state<Record<GeoKind, string[]>>({
    geosite: [...defaultGeoProviders.geosite],
    geoip: [...defaultGeoProviders.geoip],
  });
  let hosted = $state<Record<HostedGeoKind, GeoProviderItem[]>>({ mmdb: [], asn: [] });
  let hostedSupported = $state<Record<HostedGeoKind, string[]>>({
    mmdb: [...defaultHostedGeoProviders.mmdb],
    asn: [...defaultHostedGeoProviders.asn],
  });

  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let error = $state(false);

  // GeoSite / GeoIP drawer state
  let open = $state(false);
  let editing = $state('');
  let draft = $state<GeoProviderDraft>({ name: '', clients: [] });
  let baseline = $state('');
  let discard = $state(false);
  let deleteOpen = $state(false);
  let deleting = $state<GeoProviderItem>();
  let catalogOpen = $state(false);
  let catalogProvider = $state('');

  // MMDB / ASN drawer state
  let hostedOpen = $state(false);
  let hostedEditing = $state('');
  let hostedDraft = $state<HostedGeoProviderDraft>({ name: '', host: true });
  let hostedBaseline = $state('');
  let hostedDiscard = $state(false);
  let hostedDeleting = $state<{ kind: HostedGeoKind; name: string }>();

  const isHostedTab = $derived(activeTab === 'mmdb' || activeTab === 'asn');
  const clients = $derived(((snapshot?.config.clients ?? []) as { id: string; name?: string }[]));
  const dirty = $derived(
    (open && JSON.stringify(draft) !== baseline) ||
    (hostedOpen && JSON.stringify(hostedDraft) !== hostedBaseline)
  );

  // Standard providers (geosite / geoip)
  const currentStandardKind = $derived(activeTab === 'geoip' ? 'geoip' : 'geosite');
  const configured = $derived(geoProviderConfigs(snapshot?.config, currentStandardKind));
  const remaining = $derived(
    supported[currentStandardKind].filter((name) => !configured.some((provider) => provider.name === name))
  );
  const providerOptions = $derived(remaining.map((name) => ({ value: name, label: providerLabel(name) })));

  // Hosted providers (mmdb / asn)
  const currentHostedKind = $derived(activeTab === 'asn' ? 'asn' : 'mmdb');
  const hostedConfigured = $derived({
    mmdb: hostedGeoProviderConfigs(snapshot?.config, 'mmdb'),
    asn: hostedGeoProviderConfigs(snapshot?.config, 'asn'),
  } satisfies Record<HostedGeoKind, HostedGeoProviderDraft[]>);
  const hostedRemaining = $derived({
    mmdb: hostedSupported.mmdb.filter((name) => !hostedConfigured.mmdb.some((provider) => provider.name === name)),
    asn: hostedSupported.asn.filter((name) => !hostedConfigured.asn.some((provider) => provider.name === name)),
  } satisfies Record<HostedGeoKind, string[]>);
  const hostedOptions = $derived(
    hostedRemaining[currentHostedKind].map((name) => ({ value: name, label: providerLabel(name) }))
  );

  $effect(() => { onstatechange?.(dirty, busy); });
  onMount(() => { void loadProviders(); });
  onDestroy(() => { onstatechange?.(false, false); });

  export async function loadProviders() {
    loading = true;
    error = false;
    try {
      const [cfg, geositeRes, geoipRes, mmdbRes, asnRes] = await Promise.all([
        api.getConfig(),
        api.getGeoProviders('geosite'),
        api.getGeoProviders('geoip'),
        api.getGeoProviders('mmdb'),
        api.getGeoProviders('asn'),
      ]);
      snapshot = cfg;
      providers = {
        geosite: geositeRes.items || [],
        geoip: geoipRes.items || [],
      };
      supported = {
        geosite: geositeRes.supported?.length ? geositeRes.supported : [...defaultGeoProviders.geosite],
        geoip: geoipRes.supported?.length ? geoipRes.supported : [...defaultGeoProviders.geoip],
      };
      hosted = {
        mmdb: mmdbRes.items || [],
        asn: asnRes.items || [],
      };
      hostedSupported = {
        mmdb: mmdbRes.supported?.length ? mmdbRes.supported : [...defaultHostedGeoProviders.mmdb],
        asn: asnRes.supported?.length ? asnRes.supported : [...defaultHostedGeoProviders.asn],
      };
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

  function providerHost(name: string) {
    return configured.find((provider) => provider.name === name)?.host === true;
  }

  function edit(item?: GeoProviderItem) {
    const current = item ? configured.find((provider) => provider.name === item.name) : undefined;
    editing = current?.name ?? '';
    draft = current
      ? { name: current.name, clients: [...current.clients], host: current.host === true }
      : { name: remaining[0] ?? '', clients: [], host: false };
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
      ? configured.map((item) => item.name === editing ? { name: draft.name, clients: [...draft.clients], host: draft.host } : item)
      : [...configured, { name: draft.name, clients: [...draft.clients], host: draft.host }];
    busy = true;
    message = '';
    try {
      const op = currentStandardKind === 'geoip' ? 'update_geoip' : 'update_geosite';
      const value = geoPatchValue(next);
      const result = await api.patchConfig(snapshot.version, [{ op, value }]);
      snapshot = { version: result.version, config: { ...snapshot.config, [currentStandardKind]: value ?? undefined } };
      editing = draft.name;
      baseline = JSON.stringify(draft);
      error = false;
      message = result.warnings?.length ? `已保存。${result.warnings.join('；')}` : '提供商已保存';
      const res = await api.getGeoProviders(currentStandardKind);
      providers = { ...providers, [currentStandardKind]: res.items || [] };
    } catch (e) {
      fail(e);
    } finally {
      busy = false;
    }
  }

  function askDelete(item: GeoProviderItem) {
    hostedDeleting = undefined;
    deleting = item;
    deleteOpen = true;
  }

  async function remove() {
    if (!snapshot || !deleting || busy) return;
    busy = true;
    try {
      const next = configured.filter((item) => item.name !== deleting?.name);
      const value = geoPatchValue(next);
      const op = currentStandardKind === 'geoip' ? 'update_geoip' : 'update_geosite';
      const result = await api.patchConfig(snapshot.version, [{ op, value }]);
      snapshot = { version: result.version, config: { ...snapshot.config, [currentStandardKind]: value ?? undefined } };
      error = false;
      message = '提供商已删除';
      const res = await api.getGeoProviders(currentStandardKind);
      providers = { ...providers, [currentStandardKind]: res.items || [] };
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

  function addHosted(nextKind: HostedGeoKind) {
    hostedEditing = '';
    hostedDraft = { name: hostedRemaining[nextKind][0] ?? '', host: true };
    hostedBaseline = JSON.stringify(hostedDraft);
    message = '';
    hostedOpen = true;
  }

  function editHosted(item: HostedGeoProviderDraft) {
    hostedEditing = item.name;
    hostedDraft = { name: item.name, host: item.host === true };
    hostedBaseline = JSON.stringify(hostedDraft);
    message = '';
    hostedOpen = true;
  }

  function requestHostedClose() {
    if (busy) return false;
    if (hostedOpen && JSON.stringify(hostedDraft) !== hostedBaseline) {
      hostedDiscard = true;
      return false;
    }
    return true;
  }

  async function saveHosted() {
    if (!snapshot || busy) return;
    const kind = currentHostedKind;
    const validation = validateHostedGeoProvider(
      hostedDraft.name,
      hostedConfigured[kind].map((item) => item.name),
      hostedEditing,
    );
    if (validation) {
      message = validation;
      error = true;
      return;
    }
    const next = hostedEditing
      ? hostedConfigured[kind].map((item) => (item.name === hostedEditing ? { ...hostedDraft } : item))
      : [...hostedConfigured[kind], { ...hostedDraft }];
    const ok = await patchHosted(kind, next, `${hostedGeoLabel(kind)} 提供商已保存`);
    if (ok) {
      hostedEditing = hostedDraft.name;
      hostedBaseline = JSON.stringify(hostedDraft);
      hostedOpen = false;
    }
  }

  function askDeleteHosted(nextKind: HostedGeoKind, name: string) {
    deleting = undefined;
    hostedDeleting = { kind: nextKind, name };
    deleteOpen = true;
  }

  async function removeHosted() {
    if (!hostedDeleting) return;
    const { kind: nextKind, name } = hostedDeleting;
    await patchHosted(nextKind, hostedConfigured[nextKind].filter((item) => item.name !== name), `${hostedGeoLabel(nextKind)} 提供商已删除`);
    hostedDeleting = undefined;
  }

  async function patchHosted(nextKind: HostedGeoKind, next: HostedGeoProviderDraft[], success: string): Promise<boolean> {
    if (!snapshot) return false;
    busy = true;
    message = '';
    try {
      const value = hostedGeoPatchValue(next);
      const result = await api.patchConfig(snapshot.version, [{ op: nextKind === 'mmdb' ? 'update_mmdb' : 'update_asn', value }]);
      snapshot = { version: result.version, config: { ...snapshot.config, [nextKind]: value ?? undefined } };
      const res = await api.getGeoProviders(nextKind);
      hosted = { ...hosted, [nextKind]: res.items || [] };
      hostedSupported = { ...hostedSupported, [nextKind]: res.supported?.length ? res.supported : [...defaultHostedGeoProviders[nextKind]] };
      error = false;
      message = result.warnings?.length ? `已保存。${result.warnings.join('；')}` : success;
      return true;
    } catch (e) {
      fail(e);
      return false;
    } finally {
      busy = false;
    }
  }

  async function confirmDelete() {
    if (hostedDeleting) {
      await removeHosted();
      return;
    }
    await remove();
  }
</script>

<svelte:window onbeforeunload={(event) => { if (dirty || busy) { event.preventDefault(); event.returnValue = ''; } }} />
<div class="geo-data-view">
  <div class="header-row">
    <PixelTabs
      id="geodata-subtabs"
      label="Geo 数据分类"
      items={geoTabs.map((t) => ({ value: t.id, label: t.label }))}
      value={activeTab}
      onchange={(value) => { activeTab = value as GeoTab; message = ''; }}
    />
  </div>

  {#if !isHostedTab}
    <!-- Geosite & GeoIP Toolbar -->
    <div class="toolbar">
      <span>共 {providers[currentStandardKind].length} 个提供商</span>
      <div class="actions">
        <PixelButton disabled={loading || busy} onclick={loadProviders}>刷新</PixelButton>
        <PixelButton disabled={loading || busy || dirty || isUpdating || !snapshot} onclick={() => onStartUpdate(currentStandardKind)}>更新 {geoTabLabel(activeTab)}</PixelButton>
        <PixelButton variant="primary" disabled={loading || busy || !snapshot || remaining.length === 0 || clients.length === 0} onclick={() => edit()}>添加提供商</PixelButton>
      </div>
    </div>
  {:else}
    <!-- MMDB & ASN Toolbar -->
    <div class="toolbar">
      <span>共 {hostedConfigured[currentHostedKind].length} 个提供商</span>
      <div class="actions">
        <PixelButton disabled={loading || busy} onclick={loadProviders}>刷新</PixelButton>
        <PixelButton disabled={loading || busy || isUpdating || !snapshot} onclick={() => onStartUpdate(currentHostedKind)}>更新 {geoTabLabel(activeTab)}</PixelButton>
        <PixelButton variant="primary" disabled={loading || busy || !snapshot || hostedRemaining[currentHostedKind].length === 0} onclick={() => addHosted(currentHostedKind)}>添加 {geoTabLabel(activeTab)}</PixelButton>
      </div>
    </div>
  {/if}

  {#if message && !open && !hostedOpen}
    <div class="notice" class:error role={error ? 'alert' : 'status'}>{message}</div>
  {/if}

  {#if !isHostedTab}
    {#if !loading && clients.length === 0}
      <div class="notice">请先在客户端管理中添加客户端，再配置 {geoTabLabel(activeTab)} 输出目标。</div>
    {/if}

    {#if !loading && remaining.length === 0 && clients.length > 0}
      <div class="notice">已配置全部可用提供商。</div>
    {/if}

    {#if loading && providers[currentStandardKind].length === 0}
      <div class="notice">读取 {geoTabLabel(activeTab)} 状态…</div>
    {:else if providers[currentStandardKind].length === 0}
      <div class="notice">没有配置 {geoTabLabel(activeTab)} 提供商</div>
    {:else}
      <div class="cards">
        {#each providers[currentStandardKind] as p (p.name)}
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
              {#if providerHost(p.name)}
                <div class="meta-row">
                  <span>公开原始数据库</span>
                  <span>已启用</span>
                </div>
              {/if}
            </div>
            <div class="stats" class:ip-stats={currentStandardKind === 'geoip'}>
              <div class="stat"><span>列表</span><strong>{p.lists.toLocaleString()}</strong></div>
              {#if currentStandardKind === 'geosite'}<div class="stat"><span>变体</span><strong>{p.variants.toLocaleString()}</strong></div>{/if}
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
  {:else}
    <!-- MMDB / ASN Cards -->
    {#if loading && hostedConfigured[currentHostedKind].length === 0}
      <div class="notice">读取 {geoTabLabel(activeTab)} 状态…</div>
    {:else if hostedConfigured[currentHostedKind].length === 0}
      <div class="notice">没有配置 {geoTabLabel(activeTab)} 提供商</div>
    {:else}
      <div class="cards">
        {#each hostedConfigured[currentHostedKind] as item (item.name)}
          {@const status = hosted[currentHostedKind].find((provider) => provider.name === item.name)}
          <PixelCard class="provider-card">
            <div class="card-heading">
              <div class="card-identity">
                <h2>{providerLabel(item.name)}</h2>
                {#if providerLabel(item.name) !== item.name}
                  <code>{item.name}</code>
                {/if}
              </div>
              <div class="status-cell">
                <PixelBadge status={getStatusType(status?.result)}>{statusLabel(status?.result)}</PixelBadge>
                {#if status?.checked_at}
                  <span class="check-time">{formatTime(status.checked_at)}</span>
                {/if}
              </div>
            </div>
            <div class="card-meta">
              <div class="meta-row">
                <span>数据版本</span>
                <code>{status?.version || '—'}</code>
              </div>
              <div class="meta-row">
                <span>公开原始数据库</span>
                <span>{item.host === true ? '已启用' : '未启用'}</span>
              </div>
            </div>
            <div class="card-footer">
              <div class="actions">
                <PixelButton size="sm" disabled={busy} onclick={() => editHosted(item)}>编辑</PixelButton>
                <PixelButton size="sm" variant="danger" disabled={busy} onclick={() => askDeleteHosted(currentHostedKind, item.name)}>删除</PixelButton>
              </div>
            </div>
          </PixelCard>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<PixelDrawer bind:open title={editing ? `编辑 ${providerLabel(editing)}` : '添加提供商'} icon={geositeIcon} width="520px" onrequestclose={requestClose}>
  <div class="editor-form">
    {#if message && open}
      <div class="notice" class:error role={error ? 'alert' : 'status'}>{message}</div>
    {/if}
    <fieldset disabled={busy}>
      <legend>提供商</legend>
      {#if editing}
        <p class="field-hint">{providerLabel(editing)}</p>
      {:else}
        <PixelSelect id={`${currentStandardKind}-provider`} label="提供商" options={[{ value: '', label: '请选择提供商' }, ...providerOptions]} bind:value={draft.name} disabled={busy} />
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
    <fieldset disabled={busy}>
      <legend>公开页</legend>
      <PixelCheckbox checked={draft.host === true} disabled={busy} onchange={(checked) => { draft.host = checked; }}>
        公开原始数据库
      </PixelCheckbox>
    </fieldset>
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestClose()) open = false; }}>关闭</PixelButton>
    <PixelButton variant="primary" disabled={busy || (!dirty && !!editing)} onclick={save}>{busy ? '保存中…' : '保存'}</PixelButton>
  {/snippet}
</PixelDrawer>

<PixelDrawer bind:open={hostedOpen} title={hostedEditing ? `编辑 ${providerLabel(hostedEditing)}` : `添加 ${geoTabLabel(activeTab)}`} icon={geositeIcon} width="520px" onrequestclose={requestHostedClose}>
  <div class="editor-form">
    {#if message && hostedOpen}
      <div class="notice" class:error role={error ? 'alert' : 'status'}>{message}</div>
    {/if}
    <fieldset disabled={busy}>
      <legend>提供商</legend>
      {#if hostedEditing}
        <p class="field-hint">{providerLabel(hostedEditing)}</p>
      {:else}
        <PixelSelect id={`${currentHostedKind}-provider`} label="提供商" options={[{ value: '', label: '请选择提供商' }, ...hostedOptions]} bind:value={hostedDraft.name} disabled={busy} />
      {/if}
    </fieldset>
    <fieldset disabled={busy}>
      <legend>公开页</legend>
      <PixelCheckbox checked={hostedDraft.host === true} disabled={busy} onchange={(checked) => { hostedDraft.host = checked; }}>
        公开原始数据库
      </PixelCheckbox>
      <p class="field-hint">启用后，将在公开页提供该原始数据库下载。</p>
    </fieldset>
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestHostedClose()) hostedOpen = false; }}>关闭</PixelButton>
    <PixelButton variant="primary" disabled={busy || (!dirty && !!hostedEditing)} onclick={saveHosted}>{busy ? '保存中…' : '保存'}</PixelButton>
  {/snippet}
</PixelDrawer>

<GeoCatalogDrawer kind={currentStandardKind} bind:open={catalogOpen} provider={catalogProvider} />

<PixelDialog bind:open={discard} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger onconfirm={() => { open = false; }}>
  当前修改尚未保存，关闭后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={hostedDiscard} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger onconfirm={() => { hostedOpen = false; }}>
  当前修改尚未保存，关闭后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={deleteOpen} title="删除提供商？" confirmLabel="确认删除" danger onconfirm={confirmDelete}>
  {#if hostedDeleting}
    删除 {providerLabel(hostedDeleting.name)} 后，公开页将不再提供该 {hostedGeoLabel(hostedDeleting.kind)} 数据库。
  {:else}
    删除 {providerLabel(deleting?.name || '')} 后将停止自动发布其全部列表。
  {/if}
</PixelDialog>

<style>
  .geo-data-view { display: grid; gap: 18px; }
  .header-row { display: flex; align-items: center; justify-content: space-between; }
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
