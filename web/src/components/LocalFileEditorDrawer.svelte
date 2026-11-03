<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte';
  import { autocompletion } from '@codemirror/autocomplete';
  import { api, APIRequestError, type LintReport, type LocalFileDetail, type RuleSyntax } from '../api/client';
  import CodeEditor from './CodeEditor.svelte';
  import RuleInsertBar from './RuleInsertBar.svelte';
  import PixelTabs from './pixel/PixelTabs.svelte';
  import PixelTable from './pixel/PixelTable.svelte';
  import PixelButton from './pixel/PixelButton.svelte';
  import PixelDialog from './pixel/PixelDialog.svelte';
  import PixelDrawer from './pixel/PixelDrawer.svelte';
  import PixelInput from './pixel/PixelInput.svelte';
  import rulesIcon from '../assets/icons/nav/rules.svg';
  import { localFileLanguage, referencedRuleLabel, validateLocalFileName } from '../views/localFiles';
  import {
    createLinter, diagnosticMessage, diagnosticReason, documentDiagnostics, editorDiagnostics,
    fallbackRuleSyntax, formatLabel, loadRuleSyntax, ruleCompletion,
  } from '../utils/ruleSyntax';

  /**
   * 本地规则文件编辑抽屉：带语法检查、插入栏、自动补全和解析结果视图。
   * 本地文件页和规则编辑器的「本地文件」输入共用它。
   */
  let {
    open = $bindable(false),
    name = '',
    rules = [],
    onsaved,
    onstatechange,
  }: {
    open: boolean;
    /** 要编辑的文件；为空时新建。只在打开时读取。 */
    name?: string;
    /** 引用此文件的规则，用于保存确认提示。 */
    rules?: { id: string; name: string }[];
    onsaved?: (detail: LocalFileDetail, created: boolean) => void;
    onstatechange?: (dirty: boolean, busy: boolean) => void;
  } = $props();

  let creating = $state(false);
  let draftName = $state('');
  let draftContent = $state('');
  let savedContent = $state('');
  let nameError = $state('');
  let editorLoading = $state(false);
  let busy = $state(false);
  let message = $state('');
  let success = $state(false);
  let discardOpen = $state(false);
  let requestId = 0;

  let editor = $state<CodeEditor>();
  let editorTab = $state<'edit' | 'parsed'>('edit');
  let syntax = $state<RuleSyntax>(fallbackRuleSyntax);
  let lint = $state<LintReport | null>(null);
  let lintError = $state('');
  let saveConfirmOpen = $state(false);
  let saveConfirmReport = $state<LintReport | null>(null);
  const linter = createLinter((report, error) => {
    lint = report;
    lintError = error ?? '';
  });
  const completion = autocompletion({ override: [ruleCompletion(() => syntax)] });

  const editorOpen = $derived(open);
  const editorLanguage = $derived(localFileLanguage(draftName));
  const isYAML = $derived(lint?.format ? lint.format === 'mihomo-yaml' : editorLanguage === 'yaml');
  const lintDiagnostics = $derived(editorDiagnostics(lint));
  const lintDocumentIssues = $derived(documentDiagnostics(lint));
  const editingRules = $derived(rules);
  const parsedTabs = $derived([
    { value: 'edit', label: '编辑' },
    { value: 'parsed', label: lint ? `解析结果 · ${lint.entries}` : '解析结果' },
  ]);
  const dirty = $derived(open && (creating ? draftName.trim() !== '' || draftContent !== '' : draftContent !== savedContent));

  $effect(() => {
    if (!open || editorLoading) { linter.cancel(); return; }
    linter.run(draftContent, draftName.trim());
  });
  $effect(() => { onstatechange?.(dirty, busy || editorLoading); });
  $effect(() => {
    if (open) untrack(() => { void start(name); });
  });
  onMount(() => { void loadRuleSyntax().then((loaded) => { syntax = loaded; }); });
  onDestroy(() => { requestId++; linter.cancel(); });

  async function start(file: string) {
    const id = ++requestId;
    reset();
    creating = !file;
    draftName = file;
    if (!file) return;
    editorLoading = true;
    try {
      const detail = await api.getLocalFile(file);
      if (id !== requestId) return;
      draftContent = detail.content;
      savedContent = detail.content;
    } catch (error) {
      if (id !== requestId) return;
      failed(error);
    } finally {
      if (id === requestId) editorLoading = false;
    }
  }

  function reset() {
    creating = false;
    draftName = '';
    draftContent = '';
    savedContent = '';
    nameError = '';
    message = '';
    editorLoading = false;
    editorTab = 'edit';
    lint = null;
    lintError = '';
    saveConfirmReport = null;
    linter.cancel();
  }

  function failed(error: unknown) {
    success = false;
    message = (error as Error).message;
  }

  function requestClose() {
    if (busy) return false;
    if (dirty) {
      discardOpen = true;
      return false;
    }
    return true;
  }

  function closeEditor() {
    requestId++;
    open = false;
    reset();
  }

  async function save(force = false) {
    if (busy || editorLoading) return;
    const fileName = creating ? draftName.trim() : draftName;
    if (creating) {
      nameError = validateLocalFileName(draftName) ?? '';
      if (nameError) return;
    }
    busy = true;
    message = '';
    if (!force) {
      // 保存前按当前内容再校验一次，避免依据过期的防抖结果放行。
      let report: LintReport | null = null;
      try { report = await api.lintRules(draftContent, fileName); } catch { /* 校验不可用时不阻塞保存 */ }
      if (report) lint = report;
      if (report && report.diagnostic_count > 0) {
        busy = false;
        saveConfirmReport = report;
        saveConfirmOpen = true;
        return;
      }
    }
    try {
      const created = creating;
      const detail = created
        ? await api.createLocalFile(fileName, draftContent)
        : await api.saveLocalFile(fileName, draftContent);
      creating = false;
      saveConfirmReport = null;
      draftName = detail.name;
      savedContent = detail.content;
      success = true;
      message = `已保存 ${detail.name}`;
      onsaved?.(detail, created);
    } catch (error) {
      failed(error);
      if (error instanceof APIRequestError && error.code === 'invalid_filename') {
        nameError = '文件名不能包含路径，且需以 .list、.yaml 或 .txt 结尾';
      }
    } finally { busy = false; }
  }

  function jumpTo(line?: number) {
    if (!line) return;
    editorTab = 'edit';
    queueMicrotask(() => editor?.focusLine(line));
  }
