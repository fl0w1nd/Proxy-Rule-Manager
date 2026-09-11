<script lang="ts">
  import PixelButton from '../../components/pixel/PixelButton.svelte';
  import PixelTabs from '../../components/pixel/PixelTabs.svelte';
  import PixelDrawer from '../../components/pixel/PixelDrawer.svelte';
  import PixelCheckbox from '../../components/pixel/PixelCheckbox.svelte';
  import PixelSelect from '../../components/pixel/PixelSelect.svelte';
  import PixelInput from '../../components/pixel/PixelInput.svelte';
  import PixelTagInput from '../../components/pixel/PixelTagInput.svelte';
  import PixelTextarea from '../../components/pixel/PixelTextarea.svelte';
  import CodeEditor from '../../components/CodeEditor.svelte';
  import OpsEditor from '../../components/forms/OpsEditor.svelte';
  import RulePreviewPanel from '../../components/RulePreviewPanel.svelte';
  import SourceEditor from '../SourceEditor.svelte';
  import type { ClientConfig } from '../clients';
  import { mergeStrategies, type RuleConfig, type RuleIssue, type RuleSource } from '../rules';
  import type { RulePreview } from '../../api/client';
  import rulesIcon from '../../assets/icons/nav/rules.svg';

  interface Props {
    open: boolean;
    draft: RuleConfig;
    editorTab: string;
    drawerView: 'edit' | 'preview';
    editing: string;
    dirty: boolean;
    busy: boolean;
    previewing: boolean;
    preview: RulePreview | null;
    message: string;
    error: boolean;
    fieldError: RuleIssue | null;
    issues: RuleIssue[];
    clients: ClientConfig[];
    fileOptions: { value: string; label: string }[];
    refOptions: { value: string; label: string }[];
    resolveClientIcon: (client: ClientConfig) => string;
    formatLabel: (templateID?: string) => string;
    onrequestclose: () => boolean;
    onReject: (issue: RuleIssue) => void;
    onClearFieldError: (path: string) => void;
    onSave: () => void;
    onPreviewDraft: () => void;
  }

  let {
    open = $bindable(false),
    draft = $bindable(),
    editorTab = $bindable('props'),
    drawerView = $bindable<'edit' | 'preview'>('edit'),
    editing,
    dirty,
    busy,
    previewing,
    preview,
    message,
    error,
    fieldError,
    issues,
    clients,
    fileOptions,
    refOptions,
    resolveClientIcon,
    formatLabel,
    onrequestclose,
    onReject,
    onClearFieldError,
    onSave,
    onPreviewDraft,
  }: Props = $props();

  const alertPaths = $derived(new Set([...issues.map((item) => item.path), ...(fieldError ? [fieldError.path] : [])]));
  const editorTabs = $derived([
    { value: 'props', label: '属性', alert: alertPaths.has('id') || alertPaths.has('name') },
    { value: 'sources', label: '来源', alert: alertPaths.has('sources') },
    { value: 'pipeline', label: '处理', alert: alertPaths.has('merge') || alertPaths.has('ops') },
    { value: 'outputs', label: '输出', alert: alertPaths.has('outputs') },
  ].map((item) => ({ ...item, disabled: busy || previewing })));
  const groupProcessingCount = $derived(draft.sources.filter((source: RuleSource) => source.group && (source.preprocess !== undefined || (source.ops?.length ?? 0) > 0)).length);
  const preprocessStats = $derived.by(() => {
    let inherit = 0, override = 0, off = 0;
    for (const source of draft.sources) {
      if (source.preprocess === undefined) inherit++;
      else if (source.preprocess) override++;
      else off++;
    }
    return { inherit, override, off };
  });
  const title = $derived(drawerView === 'preview' ? `预览 · ${draft.name || draft.id || '未命名'}` : editing ? '编辑规则' : '新建规则');

  function close() {
    if (onrequestclose()) open = false;
  }
</script>

