<script lang="ts">
  import PixelSelect from './pixel/PixelSelect.svelte';
  import PixelInput from './pixel/PixelInput.svelte';
  import PixelCheckbox from './pixel/PixelCheckbox.svelte';
  import PixelButton from './pixel/PixelButton.svelte';
  import type { RuleSyntax } from '../api/client';
  import { formatRuleLine, formatYAMLItem, noResolveKinds } from '../utils/ruleSyntax';

  let { syntax, yaml = false, disabled = false, oninsert }: {
    syntax: RuleSyntax;
    /** 目标是 YAML payload 文件时生成 `  - '...'` 列表项。 */
    yaml?: boolean;
    disabled?: boolean;
    oninsert: (line: string) => void;
  } = $props();
  const id = $props.id();

  let type = $state('DOMAIN-SUFFIX');
  let value = $state('');
  let noResolve = $state(false);

  // 插入栏只提供规范写法；逻辑规则（AND/OR/NOT）结构复杂，直接在编辑器里写。
  const options = $derived(syntax.types
    .filter((item) => !item.alias && !['and', 'or', 'not'].includes(item.kind))
    .map((item) => ({ value: item.name, label: item.name })));
  const kind = $derived(syntax.types.find((item) => item.name === type)?.kind ?? '');
  const canNoResolve = $derived(noResolveKinds.has(kind));

  function insert() {
    if (disabled || !value.trim()) return;
    const line = formatRuleLine(type, value, canNoResolve && noResolve);
    oninsert(yaml ? formatYAMLItem(line) : line);
    value = '';
  }
</script>

<div class="insert-bar" role="group" aria-label="插入规则">
  <PixelSelect id="{id}-type" label="插入规则类型" size="sm" {options} value={type} {disabled} onchange={(next) => { type = next; }} />
  <form class="value" onsubmit={(event) => { event.preventDefault(); insert(); }}>
    <PixelInput aria-label="插入规则值" placeholder="值，例如 example.com" spellcheck="false" bind:value {disabled} />
  </form>
  {#if canNoResolve}
    <PixelCheckbox size="sm" label="no-resolve" checked={noResolve} {disabled} onchange={(checked) => { noResolve = checked; }} />
  {/if}
  <PixelButton size="sm" disabled={disabled || !value.trim()} onclick={insert}>插入</PixelButton>
</div>

<style>
  .insert-bar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; flex-shrink: 0; }
  .insert-bar :global(.pixel-select) { width: 190px; flex-shrink: 0; }
  .value { flex: 1 1 180px; min-width: 140px; margin: 0; }
</style>
