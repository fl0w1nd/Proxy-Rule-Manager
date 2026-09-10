<script lang="ts">
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import CodeEditor from '../components/CodeEditor.svelte';
  import OpsEditor from '../components/forms/OpsEditor.svelte';
  import SourceFields from './SourceFields.svelte';
  import { emptySource, type RuleSource } from './rules';
  let { sources = $bindable(), preprocess, disabled = false, fileOptions, refOptions }: {
    sources: RuleSource[]; preprocess?: string; disabled?: boolean;
    fileOptions: { value: string; label: string }[]; refOptions: { value: string; label: string }[];
  } = $props();
  const uid = $props.id();
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
  <div class="move-control">
    <span class="move-label">移入来源组</span>
    <PixelSelect id="move-{uid}-{index}-{child ?? 'root'}" label="来源 {index + 1} 移入目标"
      options={[{ value: '', label: '选择目标' },
        ...sources.map((target, targetIndex) => ({ value: String(targetIndex), label: target.label || `${target.group ? '来源组' : '来源'} ${targetIndex + 1}` })).filter((_, targetIndex) => targetIndex !== index)]}
      value="" {disabled}
      onchange={(value) => { if (value !== '') { dragging = { index, child }; move(Number(value)); } }} />
  </div>
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
      <CodeEditor value={source.preprocess ?? ''} language="javascript" label="JavaScript" filename="process.js" height="200px"
        placeholder={"function process(content) { return content; }"} readonly={disabled}
        onchange={(value) => source.preprocess = value} />
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
    <section class:card={!!source.group} class:plain={!source.group} class:over={over === i} class:dimmed={dragging?.index === i && dragging.child === undefined} aria-label="来源 {i + 1}"
      ondragover={event => { if (dragging && !disabled) { event.preventDefault(); over = i; } }}
      ondragleave={() => over = null} ondrop={event => { event.preventDefault(); event.stopPropagation(); move(i); }}>
      {#if source.group}
      <details open>
        <summary>{source.label || (source.group ? `来源组 ${i + 1}` : `来源 ${i + 1}`)}{source.group ? ` · ${source.group.length} 个来源` : ''}</summary>
        <div class="body">
          <div class="actions">
            <PixelButton size="sm" disabled={disabled || sources.length === 1} onclick={() => sources = sources.filter((_, j) => i !== j)}>删除{source.group ? '来源组' : '来源'}</PixelButton>
          </div>
            <label>组名<PixelInput bind:value={source.label} {disabled} placeholder="可选" /></label>
            {#each source.group as member, j (member)}
              <div class="member" class:dimmed={dragging?.index === i && dragging?.child === j}>
                <div class="member-title">{member.label || `来源 ${j + 1}`}</div>
                <div class="body">
                  <div class="actions">
                    <button type="button" class="handle" draggable={!disabled} {disabled} aria-label="拖动组内来源 {j + 1}" ondragstart={event => { dragging = { index: i, child: j }; event.dataTransfer?.setData('text/plain', `${i}:${j}`); }} ondragend={() => { dragging = null; over = null; }}><PixelIcon name="grip" size={12} /> 拖动</button>
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
          <button type="button" class="handle" draggable={!disabled} {disabled} aria-label="拖动来源 {i + 1}" ondragstart={event => { dragging = { index: i }; event.dataTransfer?.setData('text/plain', String(i)); }} ondragend={() => { dragging = null; over = null; }}><PixelIcon name="grip" size={12} /></button>
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
  .over { outline: 2px dashed var(--accent); outline-offset: 2px; background: var(--surface-2); }
  .dimmed { opacity: 0.5; }
  summary { cursor: pointer; padding: 12px; font-size: 13px; overflow-wrap: anywhere; }
  .body { padding: 0 12px 12px; }
  .plain { display: grid; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--border); }
  .plain strong { margin-right: auto; font-size: 13px; }
  .member-title { padding: 10px 12px; font-size: 13px; }
  .member { border-bottom: 1px solid var(--border); }
  .processing { border: 1px solid var(--border); border-radius: 3px; min-width: 0; }
  .inherit-hint { margin: 0 0 8px; color: var(--sec); font-size: 12px; }
  .inherit-reset { padding: 0 2px; border: 0; background: none; color: var(--text); font: inherit; text-decoration: underline; cursor: pointer; }
  .inherit-reset:disabled { opacity: .45; cursor: not-allowed; }
  label { display: grid; gap: 6px; font-size: 13px; }
  .move-control { display: flex; align-items: center; gap: 6px; }
  .move-label { color: var(--sec); white-space: nowrap; }
  .move-control :global(.pixel-select) { width: 160px; flex-shrink: 0; }
  .handle { display: inline-flex; align-items: center; gap: 4px; cursor: grab; color: var(--sec); background: var(--surface); border: 1px solid var(--border-vis); border-radius: 3px; padding: 5px 8px; font: inherit; }
  .handle:disabled { opacity: .45; cursor: not-allowed; }
  .drop-out { padding: 16px; text-align: center; color: var(--sec); border: 2px dashed var(--border-vis); border-radius: 4px; background: var(--surface-2); }
</style>
