<script lang="ts">
  import { untrack } from 'svelte';
  import { api, APIRequestError, type GeoKind, type GeoEntry, type GeoListOverview } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelSkeleton from '../components/pixel/PixelSkeleton.svelte';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import { retroScroll } from '../utils/scrollbars';
  import { entryTypeLabel, geoLabel } from './geodata';
  import geositeIcon from '../assets/icons/nav/geosite.svg';

  let {
    open = $bindable(false),
    provider = '',
    kind = 'geosite',
  }: {
    open: boolean;
    provider: string;
    kind?: GeoKind;
  } = $props();

  let catalog = $state<GeoListOverview[]>([]);
  let contentLists = $state<GeoListOverview[] | null>(null);
  let query = $state('');
  let match = $state<'name' | 'content'>('name');
  let loading = $state(false);
  let searching = $state(false);
  let error = $state('');
  let previewKey = $state('');
  let previewList = $state('');
  let previewAttr = $state('');
  let previewTitle = $state('');
  let previewItems = $state<GeoEntry[]>([]);
  let previewTotal = $state(0);
  let previewLoading = $state(false);
  let previewError = $state('');
  let catalogRequest = 0;
  let searchRequest = 0;
  let previewRequest = 0;
  const pageSize = 100;

  const displayed = $derived.by(() => {
    if (match === 'content' && query.trim() && contentLists) return contentLists;
    const term = query.trim().toLowerCase();
    if (!term || match === 'content') return catalog;
    return catalog.filter((list) => list.name.includes(term) || (list.variants ?? []).some((variant) => variant.attr.includes(term)));
  });

  $effect(() => {
    if (!open || !provider) return;
    const name = provider;
    untrack(() => { void loadCatalog(name); });
    return () => { catalogRequest += 1; searchRequest += 1; };
  });

  $effect(() => {
    if (!open || !provider || match !== 'content') return;
    const term = query.trim();
    const currentProvider = provider;
    if (!term) {
      contentLists = null;
      searching = false;
      return;
    }
    const timer = setTimeout(() => { void searchContent(currentProvider, term); }, 280);
    return () => { clearTimeout(timer); searchRequest += 1; };
  });

  async function loadCatalog(name: string) {
    const requestId = ++catalogRequest;
    loading = true;
    error = '';
    catalog = [];
    contentLists = null;
    query = '';
    match = 'name';
    clearPreview();
    try {
      const result = await api.getGeoCatalog(name, '', 'name', kind);
      if (requestId !== catalogRequest) return;
      catalog = result.lists ?? [];
    } catch (e) {
      if (requestId !== catalogRequest) return;
      error = catalogError(e);
    } finally {
      if (requestId === catalogRequest) loading = false;
    }
  }

  async function searchContent(name: string, term: string) {
    const requestId = ++searchRequest;
    searching = true;
    error = '';
    try {
      const result = await api.getGeoCatalog(name, term, 'content', kind);
      if (requestId !== searchRequest) return;
      contentLists = result.lists ?? [];
    } catch (e) {
      if (requestId !== searchRequest) return;
      error = catalogError(e);
      contentLists = [];
    } finally {
      if (requestId === searchRequest) searching = false;
    }
  }

  function catalogError(error: unknown): string {
    if (error instanceof APIRequestError && error.code === `${kind}_cache_missing`) {
      return '尚未下载该提供商数据，请先执行更新';
    }
    return (error as Error).message;
  }

  function isActive(list: string, attr?: string) {
    return attr ? previewKey === `${list}@${attr}` : previewKey === list;
  }

  function clearPreview() {
    previewRequest += 1;
    previewKey = '';
    previewList = '';
    previewAttr = '';
    previewTitle = '';
    previewItems = [];
    previewTotal = 0;
    previewLoading = false;
    previewError = '';
  }

  async function preview(list: GeoListOverview, attr?: string) {
    if (!provider) return;
    const key = attr ? `${list.name}@${attr}` : list.name;
    if (previewKey === key && previewLoading) return;
    const requestId = ++previewRequest;
    previewKey = key;
    previewList = list.name;
    previewAttr = attr ?? '';
    previewTitle = attr ? `${list.name} @${attr}` : list.name;
    previewItems = [];
    previewTotal = attr ? (list.variants?.find((item) => item.attr === attr)?.entries ?? 0) : list.entries;
    previewLoading = true;
    previewError = '';
    try {
      const result = await api.getGeoList(provider, list.name, { attr, limit: pageSize, offset: 0 }, kind);
      if (requestId !== previewRequest) return;
      previewItems = result.items ?? [];
      previewTotal = result.total;
    } catch (e) {
      if (requestId !== previewRequest) return;
      previewError = (e as Error).message;
    } finally {
      if (requestId === previewRequest) previewLoading = false;
    }
  }

  async function loadMore() {
    if (!provider || !previewKey || previewLoading || previewItems.length >= previewTotal) return;
    const requestId = previewRequest;
    previewLoading = true;
    try {
      const result = await api.getGeoList(provider, previewList, { attr: previewAttr || undefined, limit: pageSize, offset: previewItems.length }, kind);
      if (requestId !== previewRequest) return;
      previewItems = [...previewItems, ...(result.items ?? [])];
      previewTotal = result.total;
    } catch (e) {
      if (requestId !== previewRequest) return;
      previewError = (e as Error).message;
    } finally {
      if (requestId === previewRequest) previewLoading = false;
    }
  }

  function close() {
    open = false;
  }
