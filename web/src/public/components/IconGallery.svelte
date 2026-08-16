<script lang="ts">
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import type { PublicIconSet } from '../types';
  import { copyURL, formatCount, iconPath } from '../utils';

  interface Props { sets: PublicIconSet[]; }
  let { sets }: Props = $props();
  let selected = $state<PublicIconSet | null>(null);
  let shown = $state(60);

  function openSet(set: PublicIconSet) {
    selected = set;
    shown = Math.min(60, set.icons.length);
  }

  async function copy(path: string, button: HTMLButtonElement) {
    const ok = await copyURL(path);
    if (ok) {
      const label = button.textContent;
      button.textContent = '已复制';
      setTimeout(() => { button.textContent = label; }, 1500);
    }
  }
</script>

<section class="public-block" aria-labelledby="icons-heading">
  {#if selected}
    <button class="icon-back" type="button" onclick={() => { selected = null; }}>［ 返回图标集 ］</button>
    <div class="block-head">
      <h2 class="block-title" id="icons-heading">{selected.name} <span class="count">{formatCount(selected.count)}</span></h2>
    </div>
    <div class="icon-grid">
      {#each selected.icons.slice(0, shown) as fileName (fileName)}
        {@const path = iconPath(selected.name, fileName)}
        <article class="icon-card">
          <img loading="lazy" src={path} alt={fileName.replace(/\.[^.]+$/, '')} />
          <strong title={fileName}>{fileName.replace(/\.[^.]+$/, '')}</strong>
          <span>
            <button type="button" onclick={(event) => copy(path, event.currentTarget)}>复制</button>
            <a href={path} download={fileName}>下载</a>
          </span>
        </article>
      {/each}
    </div>
    {#if shown < selected.icons.length}
      <button class="more-btn" type="button" onclick={() => { shown = Math.min(shown + 60, selected!.icons.length); }}>
        加载更多（{formatCount(shown)} / {formatCount(selected.count)}）
      </button>
    {/if}
  {:else}
    <div class="block-head">
      <h2 class="block-title" id="icons-heading">图标集 <span class="count">{sets.length}</span></h2>
    </div>
    <div class="icon-sets">
      {#each sets as set (set.name)}
        <button class="icon-set-card" type="button" onclick={() => openSet(set)}>
          <img src="static/icons/iconset.svg" width="32" height="32" alt="" class="is-icon" aria-hidden="true" />
          <span><strong>{set.name}</strong><small>{formatCount(set.count)} 个图标</small></span>
        </button>
      {:else}
        <div class="empty">还没有图标集。</div>
      {/each}
    </div>
  {/if}
</section>
