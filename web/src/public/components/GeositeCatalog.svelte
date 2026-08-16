<script lang="ts">
  import PixelButton from '../../components/pixel/PixelButton.svelte';
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import type { PreviewItem, PublicClient, PublicClientOption, PublicGeositeCatalog } from '../types';
  import { copyURL, formatCount, geositePath } from '../utils';

  interface Props {
    catalogs: PublicGeositeCatalog[];
    client: PublicClient;
    target: PublicClientOption;
    onpreview: (item: PreviewItem) => void;
  }

  let { catalogs, client, target, onpreview }: Props = $props();
  let query = $state('');
  let expanded = $state<Record<string, boolean>>({});
  let showAll = $state<Record<string, boolean>>({});
  const pageSize = 100;

  function toggle(key: string) {
    expanded = { ...expanded, [key]: !expanded[key] };
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
      key: `geosite:${provider}/${name}@${attr || ''}:${target.id}`,
      title: `${name}${attr ? ` @${attr}` : ''}`,
      tags: ['geosite', provider],
      path: geositePath(target, provider, name, attr),
      entries,
      source: { kind: 'geosite', provider, name, attr },
    });
  }
</script>

<section class="public-block" aria-labelledby="geosite-heading">
  <div class="block-head catalog-head">
    <h2 class="block-title" id="geosite-heading">Geosite 列表</h2>
    <input class="filter-input" type="search" placeholder="搜索列表…" bind:value={query} spellcheck="false" />
  </div>

  <div class="geo-root">
    {#each catalogs as catalog (catalog.provider)}
      {@const term = query.trim().toLowerCase()}
      {@const matched = catalog.lists.filter((list) => !term || list.name.toLowerCase().includes(term))}
      {@const shown = showAll[catalog.provider] ? matched : matched.slice(0, pageSize)}
      {@const variantCount = catalog.lists.reduce((total, list) => total + list.variants.length, 0)}
      <article class="geo-provider">
        <header>
          <strong><PixelIcon name="globe" size={18} /> {catalog.provider}</strong>
          <span>{formatCount(catalog.lists.length)} 个列表 · {formatCount(variantCount)} 个属性变体</span>
          <span>格式：<b>{client.name} · {target.name}</b></span>
        </header>
        {#each shown as list (list.name)}
          {@const key = `${catalog.provider}/${list.name}`}
          {@const path = geositePath(target, catalog.provider, list.name)}
          <div class="geo-item" class:open={expanded[key]}>
            <div class="geo-row">
              <button class="geo-toggle" type="button" onclick={() => toggle(key)} aria-expanded={!!expanded[key]}>
                <PixelIcon name={expanded[key] ? 'chevron-down' : 'chevron-right'} size={9} />
                <strong>{list.name}</strong>
              </button>
              <span>{formatCount(list.entries)} 条</span>
              <span class="geo-tags">{#each list.variants.slice(0, 6) as variant}<em>@{variant.attr}</em>{/each}</span>
              <span class="row-actions">
                <PixelButton size="sm" onclick={() => preview(catalog.provider, list.name, list.entries)}>预览</PixelButton>
                <button class="pill-btn" type="button" onclick={(event) => copy(path, event.currentTarget)}>复制链接</button>
                <a class="pill-btn" href={path} target="_blank" rel="noopener">打开</a>
              </span>
            </div>
            {#if expanded[key]}
              <div class="geo-detail">
                <div class="geo-file-row">
                  <span class="geo-tag full">完整列表</span><span>{formatCount(list.entries)} 条</span>
                  <span class="row-actions">
                    <PixelButton size="sm" onclick={() => preview(catalog.provider, list.name, list.entries)}>预览</PixelButton>
                    <a class="pill-btn" href={path} target="_blank" rel="noopener">打开</a>
                  </span>
                </div>
                {#each list.variants as variant (variant.attr)}
                  {@const variantPath = geositePath(target, catalog.provider, list.name, variant.attr)}
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
        {#if !showAll[catalog.provider] && matched.length > pageSize}
          <button class="more-btn" type="button" onclick={() => { showAll = { ...showAll, [catalog.provider]: true }; }}>
            显示全部 {formatCount(matched.length)} 个
          </button>
        {/if}
      </article>
    {:else}
      <div class="empty">还没有 Geosite 数据，先运行 prm update。</div>
    {/each}
  </div>
</section>