</script>

<PixelDrawer bind:open title={provider ? `${provider} 目录` : `${geoLabel(kind)} 目录`} icon={geositeIcon} width="880px" scrollable={false} onclose={close}>
  <div class="catalog-shell">
    <div class="catalog-toolbar">
      <PixelTabs id={`${kind}-search-mode`} label="检索方式" items={[{ value: 'name', label: '名称' }, { value: 'content', label: '内容' }]} value={match} onchange={(value) => { match = value as 'name' | 'content'; }} />
      <PixelInput class="catalog-search" type="search" placeholder={match === 'content' ? (kind === 'geoip' ? 'IP 地址或 CIDR…' : '域名或规则内容…') : (kind === 'geoip' ? '搜索列表…' : '搜索列表或变体…')} bind:value={query} spellcheck="false" aria-label="搜索目录" />
      {#if searching}<span class="search-status">检索中…</span>{/if}
    </div>
    {#if error}
      <div class="catalog-error" role="alert">{error}</div>
    {/if}
    <div class="catalog-panes">
      <div class="catalog-lists" use:retroScroll>
        {#if loading}
          <PixelSkeleton rows={10} />
        {:else if displayed.length === 0}
          <div class="catalog-empty">{catalog.length === 0 ? '还没有列表' : '没有匹配的列表'}</div>
        {:else}
          {#each displayed as list (list.name)}
            <div class="geo-item" class:active={isActive(list.name)}>
              <div class="geo-row">
                <button class="geo-main" type="button" aria-current={isActive(list.name) ? 'true' : undefined} onclick={() => preview(list)}>
                  <strong>{list.name}</strong>
                  <span class="geo-count" aria-hidden="true">{list.entries.toLocaleString()} 条</span>
                </button>
                {#if list.variants?.length}
                  <span class="geo-tags">
                    {#each list.variants as variant (variant.attr)}
                      <button
                        class="geo-tag"
                        type="button"
                        class:active={isActive(list.name, variant.attr)}
                        aria-current={isActive(list.name, variant.attr) ? 'true' : undefined}
                        onclick={() => preview(list, variant.attr)}
                      >@{variant.attr}</button>
                    {/each}
                  </span>
                {/if}
              </div>
            </div>
          {/each}
        {/if}
      </div>
      <div class="catalog-preview">
        {#if previewKey}
          <div class="preview-head">
            <strong>{previewTitle}</strong>
            <span>{previewItems.length.toLocaleString()}/{previewTotal.toLocaleString()}</span>
          </div>
        {/if}
        <div class="preview-body" use:retroScroll aria-busy={previewLoading}>
          {#if !previewKey}
            <div class="catalog-empty">选择列表预览规则</div>
          {:else if previewError}
            <div class="catalog-error" role="alert">{previewError}</div>
          {:else if previewItems.length === 0 && previewLoading}
            <PixelSkeleton variant="preview" rows={8} />
          {:else}
            <ul class="entry-list">
              {#each previewItems as entry, index (index)}
                <li>
                  <span class="entry-type">{entryTypeLabel(entry.type)}</span>
                  <code>{entry.value}</code>
                  {#if entry.attrs?.length}
                    <span class="entry-attrs">{entry.attrs.map((attr) => `@${attr}`).join(' ')}</span>
                  {/if}
                </li>
              {/each}
            </ul>
            {#if previewItems.length < previewTotal}
              <div class="preview-more">
                <PixelButton size="sm" disabled={previewLoading} onclick={loadMore}>{previewLoading ? '加载中…' : '加载更多'}</PixelButton>
              </div>
            {/if}
          {/if}
        </div>
      </div>
    </div>
  </div>
</PixelDrawer>

<style>
  .catalog-shell { display: flex; flex-direction: column; gap: 12px; min-height: 0; height: 100%; }
  .catalog-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .catalog-toolbar :global(.catalog-search) { flex: 1; min-width: 180px; }
  .search-status { color: var(--sec); font: 12px/20px var(--font-ui); }
  .catalog-panes { display: grid; grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr); gap: 12px; min-height: 0; flex: 1; }
  .catalog-lists, .catalog-preview {
    min-height: 0;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface-2);
  }
  .catalog-lists { overflow: auto; padding: 6px 18px 6px 6px; }
  .catalog-preview { display: flex; flex-direction: column; overflow: hidden; }
  .catalog-empty, .catalog-error { padding: 16px; color: var(--dim); }
  .catalog-error { color: var(--text); background: var(--status-error); border: 1px solid var(--border-vis); border-radius: 4px; margin: 8px; }
  .geo-item { border-radius: 3px; }
  .geo-item.active > .geo-row { background: var(--selected); color: var(--selected-text); }
  .geo-item.active > .geo-row .geo-main strong,
  .geo-item.active > .geo-row .geo-count { color: var(--selected-text); }
  .geo-row {
    display: flex;
    align-items: stretch;
    gap: 6px;
    min-width: 0;
    border-radius: 3px;
  }
  .geo-row:hover { background: var(--surface); }
  .geo-item.active > .geo-row:hover { background: var(--selected); }
  .geo-main {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    flex: 1 1 0;
    width: 0;
    min-width: 0;
    padding: 6px 8px;
    border: 0;
    background: none;
    color: inherit;
    font: 400 12px/20px var(--font-ui);
    text-align: left;
    cursor: pointer;
  }
  .geo-main strong {
    font: 400 13px/20px var(--font-code);
    color: var(--display);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .geo-count { color: var(--sec); font-variant-numeric: tabular-nums; white-space: nowrap; }
  .geo-tags {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
    padding: 4px 8px 4px 0;
  }
  .geo-tag {
    padding: 1px 7px;
    border: 1px solid var(--border);
    border-radius: 3px;
    background: var(--surface);
    color: var(--sec);
    font: 400 12px/20px var(--font-ui);
    cursor: pointer;
  }
  .geo-tag:hover { color: var(--text); background: var(--surface-3); }
  .geo-tag.active {
    background: var(--selected);
    color: var(--selected-text);
    border-color: var(--border-vis);
  }
  .geo-item.active .geo-tag {
    background: var(--surface);
    color: var(--sec);
  }
  .preview-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    flex-shrink: 0;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
    color: var(--sec);
  }
  .preview-head strong { color: var(--display); font: 400 13px/20px var(--font-code); overflow-wrap: anywhere; min-width: 0; }
  .preview-head span { flex-shrink: 0; font-variant-numeric: tabular-nums; }
  .preview-body { flex: 1; min-height: 0; overflow: auto; padding: 6px 8px 10px; }
  .entry-list { list-style: none; margin: 0; padding: 0; display: grid; }
  .entry-list li { display: grid; grid-template-columns: 48px minmax(0, 1fr) auto; align-items: baseline; gap: 8px; padding: 5px 6px; border-bottom: 1px solid var(--border); }
  .entry-type { color: var(--sec); font: 12px/20px var(--font-ui); }
  .entry-list code { font: 12px/20px var(--font-code); color: var(--text); overflow-wrap: anywhere; }
  .entry-attrs { color: var(--dim); font: 12px/20px var(--font-code); }
  .preview-more { padding: 10px 6px 4px; }
  @media (max-width: 720px) {
    .catalog-panes {
      grid-template-columns: 1fr;
      grid-template-rows: minmax(120px, 1fr) minmax(160px, 1fr);
    }
  }
</style>
