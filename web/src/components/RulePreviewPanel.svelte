<script lang="ts">
  import { type RulePreview, type RulePreviewSource } from '../api/client';
  import PixelTabs from './pixel/PixelTabs.svelte';
  import CodePanel from './CodePanel.svelte';
  import { retroScroll } from '../utils/scrollbars';

  interface Props {
    preview: RulePreview;
  }

  let { preview }: Props = $props();

  let previewClient = $state('');
  let previewTab = $state('');

  const previewOutputs = $derived(preview?.outputs ?? []);
  const isBinaryOutput = (item: { binary?: boolean }) => item.binary === true;
  const previewClients = $derived([...new Map(previewOutputs.map(item => [item.client_id, {
    value: item.client_id,
    label: item.client_name,
  }])).values()]);
  const previewFormats = $derived(previewOutputs.filter(item => item.client_id === previewClient));

  $effect(() => {
    const first = preview.outputs.find((item) => !isBinaryOutput(item)) ?? preview.outputs[0];
    previewTab = first?.id ?? '';
    previewClient = first?.client_id ?? '';
  });

  function durationLabel(ms: number) {
    if (ms < 1000) return `${ms} ms`;
    return `${(ms / 1000).toFixed(2)} s`;
  }

  // 预览只读本地数据，所以把这份数据的抓取时间摆出来，便于判断新旧。
  function cacheLabel(source: RulePreviewSource): string {
    if (!source.cache_fetched_at) return '';
    const at = new Date(source.cache_fetched_at);
    if (Number.isNaN(at.getTime())) return '';
    const parts = [
      `数据 ${at.toLocaleString('zh-CN', { hour12: false })}`,
      source.cache_version ? `v${source.cache_version}` : '',
    ].filter(Boolean);
    return parts.join(' · ');
  }
</script>

<div class="preview-report">
  <p class="preview-meta">{preview.rule_name} · {preview.elapsed_ms} ms · 合并 {preview.merged} 条</p>
  <div class="preview-sources">
    {#each preview.sources as source}
      <div class="preview-source">
        <strong>{source.label}</strong>
        {#each source.details ?? [] as detail}<span class="source-detail">{detail}</span>{/each}
        <span>{source.type} · {source.entries} 条 · {durationLabel(source.duration_ms)}</span>
        {#if cacheLabel(source)}<span class="source-cache">{cacheLabel(source)}</span>{/if}
        {#if source.error}<em>{source.error}</em>{/if}
      </div>
    {/each}
  </div>
  {#if preview.ops_error}
    <div class="notice error">{preview.ops_error}</div>
  {/if}
  <p class="preview-diff">过滤 {preview.pre_ops} → {preview.post_ops}，新增 {preview.ops_diff.added}，移除 {preview.ops_diff.removed}</p>
  {#if preview.ops_diff.groups?.length}
    <ul class="diff-samples">
      {#each preview.ops_diff.groups as group}
        {#each group.added ?? [] as item}<li class="add">+ {item}</li>{/each}
        {#each group.removed ?? [] as item}<li class="del">− {item}</li>{/each}
      {/each}
    </ul>
  {/if}
  {#if previewOutputs.length}
    <PixelTabs id="rule-preview-clients" label="输出客户端" items={previewClients} value={previewClient} onchange={value => { previewClient = value; previewTab = previewOutputs.find(item => item.client_id === value && !isBinaryOutput(item))?.id ?? previewOutputs.find(item => item.client_id === value)?.id ?? ''; }} />
    <div role="tabpanel" id="rule-preview-clients-panel-{previewClient}" aria-labelledby="rule-preview-clients-tab-{previewClient}" class="preview-formats">
    <PixelTabs id="rule-preview-outputs" label="客户端格式" items={previewFormats.map((item) => ({ value: item.id, label: item.name || item.id }))} bind:value={previewTab} />
    {#each previewFormats as item (item.id)}
      {#if previewTab === item.id}
        <div role="tabpanel" id="rule-preview-outputs-panel-{item.id}" aria-labelledby="rule-preview-outputs-tab-{item.id}">
        {#if item.error}
          <div class="notice error">{item.error}</div>
        {:else if isBinaryOutput(item)}
          <p class="binary-note">二进制规则集无法以文本预览，可下载后导入客户端。</p>
        {:else}
          <CodePanel filename={item.id} stat={`${item.output?.split('\n').length ?? 0} LINES`}>
            <pre use:retroScroll>{item.output}{item.truncated ? '\n… 内容已截断' : ''}</pre>
          </CodePanel>
        {/if}
        </div>
      {/if}
    {/each}
    </div>
  {:else}
    <p class="empty-hint">未选择输出客户端，因此没有产物预览。</p>
  {/if}
</div>

<style>
  .preview-report { display: grid; gap: 12px; min-width: 0; }
  .preview-meta, .preview-diff { margin: 0; color: var(--sec); }
  .preview-sources { display: grid; gap: 6px; }
  .preview-source { display: grid; gap: 2px; padding: 8px 10px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 3px; }
  .preview-source em { color: var(--text); }
  .source-detail, .preview-source strong { overflow-wrap: anywhere; }
  .source-cache { color: var(--dim); font: 11px/17px var(--font-code); }
  .notice { padding: 10px 14px; background: var(--status-error); border: 1px solid var(--border-vis); border-radius: 4px; color: var(--text); overflow-wrap: anywhere; }
  .diff-samples { margin: 0; padding: 0; list-style: none; font: 12px/20px var(--font-code); }
  .diff-samples .add { color: var(--diff-add); }
  .diff-samples .del { color: var(--diff-remove); }
  .empty-hint { margin: 0; padding: 12px; background: var(--surface-2); border: 1px dashed var(--border); border-radius: 4px; font: 12px/18px var(--font-ui); color: var(--sec); }
  .binary-note { margin: 0; padding: 10px 14px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; color: var(--sec); font: 400 14px/22px var(--font-reading); }
  .preview-formats { display: grid; gap: 18px; min-width: 0; border: 1px solid var(--border); padding: 12px; border-radius: 3px; }
  .preview-report :global(.code-panel pre) { margin: 0; max-height: 280px; overflow: auto; padding: 10px 12px; color: var(--terminal-text); font: 13px/20px var(--font-code); white-space: pre-wrap; }
</style>
