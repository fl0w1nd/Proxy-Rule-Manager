<script lang="ts">
  import { onMount } from 'svelte';
  import { api, APIRequestError, type ConfigRawSnapshot, type ConfigValidationIssue } from '../api/client';
  import YamlEditor from '../components/YamlEditor.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';

  let { onstatechange, draft = '' }: { onstatechange?: (dirty: boolean, busy: boolean) => void; draft?: string } = $props();
  let snapshot = $state<ConfigRawSnapshot | null>(null);
  let text = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let validating = $state(false);
  let issues = $state<ConfigValidationIssue[]>([]);
  let message = $state('');
  let success = $state(false);
  let editor = $state<YamlEditor>();
  const dirty = $derived(snapshot !== null && text !== snapshot.yaml);
  $effect(() => { onstatechange?.(dirty, saving); });

  async function load() {
    loading = true;
    message = '';
    try {
      snapshot = await api.getConfigRaw();
      text = draft || snapshot.yaml;
    } catch (error) { message = `读取配置失败：${(error as Error).message}`; }
    finally { loading = false; }
  }
  onMount(() => { void load(); });

  function edited(value: string) {
    text = value;
    issues = [];
    message = '';
  }
  function failed(error: unknown) {
    success = false;
    message = (error as Error).message;
    if (error instanceof APIRequestError) {
      issues = error.details.errors ?? [];
      if (error.code === 'config_version_conflict') message += '。请复制当前修改，重新打开配置文件后合并。';
      if (error.code === 'config_dirty') message += '。请保留当前修改，通过外部配置提示重新加载。';
    }
  }
  async function validate() {
    if (validating || saving) return;
    const submitted = text;
    validating = true;
    message = '';
    issues = [];
    try {
      await api.validateConfig(submitted);
      if (text === submitted) { success = true; message = '配置校验通过'; }
    } catch (error) { if (text === submitted) failed(error); }
    finally { validating = false; }
  }
  async function save() {
    if (!snapshot || saving || validating || !dirty) return;
    const submitted = text;
    saving = true;
    message = '';
    issues = [];
    try {
      const result = await api.saveConfigRaw(submitted, snapshot.version);
      snapshot = { ...snapshot, yaml: submitted, version: result.version };
      success = true;
      message = ['配置已保存', ...result.warnings].join('；');
    } catch (error) { failed(error); }
    finally { saving = false; }
  }
</script>

<svelte:window onbeforeunload={event => { if (dirty || saving) { event.preventDefault(); event.returnValue = ''; } }} />

{#if loading}
  <div class="status" role="status">正在读取配置…</div>
{:else if !snapshot}
  <div class="status error" role="alert"><span>{message}</span><PixelButton onclick={load}>重试</PixelButton></div>
{:else}
  <section class="config-editor" aria-label="配置文件编辑器">
    <header><code>{snapshot.path}</code><span>版本 {snapshot.version}{dirty ? ' · 未保存' : ''}</span></header>
    <YamlEditor bind:this={editor} value={snapshot.yaml} {issues} readonly={saving} onchange={edited} />
    <footer>
      <PixelButton disabled={saving || validating} onclick={validate}>{validating ? '校验中…' : '校验'}</PixelButton>
      <PixelButton variant="primary" disabled={!dirty || saving || validating} onclick={save}>{saving ? '保存中…' : '保存配置'}</PixelButton>
    </footer>
    {#if message}<div class="message" class:success role={success ? 'status' : 'alert'}>{message}</div>{/if}
    {#if issues.length}
      <ul class="issues" aria-label="配置错误">
        {#each issues as issue}
          <li><button type="button" onclick={() => editor?.focusLine(issue.line ?? 0)}>
            {issue.line ? `第 ${issue.line} 行 · ` : ''}{issue.path}：{issue.message}
          </button></li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

<style>
  .config-editor { min-width: 0; overflow: hidden; border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface); box-shadow: var(--highlight-top), var(--shadow-panel); }
  header, footer { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; padding: 12px 16px; background: var(--surface-2); }
  header { justify-content: space-between; color: var(--sec); font: 400 12px/20px var(--font-ui); }
  code { font: 400 13px/20px var(--font-code); color: var(--text); overflow-wrap: anywhere; }
  footer { justify-content: flex-end; }
  .status { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface-2); }
  .error, .message { background: var(--status-error); }
  .message { padding: 12px 16px; color: var(--text); font: 400 12px/20px var(--font-ui); overflow-wrap: anywhere; }
  .success { background: var(--status-success); }
  .issues { margin: 0; padding: 8px 16px 12px 32px; max-height: 200px; overflow: auto; color: var(--error-border); }
  .issues button { border: 0; padding: 4px 0; background: transparent; color: inherit; text-align: left; cursor: pointer; overflow-wrap: anywhere; font: 400 13px/20px var(--font-code); }
  .issues button:hover { text-decoration: underline; }
</style>
