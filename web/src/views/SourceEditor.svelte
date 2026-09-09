<script lang="ts">
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import OpsEditor from '../components/forms/OpsEditor.svelte';
  import SourceFields from './SourceFields.svelte';
  import { emptySource, type RuleSource } from './rules';
  let { sources = $bindable(), preprocess, disabled = false, fileOptions, refOptions }: {
    sources: RuleSource[]; preprocess?: string; disabled?: boolean;
    fileOptions: { value: string; label: string }[]; refOptions: { value: string; label: string }[];
  } = $props();
  let dragging = $state<{ index: number; child?: number } | null>(null);
  let over = $state<number | null>(null);
  let notice = $state('');
  function convertToGroup(index: number) {
    if (disabled) return;
    const source = sources[index];
    if (source.group) return;
    const member = { ...source }; delete member.preprocess; delete member.ops;
    // 保留三态语义：undefined 继承统一预处理、'' 显式禁用、脚本是独立覆盖，
    // 不能把统一预处理物化进组，否则继承关系在保存时被静默改写。
    sources[index] = { kind: 'group', label: source.label, group: [member], preprocess: source.preprocess, ops: source.ops ?? [] };
  }
  function move(target: number | null) {
    if (disabled || !dragging) return;
    const { index, child } = dragging;
    dragging = null; over = null; notice = '';
    if (target === index || (target === null && child === undefined)) return;
    const owner = sources[index];
    const source = child === undefined ? owner : owner.group![child];
    const destination = target === null ? null : sources[target];
    if (source.kind === 'group') { notice = '请拖动组内的来源'; return; }
    const processing = child === undefined ? source : owner;
    const hasProcessing = processing.preprocess !== undefined || !!processing.ops?.length;
    if (destination && hasProcessing && (
      (processing.preprocess ?? preprocess ?? '') !== (destination.preprocess ?? preprocess ?? '') ||
      JSON.stringify(processing.ops ?? []) !== JSON.stringify(destination.ops ?? [])
    )) {
      notice = '此来源与目标的处理配置不同，请先统一预处理和过滤链'; return;
    }
    if (child !== undefined) {
      owner.group = owner.group!.filter((_, i) => i !== child);
    } else sources = sources.filter((_, i) => i !== index);
    if (destination) { delete source.preprocess; delete source.ops; }
    if (!destination) {
      const hasLocalProcessing = owner.preprocess !== undefined || !!owner.ops?.length;
      sources = [...sources, hasLocalProcessing ? {
        kind: 'group', group: [source], preprocess: owner.preprocess,
        ops: structuredClone($state.snapshot(owner.ops ?? [])),
      } : source];
    } else if (destination.group) destination.group = [...destination.group, source];
    else {
      const member = { ...destination }; delete member.preprocess; delete member.ops;
      const group: RuleSource = { kind: 'group', label: destination.label, group: [member, source], preprocess: destination.preprocess, ops: destination.ops ?? [] };
      sources = sources.map(item => item === destination ? group : item);
    }
    sources = sources.filter(item => !item.group || item.group.length > 0);
  }
</script>

