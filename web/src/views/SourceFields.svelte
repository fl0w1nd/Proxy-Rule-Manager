<script lang="ts">
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import { emptySource, sourceKinds, geositeProviders, geoipProviders, type RuleSource, type SourceKind } from './rules';
  let { source = $bindable(), disabled = false, label = '1', fileOptions, refOptions }: {
    source: RuleSource; disabled?: boolean; label?: string;
    fileOptions: { value: string; label: string }[];
    refOptions: { value: string; label: string }[];
  } = $props();
  const i = $props.id();
</script>
<div class="fields">
  <div class="stack">
    类型
    <PixelSelect id="source-kind-{i}" label="来源 {label} 类型" options={[...sourceKinds]} value={source.kind} disabled={disabled} onchange={(kind) => source = { ...emptySource(kind as SourceKind), label: source.label, preprocess: ['url', 'file', 'content'].includes(kind) ? source.preprocess : undefined, ops: source.ops }} />
  </div>
  <label>备注<input {disabled} bind:value={source.label} placeholder="可选" /></label>
</div>
{#if source.kind === 'url'}
  <label class="stack">URL<input {disabled} bind:value={source.url} placeholder="https://" /></label>
{:else if source.kind === 'file'}
  <PixelSelect id="source-file-{i}" label="来源 {label} 文件" options={fileOptions} value={source.file ?? ''} disabled={disabled} onchange={(value) => { source.file = value; }} />
{:else if source.kind === 'content'}
  <label class="stack">内联文本<textarea {disabled} bind:value={source.content} rows="6" placeholder="DOMAIN,example.com"></textarea></label>
{:else if source.kind === 'ref'}
  <PixelSelect id="source-ref-{i}" label="来源 {label} 引用" options={refOptions} value={source.ref ?? ''} disabled={disabled} onchange={(value) => { source.ref = value; }} />
{:else if source.kind === 'geosite'}
  <div class="fields">
    <div class="stack">
      提供商
      <PixelSelect id="source-geosite-provider-{i}" label="Geosite 提供商" options={[{ value: '', label: '请选择提供商' }, ...geositeProviders]} value={source.provider ?? ''} disabled={disabled} onchange={(value) => { source.provider = value; }} />
    </div>
    <label>列表<input {disabled} bind:value={source.list} placeholder="google" /></label>
    <label class="full">属性<input {disabled} bind:value={source.attrs} placeholder="可选，例如 cn,ads" /></label>
  </div>
{:else}
  <div class="fields">
    <div class="stack">
      提供商
      <PixelSelect id="source-geoip-provider-{i}" label="GeoIP 提供商" options={[{ value: '', label: '请选择提供商' }, ...geoipProviders]} value={source.provider ?? ''} disabled={disabled} onchange={(value) => { source.provider = value; }} />
    </div>
    <label>列表<input {disabled} bind:value={source.list} placeholder="cn" /></label>
  </div>
{/if}
<style>
  .fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  label, .stack { display: grid; gap: 6px; font-size: 13px; }
  .full { grid-column: 1 / -1; }
  input, textarea { width: 100%; box-sizing: border-box; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface); color: var(--text); padding: 8px; }
  textarea { min-height: 100px; max-height: 260px; overflow: auto; resize: vertical; font-family: var(--font-code); }
  @media (max-width: 520px) { .fields { grid-template-columns: 1fr; } }
</style>
