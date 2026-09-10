<script lang="ts">
  import PixelIcon from './PixelIcon.svelte';

  interface ToastMessage {
    id: number;
    type: 'success' | 'error' | 'info';
    text: string;
    remaining: number;
    shownAt: number;
    timer: ReturnType<typeof setTimeout> | null;
  }

  const DURATIONS: Record<ToastMessage['type'], number> = {
    success: 3000,
    info: 4000,
    error: 8000,
  };
  const ICONS: Record<ToastMessage['type'], string> = {
    success: 'check',
    error: 'cross',
    info: 'help',
  };

  let toasts = $state<ToastMessage[]>([]);
  let count = 0;
  let host = $state<HTMLDialogElement>();

  function schedule(toast: ToastMessage) {
    toast.timer = setTimeout(() => dismiss(toast.id), toast.remaining);
    toast.shownAt = Date.now();
  }

  function pause(toast: ToastMessage) {
    if (toast.timer) clearTimeout(toast.timer);
    toast.timer = null;
    toast.remaining = Math.max(500, toast.remaining - (Date.now() - toast.shownAt));
  }

  function resume(toast: ToastMessage) {
    if (toast.timer) return;
    schedule(toast);
  }

  export function show(text: string, type: 'success' | 'error' | 'info' = 'success', duration?: number) {
    const id = ++count;
    const toast: ToastMessage = { id, type, text, remaining: duration ?? DURATIONS[type], shownAt: Date.now(), timer: null };
    schedule(toast);
    toasts = [...toasts, toast];
  }
  export function dismiss(id: number) {
    const toast = toasts.find((t) => t.id === id);
    if (toast?.timer) clearTimeout(toast.timer);
    toasts = toasts.filter((t) => t.id !== id);
  }

  $effect(() => {
    if (!host) return;
    if (toasts.length > 0) {
      if (!host.open) host.show();
    } else if (host.open) {
      host.close();
    }
  });
</script>

<dialog bind:this={host} class="toast-container" aria-label="通知提示" oncancel={(event) => event.preventDefault()}>
  <div class="toast-list" role="status" aria-live="polite">
    {#each toasts as toast (toast.id)}
      <button class="toast-item {toast.type}" type="button" onclick={() => dismiss(toast.id)}
        onpointerenter={() => pause(toast)} onpointerleave={() => resume(toast)}
        onfocus={() => pause(toast)} onblur={() => resume(toast)} title="点击关闭">
        <span class="icon"><PixelIcon name={ICONS[toast.type]} size={12} /></span>
        <span class="text">{toast.text}</span>
      </button>
    {/each}
  </div>
</dialog>

<style>
  .toast-container {
    position: fixed;
    inset: auto 24px 24px auto;
    margin: 0;
    border: 0;
    padding: 0;
    width: max-content;
    max-width: calc(100vw - 32px);
    background: transparent;
    color: var(--text);
    pointer-events: none;
  }

  .toast-container[open] {
    display: block;
  }

  .toast-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .toast-item {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    color: var(--text);
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    box-shadow: var(--shadow-popup);
    padding: 10px 16px;
    font: 400 12px/20px var(--font-ui);
    letter-spacing: 0;
    text-shadow: none;
    animation: pixel-fade 120ms linear;
    pointer-events: auto;
    cursor: pointer;
    text-align: left;
    max-width: 100%;
    word-break: break-word;
    transition: background-color 80ms linear;
  }

  .toast-item:hover {
    background: var(--surface-2);
  }

  .toast-item.success {
    background: var(--status-success);
  }
  .toast-item.error {
    background: var(--status-error);
  }
  .toast-item.info {
    background: var(--status-info);
  }

  .icon {
    flex-shrink: 0;
    display: inline-flex;
  }

  @media (max-width: 600px) {
    .toast-container {
      inset: auto 16px 16px 16px;
      width: auto;
    }
  }
</style>