</script>

<PixelDrawer
  bind:open
  title={creating ? '新建本地文件' : draftName || '本地文件'}
  icon={rulesIcon}
  width="820px"
  scrollable={false}
  onrequestclose={requestClose}
  onclose={closeEditor}
>
  <div class="drawer-editor-form">
    {#if editorLoading}
      <p class="editor-status" role="status">正在读取文件…</p>
    {:else}
      {#if creating}
        <label class="field" for="local-file-name">
          <span>文件名</span>
          <PixelInput id="local-file-name" error={!!nameError} bind:value={draftName}
            placeholder="my-direct.list" spellcheck="false" oninput={() => { nameError = ''; }} />
          {#if nameError}<span class="field-error">{nameError}</span>{/if}
        </label>
      {/if}
      {#if editorOpen}
        <PixelTabs id="local-file-view" label="文件视图" items={parsedTabs} value={editorTab} onchange={(value) => { editorTab = value as 'edit' | 'parsed'; }} />
        <div class="editor-pane" class:hidden={editorTab !== 'edit'}>
          <RuleInsertBar {syntax} yaml={isYAML} disabled={busy} oninsert={(line) => editor?.insertLine(line)} />
          <div class="editor-field">
            <CodeEditor bind:this={editor} fill filename={draftName} language={editorLanguage} label="文件内容" value={draftContent} readonly={busy}
              diagnostics={lintDiagnostics} extensions={[completion]} onchange={(value) => { draftContent = value; }} />
          </div>
        </div>
        {#if editorTab === 'parsed'}
          <div class="parsed-pane" role="tabpanel" aria-label="解析结果">
            {#if !lint}
              <p class="editor-status">{lintError ? `校验不可用：${lintError}` : '正在解析…'}</p>
            {:else if lint.lines.length === 0}
              <p class="editor-status">没有解析出任何规则。</p>
            {:else}
              {#if lint.kinds.length}
                <p class="kinds">{#each lint.kinds as kind (kind.kind)}<span>{kind.kind} · {kind.count}</span>{/each}</p>
              {/if}
              <div class="parsed-table">
                <PixelTable minWidth="480px">
                  <thead><tr><th style="width: 64px;" class="num">行</th><th style="width: 30%;">类型</th><th>规范化后的值</th></tr></thead>
                  <tbody>
                    {#each lint.lines as line (line.line)}
                      <tr class="parsed-row" onclick={() => jumpTo(line.line)} title={line.text}>
                        <td class="num">{line.line}</td>
                        <td class="font-code">{line.kind}</td>
                        <td class="font-code value-cell">{line.value}</td>
                      </tr>
                    {/each}
                  </tbody>
                </PixelTable>
              </div>
              {#if lint.lines_truncated}<p class="editor-status">仅显示前 {lint.lines.length.toLocaleString()} 条，共 {lint.entries.toLocaleString()} 条。</p>{/if}
            {/if}
          </div>
        {/if}
        <div class="lint-status" role="status" aria-label="语法检查" title="按文件原始内容检查，不包含规则里配置的预处理脚本">
          {#if lint}
            <span>{formatLabel(lint.format)}</span>
            <span>{lint.entries.toLocaleString()} 条</span>
            {#if lint.diagnostic_count}
              <button type="button" class="lint-error" onclick={() => jumpTo(lint?.diagnostics.find((item) => item.line)?.line)}>
                ⚠ {lint.diagnostic_count} 行无法解析
              </button>
            {:else}
              <span class="lint-ok">✓ 语法正确</span>
            {/if}
            {#each lint.warnings as warning}<span class="lint-warning">{warning}</span>{/each}
          {:else if lintError}
            <span class="text-sec">语法检查不可用：{lintError}</span>
          {:else}
            <span class="text-sec">正在检查…</span>
          {/if}
        </div>
        {#if lint && (lintDocumentIssues.length || lintDiagnostics.length)}
          <ul class="lint-list" aria-label="无法解析的行">
            {#each lintDocumentIssues as issue}<li>{issue}</li>{/each}
            {#each lint.diagnostics.filter((item) => item.line).slice(0, 50) as item (item.line)}
              <li><button type="button" onclick={() => jumpTo(item.line)}><span class="font-code">{item.line}</span> {item.text} — {diagnosticReason(item.reason)}</button></li>
            {/each}
          </ul>
        {/if}
      {/if}
    {/if}
    {#if message && editorOpen}
      <div class="files-message editor-message" class:success role={success ? 'status' : 'alert'}>{message}</div>
    {/if}
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestClose()) closeEditor(); }}>取消</PixelButton>
    <PixelButton variant="primary" disabled={busy || editorLoading || (!creating && !dirty)} onclick={() => save()}>
      {busy ? '保存中…' : '保存'}
    </PixelButton>
  {/snippet}
</PixelDrawer>

<PixelDialog bind:open={discardOpen} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger
  onconfirm={closeEditor}>
  当前修改尚未保存，关闭后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={saveConfirmOpen} title="文件有无法解析的行" confirmLabel="仍然保存" cancelLabel="继续编辑" danger
  oncancel={() => { jumpTo(saveConfirmReport?.diagnostics.find((item) => item.line)?.line); saveConfirmReport = null; }}
  onconfirm={() => { void save(true); }}>
  {#if saveConfirmReport}
    <p class="confirm-text">共 {saveConfirmReport.diagnostic_count} 行无法解析，例如：{diagnosticMessage(saveConfirmReport.diagnostics[0])}。</p>
    {#if editingRules.length}
      <p class="confirm-text">引用此文件的规则 {referencedRuleLabel(editingRules)} 会在更新时报错，下游引用它们的规则也无法编译。</p>
    {:else}
      <p class="confirm-text">目前没有规则引用此文件，但被引用后会导致规则更新失败。</p>
    {/if}
  {/if}
</PixelDialog>

<style>
  .files-message {
    padding: 10px 14px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--status-error);
    color: var(--text);
    overflow-wrap: anywhere;
  }
  .files-message.success { background: var(--status-success); }
  .editor-message { flex-shrink: 0; margin: 0; max-height: 120px; overflow-y: auto; }
  .text-sec { color: var(--sec); }
  .drawer-editor-form {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    height: 100%;
    gap: 12px;
  }
  .editor-status { margin: 0; color: var(--sec); }
  .field { display: flex; flex-direction: column; gap: 6px; flex-shrink: 0; }
  .field span { color: var(--display); font: 400 12px/20px var(--font-ui); }
  .field-error { color: var(--error-border); }
  .editor-field { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 6px; }
  .editor-pane, .parsed-pane { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }
  .editor-pane.hidden { display: none; }
  .parsed-pane { overflow: auto; }
  .parsed-table { min-height: 0; }
  .parsed-row { cursor: pointer; }
  .value-cell { overflow-wrap: anywhere; }
  .kinds { margin: 0; display: flex; flex-wrap: wrap; gap: 6px 12px; color: var(--sec); font: 12px/18px var(--font-code); }
  .lint-status { flex-shrink: 0; display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; color: var(--sec); font: 12px/18px var(--font-ui); }
  .lint-ok { color: var(--text); }
  .lint-warning { color: var(--text); }
  .lint-error { padding: 0; border: 0; background: none; color: var(--error-border); font: inherit; text-decoration: underline; cursor: pointer; }
  .lint-list { flex-shrink: 0; margin: 0; padding: 6px 10px; max-height: 112px; overflow-y: auto; list-style: none; display: grid; gap: 2px;
    border: 1px solid var(--border); border-radius: 3px; background: var(--surface-2); font: 12px/18px var(--font-ui); color: var(--sec); }
  .lint-list button { padding: 0; border: 0; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; overflow-wrap: anywhere; }
  .lint-list button:hover { color: var(--text); text-decoration: underline; }
  .lint-list .font-code { color: var(--error-border); margin-right: 6px; }
  .confirm-text { margin: 0 0 8px; }
</style>
