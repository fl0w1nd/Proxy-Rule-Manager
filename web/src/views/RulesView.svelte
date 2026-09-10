<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { api, APIRequestError, conflictHint, type ConfigSnapshot, type LocalFileItem, type RuleItem, type RulePreview, type TemplateItem } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelInput from '../components/pixel/PixelInput.svelte';
  import PixelSwitch from '../components/pixel/PixelSwitch.svelte';
  import LocalFilesView from './LocalFilesView.svelte';
  import RuleTable from './rules/RuleTable.svelte';
  import RuleEditorDrawer from './rules/RuleEditorDrawer.svelte';
  import RulePreviewDrawer from './rules/RulePreviewDrawer.svelte';
  import RuleDialogs from './rules/RuleDialogs.svelte';
  import { clientIconID, clientIconSrc, defaultClientIcon, type ClientConfig } from './clients';
  import {
    emptyRule,
    moveRuleOrder,
    readRule,
    refChoices,
    ruleReferences,
    ruleTopology,
    serializeRule,
    splitCSV,
    validateRuleAll,
    type RuleConfig,
    type RuleIssue,
  } from './rules';

  interface Props {
    onStartUpdate: (scope: 'rules', ruleIds: string[]) => void;
    activeRuleId?: string | null;
    isUpdating?: boolean;
    currentProcessingRuleId?: string | null;
    onstatechange?: (dirty: boolean, saving: boolean) => void;
  }

  let {
    onStartUpdate,
    activeRuleId = null,
    isUpdating = false,
    currentProcessingRuleId = null,
    onstatechange,
  }: Props = $props();

  let tab = $state('compile');
  let filesDirty = $state(false);
  let filesBusy = $state(false);
  let pending = $state('');
  let leaveDialog = $state(false);
  let snapshot = $state<ConfigSnapshot>();
  let statuses = $state<RuleItem[]>([]);
  let files = $state<LocalFileItem[]>([]);
  let searchQuery = $state('');
  let loading = $state(true);
  let busy = $state(false);
  let message = $state('');
  let error = $state(false);
  let fieldError = $state<RuleIssue | null>(null);
  let issues = $state<RuleIssue[]>([]);
  let open = $state(false);
  let editing = $state('');
  let draft = $state<RuleConfig>(emptyRule());
  let baseline = $state('');
  let tagText = $state('');
  let discard = $state(false);
  let deleteOpen = $state(false);
  let deleting = $state<RuleConfig>();
  let refConflictOpen = $state(false);
  let refConflictList = $state<string[]>([]);
  let selected = $state<string[]>([]);
  let selecting = $state(false);
  let batchClient = $state('');
  let editorTab = $state('props');
  let drawerView = $state<'edit' | 'preview'>('edit');
  let previewing = $state(false);
  let preview = $state<RulePreview | null>(null);
  let rowPreviewOpen = $state(false);
  let rowPreviewing = $state(false);
  let rowPreview = $state<RulePreview | null>(null);
  let rowPreviewError = $state('');
  let rowPreviewName = $state('');
  let templates = $state<TemplateItem[]>([]);

  const tabs = [
    { value: 'compile', label: '编译规则' },
    { value: 'files', label: '本地文件' },
  ];
  const configRules = $derived(((snapshot?.config.rules ?? []) as Record<string, unknown>[]));
  const clients = $derived(((snapshot?.config.clients ?? []) as ClientConfig[]));
  const rows = $derived(configRules.map((raw) => {
    const id = String(raw.id ?? '');
    const status = statuses.find((item) => item.id === id);
    return { id, name: String(raw.name ?? id), entries: status?.entries ?? 0, version_at: status?.version_at, last_check: status?.last_check, raw };
  }));
  function measureVisualTextWidth(text: string): number {
    let width = 0;
    for (let i = 0; i < text.length; i++) {
      width += text.charCodeAt(i) > 255 ? 19 : 8.5;
    }
    return width;
  }
  const nameColumnWidth = $derived.by(() => {
    let maxTextWidth = measureVisualTextWidth('名称 / ID');
    for (const rule of rows) {
      const refs = topologies.get(rule.id);
      const hasRefs = !!refs && (refs.upstream.length > 0 || refs.downstream.length > 0);
      const outs = ruleOutputs(rule.raw);
      const nameW = measureVisualTextWidth(rule.name) + (hasRefs ? 30 : 0);
      const iconsW = outs.length ? 6 + Math.min(outs.length, 4) * 14 + (outs.length > 4 ? 30 : 0) : 0;
      const idW = measureVisualTextWidth(rule.id) + iconsW;
      const w = Math.max(nameW, idW);
      if (w > maxTextWidth) maxTextWidth = w;
    }
    return Math.max(160, Math.ceil(maxTextWidth + 48));
  });
  const filteredRules = $derived(rows.filter((rule) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return rule.name.toLowerCase().includes(q) || rule.id.toLowerCase().includes(q);
  }));
  const dirty = $derived(open && JSON.stringify({ ...draft, tags: splitCSV(tagText) }) !== baseline);
  const identities = $derived(configRules.map((raw) => ({ id: String(raw.id ?? ''), name: String(raw.name ?? ''), sources: raw.sources })));
  const topologies = $derived(new Map(identities.map((rule) => [rule.id, ruleTopology(identities, rule.id)])));
  const refOptions = $derived([{ value: '', label: '请选择规则' }, ...refChoices(identities, editing || draft.id)]);
  const fileOptions = $derived([{ value: '', label: files.length ? '请选择文件' : '暂无本地文件' }, ...files.map((file) => ({ value: file.name, label: file.name }))]);
  const clientOptions = $derived([{ value: '', label: '选择客户端' }, ...clients.map((client) => ({ value: client.id, label: client.name || client.id }))]);
  const canReorder = $derived(!searchQuery && !busy && !isUpdating);
  const issueTab: Record<string, string> = { id: 'props', name: 'props', sources: 'sources', merge: 'pipeline', ops: 'pipeline', outputs: 'outputs' };

  $effect(() => { onstatechange?.(dirty || filesDirty, busy || filesBusy || previewing); });
  onMount(() => { void load(); });
  onDestroy(() => { onstatechange?.(false, false); });

  function ruleClientIcon(client: ClientConfig): string {
    if (client.icon && clientIconID(client.icon)) return clientIconSrc(client.icon);
    return `/static/icons/${encodeURIComponent(defaultClientIcon(client.id))}.svg`;
  }
  function formatLabel(templateID?: string): string {
    if (!templateID) return '';
    const tpl = templates.find((item) => item.id === templateID);
    return tpl ? `${tpl.name || tpl.id} · ${tpl.extension}` : templateID;
  }
  function ruleOutputs(raw: Record<string, unknown>) {
    const outputIDs = Array.isArray(raw.outputs) ? raw.outputs.map(String) : [];
    return outputIDs.map((id) => {
      const client = clients.find((item) => item.id === id);
      const formats = client?.formats?.length
        ? client.formats.map((format) => formatLabel(format.template))
        : [formatLabel(client?.template)];
      return {
        id,
        name: client?.name || id,
        icon: client ? ruleClientIcon(client) : `/static/icons/${encodeURIComponent(defaultClientIcon(id))}.svg`,
        formats: formats.filter(Boolean),
      };
    });
  }

  function filesState(nextDirty: boolean, nextBusy: boolean) {
    filesDirty = nextDirty;
    filesBusy = nextBusy;
  }
  function leave(next: string) {
    filesState(false, false);
    tab = next;
    if (next === 'compile') void load();
  }
  function switchTab(next: string) {
    if (next === tab || filesBusy || busy || previewing) return;
    if (filesDirty || dirty) { pending = next; leaveDialog = true; return; }
    leave(next);
  }
  export async function loadRules() { await load(); }
  async function load() {
    loading = true;
    error = false;
    message = '';
    try {
      const [cfg, list, local] = await Promise.all([api.getConfig(), api.getRules(), api.listLocalFiles()]);
      snapshot = cfg;
      statuses = list.items || [];
      files = local.items || [];
      void api.listTemplates().then((res) => { templates = res.items || []; }).catch(() => { templates = []; });
    } catch (e) { fail(e); }
    finally { loading = false; }
  }
  function fail(e: unknown) {
    error = true;
    message = (e as Error).message;
    if (e instanceof APIRequestError && e.details.errors?.length) message += '：' + e.details.errors.map((item) => `${item.path} ${item.message}`).join('；');
    const hint = conflictHint(e, '请关闭编辑器后刷新配置再重试。');
    if (hint) message += '。' + hint;
  }
  function edit(raw?: Record<string, unknown>) {
    const next = raw ? readRule(raw) : emptyRule();
    editing = raw ? next.id : '';
    draft = next;
    editorTab = 'props';
    drawerView = 'edit';
    preview = null;
    tagText = (next.tags ?? []).join(', ');
    baseline = JSON.stringify({ ...next, tags: splitCSV(tagText) });
    message = '';
    error = false;
    fieldError = null;
    issues = [];
    open = true;
    void api.listLocalFiles().then((res) => { files = res.items || []; }).catch(fail);
  }
  function requestClose() {
    if (busy || previewing) return false;
    if (dirty) { discard = true; return false; }
    return true;
  }
  function applyTags() { draft = { ...draft, tags: splitCSV(tagText) }; }
  function clearFieldError(path: string) {
    if (fieldError?.path === path) fieldError = null;
    if (issues.length) issues = issues.filter((item) => item.path !== path);
  }
  async function scrollToIssue(path: string) {
    await tick();
    const el = document.querySelector(`.editor-form [data-field="${path}"]`);
    el?.scrollIntoView?.({ block: 'center', behavior: 'smooth' });
    if (path === 'id' || path === 'name') (el?.querySelector('input') as HTMLElement | null)?.focus({ preventScroll: true });
  }
  function reject(issue: RuleIssue) {
    fieldError = issue;
    drawerView = 'edit';
    editorTab = issueTab[issue.path] ?? 'props';
    void scrollToIssue(issue.path);
  }
  async function save() {
    if (!snapshot || busy) return;
    applyTags();
    const found = validateRuleAll(draft, identities, clients, files, editing);
    if (found.length) { issues = found; reject(found[0]); return; }
    busy = true; message = '';
    fieldError = null;
    issues = [];
    try {
      const value = serializeRule(draft);
      const result = await api.patchConfig(snapshot.version, [editing ? { op: 'update_rule', id: editing, value } : { op: 'add_rule', value }]);
      const nextRules = editing ? configRules.map((rule) => String(rule.id) === editing ? value : rule) : [...configRules, value];
      snapshot = { version: result.version, config: { ...snapshot.config, rules: nextRules } };
      editing = String(value.id);
      baseline = JSON.stringify({ ...draft, tags: splitCSV(tagText) });
      statuses = (await api.getRules()).items || [];
      error = false;
      message = result.warnings?.length ? `已保存。${result.warnings.join('；')}` : '规则已保存';
    } catch (e) { fail(e); }
    finally { busy = false; }
  }
  function openDraftPreview() {
    drawerView = 'preview';
    void runPreview();
  }
  async function runPreview() {
    if (previewing) return;
    preview = null;
    applyTags();
    const found = validateRuleAll(draft, identities, clients, files, editing);
    if (found.length) { issues = found; reject(found[0]); return; }
    previewing = true; message = '';
    fieldError = null;
    issues = [];
    try {
      preview = await api.previewRule(serializeRule(draft));
      error = false;
    } catch (e) { fail(e); }
    finally { previewing = false; }
  }
  async function openRulePreview(raw: Record<string, unknown>) {
    if (rowPreviewing) return;
    rowPreviewName = String(raw.name || raw.id || '');
    rowPreview = null;
    rowPreviewError = '';
    rowPreviewOpen = true;
    rowPreviewing = true;
    try {
      rowPreview = await api.previewRule(serializeRule(readRule(raw)));
    } catch (e) { rowPreviewError = (e as Error).message; }
    finally { rowPreviewing = false; }
  }
  function askDelete(raw: Record<string, unknown>) {
    const id = String(raw.id ?? '');
    const refs = ruleReferences(identities, id);
    if (refs.length) {
      error = true;
      refConflictList = refs;
      message = refs.length > 3
        ? `规则「${String(raw.name || id)}」正被 ${refs.length} 条规则（如 ${refs.slice(0, 2).join('、')} 等）引用，无法直接删除`
        : `规则正被 ${refs.join('、')} 引用，无法删除`;
      return;
    }
    deleting = readRule(raw);
    deleteOpen = true;
  }
  async function remove() {
    if (!snapshot || !deleting || busy) return;
    busy = true;
    try {
      const result = await api.patchConfig(snapshot.version, [{ op: 'remove_rule', id: deleting.id }]);
      snapshot = { version: result.version, config: { ...snapshot.config, rules: configRules.filter((rule) => String(rule.id) !== deleting?.id) } };
      statuses = statuses.filter((item) => item.id !== deleting?.id);
      selected = selected.filter((id) => id !== deleting?.id);
      if (open && editing === deleting.id) open = false;
      error = false;
      message = '规则已删除';
    } catch (e) { fail(e); }
    finally { busy = false; deleting = undefined; }
  }
  function setSelecting(on: boolean) {
    selecting = on;
    if (!on) selected = [];
  }
  async function batchOutputs(add: boolean) {
    if (!snapshot || !selected.length || !batchClient || busy) return;
    busy = true; message = '';
    try {
      const result = await api.patchConfig(snapshot.version, [{
        op: add ? 'batch_add_output' : 'batch_remove_output',
        rule_ids: selected,
        output_ids: [batchClient],
      }]);
      const picked = new Set(selected);
      snapshot = {
        version: result.version,
        config: {
          ...snapshot.config,
          rules: configRules.map((rule) => {
            if (!picked.has(String(rule.id))) return rule;
            const outputs = Array.isArray(rule.outputs) ? rule.outputs.map(String) : [];
            const next = add ? [...new Set([...outputs, batchClient])] : outputs.filter((id) => id !== batchClient);
            return { ...rule, outputs: next };
          }),
        },
      };
      selected = [];
      error = false;
      message = add ? '已添加输出客户端' : '已移除输出客户端';
    } catch (e) { fail(e); }
    finally { busy = false; }
  }
  async function dropRule(targetID: string, draggingID: string) {
    if (!snapshot || !canReorder || !draggingID || draggingID === targetID) return;
    const order = moveRuleOrder(configRules.map((rule) => String(rule.id)), draggingID, targetID);
    if (order.every((id, index) => id === String(configRules[index]?.id))) return;
    busy = true; message = '';
    try {
      const result = await api.patchConfig(snapshot.version, [{ op: 'reorder_rules', order }]);
      const byID = new Map(configRules.map((rule) => [String(rule.id), rule]));
      snapshot = { version: result.version, config: { ...snapshot.config, rules: order.map((id) => byID.get(id)!).filter(Boolean) } };
      error = false;
    } catch (e) { fail(e); await load(); }
    finally { busy = false; }
  }
