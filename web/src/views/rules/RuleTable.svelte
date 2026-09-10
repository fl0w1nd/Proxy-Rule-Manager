<script lang="ts">
  import { onDestroy } from 'svelte';
  import PixelTable from '../../components/pixel/PixelTable.svelte';
  import PixelButton from '../../components/pixel/PixelButton.svelte';
  import PixelBadge from '../../components/pixel/PixelBadge.svelte';
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import RefsIcon from '../../components/pixel/RefsIcon.svelte';
  import PixelCheckbox from '../../components/pixel/PixelCheckbox.svelte';
  import type { RuleIdentity } from '../rules';

  export interface RuleRow {
    id: string;
    name: string;
    entries: number;
    version_at?: string;
    last_check?: { result?: string; checked_at?: string };
    raw: Record<string, unknown>;
  }
  export interface RuleOutputInfo {
    id: string;
    name: string;
    icon: string;
    formats: string[];
  }

  interface Props {
    rows: RuleRow[];
    loading: boolean;
    busy: boolean;
    isUpdating: boolean;
    activeRuleId: string | null;
    currentProcessingRuleId: string | null;
    selecting: boolean;
    selected: string[];
    canReorder: boolean;
    searchQuery: string;
    nameColumnWidth: number;
    topologies: Map<string, { upstream: RuleIdentity[]; downstream: RuleIdentity[] }>;
    resolveOutputs: (raw: Record<string, unknown>) => RuleOutputInfo[];
    onStartUpdate: (scope: 'rules', ruleIds: string[]) => void;
    onEdit: (raw: Record<string, unknown>) => void;
    onPreview: (raw: Record<string, unknown>) => void;
    onDelete: (raw: Record<string, unknown>) => void;
    onReorder: (targetID: string, draggingID: string) => void;
  }

  let {
    rows,
    loading,
    busy,
    isUpdating,
    activeRuleId,
    currentProcessingRuleId,
    selecting,
    selected = $bindable(),
    canReorder,
    searchQuery,
    nameColumnWidth,
    topologies,
    resolveOutputs,
    onStartUpdate,
    onEdit,
    onPreview,
    onDelete,
    onReorder,
  }: Props = $props();

  let dragging = $state('');
  let dropTarget = $state('');
  let hint = $state<{ kind: 'refs' | 'outputs'; id: string; top: number; left: number; arrowOffset: number; above: boolean } | null>(null);
  let hintTimer = 0;

  const selectedSet = $derived(new Set(selected));
  const filteredSelected = $derived(rows.filter((rule) => selectedSet.has(rule.id)).length);

  onDestroy(() => clearTimeout(hintTimer));

  $effect(() => {
    if (!hint) return;
    const onMove = () => hideHint();
    window.addEventListener('scroll', onMove, true);
    window.addEventListener('resize', onMove);
    return () => {
      window.removeEventListener('scroll', onMove, true);
      window.removeEventListener('resize', onMove);
    };
  });

  function showHint(kind: 'refs' | 'outputs', id: string, el: HTMLElement, immediate = false) {
    clearTimeout(hintTimer);
    const place = () => {
      const box = el.getBoundingClientRect();
      const below = box.bottom + 7;
      const above = below + 200 > window.innerHeight;
      const center = box.left + box.width / 2;
      const maxHalf = kind === 'outputs' ? 140 : 120;
      const left = Math.max(maxHalf + 8, Math.min(window.innerWidth - maxHalf - 8, center));
      const arrowOffset = Math.max(-maxHalf + 16, Math.min(maxHalf - 16, center - left));
      hint = { kind, id, top: above ? box.top - 7 : below, left, arrowOffset, above };
    };
    if (immediate) place();
    else hintTimer = window.setTimeout(place, 160);
  }
  function hideHint() {
    clearTimeout(hintTimer);
    hint = null;
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

  function toggleSelected(id: string, checked: boolean) {
    selected = checked ? [...selectedSet, id].filter((item, index, list) => list.indexOf(item) === index) : selected.filter((item) => item !== id);
  }
  function toggleFiltered(checked: boolean) {
    const ids = rows.map((rule) => rule.id);
    selected = checked ? [...new Set([...selected, ...ids])] : selected.filter((id) => !ids.includes(id));
  }
</script>

<PixelTable class="rules-table" minWidth="820px">
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
          <PixelCheckbox size="sm" label="全选" checked={rows.length > 0 && filteredSelected === rows.length} disabled={busy || !rows.length}
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
    {:else if rows.length === 0}
      <tr><td colspan={selecting ? 7 : 6} class="table-empty">没有匹配的规则</td></tr>
    {:else}
      {#each rows as rule (rule.id)}
        {@const isRuleActive = activeRuleId === rule.id || currentProcessingRuleId === rule.id}
        {@const refs = topologies.get(rule.id) ?? { upstream: [], downstream: [] }}
        {@const hasOut = refs.upstream.length > 0}
        {@const hasIn = refs.downstream.length > 0}
        {@const outs = resolveOutputs(rule.raw)}
        <tr class:selected={selecting && selectedSet.has(rule.id)} class:drop-target={dropTarget === rule.id}
          ondragover={(event) => { if (!canReorder || !dragging) return; event.preventDefault(); dropTarget = rule.id; }}
          ondrop={(event) => { event.preventDefault(); onReorder(rule.id, dragging); dragging = ''; dropTarget = ''; }}>
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
                <button class="rule-open" type="button" onclick={() => onEdit(rule.raw)}>
                  <div class="rule-name font-name">{rule.name}</div>
                </button>
                <div class="rule-sub">
                  <button class="rule-open rule-id-open" type="button" aria-label="编辑规则 {rule.name}" onclick={() => onEdit(rule.raw)}>
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
              <PixelButton size="sm" disabled={busy} onclick={() => onEdit(rule.raw)}>编辑</PixelButton>
              <PixelButton size="sm" disabled={busy} onclick={() => onPreview(rule.raw)}>预览</PixelButton>
              <PixelButton size="sm" variant="secondary" disabled={isUpdating || isRuleActive || busy}
                title={isUpdating ? '当前有更新任务正在进行中' : '更新此规则'}
                onclick={() => onStartUpdate('rules', [rule.id])}>更新</PixelButton>
              <PixelButton size="sm" variant="danger" disabled={busy || isRuleActive} onclick={() => onDelete(rule.raw)}>删除</PixelButton>
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
    <div class="refs-bubble" class:above={current.above} role="tooltip" style="top: {current.top}px; left: {current.left}px; --arrow-left: calc(50% + {current.arrowOffset}px);">
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
      <div class="refs-bubble outputs-bubble" class:above={current.above} role="tooltip" style="top: {current.top}px; left: {current.left}px; --arrow-left: calc(50% + {current.arrowOffset}px);">
        <strong>输出客户端</strong>
        <ul class="out-list">
          {#each resolveOutputs(rule.raw) as out (out.id)}
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

<style>
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
  :global(.rules-table .c-actions) { width: 240px; }
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
    box-shadow: var(--shadow-popup);
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
    left: var(--arrow-left, 50%);
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
</style>
