<script lang="ts">
  import type { Snippet } from 'svelte';
  import PixelButton from './PixelButton.svelte';
  import PixelIcon from './PixelIcon.svelte';

  interface Props {
    open: boolean;
    title?: string;
    width?: string;
    onclose?: () => void;
    children?: Snippet;
    footer?: Snippet;
  }

  let {
    open = $bindable(false),
    title = '终端控制台',
    width = '520px',
    onclose,
    children,
    footer,
  }: Props = $props();

  function handleKeydown(ev: KeyboardEvent) {
    if (ev.key === 'Escape' && open) {
      close();
    }
  }

  function close() {
    open = false;
    onclose?.();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div class="drawer-backdrop" onclick={close} role="presentation">
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
      class="drawer-panel"
      style="max-width: {width};"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div class="drawer-header">
        <div class="drawer-title">
          <PixelIcon name="terminal" size={14} color="var(--orange)" />
          <span>{title}</span>
        </div>
        <PixelButton size="sm" variant="ghost" onclick={close} aria-label="关闭抽屉">
          <PixelIcon name="cross" size={10} />
        </PixelButton>
      </div>

      <div class="drawer-body">
        {#if children}
          {@render children()}
        {/if}
      </div>

      {#if footer}
        <div class="drawer-footer">
          {@render footer()}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .drawer-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: var(--backdrop);
    display: flex;
    justify-content: flex-end;
  }

  .drawer-panel {
    width: 100%;
    height: 100%;
    background: var(--surface);
    border-left: 2px solid var(--border-vis);
    box-shadow: -4px 0 0 var(--shadow);
    display: flex;
    flex-direction: column;
    animation: pixel-slide-left 120ms steps(3, end) forwards;
  }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    background: var(--surface-2);
    border-bottom: 2px solid var(--border-vis);
  }

  .drawer-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: "Doto", "Space Mono", monospace;
    font-size: 15px;
    font-weight: 800;
    color: var(--display);
    letter-spacing: 0.05em;
  }

  .drawer-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px 18px;
    scrollbar-color: var(--border-vis) var(--bg);
  }

  .drawer-footer {
    padding: 12px 18px;
    background: var(--surface-2);
    border-top: 2px dashed var(--border-vis);
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }

  @media (max-width: 600px) {
    .drawer-panel {
      max-width: 100% !important;
      border-left: none;
    }
  }
</style>