<PixelDrawer bind:open {title} icon={rulesIcon} width="760px" {onrequestclose}>
  <div class="editor-form">
    {#if message}<div class="notice" class:error role="status">{message}</div>{/if}
    {#if issues.length}
      <div class="notice error issue-list" role="alert" aria-label="表单错误">
        <p>请先修正 {issues.length} 个问题（点击跳转到对应位置）：</p>
        <ul>
          {#each issues as issue, index (index)}
            <li><button type="button" onclick={() => onReject(issue)}>{issue.message}</button></li>
          {/each}
        </ul>
      </div>
    {/if}
    {#if drawerView === 'edit'}
      <PixelTabs id="rule-editor" label="规则编辑" items={editorTabs} value={editorTab} onchange={value => { editorTab = value; }} />
      {#if editorTab === 'props'}
      <div role="tabpanel" id="rule-editor-panel-props" aria-labelledby="rule-editor-tab-props" class="editor-fields">
      <fieldset disabled={busy || previewing}>
        <legend>基本属性</legend>
        <div class="fields">
          <label data-field="id">规则 ID<PixelInput error={fieldError?.path === 'id'} bind:value={draft.id} disabled={!!editing} placeholder="例如：google" oninput={() => onClearFieldError('id')} />
            {#if fieldError?.path === 'id'}<span class="field-error">{fieldError.message}</span>{/if}
          </label>
          <label data-field="name">名称<PixelInput error={fieldError?.path === 'name'} bind:value={draft.name} placeholder="显示名称" oninput={() => onClearFieldError('name')} />
            {#if fieldError?.path === 'name'}<span class="field-error">{fieldError.message}</span>{/if}
          </label>
          <label class="full">说明<PixelTextarea bind:value={draft.description} rows={2} placeholder="可选说明" /></label>
          <div class="field-item full" data-field="tags">
            <label for="rule-tags">标签</label>
            <PixelTagInput
              id="rule-tags"
              bind:value={draft.tags}
              placeholder="输入标签后按回车或逗号…"
              disabled={busy || previewing}
            />
          </div>
        </div>
      </fieldset>
      </div>
      {:else if editorTab === 'sources'}
      <div role="tabpanel" id="rule-editor-panel-sources" aria-labelledby="rule-editor-tab-sources" class="editor-fields">
      <section aria-label="统一预处理" class="unified-preprocess">
        <h3>统一预处理 <span class="optional-tag">可选</span></h3>
        <p class="unified-hint">应用于所有未单独配置的来源 · 继承 {preprocessStats.inherit} · 覆盖 {preprocessStats.override} · 禁用 {preprocessStats.off}</p>
        <CodeEditor value={draft.preprocess ?? ''} language="javascript" label="JavaScript" filename="process.js" height="220px"
          placeholder={"function process(content) { return content; }"} readonly={busy || previewing}
          onchange={(value) => { draft.preprocess = value; }} />
      </section>
      <div class="field-zone" data-field="sources" oninput={() => onClearFieldError('sources')} onchange={() => onClearFieldError('sources')}>
        {#if fieldError?.path === 'sources'}<p class="field-error">{fieldError.message}</p>{/if}
        <SourceEditor bind:sources={draft.sources} preprocess={draft.preprocess} disabled={busy || previewing} {fileOptions} {refOptions} />
      </div>
      </div>
      {:else if editorTab === 'pipeline'}
      <div role="tabpanel" id="rule-editor-panel-pipeline" aria-labelledby="rule-editor-tab-pipeline" class="editor-fields">
      {#if groupProcessingCount}
        <p class="pipeline-hint">
          {groupProcessingCount} 个来源组配置了独立预处理/过滤
          <button type="button" class="pipeline-link" onclick={() => { editorTab = 'sources'; }}>查看来源</button>
        </p>
      {/if}
      <section data-field="merge" oninput={() => onClearFieldError('merge')} onchange={() => onClearFieldError('merge')}>
        <div class="section-head"><h3>合并策略</h3></div>
        {#if fieldError?.path === 'merge'}<p class="field-error section-error">{fieldError.message}</p>{/if}
        <div class="format-single">
          <PixelSelect id="rule-merge" label="合并策略" options={[...mergeStrategies]} value={draft.merge?.strategy || 'union'} disabled={busy || previewing}
            onchange={(value) => { draft.merge = { strategy: value }; }} />
        </div>
      </section>
      <section aria-label="全局过滤链" data-field="ops" oninput={() => onClearFieldError('ops')} onchange={() => onClearFieldError('ops')}>
        <div class="section-head"><h3>全局过滤链</h3></div>
        {#if fieldError?.path === 'ops'}<p class="field-error section-error">{fieldError.message}</p>{/if}
        <div class="format-single"><OpsEditor bind:value={draft.ops!} disabled={busy || previewing} /></div>
      </section>
      </div>
      {:else}
      <div role="tabpanel" id="rule-editor-panel-outputs" aria-labelledby="rule-editor-tab-outputs" class="editor-fields">
      <section data-field="outputs" oninput={() => onClearFieldError('outputs')} onchange={() => onClearFieldError('outputs')}>
        <div class="section-head"><h3>输出客户端</h3></div>
        {#if fieldError?.path === 'outputs'}<p class="field-error section-error">{fieldError.message}</p>{/if}
        {#if !clients.length}
          <p class="empty-hint">还没有客户端。请先在客户端管理中创建。</p>
        {:else}
          <div class="outputs">
            {#each clients as client (client.id)}
              {@const formats = (client.formats?.length ? client.formats.map((format) => formatLabel(format.template)) : [formatLabel(client.template)]).filter(Boolean)}
              {@const on = draft.outputs.includes(client.id)}
              <div class="output-item" class:on>
                <div class="output-head">
                  <img src={resolveClientIcon(client)} width="16" height="16" alt="" />
                  <PixelCheckbox
                    label={client.name || client.id}
                    checked={on}
                    disabled={busy || previewing}
                    onchange={(checked) => {
                      draft.outputs = checked ? [...draft.outputs, client.id] : draft.outputs.filter((id) => id !== client.id);
                    }}
                  />
                </div>
                {#if formats.length}
                  <div class="output-formats">
                    {#each formats as fmt}<span>{fmt}</span>{/each}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </section>
      </div>
      {/if}
    {:else}
      <div class="editor-fields">
        {#if previewing}<p role="status" class="preview-meta">正在抓取来源并编译…</p>{/if}
        {#if preview}
          {#key preview}<RulePreviewPanel {preview} />{/key}
        {/if}
      </div>
    {/if}
  </div>
  {#snippet footer()}
    {#if drawerView === 'edit'}
      <PixelButton disabled={busy || previewing} onclick={close}>关闭</PixelButton>
      <PixelButton disabled={busy || previewing} onclick={onPreviewDraft}>预览当前修改</PixelButton>
    {:else}
      <PixelButton disabled={busy || previewing} onclick={() => { drawerView = 'edit'; }}>返回编辑</PixelButton>
    {/if}
    <PixelButton variant="primary" disabled={busy || previewing || (!dirty && !!editing)} onclick={onSave}>{busy ? '保存中…' : '保存规则'}</PixelButton>
  {/snippet}
</PixelDrawer>

<style>
  .editor-form { display: flex; flex-direction: column; gap: 18px; min-width: 0; padding-bottom: 36px; }
  .section-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; padding: 0 16px; }
  .notice { display: flex; align-items: center; gap: 10px; padding: 10px 14px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; color: var(--text); overflow-wrap: anywhere; }
  .notice.error { background: var(--status-error); }
  .issue-list { display: grid; gap: 6px; align-items: start; }
  .issue-list p { margin: 0; }
  .issue-list ul { margin: 0; padding: 0 0 0 2px; list-style: none; display: grid; gap: 2px; }
  .issue-list button { padding: 0 2px; border: 0; background: none; color: inherit; font: inherit; text-align: left; text-decoration: underline; cursor: pointer; overflow-wrap: anywhere; }
  h3, legend { font: 400 12px/20px var(--font-ui); color: var(--display); margin: 0; }
  fieldset { min-width: 0; margin: 12px 0 0; padding: 14px 16px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; display: grid; gap: 12px; }
  .fields { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 20px; }
  label, .field-item { display: grid; gap: 6px; color: var(--sec); font: 12px/18px var(--font-ui); }
  .field-item label { color: inherit; font: inherit; }
  .full { grid-column: 1 / -1; }
  .field-error { margin: 0; color: var(--error-border); font: 12px/18px var(--font-ui); }
  .section-error { margin: 8px 16px 0; }
  .field-zone { display: grid; gap: 8px; min-width: 0; }
  .format-single { margin: 12px 16px 0; display: grid; gap: 8px; }
  .empty-hint { margin: 12px 16px 0; padding: 12px; background: var(--surface-2); border: 1px dashed var(--border); border-radius: 4px; font: 12px/18px var(--font-ui); color: var(--sec); }
  .outputs { margin: 12px 16px 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 8px; }
  .output-item { display: grid; gap: 4px; align-content: start; min-width: 0; padding: 8px 10px; border: 1px solid var(--border); border-radius: 3px; background: var(--surface); }
  .output-item.on { border-color: var(--border-vis); background: var(--surface-2); }
  .output-head { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .output-head img { width: 16px; height: 16px; flex-shrink: 0; image-rendering: pixelated; }
  .output-formats { display: grid; gap: 1px; padding-left: 48px; color: var(--sec); font: 11px/17px var(--font-code); }
  .output-formats span { overflow-wrap: anywhere; }
  .editor-fields { display: grid; gap: 18px; min-width: 0; }
  .preview-meta { margin: 0; color: var(--sec); }
  .pipeline-hint { margin: 0; padding: 10px 14px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 4px; color: var(--sec); font: 12px/18px var(--font-ui); }
  .pipeline-link { padding: 0 2px; border: 0; background: none; color: var(--text); font: inherit; text-decoration: underline; cursor: pointer; }
  .unified-preprocess { display: grid; gap: 8px; padding: 12px; border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface); }
  .optional-tag { color: var(--dim); }
  .unified-hint { margin: 0; color: var(--sec); font: 12px/18px var(--font-ui); }
  @media (max-width: 600px) { .fields { grid-template-columns: 1fr; } }
</style>
