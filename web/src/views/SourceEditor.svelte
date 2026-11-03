<script lang="ts">
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelMenu from '../components/pixel/PixelMenu.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import GeoCatalogDrawer from './GeoCatalogDrawer.svelte';
  import SourceRow from './SourceRow.svelte';
  import type { GeoKind, RulePreview } from '../api/client';
  import {
    addInput, dissolveGroup, duplicateSource, emptySource, geoipProviders, geositeProviders, hasTextInput,
    mergeStrategies, moveInput, moveLosesProcessing, moveSource, removeInput, sourceKinds, splitInput,
    type RuleIssue, type RuleSource, type SourceKind,
  } from './rules';

  let {
    sources = $bindable(),
    expanded = $bindable(null),
    merge = 'union',
    onmergechange,
    defaultPreprocess = '',
    disabled = false,
    issues = [],
    fileOptions,
    refOptions,
    ongotopipeline,
    oneditfile,
    ontrial,
  }: {
    sources: RuleSource[];
    /** 当前展开的来源下标；同一时间只展开一个。 */
    expanded?: number | null;
    merge?: string;
    onmergechange: (strategy: string) => void;
    defaultPreprocess?: string;
    disabled?: boolean;
    /** 来源相关的校验错误（带 source / input 下标）。 */
    issues?: RuleIssue[];
    fileOptions: { value: string; label: string }[];
    refOptions: { value: string; label: string }[];
    ongotopipeline?: () => void;
    oneditfile?: (name: string, assign: (name: string) => void) => void;
    ontrial: (source: RuleSource) => Promise<RulePreview>;
  } = $props();

  type Drag = { index: number; input?: number };
  let drag = $state<Drag | null>(null);
  let overRow = $state<number | null>(null);
  let overGap = $state<number | null>(null);
  let pendingMove = $state<{ index: number; input: number; target: number } | null>(null);
  let moveConfirmOpen = $state(false);

  let geoOpen = $state(false);
  let geoKind = $state<GeoKind>('geosite');
  let geoProvider = $state('');
  let geoTarget: RuleSource | null = null;

  const generalIssues = $derived(issues.filter((issue) => issue.source === undefined));
  const inheriting = $derived(sources.filter((source) => source.preprocess === undefined && hasTextInput(source)).length);
  const operator = $derived(merge === 'intersect' ? '∩ 交集' : merge === 'difference' ? '− 减去' : '∪ 并集');

  function snapshot(): RuleSource[] {
    return $state.snapshot(sources) as RuleSource[];
  }
  function apply(next: RuleSource[]) {
    sources = next.length ? next : [emptySource()];
  }
  function toggle(index: number) {
    expanded = expanded === index ? null : index;
  }
  /** 行被移动或删除后，让展开状态跟着原来那一行走。 */
  function followRow(from: number, to: number | null) {
    if (expanded === null) return;
    if (expanded === from) { expanded = to; return; }
    if (to === null) { if (expanded > from) expanded--; return; }
    if (from < expanded && to >= expanded) expanded--;
    else if (from > expanded && to <= expanded) expanded++;
  }

  function addSource(kind: SourceKind) {
    const source = emptySource(kind);
    if (kind === 'geosite') source.provider = geositeProviders[0].value;
    if (kind === 'geoip') source.provider = geoipProviders[0].value;
    sources = [...sources, source];
    expanded = sources.length - 1;
  }
  function removeSource(index: number) {
    apply(snapshot().filter((_, i) => i !== index));
    followRow(index, null);
  }
  function reorder(from: number, to: number) {
    if (to < 0 || to >= sources.length || from === to) return;
    apply(moveSource(snapshot(), from, to));
    followRow(from, to);
  }
  function requestMove(index: number, input: number, target: number) {
    if (moveLosesProcessing(snapshot(), index, target)) {
      pendingMove = { index, input, target };
      moveConfirmOpen = true;
      return;
    }
    doMove(index, input, target);
  }
  function doMove(index: number, input: number, target: number) {
    const current = snapshot();
    const removesRow = !current[index].group || current[index].group!.length <= 1;
    apply(moveInput(current, index, input, target));
    const targetAfter = removesRow && index < target ? target - 1 : target;
    expanded = targetAfter;
  }
  function split(index: number, input: number, position = index + 1) {
    const current = snapshot();
    const removesRow = !!current[index].group && current[index].group!.length <= 1;
    apply(splitInput(current, index, input, position));
    const at = removesRow && position > index ? position - 1 : position;
    expanded = at;
  }

  function rowMenu(index: number) {
    const source = sources[index];
    return [
      { label: '上移', disabled: index === 0, onselect: () => reorder(index, index - 1) },
      { label: '下移', disabled: index === sources.length - 1, onselect: () => reorder(index, index + 1) },
      { label: '复制', onselect: () => { apply(duplicateSource(snapshot(), index)); expanded = index + 1; } },
      // 单输入的来源可以整体并入另一行，成为那一行的一个输入。
      ...(source.group ? [] : sources.map((target, t) => ({ t, label: `并入 来源 ${t + 1}${target.label ? `（${target.label}）` : ''}` }))
        .filter((item) => item.t !== index)
        .map((item) => ({ label: item.label, onselect: () => requestMove(index, 0, item.t) }))),
      ...(source.group?.length === 1 ? [{ label: '解散为普通来源', onselect: () => apply(dissolveGroup(snapshot(), index)) }] : []),
      { label: '删除来源', danger: true, disabled: sources.length === 1, onselect: () => removeSource(index) },
    ];
  }
  function inputMenu(index: number) {
    return (input: number) => [
      ...sources.map((target, t) => ({ label: `移到 来源 ${t + 1}${target.label ? `（${target.label}）` : ''}`, t }))
        .filter((item) => item.t !== index)
        .map((item) => ({ label: item.label, onselect: () => requestMove(index, input, item.t) })),
      { label: '拆为独立来源', onselect: () => split(index, input) },
      { label: '删除输入', danger: true, onselect: () => apply(removeInput(snapshot(), index, input)) },
    ];
  }

  function startRowDrag(event: DragEvent, index: number) {
    drag = { index };
    event.dataTransfer?.setData('text/plain', `source:${index}`);
  }
  function startInputDrag(event: DragEvent, index: number, input: number) {
    event.stopPropagation();
    drag = { index, input };
    event.dataTransfer?.setData('text/plain', `input:${index}:${input}`);
  }
  function endDrag() {
    drag = null;
    overRow = null;
    overGap = null;
  }
  function dropOnRow(target: number) {
    const current = drag;
    endDrag();
    if (!current || disabled) return;
    if (current.input === undefined) reorder(current.index, target);
    else if (target !== current.index) requestMove(current.index, current.input, target);
  }
  function dropOnGap(position: number) {
    const current = drag;
    endDrag();
    if (!current || disabled || current.input === undefined) return;
    split(current.index, current.input, position);
  }

  function pickGeo(source: RuleSource) {
    geoTarget = source;
    geoKind = source.kind === 'geoip' ? 'geoip' : 'geosite';
    if (!source.provider) source.provider = (geoKind === 'geosite' ? geositeProviders : geoipProviders)[0].value;
    geoProvider = source.provider;
    geoOpen = true;
  }
  function picked(list: string, attr: string) {
    if (!geoTarget) return;
    geoTarget.provider = geoProvider;
    geoTarget.list = list;
    if (geoTarget.kind === 'geosite') geoTarget.attrs = attr;
    geoTarget = null;
  }
