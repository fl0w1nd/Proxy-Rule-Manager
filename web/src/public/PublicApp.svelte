<script lang="ts">
  import { onMount } from 'svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import ClientPicker from './components/ClientPicker.svelte';
  import FilePreviewModal from './components/FilePreviewModal.svelte';
  import GeositeCatalog from './components/GeositeCatalog.svelte';
  import IconGallery from './components/IconGallery.svelte';
  import RuleCatalog from './components/RuleCatalog.svelte';
  import type { PreviewItem, PublicPageData, PublicRule, PublicView } from './types';
  import { clientUsable, formatUpdatedAt, geositePath, selectedOption } from './utils';

  interface Props { data: PublicPageData; }
  let { data }: Props = $props();

  let view = $state<PublicView>('rules');
  let selectedClient = $state(0);
  let selectedTargets = $state<Record<string, string>>({});
  let theme = $state<'dark' | 'light'>('dark');
  let previewItem = $state<PreviewItem | null>(null);

  const activeClient = $derived(data.clients[selectedClient]);
  const activeTarget = $derived(activeClient ? selectedOption(activeClient, selectedTargets[activeClient.id], view) : undefined);

  onMount(() => {
    try {
      const savedTheme = localStorage.getItem('prm-theme');
      if (savedTheme === 'dark' || savedTheme === 'light') theme = savedTheme;
      const savedClient = Number.parseInt(localStorage.getItem('prm-client') || '', 10);
      if (Number.isInteger(savedClient) && savedClient >= 0 && savedClient < data.clients.length) selectedClient = savedClient;
      const restored: Record<string, string> = {};
      for (const client of data.clients) {
        const saved = localStorage.getItem(`prm-target-${client.id}`);
        if (saved) restored[client.id] = saved;
      }
      selectedTargets = restored;
      normalizeSelection(view);
    } catch {
      normalizeSelection(view);
    }
  });

  function normalizeSelection(nextView: PublicView) {
    if (nextView === 'icons' || data.clients.length === 0) return;
    let index = selectedClient;
    if (!clientUsable(data.clients[index], nextView)) {
      const fallback = data.clients.findIndex((client) => clientUsable(client, nextView));
      if (fallback >= 0) index = fallback;
    }
    selectedClient = index;
    const client = data.clients[index];
    const target = selectedOption(client, selectedTargets[client.id], nextView);
    selectedTargets = { ...selectedTargets, [client.id]: target.id };
    try {
      localStorage.setItem('prm-client', String(index));
      localStorage.setItem(`prm-target-${client.id}`, target.id);
    } catch {}
  }

  function setView(nextView: PublicView) {
    view = nextView;
    previewItem = null;
    normalizeSelection(nextView);
  }

  function selectClient(index: number) {
    selectedClient = index;
    normalizeSelection(view);
  }

  function selectTarget(clientIndex: number, targetID: string) {
    const client = data.clients[clientIndex];
    const target = client.options.find((option) => option.id === targetID);
    if (!target) return;
    selectedClient = clientIndex;
    selectedTargets = { ...selectedTargets, [client.id]: targetID };
    try {
      localStorage.setItem('prm-client', String(clientIndex));
      localStorage.setItem(`prm-target-${client.id}`, targetID);
    } catch {}
    if (previewItem?.source.kind === 'rule') {
      const source = previewItem.source;
      const rule = data.rules.find((item) => item.id === source.rule_id);
      if (rule) previewItem = rulePreview(rule, target);
    } else if (previewItem?.source.kind === 'geosite') {
      const source = previewItem.source;
      previewItem = {
        ...previewItem,
        key: `geosite:${source.provider}/${source.name}@${source.attr || ''}:${target.id}`,
        path: geositePath(target, source.provider, source.name, source.attr),
      };
    }
  }

  function rulePreview(rule: PublicRule, target: NonNullable<typeof activeTarget>): PreviewItem {
    const file = rule.files.find((candidate) => candidate.target_id === target.id);
    return {
      key: `rule:${rule.id}:${target.id}`,
      title: rule.name,
      tags: rule.tags,
      description: rule.description,
      path: file?.path,
      size: file?.size,
      entries: rule.entries,
      source: { kind: 'rule', rule_id: rule.id },
    };
  }

  function openRule(rule: PublicRule) {
    if (activeTarget) previewItem = rulePreview(rule, activeTarget);
  }

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.classList.add('theme-switching');
    document.documentElement.setAttribute('data-theme', theme);
    try { localStorage.setItem('prm-theme', theme); } catch {}
    requestAnimationFrame(() => requestAnimationFrame(() => document.documentElement.classList.remove('theme-switching')));
  }
</script>

<div class="public-app">
  <header class="public-topbar">
    <div class="public-brand">
      <img src="static/icons/prm.svg" width="40" height="40" alt="PRM" />
      <div><strong>PROXY RULE MANAGER</strong><small>规则索引 · 更新于 {formatUpdatedAt(data.updated_at)}</small></div>
    </div>
    <div class="top-actions">
      {#if data.admin_url}<a class="bracket-btn" href={data.admin_url}>［ 管理 ］</a>{/if}
      <button class="bracket-btn" type="button" onclick={toggleTheme}>［ {theme === 'dark' ? '亮色' : '暗色'} ］</button>
    </div>
  </header>

  <nav class="view-switch" aria-label="内容视图">
    <button class:on={view === 'rules'} type="button" onclick={() => setView('rules')}>规则</button>
    <button class:on={view === 'geosite'} type="button" onclick={() => setView('geosite')}>Geosite</button>
    <button class:on={view === 'icons'} type="button" onclick={() => setView('icons')}>图标</button>
  </nav>

  {#if view !== 'icons' && activeClient && activeTarget}
    <ClientPicker
      clients={data.clients}
      {view}
      selectedIndex={selectedClient}
      {selectedTargets}
      onselectclient={selectClient}
      onselecttarget={selectTarget}
    />
  {/if}

  {#if view === 'rules' && activeClient && activeTarget}
    <RuleCatalog rules={data.rules} tags={data.tags} target={activeTarget} onpreview={openRule} />
  {:else if view === 'geosite' && activeClient && activeTarget}
    <GeositeCatalog catalogs={data.geosite} client={activeClient} target={activeTarget} onpreview={(item) => { previewItem = item; }} />
  {:else if view === 'icons'}
    <IconGallery sets={data.icon_sets} />
  {/if}
</div>

{#if previewItem && activeClient && activeTarget}
  <FilePreviewModal item={previewItem} client={activeClient} target={activeTarget} onclose={() => { previewItem = null; }} />
{/if}