</script>

<svelte:window
  onbeforeunload={event => { if (dirty || filesDirty || busy || filesBusy || previewing) { event.preventDefault(); event.returnValue = ''; } }} />
<div class="rules-page">
  <PixelTabs id="rules" label="规则管理" items={tabs.map(item => ({ ...item, disabled: filesBusy || busy || previewing }))}
    value={tab} onchange={switchTab} />

  {#if tab === 'compile'}
    <div class="rules-view" role="tabpanel" id="rules-panel-compile" aria-labelledby="rules-tab-compile">
      <div class="rules-toolbar">
        <div class="toolbar-left">
          <span class="count-badge">
            {#if searchQuery}
              匹配 {filteredRules.length} / 共 {rows.length} 条规则
            {:else}
              共 {rows.length} 条规则
            {/if}
          </span>
          <PixelSwitch label="选择" checked={selecting} disabled={busy} onchange={setSelecting} />
          {#if selecting}
            <div class="select-cluster">
              <span class="select-count">已选 {selected.length}</span>
              <PixelButton size="sm" disabled={loading || busy || isUpdating || !selected.length} onclick={() => onStartUpdate('rules', [...selected])}>更新</PixelButton>
              <div class="batch-client">
                <PixelSelect id="batch-output" label="批量输出客户端" options={clientOptions} bind:value={batchClient} disabled={busy} size="sm" />
              </div>
              <PixelButton size="sm" disabled={busy || !selected.length || !batchClient} onclick={() => batchOutputs(true)}>添加输出</PixelButton>
              <PixelButton size="sm" disabled={busy || !selected.length || !batchClient} onclick={() => batchOutputs(false)}>移除输出</PixelButton>
            </div>
          {/if}
        </div>
        <div class="toolbar-right">
          <div class="search-wrap">
            <PixelInput placeholder="搜索规则名称 / ID…" bind:value={searchQuery} spellcheck="false" aria-label="搜索规则" />
          </div>
          <PixelButton size="sm" disabled={loading || busy} onclick={load}>
            <PixelIcon name="refresh" size={12} />
            刷新
          </PixelButton>
          <PixelButton size="sm" variant="primary" disabled={loading || busy || !snapshot} onclick={() => edit()}>新建规则</PixelButton>
        </div>
      </div>

      {#if message && !open}
        <div class="rules-error" class:notice={!error} role="status">
          <span>{message}</span>
          {#if refConflictList.length > 3}
            <PixelButton size="sm" variant="ghost" onclick={() => { refConflictOpen = true; }}>查看引用清单 ({refConflictList.length})</PixelButton>
          {/if}
        </div>
      {/if}

      {#if error && !open && !message}
        <div class="rules-error">
          <PixelIcon name="warn" size={16} />
          <span>读取规则列表失败</span>
          <PixelButton size="sm" onclick={load}>重试</PixelButton>
        </div>
      {/if}

      <RuleTable
        rows={filteredRules}
        {loading}
        {busy}
        {isUpdating}
        {activeRuleId}
        {currentProcessingRuleId}
        {selecting}
        bind:selected
        {canReorder}
        {searchQuery}
        {nameColumnWidth}
        {topologies}
        resolveOutputs={ruleOutputs}
        {onStartUpdate}
        onEdit={(raw) => edit(raw)}
        onPreview={openRulePreview}
        onDelete={askDelete}
        onReorder={dropRule}
      />
    </div>
  {:else if tab === 'files'}
    <div role="tabpanel" id="rules-panel-files" aria-labelledby="rules-tab-files">
      <LocalFilesView onstatechange={filesState} />
    </div>
  {/if}
</div>

<RuleEditorDrawer
  bind:open
  bind:draft
  bind:tagText
  bind:editorTab
  bind:drawerView
  {editing}
  {dirty}
  {busy}
  {previewing}
  {preview}
  {message}
  {error}
  {fieldError}
  {issues}
  {clients}
  {fileOptions}
  {refOptions}
  resolveClientIcon={ruleClientIcon}
  {formatLabel}
  onrequestclose={requestClose}
  onReject={reject}
  onClearFieldError={clearFieldError}
  onSave={save}
  onPreviewDraft={openDraftPreview}
/>

<RulePreviewDrawer
  bind:open={rowPreviewOpen}
  name={rowPreviewName}
  previewing={rowPreviewing}
  preview={rowPreview}
  error={rowPreviewError}
/>

<RuleDialogs
  bind:leaveOpen={leaveDialog}
  bind:discardOpen={discard}
  bind:deleteOpen
  bind:refConflictOpen
  deletingName={deleting?.name || deleting?.id || ''}
  {refConflictList}
  onConfirmLeave={() => { open = false; leave(pending); pending = ''; }}
  onCancelLeave={() => { pending = ''; }}
  onConfirmDiscard={() => { open = false; }}
  onConfirmDelete={remove}
/>

<style>
  .rules-page { display: flex; flex-direction: column; gap: 18px; min-width: 0; }
  .rules-view { display: flex; flex-direction: column; gap: 16px; }
  .rules-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .toolbar-left, .toolbar-right { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .select-cluster { display: inline-flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 4px 8px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; }
  .select-count { color: var(--sec); white-space: nowrap; padding: 0 2px; }
  .batch-client { width: 148px; flex: 0 0 148px; }
  .search-wrap { width: 260px; }
  .rules-error { display: flex; align-items: center; gap: 10px; padding: 10px 14px; background: var(--status-error); border: 1px solid var(--border-vis); border-radius: 4px; color: var(--text); overflow-wrap: anywhere; }
  .rules-error.notice { background: var(--surface-2); }
</style>