{#snippet moveControl(index: number, child?: number)}
  <label class="move-control">移入来源组
    <select {disabled} value="" onchange={event => { if (event.currentTarget.value !== '') { dragging = { index, child }; move(Number(event.currentTarget.value)); event.currentTarget.value = ''; } }}>
      <option value="">选择目标</option>
      {#each sources as target, targetIndex}
        {#if targetIndex !== index}<option value={targetIndex}>{target.label || `${target.group ? '来源组' : '来源'} ${targetIndex + 1}`}</option>{/if}
      {/each}
    </select>
  </label>
{/snippet}

{#snippet processing(source: RuleSource, opsLabel: string)}
  <details class="processing">
    <summary>预处理 · {source.preprocess === undefined ? '继承统一' : source.preprocess ? '已配置' : '已禁用'}</summary>
    <div class="body">
      <p class="inherit-hint">
        {#if source.preprocess === undefined}
          留空则继承统一预处理
        {:else}
          已覆盖统一预处理{source.preprocess === '' ? '（当前为禁用）' : ''}
          <button type="button" class="inherit-reset" {disabled} onclick={() => source.preprocess = undefined}>恢复继承</button>
        {/if}
      </p>
      <label>JavaScript<textarea value={source.preprocess ?? ''} oninput={event => source.preprocess = event.currentTarget.value} {disabled} rows="8" spellcheck="false" placeholder={"function process(content) { return content; }"}></textarea></label>
    </div>
  </details>
  <details class="processing">
    <summary>{opsLabel} · {source.ops?.length ?? 0} 项</summary>
    <div class="body"><OpsEditor bind:value={source.ops!} {disabled} /></div>
  </details>
{/snippet}

<section class="source-editor">
  <header><h3>来源</h3><PixelButton size="sm" {disabled} onclick={() => sources = [...sources, emptySource()]}>添加来源</PixelButton></header>
  {#if notice}<p role="status" class="hint">{notice}</p>{/if}
  {#each sources as source, i (source)}
    <section class:card={!!source.group} class:plain={!source.group} class:over={over === i} aria-label="来源 {i + 1}"
      ondragover={event => { if (dragging && !disabled) { event.preventDefault(); over = i; } }}
      ondragleave={() => over = null} ondrop={event => { event.preventDefault(); event.stopPropagation(); move(i); }}>
      {#if source.group}
      <details open>
        <summary>{source.label || (source.group ? `来源组 ${i + 1}` : `来源 ${i + 1}`)}{source.group ? ` · ${source.group.length} 个来源` : ''}</summary>
        <div class="body">
          <div class="actions">
            <PixelButton size="sm" disabled={disabled || sources.length === 1} onclick={() => sources = sources.filter((_, j) => i !== j)}>删除{source.group ? '来源组' : '来源'}</PixelButton>
          </div>
            <label>组名<input bind:value={source.label} {disabled} placeholder="可选" /></label>
            {#each source.group as member, j (member)}
              <div class="member">
                <div class="member-title">{member.label || `来源 ${j + 1}`}</div>
                <div class="body">
                  <div class="actions">
                    <button type="button" class="handle" draggable={!disabled} {disabled} aria-label="拖动组内来源 {j + 1}" ondragstart={event => { dragging = { index: i, child: j }; event.dataTransfer?.setData('text/plain', `${i}:${j}`); }} ondragend={() => { dragging = null; over = null; }}>⠿ 拖动</button>
                    {@render moveControl(i, j)}
                    <PixelButton size="sm" {disabled} onclick={() => { dragging = { index: i, child: j }; move(null); }}>移出组</PixelButton>
                    <PixelButton size="sm" disabled={disabled || source.group!.length === 1} onclick={() => source.group = source.group!.filter((_, k) => k !== j)}>删除来源</PixelButton>
                  </div>
                  <SourceFields bind:source={source.group[j]} label={`${i + 1}.${j + 1}`} {disabled} {fileOptions} {refOptions} />
                </div>
              </div>
            {/each}
            <PixelButton size="sm" {disabled} onclick={() => source.group = [...source.group!, emptySource()]}>添加组内来源</PixelButton>
            {@render processing(source, '组内过滤链')}
        </div>
      </details>
      {:else}
        <div class="actions">
          <strong>来源 {i + 1}</strong>
          <button type="button" class="handle" draggable={!disabled} {disabled} aria-label="拖动来源 {i + 1}" ondragstart={event => { dragging = { index: i }; event.dataTransfer?.setData('text/plain', String(i)); }} ondragend={() => { dragging = null; over = null; }}>⠿</button>
          <PixelButton size="sm" {disabled} title="多个来源共用一个组，共享同一份预处理和过滤链" onclick={() => convertToGroup(i)}>转为来源组</PixelButton>
          {#if sources.length > 1}{@render moveControl(i)}{/if}
          <PixelButton size="sm" disabled={disabled || sources.length === 1} onclick={() => sources = sources.filter((_, j) => i !== j)}>删除来源</PixelButton>
        </div>
        <SourceFields bind:source={sources[i]} label={String(i + 1)} {disabled} {fileOptions} {refOptions} />
        {@render processing(source, '过滤链')}
      {/if}
    </section>
  {/each}
  {#if dragging?.child !== undefined}
    <div class="drop-out" role="region" aria-label="移出来源组" ondragover={event => event.preventDefault()} ondrop={event => { event.preventDefault(); move(null); }}>拖到此处作为独立来源</div>
  {/if}
</section>

<style>
  .source-editor, .body { display: grid; gap: 12px; min-width: 0; }
  header, .actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  h3 { margin: 0 auto 0 0; font-size: 14px; }
  .hint { margin: 0; color: var(--sec); font-size: 12px; }
  .card { border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface); }
  .over, .drop-out { outline: 2px dashed var(--accent); }
  summary { cursor: pointer; padding: 12px; font-size: 13px; overflow-wrap: anywhere; }
  .body { padding: 0 12px 12px; }
  .plain { display: grid; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--border); }
  .plain strong { margin-right: auto; font-size: 13px; }
  .member-title { padding: 10px 12px; font-size: 13px; }
  .member { border-bottom: 1px solid var(--border); }
  .processing { border: 1px solid var(--border); border-radius: 3px; min-width: 0; }
  .inherit-hint { margin: 0; color: var(--sec); font-size: 12px; }
  .inherit-reset { padding: 0 2px; border: 0; background: none; color: var(--text); font: inherit; text-decoration: underline; cursor: pointer; }
  .inherit-reset:disabled { opacity: .45; cursor: not-allowed; }
  label { display: grid; gap: 6px; font-size: 13px; }
  input {
    width: 100%;
    min-width: 0;
    min-height: 32px;
    box-sizing: border-box;
    padding: 4px 8px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    box-shadow: var(--edge-inset);
    background: var(--surface);
    color: var(--text);
    font: 13px/20px var(--font-code);
    outline: none;
    transition: background-color 80ms linear;
  }
  input:focus-visible {
    outline: 1px solid var(--selected);
    border-color: var(--selected);
  }
  textarea {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    min-height: 130px;
    max-height: 280px;
    padding: 8px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    box-shadow: var(--edge-inset);
    background: var(--surface);
    color: var(--text);
    overflow: auto;
    font: 12px/1.6 var(--font-code);
    tab-size: 2;
    resize: vertical;
    outline: none;
  }
  textarea:focus-visible {
    outline: 1px solid var(--selected);
    border-color: var(--selected);
  }
  .move-control { display: flex; align-items: center; gap: 6px; }
  select { max-width: 180px; padding: 5px; background: var(--surface); color: var(--text); border: 1px solid var(--border-vis); border-radius: 3px; }
  .handle { cursor: grab; color: var(--sec); background: var(--surface); border: 1px solid var(--border-vis); border-radius: 3px; padding: 5px 8px; }
  .drop-out { padding: 16px; text-align: center; color: var(--sec); }
</style>
