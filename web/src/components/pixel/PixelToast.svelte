<script lang="ts">
  interface ToastMessage {
    id: number;
    type: 'success' | 'error' | 'info';
    text: string;
  }

  let toasts = $state<ToastMessage[]>([]);
  let count = 0;

  export function show(text: string, type: 'success' | 'error' | 'info' = 'success', duration = 3000) {
    const id = ++count;
    toasts = [...toasts, { id, type, text }];
    setTimeout(() => {
      toasts = toasts.filter((t) => t.id !== id);
    }, duration);
  }
  export function dismiss(id: number) {
    toasts = toasts.filter((t) => t.id !== id);
  }
</script>

{#if toasts.length > 0}
  <div class="toast-container" role="status" aria-live="polite">
    {#each toasts as toast (toast.id)}
      <button class="toast-item {toast.type}" type="button" onclick={() => dismiss(toast.id)} title="点击关闭">
        <span class="icon">
          {toast.type === 'success' ? '✓' : toast.type === 'error' ? '×' : '›'}
        </span>
        <span class="text">{toast.text}</span>
      </button>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    position: fixed;
    bottom: 24px;
    right: 24px;
    z-index: 200;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: calc(100vw - 32px);
    pointer-events: none;
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
  }

  @media (max-width: 600px) {
    .toast-container {
      bottom: 16px;
      right: 16px;
      left: 16px;
    }
  }
</style>
