<script lang="ts">
  import PixelButton from '../../components/pixel/PixelButton.svelte';
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import type { PreviewItem, PublicClient, PublicClientOption, PublicGeoCatalog, PublicGeositeList } from '../types';
  import { copyURL, formatCount, geoPath, geoPublished } from '../utils';

  interface Props {
    kind?: 'geosite' | 'geoip';
    catalogs: PublicGeoCatalog[];
    client: PublicClient;
    target: PublicClientOption;
    onpreview: (item: PreviewItem) => void;
  }

  let { kind = 'geosite', catalogs, client, target, onpreview }: Props = $props();
  let query = $state('');
  let expanded = $state<Record<string, boolean>>({});
  let showAll = $state<Record<string, boolean>>({});
  const pageSize = 100;

  function listsForTarget(lists: PublicGeositeList[]) {
    return lists.flatMap((list) => {
      const variants = (list.variants ?? []).filter((variant) => geoPublished(variant.targets, target.id));
      const hasFull = geoPublished(list.targets, target.id);
      if (!hasFull && variants.length === 0) return [];
      return [{ ...list, variants, hasFull }];
    });
  }

  const prepared = $derived.by(() => {
    const term = query.trim().toLowerCase();
    return catalogs.map((catalog) => ({
      ...catalog,
      lists: listsForTarget(catalog.lists).filter((list) => !term || list.name.toLowerCase().includes(term)),
    }));
  });
  const hasPublished = $derived(catalogs.some((catalog) => listsForTarget(catalog.lists).length > 0));

  function toggle(key: string) {
    expanded = { ...expanded, [key]: !expanded[key] };
  }

  function activateRow(provider: string, list: PublicGeositeList & { hasFull: boolean }, key: string) {
    if (list.variants.length > 0) {
      toggle(key);
      return;
    }
    if (list.hasFull) preview(provider, list.name, list.entries);
  }

  function handleRowKey(event: KeyboardEvent, run: () => void) {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    run();
  }

  async function copy(path: string, button: HTMLButtonElement) {
    const ok = await copyURL(path);
    if (ok) {
      const original = button.textContent;
      button.textContent = '已复制';
      setTimeout(() => { button.textContent = original; }, 1500);
    }
  }

  function preview(provider: string, name: string, entries: number, attr?: string) {
    onpreview({
      key: `${kind}:${provider}/${name}@${attr || ''}:${target.id}`,
      title: `${name}${attr ? ` @${attr}` : ''}`,
      tags: [kind, provider],
      path: geoPath(target, provider, name, attr, kind),
      entries,
      source: { kind, provider, name, attr },
    });
  }
</script>

<section class="public-block" aria-labelledby={`${kind}-heading`}>
  <div class="block-head catalog-head">
    <h2 class="block-title" id={`${kind}-heading`}>{kind === 'geoip' ? 'GeoIP' : 'Geosite'} 列表</h2>
    <input class="filter-input" type="search" placeholder="搜索列表…" bind:value={query} spellcheck="false" />
  </div>

  <div class="geo-root">
    {#if !hasPublished}
      <div class="empty">{kind === 'geoip' ? '还没有 GeoIP 数据，先运行 prm update。' : '还没有 Geosite 数据，先运行 prm update。'}</div>
    {:else}
    {#each prepared.filter((catalog) => catalog.lists.length > 0) as catalog (catalog.provider)}
      {@const shown = showAll[catalog.provider] ? catalog.lists : catalog.lists.slice(0, pageSize)}
      {@const variantCount = catalog.lists.reduce((total, list) => total + list.variants.length, 0)}
      <article class="geo-provider">
        <header>
          <strong>{catalog.provider}</strong>
          <span>{formatCount(catalog.lists.length)} 个列表{#if variantCount > 0} · {formatCount(variantCount)} 个属性变体{/if}</span>
          <span>格式：<b>{client.name} · {target.name}</b></span>
        </header>
        {#each shown as list (list.name)}
          {@const key = `${catalog.provider}/${list.name}`}
          {@const path = geoPath(target, catalog.provider, list.name, undefined, kind)}
          {@const expandable = list.variants.length > 0}
          <div class="geo-item" class:open={expanded[key]}>
            <div
              class="geo-row"
              role="button"
              tabindex="0"
              aria-label={list.name}
              aria-expanded={expandable ? !!expanded[key] : undefined}
              onclick={() => activateRow(catalog.provider, list, key)}
              onkeydown={(event) => handleRowKey(event, () => activateRow(catalog.provider, list, key))}
            >
              {#if expandable}
                <span class="geo-toggle">
                  <PixelIcon name={expanded[key] ? 'chevron-down' : 'chevron-right'} size={12} />
                  <strong>{list.name}</strong>
                </span>
              {:else}
                <span class="geo-name"><strong>{list.name}</strong></span>
              {/if}
              <span>{formatCount(list.entries)} 条</span>
              <span class="geo-tags">{#each list.variants.slice(0, 6) as variant}<em>@{variant.attr}</em>{/each}</span>
              <span class="row-actions" onclick={(event) => event.stopPropagation()} onkeydown={(event) => event.stopPropagation()}>
                {#if list.hasFull}
                  <PixelButton size="sm" onclick={() => preview(catalog.provider, list.name, list.entries)}>预览</PixelButton>
                  <button class="pill-btn" type="button" onclick={(event) => copy(path, event.currentTarget)}>复制链接</button>
                  <a class="pill-btn" href={path} target="_blank" rel="noopener">打开</a>
                {/if}
              </span>
            </div>
            {#if expandable && expanded[key]}
              <div class="geo-detail">
                {#if list.hasFull}
                  <div class="geo-file-row">
                    <span class="geo-tag full">完整列表</span><span>{formatCount(list.entries)} 条</span>
                    <span class="row-actions">
                      <PixelButton size="sm" onclick={() => preview(catalog.provider, list.name, list.entries)}>预览</PixelButton>
                      <a class="pill-btn" href={path} target="_blank" rel="noopener">打开</a>
                    </span>
                  </div>
                {/if}
                {#each list.variants as variant (variant.attr)}
                  {@const variantPath = geoPath(target, catalog.provider, list.name, variant.attr, kind)}
                  <div class="geo-file-row">
                    <span class="geo-tag">@{variant.attr}</span><span>{formatCount(variant.entries)} 条</span>
                    <span class="row-actions">
                      <PixelButton size="sm" onclick={() => preview(catalog.provider, list.name, variant.entries, variant.attr)}>预览</PixelButton>
                      <a class="pill-btn" href={variantPath} target="_blank" rel="noopener">打开</a>
                    </span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {:else}
          <div class="empty">没有匹配的列表。</div>
        {/each}
        {#if !showAll[catalog.provider] && catalog.lists.length > pageSize}
          <button class="more-btn" type="button" onclick={() => { showAll = { ...showAll, [catalog.provider]: true }; }}>
            显示全部 {formatCount(catalog.lists.length)} 个
          </button>
        {/if}
      </article>
    {:else}
      <div class="empty">没有匹配的列表。</div>
    {/each}
    {/if}
  </div>
</section>
