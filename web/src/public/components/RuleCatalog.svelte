<script lang="ts">
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import type { PublicClientOption, PublicRule } from '../types';
  import { formatCount } from '../utils';

  interface Props {
    rules: PublicRule[];
    tags: string[];
    target: PublicClientOption;
    onpreview: (rule: PublicRule) => void;
  }

  let { rules, tags, target, onpreview }: Props = $props();
  let query = $state('');
  let activeTags = $state<string[]>([]);

  const filteredRules = $derived.by(() => {
    const term = query.trim().toLowerCase();
    return rules.filter((rule) => {
      const matchesTerm = !term || rule.name.toLowerCase().includes(term) || rule.id.toLowerCase().includes(term);
      const normalized = rule.tags.map((tag) => tag.toLowerCase());
      const matchesTags = activeTags.length === 0 || activeTags.some((tag) => normalized.includes(tag));
      return matchesTerm && matchesTags;
    });
  });

  function toggleTag(tag: string) {
    const normalized = tag.toLowerCase();
    activeTags = activeTags.includes(normalized)
      ? activeTags.filter((value) => value !== normalized)
      : [...activeTags, normalized];
  }
</script>

<section class="public-block" aria-labelledby="rules-heading">
  <div class="block-head catalog-head">
    <h2 class="block-title" id="rules-heading">
      规则 <span class="count">{filteredRules.length} / {rules.length}</span>
    </h2>
    <input class="filter-input" type="search" placeholder="搜索规则…" bind:value={query} spellcheck="false" />
  </div>

  <div class="chips" aria-label="规则标签">
    {#each tags as tag}
      <button class:on={activeTags.includes(tag.toLowerCase())} type="button" onclick={() => toggleTag(tag)}>{tag}</button>
    {/each}
  </div>

  <div class="table-scroll" role="region" aria-labelledby="rules-heading">
    <table class="data-table rule-table">
      <thead>
        <tr><th>名称</th><th>标签</th><th class="num">条目</th><th class="action">查看</th></tr>
      </thead>
      <tbody>
        {#each filteredRules as rule (rule.id)}
          <tr
            class="rule-row"
            onclick={() => onpreview(rule)}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onpreview(rule); } }}
            tabindex="0"
            role="button"
            aria-label="查看规则 {rule.name}"
          >
            <th scope="row">
              <button
                type="button"
                class="rule-name-btn"
                onclick={(e) => { e.stopPropagation(); onpreview(rule); }}
              >
                {rule.name}
              </button>
            </th>
            <td><span class="rtags">{#each rule.tags as tag}<em>{tag}</em>{/each}</span></td>
            <td class="num">{formatCount(rule.entries)}</td>
            <td class="action">
              <button
                type="button"
                class="action-btn"
                aria-label="预览 {rule.name}"
                onclick={(e) => { e.stopPropagation(); onpreview(rule); }}
              >
                <PixelIcon name="chevron-right" size={10} />
              </button>
            </td>
          </tr>
        {:else}
          <tr><td colspan="4" class="empty">没有匹配的规则。</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  <div class="target-hint">当前格式：{target.name} · {target.ext}</div>
</section>