</script>

<section class="source-editor" aria-label="来源列表">
  <header>
    <div class="merge">
      <span id="source-merge-label">合并方式</span>
      <PixelSelect id="rule-merge" label="合并策略" size="sm" options={[...mergeStrategies]} value={merge || 'union'} {disabled} onchange={onmergechange} />
    </div>
    <PixelMenu label="添加来源" text="+ 添加来源" {disabled}
      items={sourceKinds.map((kind) => ({ label: kind.label, onselect: () => addSource(kind.value) }))} />
  </header>
  {#if defaultPreprocess.trim()}
    <p class="hint">
      默认预处理已配置 · {inheriting} 个来源使用
      {#if ongotopipeline}<button type="button" class="link" onclick={ongotopipeline}>编辑默认脚本</button>{/if}
    </p>
  {/if}
  {#each generalIssues as issue}<p class="error" role="alert">{issue.message}</p>{/each}

  <div class="list" class:dragging={!!drag}>
    {#each sources as source, i (source)}
      {#if drag?.input !== undefined}
        <div class="gap" class:over={overGap === i} role="region" aria-label="拖到此处作为第 {i + 1} 个独立来源"
          ondragover={(event) => { event.preventDefault(); overGap = i; overRow = null; }}
          ondragleave={() => { if (overGap === i) overGap = null; }}
          ondrop={(event) => { event.preventDefault(); dropOnGap(i); }}>拖到此处作为独立来源</div>
      {:else if i > 0}
        <div class="operator" aria-hidden="true">{operator}</div>
      {/if}
      <div class="row-wrap" class:over={overRow === i} class:dimmed={drag?.index === i && drag.input === undefined}
        role="listitem"
        ondragover={(event) => { if (drag && !disabled) { event.preventDefault(); overRow = i; overGap = null; } }}
        ondragleave={() => { if (overRow === i) overRow = null; }}
        ondrop={(event) => { event.preventDefault(); dropOnRow(i); }}>
        {#if merge === 'difference' && i === 0 && sources.length > 1}
          <span class="role">基准</span>
        {/if}
        <SourceRow bind:source={sources[i]} index={i} expanded={expanded === i} {disabled} {defaultPreprocess}
          issues={issues.filter((issue) => issue.source === i)} {fileOptions} {refOptions}
          menu={rowMenu(i)} inputMenu={inputMenu(i)}
          ontoggle={() => toggle(i)}
          onaddinput={() => apply(addInput(snapshot(), i))}
          ondragstart={(event) => startRowDrag(event, i)} ondragend={endDrag}
          oninputdragstart={(event, input) => startInputDrag(event, i, input)}
          {oneditfile} onpickgeo={pickGeo} {ontrial} />
      </div>
    {/each}
    {#if drag?.input !== undefined}
      <div class="gap" class:over={overGap === sources.length} role="region" aria-label="拖到此处作为最后一个独立来源"
        ondragover={(event) => { event.preventDefault(); overGap = sources.length; overRow = null; }}
        ondragleave={() => { if (overGap === sources.length) overGap = null; }}
        ondrop={(event) => { event.preventDefault(); dropOnGap(sources.length); }}>拖到此处作为独立来源</div>
    {/if}
  </div>
</section>

<PixelDialog bind:open={moveConfirmOpen} title="移入后使用目标来源的处理设置" confirmLabel="移动" cancelLabel="取消"
  oncancel={() => { pendingMove = null; }}
  onconfirm={() => { const move = pendingMove; pendingMove = null; if (move) doMove(move.index, move.input, move.target); }}>
  这是原来源唯一的输入，移动后原来源会被删除，它单独配置的预处理和过滤也会一并丢弃。
</PixelDialog>
<GeoCatalogDrawer bind:open={geoOpen} provider={geoProvider} kind={geoKind} onpick={picked} />

<style>
  .source-editor { display: grid; gap: 10px; min-width: 0; }
  header { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
  .merge { display: flex; align-items: center; gap: 8px; color: var(--sec); font: 12px/20px var(--font-ui); }
  .merge :global(.pixel-select) { width: 120px; }
  .hint { margin: 0; color: var(--sec); font: 12px/18px var(--font-ui); }
  .link { padding: 0 2px; border: 0; background: none; color: var(--text); font: inherit; text-decoration: underline; cursor: pointer; }
  .error { margin: 0; color: var(--error-border); font: 12px/18px var(--font-ui); }
  .list { display: grid; gap: 0; min-width: 0; }
  .row-wrap { position: relative; min-width: 0; border-radius: 4px; }
  .row-wrap.over { outline: 2px dashed var(--accent); outline-offset: 2px; }
  .row-wrap.dimmed { opacity: .5; }
  .role { position: absolute; top: -9px; left: 10px; z-index: 1; padding: 0 6px; background: var(--surface); border: 1px solid var(--border); border-radius: 3px; color: var(--sec); font: 11px/16px var(--font-ui); }
  .operator { padding: 4px 0; text-align: center; color: var(--dim); font: 12px/18px var(--font-ui); }
  .gap { margin: 4px 0; padding: 8px; text-align: center; color: var(--sec); border: 2px dashed var(--border-vis); border-radius: 4px; font: 12px/18px var(--font-ui); }
  .gap.over { background: var(--surface-2); border-color: var(--accent); }
</style>
