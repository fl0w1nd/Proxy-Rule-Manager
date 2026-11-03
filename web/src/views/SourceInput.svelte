<script lang="ts">
  import { onDestroy } from 'svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelMenu from '../components/pixel/PixelMenu.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import CodeEditor from '../components/CodeEditor.svelte';
  import type { LintReport } from '../api/client';
  import { createLinter, diagnosticReason, documentDiagnostics, editorDiagnostics } from '../utils/ruleSyntax';
  import { emptySource, sourceKinds, geositeProviders, geoipProviders, type RuleSource, type SourceKind } from './rules';

  type MenuItem = { label: string; onselect: () => void; disabled?: boolean; danger?: boolean };

  let {
    source = $bindable(),
    label,
    disabled = false,
    fileOptions,
    refOptions,
    errors = [],
    menu = [],
    draggable = false,
    ondragstart,
    ondragend,
    oneditfile,
    onpickgeo,
  }: {
    source: RuleSource;
    /** 无障碍名称前缀，如「来源 1」或「来源 1 输入 2」。 */
    label: string;
    disabled?: boolean;
    fileOptions: { value: string; label: string }[];
    refOptions: { value: string; label: string }[];
    errors?: string[];
    menu?: MenuItem[];
    /** 是否显示拖动手柄（多输入来源内的输入才能拖到别的来源）。 */
    draggable?: boolean;
    ondragstart?: (event: DragEvent) => void;
    ondragend?: () => void;
    /** 打开本地文件编辑器；name 为空表示新建，assign 用于回填新文件名。 */
    oneditfile?: (name: string, assign: (name: string) => void) => void;
    onpickgeo?: (source: RuleSource) => void;
  } = $props();
  const id = $props.id();

  let lint = $state<LintReport | null>(null);
  const linter = createLinter((report) => { lint = report; }, 500);
  onDestroy(() => linter.cancel());
  // 内联文本按原始内容实时检查语法，和本地文件编辑器使用同一套解析器。
  $effect(() => {
    if (source.kind !== 'content' || !source.content?.trim()) { linter.cancel(); lint = null; return; }
    linter.run(source.content);
  });

  function changeKind(kind: string) {
    const next: RuleSource = { ...emptySource(kind as SourceKind), ops: source.ops ?? [] };
    if (source.label) next.label = source.label;
    // 预处理只对文本来源生效，切到引用或 Geo 类型时去掉，避免留下无效配置。
    if (['url', 'file', 'content'].includes(kind) && source.preprocess !== undefined) next.preprocess = source.preprocess;
    if (kind === 'geosite' || kind === 'geoip') next.provider = (kind === 'geosite' ? geositeProviders : geoipProviders)[0].value;
    source = next;
  }
</script>

