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
    color: var(--display);
    border: 2px solid var(--border-vis);
    box-shadow: 4px 4px 0 var(--shadow);
    padding: 10px 16px;
    font-family: "Space Mono", monospace;
    font-size: 12px;
    font-weight: 700;
    animation: pixel-pop 100ms steps(2, end);
    pointer-events: auto;
    cursor: pointer;
    text-align: left;
    max-width: 100%;
    word-break: break-word;
  }

  .toast-item:hover {
    filter: brightness(1.1);
  }

  .toast-item.success {
    border-color: var(--green);
    color: var(--green);
  }
  .toast-item.error {
    border-color: var(--red);
    color: var(--red);
  }
  .toast-item.info {
    border-color: var(--blue);
    color: var(--blue);
  }

  .icon {
    font-weight: 800;
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
