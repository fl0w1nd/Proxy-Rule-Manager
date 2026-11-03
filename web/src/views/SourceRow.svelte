<script lang="ts">
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelMenu from '../components/pixel/PixelMenu.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import CodeEditor from '../components/CodeEditor.svelte';
  import OpsEditor from '../components/forms/OpsEditor.svelte';
  import SourceInput from './SourceInput.svelte';
  import type { RulePreview, RulePreviewSource } from '../api/client';
  import { diagnosticMessage } from '../utils/ruleSyntax';
  import {
    defaultPreprocessScript, hasTextInput, inputSummary, preprocessMode, sourceInputs, sourceKindLabel,
    type PreprocessMode, type RuleIssue, type RuleSource,
  } from './rules';

  type MenuItem = { label: string; onselect: () => void; disabled?: boolean; danger?: boolean };

  let {
    source = $bindable(),
    index,
    expanded = false,
    disabled = false,
    defaultPreprocess = '',
    issues = [],
    fileOptions,
    refOptions,
    menu,
    inputMenu,
    ontoggle,
    onaddinput,
    ondragstart,
    ondragend,
    oninputdragstart,
    oneditfile,
    onpickgeo,
    ontrial,
  }: {
    source: RuleSource;
    index: number;
    expanded?: boolean;
    disabled?: boolean;
    /** 规则级默认预处理脚本，用于「使用默认」的提示和「自定义」的初始内容。 */
    defaultPreprocess?: string;
    /** 属于这一行的校验错误。 */
    issues?: RuleIssue[];
    fileOptions: { value: string; label: string }[];
    refOptions: { value: string; label: string }[];
    menu: MenuItem[];
    /** 每个输入的 ⋯ 菜单。 */
    inputMenu: (input: number) => MenuItem[];
    ontoggle: () => void;
    onaddinput: () => void;
    ondragstart: (event: DragEvent) => void;
    ondragend: () => void;
    oninputdragstart: (event: DragEvent, input: number) => void;
    oneditfile?: (name: string, assign: (name: string) => void) => void;
    onpickgeo?: (source: RuleSource) => void;
    ontrial: (source: RuleSource) => Promise<RulePreview>;
  } = $props();

  const label = $derived(`来源 ${index + 1}`);
  const inputs = $derived(sourceInputs(source));
  const multi = $derived(!!source.group);
  const mode = $derived(preprocessMode(source));
  const showPreprocess = $derived(hasTextInput(source) || source.preprocess !== undefined);
  const summary = $derived(inputs.map(inputSummary).filter(Boolean));
  const rowIssues = $derived(issues.filter((issue) => issue.input === undefined).map((issue) => issue.message));

  let trialing = $state(false);
  let trial = $state<RulePreviewSource | null>(null);
  let trialMerged = $state<number | null>(null);
  let trialError = $state('');
  const hasChips = $derived(mode === 'custom' || (mode === 'off' && !!defaultPreprocess.trim()) || !!source.ops?.length || issues.length > 0 || !!trial || !!trialError);

  function inputLabel(j: number) {
    return multi ? `${label} 输入 ${j + 1}` : label;
  }
  function inputErrors(j: number) {
    return issues.filter((issue) => multi ? issue.input === j : issue.input === undefined).map((issue) => issue.message);
  }
  function setMode(next: PreprocessMode) {
    if (disabled || next === mode) return;
    if (next === 'inherit') source.preprocess = undefined;
    else if (next === 'off') source.preprocess = '';
    else source.preprocess = defaultPreprocess.trim() || defaultPreprocessScript;
  }
  async function runTrial() {
    if (trialing) return;
    trialing = true;
    trialError = '';
    try {
      const report = await ontrial($state.snapshot(source) as RuleSource);
      trial = report.sources[0] ?? null;
      trialMerged = report.merged;
    } catch (error) {
      trial = null;
      trialMerged = null;
      trialError = (error as Error).message;
    } finally { trialing = false; }
  }
  const modes: { value: PreprocessMode; label: string }[] = [
    { value: 'inherit', label: '使用默认' },
    { value: 'custom', label: '自定义' },
    { value: 'off', label: '关闭' },
  ];
</script>

