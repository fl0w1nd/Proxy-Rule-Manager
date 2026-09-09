<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { api, APIRequestError, type ConfigSnapshot, type LocalFileItem, type RuleItem, type RulePreview, type TemplateItem } from '../api/client';
  import PixelTable from '../components/pixel/PixelTable.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import RefsIcon from '../components/pixel/RefsIcon.svelte';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelCheckbox from '../components/pixel/PixelCheckbox.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelSwitch from '../components/pixel/PixelSwitch.svelte';
  import OpsEditor from '../components/forms/OpsEditor.svelte';
  import CodePanel from '../components/CodePanel.svelte';
  import SourceEditor from './SourceEditor.svelte';
  import LocalFilesView from './LocalFilesView.svelte';
  import { clientIconID, clientIconSrc, defaultClientIcon, type ClientConfig } from './clients';
  import {
    emptyRule,
    mergeStrategies,
    moveRuleOrder,
    readRule,
    refChoices,
    ruleReferences,
    ruleTopology,
    serializeRule,
    splitCSV,
    validateRule,
    type RuleConfig,
  } from './rules';
  import { retroScroll } from '../utils/scrollbars';
  import rulesIcon from '../assets/icons/nav/rules.svg';

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
  let dragging = $state('');
  let dropTarget = $state('');
  let editorTab = $state('edit');
  let previewClient = $state('');
  let previewing = $state(false);
  let preview = $state<RulePreview | null>(null);
  let previewTab = $state('');
  let hint = $state<{ kind: 'refs' | 'outputs'; id: string; top: number; left: number; above: boolean } | null>(null);
  let hintTimer = 0;
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
  const selectedSet = $derived(new Set(selected));
  const filteredSelected = $derived(filteredRules.filter((rule) => selectedSet.has(rule.id)).length);
  const canReorder = $derived(!searchQuery && !busy && !isUpdating);
  const previewOutputs = $derived(preview?.outputs ?? []);
  const previewClients = $derived([...new Map(previewOutputs.map(item => [item.client_id, { value: item.client_id, label: item.client_name }])).values()]);
  const previewFormats = $derived(previewOutputs.filter(item => item.client_id === previewClient));

  $effect(() => { onstatechange?.(dirty || filesDirty, busy || filesBusy || previewing); });
  onMount(() => { void load(); });
  onDestroy(() => {
    clearTimeout(hintTimer);
    onstatechange?.(false, false);
  });

  function showHint(kind: 'refs' | 'outputs', id: string, el: HTMLElement, immediate = false) {
    clearTimeout(hintTimer);
    const place = () => {
      const box = el.getBoundingClientRect();
      const below = box.bottom + 7;
      const above = below + 200 > window.innerHeight;
      hint = { kind, id, top: above ? box.top - 7 : below, left: box.left + box.width / 2, above };
    };
    if (immediate) place();
    else hintTimer = window.setTimeout(place, 160);
  }
  function hideHint() {
    clearTimeout(hintTimer);
    hint = null;
  }
  function ruleClientIcon(client: ClientConfig): string {
    if (client.icon && clientIconID(client.icon)) return clientIconSrc(client.icon);
    return `/static/icons/${encodeURIComponent(defaultClientIcon(client.id))}.svg`;
  }
  function formatLabel(templateID?: string): string {
    if (!templateID) return '';
    const tpl = templates.find((item) => item.id === templateID);
    return tpl ? `${tpl.name || tpl.id} · ${tpl.extension}` : templateID;
  }
  interface RuleOutputInfo { id: string; name: string; icon: string; formats: string[] }
  function ruleOutputs(raw: Record<string, unknown>): RuleOutputInfo[] {
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
    if (e instanceof APIRequestError && e.status === 409) message += '。请关闭编辑器后刷新配置再重试。';
  }
  function formatTime(iso?: string) {
    if (!iso) return '—';
    return new Date(iso).toLocaleString('zh-CN', { hour12: false });
  }
  function getCheckStatusType(res?: string): 'success' | 'warning' | 'error' | 'neutral' | 'active' {
    if (!res) return 'neutral';
    if (res === 'updated') return 'success';
    if (res === 'unchanged') return 'neutral';
    if (res === 'failed') return 'error';
    if (res === 'cancelled') return 'warning';
    if (res === 'updating') return 'active';
    return 'neutral';
  }
  function getCheckStatusLabel(res?: string): string {
    const map: Record<string, string> = { updated: '已更新', unchanged: '无变化', failed: '失败', cancelled: '已取消', updating: '更新中', none: '未检查' };
    return (res && map[res]) || res || '未检查';
  }
  function edit(raw?: Record<string, unknown>) {
    const next = raw ? readRule(raw) : emptyRule();
    editing = raw ? next.id : '';
    draft = next;
    editorTab = 'edit';
    preview = null;
    tagText = (next.tags ?? []).join(', ');
    baseline = JSON.stringify({ ...next, tags: splitCSV(tagText) });
    message = '';
    error = false;
    open = true;
    void api.listLocalFiles().then((res) => { files = res.items || []; }).catch(fail);
  }
  function requestClose() {
    if (busy || previewing) return false;
    if (dirty) { discard = true; return false; }
    return true;
  }
  function applyTags() { draft = { ...draft, tags: splitCSV(tagText) }; }
  async function save() {
    if (!snapshot || busy) return;
    applyTags();
    const validation = validateRule(draft, identities, clients, files, editing);
    if (validation) { message = validation; error = true; return; }
    busy = true; message = '';
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
  async function runPreview() {
    if (previewing) return;
    preview = null;
    applyTags();
    const validation = validateRule(draft, identities, clients, files, editing);
    if (validation) { message = validation; error = true; return; }
    previewing = true; message = '';
    try {
      preview = await api.previewRule(serializeRule(draft));
      previewTab = preview.outputs[0]?.id ?? '';
      previewClient = preview.outputs[0]?.client_id ?? '';
      error = false;
    } catch (e) { fail(e); }
    finally { previewing = false; }
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
  function toggleSelected(id: string, checked: boolean) {
    selected = checked ? [...selectedSet, id].filter((item, index, list) => list.indexOf(item) === index) : selected.filter((item) => item !== id);
  }
  function toggleFiltered(checked: boolean) {
    const ids = filteredRules.map((rule) => rule.id);
    selected = checked ? [...new Set([...selected, ...ids])] : selected.filter((id) => !ids.includes(id));
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
  async function dropRule(targetID: string) {
    if (!snapshot || !canReorder || !dragging || dragging === targetID) { dragging = ''; dropTarget = ''; return; }
    const order = moveRuleOrder(configRules.map((rule) => String(rule.id)), dragging, targetID);
    dragging = '';
    dropTarget = '';
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
  function durationLabel(ms: number) {
    if (ms < 1000) return `${ms} ms`;
    return `${(ms / 1000).toFixed(2)} s`;
  }
</script>

<svelte:window
  onbeforeunload={event => { if (dirty || filesDirty || busy || filesBusy || previewing) { event.preventDefault(); event.returnValue = ''; } }}
  onscroll={hideHint} />
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
            <input type="text" class="pixel-input" placeholder="搜索规则名称 / ID…" bind:value={searchQuery} spellcheck="false" />
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

      <PixelTable class="rules-table" minWidth="760px">
        <colgroup>
          <col class="c-lead" />
          {#if selecting}<col class="c-lead" />{/if}
          <col class="c-name" style="width: {nameColumnWidth}px;" />
          <col class="c-num" />
          <col class="c-time" />
          <col class="c-status" />
          <col class="c-actions" />
        </colgroup>
        <thead>
          <tr>
            <th class="lead-col"></th>
            {#if selecting}
              <th class="lead-col">
                <PixelCheckbox size="sm" label="全选" checked={filteredRules.length > 0 && filteredSelected === filteredRules.length} disabled={busy || !filteredRules.length}
                  onchange={toggleFiltered} />
              </th>
            {/if}
            <th>名称 / ID</th>
            <th class="num">条目数量</th>
            <th>内容版本时间</th>
            <th>上次检查状态</th>
            <th class="col-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && rows.length === 0}
            <tr><td colspan={selecting ? 7 : 6} class="table-empty">加载规则列表中…</td></tr>
          {:else if filteredRules.length === 0}
            <tr><td colspan={selecting ? 7 : 6} class="table-empty">没有匹配的规则</td></tr>
          {:else}
            {#each filteredRules as rule (rule.id)}
              {@const isRuleActive = activeRuleId === rule.id || currentProcessingRuleId === rule.id}
              {@const refs = topologies.get(rule.id) ?? { upstream: [], downstream: [] }}
              {@const hasOut = refs.upstream.length > 0}
              {@const hasIn = refs.downstream.length > 0}
              {@const outs = ruleOutputs(rule.raw)}
              <tr class:selected={selecting && selectedSet.has(rule.id)} class:drop-target={dropTarget === rule.id}
                ondragover={(event) => { if (!canReorder || !dragging) return; event.preventDefault(); dropTarget = rule.id; }}
                ondrop={(event) => { event.preventDefault(); dropRule(rule.id); }}>
                <td class="lead-col">
                  <button class="drag-handle" type="button" aria-label="拖动排序 {rule.name}" disabled={!canReorder}
                    draggable={canReorder} title={searchQuery ? '搜索时无法排序' : '拖动排序'}
                    ondragstart={(event) => { if (!canReorder) { event.preventDefault(); return; } dragging = rule.id; event.dataTransfer?.setData('text/plain', rule.id); if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'; }}
                    ondragend={() => { dragging = ''; dropTarget = ''; }}>
                    <PixelIcon name="grip" size={12} />
                  </button>
                </td>
                {#if selecting}
                  <td class="lead-col">
                    <PixelCheckbox size="sm" label="选择 {rule.name}" checked={selectedSet.has(rule.id)} disabled={busy} onchange={(checked) => toggleSelected(rule.id, checked)} />
                  </td>
                {/if}
                <td class="col-name">
                  <div class="rule-identity">
                    <div class="rule-text">
                      <button class="rule-open" type="button" onclick={() => edit(rule.raw)}>
                        <div class="rule-name font-name">{rule.name}</div>
                      </button>
                      <div class="rule-sub">
                        <button class="rule-open rule-id-open" type="button" aria-label="编辑规则 {rule.name}" onclick={() => edit(rule.raw)}>
                          <span class="rule-id text-dim">{rule.id}</span>
                        </button>
                        {#if outs.length}
                          <button
                            type="button"
                            class="outputs-mark"
                            aria-label={`输出客户端 ${outs.length} 个：${outs.map((out) => out.name).join('、')}`}
                            aria-expanded={hint?.kind === 'outputs' && hint.id === rule.id}
                            onpointerenter={(event) => showHint('outputs', rule.id, event.currentTarget)}
                            onpointerleave={hideHint}
                            onfocus={(event) => showHint('outputs', rule.id, event.currentTarget, true)}
                            onblur={hideHint}
                            onclick={(event) => {
                              event.stopPropagation();
                              if (hint?.kind === 'outputs' && hint.id === rule.id) hideHint();
                              else showHint('outputs', rule.id, event.currentTarget, true);
                            }}
                            onkeydown={(event) => { if (event.key === 'Escape') { event.stopPropagation(); hideHint(); } }}
                          >
                            {#each outs.slice(0, 4) as out (out.id)}
                              <img src={out.icon} width="12" height="12" alt="" />
                            {/each}
                            {#if outs.length > 4}
                              <span class="outputs-more text-dim">+{outs.length - 4}</span>
                            {/if}
                          </button>
                        {/if}
                      </div>
                    </div>
                    {#if hasOut || hasIn}
                      <button
                        type="button"
                        class="refs-mark"
                        aria-label={`引用关系，引用 ${refs.upstream.length}，被引用 ${refs.downstream.length}`}
                        aria-expanded={hint?.kind === 'refs' && hint.id === rule.id}
                        onpointerenter={(event) => showHint('refs', rule.id, event.currentTarget)}
                        onpointerleave={hideHint}
                        onfocus={(event) => showHint('refs', rule.id, event.currentTarget, true)}
                        onblur={hideHint}
                        onclick={(event) => {
                          event.stopPropagation();
                          if (hint?.kind === 'refs' && hint.id === rule.id) hideHint();
                          else showHint('refs', rule.id, event.currentTarget, true);
                        }}
                        onkeydown={(event) => { if (event.key === 'Escape') { event.stopPropagation(); hideHint(); } }}
                      >
                        <RefsIcon direction={hasOut && hasIn ? 'both' : hasOut ? 'left' : 'right'} size={16} />
                      </button>
                    {/if}
                  </div>
                </td>
                <td class="num">{rule.entries.toLocaleString()}</td>
                <td class="text-sec">{formatTime(rule.version_at)}</td>
                <td>
                  <div class="status-cell">
                    <PixelBadge status={isRuleActive ? 'active' : getCheckStatusType(rule.last_check?.result)} pulse={isRuleActive}>
                      {isRuleActive ? '更新中…' : getCheckStatusLabel(rule.last_check?.result)}
                    </PixelBadge>
                    {#if rule.last_check?.checked_at}
                      <span class="check-time">{formatTime(rule.last_check.checked_at)}</span>
                    {/if}
                  </div>
                </td>
                <td class="col-actions">
                  <div class="rule-actions">
                    <PixelButton size="sm" disabled={busy} onclick={() => edit(rule.raw)}>编辑</PixelButton>
                    <PixelButton size="sm" variant="secondary" disabled={isUpdating || isRuleActive || busy}
                      title={isUpdating ? '当前有更新任务正在进行中' : '更新此规则'}
                      onclick={() => onStartUpdate('rules', [rule.id])}>更新</PixelButton>
                    <PixelButton size="sm" variant="danger" disabled={busy || isRuleActive} onclick={() => askDelete(rule.raw)}>删除</PixelButton>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </PixelTable>
      {#if hint}
        {@const current = hint}
        {#if current.kind === 'refs'}
          {@const refs = topologies.get(current.id) ?? { upstream: [], downstream: [] }}
          <div class="refs-bubble" class:above={current.above} role="tooltip" style="top: {current.top}px; left: {current.left}px">
            <div>
              <strong>引用</strong>
              {#if refs.upstream.length}
                <ul>{#each refs.upstream as item}<li>{item.name || item.id}</li>{/each}</ul>
              {:else}
                <p>没有引用其他规则</p>
              {/if}
            </div>
            <div>
              <strong>被引用</strong>
              {#if refs.downstream.length}
                <ul>{#each refs.downstream as item}<li>{item.name || item.id}</li>{/each}</ul>
              {:else}
                <p>没有被其他规则引用</p>
              {/if}
            </div>
          </div>
        {:else}
          {@const rule = rows.find((item) => item.id === current.id)}
          {#if rule}
            <div class="refs-bubble outputs-bubble" class:above={current.above} role="tooltip" style="top: {current.top}px; left: {current.left}px">
              <strong>输出客户端</strong>
              <ul class="out-list">
                {#each ruleOutputs(rule.raw) as out (out.id)}
                  <li>
                    <img src={out.icon} width="16" height="16" alt="" />
                    <span class="out-meta">
                      <span class="out-name">{out.name}</span>
                      {#each out.formats as fmt}<span class="out-format">{fmt}</span>{/each}
                    </span>
                  </li>
                {/each}
              </ul>
            </div>
          {/if}
        {/if}
      {/if}
    </div>
  {:else if tab === 'files'}
    <div role="tabpanel" id="rules-panel-files" aria-labelledby="rules-tab-files">
      <LocalFilesView onstatechange={filesState} />
    </div>
  {/if}
</div>

<PixelDrawer bind:open title={editing ? '编辑规则' : '新建规则'} icon={rulesIcon} width="760px" onrequestclose={requestClose}>
  <div class="editor-form">
    {#if message}<div class="notice" class:error role="status">{message}</div>{/if}
    <PixelTabs id="rule-editor" label="规则编辑" items={[{ value: 'edit', label: '编辑', disabled: busy || previewing }, { value: 'preview', label: '调试预览', disabled: busy || previewing }]} value={editorTab} onchange={value => { if (value === editorTab) return; editorTab = value; if (value === 'preview') void runPreview(); }} />
    {#if editorTab === 'edit'}
    <div role="tabpanel" id="rule-editor-panel-edit" aria-labelledby="rule-editor-tab-edit" class="editor-fields">
    <fieldset disabled={busy || previewing}>
      <legend>基本属性</legend>
      <div class="fields">
        <label>规则 ID<input bind:value={draft.id} disabled={!!editing} placeholder="例如：google" /></label>
        <label>名称<input bind:value={draft.name} placeholder="显示名称" /></label>
        <label class="full">说明<textarea bind:value={draft.description} rows="2" placeholder="可选说明"></textarea></label>
        <label class="full">标签<input bind:value={tagText} placeholder="用逗号分隔" /></label>
      </div>
    </fieldset>

    <SourceEditor bind:sources={draft.sources} bind:preprocess={draft.preprocess} disabled={busy || previewing} {fileOptions} {refOptions} />

    <section>
      <div class="section-head"><h3>合并策略</h3></div>
      <div class="format-single">
        <PixelSelect id="rule-merge" label="合并策略" options={[...mergeStrategies]} value={draft.merge?.strategy || 'union'} disabled={busy || previewing}
          onchange={(value) => { draft.merge = { strategy: value }; }} />
      </div>
    </section>

    <section aria-label="全局过滤链">
      <div class="section-head"><h3>全局过滤链</h3></div>
      <div class="format-single"><OpsEditor bind:value={draft.ops!} disabled={busy || previewing} /></div>
    </section>

    <section>
      <div class="section-head"><h3>输出客户端</h3></div>
      {#if !clients.length}
        <p class="empty-hint">还没有客户端。请先在客户端管理中创建。</p>
      {:else}
        <div class="outputs">
          {#each clients as client (client.id)}
            <PixelCheckbox
              label={client.name || client.id}
              checked={draft.outputs.includes(client.id)}
              disabled={busy || previewing}
              onchange={(checked) => {
                draft.outputs = checked ? [...draft.outputs, client.id] : draft.outputs.filter((id) => id !== client.id);
              }}
            />
          {/each}
        </div>
      {/if}
    </section>
    </div>
    {:else}
    <div role="tabpanel" id="rule-editor-panel-preview" aria-labelledby="rule-editor-tab-preview">
  {#if previewing}<p role="status" class="preview-meta">正在抓取来源并编译…</p>{/if}
  {#if preview}
    <div class="preview-report">
      <p class="preview-meta">{preview.rule_name} · {preview.elapsed_ms} ms · 合并 {preview.merged} 条</p>
      <div class="preview-sources">
        {#each preview.sources as source}
          <div class="preview-source">
            <strong>{source.label}</strong>
            {#each source.details ?? [] as detail}<span class="source-detail">{detail}</span>{/each}
            <span>{source.type} · {source.entries} 条 · {durationLabel(source.duration_ms)}</span>
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
        <PixelTabs id="rule-preview-clients" label="输出客户端" items={previewClients} value={previewClient} onchange={value => { previewClient = value; previewTab = previewOutputs.find(item => item.client_id === value)?.id ?? ''; }} />
        <div role="tabpanel" id="rule-preview-clients-panel-{previewClient}" aria-labelledby="rule-preview-clients-tab-{previewClient}" class="preview-formats">
        <PixelTabs id="rule-preview-outputs" label="客户端格式" items={previewFormats.map((item) => ({ value: item.id, label: item.name || item.id }))} bind:value={previewTab} />
        {#each previewFormats as item (item.id)}
          {#if previewTab === item.id}
            <div role="tabpanel" id="rule-preview-outputs-panel-{item.id}" aria-labelledby="rule-preview-outputs-tab-{item.id}">
            {#if item.error}
              <div class="notice error">{item.error}</div>
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
  {/if}
    </div>
    {/if}
  </div>
  {#snippet footer()}
    <PixelButton disabled={busy || previewing} onclick={() => { if (requestClose()) open = false; }}>关闭</PixelButton>
    <PixelButton variant="primary" disabled={busy || previewing || (!dirty && !!editing)} onclick={save}>{busy ? '保存中…' : '保存规则'}</PixelButton>
  {/snippet}
</PixelDrawer>

<PixelDialog bind:open={leaveDialog} title="切换规则页面？" confirmLabel="放弃修改并切换" cancelLabel="继续编辑" danger
  oncancel={() => { pending = ''; }} onconfirm={() => { open = false; leave(pending); pending = ''; }}>
  当前内容尚未保存，切换后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={discard} title="放弃未保存的修改？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger onconfirm={() => { open = false; }}>
  当前修改尚未保存，关闭后将丢弃这些修改。
</PixelDialog>
<PixelDialog bind:open={deleteOpen} title="删除规则？" confirmLabel="删除" danger onconfirm={remove}>
  删除 {deleting?.name || deleting?.id} 的规则配置。
</PixelDialog>
<PixelDialog bind:open={refConflictOpen} title="规则引用详情" confirmLabel="知道了" showCancel={false} onconfirm={() => { refConflictOpen = false; }}>
  <div class="ref-conflict-content">
    <p>以下规则正在引用该规则，请先解除引用后再删除：</p>
    <div class="ref-conflict-list" use:retroScroll>
      {#each refConflictList as item}<div class="ref-conflict-item"><code>{item}</code></div>{/each}
    </div>
  </div>
</PixelDialog>

<style>
  .rules-page, .editor-form { display: flex; flex-direction: column; gap: 18px; min-width: 0; }
  .editor-form { padding-bottom: 36px; }
  .rules-view { display: flex; flex-direction: column; gap: 16px; }
  .rules-toolbar, .section-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .section-head { padding: 0 16px; }
  .toolbar-left, .toolbar-right { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .select-cluster { display: inline-flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 4px 8px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; }
  .select-count { color: var(--sec); white-space: nowrap; padding: 0 2px; }
  .batch-client { width: 148px; flex: 0 0 148px; }
  .search-wrap .pixel-input { width: 260px; }
  .rules-error, .notice { display: flex; align-items: center; gap: 10px; padding: 10px 14px; background: var(--status-error); border: 1px solid var(--border-vis); border-radius: 4px; color: var(--text); overflow-wrap: anywhere; }
  .notice { background: var(--surface-2); }
  .notice.error, .rules-error:not(.notice) { background: var(--status-error); }
  .rule-id { color: var(--dim); font: 400 13px/20px var(--font-code); }
  .text-sec { color: var(--sec); font-variant-numeric: tabular-nums; white-space: nowrap; }
  .status-cell { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; min-width: 0; }
  .status-cell :global(.pixel-badge) { align-self: flex-start; max-width: 100%; width: fit-content; }
  .check-time { color: var(--dim); font-variant-numeric: tabular-nums; font-size: 11px; line-height: 16px; white-space: nowrap; }
  .table-empty { text-align: center; color: var(--dim); padding: 36px 0; }
  :global(.rules-table .pixel-table) { table-layout: fixed; }
  :global(.rules-table .c-lead) { width: 40px; }
  :global(.rules-table .c-num) { width: 96px; }
  :global(.rules-table .c-time) { width: 170px; }
  :global(.rules-table .c-status) { width: 180px; }
  :global(.rules-table .c-actions) { width: 180px; }
  :global(.rules-table .pixel-table .lead-col) { padding-left: 8px; padding-right: 8px; }
  :global(.rules-table .pixel-table .col-name),
  :global(.rules-table .pixel-table .col-actions) { overflow: hidden; }
  :global(.rules-table .pixel-table .col-actions) { text-align: right; white-space: nowrap; }
  :global(.lead-col .checkbox-label) {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
  }
  .drag-handle { display: inline-flex; align-items: center; justify-content: center; width: 24px; height: 24px; padding: 0; border: 0; background: transparent; color: var(--dim); cursor: grab; }
  .drag-handle:disabled { opacity: .35; cursor: not-allowed; }
  .rule-identity { display: flex; align-items: flex-start; gap: 6px; min-width: 0; }
  .rule-text { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .rule-open { display: block; width: fit-content; max-width: 100%; padding: 0; border: 0; background: none; color: inherit; text-align: left; cursor: pointer; }
  .rule-name, .rule-id { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rule-sub { display: flex; align-items: center; gap: 8px; margin-top: 4px; min-width: 0; }
  .rule-id-open { flex-shrink: 1; min-width: 0; }
  .refs-mark, .outputs-mark {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    padding: 2px;
    border: 1px solid transparent;
    border-radius: 3px;
    background: transparent;
    cursor: default;
  }
  .refs-mark { width: 24px; height: 24px; color: var(--display); }
  .outputs-mark { gap: 2px; padding: 2px 3px; }
  .outputs-mark img { width: 12px; height: 12px; image-rendering: pixelated; }
  .outputs-more { color: var(--sec); font: 400 12px/18px var(--font-display); padding-left: 2px; }
  .refs-mark:hover,
  .refs-mark:focus-visible,
  .refs-mark[aria-expanded='true'],
  .outputs-mark:hover,
  .outputs-mark:focus-visible,
  .outputs-mark[aria-expanded='true'] {
    background: var(--surface-2);
    border-color: var(--border-vis);
  }
  :global(.pixel-table tbody tr.selected) .refs-mark { color: var(--selected-text); }
  .refs-bubble {
    position: fixed;
    z-index: 40;
    display: grid;
    gap: 8px;
    width: max-content;
    max-width: 240px;
    padding: 6px 10px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    background: var(--surface-2);
    color: var(--text);
    box-shadow: 0 4px 12px rgb(0 0 0 / 25%), var(--shadow-popup);
    font: 400 11px/17px var(--font-ui);
    overflow-wrap: anywhere;
    transform: translateX(-50%);
    animation: pixel-fade 100ms linear;
    pointer-events: none;
  }
  .refs-bubble.above { transform: translate(-50%, -100%); }
  .refs-bubble::before,
  .refs-bubble::after {
    content: '';
    position: absolute;
    left: 50%;
    width: 0;
    height: 0;
    border-left: 4px solid transparent;
    border-right: 4px solid transparent;
    transform: translateX(-50%);
    pointer-events: none;
  }
  .refs-bubble::before { top: -5px; border-bottom: 5px solid var(--border-vis); }
  .refs-bubble::after { top: -4px; border-bottom: 4px solid var(--surface-2); }
  .refs-bubble.above::before { top: auto; bottom: -5px; border-bottom: 0; border-top: 5px solid var(--border-vis); }
  .refs-bubble.above::after { top: auto; bottom: -4px; border-bottom: 0; border-top: 4px solid var(--surface-2); }
  .refs-bubble strong { font: 400 11px/17px var(--font-ui); color: var(--sec); }
  .refs-bubble p, .refs-bubble ul { margin: 2px 0 0; padding: 0; list-style: none; color: var(--text); }
  .outputs-bubble { max-width: 280px; }
  .out-list { display: grid; gap: 6px; }
  .out-list li { display: flex; align-items: flex-start; gap: 8px; }
  .out-list img { width: 16px; height: 16px; margin-top: 1px; image-rendering: pixelated; }
  .out-meta { display: grid; gap: 1px; min-width: 0; }
  .out-name { color: var(--text); overflow-wrap: anywhere; }
  .out-format { color: var(--sec); font: 400 11px/17px var(--font-code); overflow-wrap: anywhere; }
  .rule-actions { display: inline-flex; flex-wrap: nowrap; gap: 6px; }
  :global(.pixel-table tbody tr.drop-target) { background: var(--status-info); }
  h3, legend { font: 400 12px/20px var(--font-ui); color: var(--display); margin: 0; }
  code { font: 12px/20px var(--font-code); color: var(--sec); overflow-wrap: anywhere; }
  fieldset { min-width: 0; margin: 12px 0 0; padding: 14px 16px; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; display: grid; gap: 12px; }
  .fields { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 20px; }
  label { display: grid; gap: 6px; color: var(--sec); font: 12px/18px var(--font-ui); }
  .full { grid-column: 1 / -1; }
  input, textarea { width: 100%; min-width: 0; padding: 4px 8px; border: 1px solid var(--border-vis); border-radius: 3px; box-shadow: var(--edge-inset); color: var(--text); background: var(--surface); font: 13px/20px var(--font-code); }
  input { min-height: 32px; }
  textarea { min-height: 72px; resize: vertical; }
  input:focus-visible, textarea:focus-visible { outline: 1px solid var(--selected); border-color: var(--selected); }
  .format-single { margin: 12px 16px 0; display: grid; gap: 8px; }
  .empty-hint { margin: 12px 16px 0; padding: 12px; background: var(--surface-2); border: 1px dashed var(--border); border-radius: 4px; font: 12px/18px var(--font-ui); color: var(--sec); }
  .outputs { margin: 12px 16px 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 8px; }
  .ref-conflict-content { display: grid; gap: 12px; }
  .ref-conflict-list { max-height: 240px; overflow-y: auto; background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 3px; padding: 8px 12px; display: grid; gap: 4px; }
  .ref-conflict-item { padding: 3px 0; border-bottom: 1px dashed var(--border); }
  .ref-conflict-item:last-child { border-bottom: none; }
  .editor-fields, .preview-formats { display: grid; gap: 18px; min-width: 0; }
  .source-detail, .preview-source strong { overflow-wrap: anywhere; }
  .preview-formats { border: 1px solid var(--border); padding: 12px; border-radius: 3px; }
  .preview-report { display: grid; gap: 12px; }
  .preview-meta, .preview-diff { margin: 0; color: var(--sec); }
  .preview-sources { display: grid; gap: 6px; }
  .preview-source { display: grid; gap: 2px; padding: 8px 10px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 3px; }
  .preview-source em { color: var(--text); }
  .diff-samples { margin: 0; padding: 0; list-style: none; font: 12px/20px var(--font-code); }
  .diff-samples .add { color: var(--text); }
  .diff-samples .del { color: var(--sec); }
  .preview-report :global(.code-panel pre) { margin: 0; max-height: 280px; overflow: auto; padding: 10px 12px; color: var(--terminal-text); font: 13px/20px var(--font-code); white-space: pre-wrap; }
  @media (max-width: 600px) { .fields { grid-template-columns: 1fr; } }
</style>
