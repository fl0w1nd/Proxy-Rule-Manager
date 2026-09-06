<script lang="ts">
  import { api, type UpdateProgressEvent, type UpdateDetail } from '../api/client';
  import PixelProgressBar from './pixel/PixelProgressBar.svelte';
  import PixelButton from './pixel/PixelButton.svelte';
  import PixelIcon from './pixel/PixelIcon.svelte';
  import { finishSummary, getStatusLabel } from '../updateLabels';
  import { retroScroll } from '../utils/scrollbars';

  interface Props {
    jobId: string | null;
    onfinish?: (detail?: UpdateDetail) => void;
    onprogressrule?: (ruleId: string) => void;
    onclose?: () => void;
  }

  let { jobId, onfinish, onprogressrule, onclose }: Props = $props();

  let activeJobId = $state<string | null>(null);
  let statusText = $state('就绪');
  let currentMsg = $state('等待指令');
  let currentCount = $state(0);
  let totalCount = $state(0);
  let isRunning = $state(false);
  let isCancelling = $state(false);
  let events = $state<UpdateProgressEvent[]>([]);
  let eventSource: EventSource | null = null;
  let pollTimeout: any = null;
  let logBox: HTMLDivElement | null = null;

  $effect(() => {
    if (jobId) {
      activeJobId = jobId;
      connectJob(jobId);
    }
    return () => {
      cleanup();
    };
  });

  function cleanup() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    if (pollTimeout) {
      clearTimeout(pollTimeout);
      pollTimeout = null;
    }
  }

  function pollFallback(id: string) {
    if (!isRunning) return;
    api.getUpdateDetail(id).then((d) => {
      if (['completed', 'cancelled', 'completed_with_warnings', 'completed_with_errors', 'interrupted'].includes(d.status)) {
        isRunning = false;
        isCancelling = false;
        statusText = getStatusLabel(d.status);
        currentMsg = finishSummary(d);
        cleanup();
        onfinish?.(d);
      } else {
        pollTimeout = setTimeout(() => pollFallback(id), 1500);
      }
    }).catch(() => {
      if (isRunning) {
        pollTimeout = setTimeout(() => pollFallback(id), 2500);
      }
    });
  }

  function connectJob(id: string) {
    cleanup();
    activeJobId = id;
    isRunning = true;
    isCancelling = false;
    statusText = '进行中';
    currentMsg = '正在连接实时事件流…';
    currentCount = 0;
    totalCount = 0;
    events = [];

    eventSource = api.subscribeUpdateEvents(
      id,
      (ev) => {
        events = [...events, ev];
        currentMsg = ev.message;
        if (ev.total > 0) {
          currentCount = ev.current;
          totalCount = ev.total;
        }
        if (ev.rule_id) {
          onprogressrule?.(ev.rule_id);
        }
        scrollLog();
      },
      (detail) => {
        isRunning = false;
        isCancelling = false;
        statusText = getStatusLabel(detail.status);
        currentMsg = finishSummary(detail);
        cleanup();
        onfinish?.(detail);
      },
      () => {
        // SSE error fallback: start polling update status until finished
        if (isRunning && !pollTimeout) {
          pollFallback(id);
        }
      }
    );
  }

  function scrollLog() {
    if (logBox) {
      requestAnimationFrame(() => {
        if (logBox) {
          const vp = logBox.querySelector<HTMLElement>('[data-overlayscrollbars-viewport]') || logBox;
          vp.scrollTop = vp.scrollHeight;
        }
      });
    }
  }

  async function handleCancel() {
    const targetId = activeJobId || jobId;
    if (!targetId || isCancelling) return;
    isCancelling = true;
    try {
      await api.cancelUpdate(targetId);
      currentMsg = '正在请求取消任务…';
    } catch (e: any) {
      currentMsg = `取消失败: ${e.message}`;
      isCancelling = false;
    }
  }
</script>