<div class="source-input" class:invalid={errors.length > 0}>
  <div class="line">
    {#if draggable}
      <button type="button" class="handle" draggable={!disabled} {disabled} aria-label="拖动{label}" {ondragstart} {ondragend}><PixelIcon name="grip" size={12} /></button>
    {/if}
    <div class="kind">
      <PixelSelect id="{id}-kind" label="{label} 类型" size="sm" options={[...sourceKinds]} value={source.kind} {disabled} onchange={changeKind} />
    </div>
    <div class="value">
      {#if source.kind === 'url'}
        <PixelInput aria-label="{label} URL" {disabled} bind:value={source.url} placeholder="https://" spellcheck="false" />
      {:else if source.kind === 'file'}
        <div class="with-action">
          <PixelSelect id="{id}-file" label="{label} 文件" size="sm" options={fileOptions} value={source.file ?? ''} {disabled} onchange={(value) => { source.file = value; }} />
          {#if oneditfile}
            {#if source.file}
              <PixelButton size="sm" {disabled} onclick={() => oneditfile(source.file ?? '', (name) => { source.file = name; })}>编辑</PixelButton>
            {:else}
              <PixelButton size="sm" {disabled} onclick={() => oneditfile('', (name) => { source.file = name; })}>新建</PixelButton>
            {/if}
          {/if}
        </div>
      {:else if source.kind === 'ref'}
        <PixelSelect id="{id}-ref" label="{label} 引用" size="sm" options={refOptions} value={source.ref ?? ''} {disabled} onchange={(value) => { source.ref = value; }} />
      {:else if source.kind === 'geosite' || source.kind === 'geoip'}
        {@const providers = source.kind === 'geosite' ? geositeProviders : geoipProviders}
        <div class="geo">
          <PixelSelect id="{id}-provider" label="{label} 提供商" size="sm" options={[{ value: '', label: '提供商' }, ...providers]} value={source.provider ?? ''} {disabled} onchange={(value) => { source.provider = value; }} />
          <PixelInput aria-label="{label} 列表" {disabled} bind:value={source.list} placeholder={source.kind === 'geosite' ? 'google' : 'cn'} spellcheck="false" />
          {#if source.kind === 'geosite'}
            <PixelInput aria-label="{label} 属性" {disabled} bind:value={source.attrs} placeholder="属性，可选" spellcheck="false" />
          {/if}
          {#if onpickgeo}<PixelButton size="sm" {disabled} onclick={() => onpickgeo(source)}>目录</PixelButton>{/if}
        </div>
      {:else}
        <span class="content-hint">在下方编辑内联规则</span>
      {/if}
    </div>
    <div class="note">
      <PixelInput aria-label="{label} 备注" {disabled} bind:value={source.label} placeholder="备注" />
    </div>
    {#if menu.length}<PixelMenu label="{label} 操作" items={menu} {disabled} />{/if}
  </div>
  {#if source.kind === 'content'}
    <div class="content">
      <CodeEditor value={source.content ?? ''} label="{label} 内联文本" filename="inline.list" height="140px"
        placeholder="DOMAIN-SUFFIX,example.com" readonly={disabled} diagnostics={editorDiagnostics(lint)}
        onchange={(value) => { source.content = value; }} />
      {#if lint && lint.diagnostic_count}
        <p class="lint" role="status">
          ⚠ {lint.diagnostic_count} 行无法解析：{[...documentDiagnostics(lint), ...lint.diagnostics.filter((item) => item.line).slice(0, 3).map((item) => `第 ${item.line} 行 ${diagnosticReason(item.reason)}`)].join('；')}
        </p>
      {:else if lint}
        <p class="lint ok" role="status">{lint.entries} 条规则</p>
      {/if}
    </div>
  {/if}
  {#each errors as error}<p class="error">{error}</p>{/each}
</div>

<style>
  .source-input { display: grid; gap: 6px; min-width: 0; }
  .line { display: flex; align-items: center; gap: 6px; min-width: 0; }
  .kind { width: 104px; flex-shrink: 0; }
  .value { flex: 1 1 auto; min-width: 0; }
  .note { width: 110px; flex-shrink: 0; }
  .with-action, .geo { display: flex; align-items: center; gap: 6px; min-width: 0; }
  .with-action > :global(.pixel-select) { flex: 1; min-width: 0; }
  .geo > :global(.pixel-select) { width: 112px; flex-shrink: 0; }
  .geo > :global(.pixel-field) { flex: 1; min-width: 60px; }
  .content-hint { color: var(--dim); font: 12px/20px var(--font-ui); }
  .content { display: grid; gap: 4px; min-width: 0; }
  .lint { margin: 0; color: var(--error-border); font: 12px/18px var(--font-ui); overflow-wrap: anywhere; }
  .lint.ok { color: var(--sec); }
  .error { margin: 0; color: var(--error-border); font: 12px/18px var(--font-ui); }
  .invalid .line :global(.pixel-field), .invalid .line :global(.pixel-select button) { border-color: var(--error-border); }
  .handle { display: inline-flex; align-items: center; justify-content: center; width: 24px; height: 28px; flex-shrink: 0; padding: 0; cursor: grab; color: var(--sec); background: var(--surface); border: 1px solid var(--border-vis); border-radius: 3px; }
  .handle:disabled { opacity: .45; cursor: not-allowed; }
  @media (max-width: 640px) {
    .line { flex-wrap: wrap; }
    .value { flex-basis: 100%; order: 3; }
    .note { flex: 1; }
    .geo { flex-wrap: wrap; }
  }
</style>
