<script lang="ts">
  import PixelButton from '../pixel/PixelButton.svelte';
  import PixelSelect from '../pixel/PixelSelect.svelte';
  import PixelCheckbox from '../pixel/PixelCheckbox.svelte';
  import { ruleKinds, type FilterOp } from './ops';
  let { value = $bindable([]), disabled = false }: { value: FilterOp[]; disabled?: boolean } = $props();
  const id = $props.id();
  const types = [{ value: 'include_kinds', label: '保留类型' }, { value: 'exclude_kinds', label: '排除类型' }, { value: 'filter_values', label: '匹配值' }];
  const modes = [{ value: 'keyword', label: '关键词' }, { value: 'suffix', label: '后缀' }, { value: 'prefix', label: '前缀' }, { value: 'exact', label: '精确' }, { value: 'regex', label: '正则表达式' }];
  function move(index: number, offset: number) {
    const next = [...value];
    [next[index], next[index + offset]] = [next[index + offset], next[index]];
    value = next;
  }
</script>
<div class="ops-editor">
  {#each value as op, i}
    <section>
      <div class="op-head">
        <span>{i + 1}.</span>
        <PixelSelect id="{id}-type-{i}" label="过滤操作 {i + 1}" options={types} value={op.type} {disabled}
          onchange={type => { value[i] = type === 'filter_values' ? { type, mode: 'keyword', pattern: '' } : { type, kinds: [] }; }} />
        <PixelButton size="sm" disabled={disabled || i === 0} onclick={() => move(i, -1)} aria-label="上移操作 {i + 1}">↑</PixelButton>
        <PixelButton size="sm" disabled={disabled || i === value.length - 1} onclick={() => move(i, 1)} aria-label="下移操作 {i + 1}">↓</PixelButton>
        <PixelButton size="sm" {disabled} onclick={() => { value = value.filter((_, j) => j !== i); }} aria-label="删除操作 {i + 1}">删除</PixelButton>
      </div>
      {#if op.type === 'filter_values'}
        <div class="value-fields">
          <PixelSelect id="{id}-mode-{i}" label="匹配方式 {i + 1}" options={modes} value={op.mode || 'keyword'} {disabled} onchange={mode => { op.mode = mode; }} />
          <input aria-label="匹配值 {i + 1}" placeholder="匹配值" bind:value={op.pattern} {disabled} />
        </div>
      {:else}
        <details>
          <summary>规则类型 · 已选 {op.kinds?.length ?? 0}</summary>
          <div class="kinds">
            {#each ruleKinds as kind}
              <PixelCheckbox
                size="sm"
                {disabled}
                checked={op.kinds?.includes(kind) ?? false}
                onchange={checked => {
                  op.kinds = checked ? [...(op.kinds ?? []), kind] : (op.kinds ?? []).filter(k => k !== kind);
                }}
                label={kind}
              />
            {/each}
          </div>
        </details>
        {#if op.kinds?.length}<p class="selection">{op.kinds.join(' · ')}</p>{/if}
      {/if}
    </section>
  {/each}
  <div class="op-head">
    <PixelButton size="sm" {disabled} onclick={() => { value = [...value, { type: 'include_kinds', kinds: [] }]; }}>添加过滤</PixelButton>
  </div>
</div>
<style>
  .ops-editor { display: grid; gap: 10px; }
  section { border: 1px solid var(--border); border-radius: 3px; padding: 10px; display: grid; gap: 10px; }
  .op-head, .value-fields { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .value-fields input { flex: 1; min-width: 160px; }
  summary { cursor: pointer; color: var(--sec); }
  .kinds { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 6px; padding-top: 10px; }
  :global(.kinds .pixel-checkbox) { font-family: var(--font-code); }
  input {
    min-height: 32px;
    box-sizing: border-box;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--edge-inset);
    padding: 4px 8px;
    font: 13px/20px var(--font-code);
  }
</style>