<div class="console-root">
  <div class="console-summary">
    <div class="summary-top">
      <span class="status-badge {isRunning ? 'running' : 'done'}">
        <span class="dot"></span>
        {statusText}
      </span>
      {#if activeJobId}
        <span class="job-id">JOB #{activeJobId.slice(0, 8)}</span>
      {/if}
    </div>
    <div class="current-msg" title={currentMsg}>{currentMsg}</div>
    <div class="progress-box">
      <PixelProgressBar current={currentCount} total={totalCount} />
    </div>
  </div>

  <div class="console-logs" bind:this={logBox} use:retroScroll role="log" aria-live="polite">
    {#if events.length === 0}
      <div class="log-empty">等待日志事件流…</div>
    {:else}
      {#each events as ev, i}
        <div class="log-row {ev.kind || 'info'}">
          <span class="log-time">
            {new Date(ev.time).toLocaleTimeString('zh-CN', { hour12: false })}
          </span>
          <span class="log-marker">
            {#if ev.kind === 'success'}✓
            {:else if ev.kind === 'error'}×
            {:else if ev.kind === 'warning'}!
            {:else}›{/if}
          </span>
          <span class="log-msg">{ev.message}</span>
        </div>
      {/each}
    {/if}
  </div>

  <div class="console-actions">
    {#if isRunning}
      <PixelButton variant="danger" size="sm" onclick={handleCancel} disabled={isCancelling}>
        <PixelIcon name="cross" size={12} />
        {isCancelling ? '取消中…' : '取消更新'}
      </PixelButton>
    {:else if activeJobId || events.length > 0}
      <PixelButton variant="primary" size="sm" onclick={() => onclose?.()}>
        <PixelIcon name="check" size={12} />
        完成并关闭
      </PixelButton>
    {/if}
  </div>
</div>

<style>
  .console-root {
    display: flex;
    flex-direction: column;
    height: 100%;
    gap: 14px;
  }

  .console-summary {
    background: var(--surface-2);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    padding: 12px 14px;
  }

  .summary-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }

  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0 6px;
    min-height: 20px;
    border: 1px solid var(--border-vis);
    border-radius: 3px;
  }
  .status-badge.running {
    background: var(--status-info);
    color: var(--text);
  }
  .status-badge.running .dot {
    width: 6px;
    height: 6px;
    background: currentColor;
    animation: pixel-signal 600ms steps(2, end) infinite;
  }
  .status-badge.done {
    background: var(--status-success);
    color: var(--text);
  }
  .status-badge.done .dot {
    width: 6px;
    height: 6px;
    background: currentColor;
  }

  .job-id {
    color: var(--dim);
    font: 400 13px/20px var(--font-code);
  }

  .current-msg {
    color: var(--display);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-bottom: 8px;
  }

  .console-logs {
    flex: 1;
    min-height: 280px;
    max-height: 480px;
    overflow-y: auto;
    background: var(--terminal-bg);
    border: 1px solid var(--border-vis);
    border-radius: 3px;
    padding: 10px 12px;
    font: 400 13px/20px var(--font-code);
    color: var(--terminal-text);
    scrollbar-color: var(--bevel-dark) var(--surface-2);
  }

  .log-empty {
    color: var(--terminal-muted);
    text-align: center;
    padding: 40px 0;
  }

  .log-row {
    display: grid;
    grid-template-columns: 65px 14px minmax(0, 1fr);
    gap: 6px;
    align-items: baseline;
    padding: 2px 0;
    border-bottom: 1px solid var(--border);
  }

  .log-time {
    color: var(--terminal-muted);
    font-variant-numeric: tabular-nums;
  }

  .log-marker {
    text-align: center;
  }

  .log-msg {
    word-break: break-all;
  }

  .log-row.info .log-marker,
  .log-row.info .log-msg {
    color: var(--terminal-muted);
  }
  .log-row.success .log-marker,
  .log-row.success .log-msg {
    color: var(--diff-add);
  }
  .log-row.warning .log-marker,
  .log-row.warning .log-msg {
    color: var(--terminal-text);
  }
  .log-row.error .log-marker,
  .log-row.error .log-msg {
    color: var(--diff-remove);
  }

  .console-actions {
    display: flex;
    justify-content: flex-end;
  }
</style>