<section class="source-row" class:expanded class:invalid={issues.length > 0} aria-label={label} data-source={index}>
  <div class="head">
    <button type="button" class="handle" draggable={!disabled} {disabled} aria-label="拖动{label}" {ondragstart} {ondragend}><PixelIcon name="grip" size={12} /></button>
    <button type="button" class="toggle" aria-expanded={expanded} aria-label="{label}：{summary.join('、') || '未配置'}" onclick={ontoggle}>
      <span class="kind">{multi ? `${inputs.length} 个输入` : sourceKindLabel(source.kind)}</span>
      {#if source.label}<strong class="name">{source.label}</strong>{/if}
      <span class="summary">{summary.join(' · ')}</span>
    </button>
    {#if hasChips}
      <span class="chips">
        {#if mode === 'custom'}<span class="chip">脚本</span>{:else if mode === 'off' && defaultPreprocess.trim()}<span class="chip">不预处理</span>{/if}
        {#if source.ops?.length}<span class="chip">过滤 {source.ops.length}</span>{/if}
        {#if issues.length}<span class="chip error">{issues.length} 个问题</span>
        {:else if trialError || trial?.error}<span class="chip error">✗ 失败</span>
        {:else if trial?.diagnostics}<span class="chip warn">⚠ {trial.diagnostics} 行</span>
        {:else if trial}<span class="chip ok">✓ {(trialMerged ?? trial.entries).toLocaleString()}</span>{/if}
      </span>
    {/if}
    <PixelMenu label="{label} 操作" items={menu} {disabled} />
  </div>
  {#if expanded}
    <div class="body">
      <div class="inputs" role="group" aria-label="{label} 输入">
        {#each inputs as _input, j (j)}
          {#if multi}
            <SourceInput bind:source={source.group![j]} label={inputLabel(j)} {disabled} {fileOptions} {refOptions} errors={inputErrors(j)}
              menu={inputMenu(j)} draggable ondragstart={(event) => oninputdragstart(event, j)} {ondragend} {oneditfile} {onpickgeo} />
          {:else}
            <SourceInput bind:source label={inputLabel(j)} {disabled} {fileOptions} {refOptions} errors={inputErrors(j)} {oneditfile} {onpickgeo} />
          {/if}
        {/each}
        <button type="button" class="add-input" {disabled} onclick={onaddinput}>+ 添加输入<span>（与上面的输入合并后统一处理）</span></button>
      </div>
      {#if multi}
        {#each rowIssues as issue}<p class="error">{issue}</p>{/each}
      {/if}

      {#if showPreprocess}
        <div class="setting">
          <span class="setting-label" id="{label}-preprocess">预处理</span>
          <div class="segmented" role="radiogroup" aria-labelledby="{label}-preprocess">
            {#each modes as item (item.value)}
              <button type="button" role="radio" aria-checked={mode === item.value} {disabled} onclick={() => setMode(item.value)}>{item.label}</button>
            {/each}
          </div>
          <span class="hint">
            {#if mode === 'inherit'}{defaultPreprocess.trim() ? '使用「处理」页的默认脚本' : '未配置默认脚本，不做预处理'}
            {:else if mode === 'off'}不做预处理{:else}仅作用于本来源{/if}
          </span>
        </div>
        {#if mode === 'custom'}
          <CodeEditor value={source.preprocess ?? ''} language="javascript" label="{label} 预处理脚本" filename="process.js" height="180px"
            placeholder={'function process(content) { return content; }'} readonly={disabled}
            onchange={(value) => { source.preprocess = value; }} />
        {/if}
      {/if}

      <details class="setting-block" open={!!source.ops?.length}>
        <summary>过滤 · {source.ops?.length ?? 0} 项</summary>
        <div class="ops"><OpsEditor bind:value={source.ops!} {disabled} /></div>
      </details>

      <div class="trial">
        <PixelButton size="sm" disabled={disabled || trialing} onclick={runTrial}>{trialing ? '运行中…' : '试运行'}</PixelButton>
        <div class="trial-result" role="status" aria-label="{label} 试运行结果">
          {#if trialError}
            <span class="bad">{trialError}</span>
          {:else if trial}
            {#if trial.error}
              <span class="bad">{trial.error}</span>
            {:else}
              <span>{(trialMerged ?? trial.entries).toLocaleString()} 条{source.ops?.length ? '（过滤后）' : ''} · {trial.duration_ms} ms</span>
              {#if trial.diagnostics}
                <span class="bad">⚠ {trial.diagnostics} 行无法解析</span>
                <ul>
                  {#each (trial.diagnostic_items ?? []).slice(0, 5) as item}<li><code>{item.text}</code> {diagnosticMessage(item)}</li>{/each}
                </ul>
              {/if}
            {/if}
          {:else}
            <span class="hint">只抓取并解析这一个来源，不保存</span>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</section>

<style>
  .source-row { border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface); min-width: 0; }
  .source-row.invalid { border-color: var(--error-border); }
  .head { display: flex; align-items: center; gap: 8px; padding: 6px 8px; min-width: 0; }
  .handle { display: inline-flex; align-items: center; justify-content: center; width: 24px; height: 28px; flex-shrink: 0; padding: 0; cursor: grab; color: var(--sec); background: none; border: 1px solid transparent; border-radius: 3px; }
  .handle:hover:not(:disabled) { border-color: var(--border-vis); }
  .handle:disabled { opacity: .45; cursor: not-allowed; }
  .toggle { display: flex; align-items: baseline; gap: 8px; flex: 1 1 auto; min-width: 0; padding: 4px 2px; border: 0; background: none; color: var(--text); font: 13px/20px var(--font-ui); text-align: left; cursor: pointer; }
  .toggle:hover .summary { color: var(--text); }
  .kind { flex-shrink: 0; padding: 0 6px; border: 1px solid var(--border); border-radius: 3px; color: var(--sec); font-size: 11px; line-height: 18px; }
  .name { flex-shrink: 0; font-weight: 400; color: var(--display); max-width: 30%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .summary { min-width: 0; color: var(--sec); font-family: var(--font-code); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .chips { display: flex; gap: 4px; flex-shrink: 0; }
  .chip { padding: 0 6px; border-radius: 3px; background: var(--surface-2); color: var(--sec); font: 11px/18px var(--font-ui); white-space: nowrap; }
  .chip.ok { color: var(--text); background: var(--status-success); }
  .chip.warn { color: var(--text); background: var(--status-warning); }
  .chip.error { color: var(--text); background: var(--status-error); }
  .body { display: grid; gap: 12px; padding: 4px 12px 12px; border-top: 1px solid var(--border); min-width: 0; }
  .inputs { display: grid; gap: 10px; padding-top: 8px; min-width: 0; }
  .add-input { justify-self: start; padding: 0; border: 0; background: none; color: var(--text); font: 12px/18px var(--font-ui); text-decoration: underline; cursor: pointer; }
  .add-input span { color: var(--dim); text-decoration: none; display: inline-block; margin-left: 4px; }
  .add-input:disabled { opacity: .45; cursor: not-allowed; }
  .setting { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 12px; }
  .setting-label { color: var(--sec); font: 12px/20px var(--font-ui); width: 48px; }
  .segmented { display: inline-flex; border: 1px solid var(--border-vis); border-radius: 3px; overflow: hidden; }
  .segmented button { padding: 2px 10px; border: 0; border-right: 1px solid var(--border-vis); background: var(--surface-2); color: var(--sec); font: 12px/20px var(--font-ui); cursor: pointer; }
  .segmented button:last-child { border-right: 0; }
  .segmented button[aria-checked='true'] { background: var(--selected); color: var(--selected-text); }
  .segmented button:disabled { cursor: not-allowed; opacity: .6; }
  .hint { color: var(--dim); font: 12px/18px var(--font-ui); }
  .setting-block summary { cursor: pointer; color: var(--sec); font: 12px/20px var(--font-ui); }
  .ops { padding-top: 8px; }
  .trial { display: flex; align-items: flex-start; gap: 10px; flex-wrap: wrap; padding-top: 4px; border-top: 1px dashed var(--border); padding-top: 10px; }
  .trial-result { display: grid; gap: 2px; flex: 1; min-width: 0; color: var(--sec); font: 12px/18px var(--font-ui); }
  .trial-result ul { margin: 2px 0 0; padding-left: 16px; }
  .trial-result code { font: 12px/18px var(--font-code); color: var(--text); margin-right: 6px; }
  .bad { color: var(--error-border); overflow-wrap: anywhere; }
  .error { margin: 0; color: var(--error-border); font: 12px/18px var(--font-ui); }
  @media (max-width: 640px) {
    .head { flex-wrap: wrap; }
    .toggle { flex-basis: calc(100% - 80px); }
    .chips { order: 4; flex-basis: 100%; padding-left: 32px; flex-wrap: wrap; }
  }
</style>
